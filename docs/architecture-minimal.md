# TRON — Architecture (Minimal)

**Status**: CONCEPTUAL — PRE-IMPLEMENTATION

## Principles
- Graph-first reasoning (FalkorDB central)
- Evidence-backed: every answer traceable to graph paths
- Provenance: distinguish FACT/INFERENCE/RECOMMENDATION
- Separation of concerns: Core (reasoning+policy) vs Graph vs Ingestion vs API vs Clients
- Provider/model independence (LLM abstracted)
- Deterministic over probabilistic where possible
- Portable core; thin clients
- Minimal runtime deps; cross-platform
- Testable, observable, fail-isolated
- YAGNI, fewest files

## Conceptual Core
```
Clients (Web/Desktop/API)
        ↓
TRON API (thin)
        ↓
Core Orchestrator (intent→tools→reasoning)
        ↓
Tool Layer (graph tools)
        ↓
Graph Adapter (FalkorDB)
        ↓
FalkorDB (Redis Module)
```

## Modules (conceptual)
- api: transport boundary
- core: reasoning/orchestration, policy, provenance
- tools: graph retrieval tools (deliberate)
- graph: adapter interface + FalkorDB impl
- ingestion: future (identity resolution, normalization)
- llm: provider abstraction (future)
- types: shared domain

## Graph Adapter Interface (sketch)
Define minimal interface (language-agnostic concept):
- query(cypher, params) → rows+paths
- getNeighbors(id, rels, depth) minimal
- findByLabel/name

Keep Cypher as primary (FalkorDB-native). Abstract only if justified.

## Cross-Platform Strategy
- Portable core (Go/Rust target linux/mac/win)
- Single binary + Docker
- API-first: clients consume same API
- Avoid platform-specific business logic
- Local-first; complete local setup acceptable

## Security/Boundaries
- Provenance required
- Evidence paths mandatory
- Deterministic components isolated
- No secrets in repo
- Thin trust boundary at API

## Open
Stack choice (Go/Rust/Python/TS) — UNDECIDED. Lock at Hour 0–4.

