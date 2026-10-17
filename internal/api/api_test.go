package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/actions"
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

	t.Run("ask", func(t *testing.T) {
		answer := post(t, "/v1/ask", `{"question":"Why did we choose Redis over Kafka?"}`, http.StatusOK)
		if !strings.Contains(answer["answer"].(string), "24-hour") || answer["intent"] != "decision" {
			t.Fatalf("bad question response: %v", answer)
		}
		post(t, "/v1/ask", `{"question":""}`, http.StatusBadRequest)
		post(t, "/v1/ask", `{"question":"Who owns it?","as_of":"yesterday"}`, http.StatusBadRequest)
		post(t, "/v1/ask", `{"question":"Why Redis?","as_of":"2026-01-01T00:00:00Z"}`, http.StatusBadRequest)
		post(t, "/v1/ask", `{"question":"Who owns it?","unexpected":1}`, http.StatusBadRequest)
	})
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
		// as_of returns the superseded owner (seed: person:2 owned
		// payments-api before person:1) — the E5 temporal contract.
		hist := postList(t, "/v1/owner",
			`{"target":"payments-api","as_of":"2025-02-15T00:00:00Z"}`, http.StatusOK)
		if len(hist) != 1 || hist[0].(map[string]interface{})["key"] != "person:2" {
			t.Errorf("as_of owner must be the superseded one: %v", hist)
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
		// as_of judges ownership at the instant: in Feb 2025 person:2
		// was the owner, so it ranks first then (now still drives decay).
		histRes := post(t, "/v1/experts",
			`{"service_key":"service:payments-api","limit":5,"now":"2026-10-08T12:00:00Z","as_of":"2025-02-15T00:00:00Z"}`,
			http.StatusOK)
		histExperts, _ := histRes["experts"].([]interface{})
		if len(histExperts) == 0 {
			t.Fatalf("as_of experts: empty")
		}
		histTop := histExperts[0].(map[string]interface{})
		if histTop["key"] != "person:2" || histTop["current_owner"] != true {
			t.Errorf("as_of owner must rank first: %v", histTop)
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

	t.Run("conflicts", func(t *testing.T) {
		// Baseline: the shipped seed has no contradictory state claims.
		res := post(t, "/v1/conflicts", `{}`, http.StatusOK)
		if res["layer"] != "INFERENCE" || res["rule_id"] != "contradiction/v1" {
			t.Errorf("tagging: %v", res)
		}
		if confs, _ := res["conflicts"].([]interface{}); len(confs) != 0 {
			t.Fatalf("baseline seed must have zero conflicts, got %v", confs)
		}
		// Inject one conflicting OWNS claim: person:1 already owns
		// payments-api, person:2 claims it over an overlapping window.
		if err := c.LoadEdges("OWNS", []graph.EdgeRow{{
			From: "person:2", To: "service:payments-api",
			Props: map[string]interface{}{
				"src_type": "service_registry", "src_ref": "test/conflict",
				"observed_at": "2026-10-01T00:00:00Z", "extraction": "deterministic",
				"valid_from": "2026-01-01T00:00:00Z",
			},
		}}); err != nil {
			t.Fatalf("inject conflict: %v", err)
		}
		flagged := post(t, "/v1/conflicts", `{}`, http.StatusOK)
		confs, _ := flagged["conflicts"].([]interface{})
		if len(confs) != 1 {
			t.Fatalf("want 1 conflict, got %v", confs)
		}
		g := confs[0].(map[string]interface{})
		if g["relation"] != "OWNS" || g["slot"] != "service:payments-api" {
			t.Errorf("group: %v", g)
		}
		if edges, _ := g["edges"].([]interface{}); len(edges) != 2 {
			t.Errorf("both claimants must surface: %v", g["edges"])
		}
		// A non-state relation filter is a client error, not a 500.
		post(t, "/v1/conflicts", `{"relation":"KNOWS"}`, http.StatusBadRequest)
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
		post(t, "/v1/owner",
			`{"target":"payments-api","as_of":"last tuesday"}`, http.StatusBadRequest)
		post(t, "/v1/experts",
			`{"service_key":"service:payments-api","as_of":"yesterday"}`, http.StatusBadRequest)
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

// Internal failures must not reach the client: 500 with a stable body,
// with the diagnostic kept server-side. A dead address makes the failure
// deterministic (connection refused) without needing a live database.
func TestInternalErrorsAreSanitized(t *testing.T) {
	c, _ := graph.New("localhost:1", "tron_api_test")
	srv := httptest.NewServer(Handler(c))
	defer srv.Close()

	get := func(path string) (int, string) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	post := func(body string) (int, string) {
		t.Helper()
		resp, err := http.Post(srv.URL+"/v1/owner", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	// Unexpected graph failure → generic 500, no internal detail.
	if code, body := post(`{"target":"x"}`); code != http.StatusInternalServerError {
		t.Fatalf("failure: got %d, want 500: %s", code, body)
	} else if leaks(body) {
		t.Errorf("500 body leaked internal error: %s", body)
	} else if !strings.Contains(body, "internal error") {
		t.Errorf("500 body not the generic message: %s", body)
	}

	// Health failure → 503 retained, no internal detail.
	if code, body := get("/v1/health"); code != http.StatusServiceUnavailable {
		t.Fatalf("health: got %d, want 503: %s", code, body)
	} else if leaks(body) {
		t.Errorf("health body leaked internal error: %s", body)
	}

	// Client validation error keeps its 4xx status and safe message.
	if code, body := post(`{}`); code != http.StatusBadRequest {
		t.Fatalf("validation: got %d, want 400: %s", code, body)
	} else if !strings.Contains(body, "target is required") {
		t.Errorf("validation message lost: %s", body)
	}
}

// leaks reports whether a response body contains driver/query internals
// that must stay server-side.
func leaks(body string) bool {
	for _, s := range []string{"connection refused", "dial tcp", "find owner", "REDIS", "redis"} {
		if strings.Contains(body, s) {
			return true
		}
	}
	return false
}

func TestPlanAlertsActionsEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_api_plan_test")
	if err != nil {
		t.Fatalf("falkordb unreachable at %s: %v", addr, err)
	}
	t.Cleanup(func() { c.Delete(); c.Close() })
	if _, err := seed.Load(c); err != nil {
		t.Fatalf("seed load: %v", err)
	}
	srv := httptest.NewServer(Handler(c))
	t.Cleanup(srv.Close)

	// 1. POST /v1/plan
	planBody := `{"context":"Idempotency store selection","options":[{"id":"opt:1","name":"Redis","cost_score":0.2,"risk_score":0.2,"perf_score":0.9,"ease_score":0.8}]}`
	resp, err := http.Post(srv.URL+"/v1/plan", "application/json", strings.NewReader(planBody))
	if err != nil {
		t.Fatalf("POST /v1/plan: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST /v1/plan status = %d, want 200", resp.StatusCode)
	}
	var planOut struct {
		RuleID        string `json:"rule_id"`
		RecommendedID string `json:"recommended_id"`
	}
	json.NewDecoder(resp.Body).Decode(&planOut)
	if planOut.RuleID != "decision_eval/v1" || planOut.RecommendedID != "opt:1" {
		t.Errorf("unexpected plan output: %+v", planOut)
	}

	// 2. GET /v1/alerts
	respAlerts, err := http.Get(srv.URL + "/v1/alerts")
	if err != nil {
		t.Fatalf("GET /v1/alerts: %v", err)
	}
	defer respAlerts.Body.Close()
	if respAlerts.StatusCode != http.StatusOK {
		t.Errorf("GET /v1/alerts status = %d, want 200", respAlerts.StatusCode)
	}

	// Action routes require server-owned role credentials; the default handler has none.
	for _, path := range []string{"/v1/actions/propose", "/v1/actions/approve", "/v1/actions/execute"} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("POST %s status = %d, want 403 without configured role credentials", path, resp.StatusCode)
		}
	}
}

func TestActionRoutesUseAuthenticatedRoles(t *testing.T) {
	h := Handler(nil)
	call := func(role string, path string, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if role != "" {
			r = r.WithContext(context.WithValue(r.Context(), principalKey{}, Credential{Actor: "server:actor", Role: role}))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if got := call("", "/v1/actions/propose", `{}`).Code; got != http.StatusForbidden {
		t.Fatalf("unauthenticated propose status=%d, want 403", got)
	}
	if got := call("reader", "/v1/actions/propose", `{}`).Code; got != http.StatusForbidden {
		t.Fatalf("reader propose status=%d, want 403", got)
	}
	if got := call("requester", "/v1/actions/propose", `{"action_type":"draft_ticket","target":"incident:1","parameters":{},"principal":"attacker:forged"}`).Code; got != http.StatusBadRequest {
		t.Fatalf("caller-supplied principal should be rejected, got %d", got)
	}
	proposed := call("requester", "/v1/actions/propose", `{"action_type":"draft_ticket","target":"incident:1","parameters":{"body":"Draft"}}`)
	if proposed.Code != http.StatusOK {
		t.Fatalf("requester propose status=%d, want 200: %s", proposed.Code, proposed.Body)
	}
	var result struct {
		Proposal actions.ProposedAction `json:"proposal"`
	}
	if err := json.Unmarshal(proposed.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Proposal.Principal != "server:actor" {
		t.Fatalf("proposal principal=%q, want server-owned identity", result.Proposal.Principal)
	}
	if got := call("requester", "/v1/actions/execute", `{}`).Code; got != http.StatusForbidden {
		t.Fatalf("requester execute status=%d, want 403", got)
	}
}
