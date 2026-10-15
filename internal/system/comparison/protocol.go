package comparison

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

var (
	ErrComparisonProfile = errors.New("comparison: profile_invalid")
	ErrComparisonRequest = errors.New("comparison: request_invalid")
	ErrComparisonStale   = errors.New("comparison: stale_configuration")
)

const (
	MaxComparisonBodyBytes = 256 * 1024
	MaxComparisonCitations = 128
	MaxComparisonTimeoutMS = 30000
)

type ComparisonBudget struct {
	MaxBodyBytes int `json:"max_body_bytes"`
	MaxCitations int `json:"max_citations"`
	TimeoutMS    int `json:"timeout_ms"`
}

func DefaultComparisonBudget() ComparisonBudget {
	return ComparisonBudget{MaxComparisonBodyBytes, MaxComparisonCitations, MaxComparisonTimeoutMS}
}

// ComparisonProfileDocument selects local files only at server startup. No
// request can supply a source root, file path, backend or replacement profile.
// Decoding validates shape, not source availability; runtime pins bytes later.
type ComparisonProfileDocument struct {
	SchemaVersion     int                  `json:"schema_version"`
	Name              string               `json:"name"`
	CatalogPath       string               `json:"catalog_path"`
	SchemeRoot        string               `json:"scheme_root"`
	BindingsPath      string               `json:"bindings_path,omitempty"`
	ReviewsPath       string               `json:"reviews_path,omitempty"`
	SanitizeRulesPath string               `json:"sanitize_rules_path"`
	Scopes            []ProjectDomainScope `json:"scopes"`
}

func DecodeComparisonProfile(data []byte) (ComparisonProfileDocument, error) {
	var d ComparisonProfileDocument
	if strictJSON(data, &d) != nil || d.SchemaVersion != 1 || len(d.Name) > 128 || len(d.Scopes) == 0 || len(d.Scopes) > 128 {
		return ComparisonProfileDocument{}, ErrComparisonProfile
	}
	for _, name := range []string{d.CatalogPath, d.SchemeRoot, d.SanitizeRulesPath} {
		if !canonicalAbsolute(name) {
			return ComparisonProfileDocument{}, ErrComparisonProfile
		}
	}
	for _, name := range []string{d.BindingsPath, d.ReviewsPath} {
		if name != "" && !canonicalAbsolute(name) {
			return ComparisonProfileDocument{}, ErrComparisonProfile
		}
	}
	seen := map[string]bool{}
	for _, scope := range d.Scopes {
		key, err := ComparisonScopeID(scope)
		if err != nil || seen[key] {
			return ComparisonProfileDocument{}, ErrComparisonProfile
		}
		seen[key] = true
	}
	return d, nil
}
func canonicalAbsolute(name string) bool {
	return filepath.IsAbs(name) && filepath.Clean(name) == name && len(name) <= 4096 && !strings.ContainsRune(name, 0)
}

func ComparisonScopeID(s ProjectDomainScope) (string, error) {
	if !validConditions(s.Conditions) || len(s.Industry) > 128 {
		return "", ErrComparisonProfile
	}
	if _, err := (collection.ScopedRef{Coordinates: s.Coordinates, OriginID: "repo", Path: "comparison-scope", LocalID: "scope"}).Key(); err != nil {
		return "", ErrComparisonProfile
	}
	s.Industry = normalize(s.Industry)
	s.Conditions = canonicalConditions(s.Conditions)
	data, err := json.Marshal(s)
	if err != nil {
		return "", ErrComparisonProfile
	}
	sum := sha256.Sum256(append([]byte("cks.comparison-scope.v1\x00"), data...))
	return hex.EncodeToString(sum[:]), nil
}

type ScopedParticipant struct {
	ScopeID string             `json:"scope_id"`
	Scope   ProjectDomainScope `json:"scope"`
}

type ComparisonRequest struct {
	SchemaVersion   int              `json:"schema_version"`
	ProfileDigest   string           `json:"profile_digest"`
	CatalogDigest   string           `json:"catalog_digest"`
	CollectionID    string           `json:"collection_id"`
	Mode            string           `json:"mode"`
	Participants    []string         `json:"participants"`
	Concept         ConceptKey       `json:"concept"`
	Facets          []string         `json:"facets"`
	Question        string           `json:"question,omitempty"`
	AsOf            string           `json:"as_of"`
	Budget          ComparisonBudget `json:"budget"`
	IncludeProposed bool             `json:"include_proposed"`
}

type ComparisonSelection struct {
	ProfileDigest   string
	CatalogDigest   string
	CollectionID    string
	Participants    []ScopedParticipant
	Concept         Concept
	Facets          []string
	Question        string
	AsOf            time.Time
	Budget          ComparisonBudget
	IncludeProposed bool
}

// ComparisonProtocol owns one immutable routing/configuration snapshot.
// profileDigest must be computed from loaded profile/file pins by the runtime;
// this contract constructor neither opens a dataset nor claims read readiness.
type ComparisonProtocol struct {
	profileDigest string
	catalogDigest string
	scheme        *Scheme
	scopes        map[string]ProjectDomainScope
	members       map[string]map[contract.V2Coordinates]bool
}

