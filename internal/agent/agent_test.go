package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

// scripted hands out fixed model replies in order; exhausting the
// script surfaces as a decide error (so tests fail loudly).
func scripted(replies ...Decision) Decide {
	i := 0
	return func(ctx context.Context, system string, turns []Turn) (Decision, error) {
		if i >= len(replies) {
			return Decision{}, fmt.Errorf("script exhausted after %d decides", i)
		}
		d := replies[i]
		i++
		return d, nil
	}
}

func echoTools() map[string]Tool {
	return map[string]Tool{
		"echo": {
			Description: "echo the args back",
			Params:      `{"type":"object"}`,
			Exec: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				return map[string]interface{}{"echoed": string(args)}, nil
			},
		},
		"boom": {
			Description: "always fails",
			Params:      `{"type":"object"}`,
			Exec: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				return nil, fmt.Errorf("boom")
			},
		},
	}
}

// The one runnable check for the loop: model calls a tool, result is
// fed back, model answers — transcript and operations_used recorded.
func TestLoopExecutesAndAnswers(t *testing.T) {
	s := New(DefaultSystem, echoTools())

	var seen []int
	next := scripted(
		Decision{Calls: []Call{{Name: "echo", Args: json.RawMessage(`{"q":"who owns x"}`)}}},
		Decision{Answer: "answer with evidence"},
	)
	wrapped := func(ctx context.Context, system string, turns []Turn) (Decision, error) {
		seen = append(seen, len(turns))
		return next(ctx, system, turns)
	}
	if err := s.Run(context.Background(), wrapped); err != nil {
		t.Fatal(err)
	}

	if s.Answer() != "answer with evidence" {
		t.Errorf("answer: %q", s.Answer())
	}
	if ops := s.OperationsUsed(); len(ops) != 1 || ops[0] != "echo" {
		t.Errorf("operations_used: %v", ops)
	}
	turns := s.Turns()
	if len(turns) != 1 || turns[0].Err != "" {
		t.Fatalf("turns: %+v", turns)
	}
	if got, _ := json.Marshal(turns[0].Result); !strings.Contains(string(got), "who owns x") {
		t.Errorf("result not recorded: %s", got)
	}
	// Second decide must see the executed turn (results feed back).
	if len(seen) != 2 || seen[0] != 0 || seen[1] != 1 {
		t.Errorf("transcript growth across decides: %v", seen)
	}
}

// Tool failures and unknown tools become turn errors the model can
// react to — they must not crash the session.
func TestLoopSurfacesToolErrorsToModel(t *testing.T) {
	s := New(DefaultSystem, echoTools())
	err := s.Run(context.Background(), scripted(
		Decision{Calls: []Call{
			{Name: "boom"},
			{Name: "no_such_tool", Args: json.RawMessage(`{}`)},
		}},
		Decision{Answer: "recovered"},
	))
	if err != nil {
		t.Fatal(err)
	}
	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("want 2 turns, got %+v", turns)
	}
	if turns[0].Err != "boom" || turns[1].Err != `unknown tool "no_such_tool"` {
		t.Errorf("errors fed back: %q / %q", turns[0].Err, turns[1].Err)
	}
	if s.Answer() != "recovered" {
		t.Errorf("answer: %q", s.Answer())
	}
}

// Deliberate retrieval is bounded: a model that never answers hits the
// step budget, and a non-decision reply is an error, not a hang.
func TestLoopStepBudgetAndInvalidMoves(t *testing.T) {
	t.Run("budget exceeded", func(t *testing.T) {
		s := New(DefaultSystem, echoTools())
		always := Decision{Calls: []Call{{Name: "echo"}}}
		err := s.Run(context.Background(), scripted(
			always, always, always, always, always, always, always, always,
		))
		if err == nil || !strings.Contains(err.Error(), "step budget") {
			t.Errorf("want step budget error, got %v", err)
		}
		if len(s.Turns()) != s.MaxSteps {
			t.Errorf("turns %d, want %d", len(s.Turns()), s.MaxSteps)
		}
	})

	t.Run("neither answer nor calls", func(t *testing.T) {
		s := New(DefaultSystem, echoTools())
		err := s.Run(context.Background(), scripted(Decision{}))
		if err == nil || !strings.Contains(err.Error(), "neither answer nor tool calls") {
			t.Errorf("want invalid-move error, got %v", err)
		}
	})

	t.Run("decide error propagates", func(t *testing.T) {
		s := New(DefaultSystem, echoTools())
		err := s.Run(context.Background(), func(ctx context.Context, system string, turns []Turn) (Decision, error) {
			return Decision{}, fmt.Errorf("provider down")
		})
		if err == nil || !strings.Contains(err.Error(), "provider down") {
			t.Errorf("want provider error, got %v", err)
		}
	})
}

