# graph-topology.md

**Status**: PRE-HACKATHON CONCEPTUAL DIAGRAM — NOT IMPLEMENTATION

```mermaid
graph TD
    P[Person] -->|WORKS_IN| T[Team]
    P -->|OWNS| Svc[Service/System]
    P -->|WORKED_ON| Proj[Project]
    P -->|AUTHORED| Doc[Document]
    P -->|AUTHORED| Dec[Decision]
    P -->|RESOLVED_BY| Inc[Incident]
    P -->|REVIEWED/MODIFIED| PR[PullRequest]
    
    Proj -->|IMPLEMENTS| Svc
    Svc -->|DEPENDS_ON| Svc
    Inc -->|AFFECTS| Svc
    Dec -->|SUPPORTED_BY| Doc
    Dec -->|DISCUSSED_IN| Meet[Meeting]
    Dec -->|SUPERSEDES| Dec
    Doc -->|RELATED_TO| Svc
    PR -->|MODIFIED| Svc
    PR -->|IMPLEMENTS| Svc
    
    classDef person fill:#fff3e0,stroke:#f57c00
    classDef core fill:#e3f2fd,stroke:#1976d2
    classDef activity fill:#f1f8e9,stroke:#689f38
    classDef evidence fill:#fce4ec,stroke:#c2185b
    
    class P person
    class T,Proj,Svc,Sys core
    class Doc,Dec,Meet,Chat,Ticket,Inc,PR,Repo,Tech,Dep activity
    class Svc,Doc,Dec evidence
```

**Notes**: 
- High-value paths cross entity types
- Provenance: Decision←SUPPORTED_BY→Document, Incident→AFFECTS→Service←OWNS→Person
- Expertise inferred via activity edges

