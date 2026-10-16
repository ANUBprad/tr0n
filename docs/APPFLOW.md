> **Current competition build (2026-10-16):** See `TRACKER.md` and the root README for implemented behavior. The build now includes bounded natural-language routing, inspectable source content, handoff drafts, structured file imports, and sourced ownership updates. General-purpose LLM reasoning, live connectors, and autonomous execution remain future work.

# TRON — Application Flow, Interfaces & Product Design

**Status**: Canonical. Authority for CLI, API, and UI responsibilities,
commands, workflows, and interaction design.
**Last updated**: 2026-10-15.
**Related**: [`PRD.md`](PRD.md), [`TECHSPEC.md`](TECHSPEC.md),
[`REASONING.md`](REASONING.md), [`ACTIONS.md`](ACTIONS.md),
[`SECURITY.md`](SECURITY.md), `AGENT_SPEC.md`.

> **Existing vs proposed is marked on every command and endpoint.** Existing
> surfaces are `cmd/tron/main.go`, `internal/api/api.go`, `internal/ui/ui.go`.
> A command marked PROPOSED is a design, not a claim it works.

---

## 1. Interface responsibilities

| Interface | Owns | Never does |
|---|---|---|
| **CLI** (`cmd/tron`) | Operator/dev tasks: import, sync, inspect, query, diagnose | HTTP-only concerns |
| **JSON API** (`internal/api`) | Stable programmatic access; versioned `/v1` | Business logic beyond graph ops |
| **Web UI** (`internal/ui`) | Human Q&A, evidence, plans, approvals | Invent or override answers |
| **Agent layer** (`internal/agent`) | Tool selection + phrasing | Authorize, execute, or write facts |

**Thin clients** (`architecture-minimal.md`): no business logic in CLI/UI.
All three call the same API/operations so behavior can't diverge.

## 2. CLI command groups

Convention: `tron <group> <verb> [args]`. ✅ = exists today; ⏳ = proposed.

### 2.1 Sources & ingestion
| Command | Purpose | Inputs | Outputs | Errors | Permission | Status |
|---|---|---|---|---|---|---|
| `tron source add` | Register a source + credentials ref | type, name, config | source id | bad config, unreachable | admin | ⏳ |
| `tron source list` | Show sources | — | table | — | read | ⏳ |
| `tron import <path>` | One-off file/structured import | path, source type | ingestion record | parse errors, per-record quarantine | ingest | ⏳ |
| `tron sync [source]` | Incremental sync | source id (or all) | per-source counts | rate limit, partial failure | ingest | ⏳ |
| `tron ingest status` | Show cursors, last run, failures | source id | status table | — | read | ⏳ |
| `tron seed` | Recreate synthetic graph (**destructive**) | — | node/edge totals | FalkorDB unreachable | dev only | ✅ |

### 2.2 Inspection
| Command | Purpose | Inputs | Outputs | Errors | Permission | Status |
|---|---|---|---|---|---|---|
| `tron resolve <name>` | Resolve free text to key(s) | name, optional label | key or candidates | none (candidates = not-found) | read | ✅ (via API/UI; CLI op pending) |
| `tron entity <key>` | Show node + edges | key | node + edges + provenance | not found | read | ⏳ |
| `tron evidence <key...>` | Provenance drill-down | keys | nodes + edges + provenance | unknown keys flagged | read | ✅ (op exists; CLI wrapper ⏳) |

### 2.3 Organizational queries
| Command | Purpose | Inputs | Outputs | Errors | Permission | Status |
|---|---|---|---|---|---|---|
| `tron owner <entity>` | Who owns this | name/key | owners + OWNS evidence | none | read | ✅ |
| `tron experts <service>` | Who knows this best | service key, limit, now | ranked INFERENCE + basis | none | read | ✅ (op/API; CLI wrapper ⏳) |
| `tron decision <ref>` | Why was this decided | key/title | trace + evidence | not found | read | ✅ (op/API; CLI wrapper ⏳) |
| `tron incidents <service>` | Has this happened | service, since, keyword | incidents + resolvers | bad RFC3339 | read | ✅ (op/API; CLI wrapper ⏳) |
| `tron ask "<question>"` | NL question via agent | question | answer + claims + evidence | no provider configured | read | ⏳ |

### 2.4 Decision support & actions
| Command | Purpose | Inputs | Outputs | Errors | Permission | Status |
|---|---|---|---|---|---|---|
| `tron plan <goal>` | Options + trade-offs | goal ref | options, Pareto/weights | insufficient data | read | ⏳ |
| `tron action preview` | Draft an action from a recommendation | recommendation ref | proposed action | — | read | ⏳ |
| `tron action authorize <id>` | Record authorization | action id, scope | approval record | policy deny | action:authorize | ⏳ |
| `tron action execute <id>` | Execute approved action | action id | execution report | not authorized, target fail | action:execute | ⏳ |
| `tron action status <id>` | Execution + verification state | action id | state + outcome | not found | read | ⏳ |

### 2.5 Demo & diagnostics
| Command | Purpose | Inputs | Outputs | Errors | Permission | Status |
|---|---|---|---|---|---|---|
| `tron demo` | Run 5 scenarios + temporal as-of + evidence as pass/fail | — | `[ok]/[FAIL]` lines | non-zero on failure | read | ✅ |
| `tron serve` | Serve UI + JSON API | `TRON_HTTP_ADDR` | listening banner | bind failure | — | ✅ |
| `tron doctor` | Check FalkorDB, version, graph, indexes | — | diagnostics | connectivity | read | ⏳ |
| `tron eval` | Run the evaluation suite | suite name | metrics | eval config | read | ⏳ |

