package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestBoundaries(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "docs"), 0700)
	os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("Public project guide"), 0600)
	os.WriteFile(filepath.Join(root, "outside.md"), []byte("outside"), 0600)
	entry := FileRecord{Path: "guide.md", Title: "Guide", Kind: "note", AuthorKey: "person:a", AboutKey: "service:a", SourceRef: "public/guide"}
	path := filepath.Join(root, "manifest.json")
	write := func(m Manifest) { b, _ := json.Marshal(m); os.WriteFile(path, b, 0600) }
	m := Manifest{Source: "docs", Root: "docs", Records: []FileRecord{entry}}
	write(m)
	source, rows, err := ReadManifest(path)
	if err != nil || source != "docs" || len(rows) != 1 || rows[0].Body != "Public project guide" {
		t.Fatalf("%s %+v %v", source, rows, err)
	}
	_, again, _ := ReadManifest(path)
	if rows[0] != again[0] {
		t.Fatal("snapshot is not reproducible")
	}
	m.Records[0].Path = "../outside.md"
	write(m)
	if _, _, err := ReadManifest(path); err == nil {
		t.Fatal("path escaped root")
	}
	m.Records[0].Path = "missing.md"
	write(m)
	if _, _, err := ReadManifest(path); err == nil {
		t.Fatal("missing file became deletion")
	}
	m.Records = nil
	write(m)
	if _, _, err := ReadManifest(path); err == nil {
		t.Fatal("implicit empty snapshot accepted")
	}
	m.AllowEmpty = true
	write(m)
	if _, rows, err := ReadManifest(path); err != nil || len(rows) != 0 {
		t.Fatal(err)
	}
}
