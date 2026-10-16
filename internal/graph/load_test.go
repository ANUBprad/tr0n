package graph

import (
	"strings"
	"testing"
)

// The fact-write contract (docs/knowledge-model.md, docs/GRAPH_MODEL.md,
// docs/SCHEMA.md §4.1) enforced at the shared writer. Runs against a
// real FalkorDB — `docker compose up -d` first.

func nodeRow(key string) map[string]interface{} {
	return map[string]interface{}{
		"key": key, "src_type": "hr_registry", "src_ref": "hr/registry.json",
		"observed_at": "2026-10-01T00:00:00Z", "extraction": "deterministic",
	}
}

func edgeProps() map[string]interface{} {
	return map[string]interface{}{
		"src_type": "service_registry", "src_ref": "registry.json",
		"observed_at": "2026-10-01T00:00:00Z", "extraction": "deterministic",
	}
}

func countScalar(t *testing.T, c *Client, q string) int {
	t.Helper()
	res, err := c.g.ROQuery(q, nil, nil)
	if err != nil {
		// FalkorDB rejects reads against a graph that does not exist yet;
		// an empty graph has zero of everything.
		if strings.Contains(err.Error(), "empty key") {
			return 0
		}
		t.Fatalf("scalar query %q: %v", q, err)
	}
	if !res.Next() {
		return 0
	}
	v, _ := res.Record().Get("c")
	n, ok := v.(int64)
	if !ok {
		t.Fatalf("count %q: unexpected type %T", q, v)
	}
	return int(n)
}

func nodeCount(t *testing.T, c *Client) int {
	return countScalar(t, c, "MATCH (n) RETURN count(n) AS c")
}

func edgeCount(t *testing.T, c *Client) int {
	return countScalar(t, c, "MATCH ()-[r]->() RETURN count(r) AS c")
}

func mustLoadNodes(t *testing.T, c *Client, label string, rows []map[string]interface{}) {
	t.Helper()
	if err := c.LoadNodes(label, rows); err != nil {
		t.Fatalf("LoadNodes(%s): %v", label, err)
	}
}

