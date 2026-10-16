package graph

import (
	"strings"
	"testing"
	"time"
)

func TestReviewedIdentity(t *testing.T) {
	c := newTestClient(t)
	_, err := c.g.Query(`CREATE (:Person {key:'person:ada',name:'Ada'}),(:Person {key:'person:bob',name:'Ada'})`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	if err := c.RegisterIdentity("github:ada", "person:ada", "reviewed/registry", now); err != nil {
		t.Fatal(err)
	}
	if err := c.RegisterIdentity("github:ada", "person:ada", "reviewed/registry", now); err != nil {
		t.Fatal(err)
	}
	if err := c.RegisterIdentity("github:ada", "person:bob", "reviewed/registry", now); err == nil {
		t.Fatal("silently remapped identity")
	}
	if err := c.RegisterIdentity("person:bob", "person:ada", "reviewed/registry", now); err == nil {
		t.Fatal("destructively merged canonical nodes")
	}
	r, err := c.ResolveEntity("github:ada", "Person")
	if err := c.rejectAliases([]string{"github:ada"}); err == nil {
		t.Fatal("reviewed alias was available for a new canonical node")
	}
	if err := c.LoadNodes("Person", []map[string]interface{}{{"key": "github:ada", "name": "Impostor", "src_type": "test", "src_ref": "test", "observed_at": now.Format(time.RFC3339), "extraction": "test"}}); err == nil {
		t.Fatal("loader overwrote an alias")
	}
	if _, err := c.ImportRecord(SourceRecord{Key: "github:ada", Title: "Alias collision", Body: "Not a new identity", Kind: "note", AuthorKey: "person:ada", AboutKey: "service:x", SourceRef: "test", ObservedAt: now.Format(time.RFC3339)}); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("import did not reserve alias before checking endpoints: %v", err)
	}
	if err != nil || r.Key != "person:ada" || r.MatchedOn != "reviewed_alias" {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = c.ResolveEntity("Ada", "Person")
	if err != nil || len(r.Candidates) != 2 {
		t.Fatal("same-name people falsely merged")
	}
}
