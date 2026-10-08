package semantic

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"gopkg.in/yaml.v3"
)

var packName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9]*)+$`)
var packLocal = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

type PackTypeRef struct {
	PackID  string `json:"pack_id" yaml:"pack_id"`
	LocalID string `json:"local_id" yaml:"local_id"`
}
type PackConcept struct {
	PackID     string       `json:"pack_id"`
	LocalID    string       `json:"local_id"`
	Kind       string       `json:"kind"`
	Definition string       `json:"definition"`
	Extends    *PackTypeRef `json:"extends,omitempty"`
}
type PackRelation struct {
	PackID           string      `json:"pack_id"`
	Predicate        string      `json:"predicate"`
	SubjectType      PackTypeRef `json:"subject_type"`
	ObjectType       PackTypeRef `json:"object_type"`
	Direction        string      `json:"direction"`
	Cardinality      string      `json:"cardinality"`
	RequiredEvidence bool        `json:"required_evidence"`
	ReviewRule       string      `json:"review_rule"`
}
type PackConstraint struct {
	PackID      string `json:"pack_id"`
	LocalID     string `json:"local_id"`
	Description string `json:"description"`
}
type PackProjection struct {
	PackID       string           `json:"pack_id"`
	Version      string           `json:"version"`
	Digest       string           `json:"digest"`
	OriginID     string           `json:"origin_id"`
	SourceSHA256 string           `json:"source_sha256"`
	Concepts     []PackConcept    `json:"concepts"`
	Relations    []PackRelation   `json:"relations"`
	Constraints  []PackConstraint `json:"constraints"`
}
type KnowledgeProjection struct {
	LockDigest string           `json:"lock_digest"`
	Packs      []PackProjection `json:"packs"`
}

// ProjectPackSource constructs a deterministic tuple-scoped projection from
// exact archived pack.yaml bytes. The caller must first verify the pack lock.
func ProjectPackSource(source []byte, packID, version, digest, originID string) (PackProjection, error) {
	var document struct {
		PackSchemaVersion int    `yaml:"pack_schema_version"`
		PackID            string `yaml:"pack_id"`
		Version           string `yaml:"version"`
		Owner             string `yaml:"owner"`
		Scope             string `yaml:"scope"`
		Requires          []struct {
			PackID  string `yaml:"pack_id"`
			Version string `yaml:"version"`
			Digest  string `yaml:"digest"`
		} `yaml:"requires"`
		Concepts []struct {
			ID         string       `yaml:"id"`
			Kind       string       `yaml:"kind"`
			Definition string       `yaml:"definition"`
			Extends    *PackTypeRef `yaml:"extends"`
		} `yaml:"concepts"`
		Relations []struct {
			Predicate        string      `yaml:"predicate"`
			SubjectType      PackTypeRef `yaml:"subject_type"`
			ObjectType       PackTypeRef `yaml:"object_type"`
			Direction        string      `yaml:"direction"`
			Cardinality      string      `yaml:"cardinality"`
			RequiredEvidence bool        `yaml:"required_evidence"`
			ReviewRule       string      `yaml:"review_rule"`
		} `yaml:"relation_types"`
		Constraints []struct {
			ID          string `yaml:"id"`
			Description string `yaml:"description"`
		} `yaml:"constraints"`
		Questions []string `yaml:"competency_questions"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(source))
	dec.KnownFields(true)
	if err := dec.Decode(&document); err != nil {
		return PackProjection{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return PackProjection{}, fmt.Errorf("pack source has multiple YAML documents")
	}
	if document.PackSchemaVersion != 1 || document.PackID != packID || document.Version != version {
		return PackProjection{}, fmt.Errorf("pack source identity differs from lock")
	}
	hash := sha256.Sum256(source)
	result := PackProjection{PackID: packID, Version: version, Digest: digest, OriginID: originID,
		SourceSHA256: hex.EncodeToString(hash[:]), Concepts: []PackConcept{}, Relations: []PackRelation{}, Constraints: []PackConstraint{}}
	for _, concept := range document.Concepts {
		result.Concepts = append(result.Concepts, PackConcept{PackID: packID, LocalID: concept.ID, Kind: concept.Kind,
			Definition: concept.Definition, Extends: concept.Extends})
	}
	for _, relation := range document.Relations {
		result.Relations = append(result.Relations, PackRelation{PackID: packID, Predicate: relation.Predicate,
			SubjectType: relation.SubjectType, ObjectType: relation.ObjectType, Direction: relation.Direction,
			Cardinality: relation.Cardinality, RequiredEvidence: relation.RequiredEvidence, ReviewRule: relation.ReviewRule})
	}
	for _, constraint := range document.Constraints {
		result.Constraints = append(result.Constraints, PackConstraint{PackID: packID, LocalID: constraint.ID, Description: constraint.Description})
	}
	sort.Slice(result.Concepts, func(a, b int) bool { return result.Concepts[a].LocalID < result.Concepts[b].LocalID })
	sort.Slice(result.Relations, func(a, b int) bool { return result.Relations[a].Predicate < result.Relations[b].Predicate })
	sort.Slice(result.Constraints, func(a, b int) bool { return result.Constraints[a].LocalID < result.Constraints[b].LocalID })
	return result, nil
}

