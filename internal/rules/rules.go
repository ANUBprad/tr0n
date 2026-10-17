// Package rules provides deterministic decision support, option comparison,
// and proactive risk detection over the organizational knowledge graph.
// All scores are reproducible pure functions; no hidden weights or stochastic outputs.
package rules

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/knowledge"
)

// Option represents a candidate under decision evaluation.
type Option struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	CostScore   float64 `json:"cost_score"` // 0.0 (cheapest) to 1.0 (most expensive)
	RiskScore   float64 `json:"risk_score"` // 0.0 (safest) to 1.0 (highest risk)
	PerfScore   float64 `json:"perf_score"` // 0.0 (worst) to 1.0 (best throughput/latency)
	EaseScore   float64 `json:"ease_score"` // 0.0 (hardest) to 1.0 (easiest migration)
}

// CriteriaWeights specifies normalized preferences for decision trade-offs.
// Sum should ideally normalize to 1.0; auto-normalized if not.
type CriteriaWeights struct {
	CostWeight float64 `json:"cost_weight"`
	RiskWeight float64 `json:"risk_weight"`
	PerfWeight float64 `json:"perf_weight"`
	EaseWeight float64 `json:"ease_weight"`
}

// ScoredOption holds the deterministic evaluation for an option.
type ScoredOption struct {
	Option          Option  `json:"option"`
	CompositeScore  float64 `json:"composite_score"` // higher is better
	ParetoEfficient bool    `json:"pareto_efficient"`
	Rank            int     `json:"rank"`
	Rationale       string  `json:"rationale"`
}

// PlanResult is the complete reproducible decision-support evaluation.
type PlanResult struct {
	DecisionContext string          `json:"decision_context"`
	RuleID          string          `json:"rule_id"`
	Weights         CriteriaWeights `json:"weights"`
	RankedOptions   []ScoredOption  `json:"ranked_options"`
	ParetoFrontier  []string        `json:"pareto_frontier"` // IDs of non-dominated options
	RecommendedID   string          `json:"recommended_id"`
	SensitivityNote string          `json:"sensitivity_note"`
	InputNote       string          `json:"input_note"`
	Layer           string          `json:"layer"` // knowledge.LayerInference
	Timestamp       string          `json:"timestamp"`
}

// DefaultWeights provides balanced criteria for architectural/operational choices.
func DefaultWeights() CriteriaWeights {
	return CriteriaWeights{CostWeight: 0.25, RiskWeight: 0.35, PerfWeight: 0.25, EaseWeight: 0.15}
}

