# TRON — Reasoning & Decision Engine

**Status**: Canonical. Authority for how TRON turns memory into grounded
answers, explanations, and recommendations.
**Last updated**: 2026-10-15.
**Current-state authority**: [`AGENT_SPEC.md`](AGENT_SPEC.md) (locked
operation boundary + agent seam) and [`architecture-minimal.md`](architecture-minimal.md).
**Related**: [`SCHEMA.md`](SCHEMA.md), [`INGESTION.md`](INGESTION.md),
[`ACTIONS.md`](ACTIONS.md), [`TECHSPEC.md`](TECHSPEC.md),
[`EVALUATION.md`](EVALUATION.md).

> **Current reality**: deterministic graph operations + deterministic answer
> composers exist (`internal/graph`, `internal/ui`). The agent loop exists as a
> provider-agnostic seam with **no LLM wired** (`internal/agent/agent.go`).
> Option comparison and planning are **PROPOSED**; contradiction detection ships
> as `FindConflicts` (E6 v1, 2026-10-16) and bounded context assembly as
> `AssembleEvidence` (M3, 2026-10-16).

---

## 1. The reasoning pipeline

```
Observe → Establish context → Retrieve evidence → Reason → Validate →
Plan → Evaluate alternatives → Authorize → Execute → Verify → Learn
```

| Stage | Deterministic or LLM? | Current | Notes |
|---|---|---|---|
| **Observe** | system | PARTIAL | `FindOwner/FindExperts/...` are the observable units today |
| **Establish context** | system | PROPOSED | who asks, what they may see, what they mean |
| **Retrieve evidence** | deterministic (graph) | IMPLEMENTED | bounded op traversal with provenance; `AssembleEvidence` assembles one entity's bounded context |
| **Reason** | LLM *phrases*; rules *decide* | PARTIAL | deterministic composers today; LLM synthesis later |
| **Validate** | deterministic | IMPLEMENTED (subset) | `knowledge.Answer.Validate` |
| **Plan** | rules + LLM phrasing | PROPOSED | goal/budget/deadline/dependency/risk |
| **Evaluate alternatives** | deterministic | PROPOSED | Pareto/weighted comparison |
| **Authorize** | deterministic | PROPOSED | [`ACTIONS.md`](ACTIONS.md) |
| **Execute** | system | PROPOSED | [`ACTIONS.md`](ACTIONS.md) |
| **Verify** | deterministic | PROPOSED | compare outcome to intent |
| **Learn** | system (data) | PROPOSED | new facts re-enter memory |

The pipeline is a conceptual control flow, not a promise that all stages run
for every question. A lookup question stops after **Validate**.

## 2. Retrieval: graph-first, hybrid when needed

**Graph-first.** The primary retrieval is deliberate, bounded traversal of the
operations (`AGENT_SPEC.md`). Every returned fact carries provenance; the
traversal *is* the evidence. This is the product thesis: remove the graph and
the reasoning degrades.

**When lexical/semantic retrieval helps.** Graph-first fails when the question
is phrased in words that don't match graph keys — synonyms, concepts, or
free-text ("the flaky checkout thing"). A hybrid step is justified when:

1. The query's entities are **not resolvable** by the deterministic tiers
   (`SCHEMA.md` §9), or
2. The evidence likely lives in **document text** not yet extracted to facts.

Then: a retrieval index proposes **candidate keys**, and the graph still
performs the reasoning over those candidates. The index never answers; it
routes. (See `INGESTION.md` §9.4 for the index's buildable-from-scratch
property.)

**Context assembly (IMPLEMENTED 2026-10-16).** Given (query, user scope), select
the *minimum* evidence set that can justify an answer:
- resolve entities → pick operations → run them;
- drop facts the user cannot see (permission-aware, `SECURITY.md`);
- rank by relevance/recency; cap size (deliberate retrieval, no graph dumps).

`internal/graph` `AssembleEvidence` (`EvidenceContext`) implements the retrieval
half for one entity: it resolves the entity with the canonical tiers (never
guessing between candidates), follows that entity's edges deliberately, scopes
to optional relationship types, filters **state** relations to an optional
`as_of` under the canonical half-open rule (event relations carry no window and
are not filtered, as in `FindExperts`), orders facts by a **total**
store-independent sort, caps the result by documented count/size limits, and
reports truncation explicitly. Unknown (`not_found`), ambiguous (candidates,
no pick), empty (in scope), over-budget (byte bound excluded every in-scope
fact), and malformed-temporal (error) inputs each have an explicit outcome; the
call is read-only. Permission-aware dropping and relevance/recency ranking remain
**PROPOSED** (no permission model or relevance score exists yet); `as_of` is the
only ordering/selection lever today. The agent loop reaches it through the
`assemble_context` tool.

**Evidence selection.** Keep a fact if it **bears on a claim**. Every kept
fact retains its `src_ref` so the claim can cite openable evidence.

## 3. Claim-to-source support

- **FACT** claims → a graph path whose edges carry provenance.
- **INFERENCE** claims → `rule_id` + basis paths (+ weights version).
- **RECOMMENDATION** claims → inference basis + `policy_id`.
- **PREDICTION** claims → model version + interval.

