# TRON — Product Requirements Document (PRD)

**Status**: Canonical, living. Authority for product intent, users, and
requirements. Supersedes `PROJECT.md`, `PRODUCT_SPEC.md`, and
`product-definition.md` (kept for history only).
**Last updated**: 2026-10-12.
**Related**: [`TECHSPEC.md`](TECHSPEC.md), [`SCHEMA.md`](SCHEMA.md),
[`architecture-minimal.md`](architecture-minimal.md) (current implementation),
[`knowledge-model.md`](knowledge-model.md).

> **Honesty rule** (from `AGENTS.md` and `opencode.md`): this document
> separates what is **IMPLEMENTED**, what is **NEAR-TERM**, and what is
> **VISION**. Nothing here should be read as a claim that an unbuilt
> capability exists. Verified claims cite a repository path.

---

## 1. Problem

Organizations accumulate knowledge across disconnected tools — tickets,
documents, chat, pull requests, incidents, calendars, registries. When
someone asks an organizational question, the answer usually requires
joining context that no single tool holds:

- "Who owns this service, and who actually knows it best?"
- "Why was this decision made, and what evidence supports it?"
- "Has this failure happened before, and who resolved it?"
- "What depends on this, and what breaks if we change it?"
- "Given our goals and constraints, what should we do next?"

Keyword search and conventional vector RAG retrieve *passages*. They lose
the relationships that make the answer correct: who said it, when, why,
and which other facts depend on it. TRON's premise (locked in
`architecture-minimal.md`) is that organizational knowledge is
**relational** and must be reasoned over as a graph with **provenance**.

## 2. Users and jobs to be done

| User | Job to be done | Today's pain |
|---|---|---|
| On-call engineer | Understand an incident's history, owner, and likely fixer fast | Context scattered across incident tool + chat + PRs |
| Tech lead / architect | Trace why a decision was made; assess blast radius | Decisions live in meetings and stale docs |
| New team member | Learn who knows what and how systems connect | Tribal knowledge, no map |
| Engineering manager | See ownership gaps, skill concentration, risk | No cross-tool view |
| Product / PM | Understand project status, dependencies, commitments | Status assembled by hand |

Long-term (VISION) TRON also serves executive decision support: goals,
budgets, risk, and authorized actions across company systems.

## 3. The organizational-brain vision

TRON is a **Company Brain**: a system that

1. **Understands** company data and relationships (people, teams, projects,
   services, documents, decisions, incidents, commitments).
2. **Explains** its answers with evidence and provenance, never presenting
   inference as fact.
3. **Reasons** about goals, constraints, dependencies, and risk.
4. **Recommends** decisions and actions.
5. **Executes** authorized actions safely, with approval and audit.

The distinguishing claim is **graph-central, evidence-driven reasoning**:
remove the graph engine and the core capability materially degrades. This
is enforced, not aspirational — the deterministic core
(`internal/graph`, `internal/knowledge`) runs with no LLM and no UI
(`README.md`, `architecture-minimal.md`).

## 4. Current vs near-term vs long-term

> **Current competition build (2026-10-17):** The deterministic five-workflow
> app, structured import and Markdown snapshots, bounded local Ollama retrieval,
> citation validation, and tenant graph selection exist. The optional model path
> is evaluated on a small synthetic corpus; semantic participant review and
> baseline comparisons are still open. See [`TRACKER.md`](TRACKER.md) and
> [`EVALUATION.md`](EVALUATION.md). The milestones below retain the original
> product horizon and are not a claim that all of it has shipped.

| Horizon | Definition | State |
|---|---|---|
| **CURRENT (IMPLEMENTED / PARTIAL)** | Deterministic graph operations + provenance + bounded local-model retrieval and citations over synthetic and explicitly imported snapshots. No LLM required for deterministic workflows. | See [`TRACKER.md`](TRACKER.md) |
| **NEAR-TERM (next milestones)** | Broader measured answer quality, decision support, controlled action execution, comparative evaluation. | Partial / proposed — see [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md) |
| **LONG-TERM (VISION)** | Proactive organizational intelligence, multi-source synchronization, permission-aware context, authorized cross-system actions. | Proposed — not scheduled as committed work |

### What CURRENT actually does (verified)

- Seven deterministic graph operations over FalkorDB: `FindOwner`,
  `FindExperts`, `TraceDecision`, `FindRelatedIncidents`, `TraceEvidence`,
  `ResolveEntity`, `FindConflicts` (`internal/graph`).
- Layered knowledge enforcement: FACT / INFERENCE / RECOMMENDATION, with
  `Answer.Validate` refusing untagged or unevidenced claims
  (`internal/knowledge/answer.go`).
