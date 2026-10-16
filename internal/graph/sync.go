package graph

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type SyncResult struct {
	Source    string `json:"source"`
	Revision  string `json:"revision"`
	Changed   int    `json:"changed"`
	Unchanged int    `json:"unchanged"`
	Deleted   int    `json:"deleted"`
	Complete  bool   `json:"complete"`
}

// SyncSnapshot applies an explicit, complete source snapshot. Deletions and the
// cursor are committed only after every upsert succeeds. A failed run leaves its
// successful upserts visible, retains the prior cursor, and can be retried.
// No identity is inferred: mappings to existing canonical keys are explicit.
// ponytail: one writer per tenant graph, serialized per Client. Independent
// writer processes need a shared lease/transaction protocol before deployment.
func (c *Client) SyncSnapshot(source string, records []SourceRecord) (SyncResult, error) {
	out := SyncResult{Source: source}
	if source == "" || len(source) > 100 || strings.ContainsAny(source, "\x00\r\n") || len(records) > 1000 {
		return out, fmt.Errorf("invalid source or snapshot size")
	}
	rows := append([]SourceRecord(nil), records...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	seen := map[string]bool{}
	keys := []string{}
	for _, r := range rows {
		if err := r.Validate(); err != nil {
			return out, err
		}
		if seen[r.Key] {
			return out, fmt.Errorf("duplicate record key %q", r.Key)
		}
		seen[r.Key] = true
		keys = append(keys, r.Key)
	}
	encoded, _ := json.Marshal(rows)
	out.Revision = fmt.Sprintf("%x", sha256.Sum256(encoded))
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range rows {
		changed, err := c.importRecord(r, source)
		if err != nil {
			return out, fmt.Errorf("sync %s: %w; prior upserts may be visible, cursor unchanged; safe to retry", r.Key, err)
		}
		if changed {
			out.Changed++
		} else {
			out.Unchanged++
		}
	}
	// Remove only documents owned by this source, after a complete successful
	// enumeration. The source snapshot, not a transient read error, defines delete.
	result, err := c.g.Query(`OPTIONAL MATCH (d:Document) WHERE d.sync_source=$source AND NOT d.key IN $keys
 WITH collect(d) AS removed
 FOREACH (n IN removed | DETACH DELETE n)
 MERGE (s:SyncState {source:$source})
 SET s.revision=$revision,s.completed_at=$now,s.record_count=$count
 RETURN size(removed) AS deleted`, map[string]interface{}{"source": source, "keys": keys, "revision": out.Revision, "now": time.Now().UTC().Format(time.RFC3339Nano), "count": len(rows)}, nil)
	if err != nil {
		return out, err
	}
	if result.Next() {
		v, _ := result.Record().Get("deleted")
		switch n := v.(type) {
		case int64:
			out.Deleted = int(n)
		case int:
			out.Deleted = n
		}
	}
	out.Complete = true
	return out, nil
}
