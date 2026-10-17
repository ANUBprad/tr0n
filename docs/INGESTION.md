> **Current competition build (2026-10-16):** See `TRACKER.md` and the root README for implemented behavior. The build now includes bounded natural-language routing, inspectable source content, handoff drafts, structured file imports, and sourced ownership updates. General-purpose LLM reasoning, live connectors, and autonomous execution remain future work.

# TRON — Ingestion, Source Integration & Organizational Memory

**Status**: Canonical. Authority for how raw company information becomes
trustworthy structured knowledge, and how it is compressed without losing
evidence.
**Last updated**: 2026-10-15.
**Current-state authority**: [`architecture-minimal.md`](architecture-minimal.md)
— seed status in [`TRACKER.md`](TRACKER.md). `DATA_SPEC.md` is superseded
([`README.md`](README.md) index) and is not an authority.
**Related**: [`SCHEMA.md`](SCHEMA.md), [`TECHSPEC.md`](TECHSPEC.md),
[`REASONING.md`](REASONING.md), [`SECURITY.md`](SECURITY.md),
[`EVALUATION.md`](EVALUATION.md).

> **Current reality (2026-10-17):** deterministic seed, structured JSON record
> imports, sourced ownership updates, and explicit complete Markdown snapshots
> are implemented (`internal/graph`, `internal/ingest`, `cmd/tron`). Snapshot
> sync has retry/cursor/deletion handling. There are no live service connectors,
> arbitrary-text extraction, per-document ACL propagation, or versioned body
> archive. The target pipeline below remains a design where not explicitly
> marked implemented. See [`LOCAL_MODEL_AND_SYNC.md`](LOCAL_MODEL_AND_SYNC.md).

---

## 1. Goals

1. Turn heterogeneous company artifacts into graph **facts** with provenance,
   without losing the original evidence.
2. Stay incremental, idempotent, and recoverable under partial failure.
3. Never let one bad source corrupt or block another.
4. Propagate permissions, retention, and deletions from source to derived data.
5. Keep extraction **deterministic first**; only non-deterministic extraction
   may carry `confidence` (`knowledge-model.md` invariant 5).

## 2. Sources and ingestion modes

| Mode | Trigger | Use case | Status |
|---|---|---|---|
| **File / structured import** | CLI or API call | CSV/JSON/patch dumps, exports | PROPOSED |
| **API connector** | Scheduled poll | GitHub, tracker, calendar, HRIS | PROPOSED |
| **Event / webhook** | Push from source | real-time PR/issue/incident events | PROPOSED |
| **Manual seed** | `tron seed` | demo + tests | IMPLEMENTED (destructive) |

All three real modes converge on the same pipeline (§4) so parsing,
resolution, provenance, and failure handling are written **once**.

## 3. Minimal first integrations (justification)

Ranked by product value per engineering cost. **Pick one** for the first
milestone (`IMPLEMENTATIONPLAN.md` M3), not all.

| Source | Value | Engine cost | Notes |
|---|---|---|---|
| **Markdown docs repo (git)** | Highest: decisions, ADRs, specs, ownership notes; stable schema; easy identity (`path@commit`) | Low: parse front-matter + links | Best first source; exercises Document/Decision/`AUTHORED`/`SUPPORTED_BY` |
| **Issue/tracker export (CSV/JSON)** | High: incidents, work history, resolvers | Low: column mapping | Exercises Incident/`AFFECTS`/`RESOLVED_BY` |
| **Calendar export (ICS) or API** | Medium: decision context, participants | Low: ICS parse | Exercises Meeting/`PARTICIPATED_IN`/`DISCUSSED_IN` |
| **GitHub API/webhook** | High but pricier: PR authorship, reviews, modifications | Medium: auth, rate limits, pagination, webhooks | Exercises `AUTHORED/REVIEWED/MODIFIED/HAS_REPO` |

**Recommendation**: start with the docs repo; add the tracker export second.
Both are file-like, need no secrets, and cover the decision + history
workflows end to end.

## 4. Pipeline

```
fetch → stage → parse → normalize → resolve → validate → write fact → index
                          │                         │
                          └── store immutable original (document store)
```

Each stage is pure where possible and records a per-run **ingestion record**
(source, cursor, count, errors).

### 4.1 Source identity & versioning
- Every source has a stable `src_type` and a stable item identity
  (`src_ref`, e.g. `git:org/repo/path@commit`, `csv:incidents/source/row-hash`).
- an item's **content hash** detects real change vs a re-poll.
- Update markers: a per-source **cursor** (commit SHA, `updated_at`, page
  token) drives incremental fetch.

### 4.2 Parse & normalize
- Parse to typed records. Normalize units, timestamps (RFC3339 UTC), casing,
  and enumerated vocabularies.
- Reject records that fail schema validation **per record**, not per batch.
- Ambiguous/free-text extraction (NER, LLM) is out of scope until it carries
  an explicit `extraction` method and `confidence`.

### 4.3 Deduplicate & resolve entities
- Deterministic tiers first (`SCHEMA.md` §9): key → name → case-insensitive →
  prefix → alias.
- Structured facts (a registry row) can auto-merge by exact key.
- Fuzzy matches **never auto-merge**; they yield candidates for review.
- Re-ingesting the same `(src_ref, content-hash)` is a no-op (idempotency).

