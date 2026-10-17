# TRON — Evaluation & Quality Assurance

**Status**: Canonical. Authority for how TRON measures quality and trust.
**Last updated**: 2026-10-17.
**Current-state authority**: testing posture in
[`architecture-minimal.md`](architecture-minimal.md) and the Go test suites
under `cmd/` and `internal/`.
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
| E1 | **Entity resolution** | pairs of records known to be same/different | merge correctness, no false merges | PARTIAL (2026-10-16: deterministic tiers `TestResolveEntity`; adversarial contract suite `TestResolveEntityContract`; loader rejects cross-load/label duplicate keys `TestDuplicateKeyAcrossLoadsIsRejected`; multi-source merge deferred to M2) |
| E2 | **Multi-hop graph queries** | fixtures answerable only via ≥3 hops | the product thesis, as a test | PARTIAL (seed contract test) |
| E3 | **Source-grounded correctness** | Q/A with known ground-truth answers + sources | answers match evidence | PARTIAL (participant semantic review passed 10/10 synthetic samples for the 2026-10-17 run; broader entailment evaluation and valid competing baselines remain open) |
| E4 | **Citation support** | answers + their claims | every claim cites a valid path (`Answer.Validate`) | IMPLEMENTED (structural) |
| E5 | **Temporal accuracy** | time-shifted fixtures (owner 2026 vs 2027) | correct as-of results | IMPLEMENTED (2026-10-16): `internal/graph` `TestFindOwnerAsOf`/`TestFindExpertsAsOf`, `internal/graph` `TestLoadEdgesNormalizesOffsetToUTC`, `internal/api` `TestEndpoints/owner`+`/experts`, seed handoff fixture; see §2.2 |
| E6 | **Contradiction detection** | injected conflicting state-relation claims (overlapping half-open windows) + non-conflicting decoys | both surfaced, none hidden; nothing extra flagged | IMPLEMENTED (2026-10-16): `internal/graph` `TestFindConflicts`, `internal/api` `TestEndpoints/conflicts`; scope decided 2026-10-15, see §2.1 |
| E7 | **Permission enforcement** | facts with differing scopes | no derived leakage | PROPOSED |
| E8 | **Recommendation quality** | scenarios rated by humans | useful/correct vs baseline | PARTIAL (deterministic calculator and input validation; no graph-derived scores or human scenario ratings) |
| E9 | **Action authorization** | allow / require-approval / deny fixtures | gate honors policy | PARTIAL (authenticated local drafts only; state-changing actions are denied) |
| E10 | **Duplicate & failed execution** | replayed idempotency keys; injected target failures | executes once; failures classified | PARTIAL (single-process in-memory draft replay only; no durable log or target failure adapter) |
| E11 | **Proactive alert quality** | events with/without true alerts | precision/recall of alerts | PARTIAL (snapshot rules; no labeled alert corpus or measured precision/recall) |
| E12 | **Latency & cost** | workload replay | p50/p95 per op; tokens/$ per answer | PARTIAL (per-op ms captured in UI) |

**Note (2026-10-17)**: M3 retrieval / context assembly is implemented and tested
(§2.4). It is the evidence-selection precondition for E3/E4 and does not by
itself change their status; the separate E3 grounding contract (§2.5) moves E3
to PARTIAL (structural linkage only).

### 2.1 E6 scope and v1 checklist (decided 2026-10-15; implemented 2026-10-16)

E6 v1 covers **state relations only** (`WORKS_IN, OWNS, WORKED_ON, HAS_REPO,
DEPENDS_ON`); event edges and node-property disagreements are out of scope.
Compatibility is relationship-specific (`OWNS` exclusive per target; others
only if `GRAPH_MODEL.md` declares exclusivity; same counterpart = corroboration),
and intervals are half-open (`SCHEMA.md` §5–§6, `planning/decision-log.md`).
Implemented as `graph.FindConflicts` (`contradiction/v1`) and exposed at
`POST /v1/conflicts`.

Runnable suite demonstrates — `TestFindConflicts` (`internal/graph`),
`TestEndpoints/conflicts` (`internal/api`), both real-FalkorDB and skipped
under `-short`:

