package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/graph"
)

func TestTenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires FalkorDB")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	for _, tenant := range []string{"tron_tenant_a_test", "tron_tenant_b_test"} {
		c, err := graph.New(addr, tenant)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Reset(); err != nil {
			t.Fatal(err)
		}
		err = c.LoadNodes("Person", []map[string]interface{}{{"key": "person:1", "name": tenant, "src_type": "synthetic", "src_ref": tenant, "observed_at": "2026-10-16T00:00:00Z", "extraction": "test"}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Delete(); c.Close() })
	}
	config := []Credential{{Token: strings.Repeat("a", 32), Graph: "tron_tenant_a_test", Actor: "reader-a", Role: "reader"}, {Token: strings.Repeat("b", 32), Graph: "tron_tenant_b_test", Actor: "reader-b", Role: "reader"}}
	b, _ := json.Marshal(config)
	h, closeAll, err := TenantHandler(addr, strings.NewReader(string(b)), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	for _, tc := range []struct {
		token        string
		status       int
		want, forbid string
	}{{"", 401, "", ""}, {"bad", 401, "", ""}, {strings.Repeat("a", 32), 200, "tron_tenant_a_test", "tron_tenant_b_test"}, {strings.Repeat("b", 32), 200, "tron_tenant_b_test", "tron_tenant_a_test"}} {
		r := httptest.NewRequest("GET", "/v1/catalog?graph=tron_tenant_b_test", nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.want) || (tc.forbid != "" && strings.Contains(w.Body.String(), tc.forbid)) {
			t.Fatalf("tenant leakage/status: %d %s", w.Code, w.Body.String())
		}
	}
}
