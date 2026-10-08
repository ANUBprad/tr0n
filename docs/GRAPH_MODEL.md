# TRON — Graph Model (Initial Schema, Locked)

**Status**: LOCKED (2026-10-08) — initial schema for the first useful
product, not a model of the whole world. See `planning/decision-log.md`.
Provenance/temporal rules: `knowledge-model.md`. Cypher lives only in
`internal/graph/` at implementation time.

## Scope

Entities exist only if a locked workflow needs them (ownership, expertise,
decision trace, incident history, routing). Everything else is deferred
until a real query demands it.

## Entities (initial: 11)

All nodes: `key` = `"<src>:<native-id>"` (identity, indexed, unique per
label), plus node provenance `src_type`, `src_ref`, `observed_at`,
`extraction`. `name`/`title` indexed per label for entity resolution.

| Entity | Required properties | Optional properties | Native timestamps |
|---|---|---|---|
| **Person** | `key`, `name` | `email`, `aliases[]` (other sources' keys) | — |
| **Team** | `key`, `name` | — | — |
| **Project** | `key`, `name` | `status` | — |
| **Repository** | `key`, `name` | `url` | — |
| **Service** | `key`, `name` | `status` | — |
| **Document** | `key`, `title`, `kind` (adr/runbook/spec/note) | `url` | `published_at` (optional — see amendment) |
| **Decision** | `key`, `title`, `status` (proposed/accepted/rejected/superseded) | — | `decided_at` |
| **Meeting** | `key`, `title` | — | `held_at` (required at ingest) |
| **Incident** | `key`, `title` | `severity`, `resolved_at` | `started_at` (required at ingest) |
| **PullRequest** | `key`, `title`, `number`, `state` (open/merged/closed) | — | `created_at`, `merged_at` |
| **Technology** | `key`, `name` | — | — |

**Amendment (2026-10-08, scaffold):** `Document.published_at` added as an
optional native timestamp — the `AUTHORED` row says "event time on
artifact", but Document had none, leaving `expertise/v1`'s doc signal
(0.8) unable to decay. Null is allowed: an undated document scores with
age 0 (no decay) until a source provides the date.

### Deferred entities (and why)

| Entity | Why deferred |
|---|---|
| System | Service covers MVP queries; a hierarchy needs a proven aggregation question first |
| Ticket | Incident (event history) + PullRequest (work history) cover the five workflows |
| Conversation / Chat | Needs a real chat source; synthetic meetings/documents carry discussion context for v1 |
| Deployment | "What changed" is answerable via PR `merged_at` near incident `started_at`; add when release data exists |
| Organization | Single-organization scope for now; add with multi-org tenancy (graph-name boundary already exists) |

## Relationships (initial: 16 types)

Every relationship below is **explicit** — extracted from a source by a
deterministic rule (a fact). None are inferred; none carry `confidence` in
v1 (see `knowledge-model.md`). Temporal = carries
`valid_from`/`valid_to` windows.

| Relationship | From → To | Semantics | Temporal | Source (typical) |
|---|---|---|---|---|
| `WORKS_IN` | Person → Team | Membership (the person) | **yes** | HR/org registry |
| `OWNS` | Person\|Team → Service\|Repository\|Project | Accountability (single owner edge per target per era) | **yes** | service registry |
| `WORKED_ON` | Person → Project | Project participation (assignments) | **yes** | project tracker |
| `AUTHORED` | Person → Document\|Decision\|PullRequest | Authorship of an artifact | no (event time on artifact) | git, ADR repo, tracker |
| `REVIEWED` | Person → PullRequest | Review activity | no (event time on PR) | git |
| `MODIFIED` | PullRequest → Repository | PR touched files in repo | no (event time on PR) | git |
| `HAS_REPO` | Service → Repository | Which code backs this service | yes (registry changes) | service registry |
| `DEPENDS_ON` | Service → Service | Runtime/build dependency | **yes** | service registry |
| `AFFECTS` | Incident → Service | Blast radius of an incident | no (`started_at` on incident) | incident tracker |
| `RESOLVED_BY` | Incident → Person | Who resolved it | no | incident tracker |
| `SUPPORTED_BY` | Decision → Document | Evidence for the decision | no | ADR repo |
| `DISCUSSED_IN` | Decision\|Incident → Meeting\|Document | Discussion context | no | calendar, ADR repo |
| `PARTICIPATED_IN` | Person → Meeting | Attendance | no (`held_at` on meeting) | calendar |
| `SUPERSEDES` | Decision → Decision; Document → Document | Evolution/replacement | no | ADR repo |
| `ABOUT` | Document → Service\|Project\|Technology | What a document is about | no | doc metadata/topic |
| `USES` | Service → Technology | Technology usage | no | service registry |

### Multi-hop paths the schema exists to support

- **Expertise**: `Person →AUTHORED→ PR →MODIFIED→ Repo ←HAS_REPO← Service`
  (4 hops — see `knowledge-model.md` for scoring)
- **Routing**: `Incident →AFFECTS→ Service ←OWNS← Person|Team` +
  `Incident →RESOLVED_BY→ Person` + expertise inference
- **Decision trace**: `Decision ←SUPPORTED_BY← …` + `Decision →DISCUSSED_IN→
  Meeting ←PARTICIPATED_IN← Person`
- **History**: `Service ←AFFECTS← Incident` ordered by `started_at`,
  with `RESOLVED_BY` people attached

### Removed from the original candidate list (and why)

| Cut | Reason |
|---|---|
| `MEMBER_OF` | Duplicate of `WORKS_IN` |
| `KNOWS` | Would store an inference as a fact. Expertise is *computed* from activity facts, never asserted as an edge (invariant #2 in `knowledge-model.md`) |
| `RELATED_TO` | Unfalsifiable catch-all; "because they are related" is not evidence. (ponytail: weak-link queries answered with specific paths; upgrade path: a typed weak relation once a real query needs one) |
| `CAUSED` | Root-cause chains answerable via `AFFECTS` + PR `merged_at` timeline. Upgrade path: explicit `CAUSED` when a source actually asserts causality |
| `IMPLEMENTS` | `WORKED_ON` + `MODIFIED` + `HAS_REPO` cover implementation links for MVP |
| `WORKED_ON → Service` (variant) | Would double-count with PR/incident signals in expertise scoring; service-level activity comes from `MODIFIED`/`REVIEWED`/`RESOLVED_BY`/`OWNS` only |
| `SYSTEM`-flavored duplicates of every edge | Deferred entity (see above) |

### Added (genuinely missing from the candidate list)

- `HAS_REPO` — without it, PR activity cannot reach services
  (`PR →REPO→ … ←HAS_REPO← Service`); it is the bridge that makes code
  history answer organizational questions.
- `ABOUT` — connects authored documents to their subject, so doc authorship
  contributes to expertise and decision evidence.
- `USES` — grounds "relevant technology experience" in the routing workflow.

## Indexes (design note)

- Unique index on `key` for every label (identity + fast resolve).
- Index on `name` (or `title`) per label for entity resolution by name.
- No other indexes until a query plan demands one (measure first).
