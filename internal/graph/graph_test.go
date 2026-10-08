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

// Bounded neighborhood of a decision: authors, meetings + participants,
// supporting docs, ≤2-hop supersedes chain — all with edge provenance.
func TestTraceDecision(t *testing.T) {
	c := newTestClient(t)

	prov := `{src_type: 'adr_repo', src_ref: 'adrs/', observed_at: '2026-10-01T00:00:00Z', extraction: 'deterministic'}`
	seed := `
	CREATE (ada:Person {key: 'person:ada', name: 'Ada'}),
	       (bob:Person {key: 'person:bob', name: 'Bob'}),
	       (dec:Decision {key: 'decision:42', title: 'Use FalkorDB', status: 'accepted',
	                      decided_at: '2026-03-01T00:00:00Z'}),
	       (prev:Decision {key: 'decision:7', title: 'Use Postgres', status: 'superseded',
	                       decided_at: '2025-06-01T00:00:00Z'}),
	       (old:Decision {key: 'decision:3', title: 'Prototype in SQLite', status: 'superseded',
	                      decided_at: '2024-01-01T00:00:00Z'}),
	       (ada)-[:AUTHORED ` + prov + `]->(dec),
	       (mtg:Meeting {key: 'meeting:1', title: 'Graph kickoff',
	                    held_at: '2026-02-20T15:00:00Z'}),
	       (dec)-[:DISCUSSED_IN ` + prov + `]->(mtg),
	       (ada)-[:PARTICIPATED_IN ` + prov + `]->(mtg),
	       (bob)-[:PARTICIPATED_IN ` + prov + `]->(mtg),
	       (doc:Document {key: 'doc:falkor-eval', title: 'FalkorDB evaluation',
	                     kind: 'adr', url: 'https://example/adr/1'}),
	       (dec)-[:SUPPORTED_BY ` + prov + `]->(doc),
	       (dec)-[:SUPERSEDES ` + prov + `]->(prev),
	       (prev)-[:SUPERSEDES ` + prov + `]->(old)`
	if _, err := c.g.Query(seed, nil, nil); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	trace, err := c.TraceDecision("decision:42")
	if err != nil {
		t.Fatal(err)
	}
	if trace == nil {
		t.Fatal("want trace, got nil")
	}
	d := trace.Decision
	if d.Key != "decision:42" || d.Title != "Use FalkorDB" || d.Status != "accepted" ||
		d.DecidedAt != "2026-03-01T00:00:00Z" {
		t.Errorf("decision: %+v", d)
	}
	if len(trace.AuthoredBy) != 1 || trace.AuthoredBy[0].Key != "person:ada" ||
		trace.AuthoredBy[0].Provenance.SrcType != "adr_repo" {
		t.Errorf("authors: %+v", trace.AuthoredBy)
	}
	if len(trace.DiscussedIn) != 1 {
		t.Fatalf("meetings: %+v", trace.DiscussedIn)
	}
	m := trace.DiscussedIn[0]
	if m.Key != "meeting:1" || m.HeldAt != "2026-02-20T15:00:00Z" || len(m.Participants) != 2 ||
		m.Participants[0].Key != "person:ada" || m.Participants[1].Key != "person:bob" ||
		m.Participants[0].Provenance.SrcType != "adr_repo" {
		t.Errorf("meeting: %+v", m)
	}
	if m.Provenance.SrcType != "adr_repo" {
		t.Errorf("meeting provenance: %+v", m.Provenance)
	}
	if len(trace.SupportedBy) != 1 {
		t.Fatalf("docs: %+v", trace.SupportedBy)
	}
	doc := trace.SupportedBy[0]
	if doc.Key != "doc:falkor-eval" || doc.Kind != "adr" || doc.URL != "https://example/adr/1" ||
		doc.Provenance.SrcType != "adr_repo" {
		t.Errorf("doc: %+v", doc)
	}
	if len(trace.Supersedes) != 2 {
		t.Fatalf("supersedes: %+v", trace.Supersedes)
	}
	// sorted by key; hop depth visible in evidence length
	if s := trace.Supersedes[0]; s.Key != "decision:3" || len(s.Evidence) != 2 {
		t.Errorf("2-hop supersede: %+v", s)
	}
	if s := trace.Supersedes[1]; s.Key != "decision:7" || len(s.Evidence) != 1 ||
		s.Evidence[0].SrcType != "adr_repo" {
		t.Errorf("1-hop supersede: %+v", s)
	}

	t.Run("lookup by title", func(t *testing.T) {
		byTitle, err := c.TraceDecision("Use FalkorDB")
		if err != nil {
			t.Fatal(err)
		}
		if byTitle == nil || byTitle.Decision.Key != "decision:42" {
			t.Errorf("by title: %+v", byTitle)
		}
	})

	t.Run("unknown decision is nil, not an error", func(t *testing.T) {
		none, err := c.TraceDecision("decision:ghost")
		if err != nil {
			t.Fatal(err)
		}
		if none != nil {
			t.Errorf("want nil, got %+v", none)
		}
	})
}

