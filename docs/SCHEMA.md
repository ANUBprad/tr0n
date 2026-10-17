# TRON — Canonical Organizational Knowledge Model (SCHEMA)

**Status**: Canonical. Authority for the **target** knowledge model.
**Last updated**: 2026-10-15.
**Current-schema authority**: [`GRAPH_MODEL.md`](GRAPH_MODEL.md) (locked: 11
entities / 16 relationships) and [`knowledge-model.md`](knowledge-model.md)
(locked: layers, provenance, temporal rules). This document specifies the
**full target model** and marks what exists today.
**Related**: [`PRD.md`](PRD.md), [`TECHSPEC.md`](TECHSPEC.md),
[`INGESTION.md`](INGESTION.md), [`REASONING.md`](REASONING.md),
[`SECURITY.md`](SECURITY.md).

> Nothing in §3–§7 (beyond the "current" column notes) describes existing
> structures. The graph today implements only `GRAPH_MODEL.md`.

---

## 1. What belongs where

TRON has four homes for information. Deciding placement is a design act, not
an implementation detail.

| Home | Holds | Authority | Current |
|---|---|---|---|
| **Knowledge graph (FalkorDB)** | Entities, typed relationships, layer tags, provenance properties, temporal windows | TRON (derived from sources) | IMPLEMENTED (11/16 subset) |
| **Document store** | Immutable original source artifacts and versions (files, message bodies, meeting notes) | The source | PROPOSED |
| **Retrieval index** | Optional lexical/semantic index over documents and graph text for candidate recall | Derived, rebuildable | PROPOSED |
| **External systems of record** | GitHub, HRIS, Jira, CRM, calendar, finance — the authoritative originals | The external system | External |

**Rule**: the graph stores *facts distilled from sources*, never the full
source. The originals live in the document store (or remain in the external
system), referenced by `src_ref` (`knowledge-model.md`). The graph is not a
mirror of every system; it is the connective layer over them.

**Rule**: anything expensive to recompute and cheap to store may be
materialized, but only if still tagged with its `rule_id` and layer. Today the
policy is the opposite: inferences are computed at query time and never
stored (`knowledge-model.md`, decision-log).

## 2. Knowledge layers (canonical)

Nine distinguishable states of a claim, extending the five locked layers with
the finer states the PRD requires. Each is a **tag**, not a separate table.

| State | Meaning | Evidence required | Stored? |
|---|---|---|---|
| **SOURCE** | An observed external artifact | its own identity (`src_ref`) | document store / external |
| **FACT** | Deterministic extraction from a source | `src_type`, `src_ref`, `observed_at`, `extraction` | yes (graph) |
| **REPORTED CLAIM** | A statement made by a source but not verified (e.g. "the vendor says X") | `src_ref` + `confidence` + `extraction` | yes (graph, flagged) |
| **INFERENCE** | Computed from facts by a versioned rule | `rule_id` + basis paths | computed, not stored |
| **PREDICTION** | A forward-looking model output with uncertainty | `rule_id` + model version + interval | computed |
| **RECOMMENDATION** | Advice for a human under a policy | inference basis + `policy_id` | computed (or logged) |
| **APPROVED DECISION** | A recommendation a human authorized | approver identity + scope + timestamp | yes (log + graph) |
| **ACTION** | A requested effect on an external system | references the approving decision | execution log |
| **VERIFIED OUTCOME** | The observed result of an executed action | `src_ref` of the effect + verifier | yes (graph, as a fact) |

**Invariant, unchanged from `knowledge-model.md`**: no silent promotion. A
higher level never renders as a lower one. `internal/knowledge/answer.go`
enforces the FACT/INFERENCE/RECOMMENDATION subset structurally today.

**Contradictions are not a layer.** When two sources disagree, both facts
coexist and the contradiction is *derived and surfaced*, never silently
collapsed (see §6).

## 3. Entity model (target)

Grouped by domain. **Current = exists in `GRAPH_MODEL.md` today.**

