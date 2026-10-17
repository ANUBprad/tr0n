# Integration verification

## Current release checks — 2026-10-17

- Working branch: `submission/company-brain`, based on `submission/main`; see git history for the published revision.
- `go test -count=1 ./...`: all packages pass against the running local FalkorDB service.
- `go test -race -count=1 -p 1 ./...`: all packages pass in Go 1.26.5 Alpine with `TRON_FALKOR_ADDR=falkordb:6379`.
- `go vet ./...`, `go build ./cmd/tron`, and `node --check internal/ui/app.js`: pass.
- `docker compose run --build --rm app demo`: rebuilt image passes 7/7 graph-backed checks.
- Action role checks cover unauthenticated and reader denial, server-owned requester identity, and role separation. State-changing action types are denied; local draft replay/audit state is in memory.
- Planning checks cover finite weights/scores, overflow rejection, Pareto ranking, and independent ±10% weight perturbations. Plan options remain caller-scored inputs.
- The local Qwen 3 4B model run and its limitations are recorded in [`EVALUATION.md`](EVALUATION.md); participant review of ten synthetic samples is recorded in [`MODEL_REVIEW.md`](MODEL_REVIEW.md).
- The prior UI walkthrough is a captioned sequence of screenshots. It does not demonstrate the newer optional Ollama answering flow.

## Preserved history

- Local completion checkpoint: `3059a55`.
- Upstream six-commit series through `5fa0255` merged in `92b03ea`.
- Working branch: `codex/company-brain-completion`. No force push or upstream history rewrite.
- Both implementations had added cross-load key uniqueness. The merged code uses the upstream guard and its fuller identity tests, retaining timestamp validation and the current-time ownership fix.
- The upstream evidence assembly tool and adversarial identity tests are retained. Freshness uses raw source observation timestamps, with no universal stale threshold.

## Checks completed on the merged code

- `go test ./...`: all seven packages pass against local FalkorDB.
- `go vet ./...` and `node --check internal/ui/app.js`: pass.
- `go test -race ./...`: all seven packages pass inside Go 1.26.5 Alpine with a compiler and live FalkorDB.
- Docker application image builds; app and database run in Compose.
- Fresh seed: 339 nodes and 958 edges. Demo: 7/7 checks pass.
- Sample import: two changed records on first import, zero changes on retry.
- FalkorDB restarted: imported records survive; demo remains 7/7 and HTTP health is OK.
- Browser: ownership, expertise, Redis decision rationale, source drill-down, incident history, and handoff responses checked. Handoff Markdown downloads successfully.
- Responsive widths 390 and 1440: no horizontal document overflow. Temporary viewport override reset.
- Captioned 60-second walkthrough encoded and a representative video frame inspected. This is a sequence of captured live UI states, not continuous recording.

## Submission steps still requiring the team

Review the published walkthrough and code, complete the answer-quality review, and explain your own contribution accurately. The implementation is a local prototype with an optional local Ollama adapter; hosted model support, live private connectors, production authorization, and full E1-E12 benchmark coverage are not claimed. See [`TRACKER.md`](TRACKER.md) and [`EVALUATION.md`](EVALUATION.md) for remaining scope and known identity ambiguity.

## Publication integration

The destination fork also contained `a662006`, adding explicit over-budget evidence outcomes and a total fact order. That fix and all its tests were preserved before organizing the completion into 13 commits on top of the destination main branch. The original local checkpoints remain available on `codex/company-brain-completion`.
