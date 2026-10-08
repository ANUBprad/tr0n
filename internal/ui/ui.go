// Package ui is the server-rendered Q&A surface: five built-in
// questions answered deterministically from graph operations — the
// no-LLM demo path (docs/AGENT_SPEC.md, docs/DEMO_SPEC.md). Every
// rendered answer must pass knowledge.Answer.Validate first: the
// response type is the guard, not the template.
package ui

import (
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/knowledge"
)

//go:embed page.html
var pageHTML string

// question is one built-in prompt on the index page. The set is fixed
// (DEMO_SPEC: prepared queries for reliability).
type question struct {
	ID    string
	Title string
}

var questions = []question{
	{"owner", "Who owns payments-api?"},
	{"experts", "Who knows payments-api best?"},
	{"decision", "Why did we adopt FalkorDB?"},
	{"incidents", "Has payments-api timed out before?"},
	{"route", "Who should handle a payments-api issue?"},
}

// Section is one evidence block in the panel: an optional table plus
// bullet lines (fact paths, provenance, notes).
type Section struct {
	Title  string
	Header []string
	Rows   [][]string
	Notes  []string
}

type view struct {
	Questions []question
	Active    string
	Answer    *knowledge.Answer
	Sections  []Section
}

var errUnknownQuestion = errors.New("unknown question id")

// Handler serves "/" (question index) and "/?q=<id>" (answer + panel).
func Handler(c *graph.Client) http.Handler {
	tmpl := template.Must(template.New("page").Parse(pageHTML))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		v := view{Questions: questions, Active: r.URL.Query().Get("q")}
		if v.Active != "" {
			a, sections, err := answer(c, v.Active)
			switch {
			case errors.Is(err, errUnknownQuestion):
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			case err != nil:
				http.Error(w, "graph error: "+err.Error(), http.StatusInternalServerError)
				return
			default:
				if err := a.Validate(); err != nil {
					// A composer produced an untagged/unevidenced claim —
					// refuse to render it (AGENT_SPEC response-type guard).
					http.Error(w, "answer failed validation: "+err.Error(), http.StatusInternalServerError)
					return
				}
				v.Answer, v.Sections = a, sections
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmpl.Execute(w, v)
	})
}

func answer(c *graph.Client, id string) (*knowledge.Answer, []Section, error) {
	switch id {
	case "owner":
		return ownerAnswer(c)
	case "experts":
		return expertsAnswer(c)
	case "decision":
		return decisionAnswer(c)
	case "incidents":
		return incidentsAnswer(c)
	case "route":
		return routeAnswer(c)
	default:
		return nil, nil, errUnknownQuestion
	}
}

func ownerAnswer(c *graph.Client) (*knowledge.Answer, []Section, error) {
	lat := map[string]int64{}
	start := time.Now()
	owners, err := c.FindOwner("payments-api")
	lat["FindOwner"] = time.Since(start).Milliseconds()
	if err != nil {
		return nil, nil, err
	}
	a := &knowledge.Answer{
		Question:       "Who owns payments-api?",
		Layer:          knowledge.LayerFact,
		OperationsUsed: []string{"FindOwner"},
		LatencyMS:      lat,
	}
	sec := Section{
		Title:  "Ownership — current OWNS edges",
		Header: []string{"Name", "Key", "Kind"},
	}
	if len(owners) == 0 {
		a.Answer = "No current owner is recorded for payments-api."
		a.Claims = []knowledge.Claim{{
			Text:  "payments-api has no current OWNS edge.",
			Layer: knowledge.LayerFact,
			Paths: []string{"FindOwner(payments-api) → 0 results"},
		}}
	}
	for _, o := range owners {
		sec.Rows = append(sec.Rows, []string{o.Name, o.Key, o.Kind})
		sec.Notes = append(sec.Notes, provLine(o.Key, o.Provenance))
		a.Claims = append(a.Claims, knowledge.Claim{
			Text:  fmt.Sprintf("%s (%s) owns payments-api (current).", o.Name, o.Key),
			Layer: knowledge.LayerFact,
			Paths: []string{fmt.Sprintf("(:%s %s)-[:OWNS]->(:Service service:payments-api)", o.Kind, o.Key)},
		})
		a.Answer = fmt.Sprintf("%s (%s) owns payments-api.", o.Name, o.Key)
	}
	return a, []Section{sec}, nil
}

