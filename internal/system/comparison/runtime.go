package comparison

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

var ErrComparisonClosed = errors.New("comparison: closed")
var ErrComparisonBudget = errors.New("comparison: response_budget_exceeded")

type runtimePin struct {
	Role   string `json:"role"`
	SHA256 string `json:"sha256"`
}

type ComparisonRuntime struct {
	mu         sync.RWMutex
	closed     bool
	name       string
	catalog    collection.Catalog
	scheme     *Scheme
	protocol   *ComparisonProtocol
	bindings   BindingDocument
	reviews    MappingReviewDocument
	cleaner    *sanitize.Engine
	now        func() time.Time
	openReader func(context.Context, collection.Catalog, *Scheme, *sanitize.Engine) (*MappingReader, error)
}

func readComparisonInput(name string) ([]byte, error) {
	if !canonicalAbsolute(name) {
		return nil, ErrComparisonProfile
	}
	resolved, err := filepath.EvalSymlinks(name)
	if err != nil || resolved != name {
		return nil, ErrComparisonProfile
	}
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrComparisonProfile
	}
	data, err := setup.ReadSourceFileNoFollow(filepath.Dir(name), filepath.Base(name), 1<<20)
	if err != nil {
		return nil, ErrComparisonProfile
	}
	return data, nil
}
func runtimeHash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// OpenComparisonRuntime pins configuration bytes, not dataset availability.
// Selected datasets are verified when read, preserving per-project failures.
func OpenComparisonRuntime(ctx context.Context, path string) (*ComparisonRuntime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := readComparisonInput(path)
	if err != nil {
		return nil, err
	}
	profile, err := DecodeComparisonProfile(data)
	if err != nil {
		return nil, err
	}
	pins := []runtimePin{{"profile", runtimeHash(data)}}
	catalogBytes, err := readComparisonInput(profile.CatalogPath)
	if err != nil {
		return nil, err
	}
	pins = append(pins, runtimePin{"catalog", runtimeHash(catalogBytes)})
	catalog, err := collection.Decode(bytes.NewReader(catalogBytes))
	if err != nil {
		return nil, ErrComparisonProfile
	}
	resolved, err := filepath.EvalSymlinks(profile.SchemeRoot)
	if err != nil || resolved != profile.SchemeRoot {
		return nil, ErrComparisonProfile
	}
	scheme, err := LoadScheme(profile.SchemeRoot)
	if err != nil {
		return nil, ErrComparisonProfile
	}
	pins = append(pins, runtimePin{"scheme", scheme.pack.Digest})
	bindings := BindingDocument{SchemaVersion: 2, Bindings: []ProjectBinding{}}
	if profile.BindingsPath != "" {
		raw, err := readComparisonInput(profile.BindingsPath)
		if err != nil {
			return nil, err
		}
		bindings, err = scheme.DecodeBindings(raw)
		if err != nil {
			return nil, err
		}
		pins = append(pins, runtimePin{"bindings", runtimeHash(raw)})
	}
	reviews := MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: 1}, Records: []MappingReview{}}
	if profile.ReviewsPath != "" {
		raw, err := readComparisonInput(profile.ReviewsPath)
		if err != nil {
			return nil, err
		}
		reviews, err = scheme.DecodeMappingReviews(raw)
		if err != nil {
			return nil, err
		}
		pins = append(pins, runtimePin{"reviews", runtimeHash(raw)})
	}
	rulesBytes, err := readComparisonInput(profile.SanitizeRulesPath)
	if err != nil {
		return nil, err
	}
	pins = append(pins, runtimePin{"sanitization", runtimeHash(rulesBytes)})
	rules, err := config.LoadSanitizeRulesetBytes(rulesBytes)
	if err != nil {
		return nil, ErrComparisonProfile
	}
	cleaner, err := sanitize.New(rules)
	if err != nil {
		return nil, ErrComparisonProfile
	}
	sort.Slice(pins, func(i, j int) bool { return pins[i].Role < pins[j].Role })
	pinBytes, err := json.Marshal(pins)
	if err != nil {
		return nil, ErrComparisonProfile
	}
	digest := runtimeHash(append([]byte("cks.comparison-profile.v1\x00"), pinBytes...))
	protocol, err := NewComparisonProtocol(catalog, scheme, profile.Scopes, digest)
	if err != nil {
		return nil, err
	}
	// A profile may retain historical review records, but active binding data
	// must refer to an explicitly configured project/version/condition scope.
	for _, binding := range bindings.Bindings {
		registered := false
		for _, scope := range profile.Scopes {
			registered = registered || scope.Coordinates == binding.Coordinates && canonicalConditions(scope.Conditions) == canonicalConditions(binding.Conditions)
		}
		if !registered {
			return nil, ErrComparisonProfile
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name := profile.Name
	if name == "" {
		name = "cks-comparison"
	}
	return &ComparisonRuntime{name: name, catalog: catalog, scheme: scheme, protocol: protocol, bindings: bindings, reviews: reviews, cleaner: cleaner, now: time.Now, openReader: NewMappingReader}, nil
}
func (s *ComparisonRuntime) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
func (s *ComparisonRuntime) Name() string { return s.name }

type ComparisonDescription struct {
	SchemaVersion         int                     `json:"schema_version"`
	State                 string                  `json:"state"`
	SourceReadiness       string                  `json:"source_readiness"`
	ProfileDigest         string                  `json:"profile_digest"`
	CatalogDigest         string                  `json:"catalog_digest"`
	Collections           []collection.Collection `json:"collections"`
	Scopes                []ScopedParticipant     `json:"scopes"`
	Concepts              []Concept               `json:"concepts"`
	SupportedCapabilities []string                `json:"supported_capabilities"`
	MaximumBudget         ComparisonBudget        `json:"maximum_budget"`
	WriteEnabled          bool                    `json:"write_enabled"`
}

func (s *ComparisonRuntime) DescribeRequest(ctx context.Context, data []byte) (ComparisonDescription, error) {
	var empty struct{}
	if strictJSON(data, &empty) != nil {
		return ComparisonDescription{}, ErrComparisonRequest
	}
	return s.Describe(ctx)
}
func (s *ComparisonRuntime) Describe(ctx context.Context) (ComparisonDescription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ComparisonDescription{}, ErrComparisonClosed
	}
	if err := ctx.Err(); err != nil {
		return ComparisonDescription{}, err
	}
	safe := &MappingReader{cleaner: s.cleaner}
	scopes := s.protocol.Scopes()
	for i := range scopes {
		scope, err := safe.safeDomainScope(ctx, scopes[i].Scope)
		if err != nil {
			return ComparisonDescription{}, err
		}
		scopes[i].Scope = scope
	}
	raw, err := json.Marshal(s.catalog.Collections)
	if err != nil {
		return ComparisonDescription{}, ErrComparisonProfile
	}
	var groups []collection.Collection
	if err = json.Unmarshal(raw, &groups); err != nil {
		return ComparisonDescription{}, ErrComparisonProfile
	}
	return ComparisonDescription{SchemaVersion: 1, State: "configured", SourceReadiness: "not_checked", ProfileDigest: s.protocol.ProfileDigest(), CatalogDigest: s.protocol.CatalogDigest(), Collections: groups, Scopes: scopes, Concepts: s.scheme.Concepts(), SupportedCapabilities: []string{"retained-source"}, MaximumBudget: DefaultComparisonBudget()}, nil
}

