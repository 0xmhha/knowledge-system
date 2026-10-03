package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const TextCorpusSchemaVersion = 1

type TextCorpusFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// TextCorpusManifest binds generated, reviewed Markdown to one immutable
// semantic projection and source commit. manifest.json is written last.
type TextCorpusManifest struct {
	SchemaVersion    int              `json:"schema_version"`
	Snapshot         Snapshot         `json:"snapshot"`
	SourceRoot       string           `json:"source_root"`
	ProjectionSHA256 string           `json:"projection_sha256"`
	Files            []TextCorpusFile `json:"files"`
}

// ConceptText is derived only from a reviewed, source-validated concept. It
// carries no implementation edge: text-only retrieval must not borrow one.
type ConceptText struct {
	ConceptID  string
	Text       string
	ReviewedBy string
	Evidence   EvidenceSpan
}

func (a ActiveProjection) VerifiedConceptText(id string) (ConceptText, bool) {
	for _, concept := range a.projection.Concepts {
		if concept.ID != id || concept.Status != StatusVerified || concept.ReviewedBy == "" {
			continue
		}
		for _, span := range a.projection.Evidence {
			if span.ID == concept.EvidenceID && span.Kind == SourceDocument {
				return ConceptText{ConceptID: id, Text: renderConceptText(concept, span, a.projection.Snapshot), ReviewedBy: concept.ReviewedBy, Evidence: span}, true
			}
		}
	}
	return ConceptText{}, false
}

func renderConceptText(concept Concept, span EvidenceSpan, snapshot Snapshot) string {
	terms := append([]Term(nil), concept.Terms...)
	sort.Slice(terms, func(i, j int) bool {
		if terms[i].Lang != terms[j].Lang {
			return terms[i].Lang < terms[j].Lang
		}
		return terms[i].Value < terms[j].Value
	})
	var body strings.Builder
	fmt.Fprintf(&body, "# Concept %s\n\n%s\n\n", concept.ID, oneLine(concept.Definition))
	fmt.Fprintf(&body, "Kind: %s. Included: %s. Excluded: %s.\n\n", concept.Kind,
		oneLine(strings.Join(concept.Includes, "; ")), oneLine(strings.Join(concept.Excludes, "; ")))
	for _, term := range terms {
		fmt.Fprintf(&body, "Term (%s): %s.\n", term.Lang, oneLine(term.Value))
	}
	fmt.Fprintf(&body, "\nSource: %s:%d-%d at %s. Evidence: %s.\n", span.Path, span.StartLine, span.EndLine, snapshot.Commit, span.ID)
	return body.String()
}

// ExportTextCorpus renders only verified concepts and requirements. It never
// treats proposed or rejected records as search-authoritative text.
func (a ActiveProjection) ExportTextCorpus(out, repoRoot string) (TextCorpusManifest, error) {
	p := a.projection
	if err := p.Validate(); err != nil {
		return TextCorpusManifest{}, err
	}
	sourceRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return TextCorpusManifest{}, err
	}
	sourceRoot, err = filepath.Abs(sourceRoot)
	if err != nil {
		return TextCorpusManifest{}, err
	}
	projectionBytes, err := json.Marshal(p)
	if err != nil {
		return TextCorpusManifest{}, err
	}
	projectionHash := sha256.Sum256(projectionBytes)
	manifest := TextCorpusManifest{SchemaVersion: TextCorpusSchemaVersion, Snapshot: p.Snapshot, SourceRoot: sourceRoot,
		ProjectionSHA256: hex.EncodeToString(projectionHash[:]), Files: []TextCorpusFile{}}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return TextCorpusManifest{}, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(out))
	if err != nil {
		return TextCorpusManifest{}, err
	}
	target := filepath.Join(parent, filepath.Base(out))
	rel, err := filepath.Rel(sourceRoot, target)
	if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return TextCorpusManifest{}, fmt.Errorf("semantic text corpus must be outside the source tree")
	}
	if err := os.Mkdir(out, 0755); err != nil {
		return TextCorpusManifest{}, fmt.Errorf("create new semantic corpus: %w", err)
	}
	evidence := map[string]EvidenceSpan{}
	for _, span := range p.Evidence {
		evidence[span.ID] = span
	}
	concepts := append([]Concept(nil), p.Concepts...)
	sort.Slice(concepts, func(i, j int) bool { return concepts[i].ID < concepts[j].ID })
	for _, concept := range concepts {
		if concept.Status != StatusVerified {
			continue
		}
		span := evidence[concept.EvidenceID]
		if err := writeCorpusFile(out, "concept", concept.ID, []byte(renderConceptText(concept, span, p.Snapshot)), &manifest); err != nil {
			return TextCorpusManifest{}, err
		}
	}
	requirements := append([]Requirement(nil), p.Requirements...)
	sort.Slice(requirements, func(i, j int) bool { return requirements[i].ID < requirements[j].ID })
	for _, requirement := range requirements {
		if requirement.Status != StatusVerified {
			continue
		}
		span := evidence[requirement.EvidenceID]
		var body strings.Builder
		fmt.Fprintf(&body, "# Requirement %s version %d: %s\n\n%s\n\n", requirement.ID,
			requirement.Version, oneLine(requirement.Title), oneLine(requirement.Statement))
		for _, criterion := range requirement.AcceptanceCriteria {
			fmt.Fprintf(&body, "Acceptance %s: Given %s; When %s; Then %s.\n", criterion.ID,
				oneLine(criterion.Given), oneLine(criterion.When), oneLine(criterion.Then))
		}
		fmt.Fprintf(&body, "\nSource: %s:%d-%d at %s. Evidence: %s.\n", span.Path, span.StartLine, span.EndLine, p.Snapshot.Commit, span.ID)
		if err := writeCorpusFile(out, "requirement", requirement.ID, []byte(body.String()), &manifest); err != nil {
			return TextCorpusManifest{}, err
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return TextCorpusManifest{}, err
	}
	if err := os.WriteFile(filepath.Join(out, "manifest.json"), append(data, '\n'), 0644); err != nil {
		return TextCorpusManifest{}, err
	}
	return manifest, nil
}

func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }

func writeCorpusFile(out, kind, id string, content []byte, manifest *TextCorpusManifest) error {
	idHash := sha256.Sum256([]byte(id))
	name := fmt.Sprintf("%s-%s.md", kind, hex.EncodeToString(idHash[:16]))
	if err := os.WriteFile(filepath.Join(out, name), content, 0644); err != nil {
		return err
	}
	sum := sha256.Sum256(content)
	manifest.Files = append(manifest.Files, TextCorpusFile{Path: name, SHA256: hex.EncodeToString(sum[:])})
	return nil
}