// EvaluatePlan validates and ranks caller-scored options. The returned
// recommendation is a calculation over supplied scores, not graph evidence.
func EvaluatePlan(context string, options []Option, w CriteriaWeights) (PlanResult, error) {
	if err := ValidatePlan(context, options, w); err != nil {
		return PlanResult{}, err
	}
	if w == (CriteriaWeights{}) {
		w = DefaultWeights()
	}
	nw := normalizeWeights(w)

	scored := make([]ScoredOption, len(options))
	for i, opt := range options {
		// Benefits: PerfScore, EaseScore
		// Costs/Penalties: CostScore, RiskScore -> inverted (1 - penalty) for composite utility
		costUtility := 1.0 - opt.CostScore
		riskUtility := 1.0 - opt.RiskScore
		perfUtility := opt.PerfScore
		easeUtility := opt.EaseScore

		score := nw.CostWeight*costUtility +
			nw.RiskWeight*riskUtility +
			nw.PerfWeight*perfUtility +
			nw.EaseWeight*easeUtility

		scored[i] = ScoredOption{
			Option:         opt,
			CompositeScore: math.Round(score*1000) / 1000,
			Rationale: fmt.Sprintf("cost_util=%.2f, risk_util=%.2f, perf_util=%.2f, ease_util=%.2f",
				costUtility, riskUtility, perfUtility, easeUtility),
		}
	}

	// Determine Pareto efficiency: an option is Pareto efficient if no other option
	// has greater-or-equal utility across all 4 dimensions with strictly greater in at least one.
	paretoIDs := []string{}
	for i := range scored {
		isDominated := false
		for j := range scored {
			if i == j {
				continue
			}
			optA := scored[i].Option
			optB := scored[j].Option
			// B dominates A if B is <= cost & risk and >= perf & ease, with at least one strict
			bBetterOrEqual := optB.CostScore <= optA.CostScore &&
				optB.RiskScore <= optA.RiskScore &&
				optB.PerfScore >= optA.PerfScore &&
				optB.EaseScore >= optA.EaseScore
			bStrictlyBetter := optB.CostScore < optA.CostScore ||
				optB.RiskScore < optA.RiskScore ||
				optB.PerfScore > optA.PerfScore ||
				optB.EaseScore > optA.EaseScore

			if bBetterOrEqual && bStrictlyBetter {
				isDominated = true
				break
			}
		}
		if !isDominated {
			scored[i].ParetoEfficient = true
			paretoIDs = append(paretoIDs, scored[i].Option.ID)
		}
	}
	sort.Strings(paretoIDs)

	// Sort by composite score descending
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].CompositeScore == scored[j].CompositeScore {
			return scored[i].Option.ID < scored[j].Option.ID
		}
		return scored[i].CompositeScore > scored[j].CompositeScore
	})

	for r := range scored {
		scored[r].Rank = r + 1
	}

	bestID := scored[0].Option.ID

	sensitivity := sensitivityReport(options, nw, bestID)

	return PlanResult{
		DecisionContext: context,
		RuleID:          "decision_eval/v1",
		Weights:         nw,
		RankedOptions:   scored,
		ParetoFrontier:  paretoIDs,
		RecommendedID:   bestID,
		SensitivityNote: sensitivity,
		InputNote:       "Options and criterion scores were supplied by the caller; TRON did not verify them against graph sources.",
		Layer:           knowledge.LayerInference,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func ValidatePlan(context string, options []Option, w CriteriaWeights) error {
	if strings.TrimSpace(context) == "" || len(context) > 300 {
		return fmt.Errorf("context must contain 1–300 characters")
	}
	if len(options) == 0 || len(options) > 100 {
		return fmt.Errorf("options must contain 1–100 candidates")
	}
	weights := []float64{w.CostWeight, w.RiskWeight, w.PerfWeight, w.EaseWeight}
	total := 0.0
	for _, weight := range weights {
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
			return fmt.Errorf("weights must be finite and non-negative")
		}
		total += weight
	}
	if math.IsInf(total, 0) || math.IsNaN(total) {
		return fmt.Errorf("total criterion weight must be finite")
	}
	if total <= 0 && w != (CriteriaWeights{}) {
		return fmt.Errorf("at least one criterion weight must be positive")
	}
	ids := make(map[string]bool, len(options))
	for i, option := range options {
		if strings.TrimSpace(option.ID) == "" || len(option.ID) > 128 || strings.TrimSpace(option.Name) == "" || len(option.Name) > 200 {
			return fmt.Errorf("option %d requires an id (max 128) and name (max 200)", i+1)
		}
		if ids[option.ID] {
			return fmt.Errorf("duplicate option id %q", option.ID)
		}
		ids[option.ID] = true
		for _, score := range []float64{option.CostScore, option.RiskScore, option.PerfScore, option.EaseScore} {
			if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 1 {
				return fmt.Errorf("option %q scores must be finite values between 0 and 1", option.ID)
			}
		}
	}
	return nil
}

func normalizeWeights(w CriteriaWeights) CriteriaWeights {
	total := w.CostWeight + w.RiskWeight + w.PerfWeight + w.EaseWeight
	return CriteriaWeights{CostWeight: w.CostWeight / total, RiskWeight: w.RiskWeight / total, PerfWeight: w.PerfWeight / total, EaseWeight: w.EaseWeight / total}
}

func sensitivityReport(options []Option, weights CriteriaWeights, baseline string) string {
	names := []string{"cost", "risk", "performance", "ease"}
	values := []float64{weights.CostWeight, weights.RiskWeight, weights.PerfWeight, weights.EaseWeight}
	changed := []string{}
	for i, name := range names {
		for _, factor := range []float64{0.9, 1.1} {
			perturbed := values
			perturbed[i] *= factor
			w := normalizeWeights(CriteriaWeights{CostWeight: perturbed[0], RiskWeight: perturbed[1], PerfWeight: perturbed[2], EaseWeight: perturbed[3]})
			if winner(options, w) != baseline {
				changed = append(changed, name+" weight "+fmt.Sprintf("%.0f%%", factor*100))
			}
		}
	}
	if len(changed) == 0 {
		return "Recommendation remains " + baseline + " under independent ±10% perturbations of each criterion weight."
	}
	return "Recommendation changes under these independent ±10% weight perturbations: " + strings.Join(changed, ", ") + "."
}

