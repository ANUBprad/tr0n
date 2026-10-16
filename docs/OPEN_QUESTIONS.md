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
| **Key uniqueness enforcement** | **DECIDED: defer database-level enforcement**; v1 accepts loader-/ingestion-enforced uniqueness. The non-unique index plus the bulk loader's in-batch check do **not** prevent duplicate `key` nodes across separate loads — a known limitation, **not** a relaxation (the unique-per-label requirement stands, `GRAPH_MODEL.md` Indexes; duplicate-key nodes can merge or mask an E6 conflict group). Recorded in `planning/decision-log.md`; DB enforcement is deferred to the backlog, revisit before multi-source ingestion (M2). |
| **E6 contradiction semantics** (previously ambiguous) | **DECIDED:** E6 v1 flags incompatible *state*-relation claims whose half-open `[valid_from, valid_to)` windows overlap; `OWNS` = different owners of the same target; other state relations only where `GRAPH_MODEL.md` declares exclusivity; a repeated same-counterpart claim is corroboration, not contradiction; event edges and node-property conflicts are out of scope. First exposure is the read-only `POST /v1/conflicts` (implemented 2026-10-16). See `planning/decision-log.md` and `docs/SCHEMA.md` §6. |

## Still open

| Question | Options | Notes |
|---|---|---|
| **LLM Provider** | OpenAI / Anthropic / Local (Ollama) / Gemini | UNDECIDED. Irrelevant to v1 core (no LLM needed); matters only for the future agent layer. Consider cost, rate limits, local fallback. Seam is ready: `agent.Decide` in `internal/agent` — one adapter function per provider. |
| **Agent framework** | Custom minimal loop vs library | **RESOLVED (2026-10-08): custom minimal loop** — `internal/agent`, provider-agnostic `Decide` seam, bounded step budget, tool errors fed back to the model. No framework; see decision-log. |
| **Frontend** | Server-rendered minimal page / Vite+React / CLI-first | **RESOLVED (2026-10-08): server-rendered minimal page** — stdlib `html/template` served by `tron serve` at `/`, five built-in questions + evidence panels over the JSON API. No JS toolchain; swap is cheap if needed. |
| **FalkorDB setup** | Docker compose (default candidate) vs binary | **RESOLVED (2026-10-08): Docker compose** — `compose.yaml` in repo, local-first, no volume (seed recreates data). |
| **Data generation strategy** | Go generator in-repo vs external script | **RESOLVED (2026-10-08): Go generator in-repo** — `internal/seed` + `tron seed`, deterministic `Generate()` + direct bulk load; see decision-log. |
| **Deployment** | Local-only vs cloud | UNDECIDED. Complete local setup is the floor; cloud is optional. |
| **Enter Track 01 / Track 02?** | Yes / No | UNDECIDED. Only if the natural build earns it. |
