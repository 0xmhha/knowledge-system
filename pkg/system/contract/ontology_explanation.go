package contract

// OntologyExplanation is a bounded causal retrieval trace, not a quality verdict.
// Hashes identify candidate keys without disclosing uncited paths or source text.
type OntologyExplanation struct {
	State                  string                 `json:"state"`
	Matches                []OntologyConceptMatch `json:"matches,omitempty"`
	Changes                []OntologyRankChange   `json:"changes,omitempty"`
	TotalChanged           int                    `json:"total_changed"`
	Truncated              bool                   `json:"truncated"`
	FinalBodyBudgetPartial bool                   `json:"final_body_budget_partial"`
	QualityEvaluated       bool                   `json:"quality_evaluated"`
}
type OntologyConceptMatch struct {
	ConceptID string `json:"concept_id"`
	Status    string `json:"status"`
}
type OntologyRankChange struct {
	CitationKeySHA256  string   `json:"citation_key_sha256"`
	RankBefore         int      `json:"rank_before"`
	RankAfter          int      `json:"rank_after"`
	ScoreBefore        string   `json:"score_before"`
	ScoreAfter         string   `json:"score_after"`
	RelationIDs        []string `json:"relation_ids,omitempty"`
	TextEvidenceIDs    []string `json:"text_evidence_ids,omitempty"`
	SelectedByComposer bool     `json:"selected_by_composer"`
}
