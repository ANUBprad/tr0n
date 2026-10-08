# AGENT_SPEC.md

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION  
**Principle**: Deliberate graph retrieval. Tools over dumping graph. Evidence/provenance required.

## Agent Role (Conceptual)
TRON is a reasoning agent that answers organizational questions by selecting and executing graph traversals against FalkorDB, returning answers with evidence paths.

## Core Principle
> The LLM should NOT simply receive the entire graph. Retrieval must be deliberate via graph tools.

## Conceptual Tools

| Tool | Purpose | Input | Expected Output | Graph Relationships | Why LLM alone insufficient |
|---|---|---|---|---|---|
| **search_graph** | Find entities by name/type (fuzzy/conceptual) | query, entity_types[] | matching nodes + context | any | Large graph; need precise matches + neighborhood |
| **find_owner** | Find explicit owners with evidence | entity_name, entity_type | owner(s) + OWNS paths | OWNS | Requires traversal, not general knowledge |
| **find_experts** | Find who knows system/service best | target, signals[] | ranked people + evidence paths | WORKED_ON, MODIFIED, RESOLVED_BY, AUTHORED, OWNS | Expertise inferred from history; multi-hop |
| **trace_decision** | Why decision made + support | decision_id/name | decision node + SUPPORTED_BY + DISCUSSED_IN paths + docs/meetings | SUPPORTED_BY, DISCUSSED_IN, AUTHORED, RELATED_TO | Needs provenance chain |
| **find_related_incidents** | Find similar/prior incidents | service/system, keywords | incidents + AFFECTS + RESOLVED_BY + people | AFFECTS, RESOLVED_BY, CAUSED, RELATED_TO | Temporal/relational matching |
| **trace_evidence** | Expand path for claim | source_node, claim | full evidence subgraph + paths | any (guided) | Ensures answer is grounded |
| **prepare_handoff** | Collect minimal context for handoff | target (issue/system) | curated nodes/edges + evidence bundle | relevant set | Synthesize actionable package |

## Reasoning Flow (Conceptual)
1. **Understand**: Parse intent ("who owns X", "why decision D", "who knows best")
2. **Plan**: Choose minimal tools needed
3. **Retrieve**: Call tools to get subgraph + paths
4. **Evaluate**: Is evidence sufficient? If not, iterate
5. **Synthesize**: Answer with evidence paths + reasoning
6. **Verify**: Every claim backed by path?
7. **Respond**: Return structured answer

## Answer Structure (Target)
- **Answer**: Direct answer to question
- **Evidence**: List of graph paths (node IDs, rel types, direction)
- **Reasoning**: Brief why (tool choices + traversal)
- **Confidence/Support**: Which paths support
- **Next Action** (optional): If applicable

## Design Notes
- Tools return minimal subgraphs (not everything)
- Keep tool count small for 72h
- Prioritize: find_owner, find_experts, trace_decision, find_related_incidents (cover 5 demo queries)
- No tool implements full answer - agent synthesizes
- Provenance is mandatory

No implementation planned pre-Oct 15.

