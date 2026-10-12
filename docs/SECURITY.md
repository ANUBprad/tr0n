# TRON — Security, Privacy & Reliability

**Status**: Canonical. Authority for security posture and controls.
**Last updated**: 2026-10-12.
**Supersedes**: [`SECURITY_AND_DATA.md`](SECURITY_AND_DATA.md) (kept for history).
**Related**: [`ACTIONS.md`](ACTIONS.md), [`INGESTION.md`](INGESTION.md),
[`TECHSPEC.md`](TECHSPEC.md), [`EVALUATION.md`](EVALUATION.md), `AGENTS.md`.

> Each control is labeled **CONFIRMED** (code evidence, cited) or
> **PROPOSED**. Nothing is marked implemented without a code reference.

---

## 1. Threat model (what matters most)

1. **Derived-data leakage** — an answer or aggregate reveals a fact from a
   source the asker cannot see.
2. **Unsafe action** — TRON performs an unauthorized or irreversible effect.
3. **Prompt injection** — ingested content instructs the agent to do
   something it should not (raise privileges, exfiltrate, act).
4. **Data loss / corruption** — ingestion or a write corrupts memory.
5. **Secret exposure** — credentials leak into logs, responses, or the repo.
6. **Denial / resource exhaustion** — a request or source floods memory/CPU.

## 2. Authentication & authorization

| Control | State |
|---|---|
| User/service authentication | **PROPOSED** (absent) |
| Role- or scope-based authorization | **PROPOSED** |
| Action-level authorization gate | **PROPOSED** ([`ACTIONS.md`](ACTIONS.md)) |
| Read-only default | **CONFIRMED** — API/agent never write (`internal/api/api.go`, `architecture-minimal.md`) |

Today there is **no authentication**: `tron serve` binds locally and trusts
the caller. This is acceptable only because it is loopback-only (§5). Any
network exposure without auth is a critical defect.

## 3. Tenant & organization isolation

- **Current**: single organization; a single FalkorDB graph named `tron`
  (`cmd/tron/main.go` `connect`). The documented isolation point is **one graph
  per tenant, chosen at connect time** (`architecture-minimal.md`). This is
  designed but **not exercised** — treat as PROPOSED/unverified.
- **Rule**: isolation must be enforced at the data layer (graph selection),
  not only by filtering query results.

## 4. Source permissions & information leakage (the hard one)

- Every fact carries the source's access scope (`SCHEMA.md`, `INGESTION.md`).
- **Derived leakage** is the primary risk: an inference (expertise, a metric,
  a count) built from a hidden fact can leak it. Controls (PROPOSED):
  1. compute answers **within** the asker's permitted fact set;
  2. re-check aggregate/inference inputs against scope, not just outputs;
  3. treat embeddings/summaries as derived data inheriting the most
     restrictive source scope.
- Revoking source access must remove derived visibility, not only raw rows.

## 5. Secrets & network exposure

| Control | State |
|---|---|
| No secrets in repo; config via env (`TRON_FALKOR_ADDR`, `TRON_HTTP_ADDR`) | **CONFIRMED** (`cmd/tron/main.go`) |
| `.gitignore` excludes `.env*`, `*.json`, `*.csv`, build/test artifacts | **CONFIRMED** (`.gitignore`) |
| FalkorDB bound to loopback only (`127.0.0.1:6379`) | **CONFIRMED** (`compose.yaml`; commit `60cc5a7`) |
| FalkorDB auth enabled by default | **ABSENT** — `compose.yaml` notes `REDIS_ARGS=--requirepass` is a manual opt-in |
| API bound to loopback by default (`127.0.0.1:8080`) | **CONFIRMED** (`cmd/tron/main.go`) |
| Secret handling for real sources | **PROPOSED** ([`INGESTION.md`](INGESTION.md)) |

## 6. Input validation & resource limits

| Control | State |
|---|---|
| Strict JSON: unknown fields rejected; trailing data rejected | **CONFIRMED** (`internal/api/api.go` `decode`) |
| Request body capped at 1 MiB | **CONFIRMED** (`maxBodyBytes`, `http.MaxBytesReader`) |
| HTTP server timeouts (read header/read/write/idle) | **CONFIRMED** (`cmd/tron/main.go` `newServer`) |
| RFC3339 validation on time inputs | **CONFIRMED** (`internal/api/api.go`, `internal/graph/graph.go`) |
| Query result / context size bounds | **PARTIAL** — deliberate retrieval keeps payloads small by design; no hard cap yet |
| Rate limiting | **PROPOSED** |

