# TRON

**Trusted Reasoning & Organizational Nexus**

[![Status](https://img.shields.io/badge/Status-Active%20Development-brightgreen.svg)](docs/architecture-minimal.md)
[![Hackathon](https://img.shields.io/badge/Hackathon-Graph%20Hacks%3A%20Context%20for%20AI%20Agents-blue.svg)](https://wemakedevs.org/hackathons/falkordb-graph-hacks)

## Status

**ACTIVE DEVELOPMENT** — architecture is locked: Go + FalkorDB,
five-layer knowledge model, graph operation boundary.
See [docs/architecture-minimal.md](docs/architecture-minimal.md),
[docs/knowledge-model.md](docs/knowledge-model.md), and
[planning/decision-log.md](planning/decision-log.md).
Graph operations land incrementally, each backed by a real-FalkorDB test.

## Vision

TRON aims to build a **Company Brain** that understands how an organization works by connecting people, teams, projects, services, systems, documents, tickets, incidents, decisions, meetings, engineering activity, expertise, and ownership into a unified graph.

The intelligence is graph-native: FalkorDB is the primary graph database and the reasoning is driven by multi-hop traversals with explicit provenance, not just retrieval.

## Target Track

**Primary**: [TRACK 03 — COMPANY BRAIN](https://wemakedevs.org/hackathons/falkordb-graph-hacks)

**Potential Secondary**: TRACK 01 (Agents That Act on Connected Data), TRACK 02 (Agent Memory and Coordination)

## Core Problem

Existing RAG approaches lose organizational context and relationship structure. A graph is necessary because organizational knowledge is relational (who knows what, why decisions were made, who owns what, what happened before) and requires multi-hop reasoning with evidence paths.

## Key Differentiator

Graph-central reasoning with provenance. Remove FalkorDB and the product's core reasoning capability is materially affected.

## Quick Links

- [Project Overview](docs/PROJECT.md)
- [Hackathon Details & Constraints](docs/HACKATHON.md)
- [Product Specification](docs/PRODUCT_SPEC.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Graph Model](docs/GRAPH_MODEL.md)
- [Agent Specification](docs/AGENT_SPEC.md)
- [Data Specification](docs/DATA_SPEC.md)
- [Demo Specification](docs/DEMO_SPEC.md)
- [Judging Strategy](docs/JUDGING_STRATEGY.md)
- [MVP Scope](docs/MVP_SCOPE.md)
- [Team Plan](docs/TEAM_PLAN.md)
- [Execution Plan](docs/EXECUTION_PLAN.md)
- [Security & Data](docs/SECURITY_AND_DATA.md)
- [Open Questions](docs/OPEN_QUESTIONS.md)
- [Decision Log](planning/decision-log.md)

## Build Status

- [x] Repository structure
- [x] Engineering constraints defined
- [x] Planning foundation documented
- [x] Architecture lock (stack, knowledge model, graph schema)
- [x] Go scaffold: FalkorDB client + FindOwner (real-DB test)
- [x] FindExperts with deterministic expertise/v1 scoring (real-DB test)
- [x] TraceDecision: decision provenance neighborhood (real-DB test)
- [x] FindRelatedIncidents: temporal + literal keyword filtering (real-DB test)
- [x] TraceEvidence: full refetch + 1-hop provenance drill-down (real-DB test)
- [x] ResolveEntity: deterministic internal resolver (exact key → exact name → case-insensitive → prefix)
- [x] All five graph operations implemented with real-DB tests
- [x] Seed data generator: deterministic `internal/seed` + `tron seed` (five-query connectivity contract tested)
- [x] JSON API layer: read-only `internal/api` over all six ops (`tron serve`, real-DB HTTP contract test)

## Quick Start

```bash
docker compose up -d                          # FalkorDB on :6379
go run ./cmd/tron seed                        # synthetic graph (destructive reseed)
go run ./cmd/tron serve                       # JSON API on http://127.0.0.1:8080
curl -s http://127.0.0.1:8080/v1/owner -d '{"target":"payments-api"}'
```

`TRON_FALKOR_ADDR` (default `localhost:6379`) and `TRON_HTTP_ADDR`
(default `127.0.0.1:8080`) override the endpoints. `go test ./... -short`
runs the pure tests without Docker; drop `-short` for the full
real-FalkorDB suite.

---

**Architecture is locked** — see
[docs/architecture-minimal.md](docs/architecture-minimal.md) and
[planning/decision-log.md](planning/decision-log.md). Older hackathon-era
documents (e.g. [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md),
[docs/HACKATHON.md](docs/HACKATHON.md)) are retained for history and
marked as superseded where they conflict.

