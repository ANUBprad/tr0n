# TRON demo recording script

**Target:** 3–5 minutes, one continuous session against the running product.
**Last updated:** 2026-10-17.

> **Recording status: BLOCKED on this machine.** This workspace has no screen
> recorder and no interactive desktop session, so no new video was produced and
> the existing `artifacts/demo/tron-walkthrough.mp4` (a captioned sequence of
> five captured screenshots) was **not** relabelled as a live recording. Follow
> the checklist below to record the real session on a machine with a recorder.
> See `artifacts/demo/README.md` for the exact manual step.

Use the fictional synthetic seed only. Never show credentials, tokens, private
user data, or unrelated browser tabs.

## Before recording (clean state)

1. `docker compose up -d falkordb`
2. `go run ./cmd/tron seed` — expect `seeded graph "tron": 339 nodes, 958 edges`.
3. `go run ./cmd/tron demo` — expect `7/7 demo checks passed`.
4. `go run ./cmd/tron serve` in a second terminal, then open `http://127.0.0.1:8080`.
5. (Optional local model) `docker compose --profile model up -d ollama` then
   `docker compose exec ollama ollama pull qwen3:4b` (the exact tag you will use).
   For a host Ollama: `TRON_OLLAMA_URL=http://127.0.0.1:11434` and
   `TRON_OLLAMA_MODEL=qwen3:4b`. Never show the full environment.
6. Keep the terminal visible for the graph proof; close unrelated tabs.

## Narration and clicks

**0:00–0:20 — The problem**
> "A company's answer is rarely in one document. Ownership, expertise,
> incidents, and decisions are connected. TRON stores those links in FalkorDB so
> it can show how every answer was reached."

**0:20–0:45 — Clean start and seeded graph**
Show the terminal: `docker compose up -d falkordb`, then `go run ./cmd/tron seed`
reporting 339 nodes / 958 edges, then `go run ./cmd/tron demo` ending in
`7/7 demo checks passed`.
> "The graph is FalkorDB, seeded from a deterministic synthetic fixture. Those
> seven checks are live queries, not prerecorded responses."

**0:45–1:15 — Ownership + evidence drill-down**
In the workspace (mode **Deterministic workflows**) click **01 · Ownership**.
Point out the `OWNS` path in **Evidence paths** and the **Source records** panel;
open a source to show its id, type, and validity window.
> "This is a fact returned from the graph with its source and window — not a
> generated sentence."

**1:15–1:45 — Multi-hop traversal**
Click **02 · Expertise**. Point out the `INFERENCE` badge, `expertise/v1`, the
score, and the basis paths. Switch to the terminal and run
`go run ./cmd/tron owner payments-api` (or show the **Connected context** graph).
> "TRON walks pull requests, incidents, documents, and ownership; there is no
> stored `KNOWS` edge."

**1:45–2:15 — Historical `as_of`**
Set **As of** to `2025-02-15` and ask *"Who owns payments-api?"*. Show Grace
Kowalski (the handoff predecessor). Change it to `2025-06-01` and show Ada
Mercer.
> "The window is half-open: the handoff instant belongs to the later owner."

**2:15–2:50 — Source update / synchronization**
In the terminal, `go run ./cmd/tron sync examples/markdown-source.json` (or
`go run ./cmd/tron import examples/sources.json`), show the changed/unchanged
counts, then ask a question that reflects the update in the workspace.
> "Structured sources are re-synchronized explicitly; an older observation is
> refused, never merged."

**2:50–3:40 — Optional local model**
Set **Mode** to **Local model · grounded Q&A**. Show `/v1/model` (or the UI) for
the configured tag — e.g. `qwen3:4b` — **without** revealing credentials. Ask
*"Who owns payments-api?"* and show the answer plus the per-phase timing
(`graph_ms`, `model_ms`, `turns`).
> "The model only selects from graph-retrieved evidence; it cannot invent an
> entity or a source."
> If the model is not installed, mark this segment **blocked** and record the
> deterministic path instead — do not simulate it.

**3:40–4:10 — Safe abstention**
Ask an unsupported question, e.g. *"What was the company revenue last quarter?"*
and then an ambiguous one, e.g. the bare word *"Adopt"* (Context: Detect).
> "Unsupported and ambiguous questions abstain or ask for clarification; TRON
> never picks the first candidate."

**4:10–4:40 — Verification close**
Return to the terminal and show `7/7 demo checks passed`.
> "TRON is an evidence-first Company Brain prototype. This build uses synthetic
> data; structured file import supports source refresh; live production
> connectors and hosted deployment are not implemented."

## Recording rules

- Capture the actual running product; no slides, no fabricated terminal text.
- Check audio, readability, and that the file opens before publishing.
- Keep the full unedited recording as the source artifact. Label any shortened
  edit as **edited**.
- Save under `artifacts/demo/` with a date/version name, e.g.
  `tron-live-2026-10-17.mp4`.
- Only after the file exists and plays, update this script and the README link.

## Publish the recording

Upload to a public or judge-accessible location, then replace the demo link in
`README.md`. Confirm it opens in a signed-out browser before submitting.
