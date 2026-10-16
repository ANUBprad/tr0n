package graph

// Evidence retrieval & context assembly (M3, docs/REASONING.md §2):
// given one entity, return the bounded, provenance-bearing graph facts
// that could justify an answer about it — deliberately retrieved by
// traversal, never a graph dump.
//
// This is the deterministic "Retrieve evidence" stage feeding the
// reasoning layer: every returned fact is a FACT (an edge plus its
// provenance) as it stands in the graph. Nothing here scores, ranks, or
// materializes an inference, and nothing is written — AssembleEvidence
// is read-only and never creates a stored INFERENCE/RECOMMENDATION.

import (
	"cmp"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/ANUBprad/tr0n/internal/knowledge"
)

// Documented bounds. Simple constants, not a configuration framework:
// one context is capped by fact count and by the serialized size of its
// facts, and any cap that bites is reported in the result rather than
// left implied (docs/REASONING.md §2 "cap size", "never silently imply
// that a partial result is exhaustive").
const (
	DefaultEvidenceLimit    = 64
	DefaultEvidenceMaxBytes = 64 * 1024
)

// Evidence statuses. An unresolved, empty, or ambiguous request is a
// result, not an error and never a guess.
const (
	EvidenceStatusOK         = "ok"
	EvidenceStatusNotFound   = "not_found"
	EvidenceStatusAmbiguous  = "ambiguous"
	EvidenceStatusEmpty      = "empty"
	EvidenceStatusOverBudget = "over_budget"
	EvidenceTruncatedByCount = "count"
	EvidenceTruncatedBySize  = "size"
)

// EvidenceRequest is one bounded retrieval: the entity to gather
// evidence for, plus optional scoping. Empty defaults are the
// documented constants above.
type EvidenceRequest struct {
	// Entity is free text resolved by the canonical tiers
	// (ResolveEntity). Required.
	Entity string `json:"entity"`
	// Label narrows entity resolution to one node label ("" = any).
	Label string `json:"label,omitempty"`
	// AsOf, when set, restricts state relations to those world-valid at
	// that RFC3339 instant under the canonical half-open rule. Empty
	// means no temporal restriction: every window of the entity's facts
	// is in scope (the full history an answer may need, including
	// supersession).
	AsOf string `json:"as_of,omitempty"`
	// Relations, when non-empty, keeps only these relationship types —
	// deliberate traversal rather than "whatever happens to hang off it".
	Relations []string `json:"relations,omitempty"`
	// Limit caps the number of returned facts (<=0 = default).
	Limit int `json:"limit,omitempty"`
	// MaxBytes caps the serialized size of Facts in bytes (<=0 =
	// default).
	MaxBytes int `json:"max_bytes,omitempty"`
}

// EvidenceFact is one retrieved relationship fact: an edge anchored to
// the requested entity, with both endpoint identities, the relationship
// identity, and the edge's complete provenance (including its
// valid_from/valid_to world-validity window).
type EvidenceFact struct {
	// ID is a stable, content-derived identifier (evidenceFactID): the
	// same fact always has the same ID across runs and a JSON round
	// trip, so a claim can cite specific retrieved evidence without
	// depending on position. Two genuinely identical facts share it —
	// they are the same fact.
	ID string `json:"id"`
	// Entity is the requested anchor's key; the fact was retrieved
	// because it touches this entity.
	Entity     string               `json:"entity"`
	Relation   string               `json:"relation"`
	Direction  string               `json:"direction"` // "out"|"in", from the anchor's view
	Neighbor   NodeRef              `json:"neighbor"`
	Provenance knowledge.Provenance `json:"provenance"`
}

// evidenceFactID derives a fact's stable identifier from its own
// identifying fields — anchor, relationship, direction, neighbour, and
// the full provenance (source and validity window). FNV-1a is enough:
// the ID is resolved against the carried facts, not used for security,
// and a context holds at most DefaultEvidenceLimit facts.
func evidenceFactID(f EvidenceFact) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join([]string{
		f.Entity, f.Relation, f.Direction,
		f.Neighbor.Key, f.Neighbor.Label, f.Neighbor.Name,
		f.Provenance.SrcType, f.Provenance.SrcRef, f.Provenance.ObservedAt,
		f.Provenance.Extraction, f.Provenance.ValidFrom, f.Provenance.ValidTo,
	}, "\x00")))
	return fmt.Sprintf("%016x", h.Sum64())
}

