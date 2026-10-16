// Package graph is the only package that speaks Cypher. Operation
// results carry knowledge.Provenance so every claim ships its chain.
package graph

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FalkorDB/falkordb-go/v2"

	"github.com/ANUBprad/tr0n/internal/knowledge"
)

// Client wraps one connection pool and one graph handle. The handle's
// schema cache (labels/relations/properties) is written without
// synchronization while results are parsed, so a shared handle needs
// mutual exclusion. The pool underneath is already concurrency-safe.
// ponytail: one global lock serializes every graph access — a
// deliberate ceiling for the local demo; upgrade path is a handle per
// operation via db.SelectGraph, which shares the same pool, then drop
// the lock.
type Client struct {
	mu sync.Mutex
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
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

// IncidentRef is one incident affecting a service, with resolvers and
// the AFFECTS edge's provenance (AGENT_SPEC FindRelatedIncidents).
type IncidentRef struct {
	Key        string               `json:"key"`
	Title      string               `json:"title"`
	Severity   string               `json:"severity,omitempty"`
	StartedAt  string               `json:"started_at"`
	ResolvedAt string               `json:"resolved_at,omitempty"`
	ResolvedBy []SourcedPerson      `json:"resolved_by"`
	Provenance knowledge.Provenance `json:"provenance"`
}

// FindRelatedIncidents answers "what happened here before?" —
// incidents affecting the service, newest first. since is an optional
// RFC3339 lower bound on started_at; keyword is an optional literal
// case-insensitive substring of the title. Both filters are
// deterministic: no fuzzy matching, ever.
func (c *Client) FindRelatedIncidents(serviceKey, since, keyword string) ([]IncidentRef, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var sinceT time.Time
	if since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return nil, fmt.Errorf("since must be RFC3339 (got %q): %w", since, err)
		}
		sinceT = t
	}
	const q = `
	MATCH (i:Incident)-[af:AFFECTS]->(s:Service {key: $key})
	OPTIONAL MATCH (i)-[rb:RESOLVED_BY]->(p:Person)
	RETURN i.key AS key, i.title AS title, coalesce(i.severity, '') AS severity,
	       i.started_at AS started_at, coalesce(i.resolved_at, '') AS resolved_at,
	       af.src_type AS affects_src_type, af.src_ref AS affects_src_ref,
	       af.observed_at AS affects_observed_at, af.extraction AS affects_extraction,
	       af.valid_from AS affects_valid_from,
	       coalesce(p.key, '') AS resolver_key, coalesce(p.name, '') AS resolver_name,
	       rb.src_type AS resolver_src_type, rb.src_ref AS resolver_src_ref,
	       rb.observed_at AS resolver_observed_at, rb.extraction AS resolver_extraction,
	       rb.valid_from AS resolver_valid_from`
	res, err := c.g.ROQuery(q, map[string]interface{}{"key": serviceKey}, nil)
	if err != nil {
		return nil, fmt.Errorf("related incidents %q: %w", serviceKey, err)
	}

	type item struct {
		inc     IncidentRef
		started time.Time
	}
	var items []item
	idx := map[string]int{}
	for res.Next() {
		rec := res.Record()
		k := field(rec, "key")
		i, ok := idx[k]
		if !ok {
			inc := IncidentRef{
				Key:        k,
				Title:      field(rec, "title"),
				Severity:   field(rec, "severity"),
				StartedAt:  field(rec, "started_at"),
				ResolvedAt: field(rec, "resolved_at"),
				ResolvedBy: []SourcedPerson{},
				Provenance: provFrom(rec, "affects_"),
			}
			// required at ingest; zero on corrupt rows (see filter below)
			started, _ := time.Parse(time.RFC3339, inc.StartedAt)
			items = append(items, item{inc: inc, started: started})
			idx[k] = len(items) - 1
			i = len(items) - 1
		}
		if rk := field(rec, "resolver_key"); rk != "" {
			items[i].inc.ResolvedBy = append(items[i].inc.ResolvedBy, SourcedPerson{
				PersonRef:  PersonRef{Key: rk, Name: field(rec, "resolver_name")},
				Provenance: provFrom(rec, "resolver_"),
			})
		}
	}

	kept := make([]item, 0, len(items))
	for _, it := range items {
		// an undated row cannot satisfy a since filter — excluded only then
		if !sinceT.IsZero() && (it.started.IsZero() || it.started.Before(sinceT)) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(it.inc.Title), strings.ToLower(keyword)) {
			continue
		}
		kept = append(kept, it)
	}
	sort.Slice(kept, func(a, b int) bool {
		if !kept[a].started.Equal(kept[b].started) {
			return kept[a].started.After(kept[b].started)
		}
		return kept[a].inc.Key < kept[b].inc.Key
	})

	out := make([]IncidentRef, 0, len(kept))
	for _, it := range kept {
		sortByKey(it.inc.ResolvedBy, func(p SourcedPerson) string { return p.Key })
		out = append(out, it.inc)
	}
	return out, nil
}

