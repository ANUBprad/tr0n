package knowledge

import "fmt"

// Claim layers (docs/knowledge-model.md). No silent promotion between
// them — a claim states which layer it is.
const (
	LayerFact           = "FACT"
	LayerInference      = "INFERENCE"
	LayerRecommendation = "RECOMMENDATION"
)

// Claim is one tagged, evidenced statement. Paths are graph fact paths
// ("(:Person p:1)-[:OWNS]->(:Service s:payments-api)") or, for absence
// claims, the operation reference that proves the emptiness
// ("FindOwner(payments-api) → 0 results"). Inferences additionally
// carry their rule_id.
//
// ponytail: path strings for non-TraceEvidence ops are composed from
// the operation's own results (key + relation); upgrade path = have
// each op return its traversal paths directly.
type Claim struct {
	Text   string   `json:"text"`
	Layer  string   `json:"layer"`
	Paths  []string `json:"paths"`
	RuleID string   `json:"rule_id,omitempty"`
}

// Answer is the response envelope for every Q&A surface (UI, agent,
// API). Per AGENT_SPEC an untagged, unevidenced claim must not be
// renderable — enforced here, by Validate, not by reviewer vigilance.
type Answer struct {
	Question       string           `json:"question"`
	Answer         string           `json:"answer"`
	Layer          string           `json:"layer"`
	Claims         []Claim          `json:"claims"`
	OperationsUsed []string         `json:"operations_used"`
	LatencyMS      map[string]int64 `json:"latency_ms,omitempty"`
}

// Validate refuses anything that would render an untagged or
// unevidenced claim.
func (a Answer) Validate() error {
	if a.Question == "" {
		return fmt.Errorf("question is empty")
	}
	if a.Answer == "" {
		return fmt.Errorf("answer text is empty")
	}
	switch a.Layer {
	case LayerFact, LayerInference, LayerRecommendation:
	default:
		return fmt.Errorf("answer layer %q is not FACT|INFERENCE|RECOMMENDATION", a.Layer)
	}
	for i, c := range a.Claims {
		if c.Text == "" {
			return fmt.Errorf("claim %d: empty text", i)
		}
		switch c.Layer {
		case LayerFact, LayerRecommendation:
			if c.RuleID != "" {
				return fmt.Errorf("claim %d: rule_id is for INFERENCE claims only", i)
			}
		case LayerInference:
			if c.RuleID == "" {
				return fmt.Errorf("claim %d: INFERENCE claim needs rule_id", i)
			}
		default:
			return fmt.Errorf("claim %d: layer %q is not a knowledge layer", i, c.Layer)
		}
		if len(c.Paths) == 0 {
			return fmt.Errorf("claim %d (%s): no evidence paths", i, c.Layer)
		}
	}
	return nil
}
