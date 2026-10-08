# TRON — Product Definition (Concise)

**Status**: PRE-IMPLEMENTATION

## What TRON Is
TRON is an organizational intelligence system that reasons over relationships in a company graph to answer questions with evidence and provenance.

## Core Thesis
Graph + deliberate reasoning + provenance > flat RAG. Multi-hop relationship reasoning is central; FalkorDB is fundamental.

## Target Users (conceptual)
Engineers oncall, tech leads, new members needing context.

## Killer Workflows (MVP)
1. Who owns this system? (ownership)
2. Who knows this system best? (expertise via history)
3. Why was this decision made? (provenance)
4. Has this problem happened before? (incidents history)
5. Who should handle this issue and why? (grounded routing)

## Non-Goals (now)
Full enterprise authZ, complex UI, broad integrations, heavy automation.

## Design Constraints
- Evidence-backed, explainable
- Graph-central (remove FalkorDB breaks reasoning)
- Deterministic-first where possible
- Minimal scope, serious architecture
- **Knowledge invariant**: TRON never presents an inference or a recommendation
  as a fact; every claim carries its layer tag and evidence chain
  (rules in `knowledge-model.md`)

