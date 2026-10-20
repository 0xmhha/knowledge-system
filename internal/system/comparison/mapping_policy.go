package comparison

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

var ErrMappingReviewInvalid = errors.New("comparison: mapping_review_invalid")
var mappingVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

type MappingMeaning struct {
	Kind                  string `json:"kind"`
	Direction             string `json:"direction"`
	Meaning               string `json:"meaning"`
	AutomaticTransitivity bool   `json:"automatic_transitivity"`
	EntityIdentity        bool   `json:"entity_identity"`
}

// MappingSemantics describes only a direct local-role -> common-concept link.
// Even exact never substitutes scoped entity IDs or derives links transitively.
func MappingSemantics(kind string) (MappingMeaning, error) {
	meanings := map[string]string{
		"exact":    "The proposed local role matches the shared definition within the explicit conditions.",
		"close":    "The proposed local role has substantial overlap; differences must remain explicit.",
		"broader":  "The local role includes a broader scope than the shared concept under these conditions.",
		"narrower": "The local role covers a narrower scope than the shared concept under these conditions.",
		"related":  "The roles are relevant to compare without asserting inclusion or equivalence.",
	}
	meaning, ok := meanings[kind]
	if !ok {
		return MappingMeaning{}, ErrBindingInvalid
	}
	return MappingMeaning{Kind: kind, Direction: "local_to_shared", Meaning: meaning, AutomaticTransitivity: false, EntityIdentity: false}, nil
}

type MappingReviewPolicy struct {
	MinApprovals int `json:"min_approvals"`
}

// Review records are supplied audit data, not generated human responses.
// A digest binds exact source and interpretation; reviewer labels are not an
// authenticated identity or independent quality acceptance.
type MappingReview struct {
	ID            string                 `json:"id"`
	BindingID     string                 `json:"binding_id"`
	Concept       ConceptKey             `json:"concept"`
	Coordinates   contract.V2Coordinates `json:"coordinates"`
	Conditions    Conditions             `json:"conditions"`
	BindingDigest string                 `json:"binding_digest"`
	Decision      string                 `json:"decision"`
	ReviewerID    string                 `json:"reviewer_id"`
	Reason        string                 `json:"reason"`
	ReviewedAt    string                 `json:"reviewed_at"`
}

type MappingReviewDocument struct {
	SchemaVersion int                 `json:"schema_version"`
	Policy        MappingReviewPolicy `json:"policy"`
	Records       []MappingReview     `json:"records"`
}

type MappingReviewAssessment struct {
	State                         string   `json:"state"`
	Validity                      string   `json:"validity"`
	RequiredApprovals             int      `json:"required_approvals"`
	Approvals                     int      `json:"approvals"`
	Rejections                    int      `json:"rejections"`
	UsedReviewDigests             []string `json:"used_review_digests"`
	StaleReviewCount              int      `json:"stale_review_count"`
	FutureReviewCount             int      `json:"future_review_count"`
	ReviewerIdentityAuthenticated bool     `json:"reviewer_identity_authenticated"`
	IndependentQualityAcceptance  bool     `json:"independent_quality_acceptance"`
}

type AssessedBinding struct {
	Binding   ObservedBinding         `json:"binding"`
	Review    MappingReviewAssessment `json:"review"`
	Semantics MappingMeaning          `json:"semantics"`
}

func canonicalConditions(c Conditions) Conditions {
	at, _ := time.Parse(time.RFC3339, c.ObservedAt)
	c.ObservedAt = at.UTC().Format(time.RFC3339Nano)
	return c
}

func (s *Scheme) validateMappingReview(r MappingReview) error {
	if !localID(r.ID) || !localID(r.BindingID) || !localID(r.ReviewerID) || !annotation(r.Reason) || !validConditions(r.Conditions) || !hash64(r.BindingDigest) || r.Concept.PackID != s.pack.Pack.PackID || !localID(r.Concept.ConceptID) || !mappingVersion.MatchString(r.Concept.Version) || !hash64(r.Concept.Digest) {
		return ErrMappingReviewInvalid
	}
	if _, err := (collection.ScopedRef{Coordinates: r.Coordinates, OriginID: "repo", Path: "comparison-review", LocalID: r.ID}).Key(); err != nil {
		return ErrMappingReviewInvalid
	}
	switch r.Decision {
	case "verified", "rejected", "withdrawn":
	default:
		return ErrMappingReviewInvalid
	}
	reviewed, err := time.Parse(time.RFC3339, r.ReviewedAt)
	if err != nil {
		return ErrMappingReviewInvalid
	}
	observed, _ := time.Parse(time.RFC3339, r.Conditions.ObservedAt)
	if reviewed.Before(observed) {
		return ErrMappingReviewInvalid
	}
	return nil
}

