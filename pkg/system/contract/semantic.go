package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// SemanticLink is a reviewed trace path relevant to one existing citation.
// Unconfirmed means the path is not evidence that an acceptance test passed.
type SemanticLink struct {
	RequirementID  string   `json:"requirement_id"`
	CriterionIDs   []string `json:"criterion_ids"`
	ConceptID      string   `json:"concept_id"`
	CodeSymbolID   string   `json:"code_symbol_id"`
	TestSymbolID   string   `json:"test_symbol_id"`
	AssertionIDs   []string `json:"assertion_ids"`
	CodeEvidenceID string   `json:"code_evidence_id"`
	TestEvidenceID string   `json:"test_evidence_id"`
	Citation       Citation `json:"citation"`
	Unconfirmed    bool     `json:"unconfirmed"`
}

// SemanticOverlay is additive to EvidencePack's legacy wire shape. Its own
// digest covers the overlay, while the base pack hash retains old semantics.
type SemanticOverlay struct {
	ProjectID string         `json:"project_id"`
	DatasetID string         `json:"dataset_id"`
	Commit    string         `json:"commit"`
	Links     []SemanticLink `json:"links"`
	Digest    string         `json:"digest"`
}

func (o SemanticOverlay) IsValid(citations []Citation) bool {
	if o.ProjectID == "" || o.DatasetID == "" || len(o.Commit) != 40 || len(o.Links) == 0 {
		return false
	}
	validDigest, err := VerifySemanticOverlay(o)
	if err != nil || !validDigest {
		return false
	}
	for _, link := range o.Links {
		if link.RequirementID == "" || link.ConceptID == "" || link.CodeSymbolID == "" ||
			link.TestSymbolID == "" || link.CodeEvidenceID == "" || link.TestEvidenceID == "" ||
			len(link.AssertionIDs) < 2 || len(link.CriterionIDs) == 0 || !link.Unconfirmed ||
			link.Citation.CommitHash != o.Commit {
			return false
		}
		found := false
		for _, citation := range citations {
			if citation == link.Citation {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func SemanticOverlayDigest(o SemanticOverlay) (string, error) {
	o.Digest = ""
	data, err := json.Marshal(o)
	if err != nil {
		return "", fmt.Errorf("marshal semantic overlay: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func StampSemanticOverlay(o *SemanticOverlay) error {
	if o == nil {
		return fmt.Errorf("nil semantic overlay")
	}
	digest, err := SemanticOverlayDigest(*o)
	if err != nil {
		return err
	}
	o.Digest = digest
	return nil
}

func VerifySemanticOverlay(o SemanticOverlay) (bool, error) {
	if o.Digest == "" {
		return false, fmt.Errorf("semantic overlay has no digest")
	}
	digest, err := SemanticOverlayDigest(o)
	if err != nil {
		return false, err
	}
	return digest == o.Digest, nil
}