- overlapping-but-touching half-open intervals do **not** flag; genuinely
  overlapping incompatible claims **do**;
- every flagged edge carries full provenance (`src_ref`, `valid_from`,
  `valid_to`);
- the baseline (unmodified) seed graph yields **zero** conflicts — no false
  positives on shipped data;
- event-edge and node-property decoys are never flagged;
- a state edge whose window cannot be parsed is omitted from detection (the
  loader rejects a missing `valid_from` at write); it is never treated as
  always-valid;
- exposure via `POST /v1/conflicts`: invalid `relation` → 400, empty result →
  `200 {"conflicts":[]}`, deterministic ordering of groups and members;
- runs against live FalkorDB like the other op suites (skipped under `-short`).

### 2.2 E5 scope and v1 checklist (implemented 2026-10-16)

Predicate is the locked half-open rule (`SCHEMA.md` §5):
`valid_from <= as_of AND (valid_to IS NULL OR valid_to > as_of)`. FalkorDB
has no `datetime()` function (verified against the running server), so
windows compare as canonical RFC3339 UTC strings; the loader normalizes every
`valid_from`/`valid_to` to that form on write, and an omitted `as_of` uses a
far-future sentinel that reproduces `valid_to IS NULL` exactly. Scope: the
two state reads that expose ownership — `FindOwner` and the `OWNS` branch of
`FindExperts`; **event** reads (e.g. incidents) stay event-time, never
validity-time.

Runnable suite demonstrates — `TestFindOwnerAsOf` / `TestFindExpertsAsOf`
(`internal/graph`), `TestLoadEdgesNormalizesOffsetToUTC` +
`TestLoadEdgesComparesInstantsNotStrings` (`internal/graph`),
`TestEndpoints/owner` + `TestEndpoints/experts` (`internal/api`), all
real-FalkorDB and skipped under `-short`:

- `valid_from` is inclusive (as-of its instant returns the owner),
  `valid_to` is exclusive (as-of its instant returns nobody that era);
- a gap between eras returns no owner; an open-ended window
  (`valid_to IS NULL`) holds for every `as_of` after its `valid_from`;
- an instant before every window returns nobody;
- omitting `as_of` returns exactly the current owner (unchanged behavior);
- a non-UTC offset and its UTC equivalent store as the **same** value, so the
  lexicographic comparison is chronologically correct;
- an **empty window is rejected at write**: `valid_to` must be strictly after
  `valid_from`, compared as instants (equal endpoints, incl. equal moments
  written with different offsets, error instead of storing a never-matching
  `[t, t)`);
- historical reads still ship full provenance; `as_of` ranks ownership in
  `FindExperts` while `now` still drives decay;
- `POST /v1/owner` and `/v1/experts` accept optional `as_of`; malformed →
  400.

### 2.3 E1 identity checklist (single-source, implemented 2026-10-16)

No multi-source merge exists yet, so E1 cannot be "complete" (merge
correctness needs a second source). What is runnable and locked now
(`internal/graph`, real FalkorDB, skipped under `-short`):

- deterministic tiers, first hit wins, no fuzzy match:
  `key` → `name`/`title` → case-insensitive exact → prefix
  (`TestResolveEntity`);
- an ambiguous tier returns the **candidate list**, never a single guessed
  key, and an empty query returns nothing (`TestResolveEntity`);
- database constraints enforce `key` uniqueness per label; application writes
  additionally check across labels. The loader rejects a second load of an
  existing key before any write (`TestDuplicateKeyAcrossLoadsIsRejected`).
  Direct external writers can still violate the cross-label invariant. No merge,
  rewrite, or provenance loss is performed.
- adversarial contract suite `TestResolveEntityContract` (9 subtests, live
  FalkorDB): stable `key` evidence vs weak `name`/`title`, label/kind
  narrowing, no auto-select of a candidate, determinism across graph
  insertion order, idempotent read-only resolution, and provenance + E5
  temporal windows surviving resolution unchanged;
