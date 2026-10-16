package graph

// M3 deterministic evidence retrieval & context assembly
// (docs/REASONING.md §2). Every case runs against a real FalkorDB.

import (
	"encoding/json"
	"strings"
	"testing"
)

// evidenceFixture is one entity's neighbourhood plus deliberately
// unrelated neighbours and a deliberately colliding display name:
//
//	service:payments "payments-api"  ← OWNS ada [2026-02-01, 2026-06-01)
//	                                 ← OWNS bob [2026-06-01, open)
//	service:billing  "billing-api"   ← OWNS bob (nothing to do with payments)
//	service:mail-a / service:mail-b   both named "mail-api" → ambiguous
//	person:ada       → AUTHORED → decision:1 (event, no window)
func evidenceFixture(t *testing.T, c *Client) {
	t.Helper()
	row := func(key, name string) map[string]interface{} {
		r := nodeRow(key)
		r["name"] = name
		return r
	}
	mustLoadNodes(t, c, "Person", []map[string]interface{}{
		row("person:ada", "Ada"),
		row("person:bob", "Bob"),
	})
	mustLoadNodes(t, c, "Service", []map[string]interface{}{
		row("service:payments", "payments-api"),
		row("service:billing", "billing-api"),
		row("service:mail-a", "mail-api"),
		row("service:mail-b", "mail-api"),
	})
	decision := nodeRow("decision:1")
	decision["title"] = "Adopt FalkorDB"
	mustLoadNodes(t, c, "Decision", []map[string]interface{}{decision})

	own := func(from, to, vf, vt string) EdgeRow {
		p := edgeProps()
		p["valid_from"] = vf
		if vt != "" {
			p["valid_to"] = vt
		}
		return EdgeRow{From: from, To: to, Props: p}
	}
	loadEdges(t, c, "OWNS", []EdgeRow{
		own("person:ada", "service:payments", "2026-02-01T00:00:00Z", "2026-06-01T00:00:00Z"),
		own("person:bob", "service:payments", "2026-06-01T00:00:00Z", ""),
		own("person:bob", "service:billing", "2026-01-01T00:00:00Z", ""),
	})
	loadEdges(t, c, "AUTHORED", []EdgeRow{
		{From: "person:ada", To: "decision:1", Props: edgeProps()},
	})
}

func loadEdges(t *testing.T, c *Client, rel string, rows []EdgeRow) {
	t.Helper()
	if err := c.LoadEdges(rel, rows); err != nil {
		t.Fatalf("LoadEdges(%s): %v", rel, err)
	}
}

func factOrder(f EvidenceFact) string {
	return strings.Join([]string{
		f.Entity, f.Relation, f.Direction, f.Neighbor.Key,
		f.Provenance.ValidFrom, f.Provenance.SrcRef,
	}, "|")
}