func NewComparisonProtocol(catalog collection.Catalog, scheme *Scheme, scopes []ProjectDomainScope, profileDigest string) (*ComparisonProtocol, error) {
	if scheme == nil || !hash64(profileDigest) || len(scopes) == 0 || len(scopes) > 128 {
		return nil, ErrComparisonProfile
	}
	raw, err := catalog.CanonicalBytes()
	if err != nil {
		return nil, ErrComparisonProfile
	}
	owned, err := collection.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrComparisonProfile
	}
	sum := sha256.Sum256(append([]byte("cks.collection-catalog.v1\x00"), raw...))
	p := &ComparisonProtocol{profileDigest: profileDigest, catalogDigest: hex.EncodeToString(sum[:]), scheme: scheme, scopes: map[string]ProjectDomainScope{}, members: map[string]map[contract.V2Coordinates]bool{}}
	versions := map[contract.V2Coordinates]bool{}
	for _, v := range owned.Versions {
		versions[v.Coordinates] = true
	}
	for _, scope := range scopes {
		key, err := ComparisonScopeID(scope)
		if err != nil || !versions[scope.Coordinates] {
			return nil, ErrComparisonProfile
		}
		if _, exists := p.scopes[key]; exists {
			return nil, ErrComparisonProfile
		}
		p.scopes[key] = scope
	}
	for _, c := range owned.Collections {
		p.members[c.ID] = map[contract.V2Coordinates]bool{}
		for _, coords := range c.Members {
			p.members[c.ID][coords] = true
		}
	}
	return p, nil
}
func (p *ComparisonProtocol) ProfileDigest() string { return p.profileDigest }
func (p *ComparisonProtocol) CatalogDigest() string { return p.catalogDigest }
func (p *ComparisonProtocol) Scopes() []ScopedParticipant {
	result := make([]ScopedParticipant, 0, len(p.scopes))
	for id, scope := range p.scopes {
		result = append(result, ScopedParticipant{ScopeID: id, Scope: scope})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ScopeID < result[j].ScopeID })
	return result
}
func (p *ComparisonProtocol) DecodeRequest(data []byte, now time.Time) (ComparisonSelection, error) {
	var q ComparisonRequest
	if strictJSON(data, &q) != nil {
		return ComparisonSelection{}, ErrComparisonRequest
	}
	return p.Select(q, now)
}
func (p *ComparisonProtocol) Select(q ComparisonRequest, now time.Time) (ComparisonSelection, error) {
	if q.ProfileDigest != p.profileDigest || q.CatalogDigest != p.catalogDigest {
		return ComparisonSelection{}, ErrComparisonStale
	}
	if q.SchemaVersion != 1 || q.Mode != "comparison" || len(q.Participants) < 2 || len(q.Participants) > 16 || len(q.Facets) == 0 || len(q.Facets) > 8 || len(q.Question) > 4096 || now.IsZero() {
		return ComparisonSelection{}, ErrComparisonRequest
	}
	if q.Budget.MaxBodyBytes < 1 || q.Budget.MaxBodyBytes > MaxComparisonBodyBytes || q.Budget.MaxCitations < 1 || q.Budget.MaxCitations > MaxComparisonCitations || q.Budget.TimeoutMS < 1 || q.Budget.TimeoutMS > MaxComparisonTimeoutMS {
		return ComparisonSelection{}, ErrComparisonRequest
	}
	asOf, err := time.Parse(time.RFC3339, q.AsOf)
	if err != nil || asOf.IsZero() || asOf.After(now) {
		return ComparisonSelection{}, ErrComparisonRequest
	}
	concept := p.scheme.concept(q.Concept.ConceptID)
	if q.Concept != concept.Key || q.Concept.PackID == "" {
		return ComparisonSelection{}, ErrComparisonRequest
	}
	member, ok := p.members[q.CollectionID]
	if !ok {
		return ComparisonSelection{}, ErrComparisonRequest
	}
	allowed := map[string]bool{}
	for _, f := range concept.View.Facets {
		allowed[f.ID] = true
	}
	facets := map[string]bool{}
	for _, facet := range q.Facets {
		if !allowed[facet] || facets[facet] {
			return ComparisonSelection{}, ErrComparisonRequest
		}
		facets[facet] = true
	}
	selected := []ScopedParticipant{}
	seen := map[string]bool{}
	for _, id := range q.Participants {
		scope, ok := p.scopes[id]
		if !ok || seen[id] || !member[scope.Coordinates] {
			return ComparisonSelection{}, ErrComparisonRequest
		}
		seen[id] = true
		selected = append(selected, ScopedParticipant{ScopeID: id, Scope: scope})
	}
	return ComparisonSelection{ProfileDigest: p.profileDigest, CatalogDigest: p.catalogDigest, CollectionID: q.CollectionID, Participants: selected, Concept: concept, Facets: append([]string(nil), q.Facets...), Question: q.Question, AsOf: asOf, Budget: q.Budget, IncludeProposed: q.IncludeProposed}, nil
}