- no caller creates or merges nodes without strong identity evidence:
  `LoadNodes` is the only node-creating path in production code, and no
  caller of `ResolveEntity` (`/v1/resolve`, `resolve_entity`) ever picks
  `Candidates[0]`;
- the one identified risk — a key-shaped string that misses its `key` still
  resolving via an exact name/title coincidence — is **recorded, not fixed**
  (`OPEN_QUESTIONS.md`, `decision-log.md` 2026-10-16): `keyShapeOK` cannot
  separate a key from a colon-containing title, so tightening it would change
  canonical identity semantics. Current behavior is locked by
  `TestResolveEntityContract/key-shaped_input_still_resolves_by_exact_title`.

Staleness is **not** a suite: freshness is `observed_at`-based and already
returned on every `Provenance`; no stale threshold or label is defined
(decision 2026-10-16, `SCHEMA.md` §5).

### 2.4 M3 retrieval / context assembly (implemented 2026-10-16)

`graph.AssembleEvidence` (the retrieval half of `REASONING.md` §2) is runnable
and locked on real FalkorDB. It is the evidence-*selection* precondition for
E3/E4 and does **not** by itself make them pass (no Q/A grounding, no answer
rendering). What is proven now (`internal/graph` `evidence_test.go`, skipped
under `-short`):

- **Relevance, no leak**: facts anchored on the resolved entity are returned
  with provenance and validity windows; the entity's own neighbouring facts on
  *other* entities do not leak in; the call mutates nothing (node/edge counts
  unchanged, no `INFERENCE`/`rule_id` introduced);
- **Canonical half-open `as_of`**: `valid_from <= as_of < valid_to` — inclusive
  at `valid_from`, exclusive at `valid_to` (the handoff instant belongs to the
  later owner only); checked at the boundary and immediately either side of it;
  an offset-equivalent instant (`05:30+05:30` == `00:00Z`) is the same boundary;
  the echoed `as_of` is canonical UTC; omitted `as_of` = full history; event
  relations (no window) are never window-filtered;
- **Explicit outcomes, no guessing**: ambiguous resolution returns the candidate
  list with no facts; unknown → `not_found`; resolved-but-no-in-scope-fact →
  `empty`; unsatisfiable byte bound → `over_budget` (not `ok`); unknown/mistyped
  `relations` filter → `empty` (a scope, never a silent no-filter); malformed
  `as_of` → error (never a silent no-filter); blank entity → error;
- **Bounds + truncation**: count and serialized-size caps are enforced and each
  reason is reported (`count`, `size`, or `count+size`); a single fact larger
  than `max_bytes` yields `over_budget` with `facts` empty, never `ok` with no
  facts; defaults do not truncate normal results;
- **Store-order independence**: the same facts seeded in reverse insertion order
  produce byte-identical context; the ordering is a **total** order (facts equal
  on the primary keys still have one order — `TestAssembleEvidenceOrderIsTotal`);
- **Malformed stored provenance**: a state edge with an unparseable window
  (impossible through `LoadEdges`, possible if written around it) is excluded
  under `as_of`, never treated as always-valid
  (`TestAssembleEvidenceExcludesMalformedStoredWindow`).

End-to-end integration: `internal/agent`
`TestAssembleContextToolSuppliesEvidence` drives the loop with the
`assemble_context` tool, asserts bounded/provenanced facts reach the transcript,
and builds a `knowledge.Answer` claim from a returned fact path whose
`Validate()` passes — the string-path bridge to the answer layer (superseded by
the structural grounding contract in §2.5). Its
subtests also drive the explicit outcomes and bounds **through the loop**:
unknown entity → `not_found`; resolved-but-no-in-scope-fact → `empty`; unknown
relation filter → `empty`; malformed `as_of` → turn error; unsatisfiable byte
bound → `over_budget`; ambiguous name → candidates and no pick; missing
`entity` → turn error; count bound reported. The scripted model is
predetermined, so these prove **tool behavior and transcript flow**, not that a
real model refrains from unsupported claims — end-to-end grounding (E3) stays
PARTIAL (structural only; see §2.5).

