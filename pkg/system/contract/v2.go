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
