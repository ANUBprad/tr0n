# ARCHITECTURE.md

> **SUPERSEDED (2026-10-08)** — this document is the hackathon-era
> architecture sketch. Its stack table (FastAPI/Python) is **out of date**:
> the locked architecture is Go + FalkorDB in
> [`architecture-minimal.md`](architecture-minimal.md), with the stack
> rationale in [`technology-evaluation.md`](technology-evaluation.md).
> Kept for historical context only.

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION  
**All choices marked PROPOSED / UNDECIDED**

## Guiding Principle
Graph is central. Reasoning must use deliberate graph retrieval via tools; LLM must not receive entire graph dump. Remove FalkorDB → core reasoning materially affected.

## Conceptual Architecture

```
User (Natural Language Query)
        ↓
TRON Agent (Reasoning + Tool Selection)
        ↓
Intent Understanding / Plan
        ↓
Graph Tools (search_graph, find_owner, find_experts, trace_decision, find_related_incidents, trace_evidence, prepare_handoff)
        ↓
FalkorDB (Graph DB - Primary)
        ↓
Graph Results (Subgraph + Paths + Provenance)
        ↓
Reasoning / Synthesis
        ↓
Answer + Evidence Paths + Recommendation/Action
        ↓
UI (Minimal, demo-focused)
```

## Components (Conceptual)

| Component | Choice | Notes |
|---|---|---|
| **Graph DB** | **PROPOSED: FalkorDB** | Primary. Graph must do real work. |
| **Backend/API** | **PROPOSED: FastAPI (Python)** | Simple, async-friendly; easy FalkorDB client integration. UNDECIDED vs others. |
| **Agent Framework** | **UNDECIDED** | Could use custom minimal agent + tool calling, or lightweight framework. Avoid heavy overkill. |
| **LLM Provider** | **UNDECIDED** | Need to pick provider (consider rate/availability). Keep interface abstractable. |
| **MCP** | **UNDECIDED** | Evaluate MCP vs direct function tools. Direct may be simpler for 72h. |
| **FalkorDB Client** | **PROPOSED: falkordb (Python)** / **redisgraph-py** style | Use official/standard client. |
| **Frontend** | **UNDECIDED** | Minimal: streamlit / gradio / vite+react / plain html+js. Prioritize speed for demo. |
| **Deployment** | **UNDECIDED** | Local-first required (complete local setup acceptable). Cloud optional. |
| **Data Generation** | **UNDECIDED** | Synthetic generator script (Python) to create connected company graph. |

## Agent Loop (Conceptual)
1. Parse user query
2. Select relevant graph tools (deliberate)
3. Retrieve minimal subgraph/traversals
4. Inspect evidence paths
5. Iterate if insufficient (tool use)
6. Synthesize answer with provenance
7. Return grounded response

## Design Constraints
- Don't pass full graph to LLM
- Every answer traceable to paths
- FalkorDB central to flow
- Minimal UI (demo > decoration)
- Local dev first
- Keep tech simple for 2–3 person team

## Technology Rationale (Draft)
- **Python/FastAPI**: fast to iterate, FalkorDB Python client available
- **FalkorDB**: graph-native, traversal-optimized; matches hackathon requirement
- **Keep scope tight**: avoid microservices; monolith likely fine for 72h

All above are PROPOSED/UNDECIDED — no implementation planned pre-Oct 15.

