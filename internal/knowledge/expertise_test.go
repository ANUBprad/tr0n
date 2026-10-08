package knowledge

import (
	"testing"
	"time"
)

var now = time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)

// The check that fails if weights, decay, owner-first ordering, or the
// INFERENCE tagging change. No graph needed.
func TestRankExpertise(t *testing.T) {
	recent := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) // 37d ago
	stale := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)  // 645d ago
	incident := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

	events := []ExpertiseEvent{
		{PersonKey: "person:ada", PersonName: "Ada", Class: OwnerClass, Path: "owner-path"},
		{PersonKey: "person:ada", PersonName: "Ada", Class: "authored_pr", OccurredAt: recent},
		{PersonKey: "person:ada", PersonName: "Ada", Class: "authored_pr", OccurredAt: stale},
		{PersonKey: "person:ada", PersonName: "Ada", Class: "authored_doc", OccurredAt: time.Time{}},
		{PersonKey: "person:bob", PersonName: "Bob", Class: "reviewed_pr", OccurredAt: recent},
		{PersonKey: "person:bob", PersonName: "Bob", Class: "resolved_incident", OccurredAt: incident},
	}

	got := RankExpertise(events, 5, now)

	if got.Layer != "INFERENCE" || got.RuleID != ExpertiseRuleID {
		t.Fatalf("layer tagging broken: %+v", got)
	}
	if got.Weights["authored_pr"] != 1.0 || got.Weights["reviewed_pr"] != 0.6 ||
		got.Weights["resolved_incident"] != 1.5 || got.Weights["authored_doc"] != 0.8 {
		t.Errorf("weights mismatch: %+v", got.Weights)
	}
	if len(got.Experts) != 2 {
		t.Fatalf("want 2 experts, got %d", len(got.Experts))
	}

	ada := got.Experts[0]
	if ada.Key != "person:ada" || !ada.CurrentOwner {
		t.Errorf("owner must rank first: %+v", ada)
	}
	wantAda := 1.0*decay(ageDays(recent, now)) +
		1.0*decay(ageDays(stale, now)) +
		0.8*1.0 // undated doc: age 0, full weight
	if diff := ada.Score - wantAda; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("ada score = %v, want %v", ada.Score, wantAda)
	}
	if decay(ageDays(stale, now)) >= 0.5 {
		t.Errorf("645-day-old event should be below half weight: %v", decay(ageDays(stale, now)))
	}
	if ada.TopSignals["authored_pr"] != 2 || ada.TopSignals["owner"] != 1 {
		t.Errorf("ada top signals: %+v", ada.TopSignals)
	}
	if len(ada.Basis) != 4 || ada.Basis[0].Path == "" {
		t.Errorf("ada basis missing/empty: %+v", ada.Basis)
	}
	if ada.Basis[0].Class == "authored_doc" && ada.Basis[0].OccurredAt != "" {
		t.Errorf("undated event must show empty occurred_at: %+v", ada.Basis[0])
	}

	bob := got.Experts[1]
	if bob.CurrentOwner || bob.Score <= 0 {
		t.Errorf("bob: %+v", bob)
	}
	if len(bob.Basis) != 2 || bob.Basis[0].Contribution <= 0 {
		t.Errorf("bob basis: %+v", bob.Basis)
	}
}

// future-dated event clamps to full weight (clock drift must not
// out-score fresh real data).
func TestRankExpertiseFutureClamp(t *testing.T) {
	future := now.Add(72 * time.Hour)
	got := RankExpertise([]ExpertiseEvent{
		{PersonKey: "p", PersonName: "P", Class: "authored_pr", OccurredAt: future},
	}, 5, now)
	if s := got.Experts[0].Score; s != 1.0 {
		t.Errorf("future-dated event score = %v, want 1.0", s)
	}
}

func TestRankExpertiseLimitAndTiebreak(t *testing.T) {
	events := []ExpertiseEvent{
		{PersonKey: "person:b", PersonName: "B", Class: "authored_pr", OccurredAt: now},
		{PersonKey: "person:a", PersonName: "A", Class: "authored_pr", OccurredAt: now},
		{PersonKey: "person:c", PersonName: "C", Class: "authored_pr", OccurredAt: now},
	}
	got := RankExpertise(events, 2, now)
	if len(got.Experts) != 2 {
		t.Fatalf("limit ignored: %d", len(got.Experts))
	}
	// identical scores → deterministic key order
	if got.Experts[0].Key != "person:a" || got.Experts[1].Key != "person:b" {
		t.Errorf("tiebreak broken: %v, %v", got.Experts[0].Key, got.Experts[1].Key)
	}
	// limit <= 0 → default 5
	if def := RankExpertise(events, 0, now); len(def.Experts) != 3 {
		t.Errorf("default limit: %d", len(def.Experts))
	}
}
