# system-architecture.md

**Status**: PRE-HACKATHON CONCEPTUAL DIAGRAM — NOT IMPLEMENTATION

```mermaid
graph TD
    U[User] --> UI[TRON UI - Minimal Q&A]
    UI --> API[FastAPI Backend]
    API --> AGENT[TRON Agent<br/>Deliberate Retrieval + Tool Selection]
    AGENT --> TOOLS[Graph Tools]
    TOOLS --> FB[FalkorDB - Primary Graph DB]
    FB --> TOOLS
    TOOLS --> AGENT
    AGENT --> API
    API --> UI
    UI --> E[Evidence Panel<br/>Paths + Provenance]
    
    classDef primary fill:#e1f5fe,stroke:#0288d1
    classDef graph fill:#f3e5f5,stroke:#7b1fa2
    classDef agent fill:#e8f5e9,stroke:#388e3c
    
    class FB graph
    class AGENT,TOOLS agent
    class API,UI primary
```

**Notes**: 
- FalkorDB is central (removing it materially affects reasoning)
- Tools mediate all graph access (no full dump to LLM)
- Evidence flows back through agent to UI