### 3.1 People & organization
| Entity | Key identity | Notable properties | Status |
|---|---|---|---|
| **Person** | `src:native-id` | name, email, aliases[] | Current |
| **Team** | `src:name` | name, parent team | Current |
| **Role** | `src:id` | title, level | PROPOSED |
| **Skill** | `src:name` | taxonomy, category | PROPOSED |
| **Organization / BusinessUnit** | `src:id` | name, region | PROPOSED (single-org now) |

### 3.2 Work & commitments
| Entity | Key identity | Notable properties | Status |
|---|---|---|---|
| **Project** | `src:id` | name, status | Current |
| **Goal / Objective** | `src:id` | statement, target, horizon | PROPOSED |
| **Milestone** | `src:id` | due_at, status | PROPOSED |
| **Task** | `src:id` | state, assignee, due_at | PROPOSED |
| **Commitment** | `src:id` | what, who, due_at, status | PROPOSED |

### 3.3 Knowledge artifacts
| Entity | Key identity | Notable properties | Status |
|---|---|---|---|
| **Document** | `src:id` | title, kind, url, published_at | Current |
| **Message / Chat** | `src:id` | body, channel, sent_at | PROPOSED |
| **Meeting** | `src:id` | title, held_at | Current |
| **Decision** | `src:id` | title, status, decided_at | Current |

### 3.4 Delivery & operations
| Entity | Key identity | Notable properties | Status |
|---|---|---|---|
| **Repository** | `src:id` | name, url | Current |
| **Service** | `src:id` | name, status | Current |
| **PullRequest** | `src:id` | number, state, created_at, merged_at | Current |
| **Incident** | `src:id` | title, severity, started_at, resolved_at | Current |
| **Risk** | `src:id` | description, likelihood, impact, owner | PROPOSED |
| **Policy** | `src:id` | rule text, scope | PROPOSED |

### 3.5 Business
| Entity | Key identity | Notable properties | Status |
|---|---|---|---|
| **Customer** | `src:id` | name, segment | PROPOSED |
| **Contract** | `src:id` | value, term, parties | PROPOSED |
| **Product** | `src:id` | name | PROPOSED |
| **Technology** | `src:name` | name | Current |

### 3.6 Measurement
| Entity | Key identity | Notable properties | Status |
|---|---|---|---|
| **Metric / KPI** | `src:id` | name, unit, definition | PROPOSED |
| **Observation** | `src:id` | metric, value, unit, period, observed_at | PROPOSED |
| **Budget** | `src:id` | scope, amount, currency, period | PROPOSED |
| **FinancialRecord** | `src:id` | amount, currency, category, period | PROPOSED |

### 3.7 Control (the action loop)
| Entity | Key identity | Notable properties | Status |
|---|---|---|---|
| **ProposedAction** | generated id | type, target, payload, rationale | PROPOSED |
| **Authorization** | generated id | actor, scope, decision, at | PROPOSED |
| **Execution** | generated id | action ref, idempotency key, state, attempts | PROPOSED |
| **Outcome** | generated id | action ref, verified result, `src_ref` | PROPOSED |

**Metric vs Observation is deliberate**: a KPI is a definition (unit,
formula); each number is a time-stamped observation. Never store a KPI as a
bare number — units and periods are part of the fact (an FR-4 requirement).

## 4. Relationship semantics (target)

All relationships are **explicit facts** extracted by deterministic rules, or
**derived** (never stored). Four kinds:

- **State edges** (mutating): carry `valid_from`/`valid_to`. Current = `valid_to IS NULL`.
- **Event edges** (immutable): time lives on the event node (`merged_at`, `started_at`).
- **Evidence edges**: tie an artifact to its subject/support.
- **Derived links**: computed at query time (expertise, contradiction) — never stored as facts.

