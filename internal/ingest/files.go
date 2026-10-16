// Package ingest reads an explicit manifest of authorized local Markdown files.
// It never crawls the filesystem or follows links outside the source root.
package ingest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ANUBprad/tr0n/internal/graph"
)

type Manifest struct {
	Source string `json:"source"`
	Root   string `json:"root"`
	// Empty snapshots must be deliberate because they remove managed documents.
	AllowEmpty bool         `json:"allow_empty"`
	Records    []FileRecord `json:"records"`
}
type FileRecord struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
	AuthorKey string `json:"author_key"`
	AboutKey  string `json:"about_key"`
	SourceRef string `json:"source_ref"`
}

func ReadManifest(path string) (string, []graph.SourceRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return "", nil, fmt.Errorf("manifest exceeds read limit")
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	var m Manifest
	if err := d.Decode(&m); err != nil {
		return "", nil, err
	}
	if d.Decode(new(interface{})) != io.EOF {
		return "", nil, fmt.Errorf("trailing or oversized manifest")
	}
	if len(m.Records) > 1000 || (len(m.Records) == 0 && !m.AllowEmpty) {
		return "", nil, fmt.Errorf("manifest requires 1–1000 records, or explicit allow_empty")
	}
	root := m.Root
	if !filepath.IsAbs(root) {
		root = filepath.Join(filepath.Dir(path), root)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", nil, err
	}
	rows := []graph.SourceRecord{}
	for _, entry := range m.Records {
		if filepath.IsAbs(entry.Path) || !strings.EqualFold(filepath.Ext(entry.Path), ".md") {
			return "", nil, fmt.Errorf("only relative Markdown paths are supported")
		}
		actual, err := filepath.EvalSymlinks(filepath.Join(root, entry.Path))
		if err != nil {
			return "", nil, err
		}
		relative, err := filepath.Rel(root, actual)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", nil, fmt.Errorf("source path escapes root")
		}
		file, err := os.Open(actual)
		if err != nil {
			return "", nil, err
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			return "", nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 100000 {
			file.Close()
			return "", nil, fmt.Errorf("source must be a regular file up to 100000 bytes")
		}
		body, err := io.ReadAll(io.LimitReader(file, 100001))
		file.Close()
		if err != nil || len(body) > 100000 {
			return "", nil, fmt.Errorf("source read failed or grew beyond limit")
		}
		// Keys derive from source + normalized declared path, never a person's name.
		key := fmt.Sprintf("file:%x", sha256.Sum256([]byte(m.Source+"\x00"+filepath.ToSlash(filepath.Clean(entry.Path)))))
		r := graph.SourceRecord{Key: key, Title: entry.Title, Body: string(body), Kind: entry.Kind, AuthorKey: entry.AuthorKey, AboutKey: entry.AboutKey, SourceRef: entry.SourceRef, ObservedAt: info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")}
		if err := r.Validate(); err != nil {
			return "", nil, err
		}
		rows = append(rows, r)
	}
	return m.Source, rows, nil
}
