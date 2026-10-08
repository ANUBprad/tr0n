package graph

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ANUBprad/tr0n/internal/knowledge"
)

const testGraph = "tron_test"

// newTestClient isolates each test in a fresh graph. Runs against a
// real FalkorDB — `docker compose up -d` first.
func newTestClient(t *testing.T) *Client {
	t.Helper()
	if testing.Short() {
		t.Skip("integration tests need FalkorDB; use -short to skip")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := New(addr, testGraph)
	if err != nil {
		t.Fatalf("falkordb unreachable at %s — run `docker compose up -d`: %v", addr, err)
	}
	c.g.Delete() // best-effort reset; graph may not exist yet
	t.Cleanup(func() {
		c.g.Delete()
		c.Close()
	})
	return c
}

// The one runnable check for FindOwner: provenance round-trip,
// temporal filtering (stale edges excluded), key lookup, not-found.
func TestFindOwner(t *testing.T) {
	c := newTestClient(t)

	seed := `
	CREATE (ada:Person {key: 'person:ada', name: 'Ada'}),
	       (bob:Person {key: 'person:bob', name: 'Bob'}),
	       (svc:Service {key: 'service:payments', name: 'payments-api'}),
	       (ada)-[:OWNS {src_type: 'service_registry', src_ref: 'registry.json',
	                     observed_at: '2026-10-08T00:00:00Z', extraction: 'deterministic',
	                     valid_from: '2026-02-01', valid_to: null}]->(svc),
	       (bob)-[:OWNS {src_type: 'service_registry', src_ref: 'registry.json',
	                     observed_at: '2026-10-08T00:00:00Z', extraction: 'deterministic',
	                     valid_from: '2024-01-01', valid_to: '2026-02-01'}]->(svc)`
	if _, err := c.g.Query(seed, nil, nil); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	t.Run("current owner by name with provenance", func(t *testing.T) {
		owners, err := c.FindOwner("payments-api")
		if err != nil {
			t.Fatal(err)
		}
		if len(owners) != 1 {
			t.Fatalf("want 1 current owner (Bob's superseded edge excluded), got %d: %+v", len(owners), owners)
		}
		o := owners[0]
		if o.Name != "Ada" || o.Key != "person:ada" || o.Kind != "Person" {
			t.Errorf("wrong owner: %+v", o)
		}
		p := o.Provenance
		if p.SrcType != "service_registry" || p.SrcRef != "registry.json" ||
			p.ObservedAt != "2026-10-08T00:00:00Z" || p.Extraction != "deterministic" ||
			p.ValidFrom != "2026-02-01" {
			t.Errorf("provenance not carried through: %+v", p)
		}
	})

	t.Run("lookup by key", func(t *testing.T) {
		owners, err := c.FindOwner("service:payments")
		if err != nil {
			t.Fatal(err)
		}
		if len(owners) != 1 || owners[0].Key != "person:ada" {
			t.Errorf("want Ada by key, got %+v", owners)
		}
	})

	t.Run("unknown target is empty, not an error", func(t *testing.T) {
		owners, err := c.FindOwner("no-such-thing")
		if err != nil {
			t.Fatal(err)
		}
		if len(owners) != 0 {
			t.Errorf("want empty result, got %+v", owners)
		}
	})
}

// End-to-end check for FindExperts: 5-branch UNION query, positional
// path reconstruction, provenance on every basis event, and
// expertise/v1 ranking (owner-first even when out-scored).
func TestFindExperts(t *testing.T) {
	c := newTestClient(t)

	prov := `{src_type: 'seed', src_ref: 'fixture', observed_at: '2026-10-01T00:00:00Z', extraction: 'deterministic'}`
	provT := `{src_type: 'seed', src_ref: 'fixture', observed_at: '2026-10-01T00:00:00Z', extraction: 'deterministic',
	          valid_from: '2026-01-01', valid_to: null}`
	seed := `
	CREATE (ada:Person {key: 'person:ada', name: 'Ada'}),
	       (bob:Person {key: 'person:bob', name: 'Bob'}),
	       (svc:Service {key: 'service:payments', name: 'payments-api'}),
	       (repo:Repository {key: 'repo:api', name: 'api'}),
	       (svc)-[:HAS_REPO ` + provT + `]->(repo),
	       (pr1:PullRequest {key: 'pr:1', title: 'Fix auth', number: 1, state: 'merged',
	                         created_at: '2026-09-01T00:00:00Z'}),
	       (pr2:PullRequest {key: 'pr:2', title: 'Old refactor', number: 2, state: 'merged',
	                         created_at: '2025-01-01T00:00:00Z'}),
	       (ada)-[:AUTHORED ` + prov + `]->(pr1),
	       (ada)-[:AUTHORED ` + prov + `]->(pr2),
	       (bob)-[:REVIEWED ` + prov + `]->(pr1),
	       (pr1)-[:MODIFIED ` + prov + `]->(repo),
	       (pr2)-[:MODIFIED ` + prov + `]->(repo),
	       (inc:Incident {key: 'incident:1', title: 'Auth outage', severity: 'sev1',
	                      started_at: '2026-09-15T00:00:00Z', resolved_at: '2026-09-16T00:00:00Z'}),
	       (inc)-[:AFFECTS ` + prov + `]->(svc),
	       (inc)-[:RESOLVED_BY ` + prov + `]->(bob),
	       (doc:Document {key: 'doc:1', title: 'Auth runbook', kind: 'runbook'}),
	       (ada)-[:AUTHORED ` + prov + `]->(doc),
	       (doc)-[:ABOUT ` + prov + `]->(svc),
	       (ada)-[:OWNS ` + provT + `]->(svc)`
	if _, err := c.g.Query(seed, nil, nil); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	halfLife := func(days float64) float64 { return math.Pow(0.5, days/180) }

	got, err := c.FindExperts("service:payments", 5, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Layer != "INFERENCE" || got.RuleID != "expertise/v1" {
		t.Fatalf("tagging: %+v", got)
	}
	if len(got.Experts) != 2 {
		t.Fatalf("want 2 experts, got %d: %+v", len(got.Experts), got.Experts)
	}

	ada := got.Experts[0]
	bob := got.Experts[1]
	if ada.Key != "person:ada" || !ada.CurrentOwner {
		t.Errorf("owner must rank first: %+v", ada)
	}
	if bob.Key != "person:bob" || bob.CurrentOwner {
		t.Errorf("second should be Bob: %+v", bob)
	}
	// Bob's raw score beats Ada's — owner-first is a hard signal.
	wantAda := 1.0*halfLife(37) + 1.0*halfLife(645) + 0.8
	wantBob := 0.6*halfLife(37) + 1.5*halfLife(22)
	if !(bob.Score > ada.Score) {
		t.Errorf("fixture should out-score the owner: ada=%v bob=%v", ada.Score, bob.Score)
	}
	if math.Abs(ada.Score-wantAda) > 1e-9 || math.Abs(bob.Score-wantBob) > 1e-9 {
		t.Errorf("scores ada=%v want %v, bob=%v want %v", ada.Score, wantAda, bob.Score, wantBob)
	}
	if ada.TopSignals["authored_pr"] != 2 || ada.TopSignals["owner"] != 1 ||
		ada.TopSignals["authored_doc"] != 1 || bob.TopSignals["reviewed_pr"] != 1 ||
		bob.TopSignals["resolved_incident"] != 1 {
		t.Errorf("top signals ada=%+v bob=%+v", ada.TopSignals, bob.TopSignals)
	}

	// Every basis event carries a concrete fact path and edge provenance.
	for _, e := range append(ada.Basis, bob.Basis...) {
		if e.Path == "" {
			t.Errorf("empty basis path: %+v", e)
		}
		if len(e.Evidence) == 0 || e.Evidence[0].SrcType != "seed" {
			t.Errorf("basis evidence missing provenance: %+v", e)
		}
	}
	ownerBasis := basisByClass(t, ada, "owner")
	if want := "(:Person person:ada)-[:OWNS]->(:Service service:payments)"; ownerBasis.Path != want {
		t.Errorf("owner path = %q, want %q", ownerBasis.Path, want)
	}
	var prPaths []string
	for _, b := range ada.Basis {
		if b.Class == "authored_pr" {
			prPaths = append(prPaths, b.Path)
		}
	}
	joined := strings.Join(prPaths, " | ")
	if len(prPaths) != 2 || !strings.Contains(joined, "pr:1") || !strings.Contains(joined, "pr:2") ||
		!strings.Contains(joined, "-[:AUTHORED]->") || !strings.Contains(joined, "<-[:HAS_REPO]-") {
		t.Errorf("authored_pr paths: %q", joined)
	}

	t.Run("undated doc event shows empty occurred_at", func(t *testing.T) {
		for _, b := range ada.Basis {
			if b.Class == "authored_doc" && b.OccurredAt != "" {
				t.Errorf("want empty occurred_at, got %q", b.OccurredAt)
			}
		}
	})

	t.Run("limit keeps owner only", func(t *testing.T) {
		one, err := c.FindExperts("service:payments", 1, now)
		if err != nil {
			t.Fatal(err)
		}
		if len(one.Experts) != 1 || one.Experts[0].Key != "person:ada" {
			t.Errorf("limit: %+v", one.Experts)
		}
	})

	t.Run("unknown service is empty, not an error", func(t *testing.T) {
		none, err := c.FindExperts("service:ghost", 5, now)
		if err != nil {
			t.Fatal(err)
		}
		if len(none.Experts) != 0 {
			t.Errorf("want empty, got %+v", none.Experts)
		}
	})
}

func basisByClass(t *testing.T, e knowledge.Expert, class string) knowledge.Basis {
	t.Helper()
	for _, b := range e.Basis {
		if b.Class == class {
			return b
		}
	}
	t.Fatalf("no %s basis for %s: %+v", class, e.Key, e.Basis)
	return knowledge.Basis{}
}
