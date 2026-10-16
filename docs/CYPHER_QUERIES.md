# FalkorDB queries used by TRON

TRON sends parameterized Cypher through `internal/graph`; it does not let a model write arbitrary queries. The snippets below show the traversals the product depends on. Result shaping, deterministic sorting, provenance extraction, and `expertise/v1` scoring happen in Go. The executable source is [`../internal/graph/graph.go`](../internal/graph/graph.go); load/index behavior is in [`../internal/graph/load.go`](../internal/graph/load.go).

## Current and historical ownership

The same query handles current state (the operation supplies the actual UTC time) and an `as_of` instant. State windows are half-open: `valid_from <= as_of < valid_to`; a missing `valid_to` is open-ended.

```cypher
MATCH (o)-[r:OWNS]->(t)
WHERE (t.name = $target OR t.key = $target)
  AND r.valid_from <= $as_of
  AND (r.valid_to IS NULL OR r.valid_to > $as_of)
RETURN o.key AS owner_key, o.name AS owner_name, labels(o)[0] AS owner_kind,
       r.src_type AS src_type, r.src_ref AS src_ref, r.observed_at AS observed_at,
       r.extraction AS extraction, r.valid_from AS valid_from, r.valid_to AS valid_to
```

## Expertise: evidence paths to a service

The query unions separate fact classes so each result retains a concrete basis. Representative paths are person → authored PR → modified repository ← service repository; person → resolved incident → affected service; and person → authored document → service. A separate ownership branch flags the current owner. Go then applies the versioned deterministic rule `expertise/v1`, including time decay, and returns each contributor path and edge provenance.

```cypher
MATCH (s:Service {key: $key})-[h:HAS_REPO]->(repo:Repository)<-[m:MODIFIED]-(pr:PullRequest)<-[a:AUTHORED]-(p:Person)
RETURN p.key AS person_key, p.name AS person_name, 'authored_pr' AS class,
       coalesce(pr.created_at, '') AS occurred_at, [p, pr, repo, s] AS nodes, [a, m, h] AS edges
UNION ALL
MATCH (s:Service {key: $key})<-[af:AFFECTS]-(i:Incident)-[rb:RESOLVED_BY]->(p:Person)
RETURN p.key AS person_key, p.name AS person_name, 'resolved_incident' AS class,
       coalesce(i.resolved_at, i.started_at) AS occurred_at, [i, s, p] AS nodes, [af, rb] AS edges
UNION ALL
MATCH (s:Service {key: $key})<-[ab:ABOUT]-(d:Document)<-[ad:AUTHORED]-(p:Person)
RETURN p.key AS person_key, p.name AS person_name, 'authored_doc' AS class,
       coalesce(d.published_at, '') AS occurred_at, [p, d, s] AS nodes, [ad, ab] AS edges
```

The production query in `graph.go` also includes the reviewed-PR branch and the temporal ownership branch. Scoring weights and half-life are specified in [`knowledge-model.md`](knowledge-model.md).

## Decision provenance

Decision lookup is deterministic by key or exact title. After resolving the decision node, TRON runs bounded traversals for its author, discussion meetings and participants, supporting documents, and superseded decisions. Example provenance branches:

```cypher
MATCH (a)-[r:AUTHORED]->(d:Decision {key: $key})
RETURN a.key AS person_key, a.name AS person_name,
       r.src_type AS src_type, r.src_ref AS src_ref, r.observed_at AS observed_at,
       r.extraction AS extraction, r.valid_from AS valid_from, r.valid_to AS valid_to

MATCH (d:Decision {key: $key})-[r:SUPPORTED_BY]->(doc:Document)
RETURN doc.key AS key, doc.title AS title, doc.kind AS kind,
       r.src_type AS src_type, r.src_ref AS src_ref, r.observed_at AS observed_at,
       r.extraction AS extraction, r.valid_from AS valid_from, r.valid_to AS valid_to

MATCH path = (d:Decision {key: $key})-[:SUPERSEDES*1..2]->(x)
RETURN x.key AS key, x.title AS title, relationships(path) AS edges
```

## Incident history

```cypher
MATCH (i:Incident)-[af:AFFECTS]->(s:Service {key: $key})
OPTIONAL MATCH (i)-[rb:RESOLVED_BY]->(p:Person)
RETURN i.key AS key, i.title AS title, i.started_at AS started_at,
       i.resolved_at AS resolved_at, coalesce(p.key, '') AS resolver_key,
       coalesce(p.name, '') AS resolver_name,
       af.src_type AS affects_src_type, af.src_ref AS affects_src_ref,
       af.observed_at AS affects_observed_at, af.extraction AS affects_extraction,
       rb.src_type AS resolver_src_type, rb.src_ref AS resolver_src_ref,
       rb.observed_at AS resolver_observed_at, rb.extraction AS resolver_extraction
```

The Go operation applies optional RFC3339 `since` and literal case-insensitive title-keyword filters, then sorts newest first. The explicit `AFFECTS` and `RESOLVED_BY` paths make the service relationship and prior responders visible.

## Evidence neighborhood and conflict scan

`TraceEvidence` re-fetches requested nodes and their one-hop edges in both directions. Each returned edge includes its stored provenance properties. Conflict detection reads state edges and derives incompatible overlapping claims at query time; only `OWNS` is exclusive in schema v1. Time-window comparison and grouping happen in Go under the named rule `contradiction/v1`, so no inference is stored as a fact.

```cypher
MATCH (n) WHERE n.key IN $keys
RETURN labels(n)[0] AS label, n.key AS key, coalesce(n.name, n.title) AS name,
       properties(n) AS props

MATCH (n)-[r]->(m) WHERE n.key IN $keys
RETURN n.key AS src_key, type(r) AS relation,
       m.key AS neighbor_key, labels(m)[0] AS neighbor_label,
       coalesce(m.name, m.title) AS neighbor_name, properties(r) AS edge_props

MATCH (a)-[r]->(b)
RETURN type(r) AS relation, a.key AS from_key, coalesce(a.name, a.title) AS from_name,
       b.key AS to_key, properties(r) AS edge_props
```

The conflict scan intentionally retrieves state and event edges alike, then filters to the allowlisted state relations in Go. This makes the detector fail closed to its documented semantics; see `FindConflicts` in `graph.go` for interval validation and pairing.

## Seed indexes

`tron seed` resets the demo graph and creates a non-unique index on each node label's `key` before loading nodes and relationships. The schema's uniqueness requirement is not database-enforced yet; the serialized writer now rejects cross-load duplicates across labels. External writers still require database constraints.


## Import and ownership update operations

`ImportRecord` resolves explicit source identities, checks source/observation ordering, and uses one `MERGE` mutation to update a document. Only edges marked with that import key are replaced. `RecordOwner` closes prior open windows and creates the replacement in one mutation. Both are CLI-only writes; the question agent remains read-only. Their complete parameterized queries are in `internal/graph/import.go` and `internal/graph/ownership.go`.

The loader now rejects fractional validity endpoints so canonical string comparison cannot silently truncate a subsecond handoff. Historical owner responses return both `valid_from` and `valid_to`.
