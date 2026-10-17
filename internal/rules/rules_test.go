package rules

import (
	"math"
	"os"
	"testing"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

func TestEvaluatePlan(t *testing.T) {
	// Scenario: Architectural choice for idempotency store
	options := []Option{
		{
			ID:          "opt:redis",
			Name:        "Redis",
			Description: "In-memory key-value store with sub-millisecond latency",
			CostScore:   0.2, // low cost
			RiskScore:   0.2, // low risk
			PerfScore:   0.9, // high perf
			EaseScore:   0.8, // easy to integrate
		},
		{
			ID:          "opt:kafka",
			Name:        "Kafka Log-Compacted Topic",
			Description: "Distributed streaming log with persistence",
			CostScore:   0.6, // higher infra cost
			RiskScore:   0.5, // higher operational complexity
			PerfScore:   0.7, // good throughput, higher latency
			EaseScore:   0.4, // complex consumer setup
		},
		{
			ID:          "opt:custom-db",
			Name:        "Custom Postgres Table",
			Description: "Relational table with unique transaction index",
			CostScore:   0.4,
			RiskScore:   0.3,
			PerfScore:   0.5,
			EaseScore:   0.6,
		},
	}

	weights := DefaultWeights()
	res, err := EvaluatePlan("Idempotency store for payments-api", options, weights)
	if err != nil {
		t.Fatal(err)
	}

	if res.RuleID != "decision_eval/v1" {
		t.Errorf("expected rule_id decision_eval/v1, got %s", res.RuleID)
	}
	if len(res.RankedOptions) != 3 {
		t.Fatalf("expected 3 ranked options, got %d", len(res.RankedOptions))
	}

	// Redis dominates Kafka on cost, risk, perf, and ease -> Redis must be rank 1 and recommended
	if res.RecommendedID != "opt:redis" {
		t.Errorf("expected opt:redis to be recommended, got %s", res.RecommendedID)
	}

	// Check Pareto efficiency: Redis should definitely be Pareto efficient
	foundRedisPareto := false
	for _, p := range res.ParetoFrontier {
		if p == "opt:redis" {
			foundRedisPareto = true
		}
	}
	if !foundRedisPareto {
		t.Errorf("opt:redis should be in Pareto frontier")
	}

	// Kafka is dominated by Redis on every single metric, so Kafka must NOT be Pareto efficient
	for _, p := range res.ParetoFrontier {
		if p == "opt:kafka" {
			t.Errorf("opt:kafka is dominated by opt:redis and should NOT be in Pareto frontier")
		}
	}
}

func TestEvaluatePlanEmpty(t *testing.T) {
	if _, err := EvaluatePlan("Empty options", nil, DefaultWeights()); err == nil {
		t.Fatal("empty options must be rejected")
	}
}

func TestEvaluatePlanValidationAndSensitivity(t *testing.T) {
	options := []Option{{ID: "a", Name: "A", CostScore: 0.2, RiskScore: 0.2, PerfScore: 0.9, EaseScore: 0.8}, {ID: "b", Name: "B", CostScore: 0.8, RiskScore: 0.8, PerfScore: 0.2, EaseScore: 0.2}}
	res, err := EvaluatePlan("example", options, DefaultWeights())
	if err != nil {
		t.Fatal(err)
	}
	if res.SensitivityNote == "" || res.RecommendedID != "a" || len(res.ParetoFrontier) != 1 || res.ParetoFrontier[0] != "a" {
		t.Fatalf("unexpected validated plan result: %+v", res)
	}
	if res.InputNote == "" {
		t.Fatal("caller-scored inputs need a provenance note")
	}
	bad := DefaultWeights()
	bad.CostWeight = -1
	if _, err := EvaluatePlan("example", options, bad); err == nil {
		t.Fatal("negative weights must be rejected")
	}
	bad = CriteriaWeights{CostWeight: math.MaxFloat64, RiskWeight: math.MaxFloat64}
	if _, err := EvaluatePlan("example", options, bad); err == nil {
		t.Fatal("overflowing total weights must be rejected")
	}
	options[0].PerfScore = 2
	if _, err := EvaluatePlan("example", options, DefaultWeights()); err == nil {
		t.Fatal("out-of-range scores must be rejected")
	}
}

func TestDetectAlerts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live graph test in short mode")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_rules_test")
	if err != nil {
		t.Fatalf("falkordb unreachable at %s: %v", addr, err)
	}
	t.Cleanup(func() { c.Delete(); c.Close() })
	if _, err := seed.Load(c); err != nil {
		t.Fatalf("seed load: %v", err)
	}

	alerts, err := DetectAlerts(c)
	if err != nil {
		t.Fatalf("DetectAlerts failed: %v", err)
	}

	// Baseline seed has 20 services; some may have single expert or multiple
	t.Logf("detected %d proactive alert(s)", len(alerts))
	for _, a := range alerts {
		if a.RuleID == "" || a.Severity == "" || a.Target == "" {
			t.Errorf("alert missing required fields: %+v", a)
		}
	}
}
