package graph

import (
	"fmt"
	"strings"
	"time"
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

// The fact-write contract (docs/knowledge-model.md, docs/GRAPH_MODEL.md,
// docs/SCHEMA.md §4.1). stateRelations are the mutating relations that
// must carry a valid_from window; every other type is an event/evidence
// edge (time lives on the event node) and must not carry valid_to.
var stateRelations = []string{"WORKS_IN", "OWNS", "WORKED_ON", "HAS_REPO", "DEPENDS_ON"}

// IsStateRelation reports whether rel carries a valid-time window
// (docs/SCHEMA.md §4.1) — the only relations E6 contradiction detection
// inspects, and the only valid `relation` filter for FindConflicts.
func IsStateRelation(rel string) bool { return allowed(stateRelations, rel) }

// factProvenance is the minimum required provenance on every fact node
// and edge. valid_from/valid_to are conditional on relation kind and are
// validated separately.
var factProvenance = []string{"src_type", "src_ref", "observed_at", "extraction"}

// strCell reads a string property; an absent or non-string cell reads "".
func strCell(props map[string]interface{}, k string) string {
	s, _ := props[k].(string)
	return s
}

// keyShapeOK reports whether key is a non-empty "<src>:<native-id>" with
// both sides non-empty (docs/knowledge-model.md identity). The native id
// may itself contain colons (e.g. "github:acme/api/pull/123").
func keyShapeOK(key string) bool {
	i := strings.Index(key, ":")
	return i > 0 && i < len(key)-1
}

// validateNode enforces the node half of the fact-write contract: key
// present and "<src>:<id>" shaped, no duplicate key in the batch, and
// the minimum provenance present. seen accumulates keys across the batch.
func validateNode(label string, row map[string]interface{}, seen map[string]bool) error {
	key := strCell(row, "key")
	if key == "" {
		return fmt.Errorf("%s node: missing required field %q", label, "key")
	}
	if !keyShapeOK(key) {
		return fmt.Errorf("%s node %q: key must be %q", label, key, "<src>:<id>")
	}
	if seen[key] {
		return fmt.Errorf("%s node %q: duplicate key in batch", label, key)
	}
	seen[key] = true
	for _, f := range factProvenance {
		if strCell(row, f) == "" {
			return fmt.Errorf("%s node %q: missing required provenance field %q", label, key, f)
		}
	}
	return nil
}

// validateEdge enforces the edge half of the fact-write contract:
// endpoints present, minimum provenance, state relations carry a
// parseable valid_from, event/evidence relations carry no valid_to, and
// valid_to is strictly after valid_from — a half-open window
// [valid_from, valid_to) is empty when the two are equal or reversed.
func validateEdge(rel string, r EdgeRow) error {
	if r.From == "" || r.To == "" {
		return fmt.Errorf("%s edge: missing endpoint (from=%q to=%q)", rel, r.From, r.To)
	}
	for _, f := range factProvenance {
		if strCell(r.Props, f) == "" {
			return fmt.Errorf("%s edge %s->%s: missing required provenance field %q", rel, r.From, r.To, f)
		}
	}
	validFrom := strCell(r.Props, "valid_from")
	validTo := strCell(r.Props, "valid_to")
	if allowed(stateRelations, rel) && validFrom == "" {
		return fmt.Errorf("%s edge %s->%s: state relation requires %q", rel, r.From, r.To, "valid_from")
	}
	if !allowed(stateRelations, rel) && validTo != "" {
		return fmt.Errorf("%s edge %s->%s: event/evidence relation must not carry %q", rel, r.From, r.To, "valid_to")
	}
	// Each window endpoint present is parsed on its own (a malformed
	// valid_from must fail even when valid_to is absent, or query code
	// would misread the window) and then rewritten to canonical RFC3339
	// UTC. FalkorDB has no datetime() function (verified), so as-of
	// predicates compare windows as strings; only canonical same-format
	// strings order lexically the same as chronologically. Normalizing on
	// write keeps that true for any offset the source sends
	// (e.g. 05:30+05:30 → 00:00Z).
	var vf, vt time.Time
	if validFrom != "" {
		t, err := time.Parse(time.RFC3339, validFrom)
		if err != nil {
			return fmt.Errorf("%s edge %s->%s: valid_from %q is not RFC3339: %w", rel, r.From, r.To, validFrom, err)
		}
		vf = t
		r.Props["valid_from"] = t.UTC().Format(time.RFC3339)
	}
	if validTo != "" {
		t, err := time.Parse(time.RFC3339, validTo)
		if err != nil {
			return fmt.Errorf("%s edge %s->%s: valid_to %q is not RFC3339: %w", rel, r.From, r.To, validTo, err)
		}
		vt = t
		r.Props["valid_to"] = t.UTC().Format(time.RFC3339)
	}
	// Compare the parsed instants, not the raw strings: two equivalent
	// instants spelled with different RFC3339 offsets (05:30+05:30 and
	// 00:00Z) are the same moment and so form an empty window. Half-open
	// [valid_from, valid_to) must be non-empty: equal endpoints never
	// match an as-of instant.
	if validFrom != "" && validTo != "" && !vt.After(vf) {
		return fmt.Errorf("%s edge %s->%s: valid_to %s is not after valid_from %s", rel, r.From, r.To, validTo, validFrom)
	}
	return nil
}

// existingKeys returns the subset of keys already present in the graph,
// sorted, so LoadNodes can enforce key uniqueness across loads and labels
// (CREATE alone would duplicate a key loaded earlier).
func (c *Client) existingKeys(keys []string) ([]string, error) {
	res, err := c.g.ROQuery(
		`MATCH (n) WHERE n.key IN $keys RETURN n.key AS key ORDER BY n.key`,
		map[string]interface{}{"keys": keys}, nil)
	if err != nil {
		// A graph that does not exist yet has no keys; FalkorDB reports
		// reads against it as "empty key", same as Reset/countScalar.
		if strings.Contains(err.Error(), "empty key") {
			return nil, nil
		}
		return nil, err
	}
	var found []string
	for res.Next() {
		found = append(found, field(res.Record(), "key"))
	}
	return found, nil
}

// missingEndpoints returns the from->to of every row whose source or
// target key does not exist. It runs before any create, so a batch with
// a dangling endpoint never mutates the graph.
func (c *Client) missingEndpoints(payload []interface{}) ([]string, error) {
	const q = `
	UNWIND $rows AS r
	OPTIONAL MATCH (a {key: r.from})
	OPTIONAL MATCH (b {key: r.to})
	WITH r, a, b
	WHERE a IS NULL OR b IS NULL
	RETURN r.from AS src, r.to AS dst`
	res, err := c.g.ROQuery(q, map[string]interface{}{"rows": payload}, nil)
	if err != nil {
		return nil, err
	}
	var missing []string
	for res.Next() {
		rec := res.Record()
		missing = append(missing, field(rec, "src")+"->"+field(rec, "dst"))
	}
	return missing, nil
}

// Ping verifies the graph answers — the health endpoint's check.
func (c *Client) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.g.ROQuery("RETURN 1", nil, nil)
	return err
}

