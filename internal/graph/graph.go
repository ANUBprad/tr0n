// Package graph is the only package that speaks Cypher. Operation
// results carry knowledge.Provenance so every claim ships its chain.
package graph

import (
	"fmt"
	"sort"
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
			Key:        field(rec, "owner_key"),
			Name:       field(rec, "owner_name"),
			Kind:       field(rec, "owner_kind"),
			Provenance: recProvenance(rec),
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

// DecisionTrace is the bounded provenance neighborhood of one decision
// (AGENT_SPEC TraceDecision). Source references a human can open.
type DecisionTrace struct {
	Decision    Decision        `json:"decision"`
	AuthoredBy  []SourcedPerson `json:"authored_by"`
	DiscussedIn []MeetingRef    `json:"discussed_in"`
	SupportedBy []DocumentRef   `json:"supported_by"`
	Supersedes  []SupersedesRef `json:"supersedes"`
}

type Decision struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	DecidedAt string `json:"decided_at,omitempty"`
}

type PersonRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// SourcedPerson is a person reached via an edge whose provenance must
// ship too (AGENT_SPEC: "Provenance returned: all edges").
type SourcedPerson struct {
	PersonRef
	Provenance knowledge.Provenance `json:"provenance"`
}

type DocumentRef struct {
	Key        string               `json:"key"`
	Title      string               `json:"title"`
	Kind       string               `json:"kind"`
	URL        string               `json:"url,omitempty"`
	Provenance knowledge.Provenance `json:"provenance"`
}

type MeetingRef struct {
	Key          string               `json:"key"`
	Title        string               `json:"title"`
	HeldAt       string               `json:"held_at,omitempty"`
	Participants []SourcedPerson      `json:"participants"`
	Provenance   knowledge.Provenance `json:"provenance"`
}

type SupersedesRef struct {
	Key      string                 `json:"key"`
	Title    string                 `json:"title"`
	Evidence []knowledge.Provenance `json:"evidence"`
}

