# TRON — Graph Operation Boundary & Agent Model

**Status**: LOCKED for the graph boundary (2026-10-08); agent layer is
future work. One home for all tool contracts — referenced by
`architecture-minimal.md`.

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

## The operations (initial: 5 + 1 resolver)

All return `{layer, ...}` structures: fact paths carry provenance from
`knowledge-model.md`; computed results carry `rule_id` + basis. All are
single round-trips or a small fixed number of them; depth is bounded in the
query, never variable-length to infinity.

### FindOwner

- **Input**: entity `key` or (`label`, `name`); optional `as_of` timestamp
- **Output**: owner person/team nodes + `OWNS` edges (with provenance and
  validity windows); `current: bool`
- **Traversal**: resolve → `(o)-[:OWNS]->(t)` filtered `valid_to IS NULL`
  (or as-of window)
- **Deterministic logic**: entity resolution (exact key, then exact name,
  then prefix → candidate list), temporal filter
- **Provenance returned**: per-edge `src_*`, `valid_from`/`valid_to`,
  layer = FACT
- **Answers**: "Who owns this?"

### FindExperts

- **Input**: service `key`; `limit` (default 5); injected `now`
- **Output**: ranked persons with `score`, `current_owner: bool`,
  `top_signals` (counts per class), `basis` (the fact paths used),
  `rule_id` (`expertise/v1`), weights version
- **Traversal**: the four signal patterns from `knowledge-model.md`
  (authored PRs, reviewed PRs, resolved incidents, authored docs) + current
  `OWNS`
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

### ResolveEntity (internal resolver, not a user-facing tool)

- **Input**: free-text name + optional label filter
- **Output**: exact match → `key`; multiple matches → candidate list
  (never a guess — the caller must choose)
- **Deterministic logic**: exact `key` → exact `name` → case-insensitive
  exact → prefix. No fuzzy/embedding matching in v1.
- Used *by* the five operations; exposed to the agent only as a candidate
  chooser.

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
