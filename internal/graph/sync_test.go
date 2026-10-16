package graph

import "testing"

func TestSourceSnapshot(t *testing.T) {
	c := newTestClient(t)
	_, err := c.g.Query(`CREATE (:Person {key:'person:a'}),(:Service {key:'service:a'})`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := SourceRecord{Key: "file:a", Title: "A", Body: "Original", Kind: "note", AuthorKey: "person:a", AboutKey: "service:a", SourceRef: "own/a", ObservedAt: "2026-10-16T00:00:00Z"}
	b := a
	b.Key = "file:b"
	b.SourceRef = "own/b"
	first, err := c.SyncSnapshot("docs", []SourceRecord{a, b})
	if err != nil || first.Changed != 2 || !first.Complete {
		t.Fatalf("%+v %v", first, err)
	}
	again, err := c.SyncSnapshot("docs", []SourceRecord{b, a})
	if err != nil || again.Changed != 0 || again.Revision != first.Revision {
		t.Fatalf("retry %+v %v", again, err)
	}
	if _, err := c.SyncSnapshot("other", []SourceRecord{a}); err == nil {
		t.Fatal("other source took over record")
	}
	if _, err := c.ImportRecord(a); err == nil {
		t.Fatal("manual import took over managed record")
	}
	if _, err := c.SyncSnapshot("docs", []SourceRecord{a, a}); err == nil {
		t.Fatal("duplicate accepted")
	}
	a.Body = "Updated"
	a.ObservedAt = "2026-10-16T01:00:00Z"
	broken := b
	broken.AuthorKey = "person:missing"
	failed, err := c.SyncSnapshot("docs", []SourceRecord{a, broken})
	if err == nil || failed.Complete {
		t.Fatal("partial failure reported complete")
	}
	state, _ := c.g.ROQuery(`MATCH (s:SyncState {source:'docs'}) RETURN s.revision AS revision`, nil, nil)
	state.Next()
	if field(state.Record(), "revision") != first.Revision {
		t.Fatal("failed sync advanced cursor")
	}
	result, err := c.SyncSnapshot("docs", []SourceRecord{a})
	if err != nil || result.Deleted != 1 {
		t.Fatalf("delete %+v %v", result, err)
	}
	ev, err := c.TraceEvidence([]string{a.Key, b.Key})
	if err != nil || !ev[0].Found || ev[1].Found {
		t.Fatalf("snapshot %+v %v", ev, err)
	}
	result, err = c.SyncSnapshot("docs", []SourceRecord{a})
	if err != nil || result.Changed != 0 || result.Deleted != 0 {
		t.Fatalf("repeat %+v %v", result, err)
	}
}
