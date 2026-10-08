# DEMO_SPEC.md

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION  
**Target**: ~3 minutes | Must prove graph does the work

## Demo Narrative

```
Problem: Need organizational context with evidence
↓
Show Company Graph (conceptual view) — emphasize connections
↓
Ask question in natural language
↓
TRON shows reasoning: tool selection + planned traversal
↓
FalkorDB traversal executes (show paths)
↓
Evidence paths surfaced (provenance)
↓
Answer + justification + evidence
↓
Next action (minimal if any)
```

## 5 High-Impact Scenarios

### 1. "Who owns this system?"
- **Setup**: Pick a key system (e.g. "Platform API")
- **Action**: Query ownership
- **Expected**: Owner + OWNS path + evidence
- **Graph Work**: Direct traversal
- **Demo Point**: Explicit ownership traceable

### 2. "Who knows this system best?"
- **Setup**: Same system with history
- **Action**: Find experts
- **Expected**: Ranked experts with signals (resolved incidents, PRs, modifications, ownership)
- **Graph Work**: Multi-hop (Person→history→Service/System)
- **Demo Point**: Inferred from activity, not tags

### 3. "Why was this decision made?"
- **Setup**: Key ADR/decision
- **Action**: Trace decision
- **Expected**: Decision + SUPPORTED_BY docs + DISCUSSED_IN meetings + author
- **Graph Work**: Provenance paths
- **Demo Point**: Evidence-first (show full path)

### 4. "Has this happened before?"
- **Setup**: Incident on service
- **Action**: Find related incidents
- **Expected**: Prior incidents + AFFECTS same service/system + resolution + who handled
- **Graph Work**: Pattern matching via relationships
- **Demo Point**: Historical context via graph

### 5. "Who should handle this and why?"
- **Setup**: New issue on service
- **Action**: Route investigation
- **Expected**: Recommendation + justification (owners, recent resolvers, experts) + evidence paths
- **Graph Work**: Multi-hop reasoning combining signals
- **Demo Point**: Actionable grounded recommendation

## Demo Flow (~3 min)
1. **Hook (0–30s)**: "Company Brain — answer questions with evidence from connected org data"
2. **Graph Centrality (30–60s)**: Show graph topology/connections
3. **Scenario 1–2 (60–120s)**: Ownership + expertise
4. **Scenario 3 (120–150s)**: Decision traceability (strong provenance)
5. **Scenario 4–5 (150–180s)**: History + routing + evidence

## Must Prove
- FalkorDB traversal produces the answer (not just LLM)
- Evidence paths visible
- Multi-hop, not single lookup
- Remove FalkorDB would break this

## Demo Artifacts (Post-Oct 15)
- Demo video (required)
- Clean UI for Q&A with evidence panel
- Prepared queries for reliability

