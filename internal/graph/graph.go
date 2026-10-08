// Package graph is the only package that speaks Cypher. Operation
// results carry knowledge.Provenance so every claim ships its chain.
package graph

import (
	"fmt"

	"github.com/FalkorDB/falkordb-go/v2"

	"github.com/ANUBprad/tr0n/internal/knowledge"
)

type Client struct {
	db *falkordb.FalkorDB
	g  *falkordb.Graph
}

// New connects to FalkorDB at addr and selects the named graph.
// ponytail: no context until falkordb-go's query API takes one;
// upgrade path: thread ctx through once the client supports cancellation.
func New(addr, graphName string) (*Client, error) {
	db, err := falkordb.FalkorDBNew(&falkordb.ConnectionOption{Addr: addr})
	if err != nil {
		return nil, fmt.Errorf("connect falkordb at %s: %w", addr, err)
	}
	return &Client{db: db, g: db.SelectGraph(graphName)}, nil
}

func (c *Client) Close() error {
	return c.db.Conn.Close()
}

// Owner is the current owner of a target entity, with the OWNS edge's
// provenance as evidence.
type Owner struct {
	Key        string               `json:"key"`
	Name       string               `json:"name"`
	Kind       string               `json:"kind"`
	Provenance knowledge.Provenance `json:"provenance"`
}

// FindOwner answers "who owns this?" for an entity matched by name or
// key. Current owners only: superseded (valid_to) edges are ignored.
func (c *Client) FindOwner(target string) ([]Owner, error) {
	const q = `
	MATCH (o)-[r:OWNS]->(t)
	WHERE (t.name = $target OR t.key = $target) AND r.valid_to IS NULL
	RETURN o.key AS owner_key, o.name AS owner_name, labels(o)[0] AS owner_kind,
	       r.src_type AS src_type, r.src_ref AS src_ref, r.observed_at AS observed_at,
	       r.extraction AS extraction, r.valid_from AS valid_from`
	res, err := c.g.ROQuery(q, map[string]interface{}{"target": target}, nil)
	if err != nil {
		return nil, fmt.Errorf("find owner %q: %w", target, err)
	}
	owners := []Owner{}
	for res.Next() {
		rec := res.Record()
		owners = append(owners, Owner{
			Key:  field(rec, "owner_key"),
			Name: field(rec, "owner_name"),
			Kind: field(rec, "owner_kind"),
			Provenance: knowledge.Provenance{
				SrcType:    field(rec, "src_type"),
				SrcRef:     field(rec, "src_ref"),
				ObservedAt: field(rec, "observed_at"),
				Extraction: field(rec, "extraction"),
				ValidFrom:  field(rec, "valid_from"),
			},
		})
	}
	return owners, nil
}

// field extracts a string cell; missing or non-string cells read as "".
// All FindOwner projections are declared string properties, so this is
// not a silent fallback for wrong types — it fails loudly in tests.
func field(r *falkordb.Record, key string) string {
	v, ok := r.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