// Temporal + literal-keyword filtering over incidents affecting a
// service, newest first, with resolvers and AFFECTS provenance.
func TestFindRelatedIncidents(t *testing.T) {
	c := newTestClient(t)

	prov := `{src_type: 'incident_tracker', src_ref: 'pager', observed_at: '2026-10-01T00:00:00Z', extraction: 'deterministic'}`
	seed := `
	CREATE (svc:Service {key: 'service:payments', name: 'payments-api'}),
	       (other:Service {key: 'service:db', name: 'db'}),
	       (ada:Person {key: 'person:ada', name: 'Ada'}),
	       (bob:Person {key: 'person:bob', name: 'Bob'}),
	       (inc1:Incident {key: 'incident:1', title: 'Payment gateway timeout', severity: 'sev1',
	                       started_at: '2026-09-10T02:00:00Z', resolved_at: '2026-09-10T04:00:00Z'}),
	       (inc1)-[:AFFECTS ` + prov + `]->(svc),
	       (inc1)-[:RESOLVED_BY ` + prov + `]->(bob),
	       (inc1)-[:RESOLVED_BY ` + prov + `]->(ada),
	       (inc2:Incident {key: 'incident:2', title: 'Checkout latency spike', severity: 'sev2',
	                       started_at: '2026-05-01T00:00:00Z'}),
	       (inc2)-[:AFFECTS ` + prov + `]->(svc),
	       (inc3:Incident {key: 'incident:3', title: 'Primary DB failover', severity: 'sev1',
	                       started_at: '2026-09-20T00:00:00Z', resolved_at: '2026-09-20T01:00:00Z'}),
	       (inc3)-[:AFFECTS ` + prov + `]->(other)`
	if _, err := c.g.Query(seed, nil, nil); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	all, err := c.FindRelatedIncidents("service:payments", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 incidents (not other-service inc3), got %d: %+v", len(all), all)
	}
	if all[0].Key != "incident:1" || all[1].Key != "incident:2" {
		t.Errorf("want newest first, got %v, %v", all[0].Key, all[1].Key)
	}
	i1 := all[0]
	if i1.Severity != "sev1" || i1.ResolvedAt != "2026-09-10T04:00:00Z" ||
		i1.Provenance.SrcType != "incident_tracker" {
		t.Errorf("incident: %+v", i1)
	}
	if len(i1.ResolvedBy) != 2 || i1.ResolvedBy[0].Key != "person:ada" ||
		i1.ResolvedBy[1].Key != "person:bob" ||
		i1.ResolvedBy[0].Provenance.SrcType != "incident_tracker" {
		t.Errorf("resolvers: %+v", i1.ResolvedBy)
	}
	if all[1].ResolvedAt != "" || len(all[1].ResolvedBy) != 0 {
		t.Errorf("inc2 unresolved: %+v", all[1])
	}

	t.Run("since filter", func(t *testing.T) {
		got, err := c.FindRelatedIncidents("service:payments", "2026-06-01T00:00:00Z", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Key != "incident:1" {
			t.Errorf("since: %+v", got)
		}
	})

	t.Run("keyword is case-insensitive literal", func(t *testing.T) {
		got, err := c.FindRelatedIncidents("service:payments", "", "TiMeOuT")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Key != "incident:1" {
			t.Errorf("keyword: %+v", got)
		}
		none, err := c.FindRelatedIncidents("service:payments", "", "no-such-word")
		if err != nil {
			t.Fatal(err)
		}
		if len(none) != 0 {
			t.Errorf("want empty, got %+v", none)
		}
	})

	t.Run("invalid since is an error, not a silent pass", func(t *testing.T) {
		if _, err := c.FindRelatedIncidents("service:payments", "yesterday", ""); err == nil {
			t.Error("want error for non-RFC3339 since")
		}
	})
}

// Re-fetch with full properties + 1-hop context, both directions,
// complete provenance — the drill-down behind "why do you believe this?".
func TestTraceEvidence(t *testing.T) {
	c := newTestClient(t)

	prov := `{src_type: 'adr_repo', src_ref: 'adr/0007', observed_at: '2026-10-01T00:00:00Z', extraction: 'deterministic', valid_from: '2026-01-01T00:00:00Z'}`
	seed := `
	CREATE (ada:Person {key: 'person:ada', name: 'Ada', team: 'platform'}),
	       (svc:Service {key: 'service:payments', name: 'payments-api'}),
	       (inc:Incident {key: 'incident:1', title: 'Payment gateway timeout'}),
	       (ada)-[:OWNS ` + prov + `]->(svc),
	       (inc)-[:RESOLVED_BY ` + prov + `]->(ada),
	       (inc)-[:AFFECTS ` + prov + `]->(svc)`
	if _, err := c.g.Query(seed, nil, nil); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	keys := []string{"person:ada", "service:payments", "person:ghost"}
	ev, err := c.TraceEvidence(keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 3 || ev[0].Key != "person:ada" || ev[1].Key != "service:payments" ||
		ev[2].Key != "person:ghost" {
		t.Fatalf("input order must be preserved: %+v", ev)
	}

	ada := ev[0]
	if !ada.Found || ada.Label != "Person" || ada.Properties["team"] != "platform" ||
		len(ada.Edges) != 2 {
		t.Fatalf("ada full refetch: %+v", ada)
	}
	if e := ada.Edges[0]; e.Relation != "OWNS" || e.Direction != "out" ||
		e.Neighbor.Key != "service:payments" || e.Provenance.SrcRef != "adr/0007" ||
		e.Provenance.ValidFrom != "2026-01-01T00:00:00Z" {
		t.Errorf("out edge: %+v", e)
	}
	if e := ada.Edges[1]; e.Relation != "RESOLVED_BY" || e.Direction != "in" ||
		e.Neighbor.Key != "incident:1" || e.Neighbor.Label != "Incident" {
		t.Errorf("in edge: %+v", e)
	}

	svc := ev[1]
	if !svc.Found || len(svc.Edges) != 2 {
		t.Fatalf("svc: %+v", svc)
	}
	for _, e := range svc.Edges {
		if e.Provenance.SrcType != "adr_repo" {
			t.Errorf("every edge carries provenance: %+v", e)
		}
	}
	if svc.Edges[0].Direction != "in" || svc.Edges[1].Direction != "in" {
		t.Errorf("svc only has inbound edges: %+v", svc.Edges)
	}

	if ev[2].Found || len(ev[2].Edges) != 0 {
		t.Errorf("unknown key must surface as found=false, not vanish: %+v", ev[2])
	}

	if empty, err := c.TraceEvidence(nil); err != nil || len(empty) != 0 {
		t.Errorf("empty input: %v, %v", empty, err)
	}
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
