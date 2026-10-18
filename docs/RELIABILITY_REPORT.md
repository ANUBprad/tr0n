# TRON — reliability report: repeated-trial evaluation of the grounded-answer loop

**Date:** 2026-10-18
**Scope:** the shared agent loop (`internal/agent/grounded.go`) and the 21-case
labeled corpus (`internal/evaluation/model.go`), after the deterministic
progress guard and the author-path fix.
**Artifacts:** `artifacts/evaluation/reliability-trials-2026-10-18.json`
(post-fix, current run) and `artifacts/evaluation/reliability-trials-2026-10-17.json`
(pre-fix run, preserved).

This report contains **only measured numbers**. Where a value is inferred rather
than measured, it says so. It makes no claim of statistical conclusiveness and no
claim of human-reviewed semantic accuracy.

## 1. What was evaluated

| Item | Value |
|---|---|
| Model tag | `qwen3:4b` (Q4_K_M, 4.0B params) — the tag actually installed |
| Thinking trace | **disabled** (`"think": false`, `internal/agent/ollama.go`) |
| Sampling | `temperature=0`, `seed=42`, JSON-schema-constrained output |
| Revision under test | `3ff7473` (author-path fix) on top of `89a3df8`/`993f8d1` (progress guard `d0dedca` + evaluation counters) |
| Corpus | 21 labeled cases, unchanged since `cbd985e` |
| Graph | synthetic seed (339 nodes / 958 edges) + `evaluation.LoadFixtures` |
| Trials | 3 full runs of all 21 cases (63 results) |
| Run window | started `2026-10-18T03:52:12Z` |
| Runtime | Ollama 0.40.5, local CPU inference (no GPU offload); FalkorDB v4.22.1 / Redis 8.10.2 in Docker |
| Host | Windows 11, Intel Core i5-12450HX (8 cores / 12 threads), 16 GB RAM; Go 1.26.5 |

Each case is a single `GroundedAsk` with its own model decision loop; the model
is not restarted between cases. Latency on this host is not a representative
production workload.

## 2. The loop defects that were fixed

### 2.1 Repeated retrieval burned the step budget; author traversal took the wrong path

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

### 2.2 Author answers stated the endpoint but dropped the connecting relationship

`decision-source-authors` failed the automated rubric in all three pre-fix trials.
Inspecting the three saved outputs (`artifacts/evaluation/reliability-trials-2026-10-17.json`)
shows the same shape every time, with no loop error:

- contexts retrieved: `decision:3 [SUPPORTED_BY]` (3 facts) and `doc:3 [AUTHORED]`
  (1 fact) — so `SUPPORTED_BY` and `AUTHORED` **were** both in evidence and
  `MinContexts: 2` was met;
- the model's accepted proposal cited exactly one evidence id, the `AUTHORED`
  fact anchored to `doc:3`;
- the rendered answer was therefore
  `Yusuf Gunn — authored → Redis versus Kafka for payment idempotency.`

That answer names a person and a document but never states that the document
supports the decision — the `decision → SUPPORTED_BY → document` edge was
retrieved and then dropped by the model's citation choice. Re-running the single
case showed why it is not a stable wording artifact: the model sometimes re-cites
the three support facts after the guard's hop and sometimes switches to the
`AUTHORED` fact, so the same input passed or failed depending on the sample. Under
CPU contention a further branch appeared, in which the model cited *only* the
support facts and omitted the author entirely: the mirror image of the same
defect.

The rubric was **not** changed. Its term `"supported"` is not a synonym test: it
only appears when the answer text states the retrieved support relationship, so
the term is a faithful proxy for the question's material requirement ("which
documents support … and who authored them"). The fix is in the shared loop:

- after the model's claims are parsed and before citation validation,
  `withAuthorPath` prepends the retrieved facts that chain from the question's
  target entity to each `AUTHORED` edge the loop retrieved;
