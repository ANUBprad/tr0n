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
