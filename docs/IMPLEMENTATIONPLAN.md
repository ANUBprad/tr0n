# TRON — Implementation Roadmap

**Status**: Canonical. Authority for the dependency-aware plan from current
implementation to the target organizational brain.
**Last updated**: 2026-10-12.
**Supersedes**: `EXECUTION_PLAN.md`, `planning/milestones.md`,
`planning/backlog.md` (kept for history).
**Related**: [`PRD.md`](PRD.md), [`TECHSPEC.md`](TECHSPEC.md),
[`TRACKER.md`](TRACKER.md), [`RULES.md`](RULES.md),
[`EVALUATION.md`](EVALUATION.md).

> **Historical progress is preserved, not rewritten.** The hackathon-era
> milestones (M0–M5 in the old `milestones.md`) produced real, committed
> software. Those results are summarized here as **Milestone 0**, not erased.

---

## Milestone 0 — Architecture & repository baseline (DONE)

- **Objective / value**: A locked architecture and a working deterministic
  core that proves the thesis without an LLM. Real user value: answer the five
  flagship questions with evidence.
- **Reuse**: everything in `cmd/tron`, `internal/{graph,knowledge,api,agent,seed,ui}`.
- **Gaps**: none for the demo; documentation was fragmented (this task).
- **Dependencies**: none.
- **Files**: existing tree.
- **Test strategy**: real-FalkorDB op tests, provenance/layering invariants,
  seed determinism + connectivity contract, `tron demo` pass/fail.
  (CONFIRMED: 9 test files.)
- **Acceptance**: `tron demo` passes 6/6; architecture locked in decision-log.
- **Definition of done**: **MET** (verify with `go test ./...` and `tron demo`).
- **Risks / deferred**: stale `.tron_audit.md`; hackathon-era docs coexist —
  resolved by banners + [`README.md`](README.md) index (this task).

## Milestone 1 — Trustworthy organizational memory

- **Objective / value**: A canonical knowledge model the rest of the system can
  trust, that survives change and contradiction.
- **Reuse**: locked schema (`GRAPH_MODEL.md`), provenance/temporal model,
  `internal/knowledge` types, seed generator.
- **Gaps**: contradiction detection; staleness surfacing; identity resolution
  beyond single source; permission/retention fields on facts.
- **Dependencies**: M0.
- **Files**: `internal/graph` (queries), `internal/knowledge` (types),
  `internal/seed` (fixtures for conflicts).
- **Test strategy**: E1 (resolution), E5 (temporal), E6 (contradiction),
  provenance invariants extended.
- **Acceptance**: as-of queries correct on time-shifted fixtures; injected
  contradictions surfaced (not hidden); duplicate identity yields candidates,
  never a wrong merge.
- **Definition of done**: suites E1/E5/E6 green; schema changes recorded in ADR.
- **Risks / deferred**: real multi-source identity resolution deferred to M2.
- **Explicitly NOT done**: no real sources yet.

## Milestone 2 — Data ingestion & synchronization

- **Objective / value**: TRON learns from real (or realistic) company data
  instead of only synthetic demo data.
- **Reuse**: `internal/graph` loaders, provenance model, `internal/seed`
  as fixture source.
- **Gaps**: the whole pipeline (`internal/ingest`), one real adapter, document
  store, cursor/delta sync, deletion propagation, permission propagation.
- **Dependencies**: M1 (needs a stable model to write facts into).
- **Files**: **new** `internal/ingest`, `internal/sync`; `cmd/tron` source/
  import/sync commands.
- **Test strategy**: adapter contract tests (fixture parse, idempotent
  re-ingest, malformed quarantine, deletion retraction); E7 permissions.
- **Acceptance**: choose **one** source (recommend the docs repo,
  `INGESTION.md` §3); re-ingest writes zero new facts; correction and deletion
  both propagate; every fact cites an openable source.
- **Definition of done**: `INGESTION.md` §10 satisfied; eval E7 green.
- **Risks / deferred**: scope creep across sources — build one, defer the rest.
- **Explicitly NOT done**: no multi-source resolution until a second source.

## Milestone 3 — Evidence-grounded reasoning

- **Objective / value**: Open-ended, natural-language questions answered with
  citations, abstention, and contradiction handling.
- **Reuse**: agent loop + `Decide` seam, six ops, `Answer.Validate`.
- **Gaps**: wire **one** LLM provider adapter; context assembly; contradiction
  and uncertainty rendering; abstention gate; `POST /v1/ask`.
- **Dependencies**: M2 (real data) and the ADR for the provider (RULES §4).
- **Files**: `internal/agent` (adapter), **new** `internal/reason`;
  `internal/api`, `internal/ui` (open questions).
