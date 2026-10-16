package graph

// E3 source-grounded answer contract (docs/EVALUATION.md §2, docs/REASONING.md
// §3): a FACT claim must cite evidence that actually exists in the retrieval
// result it claims to be grounded in, and the explicit non-ok outcomes can never
// be laundered into a supported factual claim. The pure cases run without
// FalkorDB; the retrieval cases run against a real one.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/knowledge"
)

// groundedFact is one retrieved fact with its stable ID derived the same
// way AssembleEvidence derives it.
func groundedFact() EvidenceFact {
	f := EvidenceFact{
		Entity:    "service:payments",
		Relation:  "OWNS",
		Direction: "in",
		Neighbor:  NodeRef{Label: "Person", Key: "person:ada", Name: "Ada"},
		Provenance: knowledge.Provenance{
			SrcType: "service_registry", SrcRef: "acme-org/synthetic/v1",
			ObservedAt: "2026-10-01T00:00:00Z", Extraction: "deterministic",
			ValidFrom: "2026-02-01T00:00:00Z",
		},
	}
	f.ID = evidenceFactID(f)
	return f
}

func groundedContext(status string, facts ...EvidenceFact) EvidenceContext {
	c := EvidenceContext{
		Entity: &NodeRef{Label: "Service", Key: "service:payments", Name: "payments-api"},
		Status: status,
		Facts:  facts,
		Limit:  DefaultEvidenceLimit,
	}
	if status != EvidenceStatusOK {
		c.Entity = nil
	}
	return c
}

func groundedAnswerBase() knowledge.Answer {
	return knowledge.Answer{
		Question: "Who owns payments-api?",
		Answer:   "person:ada owns payments-api.",
		Layer:    knowledge.LayerFact,
		Claims: []knowledge.Claim{{
			Text:  "person:ada owns payments-api",
			Layer: knowledge.LayerFact,
			Paths: []string{"(:Person person:ada)-[:OWNS]->(:Service service:payments)"},
		}},
		OperationsUsed: []string{"assemble_context"},
	}
}