func (k *KnowledgeProjection) validate() error {
	if k == nil || !digestPattern.MatchString(k.LockDigest) {
		return fmt.Errorf("v4 knowledge lock digest is invalid")
	}
	seenPacks := map[string]bool{}
	types := map[PackTypeRef]bool{}
	for id := range CoreConceptKinds {
		types[PackTypeRef{PackID: "core", LocalID: id}] = true
	}
	for i, pack := range k.Packs {
		if !packName.MatchString(pack.PackID) || pack.PackID == "core" || seenPacks[pack.PackID] ||
			pack.OriginID != "knowledge:"+pack.PackID || !digestPattern.MatchString(pack.Digest) ||
			!digestPattern.MatchString(pack.SourceSHA256) || pack.Version == "" ||
			i > 0 && k.Packs[i-1].PackID >= pack.PackID {
			return fmt.Errorf("v4 pack identity or order invalid")
		}
		seenPacks[pack.PackID] = true
		for j, concept := range pack.Concepts {
			ref := PackTypeRef{PackID: pack.PackID, LocalID: concept.LocalID}
			if concept.PackID != pack.PackID || !packLocal.MatchString(concept.LocalID) ||
				concept.Kind == "" || concept.Definition == "" || types[ref] ||
				j > 0 && pack.Concepts[j-1].LocalID >= concept.LocalID {
				return fmt.Errorf("v4 pack concept invalid")
			}
			types[ref] = true
		}
	}
	for _, pack := range k.Packs {
		for _, concept := range pack.Concepts {
			if concept.Extends != nil && !types[*concept.Extends] {
				return fmt.Errorf("v4 pack extension target missing")
			}
		}
		seenRelations := map[string]bool{}
		for j, relation := range pack.Relations {
			if relation.PackID != pack.PackID || !packLocal.MatchString(relation.Predicate) || seenRelations[relation.Predicate] ||
				!types[relation.SubjectType] || !types[relation.ObjectType] || relation.Direction != "forward" ||
				!relation.RequiredEvidence || relation.ReviewRule == "" ||
				j > 0 && pack.Relations[j-1].Predicate >= relation.Predicate {
				return fmt.Errorf("v4 pack relation invalid")
			}
			seenRelations[relation.Predicate] = true
		}
		seenConstraints := map[string]bool{}
		for j, constraint := range pack.Constraints {
			if constraint.PackID != pack.PackID || !packLocal.MatchString(constraint.LocalID) || constraint.Description == "" ||
				seenConstraints[constraint.LocalID] || j > 0 && pack.Constraints[j-1].LocalID >= constraint.LocalID {
				return fmt.Errorf("v4 pack constraint invalid")
			}
			seenConstraints[constraint.LocalID] = true
		}
	}
	return nil
}

func (k *KnowledgeProjection) validateRetained(versionDir string) error {
	lockBytes, _, _, err := setup.ReadRetainedFile(versionDir, "repo", ".cks/knowledge/knowledge.lock.json")
	if err != nil {
		return err
	}
	var lock struct {
		LockDigest string `json:"lock_digest"`
		Packs      []struct {
			PackID   string `json:"pack_id"`
			Version  string `json:"version"`
			Digest   string `json:"digest"`
			OriginID string `json:"origin_id"`
		} `json:"packs"`
	}
	if err := json.Unmarshal(lockBytes, &lock); err != nil {
		return err
	}
	if lock.LockDigest != k.LockDigest || len(lock.Packs) != len(k.Packs) {
		return fmt.Errorf("v4 pack lock differs from projection")
	}
	byID := map[string]PackProjection{}
	for _, pack := range k.Packs {
		byID[pack.PackID] = pack
	}
	for _, pin := range lock.Packs {
		want, ok := byID[pin.PackID]
		if !ok || want.Version != pin.Version || want.Digest != pin.Digest || want.OriginID != pin.OriginID {
			return fmt.Errorf("v4 pack pin differs from lock")
		}
		buf, _, _, err := setup.ReadRetainedFile(versionDir, pin.OriginID, "pack.yaml")
		if err != nil {
			return err
		}
		actual, err := ProjectPackSource(buf, pin.PackID, pin.Version, pin.Digest, pin.OriginID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, want) {
			return fmt.Errorf("v4 pack type projection differs from archived source")
		}
	}
	return nil
}