// Valid batches load, persist provenance, and a state edge keeps its
// valid_from while an event edge carries none.
func TestLoadValidBatches(t *testing.T) {
	c := newTestClient(t)
	mustLoadNodes(t, c, "Person", []map[string]interface{}{nodeRow("person:ada")})
	mustLoadNodes(t, c, "Team", []map[string]interface{}{nodeRow("team:core")})
	mustLoadNodes(t, c, "Document", []map[string]interface{}{nodeRow("doc:1")})

	if n := nodeCount(t, c); n != 3 {
		t.Fatalf("nodes = %d, want 3", n)
	}

	state := edgeProps()
	state["valid_from"] = "2026-01-01T00:00:00Z"
	if err := c.LoadEdges("WORKS_IN", []EdgeRow{
		{From: "person:ada", To: "team:core", Props: state},
	}); err != nil {
		t.Fatalf("state edge: %v", err)
	}
	if err := c.LoadEdges("AUTHORED", []EdgeRow{
		{From: "person:ada", To: "doc:1", Props: edgeProps()},
	}); err != nil {
		t.Fatalf("event edge: %v", err)
	}
	if e := edgeCount(t, c); e != 2 {
		t.Fatalf("edges = %d, want 2", e)
	}

	res, err := c.g.ROQuery(`MATCH ()-[r:WORKS_IN]->() RETURN r.src_type AS src_type, r.valid_from AS valid_from`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Next() {
		t.Fatal("WORK_IN edge missing after load")
	}
	rec := res.Record()
	if got := field(rec, "src_type"); got != "service_registry" {
		t.Errorf("state src_type = %q, want service_registry", got)
	}
	if got := field(rec, "valid_from"); got != "2026-01-01T00:00:00Z" {
		t.Errorf("state valid_from = %q, want 2026-01-01T00:00:00Z", got)
	}
}

// A window endpoint with a non-UTC offset is accepted: RFC3339 carries an
// instant, and query-time comparisons are instant comparisons.
func TestLoadEdgesAcceptsOffsetInstant(t *testing.T) {
	c := newTestClient(t)
	mustLoadNodes(t, c, "Person", []map[string]interface{}{nodeRow("person:ada")})
	mustLoadNodes(t, c, "Team", []map[string]interface{}{nodeRow("team:core")})

	props := edgeProps()
	props["valid_from"] = "2026-01-01T05:30:00+05:30" // == 2026-01-01T00:00:00Z
	if err := c.LoadEdges("WORKS_IN", []EdgeRow{
		{From: "person:ada", To: "team:core", Props: props},
	}); err != nil {
		t.Fatalf("offset valid_from should be accepted: %v", err)
	}
	if n := edgeCount(t, c); n != 1 {
		t.Fatalf("edges = %d, want 1", n)
	}
}

// Every node-batch violation is rejected and writes nothing.
func TestLoadNodesRejectsInvalidBatch(t *testing.T) {
	c := newTestClient(t)

	missingKey := nodeRow("person:ada")
	delete(missingKey, "key")

	dup := []map[string]interface{}{nodeRow("person:ada"), nodeRow("person:ada")}

	cases := []struct {
		name string
		rows []map[string]interface{}
		want string
	}{
		{"missing key", []map[string]interface{}{missingKey}, "missing required field"},
		{"no colon", []map[string]interface{}{nodeRow("nocolon")}, "<src>:<id>"},
		{"empty src", []map[string]interface{}{nodeRow(":id")}, "<src>:<id>"},
		{"empty id", []map[string]interface{}{nodeRow("src:")}, "<src>:<id>"},
		{"duplicate key in batch", dup, "duplicate key in batch"},
	}
	for _, f := range factProvenance {
		row := nodeRow("person:ada")
		delete(row, f)
		cases = append(cases, struct {
			name string
			rows []map[string]interface{}
			want string
		}{"missing " + f, []map[string]interface{}{row}, `"` + f + `"`})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := nodeCount(t, c)
			err := c.LoadNodes("Person", tc.rows)
			if err == nil {
				t.Fatalf("want error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
			if after := nodeCount(t, c); after != before {
				t.Errorf("invalid batch mutated graph: nodes %d -> %d", before, after)
			}
		})
	}
}

// Every edge-batch violation is rejected and writes nothing.
func TestLoadEdgesRejectsInvalidBatch(t *testing.T) {
	c := newTestClient(t)
	mustLoadNodes(t, c, "Person", []map[string]interface{}{nodeRow("person:ada")})
	mustLoadNodes(t, c, "Team", []map[string]interface{}{nodeRow("team:core")})

	stateNoFrom := edgeProps() // WORKS_IN is a state relation; valid_from missing
	eventWithTo := edgeProps()
	eventWithTo["valid_to"] = "2026-01-01T00:00:00Z" // AUTHORED is an event relation
	reversed := edgeProps()
	reversed["valid_from"] = "2026-06-01T00:00:00Z"
	reversed["valid_to"] = "2026-01-01T00:00:00Z"
	// A malformed valid_from with no valid_to must still fail: parsing is
	// per-endpoint, not conditional on valid_to being present.
	badFromAlone := edgeProps()
	badFromAlone["valid_from"] = "2026-01-01"
	badTo := edgeProps()
	badTo["valid_from"] = "2026-01-01T00:00:00Z"
	badTo["valid_to"] = "whenever"

	cases := []struct {
		name  string
		rel   string
		props map[string]interface{}
		want  string
	}{
		{"state edge without valid_from", "WORKS_IN", stateNoFrom, "requires"},
		{"event edge with valid_to", "AUTHORED", eventWithTo, "must not carry"},
		{"valid_to before valid_from", "WORKS_IN", reversed, "before valid_from"},
		{"malformed valid_from without valid_to", "WORKS_IN", badFromAlone, "valid_from"},
		{"malformed valid_to", "WORKS_IN", badTo, "valid_to"},
	}
	for _, f := range factProvenance {
		props := edgeProps()
		props["valid_from"] = "2026-01-01T00:00:00Z"
		delete(props, f)
		cases = append(cases, struct {
			name  string
			rel   string
			props map[string]interface{}
			want  string
		}{"missing " + f, "WORKS_IN", props, `"` + f + `"`})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := edgeCount(t, c)
			err := c.LoadEdges(tc.rel, []EdgeRow{{From: "person:ada", To: "team:core", Props: tc.props}})
			if err == nil {
				t.Fatalf("want error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
			if after := edgeCount(t, c); after != before {
				t.Errorf("invalid batch mutated graph: edges %d -> %d", before, after)
			}
		})
	}
}

// A dangling endpoint is an explicit error, and one bad row fails the
// whole batch — no partial write.
func TestLoadEdgesRejectsMissingEndpoint(t *testing.T) {
	c := newTestClient(t)
	mustLoadNodes(t, c, "Person", []map[string]interface{}{nodeRow("person:ada")})
	mustLoadNodes(t, c, "Team", []map[string]interface{}{nodeRow("team:core")})

	props := edgeProps()
	props["valid_from"] = "2026-01-01T00:00:00Z"

	t.Run("missing source", func(t *testing.T) {
		before := edgeCount(t, c)
		err := c.LoadEdges("WORKS_IN", []EdgeRow{{From: "person:ghost", To: "team:core", Props: props}})
		if err == nil || !strings.Contains(err.Error(), "person:ghost") {
			t.Fatalf("want error naming person:ghost, got %v", err)
		}
		if after := edgeCount(t, c); after != before {
			t.Errorf("mutated graph: edges %d -> %d", before, after)
		}
	})

	t.Run("missing target", func(t *testing.T) {
		before := edgeCount(t, c)
		err := c.LoadEdges("WORKS_IN", []EdgeRow{{From: "person:ada", To: "team:ghost", Props: props}})
		if err == nil || !strings.Contains(err.Error(), "team:ghost") {
			t.Fatalf("want error naming team:ghost, got %v", err)
		}
		if after := edgeCount(t, c); after != before {
			t.Errorf("mutated graph: edges %d -> %d", before, after)
		}
	})

	t.Run("one bad row blocks the whole batch", func(t *testing.T) {
		before := edgeCount(t, c)
		err := c.LoadEdges("WORKS_IN", []EdgeRow{
			{From: "person:ada", To: "team:core", Props: props},
			{From: "person:ada", To: "team:ghost", Props: props},
		})
		if err == nil {
			t.Fatal("want error for batch containing a dangling endpoint")
		}
		if after := edgeCount(t, c); after != before {
			t.Errorf("partial write: batch with one dangling row mutated graph: edges %d -> %d", before, after)
		}
	})
}
