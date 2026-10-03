package contract

// OntologyDiagnostic describes an optional retrieval path, not a truth verdict
// about the user's question. It is absent from default/off responses.
type OntologyDiagnostic struct {
	Mode              string `json:"mode"`
	State             string `json:"state"`
	Reason            string `json:"reason,omitempty"`
	BaselineCitations int    `json:"baseline_citations"`
	MatchedConcepts   int    `json:"matched_concepts"`
	AppliedRelations  int    `json:"applied_relations"`
	BoostedCitations  int    `json:"boosted_citations"`
}
