package semantic

import (
	"context"
	"sort"
)

// TraceState is a diagnostic about reviewed links, never an automatic claim
// that implementation or acceptance tests pass.
type TraceState string

const (
	TraceSpecUnapproved        TraceState = "spec_unapproved"
	TraceMissingConcept        TraceState = "missing_concept"
	TraceConceptUnreviewed     TraceState = "concept_unreviewed"
	TraceMissingCode           TraceState = "missing_code"
	TraceMissingTest           TraceState = "missing_test"
	TraceMissingAcceptanceTest TraceState = "missing_acceptance_test"
	TraceConflict              TraceState = "conflict"
	TraceLinked                TraceState = "linked"
)

type TracePath struct {
	ConceptID               string   `json:"concept_id"`
	CodeCanonicalID         string   `json:"code_canonical_id"`
	TestCanonicalID         string   `json:"test_canonical_id"`
	ImplementationAssertion string   `json:"implementation_assertion"`
	TestedByAssertion       string   `json:"tested_by_assertion"`
	AcceptedCriterionIDs    []string `json:"accepted_criterion_ids"`
}

type RequirementTrace struct {
	RequirementID      string      `json:"requirement_id"`
	Version            int         `json:"version"`
	State              TraceState  `json:"state"`
	Missing            []string    `json:"missing"`
	ConflictAssertions []string    `json:"conflict_assertions"`
	Paths              []TracePath `json:"paths"`
}

type TraceReport struct {
	Snapshot     Snapshot           `json:"snapshot"`
	Requirements []RequirementTrace `json:"requirements"`
}

// TraceAligned refuses to report across stale graph/vector/source snapshots.
func (s *Store) TraceAligned(ctx context.Context, projectID, repoRoot, graphDir, vectorDir string) (TraceReport, error) {
	active, err := s.CurrentAligned(ctx, projectID, repoRoot, graphDir, vectorDir)
	if err != nil {
		return TraceReport{}, err
	}
	return active.Trace()
}

// Trace reports completeness of reviewed links. A linked path only proves
// reviewed traceability; test execution results are a separate W4.4 gate.
func (a ActiveProjection) Trace() (TraceReport, error) {
	p := a.projection
	if err := p.Validate(); err != nil {
		return TraceReport{}, err
	}
	report := TraceReport{Snapshot: p.Snapshot, Requirements: make([]RequirementTrace, 0, len(p.Requirements))}
	concepts := map[string]Concept{}
	for _, c := range p.Concepts {
		concepts[c.ID] = c
	}
	tested := map[string][]Assertion{}
	accepted := map[string]map[string]bool{}
	claimConcepts := map[string]map[string]bool{}
	for _, edge := range p.Assertions {
		if edge.Status != StatusVerified {
			continue
		}
		switch edge.Predicate {
		case PredicateTestedBy:
			tested[edge.SubjectID] = append(tested[edge.SubjectID], edge)
		case PredicateAcceptedBy:
			if accepted[edge.ObjectID] == nil {
				accepted[edge.ObjectID] = map[string]bool{}
			}
			accepted[edge.ObjectID][edge.SubjectID] = true
		case PredicateAbout:
			if claimConcepts[edge.SubjectID] == nil {
				claimConcepts[edge.SubjectID] = map[string]bool{}
			}
			claimConcepts[edge.SubjectID][edge.ObjectID] = true
		}
	}
	conflicts := map[string][]string{}
	for _, edge := range p.Assertions {
		if edge.Status != StatusVerified || edge.Predicate != PredicateContradicts {
			continue
		}
		for conceptID := range claimConcepts[edge.SubjectID] {
			if claimConcepts[edge.ObjectID][conceptID] {
				conflicts[conceptID] = append(conflicts[conceptID], edge.ID)
			}
		}
	}
	for _, requirement := range p.Requirements {
		item := RequirementTrace{RequirementID: requirement.ID, Version: requirement.Version,
			Missing: []string{}, ConflictAssertions: []string{}, Paths: []TracePath{}}
		if requirement.Status != StatusVerified {
			item.State = TraceSpecUnapproved
			report.Requirements = append(report.Requirements, item)
			continue
		}
		if len(requirement.ConceptIDs) == 0 {
			item.State = TraceMissingConcept
			report.Requirements = append(report.Requirements, item)
			continue
		}
		unreviewed, noCode, noTest := false, false, false
		coveredCriteria := map[string]bool{}
		for _, conceptID := range requirement.ConceptIDs {
			if concepts[conceptID].Status != StatusVerified {
				item.Missing = append(item.Missing, "unreviewed concept: "+conceptID)
				unreviewed = true
				continue
			}
			item.ConflictAssertions = append(item.ConflictAssertions, conflicts[conceptID]...)
			impls := a.VerifiedImplementations(conceptID)
			if len(impls) == 0 {
				item.Missing = append(item.Missing, "implementation: "+conceptID)
				noCode = true
				continue
			}
			conceptHasTest := false
			for _, impl := range impls {
				for _, edge := range tested[impl.CanonicalID] {
					conceptHasTest = true
					path := TracePath{ConceptID: conceptID, CodeCanonicalID: impl.CanonicalID,
						TestCanonicalID: edge.ObjectID, ImplementationAssertion: impl.AssertionID,
						TestedByAssertion: edge.ID, AcceptedCriterionIDs: []string{}}
					for _, criterion := range requirement.AcceptanceCriteria {
						if accepted[edge.ObjectID][criterion.ID] {
							path.AcceptedCriterionIDs = append(path.AcceptedCriterionIDs, criterion.ID)
							coveredCriteria[criterion.ID] = true
						}
					}
					item.Paths = append(item.Paths, path)
				}
			}
			if !conceptHasTest {
				item.Missing = append(item.Missing, "test for concept: "+conceptID)
				noTest = true
			}
		}
		missingAcceptance := false
		for _, criterion := range requirement.AcceptanceCriteria {
			if !coveredCriteria[criterion.ID] {
				item.Missing = append(item.Missing, "acceptance test: "+criterion.ID)
				missingAcceptance = true
			}
		}
		sort.Strings(item.Missing)
		sort.Strings(item.ConflictAssertions)
		sort.Slice(item.Paths, func(i, j int) bool {
			x, y := item.Paths[i], item.Paths[j]
			if x.ConceptID != y.ConceptID {
				return x.ConceptID < y.ConceptID
			}
			if x.CodeCanonicalID != y.CodeCanonicalID {
				return x.CodeCanonicalID < y.CodeCanonicalID
			}
			return x.TestCanonicalID < y.TestCanonicalID
		})
		switch {
		case unreviewed:
			item.State = TraceConceptUnreviewed
		case len(item.ConflictAssertions) > 0:
			item.State = TraceConflict
		case noCode:
			item.State = TraceMissingCode
		case noTest:
			item.State = TraceMissingTest
		case missingAcceptance:
			item.State = TraceMissingAcceptanceTest
		default:
			item.State = TraceLinked
		}
		report.Requirements = append(report.Requirements, item)
	}
	sort.Slice(report.Requirements, func(i, j int) bool {
		return report.Requirements[i].RequirementID < report.Requirements[j].RequirementID
	})
	return report, nil
}
