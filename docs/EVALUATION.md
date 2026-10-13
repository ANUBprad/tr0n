# TRON — Evaluation & Quality Assurance

**Status**: Canonical. Authority for how TRON measures quality and trust.
**Last updated**: 2026-10-13.
**Current-state authority**: testing posture in
[`architecture-minimal.md`](architecture-minimal.md) and the existing Go test
suites (10 files under `cmd/` and `internal/`).
**Related**: [`PRD.md`](PRD.md) §8, [`SCHEMA.md`](SCHEMA.md),
[`REASONING.md`](REASONING.md), [`ACTIONS.md`](ACTIONS.md),
[`SECURITY.md`](SECURITY.md), [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md).

> **No fabricated results.** This document defines *what to measure, how, and
> against what baseline*. Acceptance thresholds are set empirically once a
> baseline exists. The only claims here are about test *structure*.

---

## 1. Testing philosophy (existing + extension)

Existing posture (`architecture-minimal.md`, `AGENTS.md`):

1. **The graph's behavior is the behavior under test** — operation tests run
   against a real FalkorDB in Docker, no mock graph (CONFIRMED: `graph_test.go`,
   `api_test.go`, `seed_test.go`, etc.).
2. **Provenance is an invariant, tested** — a FACT result missing `src_*` fails.
3. **Layering is tested** — an inference labeled FACT fails.
4. **`-short` runs pure tests without Docker**; full suite needs FalkorDB
   (`README.md`).
5. **One runnable check per non-trivial unit** (`AGENTS.md`).

The suites below extend this to reasoning, actions, permissions, and
cost/latency — the parts that don't exist yet.

## 2. Test suites and their fixtures

| # | Suite | Fixture (labeled) | What it proves | Status |
|---|---|---|---|---|
| E1 | **Entity resolution** | pairs of records known to be same/different | merge correctness, no false merges | PARTIAL (tiers exist) |
| E2 | **Multi-hop graph queries** | fixtures answerable only via ≥3 hops | the product thesis, as a test | PARTIAL (seed contract test) |
| E3 | **Source-grounded correctness** | Q/A with known ground-truth answers + sources | answers match evidence | PROPOSED |
| E4 | **Citation support** | answers + their claims | every claim cites a valid path (`Answer.Validate`) | IMPLEMENTED (structural) |
| E5 | **Temporal accuracy** | time-shifted fixtures (owner 2026 vs 2027) | correct as-of results | PROPOSED (temporal as-of querying and time-shifted fixtures absent) |
| E6 | **Contradiction detection** | injected conflicting facts | both surfaced, none hidden | PROPOSED |
| E7 | **Permission enforcement** | facts with differing scopes | no derived leakage | PROPOSED |
| E8 | **Recommendation quality** | scenarios rated by humans | useful/correct vs baseline | PROPOSED |
| E9 | **Action authorization** | allow / require-approval / deny fixtures | gate honors policy | PROPOSED |
| E10 | **Duplicate & failed execution** | replayed idempotency keys; injected target failures | executes once; failures classified | PROPOSED |
| E11 | **Proactive alert quality** | events with/without true alerts | precision/recall of alerts | PROPOSED (no proactive feature yet) |
| E12 | **Latency & cost** | workload replay | p50/p95 per op; tokens/$ per answer | PARTIAL (per-op ms captured in UI) |

## 3. Baselines and comparison

TRON must be compared, on the same fixture corpus, against simpler systems so
the graph's value is measurable rather than asserted:

| Baseline | What it represents | Why include |
|---|---|---|
| **Keyword search** | grep/BM25 over documents | shows graph beats text matching on relational questions |
| **Conventional RAG** | chunk + embedding retrieval + LLM | the standard alternative the product thesis argues against |
| **Tool-using LLM without memory/evidence** | LLM with generic tools, no graph | isolates the contribution of memory + provenance |
| **TRON (graph + provenance)** | the system under test | — |

Metrics per system: answer correctness, citation validity, abstention
calibration, multi-hop accuracy, latency, cost. **Publish the baseline and
the corpus**; a result without both is not a result.

## 4. Acceptance-threshold process (empirical)

1. Build the labeled fixture corpus (start small; grow with `E1`–`E12`).
2. Run **all** systems on the same corpus.
3. Record raw scores (no rounding-away of losses).
4. Set each threshold as a **floor that blocks a merge**, e.g. "citation
   validity = 100%", "no permission-leak in E7", "E10 duplicate executes
   exactly once". Where quality is inherently graded (E8), require a margin
   over the strongest baseline, not an absolute number chosen in advance.
5. Re-measure every milestone; a regression fails CI.

Hard gates (binary, not graded): E4 = 100%; E7 = zero leaks; E9 = zero
unauthorized executions; E10 = exactly-once. These are correctness gates, not
quality targets.

## 5. Determinism and reproducibility

- Expertise and all rules are pure functions of (graph, clock); tests inject
  the clock (`internal/knowledge/expertise.go` `now`).
- Seed data is deterministic (`internal/seed`); same input → same graph
  (CONFIRMED by `seed_test.go`).
- Every eval run records: data version, model/provider version, random seeds,
  and software revision — otherwise results are not comparable over time.

## 6. What is explicitly NOT claimed yet

- No accuracy, latency, or cost numbers exist, because no labeled corpus, no
  wired LLM, and no real ingestion exist.
- The demo's pass/fail checks (`cmd/tron/demo.go`) prove that operations
  *answer*, not that the answers are *correct vs a baseline*.

## 7. Definition of done for the evaluation milestone

- At least E1, E2, E3, E4, E5, E6, E9, E10 have runnable suites with
  fixtures.
- The four baselines are implemented enough to run on the corpus.
- A published report states corpus, baselines, metrics, and thresholds.
- CI enforces the hard gates (E4, E7, E9, E10) on every change.
