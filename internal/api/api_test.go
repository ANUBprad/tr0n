package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

// The API is the product surface: seed a graph, hit every endpoint
// over real HTTP, assert the JSON contract the agent/UI will consume.
func TestEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_api_test")
	if err != nil {
		t.Fatalf("falkordb unreachable at %s — run `docker compose up -d`: %v", addr, err)
	}
	t.Cleanup(func() { c.Delete(); c.Close() })
	if _, err := seed.Load(c); err != nil {
		t.Fatalf("seed load: %v", err)
	}
	srv := httptest.NewServer(Handler(c))
	t.Cleanup(srv.Close)

	post := func(t *testing.T, path, body string, wantStatus int) map[string]interface{} {
		t.Helper()
		resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != wantStatus {
			t.Errorf("POST %s = %d, want %d", path, resp.StatusCode, wantStatus)
		}
		var out map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("POST %s: decode: %v", path, err)
		}
		return out
	}
	// list variant for endpoints returning arrays
	postList := func(t *testing.T, path, body string, wantStatus int) []interface{} {
		t.Helper()
		resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != wantStatus {
			t.Errorf("POST %s = %d, want %d", path, resp.StatusCode, wantStatus)
		}
		var out []interface{}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("POST %s: decode: %v", path, err)
		}
		return out
	}

	t.Run("owner", func(t *testing.T) {
		owners := postList(t, "/v1/owner", `{"target":"payments-api"}`, http.StatusOK)
		if len(owners) != 1 {
			t.Fatalf("owners: %v", owners)
		}
		o := owners[0].(map[string]interface{})
		if o["key"] != "person:1" || o["name"] != "Ada Mercer" {
			t.Errorf("owner: %v", o)
		}
		prov, _ := o["provenance"].(map[string]interface{})
		if prov["src_type"] != "service_registry" {
			t.Errorf("provenance not shipped: %v", prov)
		}
	})

	t.Run("experts", func(t *testing.T) {
		res := post(t, "/v1/experts",
			`{"service_key":"service:payments-api","limit":5,"now":"2026-10-08T12:00:00Z"}`,
			http.StatusOK)
		if res["layer"] != "INFERENCE" || res["rule_id"] != "expertise/v1" {
			t.Errorf("inference tagging: %v", res)
		}
		experts, _ := res["experts"].([]interface{})
		if len(experts) < 3 {
			t.Fatalf("want >= 3 experts, got %d", len(experts))
		}
		top := experts[0].(map[string]interface{})
		if top["key"] != "person:1" || top["current_owner"] != true {
			t.Errorf("owner must rank first: %v", top)
		}
	})

	t.Run("decision", func(t *testing.T) {
		res := post(t, "/v1/decision", `{"ref":"decision:1"}`, http.StatusOK)
		supported, _ := res["supported_by"].([]interface{})
		discussed, _ := res["discussed_in"].([]interface{})
		supersedes, _ := res["supersedes"].([]interface{})
		if len(supported) < 2 || len(discussed) < 1 || len(supersedes) < 1 {
			t.Errorf("trace: supported=%d discussed=%d supersedes=%d",
				len(supported), len(discussed), len(supersedes))
		}
		post(t, "/v1/decision", `{"ref":"decision:does-not-exist"}`, http.StatusNotFound)
	})

	t.Run("incidents", func(t *testing.T) {
		all := postList(t, "/v1/incidents", `{"service_key":"service:payments-api"}`, http.StatusOK)
		if len(all) != 6 {
			t.Errorf("want 6 incidents, got %d", len(all))
		}
		filtered := postList(t, "/v1/incidents",
			`{"service_key":"service:payments-api","since":"2026-01-01T00:00:00Z","keywords":"timeout"}`,
			http.StatusOK)
		if len(filtered) != 4 {
			t.Errorf("since+timeout: want 4, got %d", len(filtered))
		}
	})

	t.Run("evidence", func(t *testing.T) {
		ev := postList(t, "/v1/evidence", `{"keys":["person:1","ghost:1"]}`, http.StatusOK)
		if len(ev) != 2 {
			t.Fatalf("want 2 entries, got %d", len(ev))
		}
		p := ev[0].(map[string]interface{})
		if p["found"] != true {
			t.Errorf("person:1: %v", p)
		}
		if ev[1].(map[string]interface{})["found"] != false {
			t.Errorf("ghost must be found=false: %v", ev[1])
		}
	})

	t.Run("resolve", func(t *testing.T) {
		res := post(t, "/v1/resolve", `{"name":"payments-api","label":"Service"}`, http.StatusOK)
		if res["key"] != "service:payments-api" {
			t.Errorf("resolve: %v", res)
		}
		// "payment" prefixes team:payments, service:payments-api,
		// payments-engine, and three incident titles.
		ambiguous := post(t, "/v1/resolve", `{"name":"payment"}`, http.StatusOK)
		cands, _ := ambiguous["candidates"].([]interface{})
		if ambiguous["key"] != nil || len(cands) < 2 {
			t.Errorf("ambiguous name must list candidates, not guess: %v", ambiguous)
		}
	})

	t.Run("body hardening", func(t *testing.T) {
		// valid JSON with trailing whitespace still succeeds
		postList(t, "/v1/owner", `{"target":"payments-api"}   `, http.StatusOK)
		// oversized body is reported as a size error, not a syntax error
		post(t, "/v1/owner", `{"target":"`+strings.Repeat("a", maxBodyBytes)+`"}`,
			http.StatusRequestEntityTooLarge)
	})

	t.Run("validation and health", func(t *testing.T) {
		post(t, "/v1/owner", `{}`, http.StatusBadRequest)
		post(t, "/v1/owner", `{"target":"x","typo":1}`, http.StatusBadRequest)
		post(t, "/v1/incidents",
			`{"service_key":"s","since":"yesterday"}`, http.StatusBadRequest)
		post(t, "/v1/evidence", `{"keys":[]}`, http.StatusBadRequest)

		resp, err := http.Get(srv.URL + "/v1/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("health = %d", resp.StatusCode)
		}
	})
}

// The handler must boot without FalkorDB errors on unknown routes
// (404 from the mux) — guards route typos.
func TestUnknownRoute(t *testing.T) {
	c, _ := graph.New("localhost:1", "tron_api_test")
	srv := httptest.NewServer(Handler(c))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/typo", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown route = %d, want 404", resp.StatusCode)
	}
}

// Unsafe bodies are rejected at the handler boundary before any graph
// call, so this needs no FalkorDB — the client is never contacted.
func TestDecodeRejectsUnsafeBodies(t *testing.T) {
	c, _ := graph.New("localhost:1", "tron_api_test")
	srv := httptest.NewServer(Handler(c))
	defer srv.Close()
	post := func(body string) int {
		t.Helper()
		resp, err := http.Post(srv.URL+"/v1/owner", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	oversized := `{"target":"` + strings.Repeat("a", maxBodyBytes) + `"}`
	cases := []struct {
		name string
		body string
		want int
	}{
		{"malformed json", `{"target":`, http.StatusBadRequest},
		{"trailing json value", `{"target":"x"}{}`, http.StatusBadRequest},
		{"trailing garbage", `{"target":"x"} junk`, http.StatusBadRequest},
		{"oversized body", oversized, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		if got := post(tc.body); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