// EvidenceContext is the bounded retrieval result. Status is always set:
//
//	ok           facts were found and returned (Facts may be bounded, see Truncated)
//	not_found    the entity does not resolve, or resolves to a missing node
//	ambiguous    more than one entity matches — Candidates, no Facts, no pick
//	empty        the entity resolved but no fact is in scope
//	over_budget  the entity resolved and had in-scope facts, but the byte
//	             bound excluded every one (Facts empty, Truncated true)
type EvidenceContext struct {
	Relations  []string       `json:"relations,omitempty"`
	Entity     *NodeRef       `json:"entity,omitempty"`
	Status     string         `json:"status"`
	Candidates []Candidate    `json:"candidates,omitempty"`
	Facts      []EvidenceFact `json:"facts"`
	Limit      int            `json:"limit"`
	Bytes      int            `json:"bytes"`
	// Truncated is true only when a bound actually bit: the context is
	// partial and says so.
	Truncated       bool   `json:"truncated"`
	TruncatedReason string `json:"truncated_reason,omitempty"` // "count" | "size" | "count+size"
	AsOf            string `json:"as_of,omitempty"`
}

// AssembleEvidence resolves one entity and returns its bounded,
// provenance-bearing facts, ordered deterministically.
//
// Explicit outcomes: a malformed as_of is an error (never silently
// ignored); an unknown entity is not_found; an ambiguous match returns
// Candidates and picks none of them; a resolved entity with no in-scope
// fact returns empty. Read-only: it writes nothing.
//
// ponytail: this method deliberately holds no lock of its own — it is a
// composition of ResolveEntity and TraceEvidence, each of which locks
// (taking c.mu here would deadlock). Two sub-calls are therefore not a
// snapshot: a concurrent write could land between them. The loader is
// single-writer and seeding is recreate-only, so no v1 caller relies on
// that; upgrade path = one lock around the whole assembly if snapshot
// isolation is ever needed.
func (c *Client) AssembleEvidence(req EvidenceRequest) (EvidenceContext, error) {
	ctx := EvidenceContext{Status: EvidenceStatusEmpty, Facts: []EvidenceFact{}, Relations: append([]string(nil), req.Relations...)}
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultEvidenceLimit
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultEvidenceMaxBytes
	}
	ctx.Limit = limit

	if strings.TrimSpace(req.Entity) == "" {
		return ctx, fmt.Errorf("entity is required")
	}

	// Temporal input is validated up front: a malformed instant must
	// fail loudly rather than degrade into "no filter".
	asOfT := time.Time{}
	if req.AsOf != "" {
		t, err := time.Parse(time.RFC3339, req.AsOf)
		if err != nil {
			return ctx, fmt.Errorf("as_of %q must be RFC3339: %w", req.AsOf, err)
		}
		asOfT = t
		ctx.AsOf = t.UTC().Format(time.RFC3339)
	}

	res, err := c.ResolveEntity(req.Entity, req.Label)
	if err != nil {
		return ctx, err
	}
	if res.Key == "" {
		// Unknown and ambiguous are both safe, explicit outcomes.
		ctx.Status = EvidenceStatusNotFound
		if len(res.Candidates) > 0 {
			ctx.Status = EvidenceStatusAmbiguous
			ctx.Candidates = res.Candidates
		}
		return ctx, nil
	}

	ev, err := c.TraceEvidence([]string{res.Key})
	if err != nil {
		return ctx, err
	}
	if len(ev) == 0 || !ev[0].Found {
		// Resolved but absent (the graph changed between the two
		// sub-calls). Reported, never presented as empty-but-valid.
		ctx.Status = EvidenceStatusNotFound
		return ctx, nil
	}
	ctx.Entity = &NodeRef{Label: ev[0].Label, Key: ev[0].Key, Name: ev[0].Name}

	scope := make(map[string]bool, len(req.Relations))
	for _, r := range req.Relations {
		scope[r] = true
	}

	facts := make([]EvidenceFact, 0, len(ev[0].Edges))
	for _, e := range ev[0].Edges {
		if len(scope) > 0 && !scope[e.Relation] {
			continue
		}
		if !factValidAt(e.Relation, e.Provenance, asOfT) {
			continue
		}
		fact := EvidenceFact{
			Entity:     ev[0].Key,
			Relation:   e.Relation,
			Direction:  e.Direction,
			Neighbor:   e.Neighbor,
			Provenance: e.Provenance,
		}
		fact.ID = evidenceFactID(fact)
		facts = append(facts, fact)
	}
	sortEvidenceFacts(facts)

	if len(facts) == 0 {
		ctx.Status = EvidenceStatusEmpty
		return ctx, nil
	}
	ctx.Status = EvidenceStatusOK

	// Bound the result over an already-deterministic list, so which
	// facts survive a truncation never depends on store order.
	if len(facts) > limit {
		facts = facts[:limit]
		ctx.TruncatedReason = EvidenceTruncatedByCount
	}
	total := factsJSONSize(facts)
	for len(facts) > 0 && total > maxBytes {
		total -= factsRowSize(facts[len(facts)-1])
		if len(facts) > 1 {
			total-- // the separating comma
		}
		facts = facts[:len(facts)-1]
		ctx.TruncatedReason = reasonAfter(ctx.TruncatedReason, EvidenceTruncatedBySize)
	}
	ctx.Facts = facts
	ctx.Bytes = total
	ctx.Truncated = ctx.TruncatedReason != ""
	if len(facts) == 0 {
		// The byte bound is unsatisfiable for this result: even the
		// smallest single in-scope fact exceeds max_bytes, so all of
		// them were dropped. "ok" is documented as "facts were found
		// and returned" — with an empty fact list it would read as
		// usable evidence, or as evidence-less, depending on the
		// reader. Report the explicit outcome instead.
		ctx.Status = EvidenceStatusOverBudget
	}
	return ctx, nil
}