- Deterministic expertise ranking `expertise/v1`
  (`internal/knowledge/expertise.go`).
- Read-only JSON API with health/catalog, deterministic and grounded ask, model
  status, and seven graph operations (`internal/api/api.go`).
- Server-rendered Q&A UI with evidence panels (`internal/ui/ui.go`).
- Provider-agnostic agent loop with local Ollama adapter
  (`internal/agent/agent.go`, `internal/agent/ollama.go`); answer-quality
  evaluation remains partial (`docs/EVALUATION.md`).
- Deterministic synthetic seed graph, 50 people / 339 nodes / 958 edges
  (`internal/seed/seed.go`, `README.md`).
- `tron demo` pass/fail verification of five workflows, temporal ownership,
  and evidence drill-down (`cmd/tron/demo.go`).

### What is deliberately NOT built (verified absent)

- No live private-service connectors or arbitrary-prose fact extraction;
  ingestion is explicit and uses structured records or declared Markdown files.
- No hosted model provider or unrestricted free-form factual synthesis.
- No write path: API and agent are read-only w.r.t. the graph.
- No action execution, approval workflow, or audit log.
- Tenant graph selection is supported by server-owned bearer credentials, but
  per-document authorization and production identity management are not.
- No multi-source identity merge; aliases require an explicit reviewed mapping.

## 5. Flagship demonstration

`tron demo` (`cmd/tron/demo.go`) runs five DEMO_SPEC scenarios plus an
evidence drill-down as pass/fail checks against a real FalkorDB:

1. Who owns `payments-api`? (`FindOwner`)
2. Who knows `payments-api` best? (`FindExperts`, tagged INFERENCE)
3. Why did we adopt FalkorDB? (`TraceDecision`, `decision:1`)
4. Has `payments-api` timed out before? (`FindRelatedIncidents`)
5. Who should handle a `payments-api` issue? (RECOMMENDATION composed from
   owner + expert + incident history)

Each `[ok]` line is produced by a real traversal, not a canned string
(`cmd/tron/demo.go`). This demonstrates the product thesis at v1 scope:
**multi-hop, evidence-backed answers, with no LLM in the loop**.

The long-term flagship is broader: ingest a real (or realistic) corpus,
answer the same questions from genuine sources, and execute one authorized,
audited action end to end.

## 6. Functional requirements

Requirements are grouped by capability. IDs are stable references for
[`TECHSPEC.md`](TECHSPEC.md), [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md),
and [`EVALUATION.md`](EVALUATION.md).

### FR-1 Knowledge & memory
- **FR-1.1** Represent people, roles, teams, ownership, authority.
- **FR-1.2** Represent goals, projects, milestones, tasks, dependencies,
  commitments.
- **FR-1.3** Represent documents, meetings, decisions, rationale, approvals,
  evidence.
- **FR-1.4** Represent customers, contracts, products, services, incidents,
  risks, policies.
- **FR-1.5** Represent financial/budget/KPI/metric facts where sources exist.
- **FR-1.6** Every stored fact carries provenance and temporal validity
  (`knowledge-model.md`).
- **FR-1.7** Distinguish fact vs reported claim vs inference vs prediction vs
  recommendation vs approved decision vs verified action.
- **FR-1.8** Preserve original source documents; summaries never replace
  evidence.

*(FR-1.1–1.7 partially implemented for the 11-entity subset; the rest are
proposed. See [`SCHEMA.md`](SCHEMA.md).)*

### FR-2 Ingestion & synchronization
- **FR-2.1** Import files and structured data.
- **FR-2.2** Connect to APIs and receive events/webhooks.
- **FR-2.3** Incrementally synchronize and propagate deletions.
- **FR-2.4** Parse, normalize, deduplicate, resolve entities.
- **FR-2.5** Validate, assign confidence, isolate per-source failure.
- **FR-2.6** Propagate source permissions and retention to derived data.
- **FR-2.7** Be idempotent and observable. *(See [`INGESTION.md`](INGESTION.md).)*

### FR-3 Reasoning & retrieval
- **FR-3.1** Answer questions using graph-first, permission-aware retrieval.
- **FR-3.2** Ground every claim in a source path or a rule-tagged inference.
- **FR-3.3** Detect and surface contradictions, uncertainty, and missing info.
- **FR-3.4** Abstain or request clarification when evidence is insufficient.
- **FR-3.5** Keep deterministic business calculations deterministic.
  *(See [`REASONING.md`](REASONING.md).)*

### FR-4 Decision support
- **FR-4.1** Evaluate goals, budgets, deadlines, dependencies, risk.
- **FR-4.2** Compare options and generate recommendations with rationale.
- **FR-4.3** Return Pareto trade-offs, not only a single weighted answer.

