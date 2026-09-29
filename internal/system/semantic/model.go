// Package semantic defines the source-backed CKS semantic projection.
// It is deliberately separate from CKG's AST node taxonomy: source facts
// remain in CKV/CKG and these records describe reviewed meaning over them.
package semantic

// SchemaVersion is the file/DB projection contract version.
const SchemaVersion = 1

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
// record. A working-tree digest will be added when that source mode lands.
type Snapshot struct {
	ProjectID string `json:"project_id"`
	DatasetID string `json:"dataset_id"`
	Commit    string `json:"commit"`
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

// Projection is one versioned semantic graph view for one source snapshot.
// Empty Claims is valid: extracting sections must not invent facts.
type Projection struct {
	SchemaVersion int               `json:"schema_version"`
	Snapshot      Snapshot          `json:"snapshot"`
	Evidence      []EvidenceSpan    `json:"evidence"`
	Sections      []DocumentSection `json:"sections"`
	Claims        []Claim           `json:"claims"`
}