func mappingReviewDigest(r MappingReview) string {
	data, _ := json.Marshal(r)
	sum := sha256.Sum256(append([]byte("cks.mapping-review.v1\x00"), data...))
	return hex.EncodeToString(sum[:])
}

func (s *Scheme) validateMappingReviewDocument(d MappingReviewDocument) error {
	if d.SchemaVersion != 1 || d.Policy.MinApprovals < 1 || d.Policy.MinApprovals > 8 || len(d.Records) > 512 {
		return ErrMappingReviewInvalid
	}
	type recordID struct {
		Coordinates               contract.V2Coordinates
		Conditions                Conditions
		Concept                   ConceptKey
		BindingID, ReviewerID, ID string
	}
	seen := map[recordID]string{}
	for _, r := range d.Records {
		if s.validateMappingReview(r) != nil {
			return ErrMappingReviewInvalid
		}
		key := recordID{r.Coordinates, canonicalConditions(r.Conditions), r.Concept, r.BindingID, r.ReviewerID, r.ID}
		digest := mappingReviewDigest(r)
		if prior, exists := seen[key]; exists && prior != digest {
			return ErrMappingReviewInvalid
		}
		seen[key] = digest
	}
	return nil
}
func (s *Scheme) DecodeMappingReviews(data []byte) (MappingReviewDocument, error) {
	var d MappingReviewDocument
	if strictJSON(data, &d) != nil || s.validateMappingReviewDocument(d) != nil {
		return MappingReviewDocument{}, ErrMappingReviewInvalid
	}
	return d, nil
}
func ownReviewBinding(b ProjectBinding) (ProjectBinding, error) {
	if len(b.Evidence) > evidencev2.MaxCitations {
		return ProjectBinding{}, ErrBindingInvalid
	}
	b.Evidence = append([]BindingEvidence(nil), b.Evidence...)
	for i := range b.Evidence {
		if len(b.Evidence[i].Facets) > 32 {
			return ProjectBinding{}, ErrBindingInvalid
		}
		b.Evidence[i].Facets = append([]string(nil), b.Evidence[i].Facets...)
	}
	return b, nil
}

func sameLogicalMapping(b ProjectBinding, r MappingReview) bool {
	return r.BindingID == b.ID && r.Coordinates.ProjectID == b.Coordinates.ProjectID && r.Concept.PackID == b.Concept.PackID && r.Concept.ConceptID == b.Concept.ConceptID && r.Conditions.Network == b.Conditions.Network && r.Conditions.Configuration == b.Conditions.Configuration && r.Conditions.ComponentRole == b.Conditions.ComponentRole
}

