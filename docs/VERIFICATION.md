# Integration verification - 2026-10-16

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

Review the published walkthrough and code, and explain your own contribution accurately. The implementation is a deterministic local prototype; general LLM integration, live private connectors, production authorization, and full E1-E12 benchmark coverage are not claimed. See TRACKER.md and EVALUATION.md for remaining scope and known identity ambiguity.

## Publication integration

The destination fork also contained `a662006`, adding explicit over-budget evidence outcomes and a total fact order. That fix and all its tests were preserved before organizing the completion into 13 commits on top of the destination main branch. The original local checkpoints remain available on `codex/company-brain-completion`.
