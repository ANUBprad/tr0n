package evaluation

import "github.com/ANUBprad/tr0n/internal/graph"

// Adversarial fixtures for the labeled corpus. Every key here is disjoint from
// the synthetic seed, so loading them never changes an existing case's answer.
const (
	fixtureObservedAt = "2026-10-01T00:00:00Z"
	fixtureSrcRef     = "acme-eval/adversarial/v1"
)

func fixtureProv(extra map[string]interface{}) map[string]interface{} {
	p := map[string]interface{}{
		"src_type": "eval_fixture", "src_ref": fixtureSrcRef,
		"observed_at": fixtureObservedAt, "extraction": "deterministic",
	}
	for k, v := range extra {
		p[k] = v
	}
	return p
}

// LoadFixtures adds the real-graph setup the adversarial cases need beyond the
// synthetic seed: one service with two incompatible live ownership claims (the
// conflicting-sources case) and one service whose owner was superseded by a
// clean handoff (the stale-or-superseded case). It is additive and idempotent
// at the key level (the loader upserts).
func LoadFixtures(c *graph.Client) error {
	if err := c.LoadNodes("Service", []map[string]interface{}{
		{
			"key": "service:conflict-demo", "name": "conflict-demo",
			"src_type": "eval_fixture", "src_ref": fixtureSrcRef,
			"observed_at": fixtureObservedAt, "extraction": "deterministic",
		},
		{
			"key": "service:stale-demo", "name": "stale-demo",
			"src_type": "eval_fixture", "src_ref": fixtureSrcRef,
			"observed_at": fixtureObservedAt, "extraction": "deterministic",
		},
	}); err != nil {
		return err
	}
	// conflict-demo: two open-ended OWNS assertions, both live at any "now", so
	// no single owner can be chosen without contradicting a source.
	// stale-demo: person:7 held it over a closed window that ended when person:8
	// took over (open-ended). The closed window is superseded, never current.
	return c.LoadEdges("OWNS", []graph.EdgeRow{
		{From: "person:1", To: "service:conflict-demo", Props: fixtureProv(map[string]interface{}{"valid_from": "2025-01-01T00:00:00Z", "slot": "service_owner"})},
		{From: "person:2", To: "service:conflict-demo", Props: fixtureProv(map[string]interface{}{"valid_from": "2025-06-01T00:00:00Z", "slot": "service_owner"})},
		{From: "person:7", To: "service:stale-demo", Props: fixtureProv(map[string]interface{}{"valid_from": "2025-01-11T09:00:00Z", "valid_to": "2025-02-10T09:00:00Z", "slot": "service_owner"})},
		{From: "person:8", To: "service:stale-demo", Props: fixtureProv(map[string]interface{}{"valid_from": "2025-02-10T09:00:00Z", "slot": "service_owner"})},
	})
}