`internal/knowledge/answer.go` already refuses to render a claim with no
paths, or an INFERENCE without a `rule_id`. The engine must extend the same
discipline to PREDICTION/PROPOSED states (`SCHEMA.md` §2).

## 4. Handling contradiction, uncertainty, and missing information

| Case | Behavior |
|---|---|
| **Contradiction** (two state-relation edges, same relationship, incompatible claims, overlapping half-open validity — `SCHEMA.md` §6; for `OWNS`, different owners of one target) | Surface both with sources; do not pick silently. State that sources disagree. Event edges and node-property conflicts are out of scope for E6 v1. |
| **Stale fact** | Show with age; prefer recent; do not present stale as current. |
| **Missing information** | Say what is missing and what would resolve it; ask a clarifying question if cheap. |
| **Model uncertainty** | Surface an interval/confidence where a model provides one; otherwise do not fabricate confidence. |
| **Unresolvable entity** | Return candidates, never guess (`internal/graph` ResolveEntity). |

## 5. Deterministic calculations and business rules

Anything with a numeric or policy answer is computed deterministically and
tagged INFERENCE with a `rule_id` (`SCHEMA.md`, `knowledge-model.md`):

- goal progress vs target/horizon;
- budget vs committed + actual (units and periods respected);
- deadline feasibility (dependency-aware earliest-finish vs due date);
- dependency and blast-radius traversal (`DEPENDS_ON*`, `AFFECTS`);
- risk scores computed from explicit likelihood × impact fields;
- expertise (`expertise/v1`, `internal/knowledge/expertise.go`).

The LLM may **explain** a computed result; it may never **compute or override**
it. Same graph + same clock → same number (`NFR-1`).

## 6. Option comparison and recommendations

1. Enumerate feasible options within constraints (bounds, policy, budget).
2. Score each against explicit objectives (recovery of goal, cost, risk,
   time) using a documented normalization and weighting, **or** compute a
   Pareto set.
3. Return the recommendation(s) with the trade-off made visible, not hidden
   in a single opaque score.
4. Attach the full basis; a recommendation with no basis is not renderable.

Weighted-sum convenience and Pareto honesty are both allowed; the choice is
recorded with the recommendation so a reviewer can re-derive it (mirrors the
"no hidden weighting" stance in the paper-derived `PRD.md` success criteria).

## 7. Abstain or clarify

TRON **must** abstain (say "insufficient evidence") when:
- no fact path supports the claim;
- required entities do not resolve;
- sources contradict and the asker's intent can't disambiguate;
- a required rule input (unit, period, owner) is missing.

TRON **should** ask a clarifying question when a short exchange resolves the
ambiguity cheaply, otherwise return candidates. Never fill a gap with a guess.

## 8. Boundaries between LLM and deterministic logic

| Responsibility | Owner |
|---|---|
| Which operation(s) to call | LLM (selection) — bounded by step budget (`internal/agent`) |
| Traversal, counting, scoring, rules | Deterministic code |
| Phrasing an answer | LLM |
| Layer tag, provenance, evidence paths | Deterministic code (structured, not phrased) |
| Permission + authorization | Deterministic policy ([`ACTIONS.md`](ACTIONS.md)) |
| Creating facts | Ingestion only |
| Guessing/abstaining | Deterministic gate; LLM cannot override a required abstention |

Hard rule: **LLM output is never stored as a fact** and never grants
permission (`knowledge-model.md` invariant 3).

## 9. Approach comparison — what to use now, defer, or avoid

| Approach | Strength | Limit | Decision |
|---|---|---|---|
| **Graph reasoning (TRON's core)** | Exact paths, provenance, multi-hop, deterministic | Needs structured facts; cold-start | **USE NOW** (implemented) |
| **Conventional RAG (vector chunks)** | Handles free-text, no schema needed | Loses relationships/provenance; hallucination-prone | **HYBRID ONLY** as a recall aid (§2), never the reasoner |
| **Rules-based system** | Reproducible, auditable, cheap | Brittle to open-ended questions | **USE NOW** for all calculations/policy; the deterministic core |
| **Single-agent tool use** | Handles open-ended NL; picks tools | Non-deterministic; must be bounded | **USE (bounded) NOW** — the existing loop; wire a provider |
| **Multi-agent / frameworks** | Parallelism, role separation | Coordination cost, non-determinism, new infra, harder audit | **DEFER** — only when a concrete requirement a single bounded loop cannot meet appears; requires an ADR |

**No new infrastructure without a requirement.** Message queues, vector DBs,
service splits, and agent frameworks each need a measured reason and an ADR
(§10).

## 10. Changes that require an ADR

- Wiring a specific LLM provider (seam already exists).
- Adding a retrieval index / embeddings.
- Adding a second reasoning strategy or any multi-agent coordination.
- Materializing inferences or scores (today: never stored).
- Any optimization step that searches a model's output space (`ACTIONS.md` risk).

Each is recorded in [`RULES.md`](RULES.md) §ADR and `planning/decision-log.md`
before implementation.
