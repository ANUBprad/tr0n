# DECISION_LOG.md

**Status**: PRE-HACKATHON — Template  
**Purpose**: Record all future architecture decisions with rationale

## Format
Record each decision as:
- **Date**: YYYY-MM-DD
- **Decision**: What was decided
- **Why**: Rationale
- **Alternatives Considered**: Other options
- **Tradeoffs**: Pros/cons
- **Owner**: Who decided
- **Status**: PROPOSED/DECIDED/REVISED

## Initial Decisions (Foundation)

| Date | Decision | Why | Alternatives | Tradeoffs | Owner | Status |
|---|---|---|---|---|---|---|
| 2026-10-08 | **TRON naming & scope: Track 03 focus** | Best fit for graph-central "Company Brain" with provenance | Broader scope across all tracks | Focus vs flexibility | Team | PROPOSED |
| 2026-10-08 | **FalkorDB as primary graph DB** | Required by hackathon; graph must be central | Other graph DBs | Compliance + fit; constraint is fixed | Team | PROPOSED |
| 2026-10-08 | **Deliberate graph retrieval (no full graph dump)** | Core differentiator; evidence-driven | Dump graph to context | Accuracy/provenance vs simplicity | Team | PROPOSED |
| 2026-10-08 | **Synthetic-only data** | Compliance with data restrictions | Real org data (impossible safely) | Safe but needs generation | Team | PROPOSED |
| 2026-10-08 | **Evidence/provenance mandatory** | Explainability; Track 03 alignment | Answer-only | Trustworthy but more work | Team | PROPOSED |
| 2026-10-08 | **72h-optimized minimal scope** | Feasible for 2–3 people | Feature-rich | Reliable demo vs breadth | Team | PROPOSED |
| 2026-10-08 | **Python/FastAPI + FalkorDB client (PROPOSED)** | Fast iteration, client available | Other stacks | TBD; confirm at start | Team | PROPOSED |
| 2026-10-08 | **Pre-hackathon = planning only** | Compliance with hackathon rules | Start early | Safe, avoids rule issues | Team | PROPOSED |

## Notes
New decisions must be added here. Mark as DECIDED only after team consensus during/at hackathon start.

