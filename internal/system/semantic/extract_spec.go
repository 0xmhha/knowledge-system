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

// SpecPack is a versioned, reviewable input. Verified means that a human
// approved the specification; it never means the behavior is implemented.
type SpecPack struct {
	Version      int           `yaml:"version"`
	ProjectID    string        `yaml:"project_id"`
	Requirements []Requirement `yaml:"requirements"`
}

// ExtractSpec preserves the exact committed YAML spans for each requirement
// and acceptance criterion. It makes no implementation or test assertion.
func ExtractSpec(snapshot Snapshot, file string, source []byte) (Projection, SpecPack, error) {
	if err := snapshot.validate(); err != nil {
		return Projection{}, SpecPack{}, err
	}
	if !safeRelativePath(file) {
		return Projection{}, SpecPack{}, fmt.Errorf("spec: unsafe source path %q", file)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	decoder.KnownFields(true)
	var pack SpecPack
	if err := decoder.Decode(&pack); err != nil {
		return Projection{}, SpecPack{}, fmt.Errorf("spec: decode: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Projection{}, SpecPack{}, fmt.Errorf("spec: multiple YAML documents")
		}
		return Projection{}, SpecPack{}, fmt.Errorf("spec: trailing YAML: %w", err)
	}
	if pack.Version != 1 || pack.ProjectID != snapshot.ProjectID || len(pack.Requirements) == 0 {
		return Projection{}, SpecPack{}, fmt.Errorf("spec: version, project, or requirements invalid")
	}
	var root yaml.Node
	if err := yaml.Unmarshal(source, &root); err != nil {
		return Projection{}, SpecPack{}, err
	}
	items, err := mappingSequence(&root, "requirements")
	if err != nil || len(items) != len(pack.Requirements) {
		return Projection{}, SpecPack{}, fmt.Errorf("spec: requirement source positions invalid: %v", err)
	}
	lines := bytes.SplitAfter(source, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	p := Projection{SchemaVersion: SchemaVersion, Snapshot: snapshot, Requirements: pack.Requirements}
	seen := map[string]bool{}
	addEvidence := func(prefix string, start, end int) (string, error) {
		if start < 1 || end < start || end > len(lines) {
			return "", fmt.Errorf("spec: invalid source span %d-%d", start, end)
		}
		content := bytes.Join(lines[start-1:end], nil)
		digest := sha256.Sum256(content)
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%d:%d\x00%x", snapshot.ProjectID, snapshot.DatasetID, file, start, end, digest)
		idHash := sha256.Sum256([]byte(key))
		id := prefix + ":" + hex.EncodeToString(idHash[:12])
		p.Evidence = append(p.Evidence, EvidenceSpan{ID: id, Snapshot: snapshot,
			Kind: SourceDocument, Path: file, StartLine: start, EndLine: end,
			ContentSHA256: hex.EncodeToString(digest[:]), Extractor: "spec-yaml-v1"})
		return id, nil
	}
	for i, item := range items {
		req := &p.Requirements[i]
		if item.Kind != yaml.MappingNode || strings.TrimSpace(req.ID) == "" || seen[req.ID] {
			return Projection{}, SpecPack{}, fmt.Errorf("spec: invalid or duplicate requirement ID %q", req.ID)
		}
		seen[req.ID] = true
		end := len(lines)
		if i+1 < len(items) {
			end = items[i+1].Line - 1
		}
		req.EvidenceID, err = addEvidence("spec", item.Line, end)
		if err != nil {
			return Projection{}, SpecPack{}, err
		}
		criteria, err := mappingSequence(item, "acceptance_criteria")
		if err != nil || len(criteria) != len(req.AcceptanceCriteria) || len(criteria) == 0 {
			return Projection{}, SpecPack{}, fmt.Errorf("spec: requirement %q criteria source positions invalid: %v", req.ID, err)
		}
		for j, criterionNode := range criteria {
			criterion := &req.AcceptanceCriteria[j]
			if criterionNode.Kind != yaml.MappingNode || strings.TrimSpace(criterion.ID) == "" || seen[criterion.ID] {
				return Projection{}, SpecPack{}, fmt.Errorf("spec: invalid or duplicate criterion ID %q", criterion.ID)
			}
			seen[criterion.ID] = true
			criterionEnd := end
			if j+1 < len(criteria) {
				criterionEnd = criteria[j+1].Line - 1
			}
			criterion.EvidenceID, err = addEvidence("criterion", criterionNode.Line, criterionEnd)
			if err != nil {
				return Projection{}, SpecPack{}, err
			}
		}
	}
	return p, pack, nil
}

func mappingSequence(root *yaml.Node, field string) ([]*yaml.Node, error) {
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) != 1 {
			return nil, fmt.Errorf("expected one YAML document")
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected YAML mapping")
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == field {
			if root.Content[i+1].Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("%s must be a sequence", field)
			}
			return root.Content[i+1].Content, nil
		}
	}
	return nil, fmt.Errorf("missing %s sequence", field)
}