## 7. Prompt injection & untrusted content

- **Rule (CONFIRMED as policy, PROPOSED as enforcement)**: ingested documents,
  messages, tickets, and web content are **data, never instructions**
  (`knowledge-model.md` invariant 3, [`ACTIONS.md`](ACTIONS.md) §8).
- The LLM may not grant permissions, raise autonomy, or broaden scope.
- Because no LLM is wired today (`internal/agent/agent.go` has no provider),
  this risk is **latent**: it activates when a provider is connected. The
  control must exist *before* that.
- Defense: deterministic AUTHZ gate outside the model; validation of
  action parameters against approved scope; never execute instructions
  recovered from content.

## 8. Safe tool execution

- Tools exposed to the model are the six read-only operations
  (`internal/agent/agent.go` `Tools`); they cannot write (`CONFIRMED`).
- Future write/action tools must go through the AUTHZ gate and be
  `action:*`-scoped ([`ACTIONS.md`](ACTIONS.md)) — **PROPOSED**.
- Tool errors are fed back to the model, not fatal (`internal/agent/agent.go`),
  which prevents an error from escalating into an unbounded loop
  (bounded by `MaxSteps`).

## 9. Concurrency, idempotency, retries, recovery

| Concern | State |
|---|---|
| Graph access concurrency | **CONFIRMED** — one global mutex serializes graph calls (`internal/graph/graph.go`; `ponytail:` documented ceiling) |
| Connection pool | **CONFIRMED** — pooled Redis client (`falkordb-go`/`go-redis`), stateless commands |
| Ingestion idempotency | **PROPOSED** ([`INGESTION.md`](INGESTION.md) §6) |
| Action idempotency | **PROPOSED** ([`ACTIONS.md`](ACTIONS.md) §5) |
| Retry policy | **PROPOSED** |
| Crash recovery | **PROPOSED** — no persistent ingestion/action state today |

## 10. Logging without leakage

- Errors are logged server-side and returned as a stable, non-sensitive body
  (`serverError`: `internal/api/api.go`; commit `792b00e` "sanitize HTTP error
  responses"). **CONFIRMED.**
- **Risk to watch**: log lines currently include query keys/refs
  (`log.Printf("api: %v", err)`). These are not secrets today (synthetic
  data), but must be audited once real data lands (PROPOSED control).
- Secrets (tokens, passwords) must never be logged; the future credential
  store must redact (`INGESTION.md`, PROPOSED).

## 11. Auditability, backup, recovery, retention

| Control | State |
|---|---|
| Answer auditable via evidence chain | **CONFIRMED** — evidence *is* the audit trace (`architecture-minimal.md` principle 10) |
| Action audit log | **PROPOSED** ([`ACTIONS.md`](ACTIONS.md) §7) |
| Backup | **ABSENT** — `compose.yaml` has no volume; the graph is disposable by design (`ponytail:` comment) |
| Recovery | **PROPOSED** — `tron seed` recreates demo data; no real recovery path |
| Retention policy | **PROPOSED** ([`INGESTION.md`](INGESTION.md) §7) |

## 12. Dependencies & container images

| Control | State |
|---|---|
| Minimal dependencies | **CONFIRMED** — one direct dep (`falkordb-go/v2`), stdlib HTTP/JSON (`go.mod`) |
| Pinned dependency versions | **CONFIRMED** — `go.mod`/`go.sum` |
| Container image | `falkordb/falkordb:latest` — **UNPINNED tag** (risk: upstream drift). Proposal: pin a digest. |
| Vulnerability scanning / SBOM | **PROPOSED** |
| Reproducible builds | **PARTIAL** — `CGO_ENABLED=0` static binary; no release pipeline yet |

## 13. Current risk register (security subset)

| Risk | Severity | State | Mitigation |
|---|---|---|---|
| No auth if exposed beyond loopback | High | Open | Default loopback; add authN before any network exposure |
| FalkorDB no password | Medium | Open | Loopback bind; `--requirepass` opt-in |
| Derived-data leakage | High | Latent (no real sources) | Permission-aware retrieval (PROPOSED) |
| Prompt injection | High | Latent (no LLM) | Untrusted-data rule + AUTHZ gate before wiring a provider |
| Unpinned FalkorDB image | Low | Open | Pin image digest |
| No backup/retention | Medium | Open | Define before real data |
| Logs may carry query keys | Low | Watch | Audit logging at real-data time |

Full cross-cutting register lives in [`RULES.md`](RULES.md) §Risk Register.
