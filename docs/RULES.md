# TRON — Engineering Rules, Decision Register & Risk Register

**Status**: Canonical. Authority for engineering rules, architecture decisions
(ADR), and the risk register.
**Last updated**: 2026-10-15.
**Related**: `AGENTS.md` (the binding working rules), `opencode.md`,
`planning/decision-log.md` (chronological decision history),
[`TECHSPEC.md`](TECHSPEC.md), [`SECURITY.md`](SECURITY.md),
[`SCHEMA.md`](SCHEMA.md), [`EVALUATION.md`](EVALUATION.md).

`AGENTS.md` and `opencode.md` are the **binding** rules for how work is done.
This document records **architecture decisions** (ADR) and the **risk
register**; it does not restate the working rules, it cites them.

---

## 1. Engineering rules (binding, summarized)

Full text in `AGENTS.md`; applied to this repo:

- Lazy = efficient: climb the YAGNI ladder; the best code is code never written.
- Fewest files; deletion over addition; boring over clever.
- No new dependency if avoidable; stdlib first; interfaces only at the second
  implementation.
- Root-cause fixes at the shared function, not per-caller patches.
- Mark deliberate ceilings with a `ponytail:` comment naming the ceiling and
  upgrade path.
- One runnable check per non-trivial logic unit; no mock graph interfaces.
- Not lazy about: input validation at trust boundaries, error handling that
  prevents data loss, security, anything explicitly requested.
- `opencode.md`: be explicit about PROPOSED/UNDECIDED; never claim implemented
  functionality.

## 2. Single-source-of-truth (documentation authority)

| Subject | Authoritative document |
|---|---|
| Product vision & requirements | [`PRD.md`](PRD.md) |
| Target architecture | [`TECHSPEC.md`](TECHSPEC.md) |
| Current (locked) architecture | [`architecture-minimal.md`](architecture-minimal.md) |
| Knowledge model / schema | [`SCHEMA.md`](SCHEMA.md); locked current: [`GRAPH_MODEL.md`](GRAPH_MODEL.md), [`knowledge-model.md`](knowledge-model.md) |
| Ingestion & memory | [`INGESTION.md`](INGESTION.md) |
| Reasoning | [`REASONING.md`](REASONING.md); locked boundary: `AGENT_SPEC.md` |
| Actions & authority | [`ACTIONS.md`](ACTIONS.md) |
| Interfaces & flows | [`APPFLOW.md`](APPFLOW.md) |
| Security | [`SECURITY.md`](SECURITY.md) |
| Evaluation | [`EVALUATION.md`](EVALUATION.md) |
| Roadmap | [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md), [`TRACKER.md`](TRACKER.md) |
| Working rules | `AGENTS.md`, `opencode.md` |
| Decision chronology | `planning/decision-log.md` |
| Index | [`README.md`](README.md) |

Rules: one fact has one home. Drill down by reference, never by copy.

## 3. Architecture Decision Register (ADR)

Decisions already locked in `planning/decision-log.md` are referenced, not
repeated. New target-architecture decisions are recorded here with status.

### 3.1 Locked (existing, referenced)
| ID | Decision | Where recorded |
|---|---|---|
| D1 | Go as implementation language | decision-log |
| D2 | FalkorDB as primary graph; no graph abstraction layer | decision-log |
| D3 | Five-layer knowledge model; no silent promotion | decision-log, `knowledge-model.md` |
| D4 | Deterministic expertise (`expertise/v1`); LLM never ranks | decision-log |
| D5 | Temporal = valid-from/to on mutating edges; append-only | `knowledge-model.md` |
| D6 | Schema 11 entities / 16 relationships (v1) | `GRAPH_MODEL.md` |
| D7 | Graph boundary = 5 ops + ResolveEntity (`FindConflicts` added per A11, implemented 2026-10-16); `prepare_handoff` deferred | `AGENT_SPEC.md` |
| D8 | Test against real FalkorDB; no mocks | `architecture-minimal.md` |
| D9 | Seed = deterministic Go generator; wipe-and-recreate | decision-log |
| D10 | Agent = custom minimal loop behind one `Decide` seam | decision-log |

### 3.2 Target-architecture decisions (this document)