// factValidAt applies the canonical world-validity predicate to one
// retrieved fact (docs/SCHEMA.md §5): valid_from <= as_of AND
// (valid_to IS NULL OR valid_to > as_of).
//
// Only state relations carry a valid-time window (the write contract in
// load.go validateEdge), so only they are filtered — exactly what
// FindOwner/FindExperts do with as_of. Event/evidence relations have no
// valid_to by contract and are not window-filtered (FindExperts likewise
// leaves its authored/reviewed branches unfiltered). A zero asOf means
// "no temporal restriction": every fact is in scope.
func factValidAt(rel string, p knowledge.Provenance, asOf time.Time) bool {
	if asOf.IsZero() || !IsStateRelation(rel) {
		return true
	}
	vf, err := time.Parse(time.RFC3339, p.ValidFrom)
	if err != nil {
		return false
	}
	if vf.After(asOf) { // valid_from <= as_of
		return false
	}
	if p.ValidTo == "" {
		return true
	}
	vt, err := time.Parse(time.RFC3339, p.ValidTo)
	if err != nil {
		return false
	}
	return vt.After(asOf) // valid_to > as_of
}

// sortEvidenceFacts gives every result a store-independent order.
func sortEvidenceFacts(facts []EvidenceFact) {
	sort.Slice(facts, func(i, j int) bool { return factLess(facts[i], facts[j]) })
}

// factLess is a total order over every field of a fact. The first six
// keys are the documented order (anchor, relation, direction, neighbor,
// window, source); the remaining fields make otherwise-equal facts
// deterministic instead of inheriting whichever order the store returned
// them in — which is what decides who survives a truncation.
func factLess(a, b EvidenceFact) bool {
	return cmp.Or(
		cmp.Compare(a.Entity, b.Entity),
		cmp.Compare(a.Relation, b.Relation),
		cmp.Compare(a.Direction, b.Direction),
		cmp.Compare(a.Neighbor.Key, b.Neighbor.Key),
		cmp.Compare(a.Provenance.ValidFrom, b.Provenance.ValidFrom),
		cmp.Compare(a.Provenance.SrcRef, b.Provenance.SrcRef),
		cmp.Compare(a.Provenance.ObservedAt, b.Provenance.ObservedAt),
		cmp.Compare(a.Provenance.SrcType, b.Provenance.SrcType),
		cmp.Compare(a.Provenance.Extraction, b.Provenance.Extraction),
		cmp.Compare(a.Provenance.ValidTo, b.Provenance.ValidTo),
		cmp.Compare(a.Neighbor.Label, b.Neighbor.Label),
		cmp.Compare(a.Neighbor.Name, b.Neighbor.Name),
	) < 0
}

// factsRowSize is the serialized size of one fact. Marshal cannot fail
// for these plain string fields.
func factsRowSize(f EvidenceFact) int {
	b, _ := json.Marshal(f)
	return len(b)
}

// factsJSONSize is the serialized size of facts as a JSON array —
// "[" + comma-separated rows + "]", which is exactly what
// json.Marshal([]EvidenceFact) produces for these element types.
func factsJSONSize(facts []EvidenceFact) int {
	total := 2 // surrounding brackets
	for i := range facts {
		total += factsRowSize(facts[i])
		if i > 0 {
			total++
		}
	}
	return total
}

// reasonAfter records why truncation happened, keeping both causes when
// both bounds bit.
func reasonAfter(existing, next string) string {
	switch existing {
	case "":
		return next
	case next:
		return existing
	default:
		return existing + "+" + next
	}
}
