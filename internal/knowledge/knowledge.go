// Package knowledge holds TRON's shared knowledge-model types.
// Layering rules live in docs/knowledge-model.md; types here are the
// structural enforcement of the provenance minimum.
package knowledge

// Provenance is the evidence chain every edge must carry so any claim
// can answer "why do you believe this?".
type Provenance struct {
	SrcType    string `json:"src_type"`
	SrcRef     string `json:"src_ref"`
	ObservedAt string `json:"observed_at"`
	Extraction string `json:"extraction"`
	ValidFrom  string `json:"valid_from,omitempty"`
	ValidTo    string `json:"valid_to,omitempty"`
}
