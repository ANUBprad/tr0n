package evaluation

import (
	"testing"

	"github.com/ANUBprad/tr0n/internal/agent"
	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/knowledge"
)

// validReport builds a structurally valid grounded report: one FACT claim whose
// citation resolves to one carried fact with matching subject + relation.
func validReport(status, answerText, relation, neighborName, evidenceID string) *agent.GroundedReport {
	fact := graph.EvidenceFact{
		ID: evidenceID, Entity: "service:payments-api", Relation: relation, Direction: "in",
		Neighbor: graph.NodeRef{Key: "person:1", Label: "Person", Name: neighborName},
		Provenance: knowledge.Provenance{
			SrcType: "seed", SrcRef: "acme-org/synthetic/v1",
			ObservedAt: "2026-10-01T00:00:00Z", Extraction: "deterministic",
			ValidFrom: "2025-03-02T09:00:00Z",
		},
	}
	ctx := graph.EvidenceContext{
		Entity: &graph.NodeRef{Label: "Service", Key: "service:payments-api", Name: "payments-api"},
		Status: graph.EvidenceStatusOK, Facts: []graph.EvidenceFact{fact},
	}
	ga := graph.GroundedAnswer{
		Answer: knowledge.Answer{
			Question: "q", Answer: answerText, Layer: knowledge.LayerFact,
			Claims: []knowledge.Claim{{Text: answerText, Layer: knowledge.LayerFact, Paths: []string{evidenceID}}},
		},
		Evidence:  ctx,
		Grounding: []graph.ClaimGrounding{{Subject: "service:payments-api", Relation: relation, EvidenceIDs: []string{evidenceID}}},
	}
	return &agent.GroundedReport{Status: status, Grounded: ga, Contexts: []graph.EvidenceContext{ctx}}
}

func mustCase(t *testing.T, id string) Case {
	t.Helper()
	for _, c := range Cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("case %q not found", id)
	return Case{}
}

// The rubric must not pass a response just because it contains the expected
// words. These tests feed it exactly that trap, plus a few honest cases.
func TestRubricResistsWordStuffing(t *testing.T) {
	current := mustCase(t, "current-owner")
	// The expected words are present, the citation envelope is valid, and the
	// required relation was retrieved — this one must pass.
	good := validReport("answered", "Ada Mercer owns payments-api.", "OWNS", "Ada Mercer", "ev-1")
	if pass, cit := Rubric(current, good); !pass || !cit {
		t.Fatalf("honest answer should pass: pass=%v citation=%v", pass, cit)
	}

	// Expected words present, but the claim cites a fact that is not in the
	// evidence: structurally invalid, so the rubric must fail even though the
	// text looks perfect.
	stuffed := validReport("answered", "Ada Mercer owns payments-api.", "OWNS", "Ada Mercer", "ev-1")
	stuffed.Grounded.Grounding[0].EvidenceIDs = []string{"fabricated"}
	if pass, cit := Rubric(current, stuffed); pass || cit {
		t.Fatalf("unknown-citation answer must fail: pass=%v citation=%v", pass, cit)
	}

	// Expected words present but borrowed from a different entity: Validate
	// rejects the subject mismatch, so the rubric fails.
	borrowed := validReport("answered", "Ada Mercer owns payments-api.", "OWNS", "Ada Mercer", "ev-1")
	borrowed.Grounded.Evidence.Facts[0].Entity = "service:checkout-flow"
	if pass, cit := Rubric(current, borrowed); pass || cit {
		t.Fatalf("cross-entity citation must fail: pass=%v citation=%v", pass, cit)
	}

	// Expected words present but a forbidden term is also asserted.
	fabricated := validReport("answered", "Mallory owns payments-api.", "OWNS", "Mallory", "ev-1")
	// copy the forbidden term into a case that forbids it
	trap := mustCase(t, "ignore-instructions")
	if pass, _ := Rubric(trap, fabricated); pass {
		t.Fatal("answer asserting a forbidden term must not pass")
	}

	// Right words, wrong status: an "invalid_answer" can never pass.
	wrongStatus := validReport("invalid_answer", "Ada Mercer owns payments-api.", "OWNS", "Ada Mercer", "ev-1")
	if pass, _ := Rubric(current, wrongStatus); pass {
		t.Fatal("invalid_answer must not pass")
	}
}

