package actions

import (
	"strings"
	"testing"
)

// TestE9ActionAuthorization tests the AUTHZ gate across allow, require_approval, and deny tiers (E9).
func TestE9ActionAuthorization(t *testing.T) {
	eng := NewEngine()

	// 1. L2 Draft action: should be ALLOWed without approval record
	params := map[string]interface{}{"title": "Handoff review draft", "recipient": "person:1"}
	prop, authz, err := eng.Propose("person:14", "draft_handoff", "service:payments-api", params)
	if err != nil {
		t.Fatalf("expected proposal to succeed: %v", err)
	}
	if authz.Decision != "allow" {
		t.Errorf("expected decision=allow for draft_handoff, got %s", authz.Decision)
	}

	// Execution of allowed draft action should succeed immediately
	res, err := eng.Execute("person:14", prop.ID, "idemp-draft-1", params)
	if err != nil {
		t.Fatalf("expected execution of allowed action to succeed: %v", err)
	}
	if res.Status != StatusSucceeded {
		t.Errorf("expected status=succeeded, got %s", res.Status)
	}
	if res.VerifiedFact != "" || !strings.Contains(res.Outcome, "no external or graph state changed") {
		t.Errorf("draft must not claim a verified side effect: %+v", res)
	}

	// 2. L3 State modifying action: should REQUIRE_APPROVAL
	ownerParams := map[string]interface{}{"new_owner": "person:1", "reason": "scheduled reorg"}
	_, authzOwner, err := eng.Propose("person:14", "record_owner_handoff", "service:payments-api", ownerParams)
	if err == nil || authzOwner.Decision != "deny" {
		t.Errorf("state-changing action must remain denied without an execution adapter: decision=%s err=%v", authzOwner.Decision, err)
	}

	// 3. L5 High-Impact governance action: must be DENIED
	_, authzHigh, err := eng.Propose("person:14", "delete_repository", "repo:1", map[string]interface{}{"force": true})
	if err == nil {
		t.Errorf("expected high-impact action to be rejected")
	}
	if authzHigh.Decision != "deny" {
		t.Errorf("expected decision=deny for delete_repository, got %s", authzHigh.Decision)
	}

	// 4. Prompt injection attempt: parameter claiming role escalation must be DENIED
	_, authzInj, err := eng.Propose("person:14", "draft_handoff", "service:payments-api", map[string]interface{}{"role": "admin"})
	if err == nil {
		t.Errorf("expected injection parameter to be rejected")
	}
	if authzInj.Decision != "deny" {
		t.Errorf("expected decision=deny on prompt injection parameter, got %s", authzInj.Decision)
	}
}

// TestE10DuplicateExecutionAndIdempotency tests that duplicate deliveries execute exactly once (E10).
func TestE10DuplicateExecutionAndIdempotency(t *testing.T) {
	eng := NewEngine()

	params := map[string]interface{}{"body": "Incident notification", "channel": "#payments"}
	prop, _, err := eng.Propose("person:2", "draft_ticket", "incident:1", params)
	if err != nil {
		t.Fatalf("propose failed: %v", err)
	}

	idempKey := "dup-key-abc-123"

	// First execution: should succeed and record execution
	res1, err := eng.Execute("person:2", prop.ID, idempKey, params)
	if err != nil {
		t.Fatalf("execution 1 failed: %v", err)
	}
	if res1.Replay {
		t.Errorf("first execution should not be marked as replay")
	}

	// Replay / Duplicate delivery with same idempotency key: must return cached result and NOT double-execute
	res2, err := eng.Execute("person:2", prop.ID, idempKey, params)
	if err != nil {
		t.Fatalf("duplicate execution failed: %v", err)
	}
	if !res2.Replay {
		t.Errorf("second execution must be marked as replay")
	}
	if res1.ExecutionID != res2.ExecutionID {
		t.Errorf("expected matching execution IDs, got %s vs %s", res1.ExecutionID, res2.ExecutionID)
	}

	// Parameter tampering test: modifying parameter after proposal must be rejected
	tamperedParams := map[string]interface{}{"body": "Tampered body", "channel": "#payments"}
	propTamper, _, _ := eng.Propose("person:2", "draft_ticket", "incident:2", params)
	_, err = eng.Execute("person:2", propTamper.ID, "idemp-tamper-1", tamperedParams)
	if err == nil || !strings.Contains(err.Error(), "parameter mismatch") {
		t.Errorf("expected parameter tampering to be detected and rejected, got %v", err)
	}
	if _, err = eng.Execute("person:2", propTamper.ID, idempKey, params); err == nil || !strings.Contains(err.Error(), "different action") {
		t.Errorf("reusing an idempotency key for another action must fail, got %v", err)
	}

	// Verify append-only tamper-evident audit log
	entries := eng.AuditEntries()
	if len(entries) < 3 {
		t.Fatalf("expected audit log entries, got %d", len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].PrevHash != entries[i-1].Hash {
			t.Errorf("audit log hash chain broken at index %d", i)
		}
	}
}
