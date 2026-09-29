package semantic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteLinkedTestKeepsOutcomeSeparateFromTrace(t *testing.T) {
	p, repo := traceFixtureWithRepo(t)
	a := ActiveProjection{projection: p}
	good, err := a.ExecuteLinkedTest(context.Background(), repo, "ac-alpha", []string{"go", "version"})
	if err != nil || !good.CommandPassed || !good.SnapshotConsistent || good.OutputBytes == 0 || good.OutputSHA256 == "" ||
		good.TestCanonicalID != "pkg.TestAlpha" || len(good.TestedByAssertions) != 1 || good.TestedByAssertions[0] != "tested" ||
		len(good.AcceptedAssertions) != 1 || good.AcceptedAssertions[0] != "accepted" {
		t.Fatalf("successful command: %+v, %v", good, err)
	}
	failed, err := a.ExecuteLinkedTest(context.Background(), repo, "ac-alpha", []string{"go", "invalid-command"})
	if err != nil || failed.CommandPassed || failed.ExitCode == 0 || !failed.SnapshotConsistent {
		t.Fatalf("failed command: %+v, %v", failed, err)
	}
	if _, err := a.ExecuteLinkedTest(context.Background(), repo, "other", []string{"go", "version"}); err == nil {
		t.Fatal("unlinked criterion executed")
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.go"), []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ExecuteLinkedTest(context.Background(), repo, "ac-alpha", []string{"go", "version"}); err == nil {
		t.Fatal("dirty source executed")
	}
}

func TestExecuteLinkedTestRequiresExactReviewedTargetWhenAmbiguous(t *testing.T) {
	p, repo := traceFixtureWithRepo(t)
	second := p.Evidence[len(p.Evidence)-1]
	second.ID, second.CanonicalID = "test-beta", "pkg.TestBeta"
	p.Evidence = append(p.Evidence, second)
	p.Assertions = append(p.Assertions,
		Assertion{ID: "tested-beta", Predicate: PredicateTestedBy, SubjectID: "pkg.Alpha", ObjectID: "pkg.TestBeta",
			EvidenceIDs: []string{"code", "test-beta"}, Status: StatusVerified, ReviewedBy: "reviewer"},
		Assertion{ID: "accepted-beta", Predicate: PredicateAcceptedBy, SubjectID: "ac-alpha", ObjectID: "pkg.TestBeta",
			EvidenceIDs: []string{"criterion", "test-beta"}, Status: StatusVerified, ReviewedBy: "reviewer"})
	a := ActiveProjection{projection: p}
	if _, err := a.ExecuteLinkedTest(context.Background(), repo, "ac-alpha", []string{"go", "version"}); err == nil {
		t.Fatal("ambiguous criterion ran without selecting an exact test")
	}
	if _, err := a.ExecuteLinkedTestFor(context.Background(), repo, "ac-alpha", "pkg.Other", []string{"go", "version"}); err == nil {
		t.Fatal("unreviewed test target ran")
	}
	selected, err := a.ExecuteLinkedTestFor(context.Background(), repo, "ac-alpha", "pkg.TestBeta", []string{"go", "version"})
	if err != nil || !selected.CommandPassed || selected.TestCanonicalID != "pkg.TestBeta" ||
		len(selected.TestedByAssertions) != 1 || selected.TestedByAssertions[0] != "tested-beta" ||
		len(selected.AcceptedAssertions) != 1 || selected.AcceptedAssertions[0] != "accepted-beta" {
		t.Fatalf("selected reviewed test: %+v, %v", selected, err)
	}
}
