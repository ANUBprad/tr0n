# TRON — Knowledge & Provenance Model

**Status**: LOCKED (2026-10-08). See `planning/decision-log.md`.
This document defines what TRON may claim, how claims are backed, and how
organizational knowledge changes over time.

## The five layers

| Layer | Definition | Where it lives | Example |
|---|---|---|---|
| **SOURCE** | An external artifact observed by ingestion. Immutable; never edited. | Reference on facts (`src_type`, `src_ref`) | GitHub PR #123 |
| **FACT** | Graph state extracted from a source by a deterministic rule. Falsifiable; points at its source. | Nodes + edges in FalkorDB (the graph *is* the fact store) | Person A `-[:AUTHORED]->` PR #123 `-[:MODIFIED]->` Repository R |
| **INFERENCE** | A conclusion computed from facts by a versioned rule. Not observed anywhere. | Computed at query time (v1: never stored); carries `rule_id` + basis paths | Person A has expertise score 12.4 for Service X (`expertise/v1`, basis: 8 PRs + 4 reviews + 2 incidents) |
| **RECOMMENDATION** | Advice for a human, derived from facts + inferences under a policy. | Computed; carries its full chain | "Person A should investigate the Service X incident" (`policy: recommend-top-expert`) |
| **ACTION** (future) | The system doing something on a user's behalf. Must reference the recommendation that authorizes it. | Execution log, not the graph | Prepare an incident handoff for Person A |

### The chain (worked example)

```
SOURCE    GitHub PR #123 (merged 2026-09-14)
  ↓  deterministic extraction
FACT      (A)-[:AUTHORED]->(PR123)-[:MODIFIED]->(RepoR)<-[:HAS_REPO]-(ServiceX)
  ↓  counting those paths + recency decay
SIGNAL    8 PRs, 4 reviews, 2 incidents resolved on ServiceX
  ↓  rule expertise/v1
INFERENCE A is the strongest current expert for ServiceX   [rule_id + basis attached]
  ↓  policy: recommend-top-expert
RECOMMENDATION  Ask A to investigate                      [chain attached]
  ↓  (future)
ACTION    Prepare handoff package for A                   [references recommendation]
```

## Invariants (hard rules)

1. **Only ingestion writes facts.** The API and agent layers are read-only
   with respect to the graph (v1). One writer boundary makes provenance
   auditable.
2. **No silent promotion.** An INFERENCE is never stored or rendered as a
   FACT. A RECOMMENDATION is never rendered as a FACT. The layer tag travels
   with every claim, through API response to UI.
3. **LLM output never becomes a fact.** An LLM may select which deterministic
   operation to run, and may phrase an answer — but every claim it produces
   must cite an existing fact path or a rule-tagged inference. The LLM may
   not create edges, scores, or confidence.
4. **Every claim answers "why".**
   - FACT → source reference (see provenance model below)
   - INFERENCE → rule_id + basis paths + weights version
   - RECOMMENDATION → inference basis + policy id
   If a claim cannot produce its chain, it may not be returned.
5. **Deterministic extraction only in v1.** Sources are structured
   (registries, trackers, PR metadata), parsed by fixed rules with
   confidence = 1.0 implied. Fuzzy/LLM extraction is out of scope until it
   can carry an explicit `confidence` and an `extraction` method — at which
   point the result is a *labeled* fact with uncertainty, or an inference.

## Provenance model (minimum required)

Required properties **on every fact edge**:

| Field | Why it is required |
|---|---|
| `src_type` | Class of origin (`github`, `registry`, `adr`, `calendar`, …) — tells you what kind of evidence this is |
| `src_ref` | Native identifier or URI (e.g. `github:acme/api-server/pull/123`) — lets a human go look. This subsumes a separate `source_uri` field |
| `observed_at` | When TRON ingested it (UTC) — separates "stale ingestion" from "changed world" |
| `extraction` | Rule id + version that produced the edge — auditable, re-runnable |
| `valid_from` / `valid_to` | Temporal validity (see Temporal below); `valid_to = null` means currently valid |

Optional:

| Field | Rule |
|---|---|
| `confidence` | Only when `extraction` is non-deterministic (0..1). Never present in v1, because v1 extraction is deterministic |

Deliberately **not** modeled as provenance properties:

- `author` → modeled as an `AUTHORED` edge (a relationship, queried as
  evidence, not metadata hidden in a property).
- `supporting_relationships` → the walkable path in the graph *is* the
  supporting relationship; storing a copy would go stale.

Node provenance: nodes carry `src_type`, `src_ref`, `observed_at`,
`extraction` from first observation (same rules).

Identity: every node has a `key = "<src>:<native-id>"`. When a second source
refers to the same real-world entity, the canonical node gains `aliases`
(extra source keys) during identity resolution — never a second node.

