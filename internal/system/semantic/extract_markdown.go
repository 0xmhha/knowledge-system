package semantic

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/vector/ckv"
)

// ExtractMarkdown creates source-backed sections without inventing claims.
// Claims can be proposed separately and remain proposed until reviewed.
func ExtractMarkdown(snapshot Snapshot, file string, source []byte) (Projection, error) {
	if err := snapshot.validate(); err != nil {
		return Projection{}, err
	}
	if !safeRelativePath(file) {
		return Projection{}, fmt.Errorf("semantic: invalid Markdown path %q", file)
	}
	spans, err := ckv.ParseMarkdownSections(file, source)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{SchemaVersion: SchemaVersion, Snapshot: snapshot}
	lines := bytes.SplitAfter(source, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	for _, span := range spans {
		if span.StartLine < 1 || span.EndLine < span.StartLine || span.EndLine > len(lines) {
			return Projection{}, fmt.Errorf("semantic: Markdown span %d-%d outside %s", span.StartLine, span.EndLine, file)
		}
		text := bytes.Join(lines[span.StartLine-1:span.EndLine], nil)
		sum := sha256.Sum256(text)
		digest := hex.EncodeToString(sum[:])
		key := strings.Join([]string{snapshot.ProjectID, snapshot.DatasetID, file,
			fmt.Sprint(span.StartLine), fmt.Sprint(span.EndLine), digest}, "\x00")
		idHash := sha256.Sum256([]byte(key))
		base := hex.EncodeToString(idHash[:12])
		evidenceID := "evidence:" + base
		sectionID := "section:" + base
		heading := span.Name
		if len(span.HeadingPath) > 0 {
			heading = span.HeadingPath[len(span.HeadingPath)-1]
		}
		p.Evidence = append(p.Evidence, EvidenceSpan{
			ID: evidenceID, Snapshot: snapshot, Kind: SourceDocument,
			Path: file, StartLine: span.StartLine, EndLine: span.EndLine,
			ContentSHA256: digest, Extractor: "ckv-markdown-v1",
		})
		p.Sections = append(p.Sections, DocumentSection{
			ID: sectionID, Heading: heading,
			HeadingPath: append([]string(nil), span.HeadingPath...), EvidenceID: evidenceID,
		})
	}
	if err := p.Validate(); err != nil {
		return Projection{}, err
	}
	return p, nil
}
