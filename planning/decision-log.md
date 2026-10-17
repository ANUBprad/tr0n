# DECISION_LOG.md

**Purpose**: Record every architecture decision with rationale.

## Format
Record each decision as:
- **Date**: YYYY-MM-DD
- **Decision**: What was decided
- **Why**: Rationale
- **Alternatives Considered**: Other options
- **Tradeoffs**: Pros/cons
- **Owner**: Who decided
- **Status**: PROPOSED/DECIDED/REVISED

## Initial Decisions (Foundation)

| Date | Decision | Why | Alternatives | Tradeoffs | Owner | Status |
|---|---|---|---|---|---|---|
| 2026-10-08 | **TRON naming & scope: Track 03 focus** | Best fit for graph-central "Company Brain" with provenance | Broader scope across all tracks | Focus vs flexibility | Team | PROPOSED |
| 2026-10-08 | **FalkorDB as primary graph DB** | Required by hackathon; graph must be central | Other graph DBs | Compliance + fit; constraint is fixed | Team | PROPOSED |
| 2026-10-08 | **Deliberate graph retrieval (no full graph dump)** | Core differentiator; evidence-driven | Dump graph to context | Accuracy/provenance vs simplicity | Team | PROPOSED |
| 2026-10-08 | **Synthetic-only data** | Compliance with data restrictions | Real org data (impossible safely) | Safe but needs generation | Team | PROPOSED |
| 2026-10-08 | **Evidence/provenance mandatory** | Explainability; Track 03 alignment | Answer-only | Trustworthy but more work | Team | PROPOSED |
| 2026-10-08 | **72h-optimized minimal scope** | Feasible for 2–3 people | Feature-rich | Reliable demo vs breadth | Team | PROPOSED |
| 2026-10-08 | **Python/FastAPI + FalkorDB client (PROPOSED)** | Fast iteration, client available | Other stacks | TBD; confirm at start | Team | REVISED — superseded by Go lock below |
| 2026-10-08 | **Pre-hackathon = planning only** | Compliance with hackathon rules | Start early | Safe, avoids rule issues | Team | PROPOSED |

## Architecture Lock (2026-10-08) — Status: DECIDED

| Decision | Why | Alternatives | Tradeoffs | Status |
|---|---|---|---|---|
| **Go as implementation language** | Static cross-compiled binaries, stdlib HTTP, goroutines fit I/O-bound ingestion/API, fast iteration, one-dependency posture (Redis client) | Rust (perf, but slower iteration for CPU-bound work FalkorDB already does), Python/TS (runtime weight), Java/C# (heavier deploy) | Go gives up Rust's peak perf/embeddability — reserved for profiled hotspots only, as separate processes (never cgo, to keep `CGO_ENABLED=0`) | DECIDED |
| **Direct FalkorDB client; no graph interface/abstraction layer** | One implementation exists; an interface now is a GraphProvider-shaped speculation (AGENTS.md rung 1). All Cypher confined to `internal/graph` | GraphProvider/Manager/Repository/Factory | If a second graph DB ever appears, the seam is the package boundary | DECIDED |
| **Package layout: `cmd/tron`, `internal/{graph,knowledge,api,ingest,agent}` — created only when code exists** | Fewest files/packages; no empty scaffolding | Full monorepo scaffolding up front | Layout may grow; growth is cheap, deletion is not | DECIDED |
| **Five-layer knowledge model: SOURCE→FACT→INFERENCE→RECOMMENDATION(+ACTION), no silent promotion** | Core trust property; forces provenance to be structural, not cosmetic | Flat "everything is a claim" model | More types in responses; worth it — this is the product thesis | DECIDED |
| **Provenance minimum: src_type, src_ref, observed_at, extraction, valid_from/valid_to (+confidence only if non-deterministic)** | Exactly what "why do you believe this?" needs; author/paths modeled as edges, not duplicated properties | Larger provenance records (author, supporting_relationships inline) | Dropped fields are still answerable via the graph itself | DECIDED |
| **Graph stores facts only; inferences/recommendations computed at query time, never written** | Prevents inference→fact drift at the storage layer, not just the UI layer | Materialized inference edges | Recomputation cost — trivial at this scale (ponytail: upgrade path = cached score nodes still tagged INFERENCE) | DECIDED |
| **Expertise = deterministic weighted graph signals (expertise/v1), LLM never ranks** | Reproducible, testable, explainable ranking; same graph+clock → same answer | LLM-judged expertise, embeddings, centrality algorithms | Hand-tuned weights (versioned); PageRank deferred | DECIDED |
| **Temporal = valid_from/valid_to windows on mutating edges; append-only close+open updates** | Simplest model that never treats 2026-owner and 2027-owner as simultaneously current | Temporal DB, versioned nodes, transaction-time systems | Single UTC clock, edges-only history (ponytail: upgrade path documented) | DECIDED |
| **Initial schema: 11 entities, 16 relationships (cut: KNOWS, RELATED_TO, CAUSED, MEMBER_OF, IMPLEMENTS; added: HAS_REPO, ABOUT, USES)** | Every relationship must be an explicit source-derived fact or it is not a fact | Broad 14-entity/17-relationship candidate schema | Deferred entities re-enter only when a real query needs them | DECIDED |
| **Graph boundary = 5 deterministic operations + ResolveEntity; prepare_handoff deferred to agent layer** | Small, testable, no LLM required for v1 core; compositions belong above the boundary | 7-tool surface incl. fuzzy search + handoff | Fuzzy/entity search deferred until an embedding story exists | DECIDED |
| **Test against real FalkorDB in Docker; no mock graph interfaces** | The graph's behavior is the behavior under test | Interface + mock layer | Tests need Docker; correctness is worth it | DECIDED |
| **FalkorDB Go client: `falkordb-go/v2`** | Official, BSD-3, graph-aware API (`SelectGraph`, parameterized `ROQuery`, typed records); one graph client from the code's point of view (`go-redis/v9` is transitive) | `rueidis` (generic Redis client → hand-rolled `GRAPH.QUERY` parsing) | falkordb-go is young (25 stars); API surface is small and replaceable behind `internal/graph` | DECIDED |
| **Synthetic seed: deterministic Go generator (`internal/seed`) with direct bulk load; wipe-and-recreate** | One language (Go-only stack); `Generate()` is pure and tested for determinism + provenance invariants; Cypher stays in `internal/graph` per schema lock | Python/external script (second runtime for zero gain), JSON → transform → Cypher intermediate artifact (YAGNI) | Seeding is destructive recreate-only, not an ingestion pipeline — real ingestion arrives with real sources (`internal/ingest`); upgrade path: emit a Cypher dump if outside-repo sharing ever needs it | DECIDED |
| **Agent layer = custom minimal tool-calling loop (`internal/agent`); model behind one `Decide` function** | OPEN_QUESTIONS preferred minimal loop over a framework; the provider is still an open decision, so the core binds to nothing (a provider SDK is one adapter function away) | Agent libraries (LangChain-style dependency + lock-in), embedding a provider SDK in the core (commits the provider decision prematurely) | When a provider is chosen, its message format is translated inside the adapter — the loop, tools, and transcript stay provider-agnostic | DECIDED |

