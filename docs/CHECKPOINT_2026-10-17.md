# TRON — team handoff checkpoint

**Date:** 17 October 2026  
**Competition track:** Track 03 — Company Brain  
**Published code checkpoint:** `d7e81f5` on `Shashwat-911/tr0n-shashwat/main`  
**Status:** Working local competition prototype; latest GitHub CI passed. The full production roadmap is still incomplete.

Repository: https://github.com/Shashwat-911/tr0n-shashwat  
Verified run: https://github.com/Shashwat-911/tr0n-shashwat/actions/runs/39019480907

## 1. What we have now

TRON connects a fictional company's people, teams, services, repositories, PRs, documents, chats, tickets, incidents, and decisions in FalkorDB. It answers organizational questions with inspectable sources and relationship paths.

The stack is **Go + FalkorDB + an embedded browser UI**, with optional **local Ollama (`qwen3:4b-instruct`)**. FalkorDB stores the knowledge and performs the traversals; removing it removes the facts needed for the answers. The synthetic seed has 339 nodes and 958 relationships.

There are two answer paths:

- **Default deterministic mode:** five workflows for ownership, expertise, decision rationale, incident history, and handoff drafts. No model or API key required.
- **Optional local model mode:** the model chooses bounded graph retrievals and selects evidence IDs. The server validates citations and renders the recorded relationships. This supports more flexible retrieval, but is not unrestricted factual prose generation.

## 2. Work completed in my completion/debugging sessions

This is a summary of Shashwat's completion work with Codex assistance, built on the existing team project. It includes integrated upstream work; it does not claim that every underlying feature was authored solely by Shashwat. Original team history is preserved in Git.

| Area | Work completed | Current boundary |
|---|---|---|
| Team integration | Synced and merged upstream changes; retained evidence contracts, identity checks, and existing history | This checkpoint does not certify that no newer upstream commits exist |
| Graph correctness | Current-time ownership, historical ownership windows, conflict detection, provenance checks, sourced ownership changes | Historical expertise is not a full historical snapshot of all activity |
| Core question flows | Five question intents, context selection, ambiguity handling, unsupported-question abstention, handoff composition | Handoff drafts are downloaded for review; they are not sent or assigned |
| Browser UI | Question workspace, source inspection, connected graph context, responsive layout, handoff download | Earlier browser checks are documented; CI is not a complete browser test |
| Source ingestion | Structured JSON imports with identity/freshness validation, idempotent retries, and link updates | No automatic extraction of arbitrary prose into facts |
| Source synchronization | Explicit Markdown manifests, repeatable updates, cursor handling, source-owned deletion, reviewed identity mappings | Snapshot synchronization, not a live Slack/ticketing connector |
| Identity and access | Reviewed aliases, per-label key uniqueness, cross-label application checks, credential-selected tenant graphs | One writer per graph; no per-document ACLs or managed identity |
| Local model | Ollama adapter, bounded connected retrieval, citation validation, visible abstention/truncation, conflict safeguards | Model selection of valid evidence can still be irrelevant to a question |
| Evaluation | Ten-case synthetic model corpus, token/total-latency reporting, answer samples and participant review | No broad accuracy claim or comparative search/RAG result |
| Planning prototype | Validated caller-scored options, Pareto ranking, weight sensitivity | Option scores are not derived from graph evidence |
| Action prototype | Authenticated roles for local drafts, parameter checks, in-memory replay and audit | No durable action store or external execution; state-changing actions denied |
| Alert prototype | Snapshot rules for ownership conflicts, missing expertise, and single-expert risks | No labeled precision/recall evaluation |
| Packaging | Docker setup, pinned database image, persistence, local port bindings, repeatable demo | Complete local setup; no hosted application deployment |
| Submission materials | Judge-oriented README, graph/query docs, AI disclosure, demo script, captioned walkthrough | Walkthrough predates the optional model and newer sync features |

The earlier completion work was organized into 13 reviewable commits; see [COMPLETION_STEPS.md](COMPLETION_STEPS.md). Those groupings do not imply 13 separate development sessions.

## 3. What is actually verified

**GitHub CI passed at `d7e81f5`:** `go vet ./...`, the full race-enabled test suite against FalkorDB, synthetic seed loading, and the seven-check demo. This is the strongest current clean-environment verification for the published code.

Earlier local checks also covered the Docker build, source import retries, database persistence after restart, browser question flows, source drill-down, and handoff downloads. See [VERIFICATION.md](VERIFICATION.md) for their scope and history.

**Model evaluation:** the run at `2026-10-17T04:42:00Z` passed the automated rubric in 10/10 cases. Participant review marked all ten samples correct and relevant, completing the semantic sample-review gate for that run. Cases include current/historical ownership, decision sources, three-hop authorship, unsupported questions, ambiguity, missing entities, and one prompt-injection attempt.

That result covers a small synthetic corpus (`acme-org/synthetic/v1`). It does not establish production readiness, broad semantic accuracy, general injection resistance, or superiority over keyword search/RAG. The broader M3 milestone still has evaluation work remaining. See [MODEL_REVIEW.md](MODEL_REVIEW.md) and [EVALUATION.md](EVALUATION.md).

