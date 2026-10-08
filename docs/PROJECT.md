# PROJECT.md

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION

## What is TRON?
TRON (Trusted Reasoning & Organizational Nexus) is a Company Brain that understands organizational context by connecting people, teams, projects, services, systems, documents, tickets, incidents, decisions, meetings, engineering activity, expertise, and ownership into a unified graph.

## Problem Statement
Organizations accumulate knowledge across many siloed tools (tickets, docs, chats, PRs, incidents). When someone asks "Who knows this system best? Why was this decision made? Who owns this? Has this happened before?", answers require connecting distributed context across people, systems, and history.

## Why Existing RAG is Insufficient
- Flat chunk retrieval loses relationship structure
- Context is fragmented across entities
- Multi-hop questions ("who should investigate based on prior experience with this service") don't map cleanly to vector similarity
- Provenance/evidence paths are hard to surface reliably

## Why a Graph is Necessary
Organizational knowledge is fundamentally relational. Questions require traversals: Person → works on → Project → depends on → Service → affected by → Incidents. Multi-hop reasoning with explicit paths provides traceable, explainable answers with evidence.

## Why FalkorDB is Necessary
- Native graph database built for connected data and traversal
- Central to reasoning (not just visualization)
- Removing FalkorDB would materially affect the product's core capability
- Well-suited for multi-hop path queries, graph algorithms, and evidence extraction

## Intended Differentiator
**Graph-central, evidence-driven reasoning.** TRON uses deliberate graph retrieval (not dumping graph to LLM) with tool selection, traversals, and provenance. Every answer should be supported by concrete graph paths.

## Primary Track
TRACK 03 — COMPANY BRAIN

## Possible Secondary Tracks
- TRACK 01 — Agents That Act on Connected Data (if actions are added)
- TRACK 02 — Agent Memory and Coordination (if cross-session memory via graph)

## Target User (Conceptual)
Someone navigating organizational complexity (engineer oncall, new team member, tech lead, PM) who needs fast, explainable answers grounded in organizational context.

All functionality described is conceptual/design only.