## M1 Readiness Decisions (2026-10-15) — Status: DECIDED (implementation: E6 landed 2026-10-16, addendum below)

Recorded per the promotion rule in `docs/OPEN_QUESTIONS.md` and the
change rules in `docs/RULES.md` §6 (a decision resolving an open item or
amending a locked boundary is recorded here with its reason). Every row
is a *decision*; the corresponding code, tests, and endpoints are
**PROPOSED** until they exist and pass (`opencode.md` rules 9–10).

| Decision | Why | Alternatives | Tradeoffs / limitations | Status |
|---|---|---|---|---|
| **Defer database-level `key` uniqueness enforcement; v1 accepts loader-/ingestion-enforced uniqueness** | The shipped index is non-unique and only the bulk loader's *in-batch* check rejects duplicates (`internal/graph/load.go` `Reset`, `validateNode`); a DB constraint is unproven here and blocks nothing in M1 | DB unique constraint/index if FalkorDB supports one; ingestion-enforced only | **Known limitation, not a relaxation**: the non-unique index + in-batch check do **not** prevent duplicate `key` nodes across separate loads; the unique-per-label requirement stands (`docs/GRAPH_MODEL.md` Indexes). Duplicate-key nodes can merge or mask an E6 conflict group (edges are grouped by key regardless of node multiplicity). Revisit a DB constraint before multi-source ingestion (M2) | DECIDED (deferral) |
| **E6 v1 scope: incompatible *state*-relation claims with overlapping validity only; event relationships and node-property conflicts excluded** | Matches the canonical contradiction rule (`docs/SCHEMA.md` §6, `docs/REASONING.md` §4); event edges carry no windows (`docs/GRAPH_MODEL.md`), and node-property disagreement is identity/E1 territory (`docs/SCHEMA.md` §9) | Include event edges; include node-property conflicts; detect all differing-target claims | Narrow scope under-contradicts by design (documented ceiling); contradiction stays a *derived, query-time* result, never stored (`docs/SCHEMA.md` §4, A9) | DECIDED (scope) |
| **Relationship-specific compatibility: `OWNS` exclusive per target; other state relations exclusive only where the canonical schema declares it; same counterpart = corroboration** | Literal same-subject/different-object grouping false-positives on shipped seed data (a person `OWNS` a service *and* a repo; multi-project `WORKED_ON` — `internal/seed/seed.go`) | Literal subject-side grouping for every state relation; a config table of cardinalities | Exclusivity must be read from `docs/GRAPH_MODEL.md` (only `OWNS` declares it in v1); making another relation exclusive requires that schema row to say so first (recorded there) | DECIDED (semantics) |
| **Validity intervals are half-open `[valid_from, valid_to)`; touching endpoints do not overlap; missing `valid_from` = integrity anomaly** | Matches the canonical as-of predicate `valid_to IS NULL OR valid_to > t` (`docs/knowledge-model.md`, `docs/SCHEMA.md` §5) and makes every close+open handoff (`valid_to == next valid_from`) non-contradictory | Closed intervals; treating missing `valid_from` as infinitely old | Half-open means a shared endpoint instant belongs to the later interval only; anomalous rows are surfaced as integrity errors, never guessed at | DECIDED (intervals) |
| **First E6 exposure is the read-only public operation `POST /v1/conflicts`, expanding the graph-operation boundary (5 + 1 → 6 + 1)** | The API is the product surface (`internal/api`); read-only preserves the single-writer invariant (`docs/knowledge-model.md` invariant 1); the same handler conventions apply as for the six shipped ops | UI/CLI-only exposure; defer the API to a later milestone; agent-tool-only exposure | Amends locked D7 (5 ops + ResolveEntity): reason recorded here per `docs/RULES.md` §6, contract added in `docs/AGENT_SPEC.md`, `docs/TECHSPEC.md` six-ops line amended. Expansion decided; implementation landed 2026-10-16 (`graph.FindConflicts`, `POST /v1/conflicts`, E6 tests) | DECIDED (boundary); implementation IMPLEMENTED (2026-10-16) |

