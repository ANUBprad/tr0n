package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/knowledge"
)

// AskRequest is a bounded question: explicit targets disambiguate names and
// provide context for follow-ups such as "who knows it best?".
type AskRequest struct {
	Question string `json:"question"`
	Target   string `json:"target,omitempty"`
	AsOf     string `json:"as_of,omitempty"`
}

type Report struct {
	knowledge.Answer
	Intent     string               `json:"intent"`
	Target     *graph.Candidate     `json:"target,omitempty"`
	Candidates []graph.Candidate    `json:"candidates,omitempty"`
	Evidence   []graph.NodeEvidence `json:"evidence"`
	Warnings   []string             `json:"warnings"`
	Handoff    string               `json:"handoff,omitempty"`
}

// Intent deliberately covers five workflows. Unsupported questions abstain;
// there is no model or speculative Cypher in this no-key agent.
func Intent(question string) string {
	q := " " + strings.Join(words(question), " ") + " "
	for _, rule := range []struct {
		intent string
		words  []string
	}{
		{"handoff", []string{"handoff", "hand off", "investigate", "investigation", "should handle", "route", "triage"}},
		{"experts", []string{"expert", "knows", "know", "best person"}},
		{"owner", []string{"owns", "owned", "owner", "responsible"}},
		{"decision", []string{"why", "decision", "chosen", "choose"}},
		{"incidents", []string{"incident", "happened", "before", "history", "timeout", "timed out"}},
	} {
		for _, word := range rule.words {
			if strings.Contains(q, " "+word+" ") {
				return rule.intent
			}
		}
	}
	return ""
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// Mention matching routes only to names actually in FalkorDB. Equal scores
// return candidates, including duplicate keys, instead of selecting a winner.
func mentioned(question, label string, catalog []graph.Candidate) []graph.Candidate {
	stop := " who what which why did we do does is are was were has have had the a an for of to in on it this that service system adopt adopted choose chosen use used over best knows know owns owner "
	q := " " + strings.Join(words(question), " ") + " "
	best := 0
	var hits []graph.Candidate
	for _, c := range catalog {
		if c.Label != label {
			continue
		}
		score := 0
		for _, word := range words(c.Name) {
			if len(word) > 2 && !strings.Contains(stop, " "+word+" ") && strings.Contains(q, " "+word+" ") {
				score++
			}
		}
		if strings.Contains(strings.ToLower(question), strings.ToLower(c.Key)) {
			score += 100
		}
		if score > best {
			best, hits = score, nil
		}
		if score > 0 && score == best {
			hits = append(hits, c)
		}
	}
	return hits
}

func Ask(ctx context.Context, c *graph.Client, req AskRequest) (*Report, error) {
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" || len(req.Question) > 2000 || len(req.Target) > 300 {
		return nil, fmt.Errorf("question must contain 1–2000 characters and target at most 300")
	}
	var asOf time.Time
	var err error
	if req.AsOf != "" {
		asOf, err = time.Parse(time.RFC3339, req.AsOf)
		if err != nil {
			return nil, fmt.Errorf("as_of must be RFC3339")
		}
	}
	out := &Report{Answer: knowledge.Answer{Question: req.Question, Layer: knowledge.LayerFact, OperationsUsed: []string{}, LatencyMS: map[string]int64{}}, Intent: Intent(req.Question), Evidence: []graph.NodeEvidence{}, Warnings: []string{}}
	if out.Intent == "" {
		out.Answer.Answer = "I can help with ownership, expertise, decision provenance, incident history, or an incident handoff. Please ask one of those questions."
		return out, nil
	}
	if !asOf.IsZero() && out.Intent != "owner" && out.Intent != "experts" {
		return nil, fmt.Errorf("as_of is supported for ownership and expertise only")
	}
	step := func(name string, f func() error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		start := time.Now()
		err := f()
		out.OperationsUsed = append(out.OperationsUsed, name)
		out.LatencyMS[name] += time.Since(start).Milliseconds()
		return err
	}
	label := "Service"
	if out.Intent == "decision" {
		label = "Decision"
	}
	var cat []graph.Candidate
	if err := step("Catalog", func() error { var e error; cat, e = c.Catalog(); return e }); err != nil {
		return nil, err
	}
	var hits []graph.Candidate
	if req.Target != "" {
		var resolved graph.Resolution
		if err := step("ResolveEntity", func() error { var e error; resolved, e = c.ResolveEntity(req.Target, label); return e }); err != nil {
			return nil, err
		}
		if resolved.Key != "" {
			for _, v := range cat {
				if v.Key == resolved.Key && v.Label == label {
					hits = append(hits, v)
				}
			}
		} else {
			hits = resolved.Candidates
		}
	} else {
		hits = mentioned(req.Question, label, cat)
	}
	if len(hits) != 1 {
		out.Candidates = hits
		out.Answer.Answer = "I could not identify a unique " + strings.ToLower(label) + ". Choose an entity or include its exact name/key."
		if len(hits) > 1 {
			out.Answer.Answer = "Several graph entities match. Choose one before I answer."
		}
		return out, nil
	}
	target := hits[0]
	out.Target = &target
	keys := []string{target.Key}
	add := func(text, layer, rule string, paths ...string) {
		out.Claims = append(out.Claims, knowledge.Claim{Text: text, Layer: layer, RuleID: rule, Paths: paths})
	}
	var owners []graph.Owner
	var experts *knowledge.ExpertiseResult
	var incidents []graph.IncidentRef
	if out.Intent == "owner" || out.Intent == "handoff" {
		if err := step("FindOwner", func() error { var e error; owners, e = c.FindOwner(target.Key, asOf); return e }); err != nil {
			return nil, err
		}
		for _, o := range owners {
			keys = append(keys, o.Key)
			add(o.Name+" owns "+target.Name+" at the requested time.", knowledge.LayerFact, "", fmt.Sprintf("(%s)-[:OWNS]->(%s)", o.Key, target.Key))
		}
		if len(owners) > 1 {
			out.Warnings = append(out.Warnings, "Conflicting ownership: multiple owners are valid at this time. Resolve the source disagreement before assigning a lead.")
		}
	}
	if out.Intent == "experts" || out.Intent == "handoff" {
		if err := step("FindExperts", func() error { var e error; experts, e = c.FindExperts(target.Key, 5, time.Now().UTC(), asOf); return e }); err != nil {
			return nil, err
		}
		for i, e := range experts.Experts {
			keys = append(keys, e.Key)
			paths := []string{}
			for _, b := range e.Basis {
				keys = append(keys, b.Keys...)
				if b.Path != "" {
					paths = append(paths, b.Path)
				}
			}
			if len(paths) > 0 {
				add(fmt.Sprintf("%s ranks #%d under %s (score %.2f; owner priority %t).", e.Name, i+1, experts.RuleID, e.Score, e.CurrentOwner), knowledge.LayerInference, experts.RuleID, paths...)
			}
		}
		ownerCount := 0
		for _, e := range experts.Experts {
			if e.CurrentOwner {
				ownerCount++
			}
		}
		if out.Intent == "experts" && ownerCount > 1 {
			out.Warnings = append(out.Warnings, "Multiple current owners influence this ranking. Resolve ownership before treating the owner priority as authoritative.")
		}
		if !asOf.IsZero() {
			out.Warnings = append(out.Warnings, "The as-of date filters ownership only. Activity scores use recorded history and current recency; this is not a full historical snapshot.")
		}
	}
	if out.Intent == "incidents" || out.Intent == "handoff" {
		keyword := ""
		if strings.Contains(strings.ToLower(req.Question), "timeout") || strings.Contains(strings.ToLower(req.Question), "timed out") {
			keyword = "timeout"
		}
		if err := step("FindRelatedIncidents", func() error { var e error; incidents, e = c.FindRelatedIncidents(target.Key, "", keyword); return e }); err != nil {
			return nil, err
		}
		for i, inc := range incidents {
			if i >= 10 {
				out.Warnings = append(out.Warnings, "Showing the 10 most recent matching incidents.")
				break
			}
			keys = append(keys, inc.Key)
			paths := []string{fmt.Sprintf("(%s)-[:AFFECTS]->(%s)", inc.Key, target.Key)}
			names := []string{}
			for _, p := range inc.ResolvedBy {
				names = append(names, p.Name)
				paths = append(paths, fmt.Sprintf("(%s)-[:RESOLVED_BY]->(%s)", inc.Key, p.Key))
			}
			text := inc.Title + " — " + inc.StartedAt
			if len(names) > 0 {
				text += "; recorded resolvers: " + strings.Join(names, ", ")
			}
			add(text, knowledge.LayerFact, "", paths...)
		}
	}
	switch out.Intent {
	case "owner":
		if len(owners) == 0 {
			out.Answer.Answer = "No owner is recorded for " + target.Name + " at this time."
		} else {
			names := []string{}
			for _, o := range owners {
				names = append(names, o.Name)
			}
			out.Answer.Answer = strings.Join(names, ", ") + " — recorded ownership of " + target.Name + "."
		}
	case "experts":
		if len(experts.Experts) == 0 {
			out.Answer.Answer = "No expertise evidence is recorded for " + target.Name + "."
		} else {
			out.Layer = knowledge.LayerInference
			out.Answer.Answer = experts.Experts[0].Name + " ranks first for " + target.Name + " under expertise/v1, which prioritizes current ownership. The activity scores and paths explain the ranking."
		}
	case "incidents":
		out.Answer.Answer = fmt.Sprintf("Found %d matching incident(s) affecting %s. Matching uses the service relationship and, when requested, the literal word timeout.", len(incidents), target.Name)
	case "decision":
		var trace *graph.DecisionTrace
		if err := step("TraceDecision", func() error { var e error; trace, e = c.TraceDecision(target.Key); return e }); err != nil {
			return nil, err
		}
		if trace == nil {
			out.Answer.Answer = "This decision no longer exists in the graph."
			break
		}
		out.Answer.Answer = trace.Decision.Title + " (" + trace.Decision.Status + "). "
		if trace.Decision.Rationale != "" {
			out.Answer.Answer += trace.Decision.Rationale
			add(trace.Decision.Rationale, knowledge.LayerFact, "", "("+target.Key+").rationale")
		} else {
			out.Answer.Answer += "No rationale text is recorded. The connected records below show provenance, but do not establish the reason."
		}
		for _, d := range trace.SupportedBy {
			keys = append(keys, d.Key)
			add("Supported by "+d.Title, knowledge.LayerFact, "", fmt.Sprintf("(%s)-[:SUPPORTED_BY]->(%s)", target.Key, d.Key))
		}
		for _, m := range trace.DiscussedIn {
			keys = append(keys, m.Key)
			add("Discussed in "+m.Title, knowledge.LayerFact, "", fmt.Sprintf("(%s)-[:DISCUSSED_IN]->(%s)", target.Key, m.Key))
		}
		if trace.Decision.Status == "superseded" {
			out.Warnings = append(out.Warnings, "This decision is superseded; inspect incoming SUPERSEDES edges for its replacement.")
		}
	case "handoff":
		lead := ""
		path := ""
		if len(owners) == 1 {
			lead = owners[0].Name
			path = fmt.Sprintf("(%s)-[:OWNS]->(%s)", owners[0].Key, target.Key)
		} else if len(owners) == 0 && experts != nil && len(experts.Experts) > 0 {
			lead = experts.Experts[0].Name
			if len(experts.Experts[0].Basis) > 0 {
				path = experts.Experts[0].Basis[0].Path
			}
		}
		if lead == "" || path == "" {
			out.Answer.Answer = "I cannot recommend a lead from the available, unambiguous evidence. Review the sources and ownership first."
		} else {
			out.Layer = knowledge.LayerRecommendation
			out.Answer.Answer = "Suggested investigation lead: " + lead + " for " + target.Name + ". Review the draft and confirm availability before assigning work."
			add("Suggested lead: "+lead, knowledge.LayerRecommendation, "", path)
		}
	}
	unique := []string{}
	seen := map[string]bool{}
	for _, k := range keys {
		if !seen[k] {
			unique = append(unique, k)
			seen[k] = true
		}
	}
	if err := step("TraceEvidence", func() error { var e error; out.Evidence, e = c.TraceEvidence(unique); return e }); err != nil {
		return nil, err
	}

	if out.Intent == "handoff" {
		var b strings.Builder
		fmt.Fprintf(&b, "# Investigation handoff: %s\n\nDRAFT — requires human review; no assignment has been sent.\n\n## Request\n%s\n\n## Recommendation\n%s\n\n## Evidence\n", target.Name, req.Question, out.Answer.Answer)
		for _, cl := range out.Claims {
			fmt.Fprintf(&b, "- [%s] %s\n", cl.Layer, cl.Text)
			for _, p := range cl.Paths {
				fmt.Fprintf(&b, "  - %s\n", p)
			}
		}
		b.WriteString("\n## Sources\n")
		for _, ev := range out.Evidence {
			fmt.Fprintf(&b, "- %s (%s): %v; observed %v\n", ev.Name, ev.Key, ev.Properties["src_ref"], ev.Properties["observed_at"])
		}
		b.WriteString("\n## Checks for the investigator\n- Confirm current impact and severity.\n- Compare symptoms with the linked incidents and runbooks.\n- Confirm owner availability and record the resolution.\n")
		for _, w := range out.Warnings {
			fmt.Fprintf(&b, "\nCaution: %s\n", w)
		}
		out.Handoff = b.String()
	}
	if err := out.Answer.Validate(); err != nil {
		return nil, err
	}
	return out, nil
}
