package comparison

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// ProjectDomainScope is explicitly configured observation context, not an
// inferred industry label or a declaration that a project implements a feature.
type ProjectDomainScope struct {
	Coordinates contract.V2Coordinates `json:"coordinates"`
	Industry    string                 `json:"industry"`
	Conditions  Conditions             `json:"conditions"`
}

type CoverageRequest struct {
	Scope              ProjectDomainScope `json:"scope"`
	Concept            ConceptKey         `json:"concept"`
	Facet              string             `json:"facet"`
	RequiredCapability string             `json:"required_capability"`
	IncludeProposed    bool               `json:"include_proposed"`
}

type CoverageResult struct {
	Scope                        ProjectDomainScope `json:"scope"`
	Concept                      ConceptKey         `json:"concept"`
	Facet                        string             `json:"facet"`
	State                        string             `json:"state"`
	Reasons                      []string           `json:"reasons"`
	Mappings                     []AssessedBinding  `json:"mappings"`
	ArchiveVerified              bool               `json:"archive_verified"`
	ClassificationBasis          string             `json:"classification_basis"`
	EntityIdentity               bool               `json:"entity_identity"`
	UnreviewedCandidatesIncluded bool               `json:"unreviewed_candidates_included"`
}

func (r *MappingReader) safeDomainScope(ctx context.Context, s ProjectDomainScope) (ProjectDomainScope, error) {
	for _, p := range []*string{&s.Industry, &s.Conditions.Network, &s.Conditions.Configuration, &s.Conditions.ComponentRole} {
		result, err := r.cleaner.Sanitize(ctx, []sanitize.Sanitizable{{Citation: contract.Citation{File: "comparison-profile", StartLine: 1, EndLine: 1}, Body: *p}})
		if err != nil || result.FailClosed || len(result.Items) != 1 || result.Items[0].Dropped {
			return ProjectDomainScope{}, ErrBindingEvidence
		}
		*p = result.Items[0].Body
	}
	return s, nil
}
func hasFacet(e BindingEvidence, facet string) bool {
	for _, f := range e.Facets {
		if f == facet {
			return true
		}
	}
	return false
}
func declarationKind(kind string) bool {
	return kind == "document" || kind == "configuration" || kind == "policy"
}

// verifyCoverageArchive reuses the collection guard without inventing evidence.
// Empty refs still verify the selected immutable archive/identity. No current
// alias or live source fallback is permitted.
func (r *MappingReader) verifyCoverageArchive(ctx context.Context, collectionID string, coords contract.V2Coordinates) error {
	_, err := r.session.Read(ctx, collection.Request{CatalogDigest: r.session.Digest(), CollectionID: collectionID, Mode: collection.Implementation, Query: "comparison coverage archive verification", Target: &coords}, func(ctx context.Context, v collection.Version, q string) (contract.EvidencePackV2, error) {
		return evidencev2.BuildFromRefs(ctx, v.VersionDir, q, nil, r.cleaner)
	})
	return err
}

