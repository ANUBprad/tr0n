# MVP_SCOPE.md

> **SUPERSEDED** by [`PRD.md`](PRD.md) (scope) and
> [`IMPLEMENTATIONPLAN.md`](IMPLEMENTATIONPLAN.md) (sequencing). Kept for
> history; not canonical. See [`README.md`](README.md) for the document set.

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION  
**Goal**: Smallest set that enables a strong Track 03 demo

## MUST HAVE (Critical)
- [ ] **Synthetic company graph in FalkorDB** (post-Oct 15) — connected enough for 5 queries
- [ ] **Graph tools (minimal)**: find_owner, find_experts, trace_decision, find_related_incidents (+ search_graph/trace_evidence as needed)
- [ ] **Agent with deliberate retrieval** (tool selection, no full-graph dump)
- [ ] **Q&A interface** (minimal UI)
- [ ] **Evidence panel** showing graph paths/provenance
- [ ] **5 demo queries working reliably**
- [ ] **Multi-hop reasoning demonstrated**
- [ ] **Graph-central flow** provably uses FalkorDB

## SHOULD HAVE
- [ ] **prepare_handoff** tool (minimal)
- [ ] **Clean path visualization in UI** (simple list of paths is fine)
- [ ] **Entity search/autocomplete**
- [ ] **Reproducible data load** (script)

## NICE TO HAVE (Cut first if time tight)
- [ ] Advanced graph algorithms (PageRank, centrality)
- [ ] Rich graph visualization (force-directed)
- [ ] MCP server
- [ ] Complex actions
- [ ] Multiple LLM providers

## CUT FIRST
- Anything not directly supporting the 5 demo queries
- UI polish/decorations
- Extra tracks' bells/whistles
- Over-engineered agent framework

## Scope Discipline
**working > broad | deep > shallow | graph reasoning > UI | evidence > flashy**

Aim for \
minimal
but
undeniable\ graph centrality.

