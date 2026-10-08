package knowledgepack

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"gopkg.in/yaml.v3"
)

var packIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9]*)+$`)
var localIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type TypeRef struct {
	PackID  string `yaml:"pack_id" json:"pack_id"`
	LocalID string `yaml:"local_id" json:"local_id"`
}

type Dependency struct {
	PackID  string `yaml:"pack_id" json:"pack_id"`
	Version string `yaml:"version" json:"version"`
	Digest  string `yaml:"digest" json:"digest"`
}

type ConceptType struct {
	ID         string   `yaml:"id" json:"id"`
	Kind       string   `yaml:"kind" json:"kind"`
	Definition string   `yaml:"definition" json:"definition"`
	Extends    *TypeRef `yaml:"extends,omitempty" json:"extends,omitempty"`
}

type RelationType struct {
	Predicate        string  `yaml:"predicate" json:"predicate"`
	SubjectType      TypeRef `yaml:"subject_type" json:"subject_type"`
	ObjectType       TypeRef `yaml:"object_type" json:"object_type"`
	Direction        string  `yaml:"direction" json:"direction"`
	Cardinality      string  `yaml:"cardinality" json:"cardinality"`
	RequiredEvidence bool    `yaml:"required_evidence" json:"required_evidence"`
	ReviewRule       string  `yaml:"review_rule" json:"review_rule"`
}

type Constraint struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description" json:"description"`
}

type Pack struct {
	PackSchemaVersion   int            `yaml:"pack_schema_version" json:"pack_schema_version"`
	PackID              string         `yaml:"pack_id" json:"pack_id"`
	Version             string         `yaml:"version" json:"version"`
	Owner               string         `yaml:"owner" json:"owner"`
	Scope               string         `yaml:"scope" json:"scope"`
	Requires            []Dependency   `yaml:"requires" json:"requires"`
	Concepts            []ConceptType  `yaml:"concepts" json:"concepts"`
	RelationTypes       []RelationType `yaml:"relation_types" json:"relation_types"`
	Constraints         []Constraint   `yaml:"constraints" json:"constraints"`
	CompetencyQuestions []string       `yaml:"competency_questions" json:"competency_questions"`
}

type LoadedPack struct {
	Pack   Pack
	Root   string
	Digest string
}

func Decode(source []byte) (Pack, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	decoder.KnownFields(true)
	var pack Pack
	if err := decoder.Decode(&pack); err != nil {
		return Pack{}, fmt.Errorf("pack_incompatible: decode: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Pack{}, fmt.Errorf("pack_incompatible: multiple or invalid YAML documents")
	}
	if err := pack.Validate(); err != nil {
		return Pack{}, err
	}
	return pack, nil
}

func Load(root string) (LoadedPack, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return LoadedPack{}, fmt.Errorf("pack_incompatible: pack root is not a directory")
	}
	data, err := setup.ReadSourceFileNoFollow(root, "pack.yaml", 1<<20)
	if err != nil {
		return LoadedPack{}, fmt.Errorf("pack_incompatible: read pack.yaml: %w", err)
	}
	pack, err := Decode(data)
	if err != nil {
		return LoadedPack{}, err
	}
	digest, err := DigestTree(root)
	if err != nil {
		return LoadedPack{}, err
	}
	return LoadedPack{Pack: pack, Root: root, Digest: digest}, nil
}

func (p Pack) Validate() error {
	if p.PackSchemaVersion != 1 || !packIDPattern.MatchString(p.PackID) ||
		!versionPattern.MatchString(p.Version) || strings.TrimSpace(p.Owner) == "" ||
		strings.TrimSpace(p.Scope) == "" || len(p.CompetencyQuestions) == 0 {
		return fmt.Errorf("pack_incompatible: invalid pack header")
	}
	for _, question := range p.CompetencyQuestions {
		if strings.TrimSpace(question) == "" {
			return fmt.Errorf("pack_incompatible: empty competency question")
		}
	}
	seenDeps := map[string]bool{}
	for _, dep := range p.Requires {
		if !packIDPattern.MatchString(dep.PackID) || !versionPattern.MatchString(dep.Version) ||
			!digestPattern.MatchString(dep.Digest) || dep.PackID == p.PackID || seenDeps[dep.PackID] {
			return fmt.Errorf("pack_incompatible: invalid or duplicate dependency %q", dep.PackID)
		}
		seenDeps[dep.PackID] = true
	}
	seenTypes := map[string]bool{}
	for _, concept := range p.Concepts {
		if !localIDPattern.MatchString(concept.ID) || strings.TrimSpace(concept.Definition) == "" ||
			seenTypes[concept.ID] {
			return fmt.Errorf("pack_incompatible: invalid or duplicate concept %q", concept.ID)
		}
		switch concept.Kind {
		case "entity", "artifact", "process", "rule":
		default:
			return fmt.Errorf("pack_incompatible: concept %q has invalid kind", concept.ID)
		}
		if concept.Extends != nil && (concept.Extends.PackID == p.PackID && concept.Extends.LocalID == concept.ID) {
			return fmt.Errorf("pack_incompatible: concept %q extends itself", concept.ID)
		}
		seenTypes[concept.ID] = true
	}
	seenRelations := map[string]bool{}
	for _, relation := range p.RelationTypes {
		if !localIDPattern.MatchString(relation.Predicate) || seenRelations[relation.Predicate] ||
			relation.Direction != "forward" || !validCardinality(relation.Cardinality) ||
			!relation.RequiredEvidence || strings.TrimSpace(relation.ReviewRule) == "" {
			return fmt.Errorf("pack_incompatible: invalid relation type %q", relation.Predicate)
		}
		seenRelations[relation.Predicate] = true
	}
	seenConstraints := map[string]bool{}
	for _, c := range p.Constraints {
		if !localIDPattern.MatchString(c.ID) || c.Description == "" || seenConstraints[c.ID] {
			return fmt.Errorf("pack_incompatible: invalid constraint %q", c.ID)
		}
		seenConstraints[c.ID] = true
	}
	return nil
}

