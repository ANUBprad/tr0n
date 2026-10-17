package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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
	if !asksForAuthorship("Who authored this pull request?") || asksForAuthorship("What repository is linked?") {
		t.Fatal("author intent classification")
	}
}