// Coverage describes one selected facet in one project scope. It never turns
// missing references into feature absence or compares different projects as
// contradictory claims within a single project.
func (r *MappingReader) Coverage(ctx context.Context, collectionID string, q CoverageRequest, bindings BindingDocument, reviews MappingReviewDocument, asOf time.Time) (CoverageResult, error) {
	if err := ctx.Err(); err != nil {
		return CoverageResult{}, err
	}
	concept := r.scheme.concept(q.Concept.ConceptID)
	if concept.Key != q.Concept || q.Concept.PackID == "" || !validConditions(q.Scope.Conditions) || len(q.Scope.Industry) > 128 || asOf.IsZero() {
		return CoverageResult{}, ErrBindingInvalid
	}
	validFacet := false
	for _, f := range concept.View.Facets {
		validFacet = validFacet || f.ID == q.Facet
	}
	if !validFacet {
		return CoverageResult{}, ErrBindingInvalid
	}
	if _, err := r.session.ScopeKey(collection.Request{CatalogDigest: r.session.Digest(), CollectionID: collectionID, Mode: collection.Implementation, Query: "comparison coverage " + q.Facet, Target: &q.Scope.Coordinates}); err != nil {
		return CoverageResult{}, err
	}
	if len(bindings.Bindings) > 128 {
		return CoverageResult{}, ErrBindingInvalid
	}
	data, err := json.Marshal(bindings)
	if err != nil {
		return CoverageResult{}, ErrBindingInvalid
	}
	owned, err := r.scheme.DecodeBindings(data)
	if err != nil {
		return CoverageResult{}, err
	}
	bindings = owned
	if len(reviews.Records) > 512 {
		return CoverageResult{}, ErrMappingReviewInvalid
	}
	reviews.Records = append([]MappingReview(nil), reviews.Records...)
	if err = r.scheme.validateMappingReviewDocument(reviews); err != nil {
		return CoverageResult{}, err
	}
	scope, err := r.safeDomainScope(ctx, q.Scope)
	if err != nil {
		return CoverageResult{}, err
	}
	result := CoverageResult{Scope: scope, Concept: q.Concept, Facet: q.Facet, State: "unknown", Reasons: []string{}, Mappings: []AssessedBinding{}, ClassificationBasis: "declared_direct_mappings_and_retained_source"}
	finish := func(state, reason string) (CoverageResult, error) {
		result.State = state
		result.Reasons = append(result.Reasons, reason)
		return result, nil
	}
	if q.RequiredCapability != "" && q.RequiredCapability != "retained-source" {
		return finish("unsupported", "capability_not_provided_by_this_comparison_reader_not_project_feature_absence")
	}
	if strings.TrimSpace(q.Scope.Industry) == "" {
		return finish("unknown", "project_industry_not_selected")
	}
	if normalize(q.Scope.Industry) != normalize(r.scheme.definition.Industry) {
		return finish("incomparable", "selected_industry_differs_from_concept_scheme")
	}
	roleAllowed := false
	for _, role := range concept.View.ApplicableRoles {
		roleAllowed = roleAllowed || role == q.Scope.Conditions.ComponentRole
	}
	if !roleAllowed {
		// An explicit negative declaration can be relevant precisely because
		// the component does not play the queried positive role.
		declaredNegative := false
		for _, b := range bindings.Bindings {
			if b.Coordinates == q.Scope.Coordinates && b.Concept == q.Concept && canonicalConditions(b.Conditions) == canonicalConditions(q.Scope.Conditions) && b.Applicability == "not_applicable" {
				for _, e := range b.Evidence {
					declaredNegative = declaredNegative || hasFacet(e, q.Facet) && declarationKind(e.Kind)
				}
			}
		}
		if !declaredNegative {
			return finish("incomparable", "component_role_outside_selected_concept_definition")
		}
	}
	observed, _ := time.Parse(time.RFC3339, q.Scope.Conditions.ObservedAt)
	if asOf.Before(observed) {
		return finish("incomparable", "scope_not_observed_at_requested_time")
	}
	selected, positive, negative, unknown := 0, 0, 0, 0
	similar := false
	for _, b := range bindings.Bindings {
		if b.Coordinates != q.Scope.Coordinates || b.Concept != q.Concept || canonicalConditions(b.Conditions) != canonicalConditions(q.Scope.Conditions) {
			continue
		}
		selected++
		if !roleAllowed && b.Applicability != "not_applicable" {
			result.Reasons = append(result.Reasons, "positive_role_mapping_outside_definition")
			continue
		}
		relevant, declared := false, false
		for _, e := range b.Evidence {
			if hasFacet(e, q.Facet) {
				relevant = true
				declared = declared || declarationKind(e.Kind)
			}
		}
		if !relevant {
			result.Reasons = append(result.Reasons, "facet_evidence_not_collected")
			continue
		}
		if b.Applicability == "not_applicable" && !declared {
			result.Reasons = append(result.Reasons, "applicability_declaration_not_cited_for_requested_facet")
			continue
		}
		assessed, err := r.ReadReviewedBinding(ctx, collectionID, b, reviews, asOf)
		if err != nil {
			if ctx.Err() != nil {
				return CoverageResult{}, ctx.Err()
			}
			if errors.Is(err, collection.ErrClosed) || errors.Is(err, ErrBindingInvalid) || errors.Is(err, ErrMappingReviewInvalid) {
				return CoverageResult{}, err
			}
			result.Mappings = []AssessedBinding{}
			return finish("unavailable", "selected_evidence_unavailable_or_filtered")
		}
		if assessed.Review.State != "verified" || assessed.Review.Validity != "current" {
			if !q.IncludeProposed || assessed.Review.State != "proposed" {
				result.Reasons = append(result.Reasons, "mapping_review_not_current_or_approved")
				continue
			}
			result.UnreviewedCandidatesIncluded = true
		}
		// Only requested facet material crosses this path; all pins were checked.
		evidence := []ObservedBindingEvidence{}
		for _, e := range assessed.Binding.Evidence {
			if hasFacet(e.Reference, q.Facet) {
				evidence = append(evidence, e)
			}
		}
		assessed.Binding.Evidence = evidence
		result.Mappings = append(result.Mappings, assessed)
		switch b.Applicability {
		case "not_applicable":
			negative++
		case "unknown":
			unknown++
		default:
			positive++
			similar = similar || b.Mapping != "exact"
		}
	}
	// Recheck after the last mapping, including earlier source reads. A mutated
	// archive cannot leave an earlier successful row in an apparently good result.
	if err := r.verifyCoverageArchive(ctx, collectionID, q.Scope.Coordinates); err != nil {
		if ctx.Err() != nil {
			return CoverageResult{}, ctx.Err()
		}
		result.Mappings = []AssessedBinding{}
		return finish("unavailable", "selected_archive_verification_failed")
	}
	result.ArchiveVerified = true
	if selected == 0 {
		return finish("not_collected", "no_mapping_collected_for_selected_project_version_and_conditions")
	}
	if positive > 0 && negative > 0 {
		return finish("conflict", "opposing_applicability_claims_within_the_same_project_scope")
	}
	if negative > 0 {
		return finish("not_applicable", "explicit_cited_applicability_declaration_for_this_facet")
	}
	if positive > 0 {
		if similar {
			return finish("similar", "direct_role_overlap_not_entity_equivalence")
		}
		return finish("exact", "direct_mapping_within_declared_conditions_not_entity_equivalence")
	}
	if unknown > 0 {
		return finish("unknown", "provided_mapping_leaves_this_facet_undecided")
	}
	return finish("unknown", "no_usable_reviewed_evidence_for_requested_facet")
}
