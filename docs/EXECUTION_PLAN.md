# EXECUTION_PLAN.md

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION  
**Window**: Oct 15 00:01 AM IST – Oct 18 23:59 PM IST (72 hours)  
**Strategy**: Feature freeze early, demo harden late

## Phase Breakdown

### Hour 0–4 (Kickoff)
- **Objective**: Align, repo setup, scaffold minimal structure
- **Deliverables**: Confirm tech (PROPOSED→decide minimal), tool contracts, data plan
- **Owner**: Team
- **Exit**: Running env, FalkorDB available locally (or setup plan), agreed contracts
- **Fallback**: Simplify choices if blockers

### Hour 4–12 (Graph Foundation)
- **Objective**: Schema + synthetic data
- **Deliverables**: Loadable synthetic graph with required connectivity for 5 queries
- **Owner**: Graph-focused
- **Exit**: Can query sample traversals in FalkorDB
- **Fallback**: Smaller graph, focus on core paths

### Hour 12–24 (Tools + Agent Core)
- **Objective**: Core graph tools + agent loop
- **Deliverables**: find_owner, find_experts, trace_decision, find_related_incidents working
- **Owner**: Agent/backend
- **Exit**: Tools return evidence paths; deliberate retrieval tested
- **Fallback**: Merge tools if needed

### Hour 24–36 (Integration)
- **Objective**: Wire agent→tools→FalkorDB→API
- **Deliverables**: End-to-end for >=3 of 5 queries
- **Owner**: Team
- **Exit**: API returns answer+evidence
- **Fallback**: Stabilize core 3 first

### Hour 36–48 (UI + Complete Queries)
- **Objective**: Minimal UI + remaining 2 queries
- **Deliverables**: Q&A + evidence panel; all 5 work
- **Owner**: Frontend+agent
- **Exit**: Demo-able via UI
- **Fallback**: CLI fallback for demo if UI lags

### Hour 48–60 (Harden + Freeze)
- **Objective**: Stabilize, bugfix, **FEATURE FREEZE** (no scope creep)
- **Deliverables**: Reliable demo on prepared queries
- **Owner**: Team
- **Exit**: Freeze declared; only fixes
- **Fallback**: Cut NICE-TO-HAVE ruthlessly

### Hour 60–72 (Demo Prep + Submission)
- **Objective**: Demo script, video, README polish for submission, verify artifacts
- **Deliverables**: Demo video, live/local setup verified, submission checklist complete
- **Owner**: Team
- **Exit**: Ready to submit
- **Fallback**: Pre-recorded video if deployment flaky

## Critical Rules
- **Aggressive feature-freeze** by ~Hour 48–52
- **Test the 5 queries constantly** — treat as acceptance
- **Evidence-first**: if answer lacks path, fix before polish
- **Graph centrality check**: periodically ask "could we do this without FalkorDB?" If yes, rethink
- **Local-first**: ensure complete local setup works
- **Cut > extend** when behind

## Risk Mitigation
- CLI fallback for demo (if UI breaks)
- Minimal data set (smaller but connected)
- Direct tools vs MCP (choose simplest)
- Pre-select LLM provider early