### 4.1 Current edges (from `GRAPH_MODEL.md`)
`WORKS_IN, OWNS, WORKED_ON` (state) · `AUTHORED, REVIEWED, MODIFIED, AFFECTS,
RESOLVED_BY, SUPPORTED_BY, DISCUSSED_IN, PARTICIPATED_IN, SUPERSEDES, ABOUT,
USES` (event/evidence) · plus `HAS_REPO, DEPENDS_ON` (state).

### 4.2 Proposed edges
| Relationship | From → To | Kind | Notes |
|---|---|---|---|
| `HAS_ROLE` | Person → Role | state | time-windowed |
| `HAS_SKILL` | Person → Skill | state | must carry evidence (self-declared vs demonstrated) |
| `REPORTS_TO` | Person → Person | state | authority/chain |
| `MEMBER_OF`* | Person → Team | state | re-adds a cut edge; justified by org hierarchy, supersedes `WORKS_IN` role |
| `ACCOUNTABLE_FOR` | Person → Goal\|Commitment | state | distinct from `OWNS` (delivery vs system) |
| `CONTRIBUTES_TO` | Team\|Project → Goal | state | goal traceability |
| `DEPENDS_ON` | Task\|Milestone → Task\|Milestone | state | extend existing edge to work items |
| `BLOCKS` | Task\|Risk → Task\|Goal | state | |
| `COMMITTED` | Person → Commitment | state | who owes what |
| `MEASURES` | Metric → Service\|Project\|Team | state | |
| `GOVERNS` | Policy → Entity | state | |
| `PARTY_TO` | Organization\|Customer → Contract | state | |
| `RISK_OF` | Risk → Entity | state | |
| `APPROVES` | Person → Decision\|ProposedAction | event | |
| `PRECEDES` / `SUPERSEDES` | Decision\|Document → same | event | extend to all artifacts |

\* `MEMBER_OF` was cut in the locked schema as a duplicate of `WORKS_IN`. It
returns here **only** when roles/hierarchy are modeled; record the reversal
in [`RULES.md`](RULES.md) before implementing.

**Still forbidden**: `KNOWS` (expertise is computed, not stored),
`RELATED_TO` (unfalsifiable), any edge that asserts an inference as a fact.

## 5. Temporal model (valid-time + recorded-time)

Locked model (`knowledge-model.md`): bitemporal-lite, edges only.

- **Valid time**: `valid_from`/`valid_to` on state edges. Answers "true when".
- **Observed time**: `observed_at` on every node/edge. Answers "when did TRON
  learn this".
- **Recorded time** (PROPOSED, upgrade path): a `recorded_at` column to
  separate "when the world changed" from "when we recorded it", once
  transaction-time auditing is needed.

Rules:
- **Update = close + open.** Never rewrite a fact. Set `valid_to` on the old
  edge, insert a new edge (`knowledge-model.md`).
- **As-of queries**: `valid_from <= $t AND (valid_to IS NULL OR valid_to > $t)`.
  IMPLEMENTED (2026-10-16) as an optional `as_of` on `FindOwner`/`FindExperts`
  and `POST /v1/owner`/`/v1/experts`; omitted `as_of` = current state. FalkorDB
  has no `datetime()`, so windows are compared as strings, and the loader
  normalizes every endpoint to canonical RFC3339 UTC on write
  (`internal/graph/load.go` `validateEdge`) so the string order matches the
  chronological order.
- **Intervals are half-open `[valid_from, valid_to)`.** An instant equal to
  `valid_to` belongs to the next interval, so intervals that only touch at an
  endpoint (`valid_to == next valid_from`) do **not** overlap — a close+open
  handoff is never an overlap.
- **Missing `valid_from` on a state edge is an integrity anomaly**, not an
  infinitely old claim; the loader rejects it at write
  (`internal/graph/load.go` `validateEdge`), and query code must surface or
  omit such rows, never treat them as always valid.
- **An empty window is rejected.** When both endpoints are present, `valid_to`
  must be strictly after `valid_from` (compared as instants, so two equal
  moments written with different offsets are caught); the half-open predicate
  matches nothing otherwise (`internal/graph/load.go` `validateEdge`).
