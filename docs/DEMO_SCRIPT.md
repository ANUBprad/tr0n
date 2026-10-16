# TRON demo recording script

Target: 2–3 minutes. Record the local UI and terminal after following the README setup. Use the fictional seeded graph and avoid showing secrets or personal data. The demo command performs live FalkorDB queries; it is not a prerecorded response.

A local captioned walkthrough is available at `artifacts/demo/tron-walkthrough.mp4`. It uses five captured UI states from the running Docker app, not continuous screen recording. Review it before publishing, or use the script below for a narrated live recording.

## Before recording

1. Start FalkorDB with `docker compose up -d`.
2. Load the synthetic graph with `go run ./cmd/tron seed`.
3. Run `go run ./cmd/tron demo` and confirm it reports `7/7 demo checks passed`.
4. Start `go run ./cmd/tron serve` in a second terminal and open `http://127.0.0.1:8080`.
5. Close unrelated tabs and keep the terminal visible for the graph proof.

## Narration and clicks

**0:00–0:20 — The problem**

“A company’s answer is rarely in one document. Ownership, expertise, incidents, and decisions are connected. TRON stores those links in FalkorDB so it can show how each answer was reached.”

**0:20–0:45 — Ownership**

Open “Who owns payments-api?” Point out the `OWNS` path and the source reference in the evidence panel. Say: “This is a fact returned from the graph, with the edge’s source and validity window.”

**0:45–1:10 — Expertise**

Open “Who knows payments-api best?” Point out the `INFERENCE` badge, `expertise/v1`, score, and basis paths. Say: “TRON computes the ranking from PR, incident, document, and ownership relationships; it does not store an asserted `KNOWS` edge.”

**1:10–1:35 — Decision provenance**

Click “03 · Decisions” for “Why did we choose Redis over Kafka?” Open the linked ADR, chat, ticket, and meeting source records. Say: “The decision is explainable through its source records including the recorded Redis-versus-Kafka rationale.”

**1:35–1:55 — History**

Open “Has payments-api timed out before?” Show incident dates and resolvers. Say: “The answer follows `Incident → AFFECTS → Service` and the resolver edge.”

**1:55–2:20 — Handoff recommendation**

Open “Who should handle a payments-api issue?” Explain that the recommendation combines current ownership, expertise ranking, and incident history. Show its evidence paths.

**2:20–2:45 — Graph dependency**

Switch to the terminal and show `7/7 demo checks passed`, including the as-of ownership check and evidence drill-down. Say: “These are queries against FalkorDB. Without the graph, TRON loses the ownership history, multi-hop expertise paths, decision neighborhood, and incident responders that produce these results.”

**2:45–3:00 — Close**

“TRON is a deterministic, evidence-first Company Brain prototype. This build uses synthetic data and local setup; structured file import supports source refresh; live connectors and hosted deployment are not included.”

## Publish the recording

Upload the recording to a public or judge-accessible location, then replace the demo-video placeholder in the root README with the URL. Confirm the link opens in a signed-out browser before submitting.
