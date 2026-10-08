// Package graph is the only package that speaks Cypher. Operation
// results carry knowledge.Provenance so every claim ships its chain.
package graph

import (
	"fmt"
	"time"

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

// expertsQuery returns one row per fact-path event. Branch node/edge
// order is positional — basisPath decodes it:
//
//	owner:            nodes [owner, service],        edges [OWNS]
//	authored/reviewed nodes [person, pr, repo, svc], edges [AUTHORED|REVIEWED, MODIFIED, HAS_REPO]
//	resolved_incident nodes [incident, service, p],  edges [AFFECTS, RESOLVED_BY]
//	authored_doc:     nodes [person, doc, service],  edges [AUTHORED, ABOUT]
const expertsQuery = `
	MATCH (s:Service {key: $key})-[h:HAS_REPO]->(repo:Repository)<-[m:MODIFIED]-(pr:PullRequest)<-[a:AUTHORED]-(p:Person)
	RETURN p.key AS person_key, p.name AS person_name, 'authored_pr' AS class,
	       coalesce(pr.created_at, '') AS occurred_at, [p, pr, repo, s] AS nodes, [a, m, h] AS edges
	UNION ALL
	MATCH (s:Service {key: $key})-[h:HAS_REPO]->(repo:Repository)<-[m:MODIFIED]-(pr:PullRequest)<-[rv:REVIEWED]-(p:Person)
	RETURN p.key AS person_key, p.name AS person_name, 'reviewed_pr' AS class,
	       coalesce(pr.created_at, '') AS occurred_at, [p, pr, repo, s] AS nodes, [rv, m, h] AS edges
	UNION ALL
	MATCH (s:Service {key: $key})<-[af:AFFECTS]-(i:Incident)-[rb:RESOLVED_BY]->(p:Person)
	RETURN p.key AS person_key, p.name AS person_name, 'resolved_incident' AS class,
	       coalesce(i.resolved_at, i.started_at) AS occurred_at, [i, s, p] AS nodes, [af, rb] AS edges
	UNION ALL
	MATCH (s:Service {key: $key})<-[ab:ABOUT]-(d:Document)<-[ad:AUTHORED]-(p:Person)
	RETURN p.key AS person_key, p.name AS person_name, 'authored_doc' AS class,
	       coalesce(d.published_at, '') AS occurred_at, [p, d, s] AS nodes, [ad, ab] AS edges
	UNION ALL
	MATCH (o)-[ow:OWNS]->(s:Service {key: $key})
	WHERE ow.valid_to IS NULL
	RETURN o.key AS person_key, o.name AS person_name, 'owner' AS class,
	       '' AS occurred_at, [o, s] AS nodes, [ow] AS edges`

// FindExperts answers "who knows this system best?" for one service:
// deterministic expertise/v1 scoring over fact paths, ranked with
// current owners first. Pure function of (graph, now).
func (c *Client) FindExperts(serviceKey string, limit int, now time.Time) (*knowledge.ExpertiseResult, error) {
	res, err := c.g.ROQuery(expertsQuery, map[string]interface{}{"key": serviceKey}, nil)
	if err != nil {
		return nil, fmt.Errorf("find experts %q: %w", serviceKey, err)
	}
	events := []knowledge.ExpertiseEvent{}
	for res.Next() {
		rec := res.Record()
		ev := knowledge.ExpertiseEvent{
			PersonKey:  field(rec, "person_key"),
			PersonName: field(rec, "person_name"),
			Class:      field(rec, "class"),
			Path:       basisPath(field(rec, "class"), nodeList(rec)),
		}
		if s := field(rec, "occurred_at"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				ev.OccurredAt = t
			}
		}
		for _, e := range edgeList(rec) {
			ev.Evidence = append(ev.Evidence, edgeProvenance(e))
		}
		events = append(events, ev)
	}
	return knowledge.RankExpertise(events, limit, now), nil
}

// basisPath renders one concrete fact path from a branch's positional
// nodes; the count guard returns "" so a shape change fails tests
// loudly instead of fabricating evidence.
func basisPath(class string, nodes []*falkordb.Node) string {
	need := map[string]int{
		knowledge.OwnerClass: 2,
		"authored_pr":        4,
		"reviewed_pr":        4,
		"resolved_incident":  3,
		"authored_doc":       3,
	}[class]
	if len(nodes) < need {
		return ""
	}
	switch class {
	case knowledge.OwnerClass:
		return nref(nodes[0]) + "-[:OWNS]->" + nref(nodes[1])
	case "authored_pr", "reviewed_pr":
		rel := "AUTHORED"
		if class == "reviewed_pr" {
			rel = "REVIEWED"
		}
		return nref(nodes[0]) + "-[:" + rel + "]->" + nref(nodes[1]) +
			"-[:MODIFIED]->" + nref(nodes[2]) + "<-[:HAS_REPO]-" + nref(nodes[3])
	case "resolved_incident":
		return nref(nodes[0]) + "-[:AFFECTS]->" + nref(nodes[1]) +
			", " + nref(nodes[0]) + "-[:RESOLVED_BY]->" + nref(nodes[2])
	case "authored_doc":
		return nref(nodes[0]) + "-[:AUTHORED]->" + nref(nodes[1]) + "-[:ABOUT]->" + nref(nodes[2])
	}
	return ""
}

func nref(n *falkordb.Node) string {
	label := ""
	if len(n.Labels) > 0 {
		label = n.Labels[0] + " "
	}
	k, _ := n.Properties["key"].(string)
	return fmt.Sprintf("(:%s%s)", label, k)
}

// field extracts a string cell; missing or non-string cells read as "".
// Projections are declared string properties; fixtures pin the types,
// so a wrong type surfaces as a failed assertion, not a silent pass.
func field(r *falkordb.Record, key string) string {
	v, ok := r.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func nodeList(r *falkordb.Record) []*falkordb.Node {
	v, _ := r.Get("nodes")
	raw, _ := v.([]interface{})
	nodes := make([]*falkordb.Node, 0, len(raw))
	for _, item := range raw {
		if n, ok := item.(*falkordb.Node); ok {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

func edgeList(r *falkordb.Record) []*falkordb.Edge {
	v, _ := r.Get("edges")
	raw, _ := v.([]interface{})
	edges := make([]*falkordb.Edge, 0, len(raw))
	for _, item := range raw {
		if e, ok := item.(*falkordb.Edge); ok {
			edges = append(edges, e)
		}
	}
	return edges
}

func edgeProvenance(e *falkordb.Edge) knowledge.Provenance {
	p := e.Properties
	str := func(k string) string {
		s, _ := p[k].(string)
		return s
	}
	return knowledge.Provenance{
		SrcType:    str("src_type"),
		SrcRef:     str("src_ref"),
		ObservedAt: str("observed_at"),
		Extraction: str("extraction"),
		ValidFrom:  str("valid_from"),
		ValidTo:    str("valid_to"),
	}
}
