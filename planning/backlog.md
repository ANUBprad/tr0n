# backlog.md

**Status**: PRE-HACKATHON — Planning backlog (NOT implementation tasks)

## PRE-HACKATHON (Complete/Planned)
- [x] Create opencode.md with constraints
- [x] Initialize repo structure (docs/diagrams/planning)
- [x] Write foundation docs
- [x] Create planning artifacts

## POST-OCT 15 (Hackathon Implementation Backlog)

### P0 — Graph Foundation (Hours 0–12)
- [x] Decide final minimal schema
- [x] Implement synthetic data generator
- [x] Load data into FalkorDB
- [x] Verify connectivity for 5 queries
- [x] Smoke test traversals

### P1 — Agent + Tools (Hours 12–24)
- [x] Implement find_owner (search_graph superseded by ResolveEntity)
- [x] Implement find_experts, trace_decision, find_related_incidents
- [x] Implement trace_evidence (minimal)
- [x] Agent loop with deliberate retrieval
- [x] Evidence extraction/format

### P2 — Integration + API (Hours 24–36)
- [x] HTTP API endpoints (Go net/http)
- [x] Wire agent→tools→FalkorDB
- [x] End-to-end for 3+ queries
- [x] Error handling basics

### P3 — UI + Demo (Hours 36–48)
- [x] Minimal Q&A UI
- [x] Evidence panel (paths)
- [x] All 5 queries working
- [x] Demo script validation

### P4 — Polish/Submission (Hours 48–72)
- [ ] Feature freeze + fixes
- [ ] Demo video
- [ ] README for submission
- [ ] Local setup verification
- [ ] Final artifacts check

All items above are POST-hackathon-start only.

