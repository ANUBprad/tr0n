package knowledge

import (
	"strings"
	"testing"
)

// The AGENT_SPEC answer invariant: untagged, unevidenced claims must
// not be renderable. One runnable check for Validate.
func TestAnswerValidate(t *testing.T) {
	valid := Answer{
		Question: "Who owns payments-api?",
		Answer:   "Ada owns payments-api.",
		Layer:    LayerFact,
		Claims: []Claim{
			{Text: "Ada owns payments-api.", Layer: LayerFact,
				Paths: []string{"(:Person person:1)-[:OWNS]->(:Service service:payments-api)"}},
			{Text: "Ada ranks #1 on expertise/v1.", Layer: LayerInference,
				Paths: []string{"basis path"}, RuleID: "expertise/v1"},
			{Text: "Route to Ada.", Layer: LayerRecommendation,
				Paths: []string{"owner path", "expert path"}},
		},
		OperationsUsed: []string{"FindOwner", "FindExperts"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid answer rejected: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*Answer)
		wantErr string
	}{
		{"empty answer", func(a *Answer) { a.Answer = "" }, "answer text is empty"},
		{"empty question", func(a *Answer) { a.Question = "" }, "question is empty"},
		{"bad answer layer", func(a *Answer) { a.Layer = "OPINION" }, "is not FACT|INFERENCE|RECOMMENDATION"},
		{"claim without paths", func(a *Answer) { a.Claims[0].Paths = nil }, "no evidence paths"},
		{"claim with unknown layer", func(a *Answer) { a.Claims[0].Layer = "vibes" }, "is not a knowledge layer"},
		{"inference without rule_id", func(a *Answer) {
			a.Claims[1].RuleID = ""
		}, "needs rule_id"},
		{"rule_id on FACT claim", func(a *Answer) {
			a.Claims[0].RuleID = "expertise/v1"
		}, "INFERENCE claims only"},
		{"empty claim text", func(a *Answer) { a.Claims[0].Text = "" }, "empty text"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutated := valid
			mutated.Claims = append([]Claim(nil), valid.Claims...)
			tc.mutate(&mutated)
			err := mutated.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}

	// An answer with no claims is a summary, not a violation — but a
	// claimless layer tag still must be valid.
	noClaims := Answer{Question: "q", Answer: "insufficient data", Layer: LayerFact}
	if err := noClaims.Validate(); err != nil {
		t.Errorf("claimless answer: %v", err)
	}
}
