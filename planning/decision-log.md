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

## Notes
New decisions must be added here. Mark as DECIDED only after team consensus during/at hackathon start.

