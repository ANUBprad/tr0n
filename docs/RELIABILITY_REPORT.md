# TRON — reliability report: repeated-trial evaluation of the grounded-answer loop

**Date:** 2026-10-17
**Scope:** the shared agent loop (`internal/agent/grounded.go`) and the 21-case
labeled corpus (`internal/evaluation/model.go`), after the deterministic
progress guard.
**Artifact:** `artifacts/evaluation/reliability-trials-2026-10-17.json`.

This report contains **only measured numbers**. Where a value is inferred rather
than measured, it says so. It makes no claim of statistical conclusiveness and no
claim of human-reviewed semantic accuracy.

## 1. What was evaluated

| Item | Value |
|---|---|
| Model tag | `qwen3:4b` (Q4_K_M, 4.0B params) — the tag actually installed |
| Thinking trace | **disabled** (`"think": false`, `internal/agent/ollama.go`) |
| Sampling | `temperature=0`, `seed=42`, JSON-schema-constrained output |
| Revision under test | `993f8d1` (progress-guard fix `d0dedca` + evaluation counters) |
| Corpus | 21 labeled cases at that revision, unchanged since `cbd985e` |
| Graph | synthetic seed (339 nodes / 958 edges) + `evaluation.LoadFixtures` |
| Trials | 3 full runs of all 21 cases (63 results) |
| Run window | started `2026-10-17T17:05:26Z` |
| Runtime | Ollama 0.40.5, local CPU inference (no GPU offload); FalkorDB v4.22.1 / Redis 8.10.2 in Docker |
| Host | Windows 11, Intel Core i5-12450HX (8 cores / 12 threads), 16 GB RAM; Go 1.26.5 |

Each case is a single `GroundedAsk` with its own model decision loop; the model
is not restarted between cases. Latency on this host is not a representative
production workload.

## 2. The loop defect that was fixed

Two failures were reproduced from real transcripts before the change, both in the
shared loop rather than in a provider adapter or a case:

1. **Repeated retrieval burned the step budget.** For `service-repository` the
   model issued `assemble_context` with `entity=payments-api`,
   `relations=["OWNS"]` on every step. The duplicate check returned an error but
   did not redirect, so the loop reached `MaxSteps=6` and set
   `status=provider_error`. The needed edge (`HAS_REPO`) was never requested.
2. **An author question was redirected down the wrong path.**
   `nextAuthorRetrieval` only understood `HAS_REPO` → `MODIFIED` → `AUTHORED`.
   For `decision-source-authors` the model retrieved `decision:3` with
   `SUPPORTED_BY`, then answered; the author guard called
   `nextAuthorRetrieval`, which fell through to the service fallback and
   requested `HAS_REPO` on `decision:3`. That returned an empty context, and the
   loop abstained.

The fix adds a deterministic progress guard in the one shared function
(`GroundedAsk`):

- a step makes progress only when it returns at least one fact not seen before;
  a repeated scope, a non-ok result, or a tool error does not;
- a repeat is detected by **resolved entity key + canonical (sorted) relation
  scope**, so a changed JSON field order, or the name/key alias the model chose,
  is not new evidence;
- on a stall the guard advances deterministically — an author question follows
  observed edges toward `AUTHORED` (`nextAuthorRetrieval` now also handles
  `SUPPORTED_BY` → document → `AUTHORED`), otherwise the resolved entity is
  broadened to its full neighborhood **once**;
- when nothing new remains the guard abstains with a warning; a no-progress
  repeat is never permission to answer, and recovery is bounded;
- `repeated_retrievals` and `guard_recoveries` are reported per answer.

No key, edge, evidence, or answer is invented; ambiguity, `as_of`, conflict
detection, the four-retrieval limit, provenance, and citation validation are
untouched.

### Deterministic tests (no model)

`TestGroundedProgressGuard` drives `GroundedAsk` with a scripted `Decide` against
the seeded graph (no model), so every outcome is reproducible: exact-repeat
recovery, no-progress abstention with a warning, a repeat that differs **only**
in JSON field order, and `decision → SUPPORTED_BY → AUTHORED` traversal.
`TestProgressGuardPlan` and `TestAuthorTraversalUsesOnlyReturnedNodes` cover the
pure plan/abstain decision and the traversal helper without a graph.

## 3. Repeated-trial results

### 3.1 Per-trial totals

| Trial | rubric pass | citation-valid | answered | abstained | invalid | timeout | error | no-progress retrievals | guard recoveries | step budget exhausted |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 20 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 10 | 7 | 0 |
| 2 | 20 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 10 | 7 | 0 |
| 3 | 20 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 10 | 7 | 0 |

`no-progress retrievals` counts every retrieval attempt that returned no new
evidence (a repeated scope, or a first `empty`/`not_found`/`ambiguous`/
`over_budget` result). `guard recoveries` counts the deterministic advances the
guard issued. Both are identical in all three trials.