### FR-5 Actions & authority
- **FR-5.1** Support read-only observation and recommendations today.
- **FR-5.2** Draft proposed changes and require human approval.
- **FR-5.3** Support bounded, pre-authorized low-risk execution.
- **FR-5.4** Require explicit governance for high-impact operations.
- **FR-5.5** Record authorization, execution, and verification; be idempotent.
- **FR-5.6** Treat external content as untrusted data, never as instructions.
  *(See [`ACTIONS.md`](ACTIONS.md).)*

### FR-6 Interfaces
- **FR-6.1** CLI for connection, ingestion, inspection, query, evidence,
  decisions, actions, diagnostics.
- **FR-6.2** Read-only JSON API (implemented) extended with reasoning/actions.
- **FR-6.3** Web UI for Q&A, evidence, plans, approvals. *(See [`APPFLOW.md`](APPFLOW.md).)*

### FR-7 Trust, safety, operations
- **FR-7.1** Authentication, authorization, tenant isolation.
- **FR-7.2** Secrets management; no secrets in repo.
- **FR-7.3** Auditability, backup, recovery, retention.
- **FR-7.4** Observability: latency, cost, error rates per operation.
  *(See [`SECURITY.md`](SECURITY.md), [`EVALUATION.md`](EVALUATION.md).)*

## 7. Non-functional requirements

| ID | Requirement |
|---|---|
| NFR-1 | Deterministic core: same graph + same clock → same result (`expertise/v1`). |
| NFR-2 | Every answer is explainable: claim + layer tag + evidence chain. |
| NFR-3 | Local-first: complete local setup works (Docker + one binary). |
| NFR-4 | Cross-platform: pure Go, `CGO_ENABLED=0`, linux/macOS/windows × amd64/arm64. |
| NFR-5 | Minimal dependencies: stdlib first; one graph client. |
| NFR-6 | No secret or sensitive-data leakage in logs or responses. |
| NFR-7 | Correctness over speed for facts; measure latency before optimizing. |
| NFR-8 | Graceful failure isolation: one bad source cannot corrupt others. |

## 8. Success criteria and measurable outcomes

Metrics must be defined against a dataset and baseline before numbers are
claimed (`EVALUATION.md`). No numerical target below is a measured result.

| Area | Outcome | How measured (proposed) |
|---|---|---|
| Answer correctness | Grounded answers on a labeled question set | % claims with valid supporting path |
| Citation support | Every claim cites source | `Answer.Validate` pass rate = 100% |
| Multi-hop reasoning | ≥3-hop questions answered correctly | accuracy on fixture questions |
| Entity resolution | Duplicate identities merged; no collisions | precision/recall on labeled pairs |
| Temporal accuracy | Correct "as-of" owner/state | accuracy on time-shifted fixtures |
| Contradiction detection | Conflicting facts surfaced, not hidden | recall on injected contradictions |
| Decision support | Recommendations judged useful/valid | expert rating vs baseline |
| Action safety | No unauthorized action executes | % actions blocked pre-authorization |
| Latency | Interactive Q&A | p50/p95 per operation |
| Cost | LLM/token spend per answered question | measured once LLM exists |

## 9. Non-goals

- **Not** a general-purpose chatbot or a document-Q&A wrapper.
- **Not** a replacement for primary systems of record (GitHub, HRIS, Jira):
  TRON indexes and reasons; those systems remain authoritative for their data.
- **Not** autonomous action without authorization.
- **Not** enterprise authZ, multi-tenant hardening, or cloud deployment now
  (`product-definition.md` non-goals).
- **Not** a broad connector zoo before the memory and reasoning core is
  trustworthy.

## 10. Constraints

- **Data**: public, synthetic, or explicitly-permitted data only; no private
  or login-protected data (`opencode.md`, `DATA_SPEC.md`).
- **Graph**: FalkorDB is the primary graph engine; graph must do real work.
- **Language**: Go, one dependency posture (`technology-evaluation.md`).
- **Scope discipline**: working > broad; deep > shallow; evidence > flashy.

## 11. Unresolved product questions

Tracked in [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md); the product-level ones:

- Which single vertical (synthetic) organization tells the clearest story, and
  how far does it need to generalize?
- Is the first real ingestion source a docs repo, an issue tracker, or a
  calendar? (Determines identity-resolution scope.)
- What is the minimum credible decision-support use case worth building?
- Which low-risk action, if any, is safe enough to be the first executed one?
- What is the acceptance bar for "trustworthy enough to demo to a skeptic"?

## 12. Document map

Product intent lives **only here**. Architecture, schema, ingestion,
reasoning, actions, flows, security, evaluation, roadmap, and rules live in
their own canonical documents — see [`README.md`](README.md) for the index
and authority map.
