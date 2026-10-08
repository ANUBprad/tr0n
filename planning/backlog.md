# backlog.md

**Status**: PRE-HACKATHON — Planning backlog (NOT implementation tasks)

## PRE-HACKATHON (Complete/Planned)
- [x] Create opencode.md with constraints
- [x] Initialize repo structure (docs/diagrams/planning)
- [x] Write foundation docs
- [x] Create planning artifacts

## POST-OCT 15 (Hackathon Implementation Backlog)

### P0 — Graph Foundation (Hours 0–12)
- [ ] Decide final minimal schema
- [ ] Implement synthetic data generator
- [ ] Load data into FalkorDB
- [ ] Verify connectivity for 5 queries
- [ ] Smoke test traversals

### P1 — Agent + Tools (Hours 12–24)
- [ ] Implement search_graph, find_owner
- [ ] Implement find_experts, trace_decision, find_related_incidents
- [ ] Implement trace_evidence (minimal)
- [ ] Agent loop with deliberate retrieval
- [ ] Evidence extraction/format

### P2 — Integration + API (Hours 24–36)
- [ ] HTTP API endpoints (Go net/http)
- [ ] Wire agent→tools→FalkorDB
- [ ] End-to-end for 3+ queries
- [ ] Error handling basics

### P3 — UI + Demo (Hours 36–48)
- [ ] Minimal Q&A UI
- [ ] Evidence panel (paths)
- [ ] All 5 queries working
- [ ] Demo script validation

### P4 — Polish/Submission (Hours 48–72)
- [ ] Feature freeze + fixes
- [ ] Demo video
- [ ] README for submission
- [ ] Local setup verification
- [ ] Final artifacts check

All items above are POST-hackathon-start only.