// AssessMappingReview never loads other bindings or traverses a relation.
// Only exact current pins count; changed versions/meaning/source need review.
func (s *Scheme) AssessMappingReview(b ProjectBinding, d MappingReviewDocument, asOf time.Time) (MappingReviewAssessment, error) {
	b, err := ownReviewBinding(b)
	if err != nil {
		return MappingReviewAssessment{}, err
	}
	if len(d.Records) > 512 {
		return MappingReviewAssessment{}, ErrMappingReviewInvalid
	}
	d.Records = append([]MappingReview(nil), d.Records...)
	if err = s.validateMappingReviewDocument(d); err != nil {
		return MappingReviewAssessment{}, err
	}
	current, err := s.BindingDigest(b)
	if err != nil {
		return MappingReviewAssessment{}, err
	}
	observed, _ := time.Parse(time.RFC3339, b.Conditions.ObservedAt)
	if asOf.IsZero() || asOf.Before(observed) || d.SchemaVersion != 1 || d.Policy.MinApprovals < 1 || d.Policy.MinApprovals > 8 || len(d.Records) > 512 {
		return MappingReviewAssessment{}, ErrMappingReviewInvalid
	}
	assessment := MappingReviewAssessment{State: "proposed", Validity: "unknown", RequiredApprovals: d.Policy.MinApprovals, UsedReviewDigests: []string{}}
	type vote struct {
		at        time.Time
		decisions map[string]bool
		digests   map[string]bool
	}
	voters := map[string]*vote{}
	for _, r := range d.Records {
		if s.validateMappingReview(r) != nil {
			return MappingReviewAssessment{}, ErrMappingReviewInvalid
		}
		if r.BindingDigest == current && (r.BindingID != b.ID || r.Coordinates != b.Coordinates || r.Concept != b.Concept || canonicalConditions(r.Conditions) != canonicalConditions(b.Conditions)) {
			return MappingReviewAssessment{}, ErrMappingReviewInvalid
		}
		if !sameLogicalMapping(b, r) {
			continue
		}
		at, _ := time.Parse(time.RFC3339, r.ReviewedAt)
		if at.After(asOf) {
			assessment.FutureReviewCount++
			continue
		}
		if r.BindingDigest != current {
			assessment.StaleReviewCount++
			continue
		}
		if r.Coordinates != b.Coordinates || r.Concept != b.Concept || canonicalConditions(r.Conditions) != canonicalConditions(b.Conditions) {
			return MappingReviewAssessment{}, ErrMappingReviewInvalid
		}
		prior := voters[r.ReviewerID]
		if prior != nil && at.Before(prior.at) {
			continue
		}
		if prior == nil || at.After(prior.at) {
			prior = &vote{at: at, decisions: map[string]bool{}, digests: map[string]bool{}}
			voters[r.ReviewerID] = prior
		}
		prior.decisions[r.Decision] = true
		prior.digests[mappingReviewDigest(r)] = true
	}
	if len(voters) == 0 {
		if assessment.StaleReviewCount > 0 {
			assessment.Validity = "stale"
		}
		return assessment, nil
	}
	assessment.Validity = "current"
	conflict, withdrawals := false, 0
	for _, v := range voters {
		for digest := range v.digests {
			assessment.UsedReviewDigests = append(assessment.UsedReviewDigests, digest)
		}
		if len(v.decisions) != 1 {
			conflict = true
			continue
		}
		if v.decisions["verified"] {
			assessment.Approvals++
		} else if v.decisions["rejected"] {
			assessment.Rejections++
		} else {
			withdrawals++
		}
	}
	sort.Strings(assessment.UsedReviewDigests)
	if conflict || assessment.Approvals > 0 && assessment.Rejections > 0 {
		assessment.State = "disputed"
		assessment.Validity = "conflict"
	} else if assessment.Rejections > 0 {
		assessment.State = "rejected"
	} else if assessment.Approvals >= d.Policy.MinApprovals {
		assessment.State = "verified"
	} else if assessment.Approvals == 0 && withdrawals > 0 {
		assessment.State = "withdrawn"
	}
	return assessment, nil
}

// ReadReviewedBinding first rechecks retained bytes, then applies supplied
// recorded reviews. It never promotes a stale source or infers a reviewer.
func (r *MappingReader) ReadReviewedBinding(ctx context.Context, collectionID string, b ProjectBinding, d MappingReviewDocument, asOf time.Time) (AssessedBinding, error) {
	b, err := ownReviewBinding(b)
	if err != nil {
		return AssessedBinding{}, err
	}
	if len(d.Records) > 512 {
		return AssessedBinding{}, ErrMappingReviewInvalid
	}
	d.Records = append([]MappingReview(nil), d.Records...)
	observed, err := r.ReadBinding(ctx, collectionID, b)
	if err != nil {
		return AssessedBinding{}, err
	}
	review, err := r.scheme.AssessMappingReview(b, d, asOf)
	if err != nil {
		return AssessedBinding{}, err
	}
	meaning, err := MappingSemantics(b.Mapping)
	if err != nil {
		return AssessedBinding{}, err
	}
	observed.ReviewState = review.State
	if review.State == "verified" {
		for i, w := range observed.Warnings {
			if w == "mapping_semantics_not_reviewed" {
				observed.Warnings[i] = "supplied_mapping_review_not_independent_acceptance"
			}
		}
	}
	observed.Warnings = append(observed.Warnings, "reviewer_identity_not_authenticated", "mapping_review_does_not_approve_policy_contents")
	return AssessedBinding{Binding: observed, Review: review, Semantics: meaning}, nil
}