- each added fact becomes its own claim anchored to its own subject and relation,
  so `GroundedAnswer.Validate()` still applies to it unchanged;
- facts the model already cited are not repeated, and only facts the loop
  actually retrieved are added — no key, edge, person, or rationale is invented;
- `linkFacts` only hops between retrieved context entities, so the chain never
  wanders onto an unrelated neighbour that merely shares a node;
- the step is gated on `asksForAuthorship`, so a question without an `AUTHORED`
  edge and without author intent is left exactly as the model made it.

Post-fix the same case renders both halves of the path (see §3.2), and `three-hop`
— which has the same model behaviour but a rubric that did not test the path —
now states `service → repository → pull request → author` as well.

### Deterministic tests (no model)

`TestGroundedProgressGuard` drives `GroundedAsk` with a scripted `Decide` against
the seeded graph (no model), so every outcome is reproducible: exact-repeat
recovery, no-progress abstention with a warning, a repeat that differs **only**
in JSON field order, and `decision → SUPPORTED_BY → AUTHORED` traversal.
`TestProgressGuardPlan` and `TestAuthorTraversalUsesOnlyReturnedNodes` cover the
pure plan/abstain decision and the traversal helper without a graph.

For the author-path fix: `TestLinkFacts` covers the chain builder (shortest
chain, direction-agnostic, retrieved-contexts-only, no-chain),
`TestWithAuthorPathAddsOnlyRetrievedAuthorEdges` covers the gate and the added
claims, and `TestGroundedAuthorPathStatesTheConnectingRelationship` drives
`GroundedAsk` against the seeded graph with a scripted `Decide` in **both** model
branches — citing only the author endpoint and citing only the support facts — and
requires the rendered answer to contain both halves of the path and to validate.

## 3. Repeated-trial results

### 3.1 Per-trial totals

| Trial | rubric pass | citation-valid | answered | abstained | invalid | timeout | error | no-progress retrievals | guard recoveries | step budget exhausted |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 21 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 6 | 3 | 0 |
| 2 | 21 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 6 | 3 | 0 |
| 3 | 21 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 6 | 3 | 0 |

`no-progress retrievals` counts every retrieval attempt that returned no new
evidence (a repeated scope, or a first `empty`/`not_found`/`ambiguous`/
`over_budget` result). `guard recoveries` counts the deterministic advances the
guard issued. Both are identical in all three trials. Non-empty claims numbered
20 / 18 / 18 across the trials and every one was citation-valid (the pre-fix run
had 15 / 15 / 15: the increase is the author-path claims added for the two author
questions, plus the extra supporting documents the model cited in one trial of
`decision-source-authors`).

The pre-fix trials of the same corpus (§3.4) scored 20/21 with the same model,
corpus, and seed; the only per-case difference in the whole corpus was
`decision-source-authors`. The three post-fix trials score 21/21 with no
`provider_error`, no timeout, and no invalid answer. The single run recorded in
§6.1 of [EVALUATION.md](EVALUATION.md) predates the progress guard and scored 18/21
with two loop failures and one invalid answer.

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
| three-hop | 3 | 3 | 0 | **guard completed `HAS_REPO`→`MODIFIED`→`AUTHORED`; answer now states all three hops** |
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
| decision-source-authors | 3 | 3 | 0 | **was 0/3; answer now states `decision —SUPPORTED_BY→ document` and `person —AUTHORED→ document`** |

All 21 cases pass the automated rubric in all three trials. `decision-source-authors`
is the case that changed. The pre-fix answer was
`Yusuf Gunn — authored → Redis versus Kafka for payment idempotency.`; the
post-fix answer adds the retrieved support edge, e.g.

```
Choose Redis over Kafka for payment idempotency — supported by → Redis versus Kafka for payment idempotency.
Yusuf Gunn — authored → Redis versus Kafka for payment idempotency.
```