// Conflict is one group of state-relation edges whose claims are
// incompatible over overlapping half-open validity (rule contradiction/v1).
type Conflict struct {
	Relation string         `json:"relation"`
	Slot     string         `json:"slot"`
	Edges    []ConflictEdge `json:"edges"`
}

// ConflictEdge is one edge participating in a conflict: the claimant
// (subject) and the slot it claims, plus the edge's full provenance so
// every disagreeing source surfaces and none is silently picked.
type ConflictEdge struct {
	From       string               `json:"from"`
	FromName   string               `json:"from_name,omitempty"`
	To         string               `json:"to"`
	Provenance knowledge.Provenance `json:"provenance"`
}

// ConflictResult is the INFERENCE envelope every computed result uses
// (AGENT_SPEC FindConflicts): layer + rule_id + the conflict groups.
type ConflictResult struct {
	Layer     string     `json:"layer"`
	RuleID    string     `json:"rule_id"`
	Conflicts []Conflict `json:"conflicts"`
}

// exclusiveRelations declare a single-valued slot — one edge per target
// per era. v1: only OWNS (docs/GRAPH_MODEL.md Exclusivity). A state
// relation absent here can never contradict: a subject may legitimately
// reach several targets, and a repeated same-counterpart claim is
// corroboration, not contradiction.
var exclusiveRelations = map[string]bool{"OWNS": true}

