package graph

import "fmt"

// Catalog supplies entity names to the bounded question router and picker.
// Facts still come from the graph operations; the catalog cannot answer them.
// ponytail: capped scan for a single company; use indexed search above 5000 entities.
func (c *Client) Catalog() ([]Candidate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, err := c.g.ROQuery(`MATCH (n) WHERE n.key IS NOT NULL
	 RETURN labels(n)[0] AS label, n.key AS key, coalesce(n.name,n.title) AS name
	 ORDER BY n.key LIMIT 5000`, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	out := []Candidate{}
	for r.Next() {
		rec := r.Record()
		out = append(out, Candidate{Label: field(rec, "label"), Key: field(rec, "key"), Name: field(rec, "name")})
	}
	return out, nil
}
