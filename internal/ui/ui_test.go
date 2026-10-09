package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

// newSeededServer boots the page over the synthetic graph and asserts
// it answers like the demo promises (DEMO_SPEC scenarios 1–5).
func newSeededServer(t *testing.T) *httptest.Server {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_ui_test")
	if err != nil {
		t.Fatalf("falkordb unreachable at %s — run `docker compose up -d`: %v", addr, err)
	}
	t.Cleanup(func() { c.Delete(); c.Close() })
	if _, err := seed.Load(c); err != nil {
		t.Fatalf("seed load: %v", err)
	}
	srv := httptest.NewServer(Handler(c))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return resp.StatusCode, string(body)
}

// Index lists all five built-in questions (the no-LLM demo path).
func TestIndex(t *testing.T) {
	srv := newSeededServer(t)
	status, body := get(t, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / = %d", status)
	}
	for _, id := range []string{"owner", "experts", "decision", "incidents", "route"} {
		if !strings.Contains(body, "?q="+id) {
			t.Errorf("index missing question link %q", id)
		}
	}
	if !strings.Contains(body, "Five deterministic questions") {
		t.Error("index missing home copy")
	}
}

// Every question answers from the graph, and every rendered answer
// carries tags + evidence (Validate ran server-side; "answer failed
// validation" would be a 500, caught by the status check).
func TestQuestions(t *testing.T) {
	srv := newSeededServer(t)
	tests := []struct {
		id   string
		want []string
	}{
		{"owner", []string{"person:1", "OWNS", "FACT", "payments-api"}},
		{"experts", []string{"expertise/v1", "INFERENCE", "person:1", "Ranked experts"}},
		{"decision", []string{"Adopt FalkorDB", "decision:2", "SUPPORTED_BY", "AUTHORED"}},
		{"incidents", []string{"timeout", "AFFECTS", "sev", "FACT"}},
		{"route", []string{"RECOMMENDATION", "person:1", "Route a payments-api issue"}},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			status, body := get(t, srv.URL+"/?q="+tc.id)
			if status != http.StatusOK {
				t.Fatalf("?q=%s = %d: %s", tc.id, status, body)
			}
			if strings.Contains(body, "answer failed validation") {
				t.Fatal("composer produced an invalid answer")
			}
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Errorf("?q=%s missing %q", tc.id, want)
				}
			}
		})
	}
}

func TestBadRequests(t *testing.T) {
	srv := newSeededServer(t)
	if status, _ := get(t, srv.URL+"/?q=nope"); status != http.StatusBadRequest {
		t.Errorf("unknown question = %d, want 400", status)
	}
	if status, _ := get(t, srv.URL+"/other"); status != http.StatusNotFound {
		t.Errorf("unknown path = %d, want 404", status)
	}
}

// A graph failure must not reach the browser: 500 with a generic body,
// detail kept server-side. A dead address makes it deterministic.
func TestInternalErrorsAreSanitized(t *testing.T) {
	c, _ := graph.New("localhost:1", "tron_ui_test")
	srv := httptest.NewServer(Handler(c))
	defer srv.Close()
	status, body := get(t, srv.URL+"/?q=owner")
	if status != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", status, body)
	}
	for _, s := range []string{"connection refused", "dial tcp", "graph error", "find owner"} {
		if strings.Contains(body, s) {
			t.Errorf("response leaked internal detail %q: %s", s, body)
		}
	}
	if !strings.Contains(body, "internal error") {
		t.Errorf("want generic message, got %s", body)
	}
}
