package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ReviewItem contains everything needed to inspect one proposed claim in
// its original source. It is a review queue item, not a verified assertion.
type ReviewItem struct {
	ClaimID   string         `json:"claim_id"`
	Statement string         `json:"statement"`
	Heading   string         `json:"heading"`
	Evidence  []EvidenceSpan `json:"evidence"`
}

type AssertionReviewItem struct {
	AssertionID string         `json:"assertion_id"`
	Predicate   string         `json:"predicate"`
	SubjectID   string         `json:"subject_id"`
	ObjectID    string         `json:"object_id"`
	Evidence    []EvidenceSpan `json:"evidence"`
}

type ConceptReviewItem struct {
	Concept  Concept      `json:"concept"`
	Evidence EvidenceSpan `json:"evidence"`
}

type RequirementReviewItem struct {
	Requirement Requirement  `json:"requirement"`
	Evidence    EvidenceSpan `json:"evidence"`
}

// ReviewQueueItem makes every semantic proposal auditable even when the
// randomized precision sample is smaller than the complete proposal set.
type ReviewQueueItem struct {
	Kind       string         `json:"kind"`
	ID         string         `json:"id"`
	Status     Status         `json:"status"`
	Evidence   []EvidenceSpan `json:"evidence"`
	ReviewedBy string         `json:"reviewed_by,omitempty"`
	HoldReason string         `json:"hold_reason,omitempty"`
}

// AmbiguousTerm names one surface form shared by multiple concepts. It is a
// reviewer warning, not an automatic equivalence or a validation error.
type AmbiguousTerm struct {
	Lang       string   `json:"lang"`
	Normalized string   `json:"normalized"`
	ConceptIDs []string `json:"concept_ids"`
}

// ReviewReport separates the measured precision of adjudicated claims from
// coverage. A nil precision means nobody reviewed a claim yet; reporting 0
// or 1 there would give a false quality signal.
type ReviewReport struct {
	ProjectID           string                  `json:"project_id"`
	DatasetID           string                  `json:"dataset_id"`
	Total               int                     `json:"total"`
	Proposed            int                     `json:"proposed"`
	Verified            int                     `json:"verified"`
	Rejected            int                     `json:"rejected"`
	Precision           *float64                `json:"reviewed_precision"`
	Sample              []ReviewItem            `json:"sample"`
	AssertionTotal      int                     `json:"assertion_total"`
	AssertionProposed   int                     `json:"assertion_proposed"`
	AssertionVerified   int                     `json:"assertion_verified"`
	AssertionRejected   int                     `json:"assertion_rejected"`
	AssertionPrecision  *float64                `json:"assertion_reviewed_precision"`
	AssertionSample     []AssertionReviewItem   `json:"assertion_sample"`
	ConceptTotal        int                     `json:"concept_total"`
	ConceptProposed     int                     `json:"concept_proposed"`
	ConceptVerified     int                     `json:"concept_verified"`
	ConceptRejected     int                     `json:"concept_rejected"`
	ConceptPrecision    *float64                `json:"concept_reviewed_precision"`
	ConceptSample       []ConceptReviewItem     `json:"concept_sample"`
	AmbiguousTerms      []AmbiguousTerm         `json:"ambiguous_terms"`
	RequirementTotal    int                     `json:"requirement_total"`
	RequirementProposed int                     `json:"requirement_proposed"`
	RequirementVerified int                     `json:"requirement_verified"`
	RequirementRejected int                     `json:"requirement_rejected"`
	RequirementSample   []RequirementReviewItem `json:"requirement_sample"`
	ReviewQueue         []ReviewQueueItem       `json:"review_queue"`
}

