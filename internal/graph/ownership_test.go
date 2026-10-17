package graph

import (
	"testing"
	"time"
)

func TestOwnershipUpdateRetainsHistory(t *testing.T) {
	c := newTestClient(t)
	if _, err := c.g.Query(`CREATE (:Service {key:'service:test',name:'test'}),(:Person {key:'person:a',name:'A'}),(:Person {key:'person:b',name:'B'})`, nil, nil); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	handoff := start.Add(time.Hour)
	if ok, err := c.RecordOwner("service:test", "person:a", "registry/v1", start); err != nil || !ok {
		t.Fatalf("initial: %v %v", ok, err)
	}
	if ok, err := c.RecordOwner("service:test", "person:b", "registry/v2", handoff); err != nil || !ok {
		t.Fatalf("handoff: %v %v", ok, err)
	}
	if ok, err := c.RecordOwner("service:test", "person:b", "registry/v2", handoff); err != nil || ok {
		t.Fatalf("repeat should be idempotent: %v %v", ok, err)
	}
	for _, tc := range []struct {
		at  time.Time
		key string
	}{{handoff.Add(-time.Second), "person:a"}, {handoff, "person:b"}} {
		owners, err := c.FindOwner("service:test", tc.at)
		if err != nil || len(owners) != 1 || owners[0].Key != tc.key {
			t.Fatalf("at %s: %+v %v", tc.at, owners, err)
		}
		if tc.key == "person:a" && owners[0].Provenance.ValidTo == "" {
			t.Fatal("lost historical valid_to")
		}
	}
	if _, err := c.RecordOwner("service:test", "person:a", "backdated", start); err == nil {
		t.Fatal("accepted backdated mutation")
	}
	if _, err := c.RecordOwner("service:test", "person:missing", "invalid", handoff); err == nil {
		t.Fatal("accepted absent owner")
	}
}

func TestCurrentOwnerIsNotFutureOwner(t *testing.T) {
	c := newTestClient(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, err := c.g.Query(`CREATE (s:Service {key:'service:test',name:'test'}),
	 (a:Person {key:'person:a',name:'A'}),(b:Person {key:'person:b',name:'B'}),
	 (a)-[:OWNS {valid_from:$past,valid_to:$future}]->(s),
	 (b)-[:OWNS {valid_from:$future}]->(s)`, map[string]interface{}{"past": now.Add(-time.Hour).Format(time.RFC3339), "future": now.Add(time.Hour).Format(time.RFC3339)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.FindOwner("service:test")
	if err != nil || len(got) != 1 || got[0].Key != "person:a" {
		t.Fatalf("future owner leaked into current state: %+v %v", got, err)
	}
}
