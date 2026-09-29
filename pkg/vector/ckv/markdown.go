package ckv

import (
	"github.com/0xmhha/knowledge-system/internal/vector/parse/markdown"
)

// MarkdownSection is the source-line shape shared by CKV chunking and CKS
// semantic projection. Text is intentionally omitted: consumers should read
// the cited source range from the pinned snapshot before asserting a fact.
type MarkdownSection struct {
	Name        string
	HeadingPath []string
	StartLine   int
	EndLine     int
}

// ParseMarkdownSections delegates to CKV's heading parser so semantic source
// spans cannot drift from the vector index's section boundaries.
func ParseMarkdownSections(file string, source []byte) ([]MarkdownSection, error) {
	spans, err := markdown.New().Parse(file, source)
	if err != nil {
		return nil, err
	}
	out := make([]MarkdownSection, 0, len(spans))
	for _, span := range spans {
		out = append(out, MarkdownSection{
			Name: span.Name, HeadingPath: append([]string(nil), span.HeadingPath...),
			StartLine: span.StartLine, EndLine: span.EndLine,
		})
	}
	return out, nil
}
