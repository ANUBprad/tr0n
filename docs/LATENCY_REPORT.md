# Three-hop latency report

**Date:** 2026-10-17
**Question measured:** the `three-hop` evaluation case —
*"Find an author of a pull request that modified this service's repository.
Follow HAS_REPO to the repository, incoming MODIFIED to a PullRequest, then
incoming AUTHORED to its Person. State who authored which pull request."*
(target `service:payments-api`).

This report contains **only measured numbers**. Where a number was not measured,
it says so. Latency on this hardware is not a representative production workload.

## Environment

| Item | Value |
|---|---|
| Host | Windows 11, Intel Core i5-12450HX (8 cores / 12 threads), 16 GB RAM |
| Go | 1.26.5 windows/amd64 |
| Graph store | FalkorDB (graph module 42201 / v4.22.1, Redis 8.10.2), local Docker |
| Model runtime | Ollama 0.40.5, local, CPU inference (no GPU offload) |
| Model tag | `qwen3:4b` (Q4_K_M, 4.0B params) |
| Seed | synthetic `acme-org/synthetic/v1`; 339 nodes / 958 edges |

The repository's documented default tag is `qwen3:4b-instruct`. That tag is
**not installed** on this host; the installed tag is `qwen3:4b`. The bare
`qwen3:4b` tag ships with *thinking* enabled by default (Ollama reports the
model capability `thinking`), whereas `qwen3:4b-instruct` does not. This
difference is the whole story of the optimization below.

## Method

- Each iteration issues exactly the same question the same way; the request is
  the one recorded in `artifacts/benchmarks/three-hop-latency-*.json`
  (`question`, `target`, `model`, `graph`).
- **3 warm-up iterations** are run first and excluded; the warm-ups and their
  timings are recorded in the artifact.
- **20 measured iterations** are then run and reported.
- Percentiles use the **nearest-rank** method: `rank = ceil(p * n)` clamped to
  `[1, n]`, read from the ascending sort (`cmd/tron/bench.go`).
- Fixed configuration: `temperature=0`, `seed=42`, `num_ctx=16384`,
  `num_predict=1800`, JSON-schema-constrained output.
- The model is **preloaded** before the run (`model_preloaded_before_run`), so
  no model load is inside the measured window. The graph is **reset once**
  before the run (fresh `tron_bench`) and **not** reset between iterations
  (`graph_reset_between_runs=false`).
- Phase semantics: `graph_ms` and `model_ms` are **nested** inside the agent
  loop; `validate_ms` and `render_ms` run after it. Phases must not be summed —
  `latency_ms` (end to end) is authoritative.

### Reproduction

```powershell
$env:TRON_OLLAMA_URL="http://localhost:11434"
$env:TRON_OLLAMA_MODEL="qwen3:4b"
go build -o tron.exe ./cmd/tron
# Use cmd redirection: Windows PowerShell 5.1 '>' writes UTF-16LE, which
# corrupts the JSON for other tools. cmd's '>' passes the tool's UTF-8 through.
cmd /c ".\tron.exe bench 3 20 > artifacts\benchmarks\three-hop-latency.json"
```

`tron bench [warmup] [iterations]` also fails loudly if `TRON_OLLAMA_MODEL` is
unset, so an accidental wrong-model run is not silently reported. The committed
JSON artifacts were re-encoded from the run's UTF-16LE redirect to UTF-8 with no
change to their values.

## Baseline — before optimization

Raw artifact: `artifacts/benchmarks/three-hop-latency-baseline.json`.
Run window `2026-10-17T12:04:55Z`–`2026-10-17T12:30:42Z`. Model preloaded: yes.
n = 20 measured, **0 errors**, but **20/20 runs abstained** (`status=abstained`,
`turns=2`).

| Phase | min | p50 | p95 | max |
|---|---|---|---|---|
| end-to-end (`latency_ms`) | 48,542 | 63,482 | 93,187 | 102,236 |
| `graph_ms` | 15 | 87 | 569 | 903 |
| `model_ms` | 48,473 | 63,447 | 93,110 | 101,984 |
| `validate_ms` | 0 | 0 | 0 | 0 |
| `render_ms` | 0 | 0 | 0 | 0 |

Token usage: 40 model calls, 26,340 input tokens, **29,268 output tokens**
(**732 output tokens per call**) over 20 runs.

Two problems, both measured:

1. **Latency.** `model_ms` is 99.9% of end-to-end time; `graph_ms` is a few tens
   of milliseconds. The bottleneck is model generation, **not** the graph. (The
   handoff note's suggested "13.6 s graph / 4.5 s generation" split was never
   measured; the measured split is the opposite — graph is negligible.)
2. **Correctness.** Every run abstained after only two turns. 732 output tokens
   per call for a strict-JSON selection step is far more than the ~33
   tokens/call the earlier `qwen3:4b-instruct` run averaged, which is consistent
   with the thinking trace consuming the `num_predict` budget before the JSON
   decision was complete, leaving an empty decision. The mechanism is inferred
   from the trace lengths and turnout; the observed behaviour (2 turns, empty
   claims, abstention, every run) is the measured fact.