## 4. The CI failure we just fixed

Runs #7–#9 failed in `internal/rules/TestDetectAlerts` with:

```text
DetectAlerts failed: detect ownership conflicts: find conflicts:
ERR Invalid graph operation on empty key
```

The test opened the default `tron` graph without loading data. My local database already contained that graph, which hid the dependency. GitHub starts with a fresh FalkorDB, and the workflow's demo seed step runs after the tests.

**Fix (`d7e81f5`):** the alert test now uses its own `tron_rules_test` graph, loads the existing seed fixture, cleans up afterwards, and fails explicitly if setup fails. The corrected test passed locally and the full GitHub workflow passed.

Earlier CI changes upgraded the actions, pinned Ubuntu 24.04, enabled verbose test output, and set an explicit database address. Those changes did not fix the missing test data; the isolated seeded fixture did.

## 5. Local work that is NOT in the published checkpoint

At the time of writing, four files have uncommitted latency-instrumentation changes:

- `internal/agent/grounded.go`: adds `graph_ms`, `model_ms`, and `turns` to grounded reports.
- `internal/evaluation/model.go`: copies those fields into evaluation results.
- `cmd/tron/main.go`: prints those stage values during model evaluation.
- `internal/agent/grounded_test.go`: adds a basic metrics check.

These edits are outside `d7e81f5` and its green CI result. A teammate cloning `main` will not receive them. Review and verify the final diff before committing it.

The previous model run measured **19,227 ms total** for the three-hop case. The suggested split of 13.6 seconds graph time and 4.5 seconds generation time was not measured. No sub-three-second optimization has been established. Current pending instrumentation also does not separately time citation validation.

## 6. What remains — competition priorities

These are recommended next tasks, not claims that the competition requires every production-roadmap feature.

| Priority | Task | Completion evidence |
|---|---|---|
| P0 | Teammate runs the published revision from a fresh clone using the README | App opens, demo passes 7/7, sources can be inspected, handoff downloads |
| P0 | Review/record the final demo | Shows the real app, FalkorDB-backed multi-hop answers, sources, history, and source updates; include Ollama if claiming it in the presentation |
| P0 | Confirm final submission contents | Public repo/video links work, Track 03 is selected, setup/model/query docs are linked, AI assistance is disclosed |
| P0 | Participant technical walkthrough | Both teammates can explain graph directions, provenance, ownership windows, expertise scoring, model bounds, and their actual contributions |
| P1 | Finish the local timing changes | Reviewed commit, appropriate tests, fresh CI pass, measurements from the final implementation |
| P1 | Measure the slow three-hop question repeatedly | Record hardware/model, cold versus warm runs, stage times and sample count; report observed distributions |
| P1 | Optimize only the measured bottleneck | Reuse existing bounded graph queries where appropriate; preserve evidence IDs, direction, provenance, and answer quality |
| P1 | Broaden evaluation | More ambiguous entities, noisy permitted source content, conflicts and adversarial cases; human review of the resulting answers |

The existing video is a captioned sequence of captured app states. It is useful, but it does not show the newer Ollama flow. A continuous narrated recording would make the current product easier to judge.

## 7. Longer-term roadmap still open

- Real comparative keyword-search, text-RAG, and no-graph-model baselines.
- More robust multi-source identity resolution and permission propagation.
- Per-document access control, managed authentication, production isolation and operational recovery.
- Graph-derived planning inputs and human evaluation of recommendations.
- Durable action records, a real execution adapter, and verified outcomes before enabling state-changing actions.
- Alert quality measurement, deduplication/delivery policy, and ongoing monitoring.
- Live source connectors, if later needed and compatible with the data-use rules.

M1–M7 remain partial in the broader roadmap. Passing CI and the ten reviewed answers do not complete those milestones. See [TRACKER.md](TRACKER.md) and [IMPLEMENTATIONPLAN.md](IMPLEMENTATIONPLAN.md).

## 8. How my teammate can pick up the project

For a fresh checkout:

```sh
git clone https://github.com/Shashwat-911/tr0n-shashwat.git
cd tr0n-shashwat
docker compose up -d --wait falkordb
docker compose build app
docker compose run --rm app seed
docker compose up -d app
docker compose run --rm app demo
```

Open http://127.0.0.1:8080. The `seed` command replaces the default synthetic demo graph; use a fresh/disposable demo database for this walkthrough.

For optional local Ollama:

```sh
docker compose --profile model up -d ollama
docker compose exec ollama ollama pull qwen3:4b-instruct
docker compose up -d --build app
```

Choose Local model in the UI. Hardware and model download affect startup and response time. See [LOCAL_MODEL_AND_SYNC.md](LOCAL_MODEL_AND_SYNC.md) for the full configuration and synchronization commands.

Suggested division of work: Shashwat finishes timing instrumentation and measured performance investigation; the teammate independently checks setup, reviews the demo, and prepares the submission presentation. Both review the code and final claims together. This division is a proposal, not an assignment already agreed by the team.

**AI disclosure:** Codex assisted with implementation, debugging, tests, UI, and documentation. Participants remain responsible for reviewing the submitted code, understanding it, and accurately describing their contributions.