func winner(options []Option, w CriteriaWeights) string {
	bestID, bestScore := "", math.Inf(-1)
	for _, option := range options {
		score := w.CostWeight*(1-option.CostScore) + w.RiskWeight*(1-option.RiskScore) + w.PerfWeight*option.PerfScore + w.EaseWeight*option.EaseScore
		if score > bestScore || (score == bestScore && option.ID < bestID) {
			bestID, bestScore = option.ID, score
		}
	}
	return bestID
}

// Proactive Alerts (Milestone 6)

// Alert represents an evidence-backed organizational risk or trigger.
type Alert struct {
	DedupKey    string   `json:"dedup_key"`
	RuleID      string   `json:"rule_id"`
	Severity    string   `json:"severity"` // "critical", "warning", "info"
	Target      string   `json:"target"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Evidence    []string `json:"evidence"`
	Timestamp   string   `json:"timestamp"`
}

// DetectAlerts inspects the live graph and generates deduplicated, evidence-backed alerts.
func DetectAlerts(c *graph.Client) ([]Alert, error) {
	if c == nil {
		return nil, fmt.Errorf("graph client is required")
	}
	alerts := []Alert{}
	now := time.Now().UTC()

	// 1. Detect ownership conflicts
	confRes, err := c.FindConflicts("OWNS", "")
	if err != nil {
		return nil, fmt.Errorf("detect ownership conflicts: %w", err)
	}
	if confRes != nil && len(confRes.Conflicts) > 0 {
		for _, conf := range confRes.Conflicts {
			evidence := []string{}
			for _, edge := range conf.Edges {
				evidence = append(evidence, fmt.Sprintf("%s -> %s (src: %s, valid: %s..%s)",
					edge.From, edge.To, edge.Provenance.SrcRef, edge.Provenance.ValidFrom, edge.Provenance.ValidTo))
			}
			alerts = append(alerts, Alert{
				DedupKey:    fmt.Sprintf("conflict:OWNS:%s", conf.Slot),
				RuleID:      "alert/conflict/v1",
				Severity:    "critical",
				Target:      conf.Slot,
				Title:       fmt.Sprintf("Conflicting ownership claims on %s", conf.Slot),
				Description: fmt.Sprintf("Multiple concurrent active owners recorded for %s with overlapping validity.", conf.Slot),
				Evidence:    evidence,
				Timestamp:   now.Format(time.RFC3339),
			})
		}
	}

	// 2. Check catalog services for SPOF (Single Point of Failure in expertise)
	cat, err := c.Catalog()
	if err != nil {
		return nil, fmt.Errorf("detect service risks: %w", err)
	}
	for _, item := range cat {
		if item.Label == "Service" {
			expRes, err := c.FindExperts(item.Key, 10, now, now)
			if err != nil {
				return nil, fmt.Errorf("inspect service %s: %w", item.Key, err)
			}
			if expRes != nil {
				if len(expRes.Experts) == 0 {
					alerts = append(alerts, Alert{
						DedupKey:    fmt.Sprintf("unowned:%s", item.Key),
						RuleID:      "alert/unowned/v1",
						Severity:    "critical",
						Target:      item.Key,
						Title:       fmt.Sprintf("Service %s has no known experts or recorded owner", item.Name),
						Description: "No experts or owners could be resolved for this service in the graph.",
						Evidence:    []string{item.Key},
						Timestamp:   now.Format(time.RFC3339),
					})
				} else if len(expRes.Experts) == 1 {
					alerts = append(alerts, Alert{
						DedupKey:    fmt.Sprintf("spof:%s", item.Key),
						RuleID:      "alert/spof/v1",
						Severity:    "warning",
						Target:      item.Key,
						Title:       fmt.Sprintf("Single point of failure on %s", item.Name),
						Description: fmt.Sprintf("Service %s has only one qualified expert (%s). Knowledge loss risk if unavailable.", item.Name, expRes.Experts[0].Key),
						Evidence:    []string{fmt.Sprintf("expert:%s score=%.2f", expRes.Experts[0].Key, expRes.Experts[0].Score)},
						Timestamp:   now.Format(time.RFC3339),
					})
				}
			}
		}
	}

	// Sort alerts by severity (critical first) then Target
	sevWeight := map[string]int{"critical": 1, "warning": 2, "info": 3}
	sort.SliceStable(alerts, func(i, j int) bool {
		if sevWeight[alerts[i].Severity] != sevWeight[alerts[j].Severity] {
			return sevWeight[alerts[i].Severity] < sevWeight[alerts[j].Severity]
		}
		return alerts[i].Target < alerts[j].Target
	})

	return alerts, nil
}