type ComparisonGroup struct {
	Participant ScopedParticipant `json:"participant"`
	State       string            `json:"state"`
	Cells       []CoverageResult  `json:"cells"`
}
type CandidateEvidence struct {
	ScopeID       string                `json:"scope_id"`
	BindingDigest string                `json:"binding_digest"`
	Citations     []contract.CitationV2 `json:"citations"`
}
type ComparisonCandidate struct {
	Kind           string              `json:"kind"`
	Facet          string              `json:"facet"`
	Participants   []string            `json:"participants"`
	BindingDigests []string            `json:"binding_digests"`
	Evidence       []CandidateEvidence `json:"evidence"`
	Reasons        []string            `json:"reasons"`
}
type ComparisonBundle struct {
	SchemaVersion     int                   `json:"schema_version"`
	State             string                `json:"state"`
	ProfileDigest     string                `json:"profile_digest"`
	CatalogDigest     string                `json:"catalog_digest"`
	CollectionID      string                `json:"collection_id"`
	Mode              string                `json:"mode"`
	Concept           Concept               `json:"concept"`
	Facets            []string              `json:"facets"`
	AsOf              string                `json:"as_of"`
	Question          string                `json:"question"`
	Groups            []ComparisonGroup     `json:"groups"`
	Candidates        []ComparisonCandidate `json:"candidates"`
	UsedBodyBytes     int                   `json:"used_body_bytes"`
	UsedCitations     int                   `json:"used_citations"`
	CompletenessScope string                `json:"completeness_scope"`
	FinalAssessment   string                `json:"final_assessment"`
	OmittedCandidates int                   `json:"omitted_candidates"`
	CandidateState    string                `json:"candidate_state"`
}

