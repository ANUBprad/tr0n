# GRAPH_MODEL.md

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION  
**Purpose**: Conceptual schema for Company Brain. Cypher examples are DESIGN EXAMPLES ONLY.

## Core Entities (Candidate)

| Entity | Purpose |
|---|---|
| Person | Individual contributors, owners, experts |
| Team | Organizational unit |
| Project | Workstream/initiative |
| Service | Runtime service/component |
| System | Broader system/platform |
| Document | Specs, ADRs, runbooks, notes |
| Decision | Architectural/organizational decisions (with rationale) |
| Meeting | Syncs/decisions context |
| Chat | Discussion context (synthetic only) |
| Ticket | Work items (issues/bugs/features) |
| Incident | Outages/incidents |
| PullRequest | Engineering activity |
| Repository | Code repos |
| Technology | Tech stack |
| Deployment | Releases/environment context |

## Core Relationships (Candidate)

| Relationship | From→To | Meaning |
|---|---|---|
| WORKS_IN | Person→Team | Membership |
| OWNS | Person→Service/System/Project/Repo | Ownership |
| WORKED_ON | Person→Project/Service | Contribution history |
| KNOWS | Person→Service/System/Tech (inferred) | Expertise signal |
| AUTHORED | Person→Document/PR/Decision | Authorship |
| REVIEWED | Person→PR | Review activity |
| MODIFIED | Person→Service/System (via PR) | Code changes affecting components |
| AFFECTS | Incident/Ticket→Service/System | Impact |
| DEPENDS_ON | Service/System→Service/System | Dependencies |
| DISCUSSED_IN | Decision/Issue→Meeting/Chat/Document | Context |
| SUPPORTED_BY | Decision→Document/Evidence | Rationale support |
| CAUSED | Incident→(root cause context) | Causal linkage (conceptual) |
| RESOLVED_BY | Incident→Person/PR/Change | Resolution |
| RELATED_TO | Entity↔Entity | Weak association |
| IMPLEMENTS | PR/Project→Service/System | Implementation |
| SUPERSEDES | Decision/Doc→Decision/Doc | Evolution |

## Design Rationale

- **Multi-hop reasoning**: e.g. "Who knows System X best?" → Person MODIFIED/WORKED_ON/AUTHORED related to Service→depends on System X, or resolved incidents affecting System X.
- **Provenance**: Every answer needs traversable paths (e.g. Decision ←SUPPORTED_BY→ Document, Decision ←DISCUSSED_IN→ Meeting). Paths are first-class evidence.
- **Expertise inference**: Prefer derived signals (incidents handled, PRs modifying, ownership, authored docs) over asserted tags.
- **Ownership**: Explicit OWNS + history; avoid guessing without evidence path.
- **Temporal**: Design should allow time-aware queries (when decision made, incident time, PR merged) without overengineering for 72h.

## Design Examples (PRE-HACKATHON — NOT IMPLEMENTATION)

**Example 1: Find owner of a system**
```cypher
// DESIGN EXAMPLE ONLY - NOT EXECUTED/IMPLEMENTED
MATCH path = (p:Person)-[:OWNS]->(s:System {name: $name})
RETURN p, path
```

**Example 2: Who knows system best (conceptual scoring via paths)**
```cypher
// DESIGN EXAMPLE ONLY - conceptual multi-hop
MATCH (sys:System {name:})
OPTIONAL MATCH (p:Person)-[:OWNS]->(sys)
OPTIONAL MATCH (p2:Person)-[:RESOLVED_BY|WORKED_ON|MODIFIED*1..2]->(sys)
OPTIONAL MATCH (p3:Person)-[:AUTHORED]->(d:Document)-[:RELATED_TO]->(sys)
RETURN p,p2,p3, count(*) as evidence_count
```

**Note**: Exact scoring/queries TBD during implementation (post-Oct 15). Examples illustrate graph centrality.

## Schema Principles
- Keep minimal for strong demo
- Favor explicit relationships over overloading
- Evidence paths must be traceable
- Graph is source of truth for connections

