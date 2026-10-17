package graph

import "testing"

func TestImportRecordIdempotencyAndUpdates(t *testing.T) {
	c := newTestClient(t)
	_, err := c.g.Query(`CREATE (:Person {key:'person:a',name:'A'}),(:Service {key:'service:a',name:'A'}),(:Service {key:'service:b',name:'B'})`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := SourceRecord{Key: "docs:runbook", Title: "Runbook", Body: "Check the ledger", Kind: "runbook", AuthorKey: "person:a", AboutKey: "service:a", SourceRef: "synthetic/runbook", ObservedAt: "2026-10-16T10:00:00Z"}
	if changed, err := c.ImportRecord(r); err != nil || !changed {
		t.Fatalf("first import: %v %v", changed, err)
	}
	if changed, err := c.ImportRecord(r); err != nil || changed {
		t.Fatalf("retry: %v %v", changed, err)
	}
	if n := edgeCount(t, c); n != 2 {
		t.Fatalf("retry duplicated relationships: %d", n)
	}
	old := r
	r.Body = "Check the ledger before retrying"
	r.AboutKey = "service:b"
	r.ObservedAt = "2026-10-16T11:00:00Z"
	if changed, err := c.ImportRecord(r); err != nil || !changed {
		t.Fatalf("update: %v %v", changed, err)
	}
	if n := edgeCount(t, c); n != 2 {
		t.Fatalf("update left stale links: %d", n)
	}
	ev, err := c.TraceEvidence([]string{r.Key})
	if err != nil {
		t.Fatal(err)
	}
	if ev[0].Properties["body"] != r.Body {
		t.Fatalf("body not updated: %+v", ev)
	}
	for _, edge := range ev[0].Edges {
		if edge.Relation == "ABOUT" && edge.Neighbor.Key != "service:b" {
			t.Fatalf("stale subject: %+v", edge)
		}
	}
	if _, err := c.ImportRecord(old); err == nil {
		t.Fatal("accepted stale source")
	}
	r.SourceRef = "other/source"
	if _, err := c.ImportRecord(r); err == nil {
		t.Fatal("allowed identity takeover by another source")
	}
	r = old
	r.Key = "docs:missing"
	r.AuthorKey = "person:missing"
	if _, err := c.ImportRecord(r); err == nil {
		t.Fatal("accepted missing author")
	}
}
