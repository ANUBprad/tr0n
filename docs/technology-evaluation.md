# TRON — Technology Evaluation

**Status**: LOCKED (2026-10-08). See `planning/decision-log.md`.

## Goals

Choose stack by requirements: FalkorDB integration, API, concurrency,
ingestion pipelines, cross-platform binaries, deployment simplicity,
maintainability, long-term extensibility. No familiarity/popularity bias.

## Candidates

| Candidate | Strengths | Weaknesses | FalkorDB | Cross-platform | Deploy | Dev speed | Long-term |
|---|---|---|---|---|---|---|---|
| **Go** | Small static binaries; goroutines fit I/O-bound pipelines; `net/http` stdlib; trivial cross-compile; simple tooling (`-race`, `pprof`) | Verbosity; no generics-heavy abstractions (arguably an asset) | Official `falkordb-go` (BSD) + `rueidis` (RESP3, Apache-2.0); FalkorDB speaks Redis protocol so any raw-command client works | Excellent: `CGO_ENABLED=0 GOOS/GOARCH`, no target toolchain needed (linux/mac/win, amd64/arm64) | Single binary + distroless/scratch image | High | Strong (15+ yrs backward compatibility) |
| **Rust** | Peak perf/memory safety; fearless concurrency; best embeddability (CDYLIB/wasm) | Slower iteration; borrow-checker cost paid up front; smaller team pool | `falkordb-rs`, `rustis` — equally capable | Excellent (needs `rustup target` per platform) | Single binary | Medium | Strong |
| **TypeScript (Node)** | Fast iteration; UI-adjacent | Runtime weight; larger images; GC pauses rarely matter but image/deploy cost does | `falkordb-ts` official | Good | Node runtime in image | Very high | Good |
| **Python** | Fastest prototyping | Slow runtime; heavy images; packaging friction; weakest long-term fit for a system service | `falkordb-py` official (strongest client) | Good | venv/Docker | Very high | Medium for core services |
| **Java/Kotlin** | Mature ecosystem | JVM footprint; heavier images | `jfalkordb` official | Good | JAR/Docker | Medium | Strong |
| **C#/.NET** | Solid modern runtime | Larger runtime; less minimal | `NFalkorDB` official | Good | Native AOT possible | High | Strong |

## Go vs Rust — requirements-by-requirements

| Requirement | Go | Rust | Verdict |
|---|---|---|---|
| FalkorDB integration | Official client + RESP3 fallback | Official client + `rustis` | Tie — protocol-level, not language-level |
| HTTP/API | `net/http` stdlib, zero deps | External (axum/actix) | Go |
| Concurrency | Goroutines: natural fit for fan-out ingestion (I/O-bound) | async/tokio: more machinery for same shape | Go (workload is I/O-bound, not CPU-bound) |
| Graph querying | Network round-trips to FalkorDB — language irrelevant | same | Tie |
| LLM/provider integration | HTTP+JSON trivial | same | Tie |
| Cross-compile | One env var, static, no toolchain on target | Needs per-target toolchain | Go |
| Dev velocity | Compile+test loop is very fast | Slower, more design up front | Go |
| Memory footprint | Small enough (service is network-buffer-bound) | Smaller | Rust wins, value low for TRON |
| Debugging/testing | `-race`, `pprof` in stdlib toolchain | Good but heavier tooling | Go |
| Desktop/mobile future | API-first makes client language independent | Rust better for embedding | Neutral — not a core requirement |
| Maintainability (multi-contributor) | One true style, `gofmt`, small language | Larger surface, more idioms | Go |

**Where Rust would win for TRON**: CPU-bound in-process work — large-scale
entity resolution, local graph algorithms over millions of edges, embedded
or wasm distribution. None of these exist in TRON's near-term scope; FalkorDB
performs the graph compute. Rust can be introduced later only if profiling
proves a bottleneck, as a **separate process** speaking stdio/HTTP — not via
cgo (which would forfeit `CGO_ENABLED=0` cross-compilation).

## Decision

**LOCKED: Go** (latest stable at scaffold time; pin exact version in `go.mod`).

Rationale: portable static binaries, concurrency model matches ingestion/API
workloads, stdlib covers HTTP/JSON/testing, fastest reliable iteration,
strong FalkorDB client options, small dependency surface, proven for
long-running infrastructure services.

Rust: reserved for evidence-driven future hotspots only. Do not introduce it
to be impressive.

## Dependency posture (at scaffold)

- Core deps expected: **one** FalkorDB/Redis client (pick between official
  `falkordb-go` and `rueidis` by inspecting both repos at scaffold time —
  API surface, RESP3 support, maintenance activity — record in decision-log).
- HTTP: stdlib `net/http` (patterns/method matching sufficient).
- JSON: stdlib `encoding/json`.
- No web framework, no ORM, no agent framework, no DI container.