## Candidate — after optimization

Change: one field, `"think": false`, added to the Ollama `/api/chat` request
body (`internal/agent/ollama.go`). It is inert for models without a thinking
mode. No graph query, evidence contract, citation rule, or prompt changed.
Regression test: `internal/agent/ollama_payload_test.go`.

Raw artifact: `artifacts/benchmarks/three-hop-latency-optimized.json`
(also copied to the canonical `artifacts/benchmarks/three-hop-latency.json`).

Run window `2026-10-17T12:36:24Z`–`2026-10-17T12:41:18Z`. Model preloaded: yes.
n = 20 measured, 0 transport errors. Outcome split: **5 answered, 15
`provider_error`** (`turns=6`, the step budget). All 20 runs used the full
6-step budget: the 5 answered runs produced a citation-valid answer on the last
step; the 15 others exhausted the budget without converging.

| Phase | min | p50 | p95 | max |
|---|---|---|---|---|
| end-to-end (`latency_ms`) | 8,893 | 9,571 | 22,286 | 30,824 |
| `graph_ms` | 93 | 226 | 527 | 610 |
| `model_ms` | 8,647 | 9,227 | 22,188 | 30,319 |
| `validate_ms` | 0 | 0 | 0 | 0 |
| `render_ms` | 0 | 0 | 0 | 0 |

Token usage: 120 model calls, 104,875 input tokens, **4,905 output tokens**
(**41 output tokens per call**), down from 732.

### Why the outcome is split

Traced single runs show the two trajectories directly. In the failing
trajectory the model re-issues `{"action":"retrieve","entity":
"service:payments-api"}` every step; the authorship guard only re-plans a
traversal when the model *answers*, so a model that keeps retrieving is never
re-planned and the loop ends at the step budget. In the succeeding trajectory
the model retrieves `HAS_REPO`, then `MODIFIED`, then `AUTHORED`, then selects an
`AUTHORED` fact, and the answer validates. The 4B model on this CPU host plans
the path unreliably once its thinking trace is removed; that is a model/loop
limitation, exposed rather than introduced by this change.

`provider_error` is **not** a new user-visible failure: `GroundedAsk` sets the
same safe text and empty claims for `abstained` and `provider_error`
(`internal/agent/grounded.go`), so both are a refusal to answer, not a wrong
answer or a crash.

## Guarded — after the progress fix

Change: a deterministic progress guard in the shared agent loop
(`internal/agent/grounded.go`, commit `d0dedca`). A step makes progress only when
it returns evidence not seen before; a repeated scope (compared by resolved
entity key plus canonical, sorted relation scope), a non-`ok` result, or a tool
error does not. On a stall the guard advances along observed edges — an author
question now follows `SUPPORTED_BY` → document → `AUTHORED` as well as
`HAS_REPO` → `MODIFIED` → `AUTHORED` — or broadens the resolved entity to its full
neighborhood once, and abstains with a warning when nothing new remains. Bounded
recovery; no graph query, evidence contract, citation rule, or prompt changed.
Deterministic regression tests: `TestGroundedProgressGuard` (scripted model, no
Ollama), `TestProgressGuardPlan`, `TestAuthorTraversalUsesOnlyReturnedNodes`.

Raw artifact: `artifacts/benchmarks/three-hop-latency-guarded.json`.

Run window `2026-10-17T17:16:31Z`–`2026-10-17T17:22:58Z`. Model preloaded: yes.
n = 20 measured, 0 transport errors, **20/20 answered** (every run produced a
citation-valid answer). Every run converged in **4 model decisions** (down from
6): the model retrieves `HAS_REPO` once, repeats it once, and the guard then
completes `MODIFIED` → `AUTHORED` deterministically before the model selects the
`AUTHORED` fact.

| Phase | min | p50 | p95 | max |
|---|---|---|---|---|
| end-to-end (`latency_ms`) | 13,098 | 15,858 | 20,180 | 22,268 |
| `graph_ms` | 106 | 453 | 2,348 | 2,549 |
| `model_ms` | 12,865 | 14,943 | 19,402 | 21,548 |
| `validate_ms` | 0 | 0 | 0 | 0 |
| `render_ms` | 0 | 0 | 0 | 0 |

Token usage: 80 model calls, 92,680 input tokens, **5,840 output tokens**
(**73 output tokens per call**).

The comparison that matters is **among answers, not across the whole pool**: in
the candidate run the 15 `provider_error`s failed in ~9.6 s (6 steps, no answer),
so its pool p50 of 9,571 ms measures a fast *failure*. The candidate's five
successful answers took 16,136 / 16,639 / 17,056 / 17,338 / 22,286 ms (median
17,056 ms). The guarded run answers **every** time at a median of 15,858 ms —
faster than the candidate's successful median, and with a lower p95, because the
guard reaches `AUTHORED` in 4 decisions instead of 6.

