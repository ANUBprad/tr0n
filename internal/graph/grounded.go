package graph

// Source-grounded answer contract (E3, docs/EVALUATION.md §2, docs/REASONING.md
// §3). M3 retrieval returns complete structured provenance per EvidenceFact, but
// knowledge.Answer's Claim.Paths are plain strings: a validated answer can name a
// fact path without carrying — or being checked against — that fact's
// provenance. This contract closes that gap additively: knowledge.Answer is
// unchanged, and a GroundedAnswer carries the answer, the EvidenceContext it was
// built from, and one ClaimGrounding per claim naming the entity the claim is
// about and the IDs of the retrieved facts that support it.
//
// It proves *structural* grounding — each FACT claim cites evidence that exists
// in the retrieval result and matches the claim's subject and relationship — not
// factual correctness or entailment, which needs a wired answering model and a
// labeled Q/A corpus (docs/EVALUATION.md §6). Absence claims ("no OWNS edge")
// have no EvidenceFact to cite and keep their existing operation-reference path.

import (
	"fmt"

	"github.com/ANUBprad/tr0n/internal/knowledge"
)

// GroundedAnswer is a knowledge.Answer together with the evidence it was
// built from and, for every claim, what that claim is about and which
// retrieved facts support it. Grounding[i] describes Answer.Claims[i]
// (index-aligned; Validate enforces the match).
type GroundedAnswer struct {
	Answer    knowledge.Answer `json:"answer"`
	Evidence  EvidenceContext  `json:"evidence"`
	Grounding []ClaimGrounding `json:"grounding"`
}

// ClaimGrounding names the subject entity a claim asserts something
// about — and, for relationship claims, the relationship — plus the
// identifiers of the retrieved facts that back it. For FACT claims at
// least one ID is required; a non-FACT claim (INFERENCE/RECOMMENDATION)
// keeps its own rule_id/path discipline and may cite none.
type ClaimGrounding struct {
	// Subject is the anchor entity key the claim is about.
	Subject string `json:"subject"`
	// Relation, when non-empty, is the relationship the claim asserts;
	// every cited fact must be that relationship (never a different one).
	Relation string `json:"relation,omitempty"`
	// EvidenceIDs are the IDs of facts in Evidence.Facts that support
	// the claim.
	EvidenceIDs []string `json:"evidence_ids"`
}

// Validate refuses an answer whose FACT claims are not backed by the
// evidence it carries. It runs knowledge.Answer.Validate first (every
// claim tagged, evidenced, and layered), then:
//
//   - grounding and claims must be the same length (one entry per claim);
//   - every carried fact must have an ID (the citation target);
//   - every FACT claim must declare a subject and cite at least one
//     ID that resolves to a carried fact;
//   - each cited fact must be anchored to the claim's subject and,
//     when a relationship is declared, be that relationship;
//   - a FACT claim cannot stand on a non-ok context: not_found,
//     ambiguous, empty, and over_budget are explicit outcomes and are
//     never laundered into a supported factual claim.
//
// Truncation is carried verbatim, not re-derived: a truncated-but-ok
// context still validates, because it was partial and says so.
func (g GroundedAnswer) Validate() error {
	if err := g.Answer.Validate(); err != nil {
		return err
	}
	if len(g.Grounding) != len(g.Answer.Claims) {
		return fmt.Errorf("grounding has %d entries for %d claims", len(g.Grounding), len(g.Answer.Claims))
	}

	// Every FACT claim needs evidence, so a non-ok context can never
	// back one — check before resolving IDs so the reason is explicit.
	factClaims := 0
	for _, c := range g.Answer.Claims {
		if c.Layer == knowledge.LayerFact {
			factClaims++
		}
	}
	if factClaims > 0 && g.Evidence.Status != EvidenceStatusOK {
		return fmt.Errorf("%d FACT claim(s) on evidence status %q, not %s", factClaims, g.Evidence.Status, EvidenceStatusOK)
	}

	byID := make(map[string]EvidenceFact, len(g.Evidence.Facts))
	for _, f := range g.Evidence.Facts {
		if f.ID == "" {
			return fmt.Errorf("carried evidence (%s %s) has no id to cite", f.Relation, f.Neighbor.Key)
		}
		if prior, ok := byID[f.ID]; ok && prior != f {
			return fmt.Errorf("conflicting facts share evidence id %q", f.ID)
		}
		byID[f.ID] = f
	}

	for i, c := range g.Answer.Claims {
		if c.Layer != knowledge.LayerFact {
			continue
		}
		gr := g.Grounding[i]
		if gr.Subject == "" {
			return fmt.Errorf("claim %d (%q): FACT claim declares no subject", i, c.Text)
		}
		if len(gr.EvidenceIDs) == 0 {
			return fmt.Errorf("claim %d (%q): FACT claim cites no evidence", i, c.Text)
		}
		for _, id := range gr.EvidenceIDs {
			f, ok := byID[id]
			if !ok {
				return fmt.Errorf("claim %d (%q): cites unknown evidence %q", i, c.Text, id)
			}
			if f.Entity != gr.Subject {
				return fmt.Errorf("claim %d (%q): evidence %q is anchored to %q, not %q", i, c.Text, id, f.Entity, gr.Subject)
			}
			if gr.Relation != "" && f.Relation != gr.Relation {
				return fmt.Errorf("claim %d (%q): evidence %q is a %s fact, not %s", i, c.Text, id, f.Relation, gr.Relation)
			}
		}
	}
	return nil
}