## Fact vs inference — classification rules

Ask: *would this be true if TRON's rules were wrong?*

- **FACT**: directly observed in a structured source, constant-rule
  extraction. "Person A owns Service X" (registry says so).
- **INFERENCE**: requires aggregation, counting, comparison, scoring, or
  any rule whose parameters could differ. "A is the strongest expert"
  (depends on weights, decay, window).
- **RECOMMENDATION**: prescribes a human action. "Ask A to investigate."
- **Ambiguous** (name matches, NER, semantic similarity): in v1 — do not
  ingest. Later: ingest only with explicit `confidence` + `extraction`,
  displayed as uncertain.

## Expertise model (deterministic, graph-derived)

Answers "Who knows this system best?" **without** an LLM opinion.

For candidate person `P` and service `S`, collect events from fact paths:

| Signal | Path pattern | Weight (v1) |
|---|---|---|
| Ownership (current) | `(P)-[:OWNS]->(S)` with `valid_to IS NULL` | flag: owner (hard signal, listed first) |
| PRs authored | `(P)-[:AUTHORED]->(:PullRequest)-[:MODIFIED]->(:Repository)<-[:HAS_REPO]-(S)` | 1.0 |
| PRs reviewed | `(P)-[:REVIEWED]->(:PullRequest)-[:MODIFIED]->(:Repository)<-[:HAS_REPO]-(S)` | 0.6 |
| Incidents resolved | `(:Incident)-[:AFFECTS]->(S)` + `(:Incident)-[:RESOLVED_BY]->(P)` | 1.5 |
| Documents authored | `(P)-[:AUTHORED]->(:Document)-[:ABOUT]->(S)` | 0.8 |

Recency: each event contributes `weight × 0.5^(age_days / 180)`
(half-life 180 days). Current ownership does not decay.

```
score(P, S) = Σ_events  weight(class) × 0.5^(age_days/180)
```

- Weights live in config, versioned with the rule: `rule_id = expertise/v1`.
- Output: ranked persons with `score`, `top_signals`, and the **basis paths**
  used — tagged **INFERENCE** with rule_id and weights version.
- Ranking is a pure function of (graph, clock). The clock is injected so
  tests are deterministic.
- The LLM (later layer) may explain the ranking; it may never reorder it.

(ponytail: no PageRank/centrality, no per-service materialized scores —
counts across four bounded path patterns are cheap at this scale. Upgrade
path: precomputed score nodes with `rule_id`, still tagged INFERENCE.)

## Decision provenance

Minimum relationships to answer "Why was this decision made?":

```
Decision
├─ (A)-[:AUTHORED]->(Decision)              who wrote it
├─ Decision-[:DISCUSSED_IN]->(Meeting)      where it was argued
│    └─ (P)-[:PARTICIPATED_IN]->(Meeting)   who was in the room
├─ Decision-[:SUPPORTED_BY]->(Document)     what evidence (ADRs, specs)
│    └─ (A)-[:AUTHORED]->(Document)         who wrote the evidence
├─ Decision-[:SUPERSEDES]->(Decision|Document)  what it replaced
└─ Decision.status + decided_at             outcome
```

Alternatives considered are **content inside** the source documents/decision
records (surfaced as evidence), not separate graph edges in v1.
(ponytail: modeling rejected alternatives as nodes needs a schema nobody has
specified yet; upgrade path: `Decision-[:CONSIDERED]->Decision`.)

## Temporal knowledge (simplest correct model)

Problem: Person A owns Service X in 2026; Person B owns it in 2027. Both are
true — *at different times*.

Model — bitemporal-lite, **edges only**:

- Every edge on a **mutating relation** (`WORKS_IN`, `OWNS`, `WORKED_ON`,
  `DEPENDS_ON`) carries `valid_from` (UTC) and `valid_to` (UTC or `null`).
- **Current state**: `valid_to IS NULL`. This is the default filter for all
  "who owns / who is in / depends on" questions.
- **Point-in-time**: `valid_from <= $t AND (valid_to IS NULL OR valid_to > $t)`.
- **Update = close + open**, never rewrite: ownership change sets
  `valid_to` on the old edge and inserts a new one. Facts are append-only;
  the old statement remains true about its era.
- Non-mutating relations (`AUTHORED`, `REVIEWED`, `MODIFIED`, `AFFECTS`,
  `RESOLVED_BY`, …) describe immutable events — their time lives on the event
  node (`merged_at`, `started_at`, `held_at`), so no windows are needed.

Out of scope (deliberately): temporal databases, valid-time vs
transaction-time systems, versioned nodes.
(ponytail: single UTC clock, single-writer assumption, time on edges only —
upgrade path: add a `recorded_at` column if we ever need transaction-time
auditing separate from `observed_at`.)
