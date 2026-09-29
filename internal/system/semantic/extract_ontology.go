package semantic

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// OntologyPack is the reviewable YAML source format for project vocabulary.
// The projection stores its concepts and source spans; questions remain in
// the source file as the acceptance guide for a domain reviewer.
type OntologyPack struct {
	Version             int       `yaml:"version"`
	ProjectID           string    `yaml:"project_id"`
	Domain              string    `yaml:"domain"`
	CompetencyQuestions []string  `yaml:"competency_questions"`
	Concepts            []Concept `yaml:"concepts"`
}

// ExtractOntology turns one committed YAML vocabulary into proposed or
// reviewed source-backed concepts. It does not infer relations from wording.
func ExtractOntology(snapshot Snapshot, file string, source []byte) (Projection, OntologyPack, error) {
	if err := snapshot.validate(); err != nil {
		return Projection{}, OntologyPack{}, err
	}
	if !safeRelativePath(file) {
		return Projection{}, OntologyPack{}, fmt.Errorf("ontology: unsafe source path %q", file)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	decoder.KnownFields(true)
	var pack OntologyPack
	if err := decoder.Decode(&pack); err != nil {
		return Projection{}, OntologyPack{}, fmt.Errorf("ontology: decode: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Projection{}, OntologyPack{}, fmt.Errorf("ontology: multiple YAML documents")
		}
		return Projection{}, OntologyPack{}, fmt.Errorf("ontology: trailing YAML: %w", err)
	}
	if pack.Version != 1 || pack.ProjectID != snapshot.ProjectID || strings.TrimSpace(pack.Domain) == "" ||
		len(pack.CompetencyQuestions) == 0 || len(pack.Concepts) == 0 {
		return Projection{}, OntologyPack{}, fmt.Errorf("ontology: version, project, domain, questions or concepts invalid")
	}
	for _, question := range pack.CompetencyQuestions {
		if strings.TrimSpace(question) == "" {
			return Projection{}, OntologyPack{}, fmt.Errorf("ontology: empty competency question")
		}
	}
	var root yaml.Node
	if err := yaml.Unmarshal(source, &root); err != nil {
		return Projection{}, OntologyPack{}, err
	}
	items, err := ontologyConceptNodes(&root)
	if err != nil {
		return Projection{}, OntologyPack{}, err
	}
	if len(items) != len(pack.Concepts) {
		return Projection{}, OntologyPack{}, fmt.Errorf("ontology: concept source positions do not match decoded concepts")
	}
	lines := bytes.SplitAfter(source, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	p := Projection{SchemaVersion: SchemaVersion, Snapshot: snapshot, Concepts: pack.Concepts}
	for i, item := range items {
		start, end := item.Line, len(lines)
		if i+1 < len(items) {
			end = items[i+1].Line - 1
		}
		if start < 1 || end < start || end > len(lines) {
			return Projection{}, OntologyPack{}, fmt.Errorf("ontology: concept %d has invalid source span", i)
		}
		span := bytes.Join(lines[start-1:end], nil)
		digest := sha256.Sum256(span)
		key := strings.Join([]string{snapshot.ProjectID, snapshot.DatasetID, file,
			fmt.Sprint(start), fmt.Sprint(end), hex.EncodeToString(digest[:])}, "\x00")
		idHash := sha256.Sum256([]byte(key))
		evidenceID := "ontology:" + hex.EncodeToString(idHash[:12])
		p.Evidence = append(p.Evidence, EvidenceSpan{
			ID: evidenceID, Snapshot: snapshot, Kind: SourceDocument,
			Path: file, StartLine: start, EndLine: end,
			ContentSHA256: hex.EncodeToString(digest[:]), Extractor: "ontology-yaml-v1",
		})
		p.Concepts[i].EvidenceID = evidenceID
	}
	if err := p.Validate(); err != nil {
		return Projection{}, OntologyPack{}, err
	}
	return p, pack, nil
}

func ontologyConceptNodes(root *yaml.Node) ([]*yaml.Node, error) {
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("ontology: expected one YAML mapping")
	}
	mapping := root.Content[0]
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "concepts" {
			seq := mapping.Content[i+1]
			if seq.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("ontology: concepts must be a sequence")
			}
			return seq.Content, nil
		}
	}
	return nil, fmt.Errorf("ontology: missing concepts sequence")
}
