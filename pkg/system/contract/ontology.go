package contract

// OntologyDiagnostic describes an optional retrieval path, not a truth verdict
// about the user's question. It is absent from default/off responses.
type OntologyDiagnostic struct {
	Mode                 string               `json:"mode"`
	State                string               `json:"state"`
	Reason               string               `json:"reason,omitempty"`
	BaselineCitations    int                  `json:"baseline_citations"`
	MatchedConcepts      int                  `json:"matched_concepts"`
	AppliedRelations     int                  `json:"applied_relations"`
	BoostedCitations     int                  `json:"boosted_citations"`
	TextSearchCalls      int                  `json:"text_search_calls,omitempty"`
	TextBoostedCitations int                  `json:"text_boosted_citations,omitempty"`
	TextSources          []OntologyTextSource `json:"text_sources,omitempty"`
}

// OntologyTextSource is retrieval provenance. The rendered text stays inside
// local retrieval; source/query hashes avoid exposing unsanitized definitions.
type OntologyTextSource struct {
	ConceptID     string `json:"concept_id"`
	EvidenceID    string `json:"evidence_id"`
	QuerySHA256   string `json:"query_sha256"`
	ProjectID     string `json:"project_id"`
	DatasetID     string `json:"dataset_id"`
	SnapshotID    string `json:"snapshot_id"`
	Commit        string `json:"commit"`
	File          string `json:"file"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
	ContentSHA256 string `json:"content_sha256"`
	ReturnedHits  int    `json:"returned_hits"`
}
