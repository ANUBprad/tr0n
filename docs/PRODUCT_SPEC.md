# PRODUCT_SPEC.md

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION  
**Optimization Goal**: 72-hour hackathon (working > broad, deep > shallow)

## Core User Problem
"I need to understand organizational context quickly and get answers I can trust with evidence. Who owns this? Who knows this best? Why was this decision made? Has this happened before?"

## User Personas (Conceptual)
- **Oncall Engineer**: Needs incident context, owners, prior resolution
- **New Team Member**: Needs to understand systems, owners, docs, history
- **Tech Lead**: Needs to trace decisions, assess impact, find expertise
- **Product/PM**: Needs context on projects, decisions, dependencies

## Core Workflows (Conceptual)
1. Ask natural language question about organization
2. TRON reasons about needed graph traversal
3. Retrieves relevant subgraph via tools
4. Multi-hop reasoning with evidence paths
5. Returns answer + evidence + (optional) next action

## Primary Use Cases (Track 03 focused)
- **Ownership Discovery**: Find system/service/project owner with evidence
- **Expertise Location**: Find who knows a system best (inferred from history)
- **Decision Traceability**: Why was this decision made? What docs/discussions support it?
- **Incident History**: Has this happened before? Who handled it? How resolved?
- **Investigation Routing**: Who should investigate based on relationships/history
- **Handoff Preparation**: Package relevant context with evidence paths

## 5 Killer Demo Queries
1. **"Who owns [System X]?"** → Answer + ownership chain + evidence
2. **"Who knows [System X] best?"** → Ranked experts + reasoning (authorship, incidents, PRs, modifications) + evidence
3. **"Why was [Decision D] made?"** → Decision trace + supporting docs/meetings/discussions + evidence
4. **"Has this happened before?"** → Related incidents + resolution + people involved + evidence paths
5. **"Who should handle [current issue] and why?"** → Recommendation with justification via graph relationships + evidence

## Evidence Requirements
Every answer must be grounded in graph paths (provenance). Show: nodes/relationships traversed, why chosen, supporting facts. LLM must not hallucinate disconnected answers.

## Potential Actions (Conceptual, minimal)
Only if they naturally fit 72h: prepare handoff summary with collected evidence. Avoid complex automation.

## Design Principles
- Graph-central: FalkorDB does the connective work
- Evidence-first: answer + "why" + paths
- Deliberate retrieval: don't dump full graph to LLM
- Minimal scope: focus on strong demo for Track 03