In one of the three post-fix trials the model additionally cited the other two
supporting documents, so that answer had 4 claims and listed all three documents.
Both variants state every claim as a retrieved edge and answer both halves of the
question. **This is an automated rubric outcome; no human reviewer has judged
these outputs**, and a rendered predicate matching the term `"supported"` is a
structural proxy for relevance, not proof of entailment.

### 3.3 Latency and turns (all 63 results)

| Phase | min | p50 | p95 | max |
|---|---|---|---|---|
| end-to-end (`latency_ms`) | 1,354 | 4,009 | 13,625 | 17,462 |
| `graph_ms` | 4 | 104 | 1,271 | 5,169 |
| `model_ms` | 1,349 | 3,456 | 12,791 | 16,900 |

Percentiles are nearest-rank over the 63 results; `graph_ms` and `model_ms` are
nested inside the agent loop and must not be summed. Model decisions totalled 132
over 63 answers (average 2.1, min 1, max 4) — below the six-step budget in every
case.

These are **not** reported as an improvement over the pre-fix run in §3.4, whose
p50 was 4,664 ms and whose maximum was 99,074 ms. The change only adds
server-side rendering over facts already retrieved, and the difference is in
`model_ms` — host CPU contention between the two windows (the pre-fix run had two
multi-second outliers late in trial 3). Attributing any of it to the code change
would be an unmeasured inference, so none is made.

### 3.4 Pre-fix run for comparison

`artifacts/evaluation/reliability-trials-2026-10-17.json` (revision `993f8d1`;
same model, corpus, seed, and 3×21 configuration):

| Trial | rubric pass | citation-valid | answered | abstained | invalid | timeout | error | no-progress retrievals | guard recoveries |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 20 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 10 | 7 |
| 2 | 20 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 10 | 7 |
| 3 | 20 / 21 | 21 / 21 | 13 | 8 | 0 | 0 | 0 | 10 | 7 |

Latency (63 results, min/p50/p95/max): end-to-end 1,218 / 4,664 / 12,283 /
99,074 ms, `graph_ms` 5 / 37 / 1,057 / 1,537, `model_ms` 1,211 / 4,478 / 12,244 /
98,013. The only per-case difference from §3.2 is `decision-source-authors`
(0/3 → 3/3). The higher `no-progress retrievals` (10 vs 6) and `guard recoveries`
(7 vs 3) reflect the model taking longer paths in that window; the guard code is
identical between the two runs.

## 4. How to read these numbers

- **Deterministic vs model-based.** The progress guard, the traversal choice, the
  author-path completion, citation validation, the evidence limit, and the
  abstention rule are deterministic code and are covered by the no-model tests in
  §2. The *decision* to retrieve, to answer, and the *selected* evidence IDs come
  from the model, so case outcomes can vary run to run.
- **Rendered facts vs generated prose.** The answer text is not model prose: the
  server renders the endpoints and predicate of each cited evidence id, so a model
  cannot relabel an edge. This is why the author-path completion can add the
  retrieved connecting edge to the answer without inventing an assertion — the
  added line is the retrieved fact itself.
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

- No case fails the automated rubric in the post-fix trials. That is **not** a
  claim that every answer is right: the rubric is an expected-term/relations check
  over rendered facts, and no human has reviewed the outputs.
- `service-repository`, `three-hop`, and `decision-source-authors` pass because the
  loop completes the traversal and states the path deterministically; whether the
  *model alone* would plan or state the path is still unreliable, which is exactly
  why the loop does it.
- The added path claims are rendered in retrieval/chain order, so when the model
  already cited an upstream support fact the added `AUTHORED` line can appear
  before it. The set of lines is correct and complete; the ordering is cosmetic
  (`ponytail` note on `withAuthorPath`).
- The corpus is small, synthetic, and rubric-based. No baseline comparison,
  human-reviewed accuracy, or production-readiness claim is made.
- Broadening a stalled entity to its full neighborhood is a single bounded
  fallback (`ponytail` note in `nextProgressRetrieval`); it is not a general
  query planner.
