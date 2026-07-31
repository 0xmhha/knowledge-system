package contract

// KnowledgeChunk is indexed knowledge that has no place in the source
// tree to cite. ckv synthesizes a per-package conventions summary (error
// style, constructor count, concurrency primitives, test shape) and files
// it at <package-dir>/<convention> with no line range, because there is
// no file to point at.
//
// It is deliberately NOT a Citation. Citation.IsValid requires a positive
// line range — "a citation is a span of a file" is a contract the whole
// pack rests on, and bending it to admit a synthesized location would
// weaken it for every consumer. Knowledge rides in its own section
// instead: same evidence, honest shape.
type KnowledgeChunk struct {
	// Scope is what the knowledge is about — the package directory for a
	// conventions summary.
	Scope string `json:"scope"`
	// Kind is ckv's chunk-strategy label ("convention").
	Kind string `json:"kind"`
	// Text is the synthesized content, carried from the search that found
	// it; there is nothing on disk to re-read.
	Text string `json:"text"`
}

// IsValid reports whether k carries the fields a consumer needs.
func (k KnowledgeChunk) IsValid() bool {
	return k.Scope != "" && k.Kind != "" && k.Text != ""
}
