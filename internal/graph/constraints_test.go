package graph

import "testing"

func TestDatabaseUniquenessAndStrictKey(t *testing.T) {
	c := newTestClient(t)
	if err := c.EnsureConstraints(); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureConstraints(); err != nil {
		t.Fatal(err)
	}
	_, err := c.g.Query(`CREATE (:Document {key:'doc:1',title:'missing:key'})`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.g.Query(`CREATE (:Document {key:'doc:1'})`, nil, nil); err == nil {
		t.Fatal("database accepted duplicate key")
	}
	r, err := c.ResolveEntity("key=missing:key", "Document")
	if err != nil || r.Key != "" {
		t.Fatalf("strict missing key fell through: %+v %v", r, err)
	}
	r, err = c.ResolveEntity("missing:key", "Document")
	if err != nil || r.Key != "doc:1" {
		t.Fatalf("colon title regressed: %+v %v", r, err)
	}
}
