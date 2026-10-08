// Package agent is the deliberate-retrieval loop: a model picks
// tools, the loop executes them against the graph, results go back to
// the model until it answers. Custom minimal loop per OPEN_QUESTIONS
// — no framework, and no provider binding: the model side is one
// function (`Decide`) so the LLM provider stays an open decision
// (docs/AGENT_SPEC.md, docs/architecture-minimal.md).
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
)

// Tool is one callable graph operation exposed to the model.
type Tool struct {
	Description string
	Params      string // JSON schema for the provider's tool definition
	Exec        func(ctx context.Context, args json.RawMessage) (interface{}, error)
}

// Call is the model's request to run one tool.
type Call struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// Turn records one executed call with its result (or error) — the
// growing transcript fed back to the model each round.
type Turn struct {
	Call   Call        `json:"call"`
	Result interface{} `json:"result,omitempty"`
	Err    string      `json:"error,omitempty"`
}

// Decision is one model move: an answer, or tool calls to run.
// A provider adapter converts its native response into this shape.
type Decision struct {
	Answer string `json:"answer,omitempty"`
	Calls  []Call `json:"calls,omitempty"`
}

// Decide is the entire model seam: given the system prompt and the
// transcript so far, return the next move. One function — an
// OpenAI/Anthropic/Ollama adapter is a wrapper around it.
type Decide func(ctx context.Context, system string, turns []Turn) (Decision, error)

// Session runs one conversation within a step budget (deliberate
// retrieval: bounded tool use, no runaway loops).
type Session struct {
	System   string
	Tools    map[string]Tool
	MaxSteps int

	turns  []Turn
	answer string
}

// New builds a session with the default step budget.
func New(system string, tools map[string]Tool) *Session {
	return &Session{System: system, Tools: tools, MaxSteps: 8}
}

// Turns is the executed transcript, in order.
func (s *Session) Turns() []Turn { return s.turns }

// Answer is set once the model stops calling tools.
func (s *Session) Answer() string { return s.answer }

// OperationsUsed lists the tool names executed, in order — the
// operations_used field of the answer envelope (AGENT_SPEC).
func (s *Session) OperationsUsed() []string {
	ops := make([]string, 0, len(s.turns))
	for _, t := range s.turns {
		ops = append(ops, t.Call.Name)
	}
	return ops
}

// Run drives the loop until the model answers or the budget runs out.
// Tool failures are not fatal: they are recorded and fed back so the
// model can correct course — only decide errors and budget exhaustion
// stop the session.
func (s *Session) Run(ctx context.Context, decide Decide) error {
	for step := 0; step < s.MaxSteps; step++ {
		d, err := decide(ctx, s.System, s.turns)
		if err != nil {
			return fmt.Errorf("model decide (step %d): %w", step, err)
		}
		if d.Answer != "" {
			s.answer = d.Answer
			return nil
		}
		if len(d.Calls) == 0 {
			return fmt.Errorf("model returned neither answer nor tool calls (step %d)", step)
		}
		for _, call := range d.Calls {
			s.turns = append(s.turns, s.exec(ctx, call))
		}
	}
	return fmt.Errorf("step budget (%d) exceeded without an answer", s.MaxSteps)
}

func (s *Session) exec(ctx context.Context, call Call) Turn {
	tool, ok := s.Tools[call.Name]
	if !ok {
		return Turn{Call: call, Err: fmt.Sprintf("unknown tool %q", call.Name)}
	}
	res, err := tool.Exec(ctx, call.Args)
	if err != nil {
		return Turn{Call: call, Err: err.Error()}
	}
	return Turn{Call: call, Result: res}
}

// DefaultSystem is the v1 prompt: deliberate retrieval, evidence-first
// answers, no invented facts.
const DefaultSystem = `You are TRON, an assistant answering questions about an
organization from its knowledge graph. Rules:
- Use tools to retrieve facts; never invent people, services, or events.
- Call tools deliberately: only what the question needs, then answer.
- Answer briefly and cite which operations you used and why they suffice.`

