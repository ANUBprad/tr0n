package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

func TestOllamaAdapter(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		status        int
		done, valid   bool
	}{
		{"claim", `{"action":"answer","claims":[]}`, 200, true, true},
		{"tool", `{"action":"retrieve","entity":"payments-api","relations":["OWNS"]}`, 200, true, true},
		{"mixed", `{"calls":[{"name":"assemble_context","args":{}}],"claims":[{"text":"bad"}]}`, 200, true, false},
		{"trailing", `{"action":"answer","claims":[]} {}`, 200, true, false},
		{"unknown", `{"calls":[],"claims":[],"secret":"ignored"}`, 200, true, false},
		{"incomplete", `{}`, 200, false, false},
		{"http failure", `{}`, 503, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/chat" {
					t.Error(r.URL.Path)
				}
				var req map[string]interface{}
				if json.NewDecoder(r.Body).Decode(&req) != nil {
					t.Error("bad request JSON")
				}
				if req["stream"] != false || req["format"] == nil {
					t.Error("missing bounded structured request")
				}
				w.WriteHeader(tc.status)
				json.NewEncoder(w).Encode(map[string]interface{}{"message": map[string]string{"content": tc.content}, "done": tc.done, "prompt_eval_count": 20, "eval_count": 5})
			}))
			defer srv.Close()
			model, _ := NewOllama(srv.URL, "test")
			decide, usage := model.Decider()
			_, err := decide(context.Background(), "test", nil)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.valid && (usage.Calls != 1 || usage.InputTokens != 20) {
				t.Fatalf("usage %+v", usage)
			}
		})
	}
}

func TestGroundedAnswerPath(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_grounded_agent_test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer c.Delete()
	if _, err := seed.Load(c); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, entity, asof, want string
		maxBytes                 int
		bad                      int
	}{
		{name: "current", entity: "payments-api", want: "answered"},
		{name: "unsupported prose replaced", entity: "payments-api", bad: 3, want: "answered"},
		{name: "historical", entity: "payments-api", asof: "2025-02-15T00:00:00Z", want: "answered"},
		{name: "ambiguous", entity: "Adopt", want: "abstained"},
		{name: "missing", entity: "does-not-exist", want: "abstained"},
		{name: "empty", entity: "decision:3", want: "abstained"},
		{name: "over budget", entity: "payments-api", maxBytes: 1, want: "abstained"},
		{name: "invented id", entity: "payments-api", bad: 1, want: "invalid_answer"},
		{name: "invalid later claim", entity: "payments-api", bad: 2, want: "invalid_answer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decide := func(ctx context.Context, system string, turns []Turn) (Decision, error) {
				if len(turns) == 0 {
					raw, _ := json.Marshal(graph.EvidenceRequest{Entity: tc.entity, Relations: []string{"OWNS"}, MaxBytes: tc.maxBytes})
					return Decision{Calls: []Call{{Name: "assemble_context", Args: raw}}}, nil
				}
				ec := turns[0].Result.(graph.EvidenceContext)
				claims := []ModelClaim{}
				if len(ec.Facts) > 0 {
					f := ec.Facts[0]
					claims = append(claims, ModelClaim{Text: f.Neighbor.Name + " owns payments-api.", Subject: f.Entity, Relation: f.Relation, EvidenceIDs: []string{f.ID}})
				}
				if tc.bad == 1 {
					claims[0].EvidenceIDs = []string{"invented"}
				}
				if tc.bad == 2 {
					claims = append(claims, ModelClaim{Text: "must never escape", Subject: "x"})
				}
				if tc.bad == 3 {
					claims[0].Text = "Ada is CEO and earns a billion dollars."
				}
				raw, _ := json.Marshal(map[string]interface{}{"claims": claims})
				return Decision{Answer: string(raw)}, nil
			}
			result, err := GroundedAsk(context.Background(), c, GroundedRequest{Question: "Who owns payments-api?", AsOf: tc.asof}, decide)
			if err != nil || result.Status != tc.want {
				t.Fatalf("%+v %v", result, err)
			}
			if tc.want == "answered" {
				if strings.Contains(result.Grounded.Answer.Answer, "CEO") {
					t.Fatal("model prose escaped relationship rendering")
				}
				if err := result.Grounded.Validate(); err != nil {
					t.Fatal(err)
				}
				want := "Ada"
				if tc.asof != "" {
					want = "Grace"
				}
				if !strings.Contains(result.Grounded.Answer.Answer, want) {
					t.Fatal(result.Grounded.Answer.Answer)
				}
			} else if len(result.Grounded.Answer.Claims) > 0 {
				t.Fatal("unvalidated claims leaked")
			}
		})
	}
}