### E6 implementation landed (2026-10-16)

The readiness decisions above are unchanged; this records the code that
implements them, with evidence (`opencode.md` rules 9–10: a status changes
only with real code and passing tests).

| Change | Where | Evidence |
|---|---|---|
| `FindConflicts(relation, key)` — `contradiction/v1`, state relations only, `OWNS` exclusive per target, half-open overlap, deterministic group/member order, per-edge provenance, derived at query time (never stored, A9) | `internal/graph/graph.go` | `internal/graph/graph_test.go` `TestFindConflicts` |
| Read-only `POST /v1/conflicts` — empty = `200 {"conflicts":[]}`, unknown `relation` = 400 | `internal/api/api.go` | `internal/api/api_test.go` `TestEndpoints/conflicts` |
| Window validation hardened: `valid_from`/`valid_to` parsed independently (a malformed `valid_from` with no `valid_to` is now rejected); RFC3339 offsets accepted (parsed per endpoint) | `internal/graph/load.go` `validateEdge` | `internal/graph/load_test.go` |
| `recProvenance`/`provFrom` carry `valid_to` | `internal/graph/graph.go` | `TestFindConflicts` provenance assertions |

Baseline seed yields zero conflicts (no false positives); a missing or
unparseable `valid_from` on a state edge is omitted from detection, never
treated as always-valid. (E5 temporal as-of querying was out of the E6 slice
and PROPOSED at that time; it has since landed — see the next section.)

### E5 implementation landed (2026-10-16)

E5 (temporal as-of querying) is now **IMPLEMENTED**. The locked predicate and
half-open intervals (see the interval decision above, `SCHEMA.md` §5) are
unchanged; this records the design choices the code needed. Evidence:
`internal/graph` `TestFindOwnerAsOf`/`TestFindExpertsAsOf`/
`TestLoadEdgesNormalizesOffsetToUTC`, `internal/api` `TestEndpoints/owner`
+`/experts`, `internal/seed` `TestSeedServesFiveQueries/find_owner`.

| Decision | Why | Alternatives | Tradeoffs / limitations | Status |
|---|---|---|---|---|
| **Window endpoints are normalized to canonical RFC3339 UTC (`…Z`) on write** | FalkorDB has no `datetime()` function (verified against the running server), so as-of predicates compare windows as **strings**; only canonical same-format strings order lexically the same as chronologically (an offset like `05:30+05:30` would otherwise sort after a later `Z` instant) | Store offsets as sent; keep a numeric epoch property; upgrade the DB | Rewrites the source's offset to UTC, so the *stored* string differs from the input offset (the instant is preserved). `internal/graph/load.go` `validateEdge`, `TestLoadEdgesNormalizesOffsetToUTC` | DECIDED (normalization) |
| **Omitted `as_of` uses a far-future sentinel (`9999-12-31T23:59:59Z`)** | One parameterized query serves both current and historical reads: at a far-future instant the half-open predicate is exactly `valid_to IS NULL`, so current-only behavior is preserved with no null/bool branch | `WHERE $as_of IS NULL OR …`; two separate queries | The sentinel is a magic value (documented in `internal/graph/graph.go` `openEndedAsOf`); any real `as_of` before year 9999 is unaffected | DECIDED (default) |
| **`as_of` is exposed read-only on `/v1/owner` and `/v1/experts` only; variadic so existing callers are unchanged** | These are the two read ops that surface ownership validity. Event reads (e.g. incidents) keep event-time; mixing them would conflate validity-time with event-time | Add `as_of` to incidents/decision; a new dedicated as-of op | Only ownership is as-of-queryable in v1 (documented ceiling); malformed `as_of` → 400 | DECIDED (scope) |

