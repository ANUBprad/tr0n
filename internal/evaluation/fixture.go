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
// synthetic seed. Currently that is one service with two incompatible live
// ownership claims (the conflicting-sources case). It is additive and
// idempotent at the key level (the loader upserts).
func LoadFixtures(c *graph.Client) error {
	if err := c.LoadNodes("Service", []map[string]interface{}{{
		"key": "service:conflict-demo", "name": "conflict-demo",
		"src_type": "eval_fixture", "src_ref": fixtureSrcRef,
		"observed_at": fixtureObservedAt, "extraction": "deterministic",
	}}); err != nil {
		return err
	}
	// Two open-ended OWNS assertions for the same service: both are live at any
	// "now", so no single owner can be chosen without contradicting a source.
	return c.LoadEdges("OWNS", []graph.EdgeRow{
		{From: "person:1", To: "service:conflict-demo", Props: fixtureProv(map[string]interface{}{"valid_from": "2025-01-01T00:00:00Z", "slot": "service_owner"})},
		{From: "person:2", To: "service:conflict-demo", Props: fixtureProv(map[string]interface{}{"valid_from": "2025-06-01T00:00:00Z", "slot": "service_owner"})},
	})
}
