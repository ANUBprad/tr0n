// Package actions implements a deterministic gate and process-local draft
// proposal/replay records. It does not execute external or graph mutations.
package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// AutonomyLevel defines the risk tier of an action.
type AutonomyLevel int

const (
	LevelObserve       AutonomyLevel = 0 // L0: Read only
	LevelRecommend     AutonomyLevel = 1 // L1: Recommendations with trade-offs
	LevelDraft         AutonomyLevel = 2 // L2: Artifact drafts for human to send
	LevelHumanApproved AutonomyLevel = 3 // L3: Execution requiring explicit approval record
	LevelPreAuthorized AutonomyLevel = 4 // L4: Scoped, reversible pre-authorized execution
	LevelHighImpact    AutonomyLevel = 5 // L5: High impact (multi-party governance required)
)

func (l AutonomyLevel) String() string {
	switch l {
	case LevelObserve:
		return "L0:Observe"
	case LevelRecommend:
		return "L1:Recommend"
	case LevelDraft:
		return "L2:Draft"
	case LevelHumanApproved:
		return "L3:HumanApproved"
	case LevelPreAuthorized:
		return "L4:PreAuthorized"
	case LevelHighImpact:
		return "L5:HighImpact"
	default:
		return "Unknown"
	}
}

// AuthzDecision is the deterministic outcome of the AUTHZ gate.
type AuthzDecision struct {
	Decision   string        `json:"decision"` // "allow", "require_approval", "deny"
	Reason     string        `json:"reason"`
	Level      AutonomyLevel `json:"level"`
	ActionType string        `json:"action_type"`
	Target     string        `json:"target"`
}

// ProposedAction describes an action proposed for execution.
type ProposedAction struct {
	ID         string                 `json:"id"`
	Principal  string                 `json:"principal"`
	ActionType string                 `json:"action_type"`
	Target     string                 `json:"target"`
	Parameters map[string]interface{} `json:"parameters"`
	CreatedAt  string                 `json:"created_at"`
	ParamHash  string                 `json:"param_hash"`
}

// ApprovalRecord binds an authorized human approval to an exact ProposedAction.
type ApprovalRecord struct {
	ApprovalID string `json:"approval_id"`
	ActionID   string `json:"action_id"`
	ApprovedBy string `json:"approved_by"`
	ApprovedAt string `json:"approved_at"`
	ExpiresAt  string `json:"expires_at"`
	ParamHash  string `json:"param_hash"` // changing any parameter invalidates the approval
}

// ExecutionStatus models the execution lifecycle.
type ExecutionStatus string

const (
	StatusAuthorized ExecutionStatus = "authorized"
	StatusExecuting  ExecutionStatus = "executing"
	StatusSucceeded  ExecutionStatus = "succeeded"
	StatusFailed     ExecutionStatus = "failed"
	StatusRolledBack ExecutionStatus = "rolled_back"
)