### 2.5 E3 source-grounded answer contract (implemented 2026-10-16)

`graph.GroundedAnswer` (`internal/graph/grounded.go`) binds a `knowledge.Answer`
to the `EvidenceContext` it was built from: each claim's `ClaimGrounding` names
the subject entity, the relationship (when asserted), and the IDs of the
retrieved `EvidenceFact`s that support it. `EvidenceFact.ID` is a stable,
content-derived identifier, so a citation targets a specific fact rather than a
position. `GroundedAnswer.Validate` runs `knowledge.Answer.Validate` first and
then checks the linkage. `knowledge.Answer`/`Claim`/`Validate` are **unchanged**,
so every existing composer, API response, and the agent bridge stay compatible.

**What this proves (structural grounding)** — `internal/graph` `grounded_test.go`
(`TestGroundedAnswer*`; the pure cases run under `-short`, the retrieval cases
are skipped under `-short`):

- **Valid citation**: an answer built from a real `AssembleEvidence` result — a
  fact cited by its ID — validates, and the fact's structured provenance
  (`src_*`, `valid_from`/`valid_to`) is preserved verbatim in the envelope and
  across a JSON round trip (`TestGroundedAnswerValidate/association_survives_a_JSON_round_trip`,
  `TestGroundedAnswerFromRetrieval`);
- **Fabricated / unknown reference rejected**:
  `.../fabricated_evidence_id_is_rejected`;
- **Evidence for another relationship or entity rejected**: a fact of a
  different `relation`, or anchored to a different entity than the claim's
  subject, cannot support the claim
  (`.../evidence_of_a_different_relationship_is_rejected`,
  `.../evidence_anchored_to_another_entity_is_rejected`);
- **Missing evidence rejected**: a FACT claim citing no fact, or no subject, and
  a grounding/claims length mismatch, all fail
  (`.../a_FACT_claim_citing_no_evidence_is_rejected`,
  `.../a_FACT_claim_with_no_subject_is_rejected`, `.../grounding_and_claims_must_align`);
- **Explicit outcomes cannot become FACT claims**: `not_found`, `ambiguous`,
  `empty`, and `over_budget` are each distinguishable and each refused as
  backing for a FACT claim (`TestGroundedAnswerExplicitOutcomes`); INFERENCE
  claims keep their own `rule_id`/path discipline and are not fact-grounded;
- **Truncation preserved, not exhaustive**: a truncated-but-`ok` context still
  validates and the `truncated` / `truncated_reason` are carried unchanged
  (`TestGroundedAnswerPreservesTruncation`);
- **Deterministic citability**: the same fact has the same ID on every retrieval
  (`TestGroundedAnswerFromRetrieval`).

**What this does NOT prove (and why E3 stays PARTIAL)** — the contract tests that
  a claim's cited evidence *exists* and *matches* the claim. The wired local model
  now selects IDs from retrieved facts; the server renders the canonical endpoints
  and relationship, preventing the model from relabeling an `OWNS` edge as a salary
  or job-title fact. The small rubric corpus measures expected terms, retrieval
  paths, and abstention, but it is not an entailment judge. Participant review
  passed on ten synthetic samples; broader semantic evaluation and comparison
  baselines remain open. A valid evidence reference is a necessary
  precondition, not sufficient evidence of question relevance. **Absence** claims ("no `OWNS` edge")
have no `EvidenceFact` to cite and remain outside the contract (operation-
reference string path); recorded in `OPEN_QUESTIONS.md`.

### 2.6 Adversarial grounded-answer suite (added 2026-10-17)

The original ten labeled cases are preserved. A labeled adversarial suite is
added in `internal/evaluation/model.go` (fictional synthetic data only) and
covers every category the quality pass requires: conflicting sources, ambiguous
entities, irrelevant evidence, stale/superseded sources, historical accuracy
(including the half-open interval), adversarial source content, unsupported
questions, cross-entity/cross-relation citation, conflicting temporal facts, and
multi-hop grounding. Each case carries a stable `id`, a `category`, an explicit
`rubric` rule, expected/forbidden terms, required relations/sources, a
minimum-context count, and an expected status/abstention flag.

