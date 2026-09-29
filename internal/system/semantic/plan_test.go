package semantic

import "testing"

func TestPlanIsReadOnlyAndNeverTreatsLinksAsPassingTests(t *testing.T) {
	p := traceFixture(t)
	plan, err := (ActiveProjection{projection: p}).Plan()
	if err != nil || len(plan.Steps) != 1 {
		t.Fatalf("plan: %+v, %v", plan, err)
	}
	step := plan.Steps[0]
	if step.Action != PlanExecuteAcceptanceTest || !step.Unconfirmed || len(step.EvidenceIDs) != 1 ||
		step.EvidenceIDs[0] != "requirement" || len(step.CodeSymbols) != 1 || len(step.TestSymbols) != 1 {
		t.Fatalf("linked trace incorrectly accepted as tested: %+v", step)
	}
	without := p
	without.Assertions = nil
	plan, err = (ActiveProjection{projection: without}).Plan()
	if err != nil || plan.Steps[0].Action != PlanLinkImplementation || !plan.Steps[0].Unconfirmed {
		t.Fatalf("missing code plan: %+v, %v", plan, err)
	}
	// Plan must not mutate its source projection or invent edges.
	if len(without.Assertions) != 0 || len(p.Assertions) != 3 {
		t.Fatal("plan mutated semantic projection")
	}
}
