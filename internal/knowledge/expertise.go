package knowledge

import (
	"math"
	"sort"
	"time"
)

// Expertise/v1 ranks people for one service as a pure function of
// (fact paths, clock) — docs/knowledge-model.md. The result is an
// INFERENCE: computed at query time, never stored, never an LLM
// opinion. The LLM (later layer) may explain a ranking; it may never
// reorder it.

const ExpertiseRuleID = "expertise/v1"

// OwnerClass is the flag signal: current ownership ranks first and
// contributes no score (the spec says "flag: owner", not a weight).
const OwnerClass = "owner"

// ExpertiseWeights are part of the rule — change a weight, bump
// ExpertiseRuleID.
// ponytail: package-level mutable map read at query time; upgrade path
// = config file loaded once, same rule id discipline.
var ExpertiseWeights = map[string]float64{
	"authored_pr":       1.0,
	"reviewed_pr":       0.6,
	"resolved_incident": 1.5,
	"authored_doc":      0.8,
}

const expertiseHalfLifeDays = 180.0

// ExpertiseEvent is one fact path counted toward a person's score.
type ExpertiseEvent struct {
	PersonKey  string
	PersonName string
	Class      string // key in ExpertiseWeights, or OwnerClass
	OccurredAt time.Time
	Path       string
	Evidence   []Provenance
}

// Basis is one event as shown to a human: what counted, how much, and
// the fact path + edge provenance it came from.
type Basis struct {
	Class        string       `json:"class"`
	OccurredAt   string       `json:"occurred_at,omitempty"`
	Contribution float64      `json:"contribution"`
	Path         string       `json:"path"`
	Evidence     []Provenance `json:"evidence"`
}

type Expert struct {
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Score        float64        `json:"score"`
	CurrentOwner bool           `json:"current_owner"`
	TopSignals   map[string]int `json:"top_signals"`
	Basis        []Basis        `json:"basis"`
}

// ExpertiseResult is the tagged INFERENCE answer.
type ExpertiseResult struct {
	Layer       string             `json:"layer"` // "INFERENCE"
	RuleID      string             `json:"rule_id"`
	Weights     map[string]float64 `json:"weights"`
	GeneratedAt time.Time          `json:"generated_at"`
	Experts     []Expert           `json:"experts"`
}

// RankExpertise scores, sorts, and trims. Pure: same events + now →
// same output. Unknown classes score 0 (visible in basis, not hidden).
func RankExpertise(events []ExpertiseEvent, limit int, now time.Time) *ExpertiseResult {
	if limit <= 0 {
		limit = 5
	}
	byKey := map[string]*Expert{}
	var order []string
	for _, ev := range events {
		e, ok := byKey[ev.PersonKey]
		if !ok {
			e = &Expert{Key: ev.PersonKey, Name: ev.PersonName, TopSignals: map[string]int{}}
			byKey[ev.PersonKey] = e
			order = append(order, ev.PersonKey)
		}
		e.TopSignals[ev.Class]++
		contribution := 0.0
		switch {
		case ev.Class == OwnerClass:
			e.CurrentOwner = true
		default:
			contribution = ExpertiseWeights[ev.Class] * decay(ageDays(ev.OccurredAt, now))
			e.Score += contribution
		}
		occurred := ""
		if !ev.OccurredAt.IsZero() {
			occurred = ev.OccurredAt.UTC().Format(time.RFC3339)
		}
		e.Basis = append(e.Basis, Basis{
			Class:        ev.Class,
			OccurredAt:   occurred,
			Contribution: contribution,
			Path:         ev.Path,
			Evidence:     ev.Evidence,
		})
	}

	experts := make([]*Expert, 0, len(order))
	for _, k := range order {
		experts = append(experts, byKey[k])
	}
	sort.Slice(experts, func(i, j int) bool {
		a, b := experts[i], experts[j]
		if a.CurrentOwner != b.CurrentOwner {
			return a.CurrentOwner
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Key < b.Key
	})
	if len(experts) > limit {
		experts = experts[:limit]
	}

	result := &ExpertiseResult{
		Layer:       "INFERENCE",
		RuleID:      ExpertiseRuleID,
		Weights:     ExpertiseWeights,
		GeneratedAt: now,
		Experts:     make([]Expert, len(experts)),
	}
	for i, e := range experts {
		result.Experts[i] = *e
	}
	return result
}

// ageDays returns the event's age. Undated sources (optional
// published_at) age 0 — full weight, visible as empty occurred_at in
// basis.
// ponytail: undated events never decay; upgrade path: require the
// timestamp at ingest once every source provides it.
func ageDays(at, now time.Time) float64 {
	if at.IsZero() {
		return 0
	}
	return now.Sub(at).Hours() / 24
}

// decay applies the 180-day half-life. Future-dated sources (clock
// drift or bad data) clamp to age 0: never score more than fresh.
func decay(age float64) float64 {
	if age < 0 {
		age = 0
	}
	return math.Pow(0.5, age/expertiseHalfLifeDays)
}
