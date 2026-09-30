package knowledgecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestCoreOnlyKnowledgeInitLockAndDrift(t *testing.T) {
	root := t.TempDir()
	args := []string{"--project-root", root}
	if _, err := execute(t, append(args, "init", "--project-id", "fixture")...); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".cks", "knowledge", "manifest.yaml")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, append(args, "init", "--project-id", "fixture")...); err == nil {
		t.Fatal("init overwrote user manifest")
	}
	after, err := os.ReadFile(manifest)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("init changed existing manifest")
	}
	if out, err := execute(t, append(args, "validate")...); err != nil || !strings.Contains(out, `"status":"unlocked"`) {
		t.Fatalf("unlocked validate: %s, %v", out, err)
	}
	if _, err := execute(t, append(args, "lock")...); err != nil {
		t.Fatal(err)
	}
	if out, err := execute(t, append(args, "validate")...); err != nil || !strings.Contains(out, `"status":"locked"`) {
		t.Fatalf("locked validate: %s, %v", out, err)
	}
	policy := filepath.Join(root, ".cks", "knowledge", "policies", "BR-17.yaml")
	if err := os.WriteFile(policy, []byte("status: proposed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, append(args, "validate")...); err == nil || !strings.Contains(err.Error(), "pack_lock_mismatch") {
		t.Fatalf("stale lock accepted: %v", err)
	}
}
