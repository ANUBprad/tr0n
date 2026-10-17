package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The decision prompt is strict JSON selection from already-retrieved facts, so
// a qwen3-style thinking trace is pure output-token latency. This pins the
// request body so the optimization (docs/LATENCY_REPORT.md) cannot silently
// regress.
func TestOllamaRequestDisablesThinking(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		blob, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(blob, &got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"message":{"content":"{\"action\":\"retrieve\",\"entity\":\"service:payments-api\",\"relations\":[\"OWNS\"]}"},"done":true,"prompt_eval_count":10,"eval_count":5}`)
	}))
	defer srv.Close()

	model, err := NewOllama(srv.URL, "qwen3:4b")
	if err != nil {
		t.Fatal(err)
	}
	decide, usage := model.Decider()
	if _, err := decide(context.Background(), "system prompt", nil); err != nil {
		t.Fatalf("decide: %v", err)
	}

	think, ok := got["think"]
	if !ok {
		t.Fatal("request body omitted 'think'")
	}
	if think != false {
		t.Fatalf("think = %v, want false", think)
	}
	if _, ok := got["format"]; !ok {
		t.Fatal("request body omitted the grounded 'format' schema")
	}
	if usage.Calls != 1 || usage.OutputTokens != 5 {
		t.Fatalf("usage not recorded: %+v", *usage)
	}
}