The rubric is a **pure function** (`evaluation.Rubric`) so it is tested without
a model: `internal/evaluation/rubric_test.go` feeds it a response containing the
expected words but an unknown/fabricated citation, a citation borrowed from a
different entity, a forbidden term, and a wrong status, and asserts each is
rejected — the rubric cannot pass on word-matching alone. `fixture.go` +
`fixture_test.go` add and re-check the real-graph conditions the suite depends on:
two incompatible live `OWNS` claims (conflict), a superseded ownership handoff
whose closed window must never read as current, the seed handoff boundary at the
half-open instant, an ambiguous name that must stay ambiguous, and the
source-freshness contract (older observation refused, same-time different content
refused, identical retry is a no-op, newer content accepted).

What the corpus still does **not** provide: an entailment judge, a comparison
against keyword/RAG/no-graph baselines, or independent human semantic review (see
§6). `citation_passed` is a structural check and is reported separately from
relevance.

## 3. Baselines and comparison

TRON must be compared, on the same fixture corpus, against simpler systems so
the graph's value is measurable rather than asserted:

| Baseline | What it represents | Why include |
|---|---|---|
| **Keyword search** | grep/BM25 over documents | shows graph beats text matching on relational questions |
| **Conventional RAG** | chunk + embedding retrieval + LLM | the standard alternative the product thesis argues against |
| **Tool-using LLM without memory/evidence** | LLM with generic tools, no graph | isolates the contribution of memory + provenance |
| **TRON (graph + provenance)** | the system under test | — |

### Comparative benchmark status

No comparative baseline implementation or measurements are included. Earlier architecture scores were hard-coded examples, not benchmark results, and have been removed from the executable and this report. Do not use them to claim TRON outperforms search or RAG.

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

## 6. Local model evaluation (measured 2026-10-17)

`go run ./cmd/tron eval-model` seeds the disposable `tron_model_eval` graph and
runs the corpus in `internal/evaluation/model.go` through the configured local
Ollama model. The current run used `qwen3:4b-instruct`. It exercises current and
historical owners, decision sources, repository lookup, three-hop authorship,
missing entities, ambiguous names, unsupported financial/salary questions, and
an instruction to invent an owner. Each request is bounded by model/tool budgets.
Raw JSON and trace output for the earlier 10-case run are saved in the root-level `model-evaluation-submission.*`
files locally; they are not committed because they contain machine-specific
latency/token counts and raw synthetic model responses. The expanded 21-case
run in §6.1 **is** committed under `artifacts/evaluation/` (its content is
entirely synthetic; the machine-specific latency/token counts are labelled as
such).

The report distinguishes rubric passage, citation structure, answer coverage,
claim counts, latency, and token use. `citation_passed` includes abstention
envelopes, so it must not be read as an answer-accuracy score. The stricter
`valid_claim_count/claim_count` checks nonempty claim envelopes only; neither
proves relevance. Latency varies with GPU/CPU offload and cold starts; hardware
cost is not estimated. Participant semantic review for [the answer samples](MODEL_REVIEW.md)
passed 10/10 on 2026-10-17, completing the M3 sample-review gate for this run.
This does not establish broad model accuracy or production readiness.

The 2026-10-17 run passed the automated term/path/abstention rubric in 10/10
cases, with 10/10 structurally valid envelopes and 10 selected claims with
valid evidence IDs. It used 20 local model calls and reported 15,193 input and
658 output tokens. Per-question latency was 947–19,227 ms (median 2,359 ms;
nearest-rank p95 19,227 ms). One three-hop case accounts for the long tail.
These figures describe this single run and this GPU/CPU setup only. Broader
semantic evaluation beyond this synthetic sample remains open.

### 6.1 Expanded adversarial corpus — 2026-10-17 run (`qwen3:4b`)

**Model tag.** This run uses the *installed* tag `qwen3:4b`, not the documented
`qwen3:4b-instruct` (which is not installed). The bare tag ships with *thinking*
on by default. The corpus was run twice: once as-is, and once with the model's
thinking trace disabled (`"think": false` on the Ollama request — see
[LATENCY_REPORT.md](LATENCY_REPORT.md)). The two configurations are reported
side by side; figures are never compared across a different corpus or model tag.

