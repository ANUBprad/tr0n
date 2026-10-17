package evaluation

import (
	"os"
	"testing"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

// openTestGraph builds the seeded synthetic graph plus the adversarial
// fixtures in a disposable database.
func openTestGraph(t *testing.T, name string) *graph.Client {
	t.Helper()
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(); c.Delete() })
	if _, err := seed.Load(c); err != nil {
		t.Fatal(err)
	}
	if err := LoadFixtures(c); err != nil {
		t.Fatal(err)
	}
	return c
}

// The fixtures must make the intended adversarial condition observable in the
// real graph: two incompatible live ownership claims on the same slot.
func TestFixtureConflictIsReal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	c := openTestGraph(t, "tron_eval_fixture_test")

	ctx, err := c.AssembleEvidence(graph.EvidenceRequest{Entity: "service:conflict-demo", Relations: []string{"OWNS"}})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Status != graph.EvidenceStatusOK || len(ctx.Facts) != 2 {
		t.Fatalf("expected two live OWNS facts, got status=%s facts=%d", ctx.Status, len(ctx.Facts))
	}
	owners := map[string]bool{}
	for _, f := range ctx.Facts {
		owners[f.Neighbor.Key] = true
	}
	if len(owners) != 2 {
		t.Fatalf("expected two distinct claimants, got %v", owners)
	}

	conflicts, err := c.FindConflicts("OWNS", "service:conflict-demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts.Conflicts) == 0 {
		t.Fatal("fixture conflict not surfaced by FindConflicts")
	}
}

// The stale/superseded fixture exposes exactly the superseded condition: the
// closed-window claimant held the service in its era but is never current, and
// the open-window claimant is the only current owner.
func TestSupersededOwnerIsNotCurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	c := openTestGraph(t, "tron_eval_superseded_test")

	past, err := c.FindOwner("stale-demo", time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(past) != 1 || past[0].Key != "person:7" {
		t.Fatalf("past era: got %+v, want person:7 only", past)
	}

	now, err := c.FindOwner("stale-demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(now) != 1 || now[0].Key != "person:8" {
		t.Fatalf("current owner: got %+v, want person:8 only", now)
	}
}

// The historical half-open rule is re-checked at and around the seed handoff
// window: [2025-01-31T09:00:00Z, 2025-03-02T09:00:00Z) belongs to person:2,
// the instant 2025-03-02T09:00:00Z belongs to person:1.
func TestHistoricalWindowBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	c := openTestGraph(t, "tron_eval_historical_test")
	at := func(s string) time.Time {
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}
	for _, tc := range []struct {
		when string
		want string // "" means no owner
	}{
		{"2025-01-15T00:00:00Z", ""},
		{"2025-01-31T09:00:00Z", "person:2"}, // valid_from inclusive
		{"2025-02-15T00:00:00Z", "person:2"},
		{"2025-03-02T09:00:00Z", "person:1"}, // valid_to exclusive
		{"2025-06-01T00:00:00Z", "person:1"},
	} {
		owners, err := c.FindOwner("payments-api", at(tc.when))
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		if len(owners) > 0 {
			got = owners[0].Key
		}
		if got != tc.want {
			t.Errorf("as_of %s: got %q want %q", tc.when, got, tc.want)
		}
	}
}

// An ambiguous name must never resolve to a single entity.
func TestAmbiguousEntityStaysAmbiguous(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	c := openTestGraph(t, "tron_eval_ambiguity_test")
	res, err := c.ResolveEntity("Adopt", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Key != "" || len(res.Candidates) == 0 {
		t.Fatalf("ambiguous name resolved to %q with %d candidates", res.Key, len(res.Candidates))
	}
}

// The source-sync freshness contract, verified end to end against the real
// graph: a newer observation with new content updates; an older observation is
// rejected; a different body at the same observation time is rejected; and a
// byte-identical retry is a no-op. No universal "stale threshold" is invented.
func TestSourceFreshnessContract(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	c := openTestGraph(t, "tron_eval_freshness_test")
	rec := graph.SourceRecord{
		Key: "doc:eval-fresh", Title: "Freshness contract probe", Body: "first body",
		Kind: "note", AuthorKey: "person:1", AboutKey: "service:payments-api",
		SourceRef: "acme-eval/freshness/v1", ObservedAt: "2026-01-01T00:00:00Z",
	}
	changed, err := c.ImportRecord(rec)
	if err != nil || !changed {
		t.Fatalf("first import: changed=%v err=%v", changed, err)
	}
	// Identical retry: no-op.
	if changed, err := c.ImportRecord(rec); err != nil || changed {
		t.Fatalf("idempotent retry: changed=%v err=%v", changed, err)
	}
	// Older observation must be refused (no silent regression).
	older := rec
	older.ObservedAt = "2025-12-01T00:00:00Z"
	older.Body = "older body"
	if _, err := c.ImportRecord(older); err == nil {
		t.Fatal("older observation must be rejected")
	}
	// Same time, different content: refused.
	conflict := rec
	conflict.Body = "different body at the same instant"
	if _, err := c.ImportRecord(conflict); err == nil {
		t.Fatal("same-time different-content must be rejected")
	}
	// Newer observation with new content: accepted.
	newer := rec
	newer.ObservedAt = "2026-02-01T00:00:00Z"
	newer.Body = "second body"
	if changed, err := c.ImportRecord(newer); err != nil || !changed {
		t.Fatalf("newer import: changed=%v err=%v", changed, err)
	}
}
