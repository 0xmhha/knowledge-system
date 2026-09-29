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
	if err != nil || !good.CommandPassed || !good.SnapshotConsistent || good.OutputBytes == 0 || good.OutputSHA256 == "" {
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