func validCardinality(value string) bool {
	switch value {
	case "one-to-one", "one-to-many", "many-to-one", "many-to-many":
		return true
	}
	return false
}

// Resolve checks exact dependency version and digest, validates type links,
// and returns a deterministic dependency-first order. Nothing is merged by
// display label or by a pack-local ID from another namespace.
func Resolve(packs []LoadedPack) ([]LoadedPack, error) {
	byID := map[string]LoadedPack{}
	for _, loaded := range packs {
		if err := loaded.Pack.Validate(); err != nil {
			return nil, err
		}
		if !digestPattern.MatchString(loaded.Digest) || byID[loaded.Pack.PackID].Pack.PackID != "" {
			return nil, fmt.Errorf("pack_incompatible: duplicate pack or digest for %q", loaded.Pack.PackID)
		}
		byID[loaded.Pack.PackID] = loaded
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	state := map[string]int{}
	var ordered []LoadedPack
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("pack_incompatible: dependency cycle at %q", id)
		}
		if state[id] == 2 {
			return nil
		}
		loaded, ok := byID[id]
		if !ok {
			return fmt.Errorf("pack_incompatible: missing pack %q", id)
		}
		state[id] = 1
		deps := append([]Dependency(nil), loaded.Pack.Requires...)
		sort.Slice(deps, func(i, j int) bool { return deps[i].PackID < deps[j].PackID })
		for _, dep := range deps {
			found, ok := byID[dep.PackID]
			if !ok || found.Pack.Version != dep.Version || found.Digest != dep.Digest {
				return fmt.Errorf("pack_incompatible: missing or mismatched dependency %q", dep.PackID)
			}
			if err := visit(dep.PackID); err != nil {
				return err
			}
		}
		state[id] = 2
		ordered = append(ordered, loaded)
		return nil
	}
	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	for _, loaded := range ordered {
		allowed := map[string]bool{"core": true, loaded.Pack.PackID: true}
		for _, dep := range loaded.Pack.Requires {
			allowed[dep.PackID] = true
		}
		checkRef := func(ref TypeRef) error {
			if !allowed[ref.PackID] {
				return fmt.Errorf("pack_incompatible: undeclared type namespace %q", ref.PackID)
			}
			if ref.PackID == "core" {
				if _, ok := semantic.CoreConceptKinds[ref.LocalID]; !ok {
					return fmt.Errorf("pack_incompatible: unknown core type %q", ref.LocalID)
				}
			} else {
				found := false
				for _, c := range byID[ref.PackID].Pack.Concepts {
					if c.ID == ref.LocalID {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("pack_incompatible: unknown type %s#%s", ref.PackID, ref.LocalID)
				}
			}
			return nil
		}
		for _, c := range loaded.Pack.Concepts {
			if c.Extends != nil {
				if err := checkRef(*c.Extends); err != nil {
					return nil, err
				}
			}
		}
		for _, relation := range loaded.Pack.RelationTypes {
			if err := checkRef(relation.SubjectType); err != nil {
				return nil, err
			}
			if err := checkRef(relation.ObjectType); err != nil {
				return nil, err
			}
		}
		local := map[string]*TypeRef{}
		for _, c := range loaded.Pack.Concepts {
			local[c.ID] = c.Extends
		}
		state := map[string]int{}
		var checkExtends func(string) error
		checkExtends = func(id string) error {
			if state[id] == 1 {
				return fmt.Errorf("pack_incompatible: concept inheritance cycle at %s#%s", loaded.Pack.PackID, id)
			}
			if state[id] == 2 {
				return nil
			}
			state[id] = 1
			if parent := local[id]; parent != nil && parent.PackID == loaded.Pack.PackID {
				if err := checkExtends(parent.LocalID); err != nil {
					return err
				}
			}
			state[id] = 2
			return nil
		}
		for id := range local {
			if err := checkExtends(id); err != nil {
				return nil, err
			}
		}
	}
	return ordered, nil
}
