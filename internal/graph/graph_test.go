package graph

import (
	"os"
	"testing"
)

const testGraph = "tron_test"

// The one runnable check for FindOwner: provenance round-trip,
// temporal filtering (stale edges excluded), key lookup, not-found.
// Runs against a real FalkorDB — `docker compose up -d` first.
func TestFindOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := New(addr, testGraph)
	if err != nil {
		t.Fatalf("falkordb unreachable at %s — run `docker compose up -d`: %v", addr, err)
	}
	t.Cleanup(func() {
		c.g.Delete()
		c.Close()
	})

	seed := `
	CREATE (ada:Person {key: 'person:ada', name: 'Ada'}),
	       (bob:Person {key: 'person:bob', name: 'Bob'}),
	       (svc:Service {key: 'service:payments', name: 'payments-api'}),
	       (ada)-[:OWNS {src_type: 'service_registry', src_ref: 'registry.json',
	                     observed_at: '2026-10-08T00:00:00Z', extraction: 'deterministic',
	                     valid_from: '2026-02-01', valid_to: null}]->(svc),
	       (bob)-[:OWNS {src_type: 'service_registry', src_ref: 'registry.json',
	                     observed_at: '2026-10-08T00:00:00Z', extraction: 'deterministic',
	                     valid_from: '2024-01-01', valid_to: '2026-02-01'}]->(svc)`
	if _, err := c.g.Query(seed, nil, nil); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	t.Run("current owner by name with provenance", func(t *testing.T) {
		owners, err := c.FindOwner("payments-api")
		if err != nil {
			t.Fatal(err)
		}
		if len(owners) != 1 {
			t.Fatalf("want 1 current owner (Bob's superseded edge excluded), got %d: %+v", len(owners), owners)
		}
		o := owners[0]
		if o.Name != "Ada" || o.Key != "person:ada" || o.Kind != "Person" {
			t.Errorf("wrong owner: %+v", o)
		}
		p := o.Provenance
		if p.SrcType != "service_registry" || p.SrcRef != "registry.json" ||
			p.ObservedAt != "2026-10-08T00:00:00Z" || p.Extraction != "deterministic" ||
			p.ValidFrom != "2026-02-01" {
			t.Errorf("provenance not carried through: %+v", p)
		}
	})

	t.Run("lookup by key", func(t *testing.T) {
		owners, err := c.FindOwner("service:payments")
		if err != nil {
			t.Fatal(err)
		}
		if len(owners) != 1 || owners[0].Key != "person:ada" {
			t.Errorf("want Ada by key, got %+v", owners)
		}
	})

	t.Run("unknown target is empty, not an error", func(t *testing.T) {
		owners, err := c.FindOwner("no-such-thing")
		if err != nil {
			t.Fatal(err)
		}
		if len(owners) != 0 {
			t.Errorf("want empty result, got %+v", owners)
		}
	})
}