- **Test strategy**: E3 (grounded correctness), E4 (citations, hard gate),
  E8 (recommendation quality), E12 (latency/cost); compare vs baselines.
- **Acceptance**: 100% citation validity; abstains when unsupported; provider
  swap is one adapter function.
- **Definition of done**: E3/E4 green; baseline comparison published for a
  small corpus.
- **Risks / deferred**: hallucination (mitigated by Validate); cost (E12).
- **Explicitly NOT done**: no multi-agent framework.

## Milestone 4 — Decision support

- **Objective / value**: Goal/budget/deadline/dependency/risk reasoning and
  option comparison with visible trade-offs.
- **Reuse**: deterministic rule style (`expertise/v1`), graph ops.
- **Gaps**: goal/commitment/budget/metric entities (SCHEMA §3), rule engine,
  Pareto/weighted comparison, `tron plan` / `POST /v1/plan`.
- **Dependencies**: M1 (schema), M3 (phrasing).
- **Files**: **new** `internal/rules`, `internal/reason`; schema + seed
  fixtures for goals/budgets.
- **Test strategy**: deterministic calculators unit-tested; E8 scenario ratings;
  no-hidden-weighting check.
- **Acceptance**: recommendations cite a reproducible computation; weight
  sensitivity is reported.
- **Definition of done**: at least one decision-support scenario end to end.
- **Risks / deferred**: R1 (optimizer exploits model error) — bounded search.
- **Explicitly NOT done**: no autonomous planning.

## Milestone 5 — Controlled action execution

- **Objective / value**: TRON can safely perform an authorized, audited,
  verified action.
- **Reuse**: read-only operation boundary, provenance/audit-by-evidence.
- **Gaps**: AUTHZ gate, approval records, executor, idempotency, verification,
  audit log, one safe action class (e.g. open a draft with a summary).
- **Dependencies**: M3 (recommendations), M4 (plans), [`ACTIONS.md`](ACTIONS.md) §9.
- **Files**: **new** `internal/actions`; `cmd/tron action ...`;
  `/v1/actions/*`.
- **Test strategy**: E9 (allow/approval/deny), E10 (exactly-once, failure
  classification); security review.
- **Acceptance**: zero unauthorized executions; duplicate delivery executes
  once; outcome verified and recorded as a fact; audit log complete.
- **Definition of done**: all [`ACTIONS.md`](ACTIONS.md) §9 conditions met.
- **Risks / deferred**: R5 (unsafe action), R6 (no backup) — mitigate before
  shipping.
- **Explicitly NOT done**: no high-impact (L5) actions.

## Milestone 6 — Proactive organizational intelligence

- **Objective / value**: TRON surfaces risks/opportunities (e.g. a decision's
  deadline slipping, a service losing its only expert) without being asked.
- **Reuse**: rules engine, graph traversal, scheduler.
- **Gaps**: triggers/watchers, alert ranking, dedup, delivery channel,
  precision controls.
- **Dependencies**: M4, M5 (alerts may propose actions).
- **Files**: **new** `internal/sync` watchers, `internal/rules` triggers;
  notification adapter.
- **Test strategy**: E11 (precision/recall on labeled events); alert fatigue
  controls.
- **Acceptance**: alerts are evidence-backed and meet a precision floor set
  empirically.
- **Definition of done**: one proactive scenario with measured precision.
- **Risks / deferred**: alert fatigue; false positives — start narrow.
- **Explicitly NOT done**: no unattended action without approval.

## Milestone 7 — Production readiness & comparative evaluation

- **Objective / value**: TRON is safe, observable, and *demonstrably* better
  than baselines.
- **Reuse**: everything; existing test discipline.
- **Gaps**: authN/Z, tenant isolation, backups/retention, metrics/tracing,
  image pinning, CI gates, the full eval corpus + baselines report.
- **Dependencies**: M1–M6.
- **Files**: `cmd/tron`, `internal/*` hardening; new CI config.
- **Test strategy**: full `EVALUATION.md` suite; hard gates E4/E7/E9/E10 in CI;
  GOOS/GOARCH compile matrix.
- **Acceptance**: all hard gates green in CI; published comparison vs keyword
  search, conventional RAG, and tool-LLM-without-memory.
- **Definition of done**: production-readiness checklist + eval report.
- **Risks / deferred**: R6, R12, R14 — close before any external exposure.

---

## Dependency spine

```
M0 (done) → M1 → M2 → M3 → M4 → M5 → M6 → M7
                      └──────┴───→ (M6 alerts may propose actions)
```

## Sequencing rule

One milestone at a time; each is gated by its acceptance criteria and by the
ADRs it requires ([`RULES.md`](RULES.md) §4). Live status is tracked in
[`TRACKER.md`](TRACKER.md). No capability is marked complete until its tests
exist and pass (`EVALUATION.md`).