// FindConflicts derives contradiction/v1 at query time (never stored,
// A9): for each exclusive state relation, edges of the same target whose
// half-open windows overlap and whose claimants differ. An empty relation
// scans all state relations; a non-exclusive relation yields no conflicts
// by design. key, when set, keeps only groups on that slot or involving
// that claimant — always the whole group, so no disagreeing source is
// hidden. Order is stable — groups by (relation, slot), members by
// (from, valid_from, src_ref).
// ponytail: O(n²) pairing within one slot's edges — fine at demo scale;
// upgrade path = sort-by-start sweep if a slot ever holds many claims.
func (c *Client) FindConflicts(relation, key string) (*ConflictResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if relation != "" && !IsStateRelation(relation) {
		return nil, fmt.Errorf("relation %q is not a state relation", relation)
	}
	res, err := c.g.ROQuery(`
	MATCH (a)-[r]->(b)
	RETURN type(r) AS relation, a.key AS from_key, coalesce(a.name, a.title) AS from_name,
	       b.key AS to_key,
	       r.src_type AS src_type, r.src_ref AS src_ref, r.observed_at AS observed_at,
	       r.extraction AS extraction, r.valid_from AS valid_from, r.valid_to AS valid_to`,
		nil, nil)
	if err != nil {
		return nil, fmt.Errorf("find conflicts: %w", err)
	}

	type claim struct {
		from  string
		name  string
		prov  knowledge.Provenance
		fromT time.Time
		toT   time.Time
	}
	// One group per (relation, slot). Slots are the target key — the
	// single-valued side for an exclusive relation.
	type group struct {
		rel, slot string
		claims    []claim
	}
	byGroup := map[string]int{}
	var groups []group

	for res.Next() {
		rec := res.Record()
		rel := field(rec, "relation")
		if relation != "" {
			if rel != relation {
				continue
			}
		} else if !IsStateRelation(rel) {
			continue
		}
		prov := recProvenance(rec)
		// A state edge whose window cannot be parsed is an integrity
		// anomaly: omitted from detection, never treated as always-valid.
		fromT, err := time.Parse(time.RFC3339, prov.ValidFrom)
		if err != nil {
			continue
		}
		var toT time.Time
		if prov.ValidTo != "" {
			toT, err = time.Parse(time.RFC3339, prov.ValidTo)
			if err != nil {
				continue
			}
		}
		cl := claim{from: field(rec, "from_key"), name: field(rec, "from_name"), prov: prov, fromT: fromT, toT: toT}
		slot := field(rec, "to_key")
		gk := rel + "\x00" + slot
		gi, ok := byGroup[gk]
		if !ok {
			gi = len(groups)
			byGroup[gk] = gi
			groups = append(groups, group{rel: rel, slot: slot})
		}
		groups[gi].claims = append(groups[gi].claims, cl)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].rel != groups[j].rel {
			return groups[i].rel < groups[j].rel
		}
		return groups[i].slot < groups[j].slot
	})

	out := &ConflictResult{Layer: "INFERENCE", RuleID: "contradiction/v1", Conflicts: []Conflict{}}
	for _, g := range groups {
		if !exclusiveRelations[g.rel] {
			continue
		}
		marked := make([]bool, len(g.claims))
		for i := 0; i < len(g.claims); i++ {
			for j := i + 1; j < len(g.claims); j++ {
				if g.claims[i].from == g.claims[j].from {
					continue // same counterpart = corroboration
				}
				if windowsOverlap(g.claims[i].fromT, g.claims[i].toT, g.claims[j].fromT, g.claims[j].toT) {
					marked[i], marked[j] = true, true
				}
			}
		}
		var edges []ConflictEdge
		for i, cl := range g.claims {
			if marked[i] {
				edges = append(edges, ConflictEdge{From: cl.from, FromName: cl.name, To: g.slot, Provenance: cl.prov})
			}
		}
		if len(edges) == 0 {
			continue
		}
		if key != "" && g.slot != key && !groupHasClaimant(edges, key) {
			continue
		}
		sort.Slice(edges, func(a, b int) bool {
			if edges[a].From != edges[b].From {
				return edges[a].From < edges[b].From
			}
			if edges[a].Provenance.ValidFrom != edges[b].Provenance.ValidFrom {
				return edges[a].Provenance.ValidFrom < edges[b].Provenance.ValidFrom
			}
			return edges[a].Provenance.SrcRef < edges[b].Provenance.SrcRef
		})
		out.Conflicts = append(out.Conflicts, Conflict{Relation: g.rel, Slot: g.slot, Edges: edges})
	}
	return out, nil
}

// windowsOverlap reports whether half-open intervals [from, to) overlap.
// A zero `to` is open-ended (still current). Intervals that only touch at
// an endpoint (aEnd == bStart) do not overlap.
func windowsOverlap(aFrom, aTo, bFrom, bTo time.Time) bool {
	if !aTo.IsZero() && !bFrom.Before(aTo) {
		return false
	}
	if !bTo.IsZero() && !aFrom.Before(bTo) {
		return false
	}
	return true
}

func groupHasClaimant(edges []ConflictEdge, key string) bool {
	for _, e := range edges {
		if e.From == key {
			return true
		}
	}
	return false
}

