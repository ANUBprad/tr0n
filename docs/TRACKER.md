# TRON — Implementation Tracker

**Status**: Canonical. Live status of milestones, components, and backlog.
**Last updated**: 2026-10-15.
**Related**: [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md),
[`EVALUATION.md`](EVALUATION.md), [`RULES.md`](RULES.md).

> **Status reflects evidence, not intent.** A component is `Verified` only when
> a committed test/command demonstrates it. Update a row only with a code path
> or PR reference. Never mark `Implemented` because something was designed.

## Status legend

| Status | Meaning |
|---|---|
| **Verified** | implemented and demonstrated by a committed test/command |
| **Implemented** | code exists, no direct test evidence yet |
| **In progress** | partially built |
| **Planned** | scheduled in a milestone, not started |
| **Not started** | no work, not yet scheduled |

## Milestones

| ID | Milestone | Status | Evidence |
|---|---|---|---|
| M0 | Architecture & repository baseline | **Verified** | `tron demo` 7/7; 10 test files; decision-log |
| M1 | Trustworthy organizational memory | **In progress** | Slice 1: schema/provenance + fact-write contract (`internal/graph/load.go`); **slice 2 (2026-10-16)**: E6 contradiction detection (`graph.FindConflicts` + `POST /v1/conflicts`); **slice 3 (2026-10-16)**: E5 temporal as-of — optional `as_of` on `FindOwner`/`FindExperts` + `/v1/owner`/`/v1/experts`, canonical-UTC window normalization, empty-window rejection; **slice 4 (2026-10-16)**: E1 loader cross-load/label duplicate-key guard + runnable identity fixture; staleness **documented** (no threshold); **outstanding**: multi-source identity merge (M2); E1 PARTIAL, E5/E6 IMPLEMENTED |
| M2 | Data ingestion & synchronization | **Not started** | only `internal/seed` writes facts |
| M3 | Evidence-grounded reasoning | **In progress** | agent loop + 7 ops + `Answer.Validate`; no LLM provider wired |
| M4 | Decision support | **Not started** | — |
| M5 | Controlled action execution | **Not started** | read-only today |
| M6 | Proactive organizational intelligence | **Not started** | — |
| M7 | Production readiness & evaluation | **Not started** | — |

## Component tracker

### Interface & application
| Component | File | Status | Gaps |
|---|---|---|---|
| CLI core | `cmd/tron/main.go` | **Implemented** | owner/seed/demo/serve only; source/action groups planned |
| Health / API server | `internal/api/api.go` | **Verified** | 7 read ops (incl. `POST /v1/conflicts`); `owner`/`experts` accept optional `as_of`; no auth |
| Web UI | `internal/ui/ui.go`, `page.html` | **Implemented** | 5 fixed questions; no open Q&A |
| Demo harness | `cmd/tron/demo.go` | **Verified** | 7 checks pass/fail (5 scenarios + temporal as-of + evidence) |

### Knowledge model
| Component | File | Status | Gaps |
|---|---|---|---|
| Graph layer | `internal/graph/graph.go` | **Verified** | one global mutex (`ponytail:` ceiling) |
| Fact-write contract | `internal/graph/load.go` | **Verified** | key shape, provenance minimum, referential integrity, temporal windows; `internal/graph/load_test.go` |
| Temporal as-of queries | `internal/graph` | **Verified** | optional `as_of` on `FindOwner`/`FindExperts` (half-open `valid_from <= as_of < valid_to`); loader normalizes windows to canonical UTC; sentinel default = current; `TestFindOwnerAsOf`, `TestFindExpertsAsOf`, `TestLoadEdgesNormalizesOffsetToUTC` (2026-10-16) → E5 IMPLEMENTED |
| Key uniqueness enforcement | `internal/graph/load.go` | **Verified (loader)** | DB-level constraint still **deferred (2026-10-15)**; since 2026-10-16 the loader rejects any batch key already present in the graph (across loads *and* labels), not just in-batch — non-destructive, no merge (`TestDuplicateKeyAcrossLoadsIsRejected`); non-unique index remains; revisit a DB constraint before M2 (`planning/decision-log.md`, `OPEN_QUESTIONS.md`) |
| Entity resolution | `internal/graph/graph.go` (`ResolveEntity`) | **Verified** | single-source; no cross-source merge (`TestResolveEntity`, `TestDuplicateKeyAcrossLoadsIsRejected`) |
| Expertise | `internal/knowledge/expertise.go` | **Verified** | weights unvalidated (R10) |
| Decision trace | `internal/knowledge/decision.go` | **Verified** | — |
| Incidents | `internal/knowledge/incidents.go` | **Verified** | — |
| Contradiction detection | `internal/graph/graph.go` | **Verified** | `FindConflicts` (`contradiction/v1`, state relations only, `OWNS` exclusive) + `POST /v1/conflicts`; `TestFindConflicts`, `TestEndpoints/conflicts` (2026-10-16) |
| Staleness surfacing | — | **Documented (no code)** | Freshness is `observed_at`-based and already returned on every `Provenance`; no universal stale threshold is defined (by decision 2026-10-16) — labeling a fact stale would need an explicit policy (`docs/SCHEMA.md` §5/§6, `planning/decision-log.md`) |