func expertsAnswer(c *graph.Client) (*knowledge.Answer, []Section, error) {
	lat := map[string]int64{}
	start := time.Now()
	res, err := c.FindExperts("service:payments-api", 5, time.Now().UTC())
	lat["FindExperts"] = time.Since(start).Milliseconds()
	if err != nil {
		return nil, nil, err
	}
	a := &knowledge.Answer{
		Question:       "Who knows payments-api best?",
		OperationsUsed: []string{"FindExperts"},
		LatencyMS:      lat,
	}
	if len(res.Experts) == 0 {
		a.Layer = knowledge.LayerFact
		a.Answer = "No expertise signals are recorded for payments-api."
		a.Claims = []knowledge.Claim{{
			Text:  "payments-api has no expertise basis events.",
			Layer: knowledge.LayerFact,
			Paths: []string{"FindExperts(service:payments-api) → 0 results"},
		}}
		return a, nil, nil
	}

	a.Layer = knowledge.LayerInference
	top := res.Experts[0]
	paths := basisPaths(top)
	if len(paths) == 0 {
		paths = []string{"FindExperts(service:payments-api) → expertise/v1 basis empty"}
	}
	a.Claims = append(a.Claims, knowledge.Claim{
		Text:   fmt.Sprintf("%s ranks #1 on payments-api (score %.2f).", top.Name, top.Score),
		Layer:  knowledge.LayerInference,
		Paths:  paths,
		RuleID: res.RuleID,
	})
	if top.CurrentOwner {
		a.Claims = append(a.Claims, knowledge.Claim{
			Text:  fmt.Sprintf("%s is also the current owner.", top.Name),
			Layer: knowledge.LayerFact,
			Paths: []string{fmt.Sprintf("(:Person %s)-[:OWNS]->(:Service service:payments-api)", top.Key)},
		})
	}
	a.Answer = fmt.Sprintf("%s is the top expert on payments-api under %s (score %.2f)",
		top.Name, res.RuleID, top.Score)
	if others := expertNames(res.Experts[1:]); others != "" {
		a.Answer += ", followed by " + others
	}
	a.Answer += "."

	sec := Section{
		Title:  "Ranked experts — INFERENCE (" + res.RuleID + ")",
		Header: []string{"#", "Name", "Key", "Score", "Owner", "Signals"},
	}
	for i, e := range res.Experts {
		owner := ""
		if e.CurrentOwner {
			owner = "yes"
		}
		sec.Rows = append(sec.Rows, []string{
			strconv.Itoa(i + 1), e.Name, e.Key, fmt.Sprintf("%.2f", e.Score),
			owner, signalsLine(e.TopSignals),
		})
	}
	basis := Section{
		Title: "Basis — fact paths behind the ranking",
		Notes: []string{"weights: " + weightsLine(res.Weights) +
			" · generated " + res.GeneratedAt.Format(time.RFC3339)},
	}
	for _, b := range top.Basis {
		basis.Notes = append(basis.Notes,
			fmt.Sprintf("%s · +%.3f · %s", b.Class, b.Contribution, b.Path))
	}
	return a, []Section{sec, basis}, nil
}

