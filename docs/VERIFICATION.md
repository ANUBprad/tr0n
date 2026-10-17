# Integration verification

## Current release checks — 2026-10-17

- Runtime revision: `c31b3e3` on `submission/main` (`Shashwat-911/tr0n-shashwat`); this verification pass adds documentation corrections only.
- `go test -race -count=1 -p 1 ./...`: all packages pass in a Go 1.26.5 Linux container connected to the running FalkorDB service.
- `go vet ./...` and `node --check internal/ui/app.js`: pass.
- `docker compose run --rm app demo`: 7/7 graph-backed checks pass.
- `GET /v1/health`: `{"status":"ok"}`; app, FalkorDB, and local Ollama containers are running.
- `GET /v1/model` reports the configured `qwen3:4b-instruct`; a live `POST /v1/grounded` ownership question returned the recorded owner and a cited relationship fact.
- The local Qwen 3 4B model run and its limitations are recorded in
  [`EVALUATION.md`](EVALUATION.md); participant semantic review remains pending
  in [`MODEL_REVIEW.md`](MODEL_REVIEW.md).
- The prior UI walkthrough is a captioned sequence of screenshots. It does not
  demonstrate the newer optional Ollama answering flow.

The race-enabled full suite, static checks, demo, and health probe were rerun on
this machine for this checkpoint. Model evaluation metrics are from the dated run
in `EVALUATION.md`, not a fresh rerun in this verification pass.

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
