> **Current competition build (2026-10-16):** See `TRACKER.md` and the root README for implemented behavior. The build now includes bounded natural-language routing, inspectable source content, handoff drafts, structured file imports, and sourced ownership updates. General-purpose LLM reasoning, live connectors, and autonomous execution remain future work.

# TRON — Graph Operation Boundary & Agent Model

**Status**: LOCKED for the graph boundary (2026-10-08); agent layer is
future work. One home for all tool contracts — referenced by
`architecture-minimal.md`. Amended 2026-10-15: `FindConflicts` added as a
**PROPOSED** operation (decided expansion of the boundary, see below and
`planning/decision-log.md`); **implemented 2026-10-16** (E6 v1).

## Principle

The LLM never receives the graph, never writes free-form Cypher, and never
scores people. Retrieval is **deliberate**: a small set of deterministic
operations in `internal/graph/` returns bounded subgraphs with provenance.
The future agent layer selects operations and phrases answers; it cannot
create edges, scores, or confidence.

```
DETERMINISTIC (v1 core — plain Go functions, testable without an LLM)
  resolve · traverse · count · decay · temporal filter · evidence assembly

PROBABILISTIC (future agent layer only)
  intent parsing → which operation(s) to call
  answer phrasing / follow-up synthesis
  (every claim must cite operation output: fact paths or rule-tagged inference)
```

v1 core ships **without any LLM** — the operations are a usable JSON API on
their own.

## The operations (initial: 5 + 1 resolver; two decided expansions: FindConflicts, AssembleContext)

All return `{layer, ...}` structures: fact paths carry provenance from
`knowledge-model.md`; computed results carry `rule_id` + basis. All are
single round-trips or a small fixed number of them; depth is bounded in the
query, never variable-length to infinity.

### FindOwner

- **Input (target contract)**: entity `key` or (`label`, `name`); optional
  `as_of` timestamp
- **Output (target contract)**: owner person/team nodes + `OWNS` edges (with
  provenance and validity windows); `current: bool`
- **Traversal**: resolve → `(o)-[:OWNS]->(t)` filtered `valid_to IS NULL`
  (or as-of window)
- **Deterministic logic**: entity resolution (exact key, then exact name,
  then prefix → candidate list), temporal filter
- **Provenance returned**: per-edge `src_*`, `valid_from`/`valid_to`,
  layer = FACT
- **Answers**: "Who owns this?"

**Current implementation** (`internal/graph/graph.go` `FindOwner`): takes a
`target` key or name and an optional `as_of` instant. With no `as_of` it
returns **current owners only** (`OWNS` edges with `valid_to IS NULL`); with
`as_of` it returns owners valid at that instant under the half-open rule
(`valid_from <= as_of AND (valid_to IS NULL OR valid_to > as_of)`), so a
superseded handoff is queryable. Every returned `Owner` carries the edge's
provenance including `valid_from`. `current: bool` is still **not** returned.
IMPLEMENTED 2026-10-16 (`internal/graph` `TestFindOwnerAsOf`, `internal/api`
`TestEndpoints/owner`, `internal/seed` `TestSeedServesFiveQueries/find_owner`).

### FindExperts

- **Input**: service `key`; `limit` (default 5); injected `now`; optional
  `as_of` (ownership is judged at this instant; `now` still drives recency
  decay, so the two are independent)
- **Output**: ranked persons with `score`, `current_owner: bool`,
  `top_signals` (counts per class), `basis` (the fact paths used),
  `rule_id` (`expertise/v1`), weights version
- **Traversal**: the four signal patterns from `knowledge-model.md`
  (authored PRs, reviewed PRs, resolved incidents, authored docs) + `OWNS`
  (current, or valid at `as_of`)
- **Deterministic logic**: weights × recency decay × sort; pure function of
  (graph, clock)
- **Provenance returned**: every basis path is a fact path with edge
  provenance; the ranking itself is tagged **INFERENCE** with `rule_id`
- **Answers**: "Who knows this system best?"

### TraceDecision

- **Input**: decision `key` or `title`
- **Output**: decision node + one bounded neighborhood: `SUPPORTED_BY`
  documents, `DISCUSSED_IN` meetings (+ `PARTICIPATED_IN` people),
  `AUTHORED` people, `SUPERSEDES` chain (≤2 hops), `status`/`decided_at`
- **Traversal**: fixed pattern from `knowledge-model.md` → Decision
  provenance section
- **Deterministic logic**: bounded expansion, no scoring
- **Provenance returned**: all edges; every document/meeting is a source
  reference a human can open
- **Answers**: "Why was this decision made?"

### FindRelatedIncidents

- **Input**: service `key`; optional `since` timestamp; optional `keywords`
  (literal, case-insensitive substring — deterministic only)
- **Output**: incidents affecting the service, ordered by `started_at`
  desc, each with `RESOLVED_BY` people, `severity`, `resolved_at`
- **Traversal**: `(:Incident)-[:AFFECTS]->(service)` + `->[:RESOLVED_BY]->(p)`
- **Deterministic logic**: temporal filter + literal keyword match
  (no semantic/LLM matching in v1)
- **Provenance returned**: incident + edges as facts
- **Answers**: "Has this happened before? Who handled it?"

### TraceEvidence

- **Input**: list of node keys / edge references previously returned by
  another operation
- **Output**: the same paths re-fetched in full + one hop of surrounding
  context, each edge with complete provenance
- **Traversal**: keyed lookups + bounded 1-hop expansion
- **Deterministic logic**: re-fetch + expand; no selection beyond the input
- **Provenance returned**: this operation exists *to* return provenance —
  it is the "why do you believe this?" drill-down