func decisionAnswer(c *graph.Client) (*knowledge.Answer, []Section, error) {
	lat := map[string]int64{}
	start := time.Now()
	trace, err := c.TraceDecision("decision:1")
	lat["TraceDecision"] = time.Since(start).Milliseconds()
	if err != nil {
		return nil, nil, err
	}
	a := &knowledge.Answer{
		Question:       "Why did we adopt FalkorDB?",
		Layer:          knowledge.LayerFact,
		OperationsUsed: []string{"TraceDecision"},
		LatencyMS:      lat,
	}
	if trace == nil {
		a.Answer = "decision:1 is not in the graph."
		a.Claims = []knowledge.Claim{{
			Text:  "decision:1 does not exist.",
			Layer: knowledge.LayerFact,
			Paths: []string{"TraceDecision(decision:1) → not found"},
		}}
		return a, nil, nil
	}
	d := trace.Decision
	var claims []knowledge.Claim
	var parts []string
	claims = append(claims, knowledge.Claim{
		Text:  fmt.Sprintf("%q is %s (decided %s).", d.Title, d.Status, d.DecidedAt),
		Layer: knowledge.LayerFact,
		Paths: []string{"(:Decision decision:1)"},
	})

	people := Section{Title: "Authors (AUTHORED)", Header: []string{"Name", "Key"}}
	var authorNames []string
	for _, p := range trace.AuthoredBy {
		people.Rows = append(people.Rows, []string{p.Name, p.Key})
		people.Notes = append(people.Notes, provLine(p.Key, p.Provenance))
		authorNames = append(authorNames, p.Name)
		claims = append(claims, knowledge.Claim{
			Text:  fmt.Sprintf("%s authored decision:1.", p.Name),
			Layer: knowledge.LayerFact,
			Paths: []string{fmt.Sprintf("(:Person %s)-[:AUTHORED]->(:Decision decision:1)", p.Key)},
		})
	}
	if len(authorNames) > 0 {
		parts = append(parts, "authored by "+strings.Join(authorNames, " and "))
	}

	docs := Section{Title: "Supporting documents (SUPPORTED_BY)", Header: []string{"Title", "Kind", "URL"}}
	for _, doc := range trace.SupportedBy {
		docs.Rows = append(docs.Rows, []string{doc.Title, doc.Kind, doc.URL})
		docs.Notes = append(docs.Notes, provLine(doc.Key, doc.Provenance))
		claims = append(claims, knowledge.Claim{
			Text:  fmt.Sprintf("Supported by %q (%s).", doc.Title, doc.Kind),
			Layer: knowledge.LayerFact,
			Paths: []string{fmt.Sprintf("(:Decision decision:1)-[:SUPPORTED_BY]->(:Document %s)", doc.Key)},
		})
	}
	if n := len(trace.SupportedBy); n > 0 {
		parts = append(parts, fmt.Sprintf("supported by %d document(s)", n))
	}

	meetings := Section{Title: "Meetings (DISCUSSED_IN)", Header: []string{"Title", "Held at", "Participants"}}
	for _, m := range trace.DiscussedIn {
		var names []string
		for _, p := range m.Participants {
			names = append(names, p.Name)
		}
		meetings.Rows = append(meetings.Rows, []string{m.Title, m.HeldAt, strings.Join(names, ", ")})
		meetings.Notes = append(meetings.Notes, provLine(m.Key, m.Provenance))
		claims = append(claims, knowledge.Claim{
			Text:  fmt.Sprintf("Discussed in %q with %d participant(s).", m.Title, len(m.Participants)),
			Layer: knowledge.LayerFact,
			Paths: []string{fmt.Sprintf("(:Decision decision:1)-[:DISCUSSED_IN]->(:Meeting %s)", m.Key)},
		})
		parts = append(parts, fmt.Sprintf("discussed in %q", m.Title))
	}

	supersedes := Section{Title: "Supersedes chain (SUPERSEDES, ≤2 hops)", Header: []string{"Key", "Title"}}
	for _, s := range trace.Supersedes {
		supersedes.Rows = append(supersedes.Rows, []string{s.Key, s.Title})
		supersedes.Notes = append(supersedes.Notes,
			fmt.Sprintf("%s: %d provenance record(s)", s.Key, len(s.Evidence)))
		claims = append(claims, knowledge.Claim{
			Text:  fmt.Sprintf("Supersedes %q (%s).", s.Title, s.Key),
			Layer: knowledge.LayerFact,
			Paths: []string{fmt.Sprintf("(:Decision decision:1)-[:SUPERSEDES]->(:Decision %s)", s.Key)},
		})
	}
	if len(trace.Supersedes) > 0 {
		parts = append(parts, fmt.Sprintf("supersedes %d earlier decision(s)", len(trace.Supersedes)))
	}

	dec := Section{
		Title:  "Decision",
		Header: []string{"Key", "Title", "Status", "Decided at"},
		Rows:   [][]string{{d.Key, d.Title, d.Status, d.DecidedAt}},
	}
	a.Answer = d.Title + " — " + strings.Join(parts, ", ") + "."
	a.Claims = claims
	sections := []Section{dec, people, docs, meetings, supersedes}
	return a, sections, nil
}

