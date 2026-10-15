// Package comparison supplies explicitly selected, versioned comparison
// vocabulary. A common concept is never a global code or project identity.
package comparison

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
)

var ErrSchemeInvalid = errors.New("comparison: scheme_invalid")

// SchemeDefinition is a sidecar to the existing knowledge Pack. The pack
// owns IDs/definitions; this document adds questions, aliases and boundaries.
type SchemeDefinition struct {
	SchemaVersion int           `json:"schema_version"`
	PackID        string        `json:"pack_id"`
	Version       string        `json:"version"`
	Industry      string        `json:"industry"`
	Concepts      []ConceptView `json:"concepts"`
}

type ConceptView struct {
	ID              string   `json:"id"`
	Aliases         []string `json:"aliases"`
	Includes        []string `json:"includes"`
	Excludes        []string `json:"excludes"`
	ApplicableRoles []string `json:"applicable_roles"`
	Facets          []Facet  `json:"facets"`
}

type Facet struct {
	ID         string   `json:"id"`
	Question   string   `json:"question"`
	Conditions []string `json:"conditions"`
}

// ConceptKey pins the entire pack directory, including the sidecar bytes.
type ConceptKey struct {
	PackID    string `json:"pack_id"`
	Version   string `json:"version"`
	Digest    string `json:"digest"`
	ConceptID string `json:"concept_id"`
}

type Concept struct {
	Key        ConceptKey  `json:"key"`
	Definition string      `json:"definition"`
	View       ConceptView `json:"view"`
}

type Resolution struct {
	State      string    `json:"state"`
	Candidates []Concept `json:"candidates"`
}

type Scheme struct {
	pack       knowledgepack.LoadedPack
	definition SchemeDefinition
}

// LoadScheme uses the current local pack loader and source reader. It neither
// installs a pack nor modifies any dataset. A second digest detects changes.
func LoadScheme(root string) (*Scheme, error) {
	pack, err := knowledgepack.Load(root)
	if err != nil {
		return nil, err
	}
	// Resolve the selected vocabulary with the existing type/dependency rules.
	// Missing dependencies are rejected; this loader never downloads them.
	if _, err = knowledgepack.Resolve([]knowledgepack.LoadedPack{pack}); err != nil {
		return nil, err
	}
	data, err := setup.ReadSourceFileNoFollow(root, "comparison.json", 1<<20)
	if err != nil {
		return nil, err
	}
	var definition SchemeDefinition
	if err = strictJSON(data, &definition); err != nil {
		return nil, ErrSchemeInvalid
	}
	if err = validateDefinition(pack.Pack, definition); err != nil {
		return nil, err
	}
	after, err := knowledgepack.Load(root)
	if err != nil {
		return nil, err
	}
	if pack.Digest != after.Digest {
		return nil, ErrSchemeInvalid
	}
	return &Scheme{pack: pack, definition: definition}, nil
}

func normalize(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func stringsValid(items []string, limit int) bool {
	if len(items) == 0 || len(items) > limit {
		return false
	}
	seen := map[string]bool{}
	for _, item := range items {
		n := normalize(item)
		if n == "" || len(item) > 2048 || seen[n] {
			return false
		}
		seen[n] = true
	}
	return true
}

func validateDefinition(pack knowledgepack.Pack, d SchemeDefinition) error {
	if d.SchemaVersion != 1 || d.PackID != pack.PackID || d.Version != pack.Version || normalize(d.Industry) == "" || len(d.Industry) > 128 || len(d.Concepts) == 0 || len(d.Concepts) > 64 || len(d.Concepts) != len(pack.Concepts) {
		return ErrSchemeInvalid
	}
	ids := map[string]bool{}
	for _, c := range pack.Concepts {
		ids[c.ID] = true
	}
	seen := map[string]bool{}
	for _, c := range d.Concepts {
		if !ids[c.ID] || seen[c.ID] || !stringsValid(c.Aliases, 64) || !stringsValid(c.Includes, 32) || !stringsValid(c.Excludes, 32) || !stringsValid(c.ApplicableRoles, 16) || len(c.Facets) == 0 || len(c.Facets) > 32 {
			return ErrSchemeInvalid
		}
		seen[c.ID] = true
		facets := map[string]bool{}
		for _, f := range c.Facets {
			if !localID(f.ID) || facets[f.ID] || normalize(f.Question) == "" || len(f.Question) > 4096 || !stringsValid(f.Conditions, 16) {
				return ErrSchemeInvalid
			}
			facets[f.ID] = true
		}
	}
	return nil
}

func localID(s string) bool {
	if s == "" || len(s) > 128 || s[0] < 'a' || s[0] > 'z' || strings.HasSuffix(s, "-") || strings.Contains(s, "--") {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func (s *Scheme) concept(id string) Concept {
	for _, c := range s.definition.Concepts {
		if c.ID != id {
			continue
		}
		// Do not return mutable slices owned by the long-lived selected scheme.
		data, _ := json.Marshal(c)
		var view ConceptView
		_ = json.Unmarshal(data, &view)
		for _, p := range s.pack.Pack.Concepts {
			if p.ID == id {
				return Concept{Key: ConceptKey{s.pack.Pack.PackID, s.pack.Pack.Version, s.pack.Digest, id}, Definition: p.Definition, View: view}
			}
		}
	}
	return Concept{}
}

func (s *Scheme) Concepts() []Concept {
	result := make([]Concept, 0, len(s.definition.Concepts))
	for _, c := range s.definition.Concepts {
		result = append(result, s.concept(c.ID))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key.ConceptID < result[j].Key.ConceptID })
	return result
}

// Resolve is a bounded lexical lookup over the selected scheme. Ambiguous
// aliases preserve all candidates; no fuzzy/LLM interpretation is claimed.
func (s *Scheme) Resolve(term string) Resolution {
	result := Resolution{State: "unknown", Candidates: []Concept{}}
	needle := normalize(term)
	if needle == "" || len(term) > 4096 {
		return result
	}
	for _, c := range s.Concepts() {
		match := normalize(c.Key.ConceptID) == needle
		for _, alias := range c.View.Aliases {
			match = match || normalize(alias) == needle
		}
		if match {
			result.Candidates = append(result.Candidates, c)
		}
	}
	if len(result.Candidates) == 1 {
		result.State = "exact_term"
	} else if len(result.Candidates) > 1 {
		result.State = "ambiguous"
	}
	return result
}

func strictJSON(data []byte, target any) error {
	if len(data) > 1<<20 {
		return ErrSchemeInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueJSON(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrSchemeInvalid
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func uniqueJSON(d *json.Decoder, depth int) error {
	if depth > 32 {
		return ErrSchemeInvalid
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return ErrSchemeInvalid
	}
	seen := map[string]bool{}
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return ErrSchemeInvalid
			}
			seen[name] = true
		}
		if err := uniqueJSON(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
