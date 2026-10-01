// Package semantic defines the source-backed CKS semantic projection.
// It is deliberately separate from CKG's AST node taxonomy: source facts
// remain in CKV/CKG and these records describe reviewed meaning over them.
package semantic

// SchemaVersion is the file/DB projection contract version.
const SchemaVersion = 3

// PackProjectionVersion introduces tuple-scoped, archived pack definitions.
// Historical v1-v3 projections remain immutable and readable.
const PackProjectionVersion = 4

type Status string

const (
	StatusProposed Status = "proposed"
	StatusVerified Status = "verified"
	StatusRejected Status = "rejected"
)

type SourceKind string

const (
	SourceDocument SourceKind = "document"
	SourceCode     SourceKind = "code"
	SourceTest     SourceKind = "test"
)

// Snapshot is the immutable project-scoped identity shared by every semantic
// record. SourceMode is omitted in historical committed projections.
type Snapshot struct {
	ProjectID  string `json:"project_id"`
	DatasetID  string `json:"dataset_id"`
	SnapshotID string `json:"snapshot_id,omitempty"`
	SourceMode string `json:"source_mode,omitempty"`
	Commit     string `json:"commit"`
}

// EvidenceSpan points to exact source lines. ContentSHA256 hashes the bytes
// of that line range as read from the indexed snapshot. CanonicalID and
// ChunkID are optional joins; neither replaces the source-line proof.
type EvidenceSpan struct {
	ID            string     `json:"id"`
	Snapshot      Snapshot   `json:"snapshot"`
	Kind          SourceKind `json:"kind"`
	Path          string     `json:"path"`
	StartLine     int        `json:"start_line"`
	EndLine       int        `json:"end_line"`
	ContentSHA256 string     `json:"content_sha256"`
	CanonicalID   string     `json:"canonical_id,omitempty"`
	ChunkID       string     `json:"chunk_id,omitempty"`
	Extractor     string     `json:"extractor"`
}

// DocumentSection is a Markdown/spec heading plus its source span.
type DocumentSection struct {
	ID          string   `json:"id"`
	Heading     string   `json:"heading"`
	HeadingPath []string `json:"heading_path,omitempty"`
	EvidenceID  string   `json:"evidence_id"`
	ChunkIDs    []string `json:"chunk_ids,omitempty"`
}

// Concept is reviewed vocabulary over CKV and CKG, not an AST node. Terms
// preserve natural-language alternatives while one preferred term per
// language makes display deterministic.
type Concept struct {
	ID         string   `json:"id" yaml:"id"`
	Kind       string   `json:"kind" yaml:"kind"`
	Definition string   `json:"definition" yaml:"definition"`
	Includes   []string `json:"includes" yaml:"includes"`
	Excludes   []string `json:"excludes" yaml:"excludes"`
	Terms      []Term   `json:"terms" yaml:"terms"`
	EvidenceID string   `json:"evidence_id" yaml:"-"`
	Status     Status   `json:"status" yaml:"status"`
	ReviewedBy string   `json:"reviewed_by,omitempty" yaml:"reviewed_by"`
}

type Term struct {
	Lang      string `json:"lang" yaml:"lang"`
	Value     string `json:"value" yaml:"value"`
	Preferred bool   `json:"preferred" yaml:"preferred"`
}

// Requirement is an approved or proposed specification, not a statement
// that code exists. Implementation is determined later from reviewed edges.
type Requirement struct {
	ID                 string                `json:"id" yaml:"id"`
	Version            int                   `json:"version" yaml:"version"`
	Title              string                `json:"title" yaml:"title"`
	Statement          string                `json:"statement" yaml:"statement"`
	Status             Status                `json:"status" yaml:"status"`
	ReviewedBy         string                `json:"reviewed_by,omitempty" yaml:"reviewed_by"`
	EvidenceID         string                `json:"evidence_id" yaml:"-"`
	ConceptIDs         []string              `json:"concept_ids,omitempty" yaml:"concept_ids"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria" yaml:"acceptance_criteria"`
}

// AcceptanceCriterion describes observable behavior without claiming a test
// already passes. Its evidence points to the exact committed specification.
type AcceptanceCriterion struct {
	ID         string `json:"id" yaml:"id"`
	Given      string `json:"given" yaml:"given"`
	When       string `json:"when" yaml:"when"`
	Then       string `json:"then" yaml:"then"`
	EvidenceID string `json:"evidence_id" yaml:"-"`
}

// Claim is a human-readable assertion extracted from a section. Extraction
// starts proposed; verification requires an explicit reviewer and live proof.
type Claim struct {
	ID          string   `json:"id"`
	Statement   string   `json:"statement"`
	SectionID   string   `json:"section_id"`
	EvidenceIDs []string `json:"evidence_ids"`
	Status      Status   `json:"status"`
	ReviewedBy  string   `json:"reviewed_by,omitempty"`
}

// Assertion is a reviewed semantic edge. It has its own evidence and status;
// the existence of its endpoints does not prove the relationship.
type Assertion struct {
	ID          string   `json:"id"`
	Predicate   string   `json:"predicate"`
	SubjectID   string   `json:"subject_id"`
	ObjectID    string   `json:"object_id"`
	EvidenceIDs []string `json:"evidence_ids"`
	Status      Status   `json:"status"`
	ReviewedBy  string   `json:"reviewed_by,omitempty"`
}

const (
	PredicateSupports      = "SUPPORTS"       // DocumentSection -> Claim
	PredicateContradicts   = "CONTRADICTS"    // Claim -> Claim
	PredicateAbout         = "ABOUT"          // Claim -> Concept
	PredicateImplementedBy = "IMPLEMENTED_BY" // Concept -> CKG canonical CodeSymbol
	PredicateTestedBy      = "TESTED_BY"      // CKG CodeSymbol -> CKG TestSymbol
	PredicateAcceptedBy    = "ACCEPTED_BY"    // legacy v1/v2 read-only criterion -> test link
	PredicateCheckedBy     = "CHECKED_BY"     // v3 AcceptanceCriterion -> CKG TestSymbol; link only
)

// Projection is one versioned semantic graph view for one source snapshot.
// Empty Claims is valid: extracting sections must not invent facts.
type Projection struct {
	SchemaVersion int                  `json:"schema_version"`
	Snapshot      Snapshot             `json:"snapshot"`
	Evidence      []EvidenceSpan       `json:"evidence"`
	Sections      []DocumentSection    `json:"sections"`
	Concepts      []Concept            `json:"concepts,omitempty"`
	Requirements  []Requirement        `json:"requirements,omitempty"`
	Claims        []Claim              `json:"claims"`
	Assertions    []Assertion          `json:"assertions,omitempty"`
	Knowledge     *KnowledgeProjection `json:"knowledge,omitempty"`
}
