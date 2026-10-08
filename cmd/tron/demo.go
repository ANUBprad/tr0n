package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
)

// runDemo executes the five DEMO_SPEC scenarios plus the evidence
// drill-down as pass/fail checks against the connected graph. It runs
// every check (a demo with three broken steps still shows what works)
// and returns a non-nil error if any failed.
//
// The narrative doubles as the judge-facing proof: each [ok] line is
// produced by a real FalkorDB traversal, not a canned string.
func runDemo(w io.Writer, c *graph.Client) error {
	fmt.Fprintln(w, "TRON demo — five scenarios + evidence drill-down")
	fails := 0
	report := func(title, detail string, ok bool) {
		if ok {
			fmt.Fprintf(w, "  [ok]   %s — %s\n", title, detail)
			return
		}
		fails++
		fmt.Fprintf(w, "  [FAIL] %s — %s\n", title, detail)
	}
	seedHint := "graph has no matching data — run: tron seed"

	owners, err := c.FindOwner("payments-api")
	ownerOK := err == nil && len(owners) > 0
	if ownerOK {
		report("1. Who owns payments-api?",
			fmt.Sprintf("%s (%s), %d current owner(s)", owners[0].Name, owners[0].Key, len(owners)), true)
	} else {
		report("1. Who owns payments-api?", failDetail(err, seedHint), false)
	}

	experts, err := c.FindExperts("service:payments-api", 5, time.Now().UTC())
	expertOK := err == nil && experts != nil && len(experts.Experts) > 0 &&
		experts.RuleID == "expertise/v1" && experts.Layer == "INFERENCE"
	switch {
	case expertOK:
		top := experts.Experts[0]
		report("2. Who knows payments-api best?",
			fmt.Sprintf("top %s (score %.2f), rule %s, %d expert(s)",
				top.Key, top.Score, experts.RuleID, len(experts.Experts)), true)
	case err == nil && experts != nil && len(experts.Experts) > 0:
		// Data exists but the tagging contract drifted — say exactly that.
		report("2. Who knows payments-api best?",
			fmt.Sprintf("contract broken: rule=%q layer=%q, want expertise/v1|INFERENCE",
				experts.RuleID, experts.Layer), false)
	default:
		report("2. Who knows payments-api best?", failDetail(err, "no ranked experts — "+seedHint), false)
	}

	trace, err := c.TraceDecision("decision:1")
	neighborhood := func(t *graph.DecisionTrace) string {
		return fmt.Sprintf("%d author(s), %d doc(s), %d meeting(s), %d superseded",
			len(t.AuthoredBy), len(t.SupportedBy), len(t.DiscussedIn), len(t.Supersedes))
	}
	decisionOK := err == nil && trace != nil &&
		len(trace.AuthoredBy) > 0 && len(trace.SupportedBy) > 0 &&
		len(trace.DiscussedIn) > 0 && len(trace.Supersedes) > 0
	switch {
	case decisionOK:
		report("3. Why did we adopt FalkorDB?",
			fmt.Sprintf("%q — %s", trace.Decision.Title, neighborhood(trace)), true)
	case err == nil && trace != nil:
		report("3. Why did we adopt FalkorDB?", "incomplete neighborhood: "+neighborhood(trace), false)
	default:
		report("3. Why did we adopt FalkorDB?", failDetail(err, "decision:1 missing — "+seedHint), false)
	}

	incidents, err := c.FindRelatedIncidents("service:payments-api", "", "timeout")
	incOK := err == nil && len(incidents) > 0
	if incOK {
		report("4. Has payments-api timed out before?",
			fmt.Sprintf("%d prior incident(s), latest %q (%s)",
				len(incidents), incidents[0].Title, incidents[0].Severity), true)
	} else {
		report("4. Has payments-api timed out before?", failDetail(err, seedHint), false)
	}

	routeOK := ownerOK && expertOK
	if routeOK {
		report("5. Who should handle a payments-api issue?",
			fmt.Sprintf("recommend %s (owner + top expert on expertise/v1)", experts.Experts[0].Key), true)
	} else {
		missing := "no owner"
		if ownerOK {
			missing = "no experts"
		}
		report("5. Who should handle a payments-api issue?", "insufficient signals: "+missing, false)
	}

	evKey := "service:payments-api"
	if ownerOK {
		evKey = owners[0].Key
	}
	ev, err := c.TraceEvidence([]string{evKey})
	evOK := err == nil && len(ev) == 1 && ev[0].Found && len(ev[0].Edges) > 0
	if evOK {
		report("Evidence drill-down (TraceEvidence)",
			fmt.Sprintf("%s — %d surrounding edge(s) with provenance", evKey, len(ev[0].Edges)), true)
	} else {
		report("Evidence drill-down (TraceEvidence)",
			failDetail(err, evKey+" not found with edges — "+seedHint), false)
	}

	const checks = 6
	if fails > 0 {
		return fmt.Errorf("%d of %d demo checks failed", fails, checks)
	}
	fmt.Fprintf(w, "%d/%d demo checks passed\n", checks, checks)
	return nil
}

// failDetail explains a failed check: FalkorDB reports a graph that
// does not exist as "empty key", which a demo runner should see as a
// seeding instruction, not a driver error. Anything else is raw.
func failDetail(err error, empty string) string {
	switch {
	case err == nil:
		return empty
	case strings.Contains(err.Error(), "empty key"):
		return "graph does not exist — run: tron seed"
	default:
		return err.Error()
	}
}