func incidentsAnswer(c *graph.Client) (*knowledge.Answer, []Section, error) {
	lat := map[string]int64{}
	start := time.Now()
	incidents, err := c.FindRelatedIncidents("service:payments-api", "", "timeout")
	lat["FindRelatedIncidents"] = time.Since(start).Milliseconds()
	if err != nil {
		return nil, nil, err
	}
	a := &knowledge.Answer{
		Question:       "Has payments-api timed out before?",
		Layer:          knowledge.LayerFact,
		OperationsUsed: []string{"FindRelatedIncidents"},
		LatencyMS:      lat,
	}
	sec := Section{
		Title:  "Prior timeout incidents on payments-api (AFFECTS)",
		Header: []string{"Key", "Title", "Severity", "Started", "Resolved", "Handled by"},
	}
	if len(incidents) == 0 {
		a.Answer = "No prior timeout incidents are recorded for payments-api."
		a.Claims = []knowledge.Claim{{
			Text:  "No incident with \"timeout\" in the title affects payments-api.",
			Layer: knowledge.LayerFact,
			Paths: []string{"FindRelatedIncidents(service:payments-api, keyword=timeout) → 0 results"},
		}}
		return a, []Section{sec}, nil
	}
	for _, inc := range incidents {
		var handlers []string
		var paths []string
		for _, p := range inc.ResolvedBy {
			handlers = append(handlers, p.Name)
			paths = append(paths, fmt.Sprintf("(:Incident %s)-[:RESOLVED_BY]->(:Person %s)", inc.Key, p.Key))
		}
		paths = append([]string{fmt.Sprintf("(:Incident %s)-[:AFFECTS]->(:Service service:payments-api)", inc.Key)}, paths...)
		resolved := inc.ResolvedAt
		if resolved == "" {
			resolved = "unresolved"
		}
		sec.Rows = append(sec.Rows, []string{
			inc.Key, inc.Title, inc.Severity, inc.StartedAt, resolved, strings.Join(handlers, ", "),
		})
		sec.Notes = append(sec.Notes, provLine(inc.Key, inc.Provenance))
		a.Claims = append(a.Claims, knowledge.Claim{
			Text:  fmt.Sprintf("%s (%s) affected payments-api, started %s.", inc.Title, inc.Severity, inc.StartedAt),
			Layer: knowledge.LayerFact,
			Paths: paths,
		})
	}
	latest := incidents[0]
	a.Answer = fmt.Sprintf("Yes — %d prior timeout incident(s) on payments-api; most recent: %q (%s, started %s).",
		len(incidents), latest.Title, latest.Severity, latest.StartedAt)
	return a, []Section{sec}, nil
}

