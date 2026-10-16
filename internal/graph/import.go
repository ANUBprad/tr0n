package graph

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SourceRecord is an explicit mapping of an allowed document, ticket or chat
// export to existing company identities. Content is data, never instructions.
type SourceRecord struct {
	Key        string `json:"key"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Kind       string `json:"kind"`
	AuthorKey  string `json:"author_key"`
	AboutKey   string `json:"about_key"`
	SourceRef  string `json:"source_ref"`
	ObservedAt string `json:"observed_at"`
}

func (r SourceRecord) Validate() error {
	if !keyShapeOK(r.Key) || !keyShapeOK(r.AuthorKey) || !keyShapeOK(r.AboutKey) {
		return fmt.Errorf("key, author_key and about_key must be source:id identities")
	}
	if strings.TrimSpace(r.Title) == "" || len(r.Title) > 500 || strings.TrimSpace(r.Body) == "" || len(r.Body) > 100000 || strings.TrimSpace(r.SourceRef) == "" || len(r.SourceRef) > 1000 {
		return fmt.Errorf("record needs title (1–500 bytes), body (1–100000 bytes), and source_ref (1–1000 bytes)")
	}
	if !allowed([]string{"adr", "runbook", "spec", "note", "chat", "ticket"}, r.Kind) {
		return fmt.Errorf("kind must be adr, runbook, spec, note, chat or ticket")
	}
	if _, err := time.Parse(time.RFC3339, r.ObservedAt); err != nil {
		return fmt.Errorf("observed_at must be RFC3339")
	}
	return nil
}

// ImportRecord atomically upserts one record and its mapped relationships.
// Retrying identical input is a no-op. Stale observations and identity/source
// collisions fail before mutation. Only this importer's own edges are replaced.
func (c *Client) ImportRecord(record SourceRecord) (bool, error) {
	if err := record.Validate(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	observed, _ := time.Parse(time.RFC3339, record.ObservedAt)
	record.ObservedAt = observed.UTC().Format(time.RFC3339Nano)
	encoded, _ := json.Marshal(record)
	hash := fmt.Sprintf("%x", sha256.Sum256(encoded))
	params := map[string]interface{}{"key": record.Key, "author": record.AuthorKey, "about": record.AboutKey}
	res, err := c.g.ROQuery(`MATCH (a:Person {key:$author}),(t {key:$about})
	 WHERE t:Service OR t:Project OR t:Technology RETURN a.key AS author,t.key AS target`, params, nil)
	if err != nil {
		return false, err
	}
	count := 0
	for res.Next() {
		count++
	}
	if count != 1 {
		return false, fmt.Errorf("author and about target must resolve uniquely to Person and Service/Project/Technology")
	}
	res, err = c.g.ROQuery(`MATCH (n {key:$key}) RETURN labels(n)[0] AS label,n.src_ref AS source,n.observed_at AS observed,n.content_hash AS hash`, params, nil)
	if err != nil {
		return false, err
	}
	count = 0
	unchanged := false
	for res.Next() {
		count++
		r := res.Record()
		if field(r, "label") != "Document" || field(r, "source") != record.SourceRef {
			return false, fmt.Errorf("key already belongs to a different label or source")
		}
		previous, e := time.Parse(time.RFC3339, field(r, "observed"))
		if e != nil || observed.Before(previous) {
			return false, fmt.Errorf("source observation is older than the stored record or stored timestamp is invalid")
		}
		unchanged = field(r, "hash") == hash
		if observed.Equal(previous) && !unchanged {
			return false, fmt.Errorf("different content at the same observation time; provide a newer source observation")
		}
	}
	if count > 1 {
		return false, fmt.Errorf("duplicate source identity in graph")
	}
	if unchanged {
		return false, nil
	}
	sourceType := "document_export"
	if record.Kind == "chat" {
		sourceType = "chat_export"
	}
	if record.Kind == "ticket" {
		sourceType = "ticket_export"
	}
	params["props"] = map[string]interface{}{"key": record.Key, "title": record.Title, "body": record.Body, "kind": record.Kind, "src_type": sourceType, "src_ref": record.SourceRef, "observed_at": record.ObservedAt, "extraction": "explicit_mapping", "content_hash": hash}
	params["edge"] = map[string]interface{}{"src_type": sourceType, "src_ref": record.SourceRef, "observed_at": record.ObservedAt, "extraction": "explicit_mapping", "ingest_key": record.Key}
	_, err = c.g.Query(`MATCH (a:Person {key:$author}),(t {key:$about})
	 MERGE (d:Document {key:$key}) SET d=$props
	 WITH a,t,d OPTIONAL MATCH (d)-[r]-() WHERE r.ingest_key=$key
	 WITH a,t,d,collect(r) AS old
	 FOREACH (edge IN old | DELETE edge)
	 CREATE (a)-[auth:AUTHORED]->(d)-[about:ABOUT]->(t)
	 SET auth=$edge,about=$edge`, params, nil)
	if err != nil {
		return false, fmt.Errorf("import record: %w", err)
	}
	return true, nil
}
