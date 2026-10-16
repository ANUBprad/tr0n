# TRON — Evaluation & Quality Assurance

**Status**: Canonical. Authority for how TRON measures quality and trust.
**Last updated**: 2026-10-15.
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
| E1 | **Entity resolution** | pairs of records known to be same/different | merge correctness, no false merges | PARTIAL (2026-10-16: deterministic tiers `TestResolveEntity`; loader rejects cross-load/label duplicate keys `TestDuplicateKeyAcrossLoadsIsRejected`; multi-source merge deferred to M2) |
| E2 | **Multi-hop graph queries** | fixtures answerable only via ≥3 hops | the product thesis, as a test | PARTIAL (seed contract test) |
| E3 | **Source-grounded correctness** | Q/A with known ground-truth answers + sources | answers match evidence | PROPOSED |
| E4 | **Citation support** | answers + their claims | every claim cites a valid path (`Answer.Validate`) | IMPLEMENTED (structural) |
| E5 | **Temporal accuracy** | time-shifted fixtures (owner 2026 vs 2027) | correct as-of results | IMPLEMENTED (2026-10-16): `internal/graph` `TestFindOwnerAsOf`/`TestFindExpertsAsOf`, `internal/graph` `TestLoadEdgesNormalizesOffsetToUTC`, `internal/api` `TestEndpoints/owner`+`/experts`, seed handoff fixture; see §2.2 |
| E6 | **Contradiction detection** | injected conflicting state-relation claims (overlapping half-open windows) + non-conflicting decoys | both surfaced, none hidden; nothing extra flagged | IMPLEMENTED (2026-10-16): `internal/graph` `TestFindConflicts`, `internal/api` `TestEndpoints/conflicts`; scope decided 2026-10-15, see §2.1 |
| E7 | **Permission enforcement** | facts with differing scopes | no derived leakage | PROPOSED |
| E8 | **Recommendation quality** | scenarios rated by humans | useful/correct vs baseline | PROPOSED |
| E9 | **Action authorization** | allow / require-approval / deny fixtures | gate honors policy | PROPOSED |
| E10 | **Duplicate & failed execution** | replayed idempotency keys; injected target failures | executes once; failures classified | PROPOSED |
| E11 | **Proactive alert quality** | events with/without true alerts | precision/recall of alerts | PROPOSED (no proactive feature yet) |
| E12 | **Latency & cost** | workload replay | p50/p95 per op; tokens/$ per answer | PARTIAL (per-op ms captured in UI) |

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
- the loader enforces `key` uniqueness **across loads and labels** (not just
  in-batch): a second load of an existing key is rejected before any write,
  so no two nodes can share a key and the resolver stays unambiguous
  (`TestDuplicateKeyAcrossLoadsIsRejected`). This is application-level
  enforcement of the requirement; the DB-level constraint stays deferred
  (2026-10-15) — no merge, no rewrite, no provenance loss.

Staleness is **not** a suite: freshness is `observed_at`-based and already
returned on every `Provenance`; no stale threshold or label is defined
(decision 2026-10-16, `SCHEMA.md` §5).

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
