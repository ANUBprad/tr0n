# TRON — Architecture (Locked)

**Status**: LOCKED (2026-10-08). See `planning/decision-log.md`.
Stack: **Go** (see `technology-evaluation.md`). Graph: **FalkorDB** (see `falkordb-analysis.md`).

## Principles

The original candidate list was challenged; cut or merged where a principle
did not change a decision.

1. **Graph-first reasoning** — FalkorDB is the source of connections; removing
   it materially breaks the product. (kept)
2. **Layered knowledge** — FACT ≠ INFERENCE ≠ RECOMMENDATION, never silently
   promoted. (kept; full rules in `knowledge-model.md`)
3. **Every claim carries its evidence chain** — provenance is part of the
   answer, not an appendix. (merged: "evidence-backed", "provenance",
   "explainability" are one requirement)
4. **Deterministic core, probabilistic edge** — graph operations are
   deterministic; LLM (when added) selects/narrates, never asserts or scores.
   (kept, sharpened)
5. **Append-only facts + temporal validity** — history is never rewritten;
   supersession via `valid_to`. (kept; model in `knowledge-model.md`)
6. **Core runs without LLM or UI** — graph operations are plain functions
   with typed results; a natural-language layer is optional on top. (added —
   this is what makes the core testable and provider-independent)
7. **Provider independence for models, deliberate dependence on FalkorDB** —
   model/LLM layer abstracted; the graph layer is intentionally
   FalkorDB-specific (that is the product thesis, not a limitation).
   (split from "database abstraction" — see dropped list)
8. **Minimal dependencies** — stdlib first; expected external deps ≈ one
   Redis-protocol client. (kept)
9. **Cross-platform by construction** — pure Go, `CGO_ENABLED=0`, API-first,
   thin clients; no platform-specific business logic. (kept)
10. **Observable by construction** — an answer's evidence chain *is* the
    audit trace; log query, latency, rule ids alongside it. (kept)
11. **Failure isolation at ingestion** — one corrupt source must not corrupt
    or block others. (narrowed from generic "failure isolation")

**Dropped from the candidate list** (challenge applied):

- *Database abstraction / DB portability* — rejected as a principle.
  Abstracting the graph engine before a second engine exists is a
  GraphProvider/GraphManager-shaped speculative interface (AGENTS.md rung 1).
  FalkorDB-specific code lives in exactly one package (`internal/graph`), which
  is the natural seam if a port is ever actually needed.
- *Temporal knowledge / incremental updates / extensibility* — real
  requirements, but each is owned by a concrete document
  (`knowledge-model.md`, append-only rule, the stable operation boundary);
  listing them as parallel "principles" added no decision power.
- *LLM-assisted vs LLM-owned truth* — kept, but folded into #4/#6.

## Conceptual structure

```
        Clients (web/desktop/CLI — thin, any platform)
                    │  HTTPS/JSON (one API)
                    ▼
              TRON API (stateless, Go)
                    │
                    ▼
      Graph Operations (deterministic, internal/graph)
        FindOwner · FindExperts · TraceDecision ·
        FindRelatedIncidents · TraceEvidence
                    │  Cypher over Redis protocol
                    ▼
        FalkorDB — facts, provenance, temporal validity
                    ▲
   [later] Ingestion writes facts (sources → facts, append-only)
   [later] Agent/reasoning layer consumes operation results,
           adds natural language; cannot write facts
```

Notes:
- The agent layer (future) sits *beside* the API path, consuming the same
  deterministic operations. It never queries FalkorDB directly with
  free-form Cypher and never writes facts.
- Ingestion is a writer; reasoning is a reader. Single writer boundary makes
  provenance enforcement auditable.

## Package layout (declared now, created only when code exists)

```
cmd/tron/            single entrypoint binary
internal/graph/      FalkorDB client + the five operations (all Cypher here)
internal/knowledge/  provenance/layer/evidence types shared by graph+api+agent
internal/api/        HTTP handlers (when the API lands)
internal/ingest/     source connectors (when ingestion lands)
internal/agent/      reasoning/LLM layer (when the agent lands)
```

Deliberately **not** created: `GraphProvider`, `GraphManager`,
`GraphRepository`, `GraphFactory`, `GraphEngine`, `Service`/`Repository`
layers, `pkg/` exports, config frameworks. Rule: a package exists only when
real code exists; an interface exists only when a second implementation
exists (Go: declare interfaces at the consumer, not the producer).

Testing posture: no mock graph interfaces. Tests run against a real
FalkorDB in Docker (see Testing below), because the graph's behavior *is*
the behavior under test.

## Graph operation boundary

Defined in `AGENT_SPEC.md` (one home for tool contracts): five deterministic
operations with input/output/traversal/provenance. The API and the future
agent both call exactly those; nobody else builds ad-hoc Cypher.

## Cross-platform strategy

- Portable core: single static Go binary (`CGO_ENABLED=0`), target
  linux/macOS/windows × amd64/arm64 via env vars, no target toolchains.
- Containers: distroless/scratch + FalkorDB container via compose.
- API-first: desktop/web/mobile/CLI are thin clients of the same JSON API;
  no business logic in clients.
- Local-first operation must always work (FalkorDB via Docker); cloud
  deployment optional, not architectural.

## Security / data boundaries

- Graph-level separation by FalkorDB graph name (tenant isolation point, if
  ever needed — one graph per tenant, chosen at connect time; no custom
  multi-tenant layer now).
- Ingestion is the only writer of facts; API/agent are read-only w.r.t. the
  graph. (ponytail: read-only API is a blunt, simple, correct v1 boundary;
  upgrade path: explicit write endpoints for user-confirmed actions.)
- No secrets in repo; config via environment.
- Every response carries layer tags + provenance (see `knowledge-model.md`).

## Testing strategy (for implementation phase)

1. **Operation tests** — each of the five ops against seeded test graphs in
   Docker: golden input → expected nodes/paths/provenance fields.
2. **Provenance invariant tests** — every FACT result must carry
   src_type/src_ref/observed_at/extraction; fail the test otherwise.
3. **Layering tests** — inference results must carry rule_id + basis paths;
   no code path may emit an inference labeled as fact.
4. **Temporal tests** — as-of queries; ownership/membership supersession
   (old edge `valid_to` set, new edge opened); "current" = `valid_to IS NULL`.
5. **Multi-hop reasoning tests** — seeded fixtures that only answer correctly
   via ≥3 hops (the product thesis expressed as a test).
6. **Ingestion tests** — idempotent re-ingest, conflicting sources preserve
   both with temporal windows, bad source isolated (failure isolation).
7. **End-to-end** — one smoke path: seed → query → answer with chain.
8. **Cross-platform** — CI builds GOOS/GOARCH matrix (compile-only is
   sufficient for v1).

## Performance considerations (measure, don't optimize yet)

| Likely bottleneck | Why | Mitigation when measured |
|---|---|---|
| Cypher wide-pattern scans | Unbounded `MATCH` fan-out | Bounded depth in ops; index `key` per label; exact-match resolve |
| Expertise scoring | Counts across 4 path patterns | Single aggregated query; materialize only if profiling demands |
| Ingestion fan-out | Many sources, API rate limits | Bounded worker pool per source; per-source isolation |
| LLM latency (future agent layer) | Model round-trip | Only affects NL layer; core ops stay fast |
| Serialization | Evidence payloads | Deliberate retrieval keeps subgraphs small by design |

Measure: op latency p50/p95, rows/paths returned per op, ingest throughput.
No optimization without a number.