- **Answers**: "What supports this answer?"

### AssembleContext (IMPLEMENTED — M3 context assembly, 2026-10-16)

Boundary note: a **decided** expansion of the locked 5 + 1 boundary (reason in
`planning/decision-log.md`), now **implemented**: `graph.AssembleEvidence`
(`internal/graph/evidence.go`) and the `assemble_context` agent tool. It is the
retrieval half of `REASONING.md` §2 context assembly — the smallest seam that
supplies the reasoning loop with bounded, provenance-bearing facts.

- **Input**: one `entity` (free text, resolved by the canonical tiers) +
  optional `label`, `as_of`, `relations`, `limit`, `max_bytes`
- **Output**: an `EvidenceContext` — the resolved `entity`, a `status`
  (`ok` | `not_found` | `ambiguous` | `empty`), the sorted `facts` (each an edge
  with both endpoint identities, relationship identity, direction, and complete
  provenance incl. `valid_from`/`valid_to`), and explicit `truncated` /
  `truncated_reason`
- **Traversal**: resolve → 1-hop expansion of that entity's edges, optionally
  narrowed to named relations. State relations are filtered to `as_of` under the
  canonical half-open rule; event relations carry no window and are not filtered
  (same treatment as `FindExperts`)
- **Deterministic logic**: store-independent sort; result capped by documented
  count/size limits; truncation reported, never implied
- **Provenance returned**: every fact carries its edge provenance; an ambiguous
  entity returns `candidates` and picks none (`ResolveEntity` contract)
- **Read-only**: writes nothing; never materializes an inference
- **Answers**: "What evidence does this entity have?" (the retrieval step an
  answer is built from)

### ResolveEntity (internal resolver, not a user-facing tool)

- **Input**: free-text name + optional label filter
- **Output**: exact match → `key`; multiple matches → candidate list
  (never a guess — the caller must choose)
- **Deterministic logic**: exact `key` → exact `name` → case-insensitive
  exact → prefix. No fuzzy/embedding matching in v1.
- Used *by* the operations; exposed to the agent only as a candidate
  chooser.

### FindConflicts (IMPLEMENTED — first E6 slice, 2026-10-16)

Boundary note: this is a **decided** expansion of the locked 5 + 1 boundary
(reason in `planning/decision-log.md`; `RULES.md` §3.2 A11), now
**implemented**: `graph.FindConflicts` (`internal/graph/graph.go`), the
read-only `POST /v1/conflicts` handler (`internal/api/api.go`), and
real-FalkorDB tests (`internal/graph/graph_test.go` `TestFindConflicts`,
`internal/api/api_test.go` `TestEndpoints/conflicts`). v1 scans state
relations only; `OWNS` is the sole exclusive relation.

- **Input (target contract)**: optional `relation` (one of the state
  relations); optional `key` (slot or subject key)
- **Output (target contract)**: `{ "layer": "INFERENCE",
  "rule_id": "contradiction/v1", "conflicts": [ { "relation", "slot",
  "edges": [ … ] } ] }`; each conflicting edge carries its full provenance
  (`src_type`, `src_ref`, `observed_at`, `extraction`, `valid_from`,
  `valid_to`); `conflicts: []` when nothing conflicts
- **Traversal**: state-relation edges only (`WORKS_IN, OWNS, WORKED_ON,
  HAS_REPO, DEPENDS_ON`), grouped per relationship and compared on half-open
  windows (`SCHEMA.md` §5–§6). `OWNS` conflicts are different owners of the
  same target with overlapping validity; any other state relationship
  conflicts only if `GRAPH_MODEL.md` declares it exclusive (v1: none); a
  repeated same-counterpart claim is corroboration
- **Deterministic logic**: derived at query time, never stored (A9); stable
  order — groups by `(relation, slot key)`, members by `(from key,
  valid_from, src_ref)`
- **Provenance returned**: every conflicting edge; both/all `src_ref`s
  surface, never a silent pick
- **Answers**: "Where do sources disagree?"
- **Exposed as**: `POST /v1/conflicts` (`APPFLOW.md`), read-only, same
  handler conventions as the other read operations

### Deliberately deferred

- `prepare_handoff` — not a graph primitive; it is a *composition* of
  FindOwner + FindExperts + FindRelatedIncidents + TraceEvidence. Lives in
  the future agent layer as a workflow, not in `internal/graph/`.
- `search_graph` (fuzzy/conceptual search) — superseded by strict
  ResolveEntity; fuzzy search requires an embedding/vector story TRON has
  not committed to.

## Answer structure (target)

```json
{
  "answer": "…",
  "layer": "FACT | INFERENCE | RECOMMENDATION",
  "claims": [
    { "text": "…", "layer": "…",
      "evidence": { "paths": ["…"], "rule_id": "… (inferences only)" } }
  ],
  "operations_used": ["FindOwner", "FindExperts"],
  "latency_ms": { "…": 0 }
}
```

Every claim is individually tagged and individually backed. An untagged,
unevidenced claim must not be renderable — enforced by the response type,
not by reviewer vigilance.

## Why the LLM alone cannot do this

- It cannot reliably traverse 4-hop paths across hundreds of entities
  (recall degrades; hallucinated edges are indistinguishable from real ones
  in its output).
- It has no access to org data at all without retrieval — the graph *is*
  the data.
- Expertise ranking must be reproducible (same graph + same clock → same
  order); model sampling is not.
- Provenance fields (`src_ref`, `observed_at`, `extraction`) are data, not
  language; only a system that reads them can return them.
