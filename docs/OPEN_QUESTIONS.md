# OPEN_QUESTIONS.md

**Status**: Living list. Items move to `planning/decision-log.md` when
decided — never decide silently in a spec.

## Resolved by the 2026-10-08 architecture lock

| Former question | Resolution |
|---|---|
| Implementation language | **Go** (`technology-evaluation.md`) |
| Exact graph schema | **Initial 11 entities / 16 relationships** (`GRAPH_MODEL.md`) |
| Expert scoring | **Deterministic `expertise/v1` weighted signals** (`knowledge-model.md`) |
| Evidence presentation | **Layer-tagged claims + path lists per answer** (`AGENT_SPEC.md`) |
| Tool interface (direct vs MCP) | **In-process Go functions behind one JSON API**; MCP only ever as an optional external adapter, if a consumer exists |
| Action mechanism | **Deferred by design**: ACTION layer + `prepare_handoff` live above the graph boundary, built only when the core is proven (`AGENT_SPEC.md`) |
| FalkorDB/Redis Go client | **`falkordb-go/v2`** (official, BSD-3, graph-aware: `SelectGraph`/`ROQuery` with typed records; pulls `go-redis/v9` transitively). `rueidis` rejected: generic client would mean hand-parsing `GRAPH.QUERY` results |

## Resolved by the M1 readiness pass (2026-10-15)

| Former question | Resolution |
|---|---|
| **Key uniqueness enforcement** | **Implemented (2026-10-17):** `tron constraints` creates/verifies per-label database uniqueness constraints; application writers also reject duplicate keys across labels. Direct external writers still need to preserve the cross-label invariant. The original 2026-10-15 deferral is historical and superseded; see `GRAPH_MODEL.md` and `planning/decision-log.md`. |
| **E6 contradiction semantics** (previously ambiguous) | **DECIDED:** E6 v1 flags incompatible *state*-relation claims whose half-open `[valid_from, valid_to)` windows overlap; `OWNS` = different owners of the same target; other state relations only where `GRAPH_MODEL.md` declares exclusivity; a repeated same-counterpart claim is corroboration, not contradiction; event edges and node-property conflicts are out of scope. First exposure is the read-only `POST /v1/conflicts` (implemented 2026-10-16). See `planning/decision-log.md` and `docs/SCHEMA.md` §6. |

## Resolved by the E3 source-grounded answer contract (2026-10-16)

| Former question | Resolution |
|---|---|
| **How should a `knowledge.Answer` carry structured per-fact provenance from context assembly?** | **DECIDED: option C — an additive `graph.GroundedAnswer` envelope.** The answer's claims are bound to the *actual* retrieval result: `GroundedAnswer` carries the unchanged `knowledge.Answer`, the `EvidenceContext` it was built from, and one `ClaimGrounding` per claim naming the claim's subject entity, the relationship (when asserted), and the IDs of the supporting facts. `graph.EvidenceFact` gains a stable content-derived `id` so a claim cites specific evidence rather than a position. `GroundedAnswer.Validate` requires every FACT claim to cite ≥1 known fact matching its subject/relationship, and refuses any FACT claim on a non-`ok` context (`not_found`/`ambiguous`/`empty`/`over_budget`). Options A (extend `knowledge.Claim` — impossible: `knowledge` cannot import `graph`) and B (link by path string — reconstructs provenance from display strings) were rejected; see `planning/decision-log.md`. **E3 stays PARTIAL**: the local model and labeled corpus now run; structural linkage and a ten-sample participant review do not establish broad factual correctness/entailment, and comparative baselines remain open. |

## Resolved by the 2026-10-17 competition build

| Question | Resolution |
|---|---|
| **Optional local answering provider** | **DECIDED: Ollama** for the optional grounded-answer path. `internal/agent/ollama.go` implements the `agent.Decide` adapter; model evaluation and participant-review status are in `EVALUATION.md`. Deterministic workflows still need no model; hosted providers remain out of scope. |

## Still open

| Question | Options | Notes |
|---|---|---|
| **Agent framework** | Custom minimal loop vs library | **RESOLVED (2026-10-08): custom minimal loop** — `internal/agent`, provider-agnostic `Decide` seam, bounded step budget, tool errors fed back to the model. No framework; see decision-log. |
| **Frontend** | Server-rendered minimal page / Vite+React / CLI-first | **RESOLVED (2026-10-08): server-rendered minimal page** — stdlib `html/template` served by `tron serve` at `/`, five built-in questions + evidence panels over the JSON API. No JS toolchain; swap is cheap if needed. |
| **FalkorDB setup** | Docker compose (default candidate) vs binary | **RESOLVED: Docker Compose** — `compose.yaml` uses a named persistent volume; `tron seed` explicitly resets only the selected demo graph. |
| **Data generation strategy** | Go generator in-repo vs external script | **RESOLVED (2026-10-08): Go generator in-repo** — `internal/seed` + `tron seed`, deterministic `Generate()` + direct bulk load; see decision-log. |
| **Deployment** | Local-only vs cloud | UNDECIDED. Complete local setup is the floor; cloud is optional. |
| **Enter Track 01 / Track 02?** | Yes / No | UNDECIDED. Only if the natural build earns it. |
| **Should a key-shaped query string be confined to the `key` tier?** | (A) confine `src:id`-shaped input to the `key` tier only; (B) require a stricter key shape than `keyShapeOK` first; (C) leave the fall-through (exact name/title tiers) as is | UNDECIDED (2026-10-16). `ResolveEntity` matches the query against tiers in order (`SCHEMA.md` §9): a string that *looks* like a key but matches no node's `key` keeps falling through to exact `name`/`title`. That is contract-legal today — the `key` tier is simply a higher-priority tier, not a gate. Risk: a non-existent key can still resolve to an unrelated entity by an exact name/title coincidence, i.e. stable-identity *evidence* arriving as weak *attributes*. Why not fixed now: `keyShapeOK` only checks for `:` with both sides non-empty, so it cannot separate a key from a legitimate colon-containing **title** (`"Adopt: FalkorDB"` is a valid title and a key-shaped string). Constraining the tier would regress that lookup; Option B is a stricter identity-shape decision. Locked as current behavior in `TestResolveEntityContract/key-shaped_input_still_resolves_by_exact_title` so the change is deliberate. Options + consequences recorded in `planning/decision-log.md`. |
| **When the answer envelope is next revised, should absence claims ("no `OWNS` edge") be groundable too?** | (A) keep the operation-reference string path (current); (B) add an explicit "absence/emptiness evidence" kind a claim can cite; (C) leave absence claims out of the grounded contract entirely | UNDECIDED (2026-10-16). `graph.GroundedAnswer` grounds a FACT claim only by citing an `EvidenceFact`, and a non-`ok` (e.g. `empty`) context can never back one — so absence claims keep their existing string path and are not structurally grounded. Making them groundable needs a deliberate evidence-of-absence design (which retrieval produced the emptiness, under which scope/as-of). See `planning/decision-log.md` E3 entry. |