func TestGroundedAnswerValidate(t *testing.T) {
	fact := groundedFact()
	good := ClaimGrounding{Subject: "service:payments", Relation: "OWNS", EvidenceIDs: []string{fact.ID}}

	build := func(gr ClaimGrounding) GroundedAnswer {
		return GroundedAnswer{
			Answer:    groundedAnswerBase(),
			Evidence:  groundedContext(EvidenceStatusOK, fact),
			Grounding: []ClaimGrounding{gr},
		}
	}

	t.Run("valid claim citing real evidence", func(t *testing.T) {
		if err := build(good).Validate(); err != nil {
			t.Fatalf("a claim citing a carried fact must validate: %v", err)
		}
	})

	t.Run("fabricated evidence id is rejected", func(t *testing.T) {
		gr := good
		gr.EvidenceIDs = []string{"deadbeefdeadbeef"}
		if err := build(gr).Validate(); err == nil || !strings.Contains(err.Error(), "unknown evidence") {
			t.Errorf("a fabricated id must be rejected: %v", err)
		}
	})

	t.Run("evidence of a different relationship is rejected", func(t *testing.T) {
		gr := good
		gr.Relation = "AUTHORED" // the carried fact is an OWNS fact
		if err := build(gr).Validate(); err == nil || !strings.Contains(err.Error(), "not AUTHORED") {
			t.Errorf("cross-relation evidence must be rejected: %v", err)
		}
	})

	t.Run("evidence anchored to another entity is rejected", func(t *testing.T) {
		gr := good
		gr.Subject = "service:billing" // the carried fact is anchored to service:payments
		if err := build(gr).Validate(); err == nil || !strings.Contains(err.Error(), "not \"service:billing\"") {
			t.Errorf("cross-entity evidence must be rejected: %v", err)
		}
	})

	t.Run("a FACT claim citing no evidence is rejected", func(t *testing.T) {
		gr := good
		gr.EvidenceIDs = nil
		if err := build(gr).Validate(); err == nil || !strings.Contains(err.Error(), "cites no evidence") {
			t.Errorf("an unevidenced FACT claim must be rejected: %v", err)
		}
	})

	t.Run("a FACT claim with no subject is rejected", func(t *testing.T) {
		gr := good
		gr.Subject = ""
		if err := build(gr).Validate(); err == nil || !strings.Contains(err.Error(), "no subject") {
			t.Errorf("a subjectless FACT claim must be rejected: %v", err)
		}
	})

	t.Run("grounding and claims must align", func(t *testing.T) {
		g := build(good)
		g.Grounding = nil
		if err := g.Validate(); err == nil || !strings.Contains(err.Error(), "grounding has 0 entries") {
			t.Errorf("missing grounding must be rejected: %v", err)
		}
	})

	t.Run("a non-ok context cannot back a FACT claim", func(t *testing.T) {
		g := build(good)
		g.Evidence = groundedContext(EvidenceStatusEmpty)
		if err := g.Validate(); err == nil || !strings.Contains(err.Error(), "empty") {
			t.Errorf("empty evidence must not back a FACT claim: %v", err)
		}
	})

	t.Run("an INFERENCE claim is not forced to be fact-grounded", func(t *testing.T) {
		// Non-FACT claims keep their own rule_id/path discipline; only FACT
		// claims are grounded by carried facts.
		g := GroundedAnswer{
			Answer: knowledge.Answer{
				Question: "Who knows payments-api best?",
				Answer:   "person:ada ranks #1.",
				Layer:    knowledge.LayerInference,
				Claims: []knowledge.Claim{{
					Text:   "person:ada ranks #1 on expertise/v1",
					Layer:  knowledge.LayerInference,
					Paths:  []string{"(:Person person:ada)-[:RESOLVED]->(:Incident incident:1)"},
					RuleID: "expertise/v1",
				}},
			},
			Evidence:  groundedContext(EvidenceStatusEmpty),
			Grounding: []ClaimGrounding{{}},
		}
		if err := g.Validate(); err != nil {
			t.Fatalf("an INFERENCE claim must not be forced to be fact-grounded: %v", err)
		}
	})

	t.Run("association survives a JSON round trip", func(t *testing.T) {
		before := build(good)
		blob, err := json.Marshal(before)
		if err != nil {
			t.Fatal(err)
		}
		var after GroundedAnswer
		if err := json.Unmarshal(blob, &after); err != nil {
			t.Fatal(err)
		}
		if len(after.Evidence.Facts) != 1 || after.Evidence.Facts[0].ID != fact.ID {
			t.Fatalf("evidence id lost in serialization: %s", blob)
		}
		if !strings.Contains(string(blob), `"valid_from":"2026-02-01T00:00:00Z"`) {
			t.Errorf("structured provenance dropped: %s", blob)
		}
		if err := after.Validate(); err != nil {
			t.Errorf("a valid grounded answer must still validate after a round trip: %v", err)
		}
	})
}