// routeAnswer is DEMO_SPEC scenario 5: a recommendation composed from
// FindOwner + FindExperts + FindRelatedIncidents (the deferred
// prepare_handoff composition, answered deterministically).
func routeAnswer(c *graph.Client) (*knowledge.Answer, []Section, error) {
	lat := map[string]int64{}
	timeOp := func(name string, run func() error) error {
		start := time.Now()
		err := run()
		lat[name] = time.Since(start).Milliseconds()
		return err
	}
	var owners []graph.Owner
	var experts *knowledge.ExpertiseResult
	var incidents []graph.IncidentRef
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"FindOwner", func() error { var err error; owners, err = c.FindOwner("payments-api"); return err }},
		{"FindExperts", func() error {
			var err error
			experts, err = c.FindExperts("service:payments-api", 3, time.Now().UTC())
			return err
		}},
		{"FindRelatedIncidents", func() error {
			var err error
			incidents, err = c.FindRelatedIncidents("service:payments-api", "", "")
			return err
		}},
	} {
		if err := timeOp(step.name, step.run); err != nil {
			return nil, nil, err
		}
	}

	a := &knowledge.Answer{
		Question:       "Who should handle a payments-api issue?",
		Layer:          knowledge.LayerRecommendation,
		OperationsUsed: []string{"FindOwner", "FindExperts", "FindRelatedIncidents"},
		LatencyMS:      lat,
	}
	var paths []string
	var claims []knowledge.Claim
	var reasons []string

	recommend := ""
	recKey := ""
	if len(owners) > 0 {
		o := owners[0]
		recommend, recKey = o.Name, o.Key
		path := fmt.Sprintf("(:%s %s)-[:OWNS]->(:Service service:payments-api)", o.Kind, o.Key)
		paths = append(paths, path)
		claims = append(claims, knowledge.Claim{
			Text:  fmt.Sprintf("%s (%s) owns payments-api.", o.Name, o.Key),
			Layer: knowledge.LayerFact,
			Paths: []string{path},
		})
		reasons = append(reasons, "current owner")
	}
	var top knowledge.Expert
	if experts != nil && len(experts.Experts) > 0 {
		top = experts.Experts[0]
		if recommend == "" {
			recommend, recKey = top.Name, top.Key
		}
		bpaths := basisPaths(top)
		if len(bpaths) == 0 {
			bpaths = []string{"FindExperts(service:payments-api) → expertise/v1 basis empty"}
		}
		paths = append(paths, bpaths...)
		claims = append(claims, knowledge.Claim{
			Text:   fmt.Sprintf("%s ranks #1 on expertise/v1 (score %.2f).", top.Name, top.Score),
			Layer:  knowledge.LayerInference,
			Paths:  bpaths,
			RuleID: experts.RuleID,
		})
		reasons = append(reasons, fmt.Sprintf("top expert (score %.2f)", top.Score))
	}
	handlers := handlerNames(incidents)
	if len(incidents) > 0 {
		var ipaths []string
		for _, inc := range incidents {
			ipaths = append(ipaths, fmt.Sprintf("(:Incident %s)-[:AFFECTS]->(:Service service:payments-api)", inc.Key))
			for _, p := range inc.ResolvedBy {
				ipaths = append(ipaths, fmt.Sprintf("(:Incident %s)-[:RESOLVED_BY]->(:Person %s)", inc.Key, p.Key))
			}
		}
		paths = append(paths, ipaths...)
		claims = append(claims, knowledge.Claim{
			Text:  fmt.Sprintf("%d incident(s) affect payments-api; handled by %s.", len(incidents), handlers),
			Layer: knowledge.LayerFact,
			Paths: ipaths,
		})
		reasons = append(reasons, fmt.Sprintf("%d prior incidents involving %s", len(incidents), handlers))
	}

	sec := Section{Title: "Inputs to the recommendation", Header: []string{"Role", "Person", "Why"}}
	if len(paths) == 0 {
		a.Layer = knowledge.LayerFact
		a.Answer = "Insufficient graph data to recommend an owner for payments-api."
		a.Claims = []knowledge.Claim{{
			Text:  "No ownership, expertise, or incident signals exist for payments-api.",
			Layer: knowledge.LayerFact,
			Paths: []string{"FindOwner/FindExperts/FindRelatedIncidents → all empty"},
		}}
		return a, []Section{sec}, nil
	}
	if len(owners) > 0 {
		sec.Rows = append(sec.Rows, []string{"Owner", owners[0].Name + " (" + owners[0].Key + ")", "current OWNS edge"})
	}
	if experts != nil && len(experts.Experts) > 0 {
		sec.Rows = append(sec.Rows, []string{
			"Top expert", top.Name + " (" + top.Key + ")",
			fmt.Sprintf("expertise/v1 score %.2f, owner=%v", top.Score, top.CurrentOwner),
		})
	}
	if handlers != "" {
		sec.Rows = append(sec.Rows, []string{"Recent handlers", handlers, fmt.Sprintf("%d prior incident(s)", len(incidents))})
	}
	notes := []string{"why " + recommend + " (" + recKey + "): " + strings.Join(reasons, ", ")}
	for _, p := range paths {
		notes = append(notes, p)
	}
	sec.Notes = notes

	claims = append(claims, knowledge.Claim{
		Text:  fmt.Sprintf("Route payments-api issues to %s (%s).", recommend, recKey),
		Layer: knowledge.LayerRecommendation,
		Paths: paths,
	})
	a.Answer = fmt.Sprintf("Route a payments-api issue to %s (%s): %s.",
		recommend, recKey, strings.Join(reasons, ", "))
	a.Claims = claims
	return a, []Section{sec}, nil
}

func handlerNames(incidents []graph.IncidentRef) string {
	seen := map[string]bool{}
	var names []string
	for _, inc := range incidents {
		for _, p := range inc.ResolvedBy {
			if !seen[p.Name] {
				seen[p.Name] = true
				names = append(names, p.Name)
			}
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func basisPaths(e knowledge.Expert) []string {
	var paths []string
	seen := map[string]bool{}
	for _, b := range e.Basis {
		if b.Path != "" && !seen[b.Path] {
			seen[b.Path] = true
			paths = append(paths, b.Path)
		}
	}
	return paths
}

func expertNames(experts []knowledge.Expert) string {
	var names []string
	for _, e := range experts {
		names = append(names, e.Name)
	}
	return strings.Join(names, ", ")
}

func signalsLine(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, " · ")
}

func weightsLine(m map[string]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%g", k, m[k]))
	}
	return strings.Join(parts, " · ")
}

func provLine(key string, p knowledge.Provenance) string {
	s := fmt.Sprintf("%s ← %s/%s · observed %s · %s", key, p.SrcType, p.SrcRef, p.ObservedAt, p.Extraction)
	if p.ValidFrom != "" {
		s += " · valid from " + p.ValidFrom
	}
	if p.ValidTo != "" {
		s += " · valid to " + p.ValidTo
	}
	return s
}
