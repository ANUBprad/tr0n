> **Current competition build (2026-10-16):** See `TRACKER.md` and the root README for implemented behavior. The build now includes bounded natural-language routing, inspectable source content, handoff drafts, structured file imports, and sourced ownership updates. General-purpose LLM reasoning, live connectors, and autonomous execution remain future work.

# TRON — Decision Authority & Action Execution

**Status**: Canonical. Authority for the autonomy model, authorization, and
how TRON performs (or refuses to perform) actions.
**Last updated**: 2026-10-12.
**Related**: [`REASONING.md`](REASONING.md), [`SECURITY.md`](SECURITY.md),
[`SCHEMA.md`](SCHEMA.md), [`TECHSPEC.md`](TECHSPEC.md), [`PRD.md`](PRD.md).

> **Current reality (2026-10-17)**: the API includes a partial local draft
> prototype. Tenant credentials must assign the `requester`, `approver`, or
> `executor` role for the corresponding route; the default handler has no
> action identity and rejects these routes. Only `draft_handoff` and
> `draft_ticket` are allowed, and execution creates a local response without
> changing graph or external state. Proposals, approvals, replay keys, and the
> audit chain live in process memory and disappear on restart. Durable action
> records, target adapters, outcome verification, and L3–L5 execution remain
> proposed. See `EVALUATION.md` for the current partial status.

---

## 1. Principle

Action is the highest-risk capability. TRON earns autonomy in **levels**, and
each level is gated by deterministic policy — never by an LLM's judgment. An
LLM may propose an action and explain it; it **cannot** authorize, expand, or
execute one.

## 2. Autonomy levels

| Level | Name | What TRON may do | Gate | Status |
|---|---|---|---|---|
| **L0** | Observe | Read graph, answer questions, surface evidence | None (read) | **IMPLEMENTED** |
| **L1** | Recommend | Produce recommendations with rationale and trade-offs | None (still no side effects) | PARTIAL (route answer is a recommendation) |
| **L2** | Draft | Produce a concrete proposed change (a diff, a message, a ticket body) for a human to send | Human reviews every artifact | PARTIAL (local handoff/ticket draft prototype) |
| **L3** | Human-approved execution | Execute an action **after** an explicit human approval, within the approved scope | Approval record required | PROPOSED |
| **L4** | Bounded pre-authorized execution | Execute a small class of low-risk, reversible actions under a standing, scoped consent | Standing authorization + allowlist + idempotency | PROPOSED |
| **L5** | High-impact operations | Anything affecting money, people, production, or external commitments | Explicit governance (multi-party approval, dry-run, rollback plan) | PROPOSED |

**Default is L0.** A capability graduates only when its lower level is proven
and its tests exist (`EVALUATION.md`).

## 3. Authorization model

Every action request is evaluated by a **deterministic gate** (`AUTHZ` in
`TECHSPEC.md`):

1. **Identity** — who is acting (user, service account) and under what scope.
2. **Scope check** — is this action type permitted for that identity?
3. **Target check** — is the target within the identity's permitted set?
4. **Risk classification** — map the action to a level (L2–L5).
5. **Approval requirement** — does this level require a human approval record?
6. **Budget / rate check** — is the identity within its action budget?
7. **Policy check** — does any `Policy` entity forbid the action?

Outcome: `allow` · `require_approval` · `deny`. Denials and approval requests
are recorded, not just returned.

## 4. Approval records

An approval is a first-class, auditable artifact (`SCHEMA.md` §3.7):

- who approved, their scope, and the exact action+parameters approved;
- a timestamp and expiry;
- a binding to the specific `ProposedAction` (an approval for action A never
  authorizes action B, and never a *modified* A);
- a tamper-evident audit entry (append-only).

Changing any parameter of an approved action invalidates the approval.

## 5. Execution, idempotency, and retries

- Every execution carries an **idempotency key** derived from
  `(action, target, parameters, approval)`.
- Execution states: `proposed → authorized → executing → succeeded | failed |
  rolled_back`.
- Retries reuse the idempotency key so a duplicate delivery cannot double-execute.
- External-system failures are classified: **retryable** (transient, network)
  vs **terminal** (rejected, invalid, permission denied). Terminal failures do
  not retry.
- Partial failure must be recoverable: either the action is atomic on the
  target or a compensating action is defined and recorded.

## 6. Outcome verification

An action is not "done" until verified:

1. **Intent** — the expected state change, declared before execution.
2. **Effect** — read the target back (or receive a source event).
3. **Match** — compare observed to intent.
4. **Record** — write a VERIFIED OUTCOME fact with the effect's `src_ref`
   (`SCHEMA.md` §2), closing the loop into memory.

Unverified or mismatched outcomes are surfaced, never silently marked done.

## 7. Audit log

Append-only, one entry per: proposal, authorization, approval, execution
attempt, retry, outcome, rollback. Each entry references the recommendation
and evidence that justified the action, so any action is explainable
end-to-end. The log is not the graph (it is the control-plane record), but its
verified outcomes become facts.

## 8. Untrusted content and prompt injection

Ingested documents, messages, tickets, and web content are **data, never
instructions** (`SECURITY.md`, `knowledge-model.md` invariant 3).

- Instructions embedded in retrieved content must not be obeyed by the agent.
- An LLM can never: grant itself or another principal a permission, raise the
  autonomy level, broaden an approval's scope, or bypass the AUTHZ gate.
- Action parameters derived from untrusted content are validated against the
  approved scope; content cannot expand what is allowed.
- The AUTHZ gate and policy checks are deterministic and live **outside** the
  model's influence.

## 9. What must be true before the first write action ships

- AUTHZ gate implemented and tested against allow/deny/approval fixtures.
- Idempotency proven: duplicate delivery executes once.
- Approval artifact recorded and bound to the exact action.
- Verification path records a VERIFIED OUTCOME.
- Audit log complete and append-only.
- A rollback or compensating action defined for the chosen action class.
- Security review signed off (`SECURITY.md`).
- Evaluation coverage for action authorization and duplicate execution
  (`EVALUATION.md`).

Until all hold, TRON stays at L0/L1.