// TestGroundedReportPhaseTiming locks the per-phase timing contract: every
// return path publishes a request id and monotonic phase durations, nested
// phases never exceed the end-to-end total, and turns counts actual provider
// calls (guard short-circuits are not model calls).
func TestGroundedReportPhaseTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_grounded_timing_test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer c.Delete()
	if _, err := seed.Load(c); err != nil {
		t.Fatal(err)
	}

	t.Run("answered records graph, model, validate, render, turns", func(t *testing.T) {
		decides := 0
		decide := func(ctx context.Context, system string, turns []Turn) (Decision, error) {
			decides++
			time.Sleep(40 * time.Millisecond) // make generation time observable
			if len(turns) == 0 {
				raw, _ := json.Marshal(graph.EvidenceRequest{Entity: "payments-api", Relations: []string{"OWNS"}})
				return Decision{Calls: []Call{{Name: "assemble_context", Args: raw}}}, nil
			}
			ec := turns[0].Result.(graph.EvidenceContext)
			f := ec.Facts[0]
			raw, _ := json.Marshal(map[string]interface{}{"claims": []ModelClaim{{Text: "x", Subject: f.Entity, Relation: f.Relation, EvidenceIDs: []string{f.ID}}}})
			return Decision{Answer: string(raw)}, nil
		}
		report, err := GroundedAsk(context.Background(), c, GroundedRequest{Question: "Who owns payments-api?"}, decide)
		if err != nil {
			t.Fatal(err)
		}
		if report.Status != "answered" {
			t.Fatalf("status %q", report.Status)
		}
		if report.RequestID == "" {
			t.Fatal("request id missing")
		}
		if report.Turns != decides || report.Turns < 2 {
			t.Fatalf("turns=%d decides=%d", report.Turns, decides)
		}
		if report.ModelMS < 30 {
			t.Fatalf("model_ms not measured: %d", report.ModelMS)
		}
		// Nested phases can never exceed the end-to-end total.
		if report.GraphMS < 0 || report.ValidateMS < 0 || report.RenderMS < 0 ||
			report.GraphMS > report.LatencyMS || report.ModelMS > report.LatencyMS {
			t.Fatalf("nested phases exceed total: total=%d graph=%d model=%d validate=%d render=%d",
				report.LatencyMS, report.GraphMS, report.ModelMS, report.ValidateMS, report.RenderMS)
		}
	})

	t.Run("abstained still records timing", func(t *testing.T) {
		decide := func(ctx context.Context, system string, turns []Turn) (Decision, error) {
			raw, _ := json.Marshal(graph.EvidenceRequest{Entity: "does-not-exist"})
			return Decision{Calls: []Call{{Name: "assemble_context", Args: raw}}}, nil
		}
		report, err := GroundedAsk(context.Background(), c, GroundedRequest{Question: "Who owns this service?"}, decide)
		if err != nil {
			t.Fatal(err)
		}
		if report.Status != "abstained" {
			t.Fatalf("status %q", report.Status)
		}
		if report.RequestID == "" || report.Turns < 1 || report.GraphMS < 0 {
			t.Fatalf("abstention lost timing: %+v", report)
		}
	})
}

// TestLinkFacts pins the chain builder behind withAuthorPath: shortest chain
// over retrieved contexts only, direction-agnostic, nil when nothing connects.
func TestLinkFacts(t *testing.T) {
	contexts := []graph.EvidenceContext{
		{Entity: &graph.NodeRef{Key: "service:x"}, Facts: []graph.EvidenceFact{
			{ID: "h1", Entity: "service:x", Relation: "HAS_REPO", Direction: "out", Neighbor: graph.NodeRef{Key: "repo:x"}},
		}},
		{Entity: &graph.NodeRef{Key: "repo:x"}, Facts: []graph.EvidenceFact{
			{ID: "m1", Entity: "repo:x", Relation: "MODIFIED", Direction: "in", Neighbor: graph.NodeRef{Key: "pr:x"}},
			{ID: "m2", Entity: "repo:x", Relation: "MODIFIED", Direction: "in", Neighbor: graph.NodeRef{Key: "pr:unretrieved"}},
		}},
		{Entity: &graph.NodeRef{Key: "pr:x"}, Facts: []graph.EvidenceFact{
			{ID: "a1", Entity: "pr:x", Relation: "AUTHORED", Direction: "in", Neighbor: graph.NodeRef{Key: "person:x"}},
		}},
	}
	path := linkFacts("service:x", "pr:x", contexts)
	if len(path) != 2 || path[0].ID != "h1" || path[1].ID != "m1" {
		t.Fatalf("shortest retrieved chain: %+v", path)
	}
	// A hop to a node that was never retrieved as its own context is not taken,
	// and the chain stops rather than continuing past the endpoint.
	if linkFacts("service:x", "person:x", contexts) != nil {
		t.Fatal("chain must stop at retrieved contexts")
	}
	if linkFacts("service:x", "service:x", contexts) != nil {
		t.Fatal("an endpoint with no chain adds nothing")
	}
	decision := []graph.EvidenceContext{
		{Entity: &graph.NodeRef{Key: "decision:3"}, Facts: []graph.EvidenceFact{
			{ID: "s3", Entity: "decision:3", Relation: "SUPPORTED_BY", Direction: "out", Neighbor: graph.NodeRef{Key: "doc:3"}},
		}},
		{Entity: &graph.NodeRef{Key: "doc:3"}, Facts: []graph.EvidenceFact{
			{ID: "a3", Entity: "doc:3", Relation: "AUTHORED", Direction: "in", Neighbor: graph.NodeRef{Key: "person:40"}},
		}},
	}
	// The connecting fact is the one the model dropped, whatever its direction.
	if path := linkFacts("decision:3", "doc:3", decision); len(path) != 1 || path[0].ID != "s3" {
		t.Fatalf("connecting SUPPORTED_BY fact: %+v", path)
	}
}

