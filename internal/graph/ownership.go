package graph

import (
	"fmt"
	"strings"
	"time"
)

// RecordOwner applies one explicitly sourced registry update. A single Cypher
// mutation closes open ownership and creates its replacement, retaining history.
// The CLI is the only writer surface; this is never an autonomous agent action.
func (c *Client) RecordOwner(serviceKey, ownerKey, sourceRef string, at time.Time) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !keyShapeOK(serviceKey) || !keyShapeOK(ownerKey) || strings.TrimSpace(sourceRef) == "" || len(sourceRef) > 1000 {
		return false, fmt.Errorf("service key, owner key and source reference are required")
	}
	if at.IsZero() {
		at = time.Now()
	}
	at = at.UTC().Truncate(time.Second)
	params := map[string]interface{}{"service": serviceKey, "owner": ownerKey, "at": at.Format(time.RFC3339)}
	r, err := c.g.ROQuery(`MATCH (s:Service {key:$service}), (o {key:$owner})
	 WHERE o:Person OR o:Team RETURN s.key AS service,o.key AS owner`, params, nil)
	if err != nil {
		return false, err
	}
	n := 0
	for r.Next() {
		n++
	}
	if n != 1 {
		return false, fmt.Errorf("service and owner must resolve to exactly one Service and one Person/Team")
	}
	r, err = c.g.ROQuery(`MATCH (o)-[r:OWNS]->(:Service {key:$service})
	 RETURN o.key AS owner,r.valid_from AS valid_from,r.valid_to AS valid_to`, params, nil)
	if err != nil {
		return false, err
	}
	open := []string{}
	equalStart := false
	for r.Next() {
		rec := r.Record()
		start, e := time.Parse(time.RFC3339, field(rec, "valid_from"))
		if e != nil {
			return false, fmt.Errorf("existing ownership has invalid valid_from")
		}
		end := field(rec, "valid_to")
		if end != "" {
			to, e := time.Parse(time.RFC3339, end)
			if e != nil || to.After(at) {
				return false, fmt.Errorf("cannot rewrite historical or scheduled ownership")
			}
			continue
		}
		if start.After(at) {
			return false, fmt.Errorf("new ownership time must follow the existing window start")
		}
		if start.Equal(at) {
			equalStart = true
		}
		open = append(open, field(rec, "owner"))
	}
	if len(open) == 1 && open[0] == ownerKey {
		return false, nil
	}
	if equalStart {
		return false, fmt.Errorf("ownership change must follow the existing window start")
	}
	params["props"] = map[string]interface{}{"src_type": "service_registry", "src_ref": sourceRef, "observed_at": at.Format(time.RFC3339), "extraction": "deterministic", "valid_from": at.Format(time.RFC3339)}
	_, err = c.g.Query(`MATCH (s:Service {key:$service}), (o {key:$owner})
	 OPTIONAL MATCH ()-[r:OWNS]->(s) WHERE r.valid_to IS NULL
	 WITH s,o,collect(r) AS old
	 FOREACH (edge IN old | SET edge.valid_to=$at)
	 CREATE (o)-[next:OWNS]->(s) SET next=$props`, params, nil)
	if err != nil {
		return false, fmt.Errorf("record ownership: %w", err)
	}
	return true, nil
}
