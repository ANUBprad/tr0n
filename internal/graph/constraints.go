package graph

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// EnsureConstraints complements the graph-wide application guard with database
// uniqueness within each domain label. It waits for asynchronous enforcement.
// Different labels still require the application's explicit-key contract.
func (c *Client) EnsureConstraints() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, label := range nodeLabels {
		if _, err := c.g.Query(fmt.Sprintf("CREATE INDEX FOR (n:%s) ON (n.key)", label), nil, nil); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
			return err
		}
		err := c.db.Conn.Do(context.Background(), "GRAPH.CONSTRAINT", "CREATE", c.name, "UNIQUE", "NODE", label, "PROPERTIES", 1, "key").Err()
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
			return fmt.Errorf("constraint %s: %w", label, err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		result, err := c.g.ROQuery("CALL db.constraints()", nil, nil)
		if err != nil {
			return err
		}
		count := 0
		pending := false
		for result.Next() {
			r := result.Record()
			status := field(r, "status")
			if status == "FAILED" {
				return fmt.Errorf("database constraint failed; resolve existing duplicate keys")
			}
			if status != "OPERATIONAL" {
				pending = true
			}
			count++
		}
		if count >= len(nodeLabels) && !pending {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database constraints did not become operational")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