// TestGroundedAuthorPathStatesTheConnectingRelationship locks the root cause of
// the decision-source-authors failure: when the model cites only the AUTHORED
// endpoint, the answer must still state the retrieved target -> document edge
// that connects it, and that added claim must pass citation validation.
func TestGroundedAuthorPathStatesTheConnectingRelationship(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_grounded_author_path_test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer c.Delete()
	if _, err := seed.Load(c); err != nil {
		t.Fatal(err)
	}
	// Whatever half of the retrieved path the model happens to cite, the answer
	// must state the whole retrieved chain target -> document -> author.
	for _, model := range []string{"author", "support"} {
		t.Run("model cites only the "+model, func(t *testing.T) {
			decide := func(ctx context.Context, system string, turns []Turn) (Decision, error) {
				if len(turns) == 0 {
					raw, _ := json.Marshal(graph.EvidenceRequest{Entity: "decision:3", Relations: []string{"SUPPORTED_BY"}})
					return Decision{Calls: []Call{{Name: "assemble_context", Args: raw}}}, nil
				}
				// Only an AUTHORED edge reaches the final answer once it is
				// retrieved; otherwise the first support fact is cited.
				var all []graph.EvidenceFact
				for _, tn := range turns {
					if ec, ok := tn.Result.(graph.EvidenceContext); ok {
						all = append(all, ec.Facts...)
					}
				}
				var pick *graph.EvidenceFact
				if model == "author" {
					for i := range all {
						if all[i].Relation == "AUTHORED" {
							pick = &all[i]
						}
					}
				}
				if pick == nil {
					for i := range all {
						if all[i].Relation != "AUTHORED" {
							pick = &all[i]
							break
						}
					}
				}
				if pick == nil {
					return Decision{Answer: `{"claims":[]}`}, nil
				}
				raw, _ := json.Marshal(map[string]interface{}{"claims": []ModelClaim{{Text: "x", Subject: pick.Entity, Relation: pick.Relation, EvidenceIDs: []string{pick.ID}}}})
				return Decision{Answer: string(raw)}, nil
			}
			report, err := GroundedAsk(context.Background(), c, GroundedRequest{
				Question: "Which documents support the Redis idempotency decision, and who authored them?",
				Target:   "decision:3",
			}, decide)
			if err != nil {
				t.Fatal(err)
			}
			if report.Status != "answered" {
				t.Fatalf("status %q warnings=%v", report.Status, report.Warnings)
			}
			if err := report.Grounded.Validate(); err != nil {
				t.Fatal(err)
			}
			text := strings.ToLower(report.Grounded.Answer.Answer)
			for _, term := range []string{"supported", "authored"} {
				if !strings.Contains(text, term) {
					t.Fatalf("answer omits %q: %q", term, report.Grounded.Answer.Answer)
				}
			}
		})
	}
}

func TestConflictingOwners(t *testing.T) {
	a := graph.EvidenceFact{Entity: "service:x", Relation: "OWNS", Direction: "in", Neighbor: graph.NodeRef{Key: "person:a"}}
	b := a
	contexts := []graph.EvidenceContext{{Facts: []graph.EvidenceFact{a, b}}}
	if conflictingOwners(contexts) {
		t.Fatal("corroboration treated as conflict")
	}
	b.Neighbor.Key = "person:b"
	contexts[0].Facts[1] = b
	if !conflictingOwners(contexts) {
		t.Fatal("different owners accepted")
	}
}