| ID | Decision | Rationale | Alternatives | Consequences | Status |
|---|---|---|---|---|---|
| **A1** | **Modular monolith**, one binary, package seams | Network/graph-bound workload; auditable single writer | Microservices | Split only on measured need; service split needs an ADR | PROPOSED |
| **A2** | **Graph-first hybrid retrieval**; index only routes | Keeps graph as reasoning substrate; index can be dropped | Vector-DB-first | Lexical/semantic index is optional and rebuildable | PROPOSED |
| **A3** | **Single writer of facts** (`internal/ingest`); API/agent read-only | Provenance enforcement is auditable structurally | Per-feature writers | All writes funnel through ingestion; verified outcomes are the only other writer | PROPOSED (seed is the only writer today) |
| **A4** | **Provenance minimum on every fact** | Any claim can answer "why" | Ad-hoc metadata | New entities must carry the minimum or be rejected | LOCKED (`knowledge-model.md`) |
| **A5** | **Bitemporal-lite on edges**; recorded-time deferred | Simplest correct model | Full bitemporal DB | Upgrade path documented | LOCKED |
| **A6** | **Deterministic controls own truth**; LLM selects/phrases only | Reproducible, auditable | LLM-owned logic | Every LLM output must cite paths/rule_id | LOCKED |
| **A7** | **Actions gated by determinism**, graduated autonomy L0–L5 | Safety; LLM cannot authorize | LLM-judged actions | Authorization gate outside model | PROPOSED ([`ACTIONS.md`](ACTIONS.md)) |
| **A8** | **Outcome verification closes the loop** (verified outcome → fact) | Actions are not "done" until verified | Fire-and-forget | Requires target read-back / event | PROPOSED |
| **A9** | **Inferences computed at query time, not stored** | Prevents inference→fact drift | Materialized inference edges | Recompute cost; trivial at this scale | LOCKED |
| **A10** | **No new infrastructure without a measured requirement + ADR** | Avoids speculative complexity | Adopt frameworks early | QoS: frameworks, queues, vector DBs, multi-agent all deferred | LOCKED (rule) |
| **A11** | **E6 contradiction semantics + read-only `POST /v1/conflicts`** — state-relation claims only, half-open overlap, `OWNS` exclusive per target; boundary 5 + 1 → 6 + 1 | Canonical rule needed a compatibility/interval spec; the API is the product surface; read-only keeps the single-writer invariant | Literal subject-side grouping (false positives); UI/CLI-only; agent-tool-only; DB-stored flags (violates A9) | Contract proposed in `AGENT_SPEC.md`; decisions in `planning/decision-log.md`; `TECHSPEC.md`/`APPFLOW.md` updated; operation **IMPLEMENTED** (2026-10-16) | DECIDED (semantics, 2026-10-15); operation IMPLEMENTED (2026-10-16) |

## 4. Decisions that require an ADR before implementation

- Wiring an LLM/embedding provider (seam exists; provider is an open decision).
- Introducing a retrieval index, message queue, scheduler, or cache.
- Splitting any component into a separate process/service.
- Materializing inferences/scores, or changing the provenance minimum.
- Any write/action capability (must also pass [`ACTIONS.md`](ACTIONS.md) §9).
- Re-adding a cut schema edge (`MEMBER_OF`, etc.).

## 5. Risk register (prioritized)

Severity: H/M/L. State: Open / Latent / Mitigated / Accepted.

| ID | Risk | Category | Sev | State | Mitigation / owner doc |
|---|---|---|---|---|---|
| R1 | Model error exploited by optimizer/planner | Correctness | H | Latent | Bound search to observed range; require verify ([`REASONING.md`](REASONING.md)) |
| R2 | Stale facts presented as current | Staleness | H | Open | `observed_at` + temporal filters; surface age ([`SCHEMA.md`](SCHEMA.md) §6) |
| R3 | Derived-data leakage across permissions | Permissions | H | Latent | Permission-aware retrieval + input re-check ([`SECURITY.md`](SECURITY.md) §4) |
| R4 | Prompt injection from ingested content | Security | H | Latent | Untrusted-data rule; deterministic AUTHZ ([`ACTIONS.md`](ACTIONS.md) §8) |
| R5 | Unsafe/unauthorized action executes | Safety | H | Latent | L0 default; gate + approvals + idempotency ([`ACTIONS.md`](ACTIONS.md)) |
| R6 | Data loss / no backup | Reliability | M | Open | No volume today; define backup before real data ([`SECURITY.md`](SECURITY.md) §11) |
| R7 | Identity resolution wrong-merge or collision | Correctness | H | Open | No auto-merge on fuzzy; candidates ([`SCHEMA.md`](SCHEMA.md) §9) |
| R8 | Latency on wide Cypher scans | Performance | M | Open | Bounded depth; index `key`; measure first (`architecture-minimal.md`) |
| R9 | LLM cost per answer | Cost | M | Latent | Deterministic core answers without LLM; budget ([`EVALUATION.md`](EVALUATION.md) E12) |
| R10 | Expertise weights unvalidated | Quality | M | Open | Versioned rule; visible basis; validate with eval |
| R11 | Complexity growth / speculative infra | Complexity | M | Open | ADR gate (A10); YAGNI; fewest files |
| R12 | Unpinned FalkorDB image drift | Supply chain | L | Open | Pin image digest |
| R13 | Single global graph lock limits concurrency | Performance | L | Accepted | Documented ceiling; per-op handle upgrade (`internal/graph/graph.go`) |
| R14 | No auth if served beyond loopback | Security | H | Mitigated (loopback) | Default loopback; add authN before exposure |
| R15 | Source schema drift breaks parsing | Reliability | M | Latent | Per-record validation + quarantine ([`INGESTION.md`](INGESTION.md) §4) |

## 6. Rules for changing this document

- A new decision appends a row to §3.2 with status PROPOSED until implemented
  and validated; only then LOCKED.
- A decision that reverses a locked one must be recorded with its reason
  (e.g. re-adding `MEMBER_OF`) and referenced from `planning/decision-log.md`.
- Risks change state only with evidence (a test, a control, a cited code path);
  never mark Mitigated on intent.