### 4.4 Validate & confidence
- Every candidate fact must carry the provenance minimum (`SCHEMA.md` §7) or
  it is rejected.
- Deterministic extraction → `confidence` omitted (implied 1.0).
- Non-deterministic extraction → labeled REPORTED CLAIM with `confidence` +
  `extraction`; never promoted to FACT.

### 4.5 Write
- `internal/ingest` is the **only** fact writer (invariant 1).
- Mutating edges: close+open, append-only (`knowledge-model.md`).
- Writes for one source run are grouped so a failure can be rolled back or
  resumed without partial corruption.

## 5. Incremental synchronization & deletion propagation

- **Incremental**: fetch only the delta since the cursor; re-derive facts for
  changed items.
- **Update**: close old state edge, open new; do not duplicate.
- **Deletion / retraction**: when a source removes an item, mark its facts
  retracted (`valid_to` set or `retracted_at`), remove them from default
  retrieval, keep them for audit; propagate to the index and document store.
- **Reconciliation**: a periodic full scan detects drift the delta missed
  (missed webhooks, cursor bugs).

## 6. Reliability: retries, idempotency, partial failure, observability

| Concern | Approach |
|---|---|
| **Retries** | Bounded exponential backoff per source; per-item, not per-batch |
| **Idempotency** | `(src_type, src_ref, content-hash)` uniqueness; re-runs converge |
| **Partial failure** | Source runs are isolated; one source's failure never blocks another; failed items are queued for retry |
| **Poison records** | Quarantine single bad records with the raw payload; never abort the batch |
| **Observability** | Per-run record: items seen/written/skipped/failed, latency, last cursor; expose via diagnostics CLI (`APPFLOW.md`) |
| **Backpressure** | Bounded worker pool per source (I/O-bound); no unbounded fan-out |

(`ponytail:` at demo scale, a single sequential per-source worker is enough;
upgrade path is a bounded pool when a real source's volume demands it.)

## 7. Permission & retention propagation

- Each stored fact records the source's **access scope** and **retention
  class**.
- Derived data (summaries, aggregates, embeddings) inherits the **most
  restrictive** scope of its inputs.
- When source access changes or is revoked, derived visibility is recomputed;
  answers must not leak what the asker cannot see (`SECURITY.md` §derived
  leakage).
- Retention: facts outlive their source only if retention allows; otherwise
  they are retracted with the source.

## 8. Source adapter contract (testable interface)

Each adapter implements one small contract so the pipeline stays source-neutral:

```
Source {
  Type() string
  Fetch(cursor) -> (items, nextCursor, error)   // incremental
  ItemID(raw) string                            // stable src_ref
  ContentHash(raw) string                        // change detection
  Scope(raw) Permission                          // access scope
}
Parser {
  Parse(raw) -> (records, error)                 // pure, per-record errors
}
```

**Testing contract for every adapter** (one runnable check each, per AGENTS.md):
- deterministic parse of a small fixture;
- idempotent re-ingest (second run writes zero new facts);
- a malformed fixture yields a quarantined record, not a crashed run;
- a synthetic deletion retracts the right facts and only those.

## 9. Semantic compression and organizational memory

### 9.1 File compression ≠ semantic compression
File compression (gzip) shrinks bytes; **semantic compression** shrinks what a
human or model must read to understand a body of information, by extracting
structure. TRON's memory is semantic: it turns prose and records into typed
entities, relationships, decisions, and commitments that can be reasoned over.

### 9.2 What extraction must preserve

| Preserve | As | Never do this |
|---|---|---|
| Entities & relationships | graph nodes/edges + provenance | flatten into prose only |
| Decisions & rationale | Decision node + `SUPPORTED_BY`/`DISCUSSED_IN` + rationale text | state a decision without its evidence |
| Commitments, owners, deadlines | Commitment/`COMMITTED` + due_at | summarize "we'll do X" with no owner/date |
| Metrics, units, periods | Metric + Observation (unit + period) | copy a bare number without unit/period |
| Contradictions | both facts retained | silently pick one |
| Uncertainty / missing info | explicit gap + question | fabricate a value |
| Source links & versions | `src_ref` + content hash | summarize away the source |
| Temporal changes | close+open; supersession | overwrite history |

### 9.3 Summaries never replace evidence
A summary is a **derived view**, tagged as such. The document store retains
the immutable original and the graph retains `src_ref`. When a summary is
insufficient, TRON **retrieves the source** — the answer pipeline falls back
from summary → graph facts → original document (see `REASONING.md`). A
citation must resolve to openable evidence, even when the displayed text came
from a summary.

### 9.4 Retrieval indexes (optional)
A lexical or semantic index over document text is a **recall aid** that
proposes candidate keys to the graph, never a source of truth. It is
rebuildable from the document store, so it can be added or dropped without an
ADR to the core. Graph traversal remains the reasoning substrate
(`TECHSPEC.md` stance).

## 10. Definition of done (for the first real ingestion milestone)

- One adapter live, with its four contract tests passing against a fixture.
- Facts land with full provenance; re-running writes nothing new.
- A deletion and a correction both propagate correctly and are visible in
  history.
- An answer can cite a fact back to an openable source in the document store.
- Ingestion diagnostics are exposed and readable.
- Security review confirms permission/retention propagation ([`SECURITY.md`](SECURITY.md)).