func TestAuthorTraversalUsesOnlyReturnedNodes(t *testing.T) {
	service := graph.EvidenceContext{Entity: &graph.NodeRef{Key: "service:x", Label: "Service"}, Facts: []graph.EvidenceFact{{Relation: "HAS_REPO", Direction: "out", Neighbor: graph.NodeRef{Key: "repo:x", Label: "Repository"}}}}
	key, relation, ok := nextAuthorRetrieval([]graph.EvidenceContext{service}, "service:x")
	if !ok || key != "repo:x" || relation != "MODIFIED" {
		t.Fatalf("%s %s %v", key, relation, ok)
	}
	repo := graph.EvidenceContext{Entity: &graph.NodeRef{Key: "repo:x", Label: "Repository"}, Facts: []graph.EvidenceFact{{Entity: "repo:x", Relation: "MODIFIED", Direction: "in", Neighbor: graph.NodeRef{Key: "pr:x", Label: "PullRequest"}}}}
	key, relation, ok = nextAuthorRetrieval([]graph.EvidenceContext{service, repo}, "service:x")
	if !ok || key != "pr:x" || relation != "AUTHORED" {
		t.Fatalf("%s %s %v", key, relation, ok)
	}
	// A decision reaches its authors through SUPPORTED_BY source documents.
	decision := graph.EvidenceContext{Entity: &graph.NodeRef{Key: "decision:3", Label: "Decision"}, Facts: []graph.EvidenceFact{{Entity: "decision:3", Relation: "SUPPORTED_BY", Direction: "out", Neighbor: graph.NodeRef{Key: "doc:3", Label: "Document"}}}}
	key, relation, ok = nextAuthorRetrieval([]graph.EvidenceContext{decision}, "decision:3")
	if !ok || key != "doc:3" || relation != "AUTHORED" {
		t.Fatalf("%s %s %v", key, relation, ok)
	}
	// Once an AUTHORED edge is in evidence the traversal stops - it never walks
	// past the destination or invents another hop.
	doc := graph.EvidenceContext{Entity: &graph.NodeRef{Key: "doc:3", Label: "Document"}, Facts: []graph.EvidenceFact{{Entity: "doc:3", Relation: "AUTHORED", Direction: "in", Neighbor: graph.NodeRef{Key: "person:40", Label: "Person"}}}}
	if key, relation, ok := nextAuthorRetrieval([]graph.EvidenceContext{decision, doc}, "decision:3"); ok {
		t.Fatalf("traversal continued past AUTHORED: %s %s", key, relation)
	}
	if !asksForAuthorship("Who authored this pull request?") || asksForAuthorship("What repository is linked?") {
		t.Fatal("author intent classification")
	}
}

// TestProgressGuardPlan locks the deterministic advance/abstain decision without
// a graph or a model: an author question follows observed edges, a non-author
// question broadens the resolved entity to a new scope, a non-ok context is
// never broadened, and nothing is returned once every move is exhausted.
func TestProgressGuardPlan(t *testing.T) {
	decision := graph.EvidenceContext{Status: graph.EvidenceStatusOK, Entity: &graph.NodeRef{Key: "decision:3", Label: "Decision"}, Facts: []graph.EvidenceFact{{Entity: "decision:3", Relation: "SUPPORTED_BY", Direction: "out", Neighbor: graph.NodeRef{Key: "doc:3", Label: "Document"}}}}

	entity, relations, found := nextProgressRetrieval([]graph.EvidenceContext{decision}, "decision:3", "Who authored the sources of this decision?", map[string]bool{})
	if !found || entity != "doc:3" || len(relations) != 1 || relations[0] != "AUTHORED" {
		t.Fatalf("author continuation: %q %v %v", entity, relations, found)
	}
	entity, relations, found = nextProgressRetrieval([]graph.EvidenceContext{decision}, "decision:3", "Which documents support this decision?", map[string]bool{})
	if !found || entity != "decision:3" || relations != nil {
		t.Fatalf("broaden fallback: %q %v %v", entity, relations, found)
	}
	if _, _, found := nextProgressRetrieval([]graph.EvidenceContext{{Status: graph.EvidenceStatusNotFound}}, "", "Who owns this service?", map[string]bool{}); found {
		t.Fatal("a non-ok context must never be broadened")
	}
	if _, _, found := nextProgressRetrieval([]graph.EvidenceContext{decision}, "decision:3", "Which documents support this decision?", map[string]bool{"decision:3\x00": true}); found {
		t.Fatal("an exhausted scope must not be retried")
	}
	if _, _, found := nextProgressRetrieval([]graph.EvidenceContext{decision}, "decision:3", "Who authored the sources?", map[string]bool{"doc:3\x00AUTHORED": true}); !found {
		t.Fatal("an exhausted author hop should fall back to broadening once")
	}
	if _, _, found := nextProgressRetrieval([]graph.EvidenceContext{decision}, "decision:3", "Who authored the sources?", map[string]bool{"doc:3\x00AUTHORED": true, "decision:3\x00": true}); found {
		t.Fatal("nothing remains once every move is exhausted")
	}
}
