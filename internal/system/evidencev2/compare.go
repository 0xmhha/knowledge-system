package evidencev2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// ResponseComparison describes saved outputs. It cannot establish common binary,
// configuration, request options, timing, ground truth, or independent samples.
type ResponseComparison struct {
	Scope                     string                   `json:"scope"`
	OffIntegrityHash          string                   `json:"off_integrity_hash"`
	OnIntegrityHash           string                   `json:"on_integrity_hash"`
	OnMode                    string                   `json:"on_mode"`
	OnState                   string                   `json:"on_state"`
	OffEvidenceState          string                   `json:"off_evidence_state"`
	OnEvidenceState           string                   `json:"on_evidence_state"`
	CitationMembershipChanged bool                     `json:"citation_membership_changed"`
	CitationOrderChanged      bool                     `json:"citation_order_changed"`
	BodyOutputsChanged        bool                     `json:"body_outputs_changed"`
	Citations                 []ResponseCitationChange `json:"citations"`
	QualityEvaluated          bool                     `json:"quality_evaluated"`
	LatencyEvaluated          bool                     `json:"latency_evaluated"`
	ControlConditionsVerified bool                     `json:"control_conditions_verified"`
	Limitations               []string                 `json:"limitations"`
}

type ResponseCitationChange struct {
	CitationKeySHA256 string `json:"citation_key_sha256"`
	OffRank           int    `json:"off_rank"`
	OnRank            int    `json:"on_rank"`
	OffBodySHA256     string `json:"off_body_sha256,omitempty"`
	OnBodySHA256      string `json:"on_body_sha256,omitempty"`
}

// CompareResponses verifies hashes and requires the same query and retained
// coordinates. Neither a changed rank nor an active arm is a quality verdict.
func CompareResponses(off, on contract.EvidencePackV2) (ResponseComparison, error) {
	var result ResponseComparison
	if err := Verify(off); err != nil {
		return result, fmt.Errorf("off_response_invalid: %w", err)
	}
	if err := Verify(on); err != nil {
		return result, fmt.Errorf("on_response_invalid: %w", err)
	}
	if off.Query != on.Query || off.Coordinates != on.Coordinates {
		return result, fmt.Errorf("comparison_input_mismatch")
	}
	if off.Metadata.Ontology != nil && off.Metadata.Ontology.Mode != "off" {
		return result, fmt.Errorf("off_arm_required")
	}
	d := on.Metadata.Ontology
	if d == nil || (d.Mode != "relations" && d.Mode != "concept_text" && d.Mode != "combined") {
		return result, fmt.Errorf("on_arm_required")
	}
	// Stable states only; an arbitrary saved string is not a public diagnostic.
	switch d.State {
	case "active", "no_match", "unavailable", "stale", "budget_exceeded":
	default:
		return result, fmt.Errorf("on_state_invalid")
	}
	result = ResponseComparison{Scope: "saved_response_structure", OffIntegrityHash: off.Metadata.IntegrityHash, OnIntegrityHash: on.Metadata.IntegrityHash, OnMode: d.Mode, OnState: d.State, OffEvidenceState: off.EvidenceState, OnEvidenceState: on.EvidenceState, Citations: []ResponseCitationChange{}, Limitations: []string{"binary/config/model identity and all request options are not bound by saved packs", "no timing, failed request denominator, model call count, ground truth, or independent sample verdict", "citation ranks describe published order; candidate score changes require explain_ontology"}}
	positions := map[string]int{}
	for i, c := range off.Citations {
		key := comparisonKey(c)
		positions[key] = len(result.Citations)
		result.Citations = append(result.Citations, ResponseCitationChange{CitationKeySHA256: key, OffRank: i + 1})
	}
	for i, c := range on.Citations {
		key := comparisonKey(c)
		at, ok := positions[key]
		if !ok {
			at = len(result.Citations)
			positions[key] = at
			result.Citations = append(result.Citations, ResponseCitationChange{CitationKeySHA256: key})
		}
		result.Citations[at].OnRank = i + 1
	}
	for _, body := range off.Bodies {
		at := positions[comparisonKey(body.Citation)]
		result.Citations[at].OffBodySHA256 = comparisonHash([]byte(body.Text))
	}
	for _, body := range on.Bodies {
		at := positions[comparisonKey(body.Citation)]
		result.Citations[at].OnBodySHA256 = comparisonHash([]byte(body.Text))
	}
	for _, c := range result.Citations {
		if c.OffRank == 0 || c.OnRank == 0 {
			result.CitationMembershipChanged = true
		}
		if c.OffRank != c.OnRank {
			result.CitationOrderChanged = true
		}
		if c.OffBodySHA256 != c.OnBodySHA256 {
			result.BodyOutputsChanged = true
		}
	}
	a, _ := json.Marshal(off.Bodies)
	b, _ := json.Marshal(on.Bodies)
	result.BodyOutputsChanged = result.BodyOutputsChanged || string(a) != string(b)
	return result, nil
}

func comparisonKey(c contract.CitationV2) string {
	b, _ := json.Marshal(c.KeyV2())
	return comparisonHash(b)
}
func comparisonHash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
