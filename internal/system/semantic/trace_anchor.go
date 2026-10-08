package semantic

import "fmt"

// TraceAnchors are exact, reviewed source spans for one requirement path.
// A link is traceability evidence only; it does not assert that a test ran or
// that a human accepted the criterion.
type TraceAnchors struct {
	Requirement EvidenceSpan
	Criterion   EvidenceSpan
	Code        EvidenceSpan
	Test        EvidenceSpan
	Path        TracePath
}

// ResolveTracePath accepts only one fully linked path with the requested
// criterion and CKG canonical IDs. Call it on an ActiveProjection obtained
// through LoadAlignedRetained or CurrentAligned, never on arbitrary JSON.
func (a ActiveProjection) ResolveTracePath(requirementID, criterionID, codeID, testID string) (TraceAnchors, error) {
	trace, err := a.Trace()
	if err != nil {
		return TraceAnchors{}, err
	}
	evidence := map[string]EvidenceSpan{}
	for _, span := range a.projection.Evidence {
		evidence[span.ID] = span
	}
	assertions := map[string]Assertion{}
	for _, assertion := range a.projection.Assertions {
		assertions[assertion.ID] = assertion
	}
	var result TraceAnchors
	found := false
	for _, item := range trace.Requirements {
		if item.RequirementID != requirementID || item.State != TraceLinked {
			continue
		}
		for _, path := range item.Paths {
			if path.CodeCanonicalID != codeID || path.TestCanonicalID != testID || !tracePathHasCriterion(path, criterionID) {
				continue
			}
			if found {
				return TraceAnchors{}, fmt.Errorf("evidence_unverified: ambiguous reviewed trace path")
			}
			found = true
			result.Path = path
		}
	}
	if !found {
		return TraceAnchors{}, fmt.Errorf("evidence_unverified: reviewed requirement path is missing")
	}
	for _, requirement := range a.projection.Requirements {
		if requirement.ID != requirementID || requirement.Status != StatusVerified {
			continue
		}
		result.Requirement = evidence[requirement.EvidenceID]
		for _, criterion := range requirement.AcceptanceCriteria {
			if criterion.ID == criterionID {
				result.Criterion = evidence[criterion.EvidenceID]
			}
		}
	}
	var codeOK, testOK bool
	result.Code, codeOK = assertionAnchor(assertions[result.Path.ImplementationAssertion], evidence, codeID, SourceCode)
	result.Test, testOK = assertionAnchor(assertions[result.Path.TestedByAssertion], evidence, testID, SourceTest)
	if result.Requirement.Kind != SourceDocument || result.Criterion.Kind != SourceDocument || !codeOK || !testOK {
		return TraceAnchors{}, fmt.Errorf("evidence_unverified: reviewed trace lacks exact source anchors")
	}
	return result, nil
}

func tracePathHasCriterion(path TracePath, id string) bool {
	for _, criterion := range path.CriterionIDs() {
		if criterion == id {
			return true
		}
	}
	return false
}