// Delete removes the whole graph — test cleanup and the reset path.
func (c *Client) Delete() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.g.Delete()
}

// Reset wipes the graph and recreates the key indexes — the bulk
// loader's precondition. Destructive by design: seeding is
// recreate-only, never incremental.
// ponytail: missing graph surfaces as the server's "empty key" delete
// error and is treated as already wiped; upgrade path = GRAPH.LIST
// pre-check if FalkorDB ever changes that error text.
func (c *Client) Reset() error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if !allowed(nodeLabels, label) {
		return fmt.Errorf("unknown node label %q", label)
	}
	if len(rows) == 0 {
		return nil
	}
	// Validate the whole batch before any write, so an invalid batch
	// never leaves a partial load behind.
	seen := make(map[string]bool, len(rows))
	for i, r := range rows {
		if err := validateNode(label, r, seen); err != nil {
			return fmt.Errorf("load %s nodes: row %d: %w", label, i, err)
		}
	}
	// Reject keys already present in the graph. The in-batch `seen` check
	// above only catches duplicates within one batch; CREATE would still
	// silently add a *second* node for a key loaded earlier, and both edge
	// endpoints and identity resolution match by key alone, so a duplicate
	// would split (or cross-product) them. Uniqueness is therefore enforced
	// across loads and labels here, not just within a batch
	// (docs/OPEN_QUESTIONS.md, decision-log 2026-10-15 + follow-up).
	// ponytail: check-then-create under the client mutex; the loader is
	// single-writer (seeding is recreate-only), so there is no race window
	// in v1 — upgrade path = a DB uniqueness constraint once FalkorDB
	// proves one.
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = strCell(r, "key")
	}
	if existing, err := c.existingKeys(keys); err != nil {
		return fmt.Errorf("check %s keys: %w", label, err)
	} else if len(existing) > 0 {
		return fmt.Errorf("load %s nodes: key(s) already present: %s",
			label, strings.Join(existing, ", "))
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if !allowed(edgeTypes, rel) {
		return fmt.Errorf("unknown relationship type %q", rel)
	}
	if len(rows) == 0 {
		return nil
	}
	// Validate the whole batch before any write, so an invalid batch
	// never leaves a partial load behind.
	for i, r := range rows {
		if err := validateEdge(rel, r); err != nil {
			return fmt.Errorf("load %s edges: row %d: %w", rel, i, err)
		}
	}
	payload := make([]interface{}, len(rows))
	for i, r := range rows {
		payload[i] = map[string]interface{}{"from": r.From, "to": r.To, "props": r.Props}
	}
	// Both endpoints must already exist; a dangling edge is an explicit
	// error, never a silent drop. Checked before the create.
	if missing, err := c.missingEndpoints(payload); err != nil {
		return fmt.Errorf("check %s edge endpoints: %w", rel, err)
	} else if len(missing) > 0 {
		return fmt.Errorf("load %s edges: %d row(s) reference a missing endpoint: %s",
			rel, len(missing), strings.Join(missing, ", "))
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