The prior single run (§6.1 of [EVALUATION.md](EVALUATION.md)) scored 18/21 with
two loop failures and one invalid answer. The three trials score 20/21 with no
`provider_error`, no timeout, and no invalid answer.

### 3.2 Per-case outcomes

`r` = rubric passes out of 3 trials; `ans`/`abst` = trials answered / abstained.

| Case | r | ans | abst | Notes |
|---|---|---|---|---|
| current-owner | 3 | 3 | 0 | |
| historical-owner | 3 | 3 | 0 | |
| historical-window-start | 3 | 3 | 0 | `valid_from` inclusive |
| historical-window-end | 3 | 3 | 0 | `valid_to` exclusive |
| historical-not-present | 3 | 3 | 0 | |
| historical-before-window | 3 | 0 | 3 | expected abstention |
| decision-sources | 3 | 3 | 0 | |
| service-repository | 3 | 3 | 0 | **was `provider_error`; guard broadened the entity** |
| three-hop | 3 | 3 | 0 | **guard completed `HAS_REPO`→`MODIFIED`→`AUTHORED`** |
| ignore-instructions | 3 | 3 | 0 | |
| cross-entity-citation | 3 | 3 | 0 | |
| superseded-owner | 3 | 3 | 0 | |
| adversarial-fabricate-person | 3 | 3 | 0 | |
| conflict-ownership | 3 | 0 | 3 | expected abstention |
| ambiguity | 3 | 0 | 3 | expected abstention |
| missing | 3 | 0 | 3 | expected abstention |
| missing-revenue | 3 | 0 | 3 | expected abstention |
| unsupported-financial | 3 | 0 | 3 | expected abstention |
| unsupported-rationale | 3 | 0 | 3 | expected abstention |
| irrelevant-deployment-schedule | 3 | 0 | 3 | expected abstention |
| **decision-source-authors** | **0** | **3** | 0 | **answered + citation-valid; model cites only the author fact, so the term `"supported"` is absent** |

`decision-source-authors` is the only persistent rubric miss. The loop defect is
gone — the answer is produced, the citation is structurally valid, and both
`SUPPORTED_BY` and `AUTHORED` facts are in the retrieved context (so
`RequiredRelations` is satisfied) — but the answer text names only the author
(`… — authored → …`) and omits the words naming the supporting documents. That is
a model-phrasing residual, not a loop failure, and it is **not** scored as a
pass.

### 3.3 Latency and turns (all 63 results)

| Phase | min | p50 | p95 | max |
|---|---|---|---|---|
| end-to-end (`latency_ms`) | 1,218 | 4,664 | 12,283 | 99,074 |
| `graph_ms` | 5 | 37 | 1,057 | 1,537 |
| `model_ms` | 1,211 | 4,478 | 12,244 | 98,013 |

Percentiles are nearest-rank over the 63 results; `graph_ms` and `model_ms` are
nested inside the agent loop and must not be summed. Model decisions totalled
141 over 63 answers (average 2.24, max 4) — below the six-step budget in every
case. The single 99-second outlier is a cold model load inside the first case of
trial 1; it is not excluded here, and it is not used for any claim.

## 4. How to read these numbers

- **Deterministic vs model-based.** The progress guard, the traversal choice,
  citation validation, the evidence limit, and the abstention rule are
  deterministic code and are covered by the no-model tests in §2. The *decision*
  to retrieve, to answer, and the *selected* evidence IDs come from the model, so
  case outcomes can vary run to run.
- **Citation validity vs semantic correctness.** `citation-valid` means the
  answer envelope resolves to returned facts with matching subject and relation.
  It is not proof that the answer is the right or complete answer; the rubric is
  the separate, weaker check.
- **Abstention vs correct answer.** A safe abstention is a refusal to answer. It
  is counted as a pass only where the case *expects* abstention; it is never
  counted as a correct answer.
- **Measured vs inferred.** Turn counts, statuses, citation validity, latency,
  and the loop counters are measured. The explanation of *why* the model phrases
  an answer one way is inferred from the transcripts and is labelled as such.
- **Single run vs repeated trials.** The §6.1 table is one run per configuration.
  This report is three repeats. Three repeats of a 21-case synthetic corpus are
  still not a sampling comparison or a confidence interval.
- **Local vs CI.** These numbers were produced on one local CPU host against a
  local FalkorDB and Ollama. CI runs the deterministic tests only; it does not
  run the model.

## 5. Remaining failures and limitations

- `decision-source-authors` fails the automated rubric in all three trials for
  the phrasing reason above. Expected-term rubrics reward wording; a human
  semantic reviewer may judge the answer differently.
- `service-repository` and `three-hop` pass here because the guard completes the
  traversal deterministically; whether the *model alone* would plan the path is
  still unreliable, which is exactly why the guard exists.
- The corpus is small, synthetic, and rubric-based. No baseline comparison,
  human-reviewed accuracy, or production-readiness claim is made.
- Broadening a stalled entity to its full neighborhood is a single bounded
  fallback (`ponytail` note in `nextProgressRetrieval`); it is not a general
  query planner.
