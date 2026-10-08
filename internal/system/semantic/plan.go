package semantic

import "context"

type PlanAction string

const (
	PlanReviewSpec            PlanAction = "review_spec"
	PlanModelConcept          PlanAction = "model_concept"
	PlanReviewConcept         PlanAction = "review_concept"
	PlanLinkImplementation    PlanAction = "link_implementation"
	PlanLinkTest              PlanAction = "link_test"
	PlanLinkAcceptanceTest    PlanAction = "link_acceptance_test"
	PlanResolveConflict       PlanAction = "resolve_conflict"
	PlanExecuteAcceptanceTest PlanAction = "execute_acceptance_test"
)

// PlanStep is advisory. It has no patch, side effect, or inferred approval.
type PlanStep struct {
	RequirementID string     `json:"requirement_id"`
	Action        PlanAction `json:"action"`
	Reason        string     `json:"reason"`
	EvidenceIDs   []string   `json:"evidence_ids"`
	AssertionIDs  []string   `json:"assertion_ids"`
	Missing       []string   `json:"missing"`
	CodeSymbols   []string   `json:"code_symbols"`
	TestSymbols   []string   `json:"test_symbols"`
	Unconfirmed   bool       `json:"unconfirmed"`
}

type ChangePlan struct {
	Snapshot Snapshot   `json:"snapshot"`
	Steps    []PlanStep `json:"steps"`
}

func (s *Store) PlanAligned(ctx context.Context, projectID, repoRoot, graphDir, vectorDir string) (ChangePlan, error) {
	active, err := s.CurrentAligned(ctx, projectID, repoRoot, graphDir, vectorDir)
	if err != nil {
		return ChangePlan{}, err
	}
	return active.Plan()
}

// Plan maps each reviewed trace gap to one explicit human/developer action.
// Even a fully linked trace needs execution evidence before acceptance passes.
func (a ActiveProjection) Plan() (ChangePlan, error) {
	trace, err := a.Trace()
	if err != nil {
		return ChangePlan{}, err
	}
	plan := ChangePlan{Snapshot: trace.Snapshot, Steps: []PlanStep{}}
	requirements := map[string]Requirement{}
	for _, r := range a.projection.Requirements {
		requirements[r.ID] = r
	}
	for _, item := range trace.Requirements {
		requirement := requirements[item.RequirementID]
		step := PlanStep{RequirementID: item.RequirementID, EvidenceIDs: []string{requirement.EvidenceID},
			AssertionIDs: []string{}, Missing: append([]string{}, item.Missing...),
			CodeSymbols: []string{}, TestSymbols: []string{}, Unconfirmed: true}
		for _, path := range item.Paths {
			step.CodeSymbols = appendUnique(step.CodeSymbols, path.CodeCanonicalID)
			step.TestSymbols = appendUnique(step.TestSymbols, path.TestCanonicalID)
		}
		switch item.State {
		case TraceSpecUnapproved:
			step.Action, step.Reason = PlanReviewSpec, "approve or reject the source specification"
		case TraceMissingConcept:
			step.Action, step.Reason = PlanModelConcept, "attach source-backed domain concepts"
		case TraceConceptUnreviewed:
			step.Action, step.Reason = PlanReviewConcept, "review the referenced concepts before trusting implementation links"
		case TraceMissingCode:
			step.Action, step.Reason = PlanLinkImplementation, "review concept-to-code anchors in the same snapshot"
		case TraceMissingTest:
			step.Action, step.Reason = PlanLinkTest, "review code-to-test anchors in the same snapshot"
		case TraceMissingAcceptanceTest:
			step.Action, step.Reason = PlanLinkAcceptanceTest, "link each acceptance criterion to a reviewed test"
		case TraceConflict:
			step.Action, step.Reason = PlanResolveConflict, "adjudicate contradictory reviewed claims before planning a change"
			step.AssertionIDs = append(step.AssertionIDs, item.ConflictAssertions...)
		case TraceLinked:
			step.Action, step.Reason = PlanExecuteAcceptanceTest, "run the linked acceptance tests and record their results"
		default:
			continue
		}
		plan.Steps = append(plan.Steps, step)
	}
	return plan, nil
}

func appendUnique(in []string, value string) []string {
	for _, current := range in {
		if current == value {
			return in
		}
	}
	return append(in, value)
}