// Review produces a deterministic sample of proposed claims. Salt permits
// repeatable independent samples; ordering by hash avoids favoring the first
// document in the extraction order. Only human-adjudicated claims count in
// precision. Call ValidateSources before using the report for a gate.
func (p Projection) Review(limit int, salt string) (ReviewReport, error) {
	if err := p.Validate(); err != nil {
		return ReviewReport{}, err
	}
	if limit < 0 {
		limit = 0
	}
	r := ReviewReport{ProjectID: p.Snapshot.ProjectID, DatasetID: p.Snapshot.DatasetID,
		Total: len(p.Claims), AssertionTotal: len(p.Assertions), ConceptTotal: len(p.Concepts), RequirementTotal: len(p.Requirements),
		Sample: []ReviewItem{}, AssertionSample: []AssertionReviewItem{},
		ConceptSample: []ConceptReviewItem{}, AmbiguousTerms: []AmbiguousTerm{}, RequirementSample: []RequirementReviewItem{}}
	r.ReviewQueue = []ReviewQueueItem{}
	sections := make(map[string]DocumentSection, len(p.Sections))
	for _, section := range p.Sections {
		sections[section.ID] = section
	}
	evidence := make(map[string]EvidenceSpan, len(p.Evidence))
	for _, e := range p.Evidence {
		evidence[e.ID] = e
	}
	queue := func(kind, id string, status Status, reviewer string, ids ...string) {
		item := ReviewQueueItem{Kind: kind, ID: id, Status: status, ReviewedBy: reviewer,
			Evidence: make([]EvidenceSpan, 0, len(ids))}
		if status == StatusProposed {
			item.HoldReason = "awaiting_human_review"
		} else if status == StatusRejected {
			item.HoldReason = "rejected_by_reviewer"
		}
		for _, evidenceID := range ids {
			item.Evidence = append(item.Evidence, evidence[evidenceID])
		}
		r.ReviewQueue = append(r.ReviewQueue, item)
	}
	type ranked struct {
		key  string
		item ReviewItem
	}
	var candidates []ranked
	for _, claim := range p.Claims {
		queue("claim", claim.ID, claim.Status, claim.ReviewedBy, claim.EvidenceIDs...)
		switch claim.Status {
		case StatusVerified:
			r.Verified++
		case StatusRejected:
			r.Rejected++
		case StatusProposed:
			r.Proposed++
			item := ReviewItem{ClaimID: claim.ID, Statement: claim.Statement, Heading: sections[claim.SectionID].Heading}
			for _, id := range claim.EvidenceIDs {
				item.Evidence = append(item.Evidence, evidence[id])
			}
			keyBytes := sha256.Sum256([]byte(salt + "\x00" + p.Snapshot.DatasetID + "\x00" + claim.ID))
			candidates = append(candidates, ranked{key: hex.EncodeToString(keyBytes[:]), item: item})
		}
	}
	if reviewed := r.Verified + r.Rejected; reviewed > 0 {
		precision := float64(r.Verified) / float64(reviewed)
		r.Precision = &precision
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].key == candidates[j].key {
			return candidates[i].item.ClaimID < candidates[j].item.ClaimID
		}
		return candidates[i].key < candidates[j].key
	})
	claimLimit := limit
	if claimLimit > len(candidates) {
		claimLimit = len(candidates)
	}
	for _, candidate := range candidates[:claimLimit] {
		r.Sample = append(r.Sample, candidate.item)
	}
	type rankedAssertion struct {
		key  string
		item AssertionReviewItem
	}
	var assertionCandidates []rankedAssertion
	for _, assertion := range p.Assertions {
		queue("assertion", assertion.ID, assertion.Status, assertion.ReviewedBy, assertion.EvidenceIDs...)
		switch assertion.Status {
		case StatusVerified:
			r.AssertionVerified++
		case StatusRejected:
			r.AssertionRejected++
		case StatusProposed:
			r.AssertionProposed++
			item := AssertionReviewItem{AssertionID: assertion.ID, Predicate: assertion.Predicate,
				SubjectID: assertion.SubjectID, ObjectID: assertion.ObjectID}
			for _, id := range assertion.EvidenceIDs {
				item.Evidence = append(item.Evidence, evidence[id])
			}
			keyBytes := sha256.Sum256([]byte(salt + "\x00" + p.Snapshot.DatasetID + "\x00" + assertion.ID))
			assertionCandidates = append(assertionCandidates, rankedAssertion{key: hex.EncodeToString(keyBytes[:]), item: item})
		}
	}
	if reviewed := r.AssertionVerified + r.AssertionRejected; reviewed > 0 {
		precision := float64(r.AssertionVerified) / float64(reviewed)
		r.AssertionPrecision = &precision
	}
	sort.Slice(assertionCandidates, func(i, j int) bool {
		if assertionCandidates[i].key == assertionCandidates[j].key {
			return assertionCandidates[i].item.AssertionID < assertionCandidates[j].item.AssertionID
		}
		return assertionCandidates[i].key < assertionCandidates[j].key
	})
	assertionLimit := limit
	if assertionLimit > len(assertionCandidates) {
		assertionLimit = len(assertionCandidates)
	}
	for _, candidate := range assertionCandidates[:assertionLimit] {
		r.AssertionSample = append(r.AssertionSample, candidate.item)
	}
	type rankedConcept struct {
		key  string
		item ConceptReviewItem
	}
	var conceptCandidates []rankedConcept
	for _, concept := range p.Concepts {
		queue("concept", concept.ID, concept.Status, concept.ReviewedBy, concept.EvidenceID)
		switch concept.Status {
		case StatusVerified:
			r.ConceptVerified++
		case StatusRejected:
			r.ConceptRejected++
		case StatusProposed:
			r.ConceptProposed++
			keyBytes := sha256.Sum256([]byte(salt + "\x00" + p.Snapshot.DatasetID + "\x00" + concept.ID))
			conceptCandidates = append(conceptCandidates, rankedConcept{key: hex.EncodeToString(keyBytes[:]),
				item: ConceptReviewItem{Concept: concept, Evidence: evidence[concept.EvidenceID]}})
		}
	}
	if reviewed := r.ConceptVerified + r.ConceptRejected; reviewed > 0 {
		precision := float64(r.ConceptVerified) / float64(reviewed)
		r.ConceptPrecision = &precision
	}
	sort.Slice(conceptCandidates, func(i, j int) bool {
		if conceptCandidates[i].key == conceptCandidates[j].key {
			return conceptCandidates[i].item.Concept.ID < conceptCandidates[j].item.Concept.ID
		}
		return conceptCandidates[i].key < conceptCandidates[j].key
	})
	conceptLimit := limit
	if conceptLimit > len(conceptCandidates) {
		conceptLimit = len(conceptCandidates)
	}
	for _, candidate := range conceptCandidates[:conceptLimit] {
		r.ConceptSample = append(r.ConceptSample, candidate.item)
	}
	type rankedRequirement struct {
		key  string
		item RequirementReviewItem
	}
	var requirementCandidates []rankedRequirement
	for _, requirement := range p.Requirements {
		queue("requirement", requirement.ID, requirement.Status, requirement.ReviewedBy, requirement.EvidenceID)
		switch requirement.Status {
		case StatusVerified:
			r.RequirementVerified++
		case StatusRejected:
			r.RequirementRejected++
		case StatusProposed:
			r.RequirementProposed++
			keyBytes := sha256.Sum256([]byte(salt + "\x00" + p.Snapshot.DatasetID + "\x00" + requirement.ID))
			requirementCandidates = append(requirementCandidates, rankedRequirement{key: hex.EncodeToString(keyBytes[:]),
				item: RequirementReviewItem{Requirement: requirement, Evidence: evidence[requirement.EvidenceID]}})
		}
	}
	sort.Slice(requirementCandidates, func(i, j int) bool {
		if requirementCandidates[i].key == requirementCandidates[j].key {
			return requirementCandidates[i].item.Requirement.ID < requirementCandidates[j].item.Requirement.ID
		}
		return requirementCandidates[i].key < requirementCandidates[j].key
	})
	requirementLimit := limit
	if requirementLimit > len(requirementCandidates) {
		requirementLimit = len(requirementCandidates)
	}
	for _, candidate := range requirementCandidates[:requirementLimit] {
		r.RequirementSample = append(r.RequirementSample, candidate.item)
	}
	terms := map[string][]string{}
	for _, concept := range p.Concepts {
		for _, term := range concept.Terms {
			key := term.Lang + "\x00" + strings.ToLower(strings.TrimSpace(term.Value))
			terms[key] = append(terms[key], concept.ID)
		}
	}
	for key, ids := range terms {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		parts := strings.SplitN(key, "\x00", 2)
		r.AmbiguousTerms = append(r.AmbiguousTerms, AmbiguousTerm{
			Lang: parts[0], Normalized: parts[1], ConceptIDs: ids,
		})
	}
	sort.Slice(r.ReviewQueue, func(i, j int) bool {
		if r.ReviewQueue[i].Kind == r.ReviewQueue[j].Kind {
			return r.ReviewQueue[i].ID < r.ReviewQueue[j].ID
		}
		return r.ReviewQueue[i].Kind < r.ReviewQueue[j].Kind
	})
	sort.Slice(r.AmbiguousTerms, func(i, j int) bool {
		if r.AmbiguousTerms[i].Lang == r.AmbiguousTerms[j].Lang {
			return r.AmbiguousTerms[i].Normalized < r.AmbiguousTerms[j].Normalized
		}
		return r.AmbiguousTerms[i].Lang < r.AmbiguousTerms[j].Lang
	})
	return r, nil
}