## E5 correctness fix + E1 loader-enforcement follow-up (2026-10-16)

Follow-up to the 2026-10-15 readiness decisions and the E5 implementation.
Evidence: `internal/graph/load_test.go` (`TestLoadEdgesRejectsInvalidBatch` equal-instant cases,
`TestLoadEdgesComparesInstantsNotStrings`, `TestDuplicateKeyAcrossLoadsIsRejected`).

| Decision | Why | Alternatives | Tradeoffs / limitations | Status |
|---|---|---|---|---|
| **Empty validity windows are rejected: `valid_to` must be strictly after `valid_from`** | The half-open predicate `valid_from <= as_of < valid_to` matches nothing when the endpoints are equal, so a zero-length window `[t, t)` is an unusable fact. `validateEdge` previously rejected only `valid_to` *before* `valid_from` (`vt.Before(vf)`), letting equal instants through. The comparison is on parsed instants, so two equal moments written with different RFC3339 offsets are caught | Keep accepting equal endpoints (a never-matching fact); reject on raw string comparison | Stricter than before (a previously-accepted equal-endpoint write now errors) — a bug fix to the write contract, not a semantic change. `internal/graph/load.go` `validateEdge` | DECIDED (fix) |
| **Key uniqueness is enforced by the loader *across loads and labels*, not only within a batch** | The 2026-10-15 decision named "loader-enforced uniqueness"; the loader only checked *in-batch*, so `CREATE` silently added a second node for a key loaded earlier. Edge endpoints and `ResolveEntity` match by key alone, so a duplicate splits or cross-products them (and can mask an E6 group). `LoadNodes` now rejects any batch key already present in the graph before writing | DB unique constraint (still deferred); `MERGE` on key (a merge-shaped rewrite that would overwrite provenance); leave the limitation | Non-destructive: it **rejects** the batch, never merges or rewrites, so no canonical-identity semantics change and no provenance loss. This *implements* the loader enforcement the deferral always named; the DB-level constraint remains deferred (revisit before M2). Check-then-create is safe under the client mutex (loader is single-writer). `internal/graph/load.go` `LoadNodes`/`existingKeys` | DECIDED (follow-up) |

Resolvers still never merge or guess (`graph.ResolveEntity`, 4 deterministic
tiers, ambiguity → candidate list); the guard removes the only path that could
have produced two nodes sharing a key.

| Decision | Why | Alternatives | Tradeoffs / limitations | Status |
|---|---|---|---|---|
| **Staleness is documented as observed-time; no universal stale threshold is defined** | Freshness already derives from `observed_at` and is returned on every `Provenance` (`internal/knowledge/knowledge.go`); a `valid_to` in the past means *historically expired for that era*, not a stale source. Inventing a threshold would mint a policy no one asked for and mislabel legitimate history (`docs/SCHEMA.md` §5/§6) | Add a configurable stale threshold + stale/not-stale label; add a computed `age` field now; leave staleness entirely for a later slice | No new field or label in v1: consumers see raw `observed_at` and decide. A future freshness policy must be recorded here before any labeling code lands | DECIDED (documented, no threshold) |

## E1 identity-resolution contract audit (2026-10-16)

Adversarial regression suite added: `internal/graph/graph_test.go`
`TestResolveEntityContract` (9 subtests, live FalkorDB). The audit compared
`graph.ResolveEntity` against the contract in `docs/SCHEMA.md` §9 and
`docs/AGENT_SPEC.md` "ResolveEntity", and traced every caller
(`POST /v1/resolve`, the `resolve_entity` agent tool).

Evidence: stable `key` tier, name/label narrowing, ambiguity → sorted
candidate list (never a guess), determinism across insertion order,
idempotent read-only resolution, provenance and E5 temporal windows intact,
and the loader's cross-load `key` uniqueness guard (`load.go` `LoadNodes`)
closing the only path to two nodes sharing a key.