## Before / after

Three measured configurations, same host, same model tag, same question, same
n = 20. Raw artifacts: `three-hop-latency-baseline.json` (thinking on),
`three-hop-latency-optimized.json` (thinking off), `three-hop-latency-guarded.json`
(thinking off + progress guard).

| Metric | Baseline (thinking on) | Candidate (thinking off) | Guarded (thinking off + guard) |
|---|---|---|---|
| answered / citation-valid | 0 / 20 | 5 / 20 | **20 / 20** |
| safe non-answers | 20 / 20 | 15 / 20 (`provider_error`) | **0 / 20** |
| transport errors | 0 | 0 | 0 |
| model decisions / run | 2 | 6 | 4 |
| end-to-end p50 (all runs) | 63,482 ms | 9,571 ms | 15,858 ms |
| end-to-end p95 (all runs) | 93,187 ms | 22,286 ms | 20,180 ms |
| median latency of **answers** | n/a (0 answers) | 17,056 ms | 15,858 ms |
| `graph_ms` p50 | 87 ms | 226 ms | 453 ms |
| output tokens / call | 732 | 41 | 73 |

Reading the table: thinking-off is what makes the question answerable at all, and
the guard is what makes it answerable **every time**. Relative to the candidate,
the guard turns 15/20 failures into answers, cuts the successful-answer median
7%, and lowers p95 — the pool p50 rises only because there are no longer any fast
failures to pull it down.

## Corpus accuracy cross-check

The three-hop question is one case. The whole 21-case corpus was run in both
configurations to check that the change does not reduce accuracy elsewhere
(`artifacts/evaluation/model-eval-2026-10-17.json` and `...-thinking-on.json`;
details in [EVALUATION.md](EVALUATION.md) §6.1):

| Corpus run | rubric pass | citation-valid | answered | abstained | invalid |
|---|---|---|---|---|---|
| thinking on | 16 / 21 | 17 / 21 | 8 | 9 | 3 (plus 1 timeout) |
| thinking off | **18 / 21** | **20 / 21** | 11 | 9 | 1 |
| thinking off + guard, 3 trials | **20 / 21 each** | **21 / 21 each** | 13 | 8 | 0 (no timeout, no error) |
| thinking off + guard + author-path fix, 3 trials | **21 / 21 each** | **21 / 21 each** | 13 | 8 | 0 (no timeout, no error) |

Four cases flipped fail→pass (`ignore-instructions`, `decision-sources`,
`three-hop`, `cross-entity-citation`) and two flipped pass→fail
(`service-repository`, `adversarial-fabricate-person`) between the two single
runs. `decision-source-authors` failed in both. The model is not deterministic,
so these are two single runs: `service-repository` failed in the earlier review
of both configurations and passed here only with thinking on, so its pass→fail
flip is run variance, not a stable regression. The `adversarial-fabricate-person`
flip is a semantic error the rubric caught (the model answered a repository when
asked for the owner), not an ungrounded or citation-invalid answer.

The guard removes both loop failures seen in the single runs (`service-repository`
and `three-hop` now pass in all three trials). The one persistent rubric miss
after the guard was `decision-source-authors`. It was root-caused on 2026-10-18:
the retrieved `decision → SUPPORTED_BY → document` edge was dropped from the
answer, so the answer named the author but not the document's link to the
decision. The shared loop now states the retrieved chain for an explicit author
question (see [EVALUATION.md](EVALUATION.md) §6.3 and
[RELIABILITY_REPORT.md](RELIABILITY_REPORT.md) §2.2), and the case passes 3/3. Full
per-case outcomes are in [RELIABILITY_REPORT.md](RELIABILITY_REPORT.md).

## Interpretation and limits

- **Thinking off** is retained: measured on the same hardware, model tag,
  question, and n, it cut median model time ~6.9×, lowered the safe non-answer
  rate from 20/20 to 15/20, and produced 5 citation-valid answers where the
  baseline produced none.
- **The progress guard** is retained: it turns the candidate's 15/20
  `provider_error`s into 20/20 citation-valid answers, cuts the successful-answer
  median from 17,056 ms to 15,858 ms, and lowers p95 from 22,286 ms to 20,180 ms.
  The candidate's lower *pool* p50 (9,571 ms) was an artifact of fast failures;
  among answers, the guarded run is faster.
- Neither change makes the question reliable by *model planning* alone: the guard
  completes the traversal deterministically, so the reliability comes from the
  deterministic layer, not from the 4B model's planning. That is the documented
  limitation.
- This is **not** a claim that the graph is fast in production, nor that the
  model is accurate in general. It is a single-question, single-host result.
- `validate_ms` and `render_ms` are 0 ms at this resolution (they are
  sub-millisecond here); they are still instrumented so a future change that
  makes them measurable is visible.
- The three-hop abstention in the baseline was a real user-visible failure, not
  just slowness. Its root cause is reported as observed (empty early decision
  with an exhausted output budget); no further mechanism is asserted without a
  measurement.
