package semantic

import "testing"

func TestResolveTracePathRequiresOneReviewedExactCodeAndTest(t *testing.T) {
	p := traceFixture(t)
	active := ActiveProjection{projection: p}
	anchors, err := active.ResolveTracePath("req-alpha", "ac-alpha", "pkg.Alpha", "pkg.TestAlpha")
	if err != nil || anchors.Requirement.ID != "requirement" || anchors.Criterion.ID != "criterion" ||
		anchors.Code.ID != "code" || anchors.Test.ID != "test" || anchors.Path.ImplementationAssertion != "impl" {
		t.Fatalf("reviewed path anchors: %+v %v", anchors, err)
	}
	for _, ids := range [][4]string{
		{"req-other", "ac-alpha", "pkg.Alpha", "pkg.TestAlpha"},
		{"req-alpha", "ac-other", "pkg.Alpha", "pkg.TestAlpha"},
		{"req-alpha", "ac-alpha", "pkg.Other", "pkg.TestAlpha"},
		{"req-alpha", "ac-alpha", "pkg.Alpha", "pkg.TestOther"},
	} {
		if _, err := active.ResolveTracePath(ids[0], ids[1], ids[2], ids[3]); err == nil {
			t.Fatalf("foreign path accepted: %v", ids)
		}
	}
	p.Assertions[2].Status, p.Assertions[2].ReviewedBy = StatusProposed, ""
	if _, err := (ActiveProjection{projection: p}).ResolveTracePath("req-alpha", "ac-alpha", "pkg.Alpha", "pkg.TestAlpha"); err == nil {
		t.Fatal("unreviewed CHECKED_BY path accepted")
	}
}
