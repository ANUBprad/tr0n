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
| Read-only API | Verified | `/v1/ask`, `/v1/catalog`, seven lower-level operations; real HTTP integration tests |
| Docker packaging and persistence | Implemented | Dockerfile, digest-pinned FalkorDB, named volume, append-only logging, health checks |
| Model-loop cancellation and call bounds | Verified | `TestLoopStepBudgetAndInvalidMoves` |
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
- E3 remains PARTIAL; local-model evaluations are now executable, with structural validity and answer relevance reported separately. Participant semantic review and comparison baselines remain outstanding.

- M3: `AssembleEvidence` and `assemble_context` supply deterministic one-hop context with relationship/time filters, count and byte bounds, and explicit truncation. Verified by `TestAssembleEvidence*` and `TestAssembleContextToolSuppliesEvidence`.
- E1: adversarial `TestResolveEntityContract` retained; key-shaped name-tier ambiguity remains documented in EVALUATION.md.
- E5/E6 temporal and conflict suites retained. Broader E1–E12 evaluation and comparison baselines remain incomplete.

## Deliberate boundaries

- Five deterministic question intents plus optional local Ollama retrieval and cited relationship selection. No unconstrained text-to-Cypher.
- Synthetic dataset, structured imports, and explicit Markdown snapshots with retry/cursor/deletion handling; no private service connectors.
- One writer per tenant graph. Optional server-owned bearer credentials select tenant graphs; no per-document permissions or managed identity.
- Document refresh replaces the latest body; it is not a bitemporal document archive. Ownership changes retain edge history.
- Validity windows use whole-second canonical UTC precision. Subsecond window endpoints are rejected rather than silently truncated.
- As-of expertise filters ownership, not all historical activity.
- M4 planning, M5 approved actions, and M6 alerts remain gated on M3 evaluation and participant review. M7 has partial tenant isolation, not production readiness.

Merged-code runtime verification is recorded in [VERIFICATION.md](VERIFICATION.md).
