# TRON — Implementation Tracker

**Status**: Canonical. Live status of milestones, components, and backlog.
**Last updated**: 2026-10-12.
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
| M0 | Architecture & repository baseline | **Verified** | `tron demo` 6/6; 9 test files; decision-log |
| M1 | Trustworthy organizational memory | **In progress** | schema/provenance/temporal + fact-write validation exist; no contradiction/staleness surfacing |
| M2 | Data ingestion & synchronization | **Not started** | only `internal/seed` writes facts |
| M3 | Evidence-grounded reasoning | **In progress** | agent loop + 6 ops + `Answer.Validate`; no LLM provider wired |
| M4 | Decision support | **Not started** | — |
| M5 | Controlled action execution | **Not started** | read-only today |
| M6 | Proactive organizational intelligence | **Not started** | — |
| M7 | Production readiness & evaluation | **Not started** | — |

## Component tracker

### Interface & application
| Component | File | Status | Gaps |
|---|---|---|---|
| CLI core | `cmd/tron/main.go` | **Implemented** | owner/seed/demo/serve only; source/action groups planned |
| Health / API server | `internal/api/api.go` | **Verified** | 6 read ops; no auth |
| Web UI | `internal/ui/ui.go`, `page.html` | **Implemented** | 5 fixed questions; no open Q&A |
| Demo harness | `cmd/tron/demo.go` | **Verified** | 6 scenarios pass/fail |

### Knowledge model
| Component | File | Status | Gaps |
|---|---|---|---|
| Graph layer | `internal/graph/graph.go` | **Verified** | one global mutex (`ponytail:` ceiling) |
| Fact-write contract | `internal/graph/load.go` | **Verified** | key shape, provenance minimum, referential integrity, temporal windows; `internal/graph/load_test.go` |
| Entity resolution | `internal/knowledge/resolve.go` | **Verified** | single-source; no cross-source merge |
| Expertise | `internal/knowledge/expertise.go` | **Verified** | weights unvalidated (R10) |
| Decision trace | `internal/knowledge/decision.go` | **Verified** | — |
| Incidents | `internal/knowledge/incidents.go` | **Verified** | — |
| Contradiction detection | — | **Not started** | M1 |
| Staleness surfacing | — | **Not started** | M1 |

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
| Eval suites E1–E12 | **In progress** | E1/E2/E4/E5 partial; rest planned |
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

## Update rules

- Change a status only with a code path, test, or command reference.
- New work enters via a milestone ([`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md));
  unscheduled ideas go to the backlog with a reason.
- Keep this file and the two above in sync: milestone status here must match
  the milestone section there.
