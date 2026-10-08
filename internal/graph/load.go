package graph

import (
	"fmt"
	"strings"
)

// EdgeRow is one edge for the bulk loader: endpoints by key plus edge
// properties (provenance). Absent optional properties are simply not
// set — "missing" and null are the same read (IS NULL) at query time.
type EdgeRow struct {
	From  string                 `json:"from"`
	To    string                 `json:"to"`
	Props map[string]interface{} `json:"props"`
}

// The locked schema (docs/GRAPH_MODEL.md): 11 labels, 16 relationship
// types. Loaders refuse anything else, so an unknown label/type is an
// error instead of a schema drift.
var nodeLabels = []string{
	"Person", "Team", "Project", "Repository", "Service", "Document",
	"Decision", "Meeting", "Incident", "PullRequest", "Technology",
}

var edgeTypes = []string{
	"WORKS_IN", "OWNS", "WORKED_ON", "AUTHORED", "REVIEWED", "MODIFIED",
	"HAS_REPO", "DEPENDS_ON", "AFFECTS", "RESOLVED_BY", "SUPPORTED_BY",
	"DISCUSSED_IN", "PARTICIPATED_IN", "SUPERSEDES", "ABOUT", "USES",
}

func allowed(set []string, s string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}

// Ping verifies the graph answers — the health endpoint's check.
func (c *Client) Ping() error {
	_, err := c.g.ROQuery("RETURN 1", nil, nil)
	return err
}

// Delete removes the whole graph — test cleanup and the reset path.
func (c *Client) Delete() error {
	return c.g.Delete()
}

// Reset wipes the graph and recreates the key indexes — the bulk
// loader's precondition. Destructive by design: seeding is
// recreate-only, never incremental.
// ponytail: missing graph surfaces as the server's "empty key" delete
// error and is treated as already wiped; upgrade path = GRAPH.LIST
// pre-check if FalkorDB ever changes that error text.
func (c *Client) Reset() error {
	if err := c.g.Delete(); err != nil && !strings.Contains(err.Error(), "empty key") {
		return fmt.Errorf("wipe graph: %w", err)
	}
	for _, l := range nodeLabels {
		q := fmt.Sprintf("CREATE INDEX FOR (n:%s) ON (n.key)", l)
		if _, err := c.g.Query(q, nil, nil); err != nil {
			return fmt.Errorf("create key index on %s: %w", l, err)
		}
	}
	return nil
}

// LoadNodes creates one node per row under the given label; each row
// must carry a unique `key` plus node provenance.
func (c *Client) LoadNodes(label string, rows []map[string]interface{}) error {
	if !allowed(nodeLabels, label) {
		return fmt.Errorf("unknown node label %q", label)
	}
	if len(rows) == 0 {
		return nil
	}
	// falkordb-go's param encoder type-switches exactly: convert to the
	// shapes it knows ([]interface{} of map/string) before sending.
	payload := make([]interface{}, len(rows))
	for i, r := range rows {
		payload[i] = r
	}
	q := fmt.Sprintf("UNWIND $rows AS r CREATE (n:%s) SET n = r", label)
	if _, err := c.g.Query(q, map[string]interface{}{"rows": payload}, nil); err != nil {
		return fmt.Errorf("load %d %s nodes: %w", len(rows), label, err)
	}
	return nil
}

// LoadEdges creates one typed edge per row between existing keys,
// with the row's properties (provenance) attached.
func (c *Client) LoadEdges(rel string, rows []EdgeRow) error {
	if !allowed(edgeTypes, rel) {
		return fmt.Errorf("unknown relationship type %q", rel)
	}
	if len(rows) == 0 {
		return nil
	}
	payload := make([]interface{}, len(rows))
	for i, r := range rows {
		payload[i] = map[string]interface{}{"from": r.From, "to": r.To, "props": r.Props}
	}
	q := fmt.Sprintf(`
	UNWIND $rows AS r
	MATCH (a {key: r.from}), (b {key: r.to})
	CREATE (a)-[e:%s]->(b) SET e = r.props`, rel)
	if _, err := c.g.Query(q, map[string]interface{}{"rows": payload}, nil); err != nil {
		return fmt.Errorf("load %d %s edges: %w", len(rows), rel, err)
	}
	return nil
}