## 3. JSON API (`/v1`)

Existing endpoints (`internal/api/api.go`), all `POST` except health; all
read-only; strict JSON (unknown fields rejected), 1 MiB body cap.

| Endpoint | Body | Returns | Status |
|---|---|---|---|
| `POST /v1/owner` | `{target, as_of?}` | `Owner[]` | ✅ |
| `POST /v1/experts` | `{service_key, limit?, now?, as_of?}` | `ExpertiseResult` (INFERENCE) | ✅ |
| `POST /v1/decision` | `{ref}` | `DecisionTrace` \| 404 | ✅ |
| `POST /v1/incidents` | `{service_key, since?, keywords?}` | `IncidentRef[]` | ✅ |
| `POST /v1/evidence` | `{keys[]}` | `NodeEvidence[]` | ✅ |
| `POST /v1/resolve` | `{name, label?}` | `Resolution` | ✅ |
| `POST /v1/conflicts` | `{relation?, key?}` | `{layer, rule_id, conflicts[]}` | ✅ (E6 v1) |
| `GET /v1/health` | — | `{status}` \| 503 | ✅ |
| `POST /v1/ask` | `{question, as_of?}` | `Answer` envelope | ⏳ |
| `POST /v1/plan` | `{goal, options[]?}` | options + basis | ⏳ |
| `POST /v1/actions/{preview,authorize,execute}` | action payload | action/execution record | ⏳ |

**Contract**: successful data returns the typed JSON; failures return
`{"error": "..."}` with a 4xx/5xx code; internal errors never leak details
(`internal/api/api.go` `serverError`).

**`POST /v1/conflicts` (E6 v1, IMPLEMENTED 2026-10-16)**: same conventions
as the other read ops — unknown fields rejected, read-only. Empty result is
success (`200` with `{"conflicts": []}`), not 404; an unknown `relation`
value is a 400. Contract detail in `AGENT_SPEC.md`, decision in
`planning/decision-log.md`.

**`as_of` on `/v1/owner` and `/v1/experts` (E5, IMPLEMENTED 2026-10-16)**:
optional RFC3339 instant; a malformed value is a 400. It selects the validity
instant (`valid_from <= as_of < valid_to`); omitted means current state. Not
offered on event reads (e.g. incidents) — events carry their own timestamp.

## 4. Web UI

Current (`internal/ui/ui.go`, `internal/ui/page.html`): server-rendered page,
five **fixed** questions, answer text + evidence tables/notes. No JavaScript
toolchain.

Design principles for the target UI:

1. **Evidence always visible.** Every answer shows its claims, layer tags,
   paths, and provenance next to the text — never behind a tab.
2. **Measured vs inferred vs recommended is visually distinct.** A color/label
   per layer; a RECOMMENDATION must look different from a FACT.
3. **Uncertainty is shown, not hidden.** Intervals and contradictions render
   as first-class content.
4. **Traceability.** A fact links to its `src_ref`; an answer lists
   `operations_used`.
5. **Approval is explicit.** Any action shows the exact payload, scope, and
   consequences before authorization (never a one-click irreversible default).
6. **Accessible & minimal.** Semantic HTML, keyboard navigable, no decorative
   noise (`MVP_SCOPE.md`: demo > decoration).

## 5. Representative workflows

### 5.1 Ask "who knows payments-api best?" (CURRENT)
1. UI sends the fixed question → `FindExperts("service:payments-api")`.
2. Deterministic ranking (`expertise/v1`) with basis paths.
3. `Answer.Validate` passes (INFERENCE claims carry `rule_id` + paths).
4. UI renders answer + ranked table + basis notes.

### 5.2 Ask an open question (PROPOSED)
1. `POST /v1/ask` → agent loop selects `resolve_entity` → `find_*`.
2. Evidence assembled and permission-filtered.
3. LLM phrases claims; each claim must cite paths/rule_id.
4. `Validate` refuses any ungrounded claim; answer or abstention returned.

### 5.3 Execute an approved action (PROPOSED)
1. `tron action preview` drafts a payload from a recommendation.
2. `tron action authorize` records an approval (scope-bound).
3. `tron action execute` runs it with an idempotency key.
4. `tron action status` shows verification; outcome becomes a fact.

## 6. Failure cases and expected behavior

| Failure | Expected behavior |
|---|---|
| FalkorDB down | `serve` health → 503; CLI prints a seed/setup hint (`cmd/tron/demo.go`) |
| Unknown/ambiguous entity | Candidates returned; no guess |
| No supporting evidence | Abstain with an explicit "insufficient evidence" claim |
| Malformed API body | 400, stable error message, no internals leaked |
| Question outside data range | Say so; do not extrapolate silently |
| Action unauthorized | Denied with reason; never executed |
| Duplicate action delivery | Executes once (idempotency key) |

## 7. CLI/API/UI parity rule

Any capability added to one interface should be reachable through the API and
exposed in the CLI for operators. The UI is a convenience layer over the API,
never a separate implementation. This keeps the "thin clients" promise
auditable.
