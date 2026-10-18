# Integration verification

## Current release checks — 2026-10-18

- `go test -count=1 -p 1 ./...`: all 11 packages pass against the running local FalkorDB service.
- `go vet ./...` and `go build -o tron.exe ./cmd/tron`: pass. `gofmt` is clean on LF-normalised copies (the checkout is CRLF). `-race` remains unavailable on this host (`go: -race requires cgo`, no C compiler); the CI/container job covers it.
- Grounded-answer loop: the deterministic progress guard (`commit d0dedca`) and the author-path fix (`commit 3ff7473`) are covered by no-model tests, and a fresh 3-trial run of the 21-case corpus scored **21/21 rubric and 21/21 citation-valid with zero step-budget exhaustion** (`qwen3:4b`, thinking off) — see [`RELIABILITY_REPORT.md`](RELIABILITY_REPORT.md) and the raw artifact `artifacts/evaluation/reliability-trials-2026-10-18.json`; the pre-fix artifact is preserved alongside it. Only `decision-source-authors` differs from the pre-fix run, and each of its three outputs was read individually, not just scored.

## Prior release checks — 2026-10-17

- Working branch: `submission/company-brain`, based on `submission/main`; see git history for the published revision.
- `go test -count=1 ./...`: all packages pass against the running local FalkorDB service.
- `go test -race -count=1 -p 1 ./...`: all packages pass in Go 1.26.5 Alpine with `TRON_FALKOR_ADDR=falkordb:6379`.
- `go vet ./...`, `go build ./cmd/tron`, and `node --check internal/ui/app.js`: pass.
- Docker Compose check (2026-10-17, host toolchain): `go test -count=1 -p 1 ./...` passes all 11 packages against local FalkorDB; `go vet ./...`, `go build -o tron.exe ./cmd/tron`, and `gofmt` (checked on LF-normalised copies; the checkout is CRLF) are clean. `-race` is **not** available on this host (`go: -race requires cgo`, no C compiler); the race run above is the CI/container check.
- Live CLI checks against the seeded graph: `seed` reports `339 nodes, 958 edges`; `demo` reports `7/7 demo checks passed`; `owner payments-api` resolves to `person:1`; `sync examples/markdown-source.json` reports `changed 2`; `import examples/sources.json` reports `2 changed`; then the graph is re-seeded.
- Grounded-answer loop: the deterministic progress guard (`commit d0dedca`) is covered by no-model tests, and a 3-trial run of the 21-case corpus scored 20/21 rubric and 21/21 citation-valid with zero step-budget exhaustion ([`RELIABILITY_REPORT.md`](RELIABILITY_REPORT.md)). The re-measured `bench 3 20` answered 20/20 three-hop runs ([`LATENCY_REPORT.md`](LATENCY_REPORT.md)). CI `TRON checks` passed for `993f8d1`.
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