func (s *ComparisonRuntime) memberCatalog(collectionID string, coords contract.V2Coordinates) collection.Catalog {
	c := collection.Catalog{SchemaVersion: 1, Projects: s.catalog.Projects, Collections: []collection.Collection{{ID: collectionID, Members: []contract.V2Coordinates{coords}}}}
	for _, v := range s.catalog.Versions {
		if v.Coordinates == coords {
			c.Versions = append(c.Versions, v)
		}
	}
	return c
}
func successfulCoverage(state string) bool {
	return state == "exact" || state == "similar" || state == "not_applicable"
}
func cellCost(c CoverageResult) (int, int) {
	body, cites := 0, 0
	for _, m := range c.Mappings {
		for _, e := range m.Binding.Evidence {
			body += len(e.Body)
			cites++
		}
	}
	return body, cites
}

// Extract routes only selected scopes, opens pinned readers and returns
// project/facet groups. A group never becomes an implementation patch target.
func (s *ComparisonRuntime) Extract(ctx context.Context, data []byte) (ComparisonBundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ComparisonBundle{}, ErrComparisonClosed
	}
	selected, err := s.protocol.DecodeRequest(data, s.now().UTC())
	if err != nil {
		return ComparisonBundle{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(selected.Budget.TimeoutMS)*time.Millisecond)
	defer cancel()
	result := ComparisonBundle{SchemaVersion: 1, State: "complete", ProfileDigest: selected.ProfileDigest, CatalogDigest: selected.CatalogDigest, CollectionID: selected.CollectionID, Mode: "comparison", Concept: selected.Concept, Facets: selected.Facets, AsOf: selected.AsOf.UTC().Format(time.RFC3339Nano), Question: selected.Question, Groups: []ComparisonGroup{}, Candidates: []ComparisonCandidate{}, CompletenessScope: "requested_project_facet_rows_only_not_all_project_evidence", FinalAssessment: "LLM_or_user_evaluates_tradeoffs_from_cited_evidence"}
	safe := &MappingReader{cleaner: s.cleaner}
	readers := make([]*MappingReader, 0, len(selected.Participants))
	for _, participant := range selected.Participants {
		if err := ctx.Err(); err != nil {
			return ComparisonBundle{}, err
		}
		scope, err := safe.safeDomainScope(ctx, participant.Scope)
		if err != nil {
			return ComparisonBundle{}, err
		}
		display := participant
		display.Scope = scope
		group := ComparisonGroup{Participant: display, State: "complete", Cells: []CoverageResult{}}
		reader, openErr := s.openReader(ctx, s.memberCatalog(selected.CollectionID, participant.Scope.Coordinates), s.scheme, s.cleaner)
		readers = append(readers, reader)
		if reader != nil {
			defer func() { _ = reader.Close() }()
		}
		if err := ctx.Err(); err != nil {
			return ComparisonBundle{}, err
		}
		for _, facet := range selected.Facets {
			var cell CoverageResult
			if openErr != nil {
				cell = CoverageResult{Scope: scope, Concept: selected.Concept.Key, Facet: facet, State: "unavailable", Reasons: []string{"selected_archive_unavailable"}, Mappings: []AssessedBinding{}}
			} else {
				cell, err = reader.Coverage(ctx, selected.CollectionID, CoverageRequest{Scope: participant.Scope, Concept: selected.Concept.Key, Facet: facet, RequiredCapability: "retained-source", IncludeProposed: selected.IncludeProposed}, s.bindings, s.reviews, selected.AsOf)
				if err != nil {
					return ComparisonBundle{}, err
				}
			}
			body, cites := cellCost(cell)
			if body > selected.Budget.MaxBodyBytes-result.UsedBodyBytes || cites > selected.Budget.MaxCitations-result.UsedCitations {
				cell.State = "budget_exceeded"
				cell.Mappings = []AssessedBinding{}
				cell.Reasons = append(cell.Reasons, "whole_facet_omitted_by_explicit_response_budget")
			} else {
				result.UsedBodyBytes += body
				result.UsedCitations += cites
			}
			if !successfulCoverage(cell.State) {
				group.State = "partial"
				result.State = "partial"
			}
			group.Cells = append(group.Cells, cell)
		}
		result.Groups = append(result.Groups, group)
	}
	// Later participants may expose a concurrent change to an earlier archive.
	// Verify every selected reader again before releasing any source evidence.
	for i, reader := range readers {
		if err := ctx.Err(); err != nil {
			return ComparisonBundle{}, err
		}
		if reader == nil {
			continue
		}
		if err := reader.verifyCoverageArchive(ctx, selected.CollectionID, selected.Participants[i].Scope.Coordinates); err != nil {
			if ctx.Err() != nil {
				return ComparisonBundle{}, ctx.Err()
			}
			group := &result.Groups[i]
			group.State, result.State = "partial", "partial"
			for j := range group.Cells {
				cell := &group.Cells[j]
				body, cites := cellCost(*cell)
				result.UsedBodyBytes -= body
				result.UsedCitations -= cites
				cell.State, cell.ArchiveVerified = "unavailable", false
				cell.UnreviewedCandidatesIncluded = false
				cell.Mappings = []AssessedBinding{}
				cell.Reasons = append(cell.Reasons, "selected_archive_failed_final_verification")
			}
		}
	}
	result.CandidateState = "complete"
	// Candidates only describe explicit mappings and differing evidence/conditions.
	// They contain mapping reasons/digests from both scopes, never an automatic
	// behavioral equivalence, performance or superiority conclusion.
	for i, a := range result.Groups {
		for _, b := range result.Groups[i+1:] {
			for index, ac := range a.Cells {
				bc := b.Cells[index]
				if len(ac.Mappings) == 0 || len(bc.Mappings) == 0 {
					continue
				}
				kind := "shared_concept_mapping_candidate"
				if ac.State != bc.State || a.Participant.Scope.Conditions != b.Participant.Scope.Conditions || evidenceContentSignature(ac) != evidenceContentSignature(bc) {
					kind = "different_evidence_or_conditions_candidate"
				}
				candidate := ComparisonCandidate{Kind: kind, Facet: ac.Facet, Participants: []string{a.Participant.ScopeID, b.Participant.ScopeID}, BindingDigests: []string{}, Reasons: []string{}}
				for side, cell := range []CoverageResult{ac, bc} {
					for _, m := range cell.Mappings {
						candidate.BindingDigests = append(candidate.BindingDigests, m.Binding.BindingDigest)
						candidate.Reasons = append(candidate.Reasons, m.Binding.Reason)
						proof := CandidateEvidence{ScopeID: candidate.Participants[side], BindingDigest: m.Binding.BindingDigest, Citations: []contract.CitationV2{}}
						for _, e := range m.Binding.Evidence {
							proof.Citations = append(proof.Citations, e.Reference.Citation)
						}
						candidate.Evidence = append(candidate.Evidence, proof)
					}
				}
				// Count every emitted citation object, including candidate proofs.
				cost := 0
				for _, proof := range candidate.Evidence {
					cost += len(proof.Citations)
				}
				if cost > selected.Budget.MaxCitations-result.UsedCitations {
					result.OmittedCandidates++
					result.CandidateState, result.State = "budget_exceeded", "partial"
					continue
				}
				result.UsedCitations += cost
				result.Candidates = append(result.Candidates, candidate)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return ComparisonBundle{}, err
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > 8<<20 {
		return ComparisonBundle{}, ErrComparisonBudget
	}
	return result, nil
}

func evidenceContentSignature(cell CoverageResult) string {
	hashes := []string{}
	for _, m := range cell.Mappings {
		for _, e := range m.Binding.Evidence {
			hashes = append(hashes, e.Reference.Citation.ContentSHA256)
		}
	}
	sort.Strings(hashes)
	data, _ := json.Marshal(hashes)
	return runtimeHash(data)
}
