# TRON — Documentation Index & Authority Map

**Status**: Canonical index. Entry point for the documentation set.
**Last updated**: 2026-10-12.

TRON is a Go + FalkorDB "company brain": a single source of truth that answers
who knows what, who owns what, why something was decided, and what to watch
out for — with evidence. Start with [`PRD.md`](PRD.md); follow the map below.

## Start here

| If you want to… | Read |
|---|---|
| Understand the product and users | [`PRD.md`](PRD.md) |
| Understand the target architecture | [`TECHSPEC.md`](TECHSPEC.md) |
| See what exists today | [`architecture-minimal.md`](architecture-minimal.md), [`TRACKER.md`](TRACKER.md) |
| Know the plan and sequence | [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md) |
| Know the rules and decisions | [`RULES.md`](RULES.md), `AGENTS.md` |
| Work on the repo | `AGENTS.md`, `opencode.md` |

## Canonical documents (authoritative)

| Document | Owns |
|---|---|
| [`PRD.md`](PRD.md) | Product intent, users, capability roadmap, acceptance |
| [`TECHSPEC.md`](TECHSPEC.md) | Target whole-system architecture |
| [`SCHEMA.md`](SCHEMA.md) | Target knowledge model |
| [`INGESTION.md`](INGESTION.md) | Ingestion & organizational memory |
| [`REASONING.md`](REASONING.md) | Evidence-grounded reasoning & decision engine |
| [`ACTIONS.md`](ACTIONS.md) | Autonomy model, authorization, action execution |
| [`APPFLOW.md`](APPFLOW.md) | Interfaces, commands, workflows, design |
| [`SECURITY.md`](SECURITY.md) | Security, privacy, reliability |
| [`EVALUATION.md`](EVALUATION.md) | Quality measurement, suites, baselines |
| [`RULES.md`](RULES.md) | Engineering rules, ADR register, risk register |
| [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md) | Dependency-aware roadmap |
| [`TRACKER.md`](TRACKER.md) | Live milestone/component status |
| [`CYPHER_QUERIES.md`](CYPHER_QUERIES.md) | FalkorDB traversals and graph-derived rules used by the product |
| [`DEMO_SCRIPT.md`](DEMO_SCRIPT.md) | Recording preparation and timed judge-facing demo narration |

## Locked current-state authorities

Describe the **implemented** system as of 2026-10-08; new docs reference them.

| Document | Owns |
|---|---|
| [`architecture-minimal.md`](architecture-minimal.md) | Current locked architecture, test posture |
| [`GRAPH_MODEL.md`](GRAPH_MODEL.md) | Locked v1 schema (11 entities / 16 relationships) |
| [`knowledge-model.md`](knowledge-model.md) | Locked knowledge model, provenance, temporal rules |
| [`AGENT_SPEC.md`](AGENT_SPEC.md) | Locked graph-operation boundary & agent model |
| [`technology-evaluation.md`](technology-evaluation.md) | Why the tech stack was chosen |

## Living references

| Document | Purpose |
|---|---|
| [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md) | Open questions, promoted to decisions when answered |
| `planning/decision-log.md` | Chronological decision history (D1–…) |
| [`falkordb-analysis.md`](falkordb-analysis.md) | FalkorDB capability analysis |
| [`HACKATHON.md`](HACKATHON.md) | Hackathon event context and rules |

## Historical / superseded (kept, not canonical)

These carry a `SUPERSEDED` banner or predate implementation. Preserved as a
record; do not treat as current.

| Document | Superseded by |
|---|---|
| [`PROJECT.md`](PROJECT.md) | [`PRD.md`](PRD.md) |
| [`PRODUCT_SPEC.md`](PRODUCT_SPEC.md) | [`PRD.md`](PRD.md) |
| [`product-definition.md`](product-definition.md) | [`PRD.md`](PRD.md) |
| [`MVP_SCOPE.md`](MVP_SCOPE.md) | [`PRD.md`](PRD.md), [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md) |
| [`SECURITY_AND_DATA.md`](SECURITY_AND_DATA.md) | [`SECURITY.md`](SECURITY.md) |
| [`EXECUTION_PLAN.md`](EXECUTION_PLAN.md) | [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md), [`TRACKER.md`](TRACKER.md) |
| [`ARCHITECTURE.md`](ARCHITECTURE.md) | [`architecture-minimal.md`](architecture-minimal.md) |
| `../planning/milestones.md` | [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md), [`TRACKER.md`](TRACKER.md) |
| `../planning/backlog.md` | [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md), [`TRACKER.md`](TRACKER.md) |
| [`DATA_SPEC.md`](DATA_SPEC.md), [`DEMO_SPEC.md`](DEMO_SPEC.md), [`JUDGING_STRATEGY.md`](JUDGING_STRATEGY.md), [`TEAM_PLAN.md`](TEAM_PLAN.md) | Pre-implementation design → the canonical set above |

## Conventions

- One fact, one home: documents reference each other, never duplicate.
- Every capability is labeled **IMPLEMENTED / PARTIAL / PROPOSED** with a code
  path where it exists.
- Source code beats prose: when they disagree, the code and its tests win and
  the document is fixed.
- Status legend lives in [`TRACKER.md`](TRACKER.md).

## Related repository docs

- Root `README.md` — build, run, and demo instructions.
- `AGENTS.md` / `opencode.md` — binding working rules.