// Tools exposes the six graph operations to the model, bound to a
// client. Args mirror the JSON API contracts (internal/api).
func Tools(c *graph.Client) map[string]Tool {
	str := func(raw json.RawMessage, key string) string {
		var m map[string]interface{}
		if json.Unmarshal(raw, &m) != nil {
			return ""
		}
		s, _ := m[key].(string)
		return s
	}
	return map[string]Tool{
		"find_owner": {
			Description: "Who owns this entity (by name or key)? Returns owner nodes with the OWNS edge provenance.",
			Params:      `{"type":"object","required":["target"],"properties":{"target":{"type":"string"}}}`,
			Exec: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				if target := str(args, "target"); target != "" {
					return c.FindOwner(target)
				}
				return nil, fmt.Errorf("target is required")
			},
		},
		"find_experts": {
			Description: "Who knows this service best? Ranked people with deterministic expertise/v1 scores and evidence paths.",
			Params:      `{"type":"object","required":["service_key"],"properties":{"service_key":{"type":"string"},"limit":{"type":"integer"},"now":{"type":"string","description":"RFC3339 clock injection"}}}`,
			Exec: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				key := str(args, "service_key")
				if key == "" {
					return nil, fmt.Errorf("service_key is required")
				}
				var req struct {
					Limit int    `json:"limit"`
					Now   string `json:"now"`
				}
				json.Unmarshal(args, &req)
				now := time.Now().UTC()
				if req.Now != "" {
					t, err := time.Parse(time.RFC3339, req.Now)
					if err != nil {
						return nil, fmt.Errorf("now must be RFC3339: %w", err)
					}
					now = t
				}
				return c.FindExperts(key, req.Limit, now)
			},
		},
		"trace_decision": {
			Description: "Why was this decision made? Decision with supporting documents, meetings, authors, and supersedes chain.",
			Params:      `{"type":"object","required":["ref"],"properties":{"ref":{"type":"string","description":"decision key or title"}}}`,
			Exec: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				if ref := str(args, "ref"); ref != "" {
					trace, err := c.TraceDecision(ref)
					if err != nil {
						return nil, err
					}
					if trace == nil {
						return nil, fmt.Errorf("decision %q not found", ref)
					}
					return trace, nil
				}
				return nil, fmt.Errorf("ref is required")
			},
		},
		"find_related_incidents": {
			Description: "Has this happened before? Incidents affecting a service, newest first, with resolvers. Optional RFC3339 since and literal case-insensitive title keyword.",
			Params:      `{"type":"object","required":["service_key"],"properties":{"service_key":{"type":"string"},"since":{"type":"string","description":"RFC3339 lower bound on started_at"},"keywords":{"type":"string"}}}`,
			Exec: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				key := str(args, "service_key")
				if key == "" {
					return nil, fmt.Errorf("service_key is required")
				}
				return c.FindRelatedIncidents(key, str(args, "since"), str(args, "keywords"))
			},
		},
		"trace_evidence": {
			Description: "What supports this answer? Re-fetch keys with full properties and one hop of surrounding context, every edge with provenance.",
			Params:      `{"type":"object","required":["keys"],"properties":{"keys":{"type":"array","items":{"type":"string"}}}}`,
			Exec: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				var req struct {
					Keys []string `json:"keys"`
				}
				json.Unmarshal(args, &req)
				if len(req.Keys) == 0 {
					return nil, fmt.Errorf("keys must be a non-empty array")
				}
				return c.TraceEvidence(req.Keys)
			},
		},
		"resolve_entity": {
			Description: "Resolve a free-text name to a graph key (exact key, exact name, case-insensitive, prefix). Returns the key or a candidate list - never a guess.",
			Params:      `{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"label":{"type":"string"}}}`,
			Exec: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				if name := str(args, "name"); name != "" {
					return c.ResolveEntity(name, str(args, "label"))
				}
				return nil, fmt.Errorf("name is required")
			},
		},
	}
}