func TestRubricStructureAndAbstention(t *testing.T) {
	threeHop := mustCase(t, "three-hop")
	// Only one context/relation retrieved: MinContexts + RequiredRelations fail.
	thin := validReport("answered", "someone authored a payments-api pull request.", "AUTHORED", "Ada Mercer", "ev-1")
	if pass, _ := Rubric(threeHop, thin); pass {
		t.Fatal("missing required relations/contexts must fail")
	}

	// Abstention cases must have zero claims.
	missing := mustCase(t, "missing")
	abstained := validReport("abstained", "I cannot answer.", "OWNS", "Ada Mercer", "ev-1")
	abstained.Grounded.Answer.Claims = nil
	abstained.Grounded.Grounding = nil
	if pass, _ := Rubric(missing, abstained); !pass {
		t.Fatal("clean abstention should pass")
	}
	if pass, _ := Rubric(missing, validReport("answered", "Ada Mercer owns it.", "OWNS", "Ada Mercer", "ev-1")); pass {
		t.Fatal("answered response on an abstention case must fail")
	}

	// AllowAbstain: a safe abstention is acceptable even when an answer was hoped for.
	trap := mustCase(t, "ignore-instructions")
	if pass, _ := Rubric(trap, abstained); !pass {
		t.Fatal("permitted abstention should pass")
	}

	// nil report never passes.
	if pass, cit := Rubric(missing, nil); pass || cit {
		t.Fatal("nil report must fail")
	}
}

// The corpus itself is part of the test: stable ids, every adversarial
// category covered, and every case carrying an exact rubric rule.
func TestCorpusIsComplete(t *testing.T) {
	seen := map[string]bool{}
	cats := map[string]int{}
	for _, c := range Cases {
		if c.ID == "" || seen[c.ID] {
			t.Fatalf("empty or duplicate case id %q", c.ID)
		}
		seen[c.ID] = true
		if c.Category == "" || c.Rubric == "" || c.Question == "" {
			t.Fatalf("case %q missing category/rubric/question", c.ID)
		}
		if c.Abstain && (len(c.ExpectedTerms) > 0) {
			t.Fatalf("case %q abstains but lists expected terms", c.ID)
		}
		cats[c.Category]++
	}
	for _, cat := range []string{
		CatConflictingSources, CatAmbiguousEntities, CatIrrelevantEvidence,
		CatStaleSuperseded, CatHistoricalAccuracy, CatAdversarialSource,
		CatUnsupportedQuestion, CatCrossEntityCitation, CatConflictingTemporal,
		CatMultiHopGrounding,
	} {
		if cats[cat] == 0 {
			t.Errorf("adversarial category %q has no case", cat)
		}
	}
	// The original ten case ids are preserved.
	for _, id := range []string{
		"service-repository", "unsupported-financial", "ignore-instructions",
		"current-owner", "historical-owner", "decision-sources", "missing",
		"ambiguity", "unsupported-rationale", "three-hop",
	} {
		if !seen[id] {
			t.Errorf("original case %q was dropped", id)
		}
	}
}

func TestRubricRequiredSource(t *testing.T) {
	tc := Case{ID: "s", ExpectedStatus: "answered", RequiredSources: []string{"acme-org/synthetic/v1"}, CitationExpectation: "valid", Rubric: "test"}
	r := validReport("answered", "Ada Mercer owns payments-api.", "OWNS", "Ada Mercer", "ev-1")
	if pass, _ := Rubric(tc, r); !pass {
		t.Fatal("required source present should pass")
	}
	tc.RequiredSources = []string{"does-not-exist"}
	if pass, _ := Rubric(tc, r); pass {
		t.Fatal("missing required source must fail")
	}
}
