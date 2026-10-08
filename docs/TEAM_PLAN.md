# TEAM_PLAN.md

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION

## 2-Person Team

| Role | Responsibilities | Focus | Interface |
|---|---|---|---|
| **A — Graph + Backend + Agent** | Schema, synthetic data design/load, FalkorDB client, graph tools, agent loop, deliberate retrieval, evidence extraction | Graph centrality, multi-hop, provenance | Define tool contracts (I/O), share evidence format |
| **B — Frontend + Integration + Deployment** | Minimal UI (Q&A + evidence panel), API integration, demo polish, deployment/local setup docs | UX for demo, clear evidence display, reliability | Consume agent/API responses, display paths |

## 3-Person Team

| Role | Responsibilities | Focus | Interface |
|---|---|---|---|
| **A — Graph + Data** | Schema, synthetic data generation, FalkorDB schema/indexing ideas, Cypher design examples | Dense connected graph for 5 queries | Provide test data + example traversals |
| **B — Agent + Backend** | Tools, agent loop, tool selection, reasoning, API (Go) | Deliberate retrieval, evidence/provenance | Tool contracts, API schema |
| **C — Frontend + Demo + Eval** | UI, evidence panel, demo script, query validation, video planning | Demo effectiveness, judge clarity | UI integration, demo dry-runs |

## Collaboration Principles
- **Minimize merge conflicts**: Clear boundaries (graph vs agent vs UI)
- **Interface-first**: Define tool I/O early (post-Oct 15 start)
- **Shared contracts**: Evidence format (paths, nodes, rels) agreed by all
- **Demo-driven**: All work traces to 5 killer queries
- **Pair on critical path**: Graph tools + agent integration
- **Avoid parallel overreach**: Focus on MUST-HAVES

## Time Allocation Guidance
- Graph setup + data: 15–20%
- Agent + tools: 40–50%
- UI + demo: 20–25%
- Buffer: 10–15%

Assignments TBD by actual team.

