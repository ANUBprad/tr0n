# TRON — implementation tracker

Updated 2026-10-17. This describes the local Track 03 build. The broader M0–M7 production roadmap remains in IMPLEMENTATIONPLAN.md; its proposed features are not submission claims.

| Capability | Status | Evidence |
|---|---|---|
| FalkorDB model and five core traversals | Verified | Live graph suite; `tron demo` 7/7 |
| Temporal ownership and conflict detection | Verified | Graph/API tests; current reads now use actual time, historical provenance includes valid_to |
| Current-owner correction for scheduled handoffs | Verified | `TestCurrentOwnerIsNotFutureOwner` |
| Question router and clarification | Verified | `internal/agent/ask.go`; `TestQuestionRouting`, `TestAskLiveGraph` |
| Decision rationale, ADR/chat/ticket source content | Verified | Seed + `TestAskLiveGraph`; source records inspectable in UI |
| Handoff draft | Verified | `Ask` composes evidence and a Markdown draft; never sends or assigns work |
| Source freshness display | Implemented | Source observation timestamps shown without a universal stale threshold |
| Structured file import and refresh | Verified | `ImportRecord`; idempotency, content/link updates, stale/collision rejection tests |
| Sourced ownership updates | Verified | `RecordOwner`; atomic close/open, history, no-op retry, invalid/backdated rejection tests |
| Cross-load duplicate identity guard | Verified | `TestDuplicateKeyAcrossLoadsIsRejected`; per-client single-writer guard, external writers not constrained |
| Browser workspace | Implemented and browser-checked | `workspace.html`, `app.js`, `app.css`; typed questions, context picker, sources, SVG context and handoff download |
| Read-only API | Verified | `/v1/ask`, `/v1/grounded`, `/v1/model`, `/v1/catalog`, health, seven graph operations; real HTTP integration tests |
| Docker packaging and persistence | Implemented | Dockerfile, digest-pinned FalkorDB, named volume, append-only logging, health checks |
| Model-loop cancellation and call bounds | Verified | `TestLoopStepBudgetAndInvalidMoves` |
| Grounded local-model Q&A & review | Partial | `/v1/grounded`, Ollama adapter, bounded retrieval/citations; ten synthetic samples reviewed; no broad entailment evaluation |
| Comparative baseline benchmark | Not implemented | Search/RAG baselines have not been run; earlier hard-coded example scores were removed |
| Decision support & option comparison (M4) | Partial | `internal/rules`, `POST /v1/plan`, `tron plan`; validated caller-scored options, Pareto and weight perturbation; scores are not graph-sourced |
| Controlled action execution (M5) | Partial | Authenticated tenant roles can create local drafts; in-memory replay/audit only; no durable state or external execution; state changes denied |
| Proactive organizational alerts (M6) | Partial | Snapshot graph rules report conflicts and service ownership risks; no labeled quality evaluation |
| Markdown source snapshots | Verified | Explicit manifest, idempotent retry, retained cursor on partial failure, source-owned deletion; see `LOCAL_MODEL_AND_SYNC.md` |
| Identity and database constraints | Implemented | Reviewed aliases, per-label unique constraints (`tron constraints`), cross-label application check |
| Tenant graph selection | Partial | Server-owned bearer credentials and UI workspace switching; no per-document ACL or managed identity |
| Demo walkthrough | Included | Captioned UI-state walkthrough in `artifacts/demo`; README includes a public download URL. Team review and optional live narration remain. |

## Roadmap additions under quality review

- Local Ollama adapter behind `Decide`, `/v1/grounded`, bounded connected retrieval,
  strict citations, server-rendered relationship facts, and visible abstention/truncation.
- Executable labeled model corpus with latency/token measurements; review artifacts
  distinguish automated rubric results from participant semantic judgement.
- Markdown snapshot manifests, idempotent retries, source-owned deletion, retained cursor
  on partial failure, and explicit identity mappings.
- Per-label database uniqueness, canonical-only `key=` lookup, reviewed aliases reserved
  across application writers, and conflicting-citation rejection.
- Credential-selected tenant graphs and UI cleanup when changing workspaces.
- See [operational setup and boundaries](LOCAL_MODEL_AND_SYNC.md).

## Integrated upstream work

- E3: stable content-derived `EvidenceFact.ID` and `GroundedAnswer` validate FACT citations against carried evidence, subjects, relationships, and context status. Covered by `TestGroundedAnswer*`. This is structural grounding, not semantic entailment; the existing `/v1/ask` response is not automatically converted to this new contract.
- M3 audit: total fact ordering and explicit `over_budget` outcomes are retained, including malformed-window, boundary, and size-limit tests.
- E3 remains PARTIAL: the ten reviewed synthetic examples and automated rubric are limited evidence; no comparative baseline or broad entailment evaluation exists.
- M4 remains PARTIAL: `rules.EvaluatePlan` calculates a ranking over caller-supplied scores; it does not derive option scores from graph evidence.
- M5 remains PARTIAL: only authenticated local draft generation is available. Replay and audit records are process-local; no target mutation or verified outcome is implemented.
- M6 remains PARTIAL: `rules.DetectAlerts` detects candidate snapshot risks, but `TestDetectAlerts` does not establish precision/recall.

- M3: `AssembleEvidence` and `assemble_context` supply deterministic one-hop context with relationship/time filters, count and byte bounds, and explicit truncation. Verified by `TestAssembleEvidence*` and `TestAssembleContextToolSuppliesEvidence`.
- E1: adversarial `TestResolveEntityContract` retained; key-shaped name-tier ambiguity remains documented in EVALUATION.md.
- E5/E6 temporal and conflict suites retained. E9–E11 have focused prototype tests, not full evaluation suites.

## Deliberate boundaries

- Five deterministic question intents plus optional local Ollama retrieval and cited relationship selection. No unconstrained text-to-Cypher.
- Synthetic dataset, structured imports, and explicit Markdown snapshots with retry/cursor/deletion handling; no private service connectors.
- One writer per tenant graph. Optional server-owned bearer credentials select tenant graphs; no per-document permissions or managed identity.
- Document refresh replaces the latest body; it is not a bitemporal document archive. Ownership changes retain edge history.
- Validity windows use whole-second canonical UTC precision. Subsecond window endpoints are rejected rather than silently truncated.
- As-of expertise filters ownership, not all historical activity.
- M7 has partial tenant isolation, not production readiness.

Merged-code runtime verification is recorded in [VERIFICATION.md](VERIFICATION.md).
