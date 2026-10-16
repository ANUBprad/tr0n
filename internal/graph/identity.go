package graph

import (
	"fmt"
	"strings"
	"time"
)

// rejectAliases reserves reviewed identifiers across all application writers.
// Caller holds c.mu; cross-process writers still require external serialization.
func (c *Client) rejectAliases(keys []string) error {
	values := make([]interface{}, len(keys))
	for i, key := range keys {
		values[i] = key
	}
	r, err := c.g.ROQuery(`MATCH (a:IdentityAlias) WHERE a.alias IN $keys RETURN a.alias AS alias LIMIT 1`, map[string]interface{}{"keys": values}, nil)
	if err != nil {
		if strings.Contains(err.Error(), "empty key") {
			return nil
		}
		return err
	}
	if r.Next() {
		return fmt.Errorf("key is reserved by a reviewed identity mapping")
	}
	return nil
}

// RegisterIdentity attaches a reviewed external identifier to an existing
// canonical entity. It never merges nodes, guesses from names, moves fact edges,
// or overwrites a conflicting mapping. Source attribution remains on the alias.
func (c *Client) RegisterIdentity(alias, canonical, source string, observed time.Time) error {
	if !keyShapeOK(alias) || !keyShapeOK(canonical) || alias == canonical || source == "" || observed.IsZero() {
		return fmt.Errorf("explicit distinct identities, source and observation are required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	params := map[string]interface{}{"alias": alias, "canonical": canonical, "source": source, "observed": observed.UTC().Format(time.RFC3339Nano)}
	res, err := c.g.ROQuery(`MATCH (n) WHERE n.key=$canonical RETURN n.key AS key`, params, nil)
	if err != nil {
		return err
	}
	count := 0
	for res.Next() {
		count++
	}
	if count != 1 {
		return fmt.Errorf("canonical identity must resolve uniquely")
	}
	existing, err := c.existingKeys([]string{alias})
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("alias collides with an existing canonical key; no destructive merge is allowed")
	}
	res, err = c.g.ROQuery(`MATCH (a:IdentityAlias {alias:$alias}) RETURN a.canonical AS canonical,a.src_ref AS source`, params, nil)
	if err != nil {
		return err
	}
	if res.Next() {
		if field(res.Record(), "canonical") != canonical || field(res.Record(), "source") != source {
			return fmt.Errorf("alias already has a different reviewed mapping")
		}
		return nil
	}
	_, err = c.g.Query(`MATCH (n {key:$canonical}) CREATE (a:IdentityAlias {alias:$alias,canonical:$canonical,src_ref:$source,observed_at:$observed,extraction:'reviewed_mapping'})`, params, nil)
	return err
}

// resolveAlias is called only after a canonical key miss and before name tiers.
func (c *Client) resolveAlias(alias, label string) (Resolution, error) {
	out := Resolution{Candidates: []Candidate{}}
	result, err := c.g.ROQuery(`MATCH (a:IdentityAlias {alias:$alias}),(n) WHERE n.key=a.canonical AND ($label='' OR $label IN labels(n)) RETURN n.key AS key,labels(n)[0] AS label,coalesce(n.name,n.title) AS name`, map[string]interface{}{"alias": alias, "label": label}, nil)
	if err != nil {
		return out, err
	}
	for result.Next() {
		r := result.Record()
		out.Candidates = append(out.Candidates, Candidate{Key: field(r, "key"), Label: field(r, "label"), Name: field(r, "name")})
	}
	if len(out.Candidates) == 1 {
		out.Key = out.Candidates[0].Key
		out.Candidates = []Candidate{}
		out.MatchedOn = "reviewed_alias"
	}
	return out, nil
}