// TraceDecision answers "why was this decision made?" — decision node,
// authors, meetings (+participants), supporting documents, and the
// SUPERSEDES chain (≤2 hops). Bounded expansion, no scoring. Lookup by
// key or title; nil when nothing matches.
// ponytail: title ambiguity resolves by key order (first match);
// upgrade path: identity resolution aliases once multiple sources
// exist.
func (c *Client) TraceDecision(ref string) (*DecisionTrace, error) {
	const resolve = `
	MATCH (d:Decision)
	WHERE d.key = $q OR d.title = $q
	RETURN d.key AS key, d.title AS title, d.status AS status,
	       coalesce(d.decided_at, '') AS decided_at
	ORDER BY d.key LIMIT 1`
	res, err := c.g.ROQuery(resolve, map[string]interface{}{"q": ref}, nil)
	if err != nil {
		return nil, fmt.Errorf("resolve decision %q: %w", ref, err)
	}
	if !res.Next() {
		return nil, nil
	}
	rec := res.Record()
	key := field(rec, "key")
	trace := &DecisionTrace{
		Decision: Decision{
			Key:       key,
			Title:     field(rec, "title"),
			Status:    field(rec, "status"),
			DecidedAt: field(rec, "decided_at"),
		},
		AuthoredBy:  []SourcedPerson{},
		DiscussedIn: []MeetingRef{},
		SupportedBy: []DocumentRef{},
		Supersedes:  []SupersedesRef{},
	}

	provCols := `, r.src_type AS src_type, r.src_ref AS src_ref, r.observed_at AS observed_at,
	             r.extraction AS extraction, r.valid_from AS valid_from`

	run := func(q string) (*falkordb.QueryResult, error) {
		return c.g.ROQuery(q, map[string]interface{}{"key": key}, nil)
	}
	if res, err := run(`
	MATCH (a)-[r:AUTHORED]->(d:Decision {key: $key})
	RETURN a.key AS person_key, a.name AS person_name` + provCols); err != nil {
		return nil, fmt.Errorf("decision authors: %w", err)
	} else {
		for res.Next() {
			rec := res.Record()
			trace.AuthoredBy = append(trace.AuthoredBy, SourcedPerson{
				PersonRef:  PersonRef{Key: field(rec, "person_key"), Name: field(rec, "person_name")},
				Provenance: recProvenance(rec),
			})
		}
	}

	if res, err := run(`
	MATCH (d:Decision {key: $key})-[r:DISCUSSED_IN]->(m:Meeting)
	RETURN m.key AS key, m.title AS title, coalesce(m.held_at, '') AS held_at` + provCols); err != nil {
		return nil, fmt.Errorf("decision meetings: %w", err)
	} else {
		for res.Next() {
			rec := res.Record()
			trace.DiscussedIn = append(trace.DiscussedIn, MeetingRef{
				Key: field(rec, "key"), Title: field(rec, "title"), HeldAt: field(rec, "held_at"),
				Participants: []SourcedPerson{}, Provenance: recProvenance(rec)})
		}
	}
	if res, err := run(`
	MATCH (d:Decision {key: $key})-[:DISCUSSED_IN]->(m:Meeting)<-[r:PARTICIPATED_IN]-(p:Person)
	RETURN m.key AS meeting_key, p.key AS person_key, p.name AS person_name` + provCols); err != nil {
		return nil, fmt.Errorf("decision participants: %w", err)
	} else {
		idx := map[string]int{}
		for i, m := range trace.DiscussedIn {
			idx[m.Key] = i
		}
		for res.Next() {
			rec := res.Record()
			if i, ok := idx[field(rec, "meeting_key")]; ok {
				m := &trace.DiscussedIn[i]
				m.Participants = append(m.Participants, SourcedPerson{
					PersonRef:  PersonRef{Key: field(rec, "person_key"), Name: field(rec, "person_name")},
					Provenance: recProvenance(rec),
				})
			}
		}
	}

	if res, err := run(`
	MATCH (d:Decision {key: $key})-[r:SUPPORTED_BY]->(doc:Document)
	RETURN doc.key AS key, doc.title AS title, doc.kind AS kind, coalesce(doc.url, '') AS url` + provCols); err != nil {
		return nil, fmt.Errorf("decision evidence: %w", err)
	} else {
		for res.Next() {
			rec := res.Record()
			trace.SupportedBy = append(trace.SupportedBy, DocumentRef{
				Key: field(rec, "key"), Title: field(rec, "title"), Kind: field(rec, "kind"),
				URL: field(rec, "url"), Provenance: recProvenance(rec)})
		}
	}

	if res, err := run(`
	MATCH path = (d:Decision {key: $key})-[:SUPERSEDES*1..2]->(x)
	RETURN x.key AS key, x.title AS title, relationships(path) AS edges`); err != nil {
		return nil, fmt.Errorf("decision supersedes: %w", err)
	} else {
		for res.Next() {
			rec := res.Record()
			s := SupersedesRef{Key: field(rec, "key"), Title: field(rec, "title")}
			for _, e := range edgeList(rec) {
				s.Evidence = append(s.Evidence, edgeProvenance(e))
			}
			trace.Supersedes = append(trace.Supersedes, s)
		}
	}

	sortByKey(trace.AuthoredBy, func(p SourcedPerson) string { return p.Key })
	sortByKey(trace.DiscussedIn, func(m MeetingRef) string { return m.Key })
	sortByKey(trace.SupportedBy, func(d DocumentRef) string { return d.Key })
	sortByKey(trace.Supersedes, func(s SupersedesRef) string { return s.Key })
	for i := range trace.DiscussedIn {
		sortByKey(trace.DiscussedIn[i].Participants, func(p SourcedPerson) string { return p.Key })
	}
	return trace, nil
}

// recProvenance assembles the provenance columns a query returned.
func recProvenance(rec *falkordb.Record) knowledge.Provenance {
	return knowledge.Provenance{
		SrcType:    field(rec, "src_type"),
		SrcRef:     field(rec, "src_ref"),
		ObservedAt: field(rec, "observed_at"),
		Extraction: field(rec, "extraction"),
		ValidFrom:  field(rec, "valid_from"),
	}
}

func sortByKey[T any](s []T, key func(T) string) {
	sort.Slice(s, func(i, j int) bool { return key(s[i]) < key(s[j]) })
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