// ExecutionResult captures the local result of a draft operation.
type ExecutionResult struct {
	ExecutionID    string          `json:"execution_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	ActionID       string          `json:"action_id"`
	Status         ExecutionStatus `json:"status"`
	Outcome        string          `json:"outcome"`
	VerifiedFact   string          `json:"verified_fact,omitempty"`
	FailureClass   string          `json:"failure_class,omitempty"` // "retryable", "terminal", or ""
	ExecutedAt     string          `json:"executed_at"`
	Replay         bool            `json:"replay,omitempty"`
}

// AuditEntry is an append-only entry in the tamper-evident action audit log.
type AuditEntry struct {
	Sequence  int    `json:"sequence"`
	Timestamp string `json:"timestamp"`
	Principal string `json:"principal"`
	Event     string `json:"event"` // "proposal", "authorization", "approval", "execution"
	ActionID  string `json:"action_id"`
	Details   string `json:"details"`
	PrevHash  string `json:"prev_hash"`
	Hash      string `json:"hash"`
}

// Engine coordinates action authorization, approvals, execution, and audit logging.
type Engine struct {
	mu           sync.Mutex
	proposals    map[string]ProposedAction
	approvals    map[string]ApprovalRecord
	executions   map[string]ExecutionResult // keyed by IdempotencyKey
	executionKey map[string]string
	auditLog     []AuditEntry
	lastHash     string
}

// NewEngine creates an in-memory draft action engine.
func NewEngine() *Engine {
	return &Engine{
		proposals:    make(map[string]ProposedAction),
		approvals:    make(map[string]ApprovalRecord),
		executions:   make(map[string]ExecutionResult),
		executionKey: make(map[string]string),
		auditLog:     make([]AuditEntry, 0),
		lastHash:     "0000000000000000000000000000000000000000000000000000000000000000",
	}
}

// ComputeParamHash generates a canonical SHA-256 hash of action parameters.
func ComputeParamHash(params map[string]interface{}) string {
	b, err := json.Marshal(params)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Authorize runs the deterministic AUTHZ policy gate (ACTIONS.md §3).
func (e *Engine) Authorize(principal, actionType, target string, params map[string]interface{}) AuthzDecision {
	if principal == "" || actionType == "" || target == "" {
		return AuthzDecision{Decision: "deny", Reason: "missing principal, action_type, or target"}
	}

	// Invariant: Prompt injection resistance - untrusted inputs cannot escalate privileges.
	for k, v := range params {
		s := fmt.Sprintf("%v", v)
		if k == "role" || k == "privilege" || k == "escalate" {
			return AuthzDecision{Decision: "deny", Reason: "untrusted parameter cannot specify role/privilege"}
		}
		if len(s) > 4096 {
			return AuthzDecision{Decision: "deny", Reason: "parameter payload exceeds size limit"}
		}
	}

	switch actionType {
	case "draft_handoff", "draft_ticket":
		// L2: Draft actions produce artifacts for human review; allowed directly.
		return AuthzDecision{
			Decision:   "allow",
			Reason:     "L2 draft actions do not produce external mutations and are pre-authorized",
			Level:      LevelDraft,
			ActionType: actionType,
			Target:     target,
		}

	case "delete_repository", "transfer_budget", "grant_admin":
		// L5: High impact governance actions - denied for automated execution.
		return AuthzDecision{
			Decision:   "deny",
			Reason:     "L5 high-impact action forbidden under automated agent execution",
			Level:      LevelHighImpact,
			ActionType: actionType,
			Target:     target,
		}

	default:
		return AuthzDecision{
			Decision:   "deny",
			Reason:     fmt.Sprintf("unknown or unclassified action type %q", actionType),
			Level:      LevelHighImpact,
			ActionType: actionType,
			Target:     target,
		}
	}
}

// Propose records a proposed action and logs it in the audit trail.
func (e *Engine) Propose(principal, actionType, target string, params map[string]interface{}) (ProposedAction, AuthzDecision, error) {
	decision := e.Authorize(principal, actionType, target, params)
	if decision.Decision == "deny" {
		return ProposedAction{}, decision, fmt.Errorf("action denied: %s", decision.Reason)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	paramHash := ComputeParamHash(params)
	if paramHash == "" {
		return ProposedAction{}, AuthzDecision{Decision: "deny", Reason: "parameters must be valid JSON"}, fmt.Errorf("parameters must be valid JSON")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	idRaw, _ := json.Marshal(struct {
		Principal, ActionType, Target, ParamHash string
	}{principal, actionType, target, paramHash})
	h := sha256.Sum256(idRaw)
	actionID := "act:" + hex.EncodeToString(h[:])
	paramsCopy, err := copyParams(params)
	if err != nil {
		return ProposedAction{}, AuthzDecision{Decision: "deny", Reason: "parameters must be valid JSON"}, err
	}

	prop := ProposedAction{
		ID:         actionID,
		Principal:  principal,
		ActionType: actionType,
		Target:     target,
		Parameters: paramsCopy,
		CreatedAt:  now,
		ParamHash:  paramHash,
	}
	e.proposals[actionID] = prop
	e.appendAudit(principal, "proposal", actionID, fmt.Sprintf("action=%s target=%s decision=%s", actionType, target, decision.Decision))

	return prop, decision, nil
}

// Approve records an approver and the proposal's parameter hash in memory.
func (e *Engine) Approve(actionID, approvedBy string, expiryDuration time.Duration) (ApprovalRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	prop, ok := e.proposals[actionID]
	if !ok {
		return ApprovalRecord{}, errors.New("proposed action not found")
	}
	if approvedBy == "" {
		return ApprovalRecord{}, errors.New("approved_by identity is required")
	}

	now := time.Now().UTC()
	if expiryDuration <= 0 {
		expiryDuration = 24 * time.Hour
	}

	approvalID := fmt.Sprintf("appr:%s:%d", actionID, now.Unix())
	appr := ApprovalRecord{
		ApprovalID: approvalID,
		ActionID:   actionID,
		ApprovedBy: approvedBy,
		ApprovedAt: now.Format(time.RFC3339),
		ExpiresAt:  now.Add(expiryDuration).Format(time.RFC3339),
		ParamHash:  prop.ParamHash,
	}
	e.approvals[actionID] = appr
	e.appendAudit(approvedBy, "approval", actionID, fmt.Sprintf("approval_id=%s expires=%s", approvalID, appr.ExpiresAt))

	return appr, nil
}

// Execute performs an action under idempotency and outcome verification (ACTIONS.md §5 & §6).
func (e *Engine) Execute(actor, actionID, idempotencyKey string, currentParams map[string]interface{}) (ExecutionResult, error) {
	if actor == "" || actionID == "" || idempotencyKey == "" || len(idempotencyKey) > 128 {
		return ExecutionResult{}, errors.New("authenticated executor, action_id, and idempotency_key (max 128) are required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	prop, ok := e.proposals[actionID]
	if !ok {
		return ExecutionResult{}, errors.New("action proposal not found")
	}

	// Parameter tamper check: current parameters must exactly match approved parameters
	currentHash := ComputeParamHash(currentParams)
	if currentHash == "" {
		return ExecutionResult{}, errors.New("parameters must be valid JSON")
	}
	if currentHash != prop.ParamHash {
		return ExecutionResult{}, errors.New("parameter mismatch: action parameters changed after proposal")
	}
	fingerprint := actor + "\x00" + actionID + "\x00" + prop.ParamHash
	if prior, exists := e.executions[idempotencyKey]; exists {
		if e.executionKey[idempotencyKey] != fingerprint {
			return ExecutionResult{}, errors.New("idempotency key was already used for a different action")
		}
		prior.Replay = true
		return prior, nil
	}

	// AUTHZ policy check
	authz := e.Authorize(prop.Principal, prop.ActionType, prop.Target, prop.Parameters)
	if authz.Decision == "require_approval" {
		appr, ok := e.approvals[actionID]
		if !ok {
			return ExecutionResult{}, errors.New("action requires human approval before execution")
		}
		// Verify expiry
		expT, err := time.Parse(time.RFC3339, appr.ExpiresAt)
		if err != nil || time.Now().UTC().After(expT) {
			return ExecutionResult{}, errors.New("approval record has expired")
		}
		if appr.ParamHash != prop.ParamHash {
			return ExecutionResult{}, errors.New("approval signature does not match proposal parameters")
		}
	} else if authz.Decision != "allow" {
		return ExecutionResult{}, fmt.Errorf("action is not authorized: %s", authz.Reason)
	}

	// Only local draft generation is implemented. State-changing adapters must
	// not report success until a target system confirms the effect.
	if prop.ActionType != "draft_handoff" && prop.ActionType != "draft_ticket" {
		return ExecutionResult{}, errors.New("state-changing execution adapter is not configured")
	}
	nowStr := time.Now().UTC().Format(time.RFC3339)
	var outcome string

	switch prop.ActionType {
	case "draft_handoff", "draft_ticket":
		outcome = fmt.Sprintf("Local draft generated for %s (target: %s); no external or graph state changed", prop.Principal, prop.Target)
	}

	execHash := sha256.Sum256([]byte(actionID + "\x00" + idempotencyKey))
	res := ExecutionResult{
		ExecutionID:    "exec:" + hex.EncodeToString(execHash[:]),
		IdempotencyKey: idempotencyKey,
		ActionID:       actionID,
		Status:         StatusSucceeded,
		Outcome:        outcome,
		ExecutedAt:     nowStr,
	}

	e.executions[idempotencyKey] = res
	e.executionKey[idempotencyKey] = fingerprint
	e.appendAudit(actor, "execution", actionID, fmt.Sprintf("status=succeeded outcome=%q", outcome))

	return res, nil
}

// AuditEntries returns a copy of the append-only audit trail.
func (e *Engine) AuditEntries() []AuditEntry {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]AuditEntry, len(e.auditLog))
	copy(out, e.auditLog)
	return out
}

func (e *Engine) appendAudit(principal, event, actionID, details string) {
	seq := len(e.auditLog) + 1
	now := time.Now().UTC().Format(time.RFC3339)
	raw := fmt.Sprintf("%d:%s:%s:%s:%s:%s:%s", seq, now, principal, event, actionID, details, e.lastHash)
	h := sha256.Sum256([]byte(raw))
	hashStr := hex.EncodeToString(h[:])

	entry := AuditEntry{
		Sequence:  seq,
		Timestamp: now,
		Principal: principal,
		Event:     event,
		ActionID:  actionID,
		Details:   details,
		PrevHash:  e.lastHash,
		Hash:      hashStr,
	}
	e.auditLog = append(e.auditLog, entry)
	e.lastHash = hashStr
}

func copyParams(params map[string]interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var copy map[string]interface{}
	if err := json.Unmarshal(b, &copy); err != nil {
		return nil, err
	}
	return copy, nil
}
