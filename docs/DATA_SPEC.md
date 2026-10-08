# DATA_SPEC.md

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION  
**Constraint**: Public, synthetic, or allowed data only. NO private/restricted company data.

## Approach
Create a small, realistic, fully synthetic company graph that is densely connected enough to demonstrate multi-hop reasoning meaningfully.

## Fictional Organization
**Company Name**: Acme Org (fictional) — neutral, synthetic

## Target Scale (for 72h demo)

| Entity | Count (Target) | Notes |
|---|---|---|
| People | 50–80 | Mix of engineers, leads, PMs |
| Teams | 8–12 | Core/platform/product/data |
| Projects | 10–15 | Cross-cutting |
| Services | 20–40 | Microservices/components |
| Systems | 8–15 | Platforms |
| Documents | 60–120 | ADRs, runbooks, specs, notes |
| Decisions | 30–60 | With rationale, support |
| Meetings | 20–40 | Decision context |
| Tickets | 120–200 | Features/bugs |
| Incidents | 30–60 | With resolution history |
| PullRequests | 80–150 | Engineering activity |
| Repositories | 10–20 | Code repos |
| Technologies | 15–30 | Stack |
| Deployments | 20–40 | Context (minimal) |

## Connectivity Requirements
Must be sufficiently connected to support:
- Ownership chains (Person→OWNS→Service)
- Dependency graphs (Service→DEPENDS_ON→Service)
- Expertise signals (MODIFIED/RESOLVED_BY/WORKED_ON)
- Decision provenance (SUPPORTED_BY/DISCUSSED_IN)
- Incident→Service→People trails

## Data Generation Strategy (Design)

**PROPOSED**: A deterministic generator script to produce the synthetic graph (language open — Go in-repo or a short one-off script; decide at scaffold, see `OPEN_QUESTIONS.md`) with controlled relationships + realistic names.
- Deterministic seed (reproducible)
- Realistic but fictional names
- Ensure multi-hop paths exist for all 5 demo queries
- Export as Cypher or directly populate FalkorDB (post-Oct 15)

**UNDECIDED**: Format (JSON → transform → Cypher, or direct generation)

## Synthetic Data Principles
- No real PII, no scraped private data
- Plausible organizational structure
- Designed for demo queries (intentional paths)
- Small enough to load fast, large enough to be non-trivial

## Example Seed Ideas (Conceptual)
- \
Platform\ system with owners, incidents, deps
- Key decision (ADR) with supporting docs + discussions
- Recurring incident pattern → shows \happened
before\

No dataset generation in this foundation phase.

