package contract

// V2Coordinates identifies one immutable build. The legacy EvidencePack has
// a separate DTO and integrity algorithm; callers must never mix the two.
type V2Coordinates struct {
	ProjectID  string `json:"project_id"`
	DatasetID  string `json:"dataset_id"`
	SnapshotID string `json:"snapshot_id"`
	SourceMode string `json:"source_mode"`
	BaseCommit string `json:"base_commit"`
}

type CitationV2 struct {
	File          string `json:"file"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
	CommitHash    string `json:"commit_hash"`
	ProjectID     string `json:"project_id"`
	DatasetID     string `json:"dataset_id"`
	SnapshotID    string `json:"snapshot_id"`
	SourceMode    string `json:"source_mode"`
	BaseCommit    string `json:"base_commit"`
	OriginID      string `json:"origin_id"`
	FileSHA256    string `json:"file_sha256"`
	ContentSHA256 string `json:"content_sha256"`
}

// CitationKeyV2 is a comparable tuple; unlike Citation.Key, it cannot merge
// identical paths and lines from different immutable datasets.
type CitationKeyV2 struct {
	ProjectID, DatasetID, SnapshotID, OriginID, File string
	StartLine, EndLine                               int
	FileSHA256, ContentSHA256                        string
}

func (c CitationV2) KeyV2() CitationKeyV2 {
	return CitationKeyV2{c.ProjectID, c.DatasetID, c.SnapshotID, c.OriginID,
		c.File, c.StartLine, c.EndLine, c.FileSHA256, c.ContentSHA256}
}

type BodyV2 struct {
	Citation CitationV2 `json:"citation"`
	Text     string     `json:"text"`
}

// KnowledgeContextV2 is the optional, explicitly requested domain overlay.
// It records reviewed source references and uncertainty; source text stays in
// the separately sanitized Bodies collection.
type KnowledgeContextV2 struct {
	State               string                `json:"state"`
	LockDigest          string                `json:"lock_digest"`
	ApplicablePolicies  []KnowledgePolicyV2   `json:"applicable_policies"`
	Decisions           []KnowledgeDecisionV2 `json:"decisions"`
	Relations           []KnowledgeRelationV2 `json:"relations"`
	Constraints         []string              `json:"constraints"`
	RelatedRequirements []string              `json:"related_requirements"`
	TestLinks           []string              `json:"test_links"`
	Unknowns            []string              `json:"unknowns"`
	Conflicts           []KnowledgeConflictV2 `json:"conflicts"`
}

type KnowledgeSemanticV2 struct {
	KnowledgeContext KnowledgeContextV2 `json:"knowledge_context"`
	CodingContext    CodingContextV2    `json:"coding_context"`
}

// CodingContextV2 separates cited source behavior from normative policy and
// rationale. Empty implemented_behavior means no reviewed implementation link
// was proved; it must not be inferred from a retrieved code chunk alone.
type CodingContextV2 struct {
	ImplementedBehavior []KnowledgeReferenceV2 `json:"implemented_behavior"`
	RequiredBehavior    []KnowledgeReferenceV2 `json:"required_behavior"`
	Rationale           []KnowledgeReferenceV2 `json:"rationale"`
	Constraints         []KnowledgeReferenceV2 `json:"constraints"`
	Evidence            []CitationV2           `json:"evidence"`
	Unknowns            []string               `json:"unknowns"`
}

type KnowledgeReferenceV2 struct {
	ID       string     `json:"id"`
	State    string     `json:"state"`
	Citation CitationV2 `json:"citation"`
}

type KnowledgePolicyV2 struct {
	ID            string     `json:"id"`
	State         string     `json:"state"`
	ReviewedBy    string     `json:"reviewed_by"`
	EffectiveFrom string     `json:"effective_from"`
	EffectiveTo   string     `json:"effective_to,omitempty"`
	Citation      CitationV2 `json:"citation"`
}

type KnowledgeDecisionV2 struct {
	ID             string     `json:"id"`
	State          string     `json:"state"`
	ReviewedBy     string     `json:"reviewed_by"`
	Date           string     `json:"date"`
	RequirementIDs []string   `json:"requirement_ids"`
	Citation       CitationV2 `json:"citation"`
}

type KnowledgeRelationV2 struct {
	ID         string     `json:"id"`
	PackID     string     `json:"pack_id"`
	Predicate  string     `json:"predicate"`
	SubjectID  string     `json:"subject_id"`
	ObjectID   string     `json:"object_id"`
	ReviewedBy string     `json:"reviewed_by"`
	Citation   CitationV2 `json:"citation"`
}

type KnowledgeConflictV2 struct {
	LeftID  string `json:"left_id"`
	RightID string `json:"right_id"`
	Reason  string `json:"reason"`
}

type EvidencePackV2 struct {
	FormatVersion  int           `json:"format_version"`
	Coordinates    V2Coordinates `json:"coordinates"`
	Query          string        `json:"query"`
	Citations      []CitationV2  `json:"citations"`
	Bodies         []BodyV2      `json:"bodies"`
	GraphNeighbors []any         `json:"graph_neighbors"`
	Semantic       any           `json:"semantic"`
	EvidenceState  string        `json:"evidence_state"`
	Metadata       V2Metadata    `json:"metadata"`
}

type V2Metadata struct {
	IntegrityHashAlgo string `json:"integrity_hash_algo"`
	IntegrityHash     string `json:"integrity_hash,omitempty"`
}
