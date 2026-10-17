package api

import (
	"encoding/json"
	"github.com/ANUBprad/tr0n/internal/agent"
	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGroundedEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_grounded_api_test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer c.Delete()
	if _, err := seed.Load(c); err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		content := `{"action":"answer","claims":[]}`
		if !strings.Contains(req.Messages[1].Content, `"result"`) {
			content = `{"action":"retrieve","entity":"payments-api","relations":["OWNS"]}`
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"done": true, "message": map[string]string{"content": content}})
	}))
	defer provider.Close()
	model, _ := agent.NewOllama(provider.URL, "test")
	h := HandlerWithModel(c, model)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"question":"Who owns payments-api?"}`, 200},
		{`{"question":""}`, 400},
		{`{"question":"owner?","as_of":"yesterday"}`, 400},
		{`{"question":"owner?","graph":"another_tenant"}`, 400},
		{`{"question":"owner?","url":"http://elsewhere"}`, 400},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/grounded", strings.NewReader(tc.body))
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		if tc.status == 200 && !strings.Contains(w.Body.String(), `"status":"abstained"`) {
			t.Fatal(w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	Handler(c).ServeHTTP(w, httptest.NewRequest("POST", "/v1/grounded", strings.NewReader(`{"question":"owner?"}`)))
	if w.Code != 503 {
		t.Fatal("missing provider not explicit")
	}
}
