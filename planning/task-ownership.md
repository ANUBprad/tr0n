# task-ownership.md

**Status**: PRE-HACKATHON — Template (assign post-Oct 15)

## Ownership Matrix (TBD)

| Area | 2-Person (A/B) | 3-Person (A/B/C) | Notes |
|---|---|---|---|
| **Graph Schema** | A | A | FalkorDB schema, relationships |
| **Synthetic Data** | A (or shared) | A | Generator, connectivity validation |
| **Graph Tools** | A | B | Tool implementations, contracts |
| **Agent Loop** | A | B | Deliberate retrieval, reasoning |
| **API (Go)** | A | B | Endpoints, I/O |
| **Frontend/UI** | B | C | Q&A + evidence panel |
| **Integration** | Shared | Shared | Agent↔API↔UI |
| **Demo/Video** | B | C | Script, recording |
| **Data Validation** | Shared | A+C | 5 queries paths exist |
| **Submission Artifacts** | Shared | Shared | README, model, queries, video, setup |

## Interface Contracts (Define at Hour 0)
- **Evidence Format**: { paths: [{nodes[], rels[], explanation}], supporting_facts[] }
- **Tool I/O**: Standardize inputs/outputs
- **API Response**: answer + evidence + reasoning

Assign actual names when team forms.