| Decision | Why | Alternatives | Tradeoffs / limitations | Status |
|---|---|---|---|---|
| **No production change to `ResolveEntity` this slice: the implementation already matches the locked contract** | The four tiers in `docs/SCHEMA.md` §9 are implemented in the specified order, hits are sorted by `key`, a single hit returns `MatchedOn` so callers can tell identity-grade (`key`) from weak-attribute matches, ambiguity returns the full candidate list, and no caller (`api.go` `/v1/resolve`, `agent.go` `resolve_entity`) ever auto-selects `Candidates[0]`. The one path that could have produced two nodes sharing a `key` is already closed by the loader guard | Change the tier order; add a scored/alias tier; refactor the full-graph scan into indexed per-tier lookups | Refactoring or reordering would be an unrequested semantic change, not a fix. The full-scan `ponytail:` ceiling (one label/key/name scan per call) stays until resolution latency is actually measured; upgrade path = per-tier indexed lookups | CONFIRMED (no change needed) |
| **A key-shaped query string stays eligible for the name/title tiers — the tier-confinement idea is recorded, not implemented** | `ResolveEntity` matches tiers by priority, not by input shape: a string that *looks* like a key but matches no node's `key` legitimately falls through to exact `name`/`title` (a contract-legal tier). The conceivable wrong-identity case — a non-existent key resolving to an unrelated entity via a name/title coincidence — is real but cannot be fixed without a canonical identity-shape decision first | (A) confine `src:id`-shaped input to the `key` tier; (B) adopt a stricter key shape than `keyShapeOK` before doing so; (C) leave the fall-through | `keyShapeOK` (`load.go`) only requires `:` with both sides non-empty, so it cannot distinguish a key from a legitimate colon-containing **title** (`"Adopt: FalkorDB"` is both). Option A regresses exact-title lookups; Option B invents an identity-shape rule. Guessing either way would silently change canonical identity semantics — out of scope for this slice. Options recorded in `docs/OPEN_QUESTIONS.md` "Still open"; current behavior locked by `TestResolveEntityContract/key-shaped_input_still_resolves_by_exact_title` so a future change is deliberate | UNDECIDED — recorded, no change |

The audit also confirmed `LoadNodes` is the only node-creating path in
production code (`CREATE`/`MERGE` appear nowhere else), so the uniqueness
guard covers every writer.

## M3 context assembly landed (2026-10-16)

Implements the retrieval half of `docs/REASONING.md` §2 ("Context assembly",
previously PROPOSED). Evidence: `internal/graph/evidence.go`,
`internal/graph/evidence_test.go` (`TestAssembleEvidence*`), and
`internal/agent` `TestAssembleContextToolSuppliesEvidence` (through the loop).

| Decision | Why | Alternatives | Tradeoffs / limitations | Status |
|---|---|---|---|---|
| **A dedicated read-only `graph.AssembleEvidence` returns a bounded, sorted `EvidenceContext` for one resolved entity** | Context assembly needs a deterministic "minimum evidence for this entity" step; composing existing ops per call would duplicate the resolve + trace + filter + bound logic at every caller | Have the agent loop call `resolve_entity` then `trace_evidence` and assemble client-side; add a general retrieval framework | One new op + one new tool; duplicates nothing from `TraceEvidence` (it reuses it internally). Reasons about one entity only (no query-level ranking) | DECIDED (contract) |
| **`as_of` filters state relations only (half-open), event relations are never window-filtered** | Event edges carry no `valid_to` (`GRAPH_MODEL.md`) and `FindExperts` already leaves authored/reviewed branches unfiltered, so window-filtering events would conflate validity-time with event-time (E5 scope decision) | Filter every returned edge by `as_of`; ignore `as_of` here | An `as_of` context can include events from outside the instant (documented ceiling); state facts (the ownership/validity ones) are correctly scoped | DECIDED (semantics) |
| **Deterministic store-independent sort (entity, relation, direction, neighbor key, valid_from, src_ref); result capped by count then serialized size; truncation always reported** | FalkorDB edge return order is not a contract; a bounded context must be reproducible and must not silently imply a partial result is exhaustive | Return DB order; unbounded result; omit truncation flag | Alphabetical-by-relation ordering means truncation drops later relations first (no relevance/recency ranking yet — PROPOSED). Limits are documented constants (`DefaultEvidenceLimit=64`, `DefaultEvidenceMaxBytes=64KiB`), not config | DECIDED (determinism) |
| **Explicit outcomes: `not_found` / `ambiguous` (never picks `Candidates[0]`) / `empty` / malformed `as_of` → error** | Same identity and safety discipline as `ResolveEntity`; a retrieval step must not invent an entity or fabricate an empty-as-answer | Fall back to the top candidate; treat unknown as empty | Slightly more status handling for callers; keeps the never-guess invariant end to end | DECIDED (safety) |
| **Exposed to the agent as the `assemble_context` tool (agent tool surface 6 → 7); graph-op boundary expands 5 + 1 → 7 + 1** | The loop needs the retrieval step to build grounded answers (`AGENT_SPEC.md`); consistent with the `FindConflicts` expansion precedent | Agent composes the two existing tools itself; keep it off the tool surface | Amends locked D7 again; reason recorded here per `docs/RULES.md` §6, contract added in `docs/AGENT_SPEC.md`. `FindConflicts` stays API-only (not a tool) | DECIDED (boundary) |
| **No new answer format: `knowledge.Answer`/`Claim.Paths` cannot carry structured provenance, so retrieval supplies string fact paths the existing `Validate` accepts** | Changing the answer schema is out of scope; fabricating a richer format would be an unrequested contract change (`opencode.md` rule 9) | Extend `Claim` to hold structured provenance (id + provenance); add a parallel evidence type | **Known gap**: structured per-fact provenance lives in `EvidenceContext`, not in a `knowledge.Answer`; linking an answer's `Claim.Paths` back to `EvidenceFact` provenance is by path string only. Recorded in `docs/OPEN_QUESTIONS.md`; revisit when the answer format is next revised | DECIDED (gap, no format change) |

