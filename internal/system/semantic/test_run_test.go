package semantic

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteLinkedTestKeepsOutcomeSeparateFromTrace(t *testing.T) {
	p, repo := traceFixtureWithRepo(t)
	a := ActiveProjection{projection: p}
	good, err := a.ExecuteLinkedTest(context.Background(), repo, "ac-alpha", []string{"go", "version"})
	if err != nil || !good.CommandPassed || !good.SnapshotConsistent || good.OutputBytes == 0 || good.OutputSHA256 == "" ||
		good.TestCanonicalID != "pkg.TestAlpha" || len(good.TestedByAssertions) != 1 || good.TestedByAssertions[0] != "tested" ||
		len(good.CheckedAssertions) != 1 || good.CheckedAssertions[0] != "accepted" || len(good.AcceptedAssertions) != 0 {
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

func TestExactGoTestCommandResolvesCommittedASTAnchor(t *testing.T) {
	repo := t.TempDir()
	gitOutput(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.test/pilot\n\ngo 1.24\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := "package pilot\nimport \"testing\"\nfunc TestAlpha(t *testing.T) { t.Log(\"ok\") }\nfunc TestBeta(t *testing.T) { t.Log(\"other\") }\n"
	if err := os.WriteFile(filepath.Join(repo, "pilot_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "nested", "nested_test.go"), []byte("package nested\nimport \"testing\"\nfunc TestNested(t *testing.T) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, repo, "add", ".")
	gitOutput(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	commit := gitOutput(t, repo, "rev-parse", "HEAD")
	a := ActiveProjection{projection: Projection{Evidence: []EvidenceSpan{{Kind: SourceTest, Path: "pilot_test.go", StartLine: 3, EndLine: 3, CanonicalID: "example.test/pilot.TestAlpha"}}}}
	argv, name, err := a.exactGoTestCommand(repo, commit, "example.test/pilot.TestAlpha")
	if err != nil || name != "TestAlpha" || strings.Join(argv, " ") != "go test -json -count=1 -run ^TestAlpha$ ." {
		t.Fatalf("resolved command=%v name=%q err=%v", argv, name, err)
	}
	a.projection.Evidence[0].StartLine = 4
	if _, _, err := a.exactGoTestCommand(repo, commit, "example.test/pilot.TestAlpha"); err == nil {
		t.Fatal("wrong source span resolved a different test")
	}
	a.projection.Evidence[0] = EvidenceSpan{Kind: SourceTest, Path: "nested/nested_test.go", StartLine: 3, EndLine: 3, CanonicalID: "example.test/pilot/nested.TestNested"}
	argv, name, err = a.exactGoTestCommand(repo, commit, "example.test/pilot/nested.TestNested")
	if err != nil || name != "TestNested" || strings.Join(argv, " ") != "go test -json -count=1 -run ^TestNested$ ./nested" {
		t.Fatalf("nested command=%v name=%q err=%v", argv, name, err)
	}
}

func TestGoTestObserverRequiresExactRunAndPass(t *testing.T) {
	o := &goTestObserver{output: &hashOutput{hash: sha256.New()}, testName: "TestAlpha"}
	for _, part := range []string{
		"{\"Action\":\"run\",\"Test\":\"TestBeta\"}\n",
		"{\"Action\":\"run\",\"Test\":\"TestAlpha\"}\n",
		"{\"Action\":\"pass\",\"Test\":\"TestBeta\"}\n",
	} {
		if _, err := o.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if !o.run || o.pass {
		t.Fatalf("wrong test passed or selected test not observed: %+v", o)
	}
	if _, err := o.Write([]byte("{\"Action\":\"pass\",\"Test\":\"TestAlpha\"}\n")); err != nil || !o.pass {
		t.Fatalf("selected pass not observed: %+v, %v", o, err)
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
		Assertion{ID: "accepted-beta", Predicate: PredicateCheckedBy, SubjectID: "ac-alpha", ObjectID: "pkg.TestBeta",
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
		len(selected.CheckedAssertions) != 1 || selected.CheckedAssertions[0] != "accepted-beta" || len(selected.AcceptedAssertions) != 0 {
		t.Fatalf("selected reviewed test: %+v, %v", selected, err)
	}
}

func TestExecuteLinkedTestDetectsSourceMutation(t *testing.T) {
	p, repo := traceFixtureWithRepo(t)
	a := ActiveProjection{projection: p}
	report, err := a.ExecuteLinkedTest(context.Background(), repo, "ac-alpha", []string{"sh", "-c", "printf 'changed\\n' > new.md"})
	if err != nil || !report.CommandPassed || report.SnapshotConsistent {
		t.Fatalf("source mutation was accepted: %+v, %v", report, err)
	}
}