**Corpus.** 21 cases = the ten original cases (ids preserved) plus eleven
adversarial cases covering the §2.6 categories: conflicting sources, ambiguous
entities, irrelevant evidence, stale/superseded ownership, historical boundaries
(`valid_from` inclusive / `valid_to` exclusive), adversarial source content,
unsupported questions, cross-entity citation, conflicting temporal facts, and
multi-hop grounding. Raw artifact: `artifacts/evaluation/model-eval-2026-10-17.json` (thinking off) and
`...-thinking-on.json`.

```powershell
$env:TRON_OLLAMA_URL="http://localhost:11434"; $env:TRON_OLLAMA_MODEL="qwen3:4b"
go build -o tron.exe ./cmd/tron
cmd /c ".\tron.exe eval-model > artifacts\evaluation\model-eval-2026-10-17.json"
```

(Use `cmd /c` redirection: Windows PowerShell 5.1 `>` writes UTF-16LE. The
committed artifacts were re-encoded to UTF-8 with no change to their values.)

| Run | rubric pass | citation-valid | answered | abstained | invalid | timeout | error |
|---|---|---|---|---|---|---|---|
| `qwen3:4b`, thinking on (default) | 16 / 21 | 17 / 21 | 8 | 9 | 3 | 1 | 0 |
| `qwen3:4b`, thinking off | **18 / 21** | **20 / 21** | 11 | 9 | 1 | 0 | 0 |

`citation_valid` and `rubric_pass` are reported separately: a response can carry
a structurally valid citation envelope and still be semantically wrong, so the
rubric is the independent check. Disabling thinking improved rubric pass (+2),
citation validity (+3), answered (+3), invalid (−2), and removed the single
timeout.

The model is not deterministic, so this is **one run per configuration, not a
sampling comparison**. Per-case changes (rubric): `ignore-instructions`,
`decision-sources`, `three-hop`, and `cross-entity-citation` flipped fail→pass;
`service-repository` and `adversarial-fabricate-person` flipped pass→fail;
`decision-source-authors` failed in both configurations and the new
`superseded-owner` case passed in both. `service-repository` and
`decision-source-authors` are loop-non-convergence cases for this 4B model (it
repeats a retrieval until the step budget is exhausted); both also failed in the
earlier 20-case review, and `service-repository` passing here only with thinking
on shows the outcome is run-dependent — neither is read as a regression of the
`think` change. `three-hop` stays unreliable across repeated runs (5/20 answered
in the 20-run benchmark); the single pass here is not a reliability claim.

`adversarial-fabricate-person` is instructive: with thinking off the model
followed a real relationship but answered with a repository instead of the owner
(`payments-api — has repo → payments-engine`). The citation was structurally
valid and the rubric still failed it, because the expected owner term was
absent. That is the rubric working as designed — citation validity is not
entailment.

No human semantic review was performed for this expanded run; the counts above
are automated rubric results only.

## 7. What is explicitly NOT claimed yet

- No broad accuracy claim exists. The labeled corpus is small and uses a simple
  expected-term/abstention rubric. Latency and token counts are per-run local
  measurements, not a representative workload or cost estimate.
- The Markdown adapter is a bounded local snapshot reader, not a live Slack,
  ticketing, or document-service connector.
- A participant reviewed the ten synthetic answer samples and marked 10/10 correct and relevant; broader semantic evaluation and keyword/RAG/no-graph LLM baselines remain open.
- The demo's pass/fail checks (`cmd/tron/demo.go`) prove that operations
  *answer*, not that the answers are *correct vs a baseline*.

## 8. Definition of done for the evaluation milestone

- At least E1, E2, E3, E4, E5, E6, E9, E10 have runnable suites with
  fixtures.
- The four baselines are implemented enough to run on the corpus.
- A published report states corpus, baselines, metrics, and thresholds.
- CI enforces the hard gates (E4, E7, E9, E10) on every change.