`AssembleEvidence` takes no client lock of its own — it composes
`ResolveEntity` + `TraceEvidence`, each of which locks; taking `c.mu` would
deadlock. Read-only: it writes nothing and never materializes an inference.

## M3 evidence contract audit (2026-10-16)

Adversarial audit of the M3 evidence-to-answer path (`internal/graph/evidence.go`
→ `internal/agent/agent.go` → `internal/knowledge/answer.go`), traced end to end
against the actual callers and serialization. Two demonstrated defects fixed;
the rest of the enumerated cases were confirmed and locked with tests.

Defect 1 — **`ok` with no facts.** A `max_bytes` smaller than a single fact's
serialized row emptied `Facts` but left `Status` at `ok`, which the contract
documents as "facts were found". Fixed by adding the explicit
`EvidenceStatusOverBudget` outcome for that case (an *additive* status; `ok`,
`not_found`, `ambiguous`, `empty` keep their meanings). Evidence:
`TestAssembleEvidenceBounds` ("byte bound below one fact is an explicit
outcome", "count and byte bounds both report").

Defect 2 — **non-total sort.** `sortEvidenceFacts` compared only the six primary
keys, so two facts equal on all of them inherited the store's return order
(`sort.Slice` is not stable) and could swap between runs — which decides who
survives a truncation. Fixed by making the comparator a total order over every
fact field (`cmp.Or`), preserving the documented primary order. Evidence:
`TestAssembleEvidenceOrderIsTotal` (pure unit: fails before the fix).

| Decision | Why | Alternatives | Tradeoffs / limitations | Status |
|---|---|---|---|---|
| **`ok` means facts were found *and returned*; an unsatisfiable byte bound is the explicit `over_budget` outcome** | `status` is the field a consumer keys on; `ok` + empty `facts` is self-contradictory and could be read as usable evidence (or as evidence-less). The existing `empty` is wrong (facts *are* in scope) and `not_found` is wrong (the entity resolves) | Leave `ok` + empty `facts` (documented, but surprising); return the oversized fact and exceed `max_bytes`; reuse `empty` | Adds one status value consumers must handle; there is no HTTP surface for `AssembleEvidence` (only the `assemble_context` tool), so no external contract breaks. Bounds are otherwise unchanged | DECIDED (fix) |
| **Fact ordering is a total order over every field, not just the primary keys** | "Store-independent order" must be true for every input, or the survivors of a truncation depend on DB scan order — a determinism (`REASONING.md` §7 reproducibility) defect, not a cosmetic one | Keep the six-key comparator and rely on `sort.Slice`; use a stable sort over the store's order (still store-dependent) | One extra comparator (`factLess`); negligible cost over ≤64 facts | DECIDED (fix) |
| **`relations` is a scope, not a lookup: an unknown type yields `empty`, never an error** | Signed off as the contract's behavior; makes deliberate traversal total (a typo shows as no evidence rather than silently returning everything) | Validate against the 16-type schema and error on unknown; treat unknown as "no filter" | A mistyped type is not surfaced as an error (documented ceiling); validating is deferred — it would reject forward-compatible filters the resolver could still honor | DECIDED (documented) |
| **A state edge with an unparseable stored window is excluded under `as_of`, never treated as always-valid** | Matches the E6 rule (an unreadable window is omitted, not assumed open) and is unreachable through `LoadEdges` (`validateEdge` requires a parseable `valid_from`); the guard only matters if a window is written around the loader | Treat malformed windows as always-valid; return an error | A malformed-window edge vanishes from as-of results (documented); the no-`as_of` read still returns it | CONFIRMED (no change) |
| **No shared snapshot across the `ResolveEntity` → `TraceEvidence` composition; single-writer assumptions hold in v1** | The two sub-calls are not one snapshot, but the only writers are the recreate-only seed and tests under a single client mutex, so no supported workflow observes a harmful inconsistency; the absent-node race is already handled (`found=false` → `not_found`) | One lock around the whole assembly (deadlock as written; would need an unlocked inner path); a read transaction | Documented ceiling in the code (`ponytail:`); upgrade path = one lock / read transaction if a concurrent-writer workflow appears | CONFIRMED (no change) |
| **No change to `knowledge.Answer`/`Claim.Paths`; the string-path bridge stays** | The retrieval half now returns complete structured provenance per `EvidenceFact`, but `Claim.Paths` is a string slice; changing the answer schema is an unrequested contract change | Extend `Claim` with structured provenance; add a parallel evidence envelope | Structured per-fact provenance lives only in `EvidenceContext`; linking a claim path back to a `EvidenceFact` is by path string. Revisit when the answer format is revised (`OPEN_QUESTIONS.md`) | UNDECIDED — recorded, no change |

The audit also confirmed the agent tool surface does not choose `Candidates[0]`,
hide truncation, or turn missing evidence into a supported claim: a claim with
no path fails `Answer.Validate`, so zero-fact contexts cannot back one. The
scripted-model tests prove tool behavior and transcript flow only; end-to-end
grounding (E3) remains PROPOSED.

## E3 source-grounded answer contract (2026-10-16)

Resolves the open question "How should a `knowledge.Answer` carry structured
per-fact provenance from context assembly?" (`docs/OPEN_QUESTIONS.md`). The M3
audit above left the string-path bridge in place: `knowledge.Answer.Validate`
only checks that a claim has at least one non-empty path, so a claim can cite a
plausible-looking path that matches no retrieved fact. This slice adds and tests
a contract that binds a claim to the *actual* retrieved evidence.

| Decision | Why | Alternatives | Tradeoffs / limitations | Status |
|---|---|---|---|---|
| **Option C — an additive `graph.GroundedAnswer` envelope (chosen over A and B)** | It is the only option that carries the real retrieval result next to the answer, so grounding is *checkable* without an import cycle or an out-of-band lookup. `knowledge.Answer`, `Claim`, and `Validate` are untouched, so every composer (`internal/ui/ui.go`), the API schema, and the agent bridge stay compatible | **A** extend `knowledge.Claim` with structured provenance — rejected: `internal/knowledge` cannot import `internal/graph` (`graph` imports `knowledge`; the reverse is a cycle), so it would need a `knowledge`-level duplicate of `graph.EvidenceFact`; it changes a type used at ~40 sites, and if left additive it still cannot verify a reference without the retrieval result, leaving the fake-path hole open. **B** keep `Claim.Paths` and link to `EvidenceContext` out of band — rejected: the link would be by path string (reconstructing provenance from display strings, exactly what this slice must stop) and cannot reliably reject unrelated evidence | Grounding metadata is a slice parallel to `Answer.Claims` (index-aligned, length-checked); the envelope lives in `internal/graph` (already imports `knowledge`) rather than a new `internal/reason` package (does not exist) — movable if the reasoning layer ever earns its own package | DECIDED |
| **`graph.EvidenceFact` gains a stable, content-derived `id`** | A claim must cite specific evidence identically across runs and a JSON round trip; a positional index drifts if facts are reordered or a subset is carried. The id is FNV-1a over the fact's own identifying fields (anchor, relation, direction, neighbour, window, source), so the same fact always hashes the same and two genuinely identical facts share it (they *are* the same fact) | Reference by slice index; re-state the fact's coordinates in the claim | One additive JSON field. It counts toward `bytes`/`max_bytes` exactly like any other field (size is computed from the marshalled fact, so bounds stay honest); existing deterministic-order and substring assertions still hold because the id is deterministic | DECIDED |
| **`GroundedAnswer.Validate` requires every FACT claim to cite ≥1 *known* fact whose anchor and (when declared) relationship match, and refuses any FACT claim when the evidence status is not `ok`** | The core goal: a FACT claim cannot cite fabricated/unknown evidence, evidence anchored to another entity, or evidence of a different relationship; and `not_found`/`ambiguous`/`empty`/`over_budget` can never be laundered into a supported factual claim. INFERENCE/RECOMMENDATION keep their existing `rule_id`/path discipline and are not required to be fact-grounded | Let a FACT claim cite any path string (the old hole); treat every non-`ok` status the same (no distinction) | **Absence** claims ("no `OWNS` edge") have no `EvidenceFact` to cite, so this contract cannot ground them (documented ceiling); they keep the existing operation-reference string path. The contract proves *structural* linkage, not factual correctness (E3 stays PARTIAL) | DECIDED |
| **Truncation and non-`ok` outcomes are carried verbatim, never re-derived or flattened** | "Never silently imply a partial result is exhaustive" (`REASONING.md` §2): the envelope preserves the context's `status`/`truncated`/`truncated_reason` unchanged. A truncated-but-`ok` context still validates — it was partial and says so | Recompute or drop the status inside the answer; add an explicit "complete" claim | A consumer must read the context, not just the claims; the contract adds no completeness assertion of its own | DECIDED |

Evidence: `internal/graph/grounded.go` (`GroundedAnswer`, `ClaimGrounding`,
`Validate`), `internal/graph/evidence.go` (`EvidenceFact.ID`, `evidenceFactID`),
`internal/graph/grounded_test.go` (unit: fabricated id, wrong relation, wrong
entity, missing evidence, non-`ok` status, JSON round trip; live: real-FalkorDB
provenance preservation, explicit outcomes, truncation). `knowledge.Answer`,
`Claim`, `Validate`, and all their callers are unchanged. E3 remains **PARTIAL**:
structural grounding is proven; factual correctness/entailment needs a wired
answering model and a labeled Q/A corpus, neither of which exists
(`docs/EVALUATION.md` §6).

## Notes
New decisions must be added here. Mark as DECIDED only after team consensus during/at hackathon start.


## Competition build completion decisions — 2026-10-16

These supersede earlier deferrals for the local Track 03 build, authorized by the participant's completion request.

- **Deterministic question agent:** `agent.Ask` resolves identities from the graph and routes five workflows. Unknown intents and ambiguous entities abstain. No LLM provider is selected or required.
- **L2 draft only:** handoff Markdown is generated from retrieved evidence, with a human-review label. It does not send messages or assign work. No L3+ executor is introduced.
- **Explicit source refresh:** CLI `import` accepts mapped Document artifacts (including chat/ticket kinds), with atomic per-record upserts, source identity checks, observation ordering, and idempotency. It preserves the graph-operation writer boundary. Batch partial progress is reported and retryable.
- **Ownership writer:** CLI `owner-set` applies explicitly sourced registry changes in one Cypher mutation, closing the previous window and opening its replacement. Repeated same-owner updates are no-ops. Backdated/scheduled-history rewriting is rejected.
- **Current means now:** the far-future ownership sentinel is removed; omitted as_of uses actual UTC time. A regression test covers a current owner whose window ends at a future handoff.
- **Whole-second validity:** state-window endpoints must have whole-second precision. This avoids silent fractional truncation under lexically compared canonical UTC timestamps.
- **Identity guard:** LoadNodes rejects graph-wide duplicate keys across loads within the single serialized writer. This supersedes the earlier in-batch-only deferral; external concurrent writers still require database constraints.
- **Source content:** Document adds body and kinds chat/ticket; Decision adds recorded rationale. Missing rationale is disclosed instead of presenting the provenance neighborhood as an explanation.
- **Local packaging:** Docker Compose pins the tested FalkorDB image digest and persists the graph. A Go multi-stage image provides the optional application service. Public hosting/authentication remain out of scope.

## 2026-10-16 - integration of Company Brain completion and upstream M3

- Preserved upstream identity contract tests and bounded evidence assembly, including explicit truncation and ambiguous outcomes. Consolidated duplicate loader guards into the upstream implementation.
- Kept source timestamps visible without assigning a universal stale threshold, consistent with the upstream freshness decision.
- Current ownership uses actual UTC now, superseding the earlier far-future sentinel decision; scheduled future owners must not appear as current owners.

## 2026-10-17 — local model, sync, and identity hardening

- **Optional answering provider: local Ollama.** `internal/agent/ollama.go`
  implements the existing `Decide` seam. The model receives bounded read-only
  graph retrieval tools and selects evidence IDs; the server validates and
  renders relationship facts. Deterministic `/v1/ask` remains model-free.
- **M3 is still partial.** The labeled synthetic model corpus is executable and
  the run is recorded in `docs/EVALUATION.md`; human semantic review and
  keyword/text-RAG/no-graph baselines are still open. This supersedes earlier
  dated statements that no provider or corpus existed, not the remaining
  quality gate.
- **Explicit Markdown snapshot sync.** `tron sync` uses declared paths and
  identity mappings, advances the cursor only after a complete snapshot, and
  deletes only source-owned documents absent from that completed snapshot.
- **Unique keys.** `tron constraints` creates/verifies per-label database
  uniqueness constraints. Application writes additionally reject cross-label
  duplicates. This supersedes the 2026-10-15 database-constraint deferral;
  cross-label uniqueness is not a database constraint, so external writers
  remain outside that guarantee.
- **Credential-selected tenant graphs.** Optional server-owned bearer
  credentials scope requests to a graph. Per-document ACLs, managed identity,
  and production deployment controls remain absent.