// NodeEvidence is one input key re-fetched in full plus one hop of
// surrounding context (AGENT_SPEC TraceEvidence — the "why do you
// believe this?" drill-down).
type NodeEvidence struct {
	Key        string                 `json:"key"`
	Label      string                 `json:"label"`
	Name       string                 `json:"name,omitempty"`
	Found      bool                   `json:"found"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Edges      []EdgeEvidence         `json:"edges"`
}

type EdgeEvidence struct {
	Relation   string               `json:"relation"`
	Direction  string               `json:"direction"` // from the input key's view
	Neighbor   NodeRef              `json:"neighbor"`
	Provenance knowledge.Provenance `json:"provenance"`
}

type NodeRef struct {
	Label string `json:"label"`
	Key   string `json:"key"`
	Name  string `json:"name,omitempty"`
}

// Resolution is the answer to ResolveEntity: one key when the match is
// unambiguous, otherwise a candidate list — never a guess.
type Resolution struct {
	Key        string      `json:"key,omitempty"`
	MatchedOn  string      `json:"matched_on,omitempty"`
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Label string `json:"label"`
	Key   string `json:"key"`
	Name  string `json:"name"`
}

// ResolveEntity is the internal deterministic resolver: exact key →
// exact name → case-insensitive exact name → case-insensitive name
// prefix. First tier with a hit wins; a hit within a tier is either a
// single key or the full candidate list. label narrows the search to
// one node label ("" = any). No fuzzy matching, ever.
// ponytail: one scan of label/key/name per call, fine for the v1
// synthetic graph; upgrade path = per-tier indexed lookups.
func (c *Client) ResolveEntity(name, label string) (Resolution, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	empty := Resolution{Candidates: []Candidate{}}
	if name == "" {
		return empty, nil
	}
	res, err := c.g.ROQuery(`
	MATCH (n) WHERE n.key IS NOT NULL AND ($label = '' OR $label IN labels(n))
	RETURN labels(n)[0] AS label, n.key AS key, coalesce(n.name, n.title) AS name`,
		map[string]interface{}{"label": label}, nil)
	if err != nil {
		return empty, fmt.Errorf("resolve %q: %w", name, err)
	}
	var all []Candidate
	for res.Next() {
		rec := res.Record()
		all = append(all, Candidate{
			Label: field(rec, "label"), Key: field(rec, "key"), Name: field(rec, "name"),
		})
	}

	lower := strings.ToLower(name)
	tiers := []struct {
		on   string
		test func(Candidate) bool
	}{
		{"key", func(c Candidate) bool { return c.Key == name }},
		{"name", func(c Candidate) bool { return c.Name == name }},
		{"name_ci", func(c Candidate) bool { return strings.EqualFold(c.Name, name) }},
		{"name_prefix", func(c Candidate) bool {
			return strings.HasPrefix(strings.ToLower(c.Name), lower)
		}},
	}
	for _, tier := range tiers {
		var hits []Candidate
		for _, cand := range all {
			if tier.test(cand) {
				hits = append(hits, cand)
			}
		}
		if len(hits) == 0 {
			continue
		}
		sortByKey(hits, func(c Candidate) string { return c.Key })
		if len(hits) == 1 {
			return Resolution{Key: hits[0].Key, MatchedOn: tier.on, Candidates: []Candidate{}}, nil
		}
		return Resolution{MatchedOn: tier.on, Candidates: hits}, nil
	}
	return empty, nil
}

// TraceEvidence re-fetches previously returned keys with full node
// properties and every adjacent edge (both directions) with complete
// provenance. Order follows the input; unknown keys come back with
// found=false instead of silently vanishing.
// ponytail: 3 queries total regardless of input size; upgrade path =
// single batched query if drill-down latency is ever measured.
func (c *Client) TraceEvidence(keys []string) ([]NodeEvidence, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]NodeEvidence, 0, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	byKey := map[string]*NodeEvidence{}

	res, err := c.g.ROQuery(`
	MATCH (n) WHERE n.key IN $keys
	RETURN labels(n)[0] AS label, n.key AS key, coalesce(n.name, n.title) AS name,
	       properties(n) AS props`,
		map[string]interface{}{"keys": keys}, nil)
	if err != nil {
		return nil, fmt.Errorf("evidence nodes: %w", err)
	}
	for res.Next() {
		rec := res.Record()
		e := &NodeEvidence{
			Key: field(rec, "key"), Label: field(rec, "label"),
			Name: field(rec, "name"), Found: true,
			Edges: []EdgeEvidence{},
		}
		if props, ok := rec.Get("props"); ok {
			if m, ok := props.(map[string]interface{}); ok {
				e.Properties = m
			}
		}
		byKey[e.Key] = e
	}

	edgeQuery := func(q, dir string) error {
		res, err := c.g.ROQuery(q, map[string]interface{}{"keys": keys}, nil)
		if err != nil {
			return err
		}
		for res.Next() {
			rec := res.Record()
			src := field(rec, "src_key")
			e, ok := byKey[src]
			if !ok {
				continue
			}
			e.Edges = append(e.Edges, EdgeEvidence{
				Relation:   field(rec, "relation"),
				Direction:  dir,
				Neighbor:   NodeRef{Label: field(rec, "neighbor_label"), Key: field(rec, "neighbor_key"), Name: field(rec, "neighbor_name")},
				Provenance: provEdgeProps(rec),
			})
		}
		return nil
	}
	if err := edgeQuery(`
	MATCH (n)-[r]->(m) WHERE n.key IN $keys
	RETURN n.key AS src_key, type(r) AS relation,
	       m.key AS neighbor_key, labels(m)[0] AS neighbor_label,
	       coalesce(m.name, m.title) AS neighbor_name,
	       properties(r) AS edge_props`, "out"); err != nil {
		return nil, fmt.Errorf("evidence out-edges: %w", err)
	}
	if err := edgeQuery(`
	MATCH (m)-[r]->(n) WHERE n.key IN $keys
	RETURN n.key AS src_key, type(r) AS relation,
	       m.key AS neighbor_key, labels(m)[0] AS neighbor_label,
	       coalesce(m.name, m.title) AS neighbor_name,
	       properties(r) AS edge_props`, "in"); err != nil {
		return nil, fmt.Errorf("evidence in-edges: %w", err)
	}

	for _, k := range keys {
		if e, ok := byKey[k]; ok {
			sortEvidenceEdges(e.Edges)
			out = append(out, *e)
			continue
		}
		out = append(out, NodeEvidence{Key: k, Found: false, Edges: []EdgeEvidence{}})
	}
	return out, nil
}

// provEdgeProps builds Provenance from a properties(r) map — complete
// edge provenance in one column.
func provEdgeProps(rec *falkordb.Record) knowledge.Provenance {
	m, _ := rec.Get("edge_props")
	props, _ := m.(map[string]interface{})
	str := func(k string) string {
		s, _ := props[k].(string)
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

func sortEvidenceEdges(edges []EdgeEvidence) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Direction != edges[j].Direction {
			return edges[i].Direction == "out"
		}
		if edges[i].Relation != edges[j].Relation {
			return edges[i].Relation < edges[j].Relation
		}
		return edges[i].Neighbor.Key < edges[j].Neighbor.Key
	})
}

// recProvenance assembles the provenance columns a query returned.
func recProvenance(rec *falkordb.Record) knowledge.Provenance {
	return provFrom(rec, "")
}

// provFrom reads provenance columns under an alias prefix
// ("" for `src_type`, "affects_" for `affects_src_type`).
func provFrom(rec *falkordb.Record, prefix string) knowledge.Provenance {
	return knowledge.Provenance{
		SrcType:    field(rec, prefix+"src_type"),
		SrcRef:     field(rec, prefix+"src_ref"),
		ObservedAt: field(rec, prefix+"observed_at"),
		Extraction: field(rec, prefix+"extraction"),
		ValidFrom:  field(rec, prefix+"valid_from"),
		ValidTo:    field(rec, prefix+"valid_to"),
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