// The regression test: retrieval must return the requested entity's own
// facts with full provenance and windows, and must not leak unrelated
// evidence (which is exactly what a full-graph dump would do).
func TestAssembleEvidenceRelevantAndProvenanced(t *testing.T) {
	c := newTestClient(t)
	evidenceFixture(t, c)
	nodes, edges := nodeCount(t, c), edgeCount(t, c)

	got, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != EvidenceStatusOK {
		t.Fatalf("status: %+v", got)
	}
	if got.Entity == nil || got.Entity.Key != "service:payments" || got.Entity.Label != "Service" {
		t.Fatalf("resolved entity: %+v", got.Entity)
	}
	if len(got.Facts) == 0 {
		t.Fatal("no facts returned for a connected entity")
	}

	ownCount := 0
	for _, f := range got.Facts {
		if f.Entity != "service:payments" {
			t.Errorf("fact anchored to the wrong entity: %+v", f)
		}
		if f.Provenance.SrcRef == "" || f.Provenance.ObservedAt == "" || f.Provenance.SrcType == "" {
			t.Errorf("provenance dropped: %+v", f)
		}
		if f.Relation == "OWNS" {
			ownCount++
			if f.Provenance.ValidFrom == "" {
				t.Errorf("state relation lost its validity window: %+v", f)
			}
		}
	}
	if ownCount != 2 {
		t.Errorf("want both OWNS facts, got %d", ownCount)
	}

	// Unrelated evidence must not appear anywhere in the context.
	blob, _ := json.Marshal(got)
	for _, foreign := range []string{"service:billing", "service:mail-a", "decision:1"} {
		if strings.Contains(string(blob), foreign) {
			t.Errorf("unrelated evidence %q leaked into the context: %s", foreign, blob)
		}
	}
	// Retrieved facts are evidence only — nothing is scored or promoted.
	if strings.Contains(string(blob), "rule_id") || strings.Contains(string(blob), "INFERENCE") {
		t.Errorf("retrieval promoted a fact into an inference: %s", blob)
	}
	// Read-only: the graph is untouched and nothing was stored.
	if nn, ee := nodeCount(t, c), edgeCount(t, c); nn != nodes || ee != edges {
		t.Errorf("retrieval mutated the graph: nodes %d->%d edges %d->%d", nodes, nn, edges, ee)
	}

	t.Run("relation scope narrows deliberately", func(t *testing.T) {
		scoped, err := c.AssembleEvidence(EvidenceRequest{
			Entity: "payments-api", Relations: []string{"OWNS"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(scoped.Facts) != 2 {
			t.Fatalf("want only the 2 OWNS facts, got %d", len(scoped.Facts))
		}
		for _, f := range scoped.Facts {
			if f.Relation != "OWNS" {
				t.Errorf("scope leaked %s", f.Relation)
			}
		}
	})
}

// Historical retrieval follows the canonical half-open rule
// (valid_from <= as_of < valid_to); event relations carry no window and
// are not window-filtered, exactly as FindExperts treats them.
func TestAssembleEvidenceAsOfHalfOpen(t *testing.T) {
	c := newTestClient(t)
	evidenceFixture(t, c)

	owners := func(asOf string) map[string]bool {
		t.Helper()
		got, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api", AsOf: asOf})
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != EvidenceStatusOK {
			t.Fatalf("status at %q: %+v", asOf, got)
		}
		seen := map[string]bool{}
		for _, f := range got.Facts {
			if f.Relation == "OWNS" {
				seen[f.Neighbor.Key] = true
			}
		}
		return seen
	}

	inWindow := owners("2026-03-01T00:00:00Z")
	if !inWindow["person:ada"] || inWindow["person:bob"] || len(inWindow) != 1 {
		t.Errorf("as_of inside ada's window: %v, want only person:ada", inWindow)
	}
	afterHandoff := owners("2026-07-01T00:00:00Z")
	if !afterHandoff["person:bob"] || afterHandoff["person:ada"] || len(afterHandoff) != 1 {
		t.Errorf("as_of after handoff: %v, want only person:bob", afterHandoff)
	}
	// Boundary: valid_from is inclusive, valid_to exclusive.
	atStart := owners("2026-02-01T00:00:00Z")
	if !atStart["person:ada"] {
		t.Errorf("valid_from instant must be valid: %v", atStart)
	}
	atEnd := owners("2026-06-01T00:00:00Z")
	if atEnd["person:ada"] || !atEnd["person:bob"] {
		t.Errorf("valid_to instant must be expired, handoff day is bob's: %v", atEnd)
	}
	// No as_of = no temporal restriction: the full history is in scope.
	all := owners("")
	if !all["person:ada"] || !all["person:bob"] {
		t.Errorf("omitted as_of must keep every window: %v", all)
	}

	// State relations filter; the event fact on the same anchor survives
	// any as_of (events have no valid_to by contract).
	for _, asOf := range []string{"2026-03-01T00:00:00Z", "2030-01-01T00:00:00Z"} {
		got, err := c.AssembleEvidence(EvidenceRequest{Entity: "Ada", AsOf: asOf})
		if err != nil {
			t.Fatal(err)
		}
		var authored bool
		for _, f := range got.Facts {
			if f.Relation == "AUTHORED" && f.Neighbor.Key == "decision:1" {
				authored = true
			}
		}
		if !authored {
			t.Errorf("event relation dropped at as_of %s: %+v", asOf, got.Facts)
		}
	}
}

// Unknown, ambiguous, empty, and malformed-temporal inputs each have an
// explicit outcome: never a guess, never a silent ignore.
func TestAssembleEvidenceExplicitOutcomes(t *testing.T) {
	c := newTestClient(t)
	evidenceFixture(t, c)

	t.Run("ambiguous resolution picks nothing", func(t *testing.T) {
		got, err := c.AssembleEvidence(EvidenceRequest{Entity: "mail-api"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != EvidenceStatusAmbiguous {
			t.Fatalf("status: %+v", got)
		}
		if len(got.Candidates) != 2 || got.Entity != nil || len(got.Facts) != 0 {
			t.Fatalf("ambiguous must return candidates and no facts: %+v", got)
		}
		blob, _ := json.Marshal(got.Candidates)
		if !strings.Contains(string(blob), "service:mail-a") ||
			!strings.Contains(string(blob), "service:mail-b") {
			t.Errorf("both candidates must surface: %s", blob)
		}
	})

	t.Run("missing entity is not_found", func(t *testing.T) {
		got, err := c.AssembleEvidence(EvidenceRequest{Entity: "no-such-entity"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != EvidenceStatusNotFound || len(got.Facts) != 0 || got.Entity != nil {
			t.Fatalf("missing entity: %+v", got)
		}
	})

	t.Run("resolved entity with no in-scope fact is empty", func(t *testing.T) {
		got, err := c.AssembleEvidence(EvidenceRequest{
			Entity: "Ada", Relations: []string{"HAS_REPO"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != EvidenceStatusEmpty || len(got.Facts) != 0 {
			t.Fatalf("empty scope: %+v", got)
		}
	})

	t.Run("malformed as_of is an error, not a silent no-filter", func(t *testing.T) {
		got, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api", AsOf: "last tuesday"})
		if err == nil {
			t.Fatalf("want an error, got %+v", got)
		}
		if !strings.Contains(err.Error(), "RFC3339") {
			t.Errorf("error should name the format: %v", err)
		}
		if len(got.Facts) != 0 {
			t.Errorf("a failed request must not return facts: %+v", got)
		}
	})

	t.Run("empty entity is an error", func(t *testing.T) {
		if _, err := c.AssembleEvidence(EvidenceRequest{Entity: "   "}); err == nil {
			t.Error("want an error for an empty entity")
		}
	})
}

// Both documented bounds bite, and truncation is reported rather than
// left implied.
func TestAssembleEvidenceBounds(t *testing.T) {
	c := newTestClient(t)
	evidenceFixture(t, c)

	full, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Facts) < 2 {
		t.Fatalf("fixture needs >=2 facts to truncate, got %d", len(full.Facts))
	}

	t.Run("count bound", func(t *testing.T) {
		got, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api", Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Facts) != 1 || !got.Truncated ||
			got.TruncatedReason != EvidenceTruncatedByCount || got.Limit != 1 {
			t.Errorf("count bound: %+v", got)
		}
	})

	t.Run("size bound", func(t *testing.T) {
		// Exactly one fact fits; the second must be dropped and said so.
		maxBytes := factsJSONSize(full.Facts[:1])
		got, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api", MaxBytes: maxBytes})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Facts) != 1 || !got.Truncated ||
			got.TruncatedReason != EvidenceTruncatedBySize {
			t.Errorf("size bound (max %d): %+v", maxBytes, got)
		}
		if got.Bytes > maxBytes {
			t.Errorf("serialized size %d exceeds the %d bound", got.Bytes, maxBytes)
		}
	})

	t.Run("defaults bound without truncating", func(t *testing.T) {
		got, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Limit != DefaultEvidenceLimit || got.Truncated {
			t.Errorf("default bounds: %+v", got)
		}
		if got.Bytes > DefaultEvidenceMaxBytes {
			t.Errorf("default size bound exceeded: %d", got.Bytes)
		}
	})
}

// The same facts come back in the same order no matter what order the
// store hands them back in.
func TestAssembleEvidenceDeterministicOrdering(t *testing.T) {
	c := newTestClient(t)
	evidenceFixture(t, c)

	first, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api"})
	if err != nil {
		t.Fatal(err)
	}
	firstBlob, _ := json.Marshal(first)

	// Rebuild the identical facts in reverse insertion order.
	if _, err := c.g.Query("MATCH (n) DETACH DELETE n", nil, nil); err != nil {
		t.Fatal(err)
	}
	evidenceFixtureReversed(t, c)

	second, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api"})
	if err != nil {
		t.Fatal(err)
	}
	secondBlob, _ := json.Marshal(second)
	if string(firstBlob) != string(secondBlob) {
		t.Errorf("output depends on store order:\n first: %s\nsecond: %s", firstBlob, secondBlob)
	}

	for i := 1; i < len(second.Facts); i++ {
		if factOrder(second.Facts[i-1]) > factOrder(second.Facts[i]) {
			t.Errorf("facts not sorted:\n %s\n %s",
				factOrder(second.Facts[i-1]), factOrder(second.Facts[i]))
		}
	}
}

// evidenceFixtureReversed writes evidenceFixture's nodes and edges in
// the opposite order, so a result that changes is a result that depends
// on scan order rather than on the documented sort.
func evidenceFixtureReversed(t *testing.T, c *Client) {
	t.Helper()
	rows := func(rs []map[string]interface{}) []map[string]interface{} {
		out := make([]map[string]interface{}, len(rs))
		for i, r := range rs {
			out[len(rs)-1-i] = r
		}
		return out
	}
	row := func(key, name string) map[string]interface{} {
		r := nodeRow(key)
		r["name"] = name
		return r
	}
	decision := nodeRow("decision:1")
	decision["title"] = "Adopt FalkorDB"
	mustLoadNodes(t, c, "Decision", []map[string]interface{}{decision})
	mustLoadNodes(t, c, "Service", rows([]map[string]interface{}{
		row("service:mail-b", "mail-api"), row("service:mail-a", "mail-api"),
		row("service:billing", "billing-api"), row("service:payments", "payments-api"),
	}))
	mustLoadNodes(t, c, "Person", rows([]map[string]interface{}{
		row("person:bob", "Bob"), row("person:ada", "Ada"),
	}))

	own := func(from, to, vf, vt string) EdgeRow {
		p := edgeProps()
		p["valid_from"] = vf
		if vt != "" {
			p["valid_to"] = vt
		}
		return EdgeRow{From: from, To: to, Props: p}
	}
	loadEdges(t, c, "AUTHORED", []EdgeRow{
		{From: "person:ada", To: "decision:1", Props: edgeProps()},
	})
	loadEdges(t, c, "OWNS", []EdgeRow{
		own("person:bob", "service:billing", "2026-01-01T00:00:00Z", ""),
		own("person:bob", "service:payments", "2026-06-01T00:00:00Z", ""),
		own("person:ada", "service:payments", "2026-02-01T00:00:00Z", "2026-06-01T00:00:00Z"),
	})
}
