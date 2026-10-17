package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

// TestGroundedProgressGuard is the deterministic expected-outcome suite for the
// shared loop's progress guard. It drives GroundedAsk with a scripted Decide
// (no model) against a real seeded graph, so every outcome is reproducible:
// a repeated retrieval is detected, the guard advances along observed edges or
// broadens the resolved entity, a stuck loop abstains instead of exhausting the
// step budget, and an author question reaches an AUTHORED edge.
func TestGroundedProgressGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_progress_guard_test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer c.Delete()
	if _, err := seed.Load(c); err != nil {
		t.Fatal(err)
	}

	claim := func(f graph.EvidenceFact) Decision {
		raw, _ := json.Marshal(map[string]interface{}{"claims": []ModelClaim{{Text: "x", Subject: f.Entity, Relation: f.Relation, EvidenceIDs: []string{f.ID}}}})
		return Decision{Answer: string(raw)}
	}
	// findFact returns the first already-returned fact of a relation - the same
	// evidence a real model would answer from.
	findFact := func(turns []Turn, relation string) (graph.EvidenceFact, bool) {
		for _, tn := range turns {
			ec, ok := tn.Result.(graph.EvidenceContext)
			if !ok {
				continue
			}
			for _, f := range ec.Facts {
				if f.Relation == relation {
					return f, true
				}
			}
		}
		return graph.EvidenceFact{}, false
	}
	narrow := func() Decision {
		raw, _ := json.Marshal(graph.EvidenceRequest{Entity: "service:payments-api", Relations: []string{"OWNS"}})
		return Decision{Calls: []Call{{Name: "assemble_context", Args: raw}}}
	}

	t.Run("repeated narrow retrieval is recovered, not exhausted", func(t *testing.T) {
		decide := func(ctx context.Context, system string, turns []Turn) (Decision, error) {
			if f, ok := findFact(turns, "HAS_REPO"); ok {
				return claim(f), nil
			}
			return narrow(), nil
		}
		r, err := GroundedAsk(context.Background(), c, GroundedRequest{Question: "Which repository belongs to payments-api?", Target: "service:payments-api"}, decide)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "answered" {
			t.Fatalf("status=%s warnings=%v repeated=%d recoveries=%d", r.Status, r.Warnings, r.RepeatedRetrievals, r.GuardRecoveries)
		}
		if r.RepeatedRetrievals < 1 || r.GuardRecoveries != 1 {
			t.Fatalf("repeated=%d recoveries=%d", r.RepeatedRetrievals, r.GuardRecoveries)
		}
		if !strings.Contains(strings.ToLower(r.Grounded.Answer.Answer), "has repo") {
			t.Fatal(r.Grounded.Answer.Answer)
		}
	})

	t.Run("no-progress loop abstains instead of exhausting the budget", func(t *testing.T) {
		decide := func(ctx context.Context, system string, turns []Turn) (Decision, error) { return narrow(), nil }
		r, err := GroundedAsk(context.Background(), c, GroundedRequest{Question: "Which repository belongs to payments-api?", Target: "service:payments-api"}, decide)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "abstained" || len(r.Grounded.Answer.Claims) != 0 {
			t.Fatalf("status=%s claims=%d", r.Status, len(r.Grounded.Answer.Claims))
		}
		if r.RepeatedRetrievals < 1 {
			t.Fatalf("repeat not counted: %+v", r)
		}
		warned := false
		for _, w := range r.Warnings {
			warned = warned || strings.Contains(w, "no progress")
		}
		if !warned {
			t.Fatalf("warnings=%v", r.Warnings)
		}
	})

	t.Run("repeat is detected by scope, not JSON field order", func(t *testing.T) {
		decide := func(ctx context.Context, system string, turns []Turn) (Decision, error) {
			if len(turns) == 0 {
				return Decision{Calls: []Call{{Name: "assemble_context", Args: json.RawMessage(`{"entity":"service:payments-api","relations":["OWNS"]}`)}}}, nil
			}
			return Decision{Calls: []Call{{Name: "assemble_context", Args: json.RawMessage(`{"relations":["OWNS"],"entity":"service:payments-api"}`)}}}, nil
		}
		r, err := GroundedAsk(context.Background(), c, GroundedRequest{Question: "Which repository belongs to payments-api?", Target: "service:payments-api"}, decide)
		if err != nil {
			t.Fatal(err)
		}
		if r.RepeatedRetrievals < 1 {
			t.Fatalf("field-order repeat not counted: %+v", r)
		}
		owns := 0
		for _, ec := range r.Contexts {
			if len(ec.Relations) == 1 && ec.Relations[0] == "OWNS" {
				owns++
			}
		}
		if owns != 1 {
			t.Fatalf("duplicate OWNS context appended: %d contexts=%d", owns, len(r.Contexts))
		}
	})

	t.Run("author question traverses SUPPORTED_BY to an AUTHORED edge", func(t *testing.T) {
		decide := func(ctx context.Context, system string, turns []Turn) (Decision, error) {
			if len(turns) == 0 {
				raw, _ := json.Marshal(graph.EvidenceRequest{Entity: "decision:3", Relations: []string{"SUPPORTED_BY"}})
				return Decision{Calls: []Call{{Name: "assemble_context", Args: raw}}}, nil
			}
			// Answer from the SUPPORTED_BY link, exactly as the built-in model
			// did; the guard must still reach the documented authors.
			if f, ok := findFact(turns, "SUPPORTED_BY"); ok {
				return claim(f), nil
			}
			return Decision{Answer: `{"claims":[]}`}, nil
		}
		r, err := GroundedAsk(context.Background(), c, GroundedRequest{Question: "Which documents support the Redis idempotency decision, and who authored them?", Target: "decision:3"}, decide)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "answered" {
			t.Fatalf("status=%s warnings=%v contexts=%d", r.Status, r.Warnings, len(r.Contexts))
		}
		if err := r.Grounded.Validate(); err != nil {
			t.Fatal(err)
		}
		rels := map[string]bool{}
		for _, ec := range r.Contexts {
			for _, f := range ec.Facts {
				rels[f.Relation] = true
			}
		}
		if !rels["SUPPORTED_BY"] || !rels["AUTHORED"] {
			t.Fatalf("relations=%v", rels)
		}
	})
}
