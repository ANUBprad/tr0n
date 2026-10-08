package seed

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
)

// The generator's contract without a database: deterministic output,
// every node and edge carrying its provenance (knowledge-model
// invariant), and the schema-required properties actually present.
func TestGenerateDeterministicAndProvenanced(t *testing.T) {
	a, b := Generate(), Generate()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("Generate must be deterministic: two runs differ")
	}

	for label, rows := range a.nodes {
		if len(rows) == 0 {
			t.Errorf("label %s has no rows", label)
		}
		for _, row := range rows {
			for _, k := range []string{"key", "src_type", "src_ref", "observed_at", "extraction"} {
				if v, _ := row[k].(string); v == "" {
					t.Errorf("%s node %v: missing provenance field %q", label, row["key"], k)
				}
			}
		}
	}
	// Required properties the five operations read.
	for _, row := range a.nodes["Incident"] {
		if _, ok := row["started_at"]; !ok {
			t.Errorf("incident %v has no started_at (sort key)", row["key"])
		}
	}
	for _, row := range a.nodes["Meeting"] {
		if _, ok := row["held_at"]; !ok {
			t.Errorf("meeting %v has no held_at", row["key"])
		}
	}
	for _, row := range a.nodes["Document"] {
		if row["kind"] == nil || row["title"] == nil {
			t.Errorf("document %v missing kind/title", row["key"])
		}
	}
	for _, row := range a.nodes["Decision"] {
		if row["status"] == nil || row["title"] == nil {
			t.Errorf("decision %v missing status/title", row["key"])
		}
	}
	for rel, rows := range a.edges {
		if len(rows) == 0 {
			t.Errorf("relationship %s has no rows", rel)
		}
		for _, r := range rows {
			for _, k := range []string{"src_type", "src_ref", "observed_at", "extraction"} {
				if v, _ := r.Props[k].(string); v == "" {
					t.Errorf("%s edge %s->%s: missing provenance field %q", rel, r.From, r.To, k)
				}
			}
		}
	}

	// DATA_SPEC lower bounds (deferred entities excluded by schema lock).
	wantMin := map[string]int{
		"Person": 50, "Team": 8, "Project": 10, "Service": 20,
		"Repository": 15, "Technology": 15, "Document": 60,
		"Decision": 30, "Meeting": 20, "Incident": 30, "PullRequest": 80,
	}
	for label, min := range wantMin {
		if got := len(a.nodes[label]); got < min {
			t.Errorf("%s: %d rows, want >= %d", label, got, min)
		}
	}
}

// The seed exists to serve the five operations. This is the
// connectivity contract: if it passes, the demo scenarios have real
// multi-hop paths to walk.
func TestSeedServesFiveQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_seed_test")
	if err != nil {
		t.Fatalf("falkordb unreachable at %s — run `docker compose up -d`: %v", addr, err)
	}
	t.Cleanup(func() { c.Delete(); c.Close() })
	if _, err := Load(c); err != nil {
		t.Fatalf("seed load: %v", err)
	}
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

	t.Run("find_owner", func(t *testing.T) {
		owners, err := c.FindOwner("payments-api")
		if err != nil {
			t.Fatal(err)
		}
		if len(owners) != 1 || owners[0].Key != "person:1" {
			t.Fatalf("payments-api owner: %+v", owners)
		}
		if owners[0].Provenance.SrcType != "service_registry" {
			t.Errorf("owner provenance: %+v", owners[0].Provenance)
		}
	})

	t.Run("find_experts", func(t *testing.T) {
		res, err := c.FindExperts("service:payments-api", 5, now)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Experts) < 3 {
			t.Fatalf("want >= 3 experts, got %d", len(res.Experts))
		}
		if top := res.Experts[0]; !top.CurrentOwner || top.Key != "person:1" {
			t.Errorf("owner must rank first: %+v", top)
		}
		classes := map[string]bool{}
		for _, e := range res.Experts {
			for _, b := range e.Basis {
				classes[b.Class] = true
			}
		}
		if len(classes) < 3 {
			t.Errorf("want >= 3 signal classes in evidence, got %v", classes)
		}
	})

	t.Run("trace_decision", func(t *testing.T) {
		trace, err := c.TraceDecision("decision:1")
		if err != nil {
			t.Fatal(err)
		}
		if trace == nil {
			t.Fatal("decision:1 not found")
		}
		if len(trace.SupportedBy) < 2 || len(trace.DiscussedIn) < 1 ||
			len(trace.AuthoredBy) < 1 || len(trace.Supersedes) < 1 {
			t.Fatalf("trace incomplete: docs=%d meetings=%d authors=%d supersedes=%d",
				len(trace.SupportedBy), len(trace.DiscussedIn),
				len(trace.AuthoredBy), len(trace.Supersedes))
		}
		if m := trace.DiscussedIn[0]; m.Key != "meeting:1" || len(m.Participants) < 6 {
			t.Errorf("discussed_in: %+v", m)
		}
	})

	t.Run("find_related_incidents", func(t *testing.T) {
		all, err := c.FindRelatedIncidents("service:payments-api", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != 6 || all[0].Key != "incident:1" || all[5].Key != "incident:6" {
			t.Fatalf("want incidents 1-6 newest first, got %d rows", len(all))
		}
		for i := 1; i < len(all); i++ {
			if all[i].StartedAt > all[i-1].StartedAt {
				t.Fatalf("not sorted desc: %s after %s", all[i].StartedAt, all[i-1].StartedAt)
			}
		}
		filtered, err := c.FindRelatedIncidents(
			"service:payments-api", "2026-01-01T00:00:00Z", "timeout")
		if err != nil {
			t.Fatal(err)
		}
		if len(filtered) != 4 {
			t.Errorf("since+timeout: want incidents 1-4, got %d", len(filtered))
		}
	})

	t.Run("trace_evidence", func(t *testing.T) {
		ev, err := c.TraceEvidence([]string{"person:1", "ghost:1"})
		if err != nil {
			t.Fatal(err)
		}
		if !ev[0].Found || len(ev[0].Edges) < 3 || ev[0].Properties["name"] == nil {
			t.Errorf("person:1 evidence: %+v", ev[0])
		}
		if ev[1].Found {
			t.Errorf("ghost must not be found: %+v", ev[1])
		}
	})

	t.Run("resolve_entity", func(t *testing.T) {
		res, err := c.ResolveEntity("payments-api", "Service")
		if err != nil {
			t.Fatal(err)
		}
		if res.Key != "service:payments-api" {
			t.Errorf("resolve: %+v", res)
		}
	})
}
