# agent-flow.md

**Status**: PRE-HACKATHON CONCEPTUAL DIAGRAM — NOT IMPLEMENTATION

```mermaid
flowchart TD
    Q[User Query - Natural Language] --> Parse[Parse Intent]
    Parse --> Plan[Plan: Select Minimal Tools]
    Plan --> Tool1[search_graph / find_owner / find_experts / trace_decision / find_related_incidents]
    Tool1 --> FB[FalkorDB Traversal]
    FB --> Sub[Subgraph + Paths + Provenance]
    Sub --> Eval{Evidence Sufficient?}
    Eval -->|No| Plan
    Eval -->|Yes| Synthesize[Synthesize Answer with Evidence]
    Synthesize --> Verify{All Claims Backed by Paths?}
    Verify -->|No| Plan
    Verify -->|Yes| Respond[Return Answer + Evidence Paths + Reasoning]
    Respond --> U[User]
    
    classDef decision fill:#fff9c4,stroke:#fbc02d
    classDef action fill:#e8f5e9,stroke:#388e3c
    classDef db fill:#f3e5f5,stroke:#7b1fa2
    
    class Eval,Verify decision
    class Parse,Plan,Tool1,Synthesize,Respond action
    class FB db
```

**Notes**: Deliberate retrieval loop. Evidence-gated (must be sufficient + all claims backed by paths). No full graph dump.