### Reasoning
| Component | File | Status | Gaps |
|---|---|---|---|
| Agent loop + tool schema | `internal/agent/agent.go` | **Implemented** | no provider adapter |
| Answer validation / layering | `internal/agent`, `internal/knowledge` | **Verified** | — |
| LLM provider adapter | — | **Not started** | M3; ADR required |
| Retrieval/context assembly | — | **Not started** | M3 |
| Rules engine | — | **Not started** | M4 |

### Ingestion
| Component | File | Status | Gaps |
|---|---|---|---|
| Seed generator | `internal/seed/*` | **Verified** | synthetic, wipe+reload only |
| Ingestion pipeline | `internal/ingest` (new) | **Not started** | M2 |
| Sync/cursors/delta | `internal/sync` (new) | **Not started** | M2 |
| Document store | — | **Not started** | M2 |

### Actions
| Component | File | Status | Gaps |
|---|---|---|---|
| Read-only operation boundary | `internal/api`, `internal/agent` | **Verified** | no writes by design |
| Authorization gate | — | **Not started** | M5 |
| Executor + idempotency + verification | — | **Not started** | M5 |
| Audit log | — | **Not started** | M5 |

### Platform / security
| Component | Status | Gaps |
|---|---|---|
| Loopback binding (FalkorDB + HTTP) | **Verified** | `compose.yaml`, `cmd/tron/main.go` |
| Strict JSON + body cap + timeouts | **Verified** | `internal/api/api.go`, `cmd/tron/main.go` |
| Authentication & authorization | **Not started** | M7 |
| Tenant isolation | **Planned** | designed, unexercised |
| Backup / retention | **Not started** | R6 |
| CI (lint/test gates) | **Not started** | `EVALUATION.md` hard gates |

### Evaluation & docs
| Component | Status | Evidence/Gaps |
|---|---|---|
| Go test suites (10 files) | **Verified** | real-FalkorDB op tests; provenance/layering/write-contract invariants |
| Eval suites E1–E12 | **In progress** | E1/E2 PARTIAL, E4 IMPLEMENTED (structural); E5 **IMPLEMENTED** (2026-10-16: `TestFindOwnerAsOf`, `TestFindExpertsAsOf`, `TestLoadEdgesNormalizesOffsetToUTC`); E6 **IMPLEMENTED** (2026-10-16: `TestFindConflicts`, `TestEndpoints/conflicts`); E3/E7+ PROPOSED; baselines none |
| Comparison baselines | **Not started** | keyword search, RAG, tool-LLM |
| Canonical docs (this set) | **Verified** | `docs/README.md` index |

## Backlog (unscheduled)

| Item | Reason deferred | Revisit when |
|---|---|---|
| Second ingestion source | one source first (M2) | M2 acceptance met |
| Vector/lexical index | index only routes (A2) | high-degree case measured (R8) |
| Multi-agent framework | unnecessary (A10) | a measured need + ADR |
| Materialized inference edges | recompute at query time (A9) | query cost measured |
| Re-add cut edges (`MEMBER_OF`, …) | not in v1 model | explicit need + ADR |
| Message queue / scheduler | one binary (A1) | throughput measured |
| Cross-tenant isolation | single tenant | before 2nd tenant |
| `tron doctor`, `tron eval` CLI | convenience | with M7 |
| DB-level `key` uniqueness constraint | deferred by decision (loader-enforced sufficient for v1; limitation recorded) | before multi-source ingestion (M2) |

## Update rules

- Change a status only with a code path, test, or command reference.
- New work enters via a milestone ([`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md));
  unscheduled ideas go to the backlog with a reason.
- Keep this file and the two above in sync: milestone status here must match
  the milestone section there.