// Against a real FalkorDB: an answer built from an actual retrieval result
// validates, preserves the fact's structured provenance, and is deterministic.
func TestGroundedAnswerFromRetrieval(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	c := newTestClient(t)
	evidenceFixture(t, c)

	ctx, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api", Relations: []string{"OWNS"}})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Status != EvidenceStatusOK || len(ctx.Facts) == 0 {
		t.Fatalf("expected an ok context with facts: %+v", ctx)
	}
	for _, f := range ctx.Facts {
		if f.ID == "" {
			t.Fatalf("a retrieved fact must carry an id to cite: %+v", f)
		}
	}

	// Same fact, same id, every run — the citation is stable, not positional.
	again, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api", Relations: []string{"OWNS"}})
	if err != nil {
		t.Fatal(err)
	}
	for i := range ctx.Facts {
		if ctx.Facts[i].ID != again.Facts[i].ID {
			t.Errorf("evidence id is not deterministic: %q vs %q", ctx.Facts[i].ID, again.Facts[i].ID)
		}
	}

	f := ctx.Facts[0]
	ga := GroundedAnswer{
		Answer: knowledge.Answer{
			Question: "Who owns payments-api?",
			Answer:   "A current OWNS edge exists.",
			Layer:    knowledge.LayerFact,
			Claims: []knowledge.Claim{{
				Text:  "payments-api has a current owner.",
				Layer: knowledge.LayerFact,
				Paths: []string{"(:Person " + f.Neighbor.Key + ")-[:OWNS]->(:Service service:payments)"},
			}},
			OperationsUsed: []string{"assemble_context"},
		},
		Evidence:  ctx,
		Grounding: []ClaimGrounding{{Subject: "service:payments", Relation: "OWNS", EvidenceIDs: []string{f.ID}}},
	}
	if err := ga.Validate(); err != nil {
		t.Fatalf("an answer citing retrieved evidence must validate: %v", err)
	}

	blob, err := json.Marshal(ga)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"src_type"`, `"valid_from"`, `"id":"` + f.ID + `"`} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("grounded answer missing %s: %s", want, blob)
		}
	}
}

// The explicit non-ok outcomes stay distinguishable and can never back a FACT
// claim.
func TestGroundedAnswerExplicitOutcomes(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	c := newTestClient(t)
	evidenceFixture(t, c)

	cases := []struct {
		name string
		req  EvidenceRequest
		want string
	}{
		{"not_found", EvidenceRequest{Entity: "definitely-not-a-service"}, EvidenceStatusNotFound},
		{"ambiguous", EvidenceRequest{Entity: "mail-api"}, EvidenceStatusAmbiguous},
		{"empty", EvidenceRequest{Entity: "payments-api", Relations: []string{"WORKED_ON"}}, EvidenceStatusEmpty},
		{"over_budget", EvidenceRequest{Entity: "payments-api", MaxBytes: 8}, EvidenceStatusOverBudget},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := c.AssembleEvidence(tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if ctx.Status != tc.want {
				t.Fatalf("status %q, want %q: %+v", ctx.Status, tc.want, ctx)
			}
			ga := GroundedAnswer{
				Answer: knowledge.Answer{
					Question: "Who owns payments-api?",
					Answer:   "person:1 owns payments-api.",
					Layer:    knowledge.LayerFact,
					Claims: []knowledge.Claim{{
						Text:  "person:1 owns payments-api",
						Layer: knowledge.LayerFact,
						Paths: []string{"(:Person person:1)-[:OWNS]->(:Service service:payments)"},
					}},
				},
				Evidence:  ctx,
				Grounding: []ClaimGrounding{{Subject: "service:payments", Relation: "OWNS", EvidenceIDs: []string{"0000000000000000"}}},
			}
			if err := ga.Validate(); err == nil {
				t.Errorf("%s must not back a FACT claim: %+v", tc.want, ctx)
			}
		})
	}
}

// A truncated-but-ok context still validates (it was partial and says so), and
// the truncation is carried verbatim, never presented as exhaustive.
func TestGroundedAnswerPreservesTruncation(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	c := newTestClient(t)
	evidenceFixture(t, c)

	ctx, err := c.AssembleEvidence(EvidenceRequest{Entity: "payments-api", Relations: []string{"OWNS"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Status != EvidenceStatusOK || !ctx.Truncated || ctx.TruncatedReason != EvidenceTruncatedByCount || len(ctx.Facts) != 1 {
		t.Fatalf("expected a truncated-by-count ok context: %+v", ctx)
	}

	f := ctx.Facts[0]
	ga := GroundedAnswer{
		Answer: knowledge.Answer{
			Question: "Who owns payments-api?",
			Answer:   "One OWNS edge was retrieved.",
			Layer:    knowledge.LayerFact,
			Claims: []knowledge.Claim{{
				Text:  "payments-api has an OWNS edge.",
				Layer: knowledge.LayerFact,
				Paths: []string{"(:Person " + f.Neighbor.Key + ")-[:OWNS]->(:Service service:payments)"},
			}},
		},
		Evidence:  ctx,
		Grounding: []ClaimGrounding{{Subject: "service:payments", Relation: "OWNS", EvidenceIDs: []string{f.ID}}},
	}
	if err := ga.Validate(); err != nil {
		t.Fatalf("a truncated-but-ok context may still ground a claim: %v", err)
	}
	if !ga.Evidence.Truncated || ga.Evidence.TruncatedReason != EvidenceTruncatedByCount {
		t.Errorf("truncation must be carried verbatim, not dropped: %+v", ga.Evidence)
	}
}