- **Staleness is observed-time, not valid-time.** Freshness is computed from
  `observed_at` ("when TRON learned it"); a `valid_to` in the past means the
  fact is *historically expired for that era*, not that the source is stale.
  No universal stale threshold is defined — surfacing must report the raw
  `observed_at`/age, not a stale/not-stale label, until a policy is explicitly
  decided (`planning/decision-log.md`, 2026-10-16).
- Event edges need no window; use the event timestamp.

## 6. Lifecycle: change, staleness, contradiction, correction, deletion

| Situation | Handling |
|---|---|
| **Owner changes** | close old `OWNS` (`valid_to`), open new. Both remain true for their era. |
| **Fact goes stale** | Nothing is deleted. Freshness is computed from `observed_at`; retrieval prefers recent facts but can surface old ones with age. |
| **Two sources contradict** | Store both facts. A contradiction is *derived* at query time (§4), never stored: two **state**-relation edges of the same type whose validity windows overlap under the half-open rule (§5) and whose claims are *incompatible* for that relationship. Compatibility is relationship-specific: `OWNS` is exclusive per target — two different owners of the same target over overlapping validity conflict (`GRAPH_MODEL.md` Exclusivity); no other v1 state relationship is exclusive, and a repeated same-counterpart claim is corroboration, not contradiction. Event edges and node-property disagreements are out of scope for E6 v1. Surface it with both `src_ref`s; never silently pick. |
| **Duplicate identity** | One canonical node; extra source keys become `aliases`. Never a second node (`knowledge-model.md`). Ambiguous match → candidate list, never a guess (`internal/graph` ResolveEntity). |
| **Source corrects a fact** | New fact with a new `observed_at`; old edge closed. The correction is visible as history. |
| **Deletion / retraction from source** | Mark affected facts via `valid_to` or a `retracted_at`, propagate per [`INGESTION.md`](INGESTION.md); preserve the audit trail. |
| **Source access revoked** | Permission propagation removes derived visibility: answers must no longer surface those facts ([`SECURITY.md`](SECURITY.md)). |

## 7. Identity, provenance, and naming (unchanged, restated)

- **Identity**: `key = "<src>:<native-id>"`. Unique per label with a database
  constraint installed by `tron constraints`; application writers additionally
  reject cross-label duplicates (`LoadNodes` checks before writing). Direct
  external writers must preserve the cross-label invariant.
- **Provenance minimum on every fact**: `src_type`, `src_ref`, `observed_at`,
  `extraction`, `valid_from`/`valid_to` (state edges). `confidence` only when
  extraction is non-deterministic (`knowledge-model.md`).
- **Name/title** indexed per label for resolution.
- **Derived links** always carry `rule_id` + basis, tagged INFERENCE.

## 8. Permission considerations

- Every fact inherits the source's **access scope** and **retention**.
- A fact derived from a source the asker cannot see must not be surfaced,
  even indirectly, through an answer or an aggregate (`SECURITY.md`,
  FR-2.6). This is a *derived-data leakage* risk, not just a query filter.
- Tenant boundary: one FalkorDB graph per tenant, chosen at connect time
  (`architecture-minimal.md`); not yet exercised.

## 9. Entity resolution contract (target)

Deterministic tiers, in order (`internal/graph` ResolveEntity today):
exact `key` → exact `name`/`title` → case-insensitive exact → prefix. Then,
PROPOSED for multi-source: alias lookup, then optional scored match
(blocking + similarity) that **always returns candidates with confidence** and
never auto-merges above a human-review threshold. Merges are recorded as an
auditable `MERGED_INTO` event, reversible.

## 10. What is explicitly deferred

Message/chat, deployment, organization hierarchy, and any entity without a
real query demanding it remain deferred (`GRAPH_MODEL.md`). Adding an entity
requires a concrete query, not a completeness argument.
