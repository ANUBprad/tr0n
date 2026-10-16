package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

func TestQuestionRouting(t *testing.T) {
	for question, want := range map[string]string{
		"Who owns payments-api and why?": "owner", "Who knows it best?": "experts",
		"Why Redis over Kafka?": "decision", "Has this happened before?": "incidents",
		"Prepare a hand-off for payments": "handoff", "unknown": "", "hello": "",
	} {
		if got := Intent(question); got != want {
			t.Errorf("%q: %q want %q", question, got, want)
		}
	}
	cat := []graph.Candidate{{Key: "service:payments-api", Name: "payments-api", Label: "Service"}, {Key: "service:inventory-api", Name: "inventory-api", Label: "Service"}}
	if got := mentioned("Who owns the Payments service?", "Service", cat); len(got) != 1 || got[0].Key != "service:payments-api" {
		t.Fatalf("payments: %+v", got)
	}
	if got := mentioned("Who owns an API?", "Service", cat); len(got) != 2 {
		t.Fatalf("ambiguity lost: %+v", got)
	}
	r, err := Ask(context.Background(), nil, AskRequest{Question: "hello"})
	if err != nil || len(r.Claims) != 0 || r.Target != nil {
		t.Fatalf("unsupported must abstain without graph claims: %+v %v", r, err)
	}
}

func TestAskLiveGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("needs FalkorDB")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_ask_test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer c.Delete()
	if _, err := seed.Load(c); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ question, target, want string }{
		{"Who owns the Payments service?", "", "Ada Mercer"},
		{"Who knows payments-api best?", "", "expertise/v1"},
		{"Why did we choose Redis over Kafka?", "", "24-hour"},
		{"Has payments-api timed out before?", "", "4 matching"},
		{"Prepare a handoff for payments-api timeout investigation", "", "Ada Mercer"},
		{"Who owns it?", "service:inventory-api", "Margaret Haddad"},
	} {
		t.Run(tc.question+tc.target, func(t *testing.T) {
			r, err := Ask(context.Background(), c, AskRequest{Question: tc.question, Target: tc.target})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(r.Answer.Answer, tc.want) {
				t.Fatalf("want %q: %+v", tc.want, r)
			}
			if err := r.Answer.Validate(); err != nil {
				t.Fatal(err)
			}
			if len(r.Evidence) == 0 || len(r.Claims) == 0 {
				t.Fatal("missing evidence")
			}
			if r.Intent == "handoff" && !strings.Contains(r.Handoff, "DRAFT") {
				t.Fatal("missing handoff draft")
			}
		})
	}
	for _, req := range []AskRequest{{Question: "Who owns an API?"}, {Question: "Who owns missing-service?"}, {Question: "Who owns it?", Target: "service:missing"}} {
		r, err := Ask(context.Background(), c, req)
		if err != nil {
			t.Fatal(err)
		}
		if r.Target != nil || len(r.Claims) > 0 {
			t.Fatalf("must abstain: %+v", r)
		}
	}
	r, err := Ask(context.Background(), c, AskRequest{Question: "Who owned payments-api?", AsOf: "2025-02-15T00:00:00Z"})
	if err != nil || !strings.Contains(r.Answer.Answer, "Grace") {
		t.Fatalf("history: %+v %v", r, err)
	}
	if _, err := Ask(context.Background(), c, AskRequest{Question: "Why Redis?", AsOf: "2025-02-15T00:00:00Z"}); err == nil {
		t.Fatal("decision silently ignored historical filter")
	}
}
