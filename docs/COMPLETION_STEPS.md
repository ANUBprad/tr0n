# Company Brain completion: 13 reviewable steps

These commits organize the AI-assisted completion work for review. They retain the existing team history through `a662006` and do not imply 13 separate development sessions. The module path remains the original project path to preserve internal imports and provenance.

1. **Knowledge contracts:** reject blank claims/evidence and retain the graph keys supporting expertise scores.
2. **Temporal graph correctness:** use actual current time, validate observation dates and whole-second validity, and support sourced ownership handoffs with history and retry checks.
3. **Source ingestion:** add the graph catalog and idempotent structured document/chat/ticket import with identity, freshness, and link-update checks.
4. **Synthetic demonstration data:** add recorded decision rationale and readable ADR/chat/ticket sources with coherent source metadata.
5. **Agent execution bounds:** preserve upstream context assembly while adding cancellation, tool-call limits, and invalid-move checks.
6. **Question workflows:** route five intents, clarify ambiguous entities, return layered evidence, and generate unsent handoff drafts.
7. **Application endpoints and CLI:** expose catalog/ask endpoints plus explicit import and ownership-update commands.
8. **Browser workspace:** implement question/context inputs, evidence inspection, graph navigation, responsive layout, and handoff download.
9. **Docker runtime:** package the Go app, pin FalkorDB, persist data, and bind local ports to loopback.
10. **Continuous checks:** run static analysis, real-FalkorDB race tests, and demo checks in GitHub Actions.
11. **Architecture and query documentation:** document implemented behavior, Cypher dependencies, ingestion boundaries, and decisions.
12. **Submission guide:** provide setup, track, AI disclosure, honest scope, and this change guide.
13. **Demo and verification evidence:** include the captured walkthrough, recording script, implementation tracker, and verification report.

## Before submitting

- Run the README setup and `docker compose run --rm app demo` on the machine used for the presentation.
- Watch the video; it is a captioned sequence of actual app screenshots, not a continuous screen recording. Consider recording the longer narrated script to show interactive source inspection and updates.
- Confirm the public source and video links open without signing in.
- Explain why FalkorDB is necessary, how validity windows work, how expertise is ranked, and what remains deterministic or unsupported.
- Disclose AI assistance and your own contribution accurately. Submit Track 03 only unless you separately satisfy another track.