// End-to-end against the seeded graph: the scripted model drives the
// six real tools, results carry provenance back through the transcript.
func TestToolsAgainstSeededGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	addr := getenv("TRON_FALKOR_ADDR", "localhost:6379")
	c, err := graph.New(addr, "tron_agent_test")
	if err != nil {
		t.Fatalf("falkordb unreachable at %s — run `docker compose up -d`: %v", addr, err)
	}
	t.Cleanup(func() { c.Delete(); c.Close() })
	if _, err := seed.Load(c); err != nil {
		t.Fatalf("seed load: %v", err)
	}

	tools := Tools(c)
	if len(tools) != 6 {
		t.Fatalf("want 6 tools, got %d", len(tools))
	}

	s := New(DefaultSystem, tools)
	err = s.Run(context.Background(), scripted(
		Decision{Calls: []Call{{Name: "find_owner", Args: json.RawMessage(`{"target":"payments-api"}`)}}},
		Decision{Calls: []Call{{Name: "find_experts", Args: json.RawMessage(
			`{"service_key":"service:payments-api","limit":3,"now":"2026-10-08T00:00:00Z"}`)}}},
		Decision{Calls: []Call{{Name: "trace_decision", Args: json.RawMessage(
			`{"ref":"Adopt FalkorDB for the org knowledge graph"}`)}}},
		Decision{Answer: "person:1 owns payments-api and is its top expert."},
	))
	if err != nil {
		t.Fatal(err)
	}
	if ops := s.OperationsUsed(); len(ops) != 3 ||
		ops[0] != "find_owner" || ops[1] != "find_experts" || ops[2] != "trace_decision" {
		t.Fatalf("operations_used: %v", ops)
	}

	turns := s.Turns()
	for i, tn := range turns {
		if tn.Err != "" {
			t.Fatalf("turn %d (%s) errored: %s", i, tn.Call.Name, tn.Err)
		}
		blob, _ := json.Marshal(tn.Result)
		switch i {
		case 0:
			if !strings.Contains(string(blob), "person:1") || !strings.Contains(string(blob), "src_type") {
				t.Errorf("find_owner must return the owner with provenance: %s", blob)
			}
		case 1:
			if !strings.Contains(string(blob), "expertise/v1") || !strings.Contains(string(blob), "INFERENCE") {
				t.Errorf("find_experts must return tagged scores: %s", blob)
			}
		}
	}

	t.Run("invalid tool args become turn errors", func(t *testing.T) {
		s := New(DefaultSystem, tools)
		if err := s.Run(context.Background(), scripted(
			Decision{Calls: []Call{{Name: "find_experts", Args: json.RawMessage(`{"limit":5}`)}}},
			Decision{Calls: []Call{{Name: "trace_decision", Args: json.RawMessage(`{}`)}}},
			Decision{Answer: "cannot be answered with those args"},
		)); err != nil {
			t.Fatal(err)
		}
		turns := s.Turns()
		if !strings.Contains(turns[0].Err, "service_key is required") ||
			!strings.Contains(turns[1].Err, "ref is required") {
			t.Errorf("missing required args must error: %q / %q", turns[0].Err, turns[1].Err)
		}
		if s.Answer() == "" {
			t.Error("model must still be able to answer after tool errors")
		}
	})
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
