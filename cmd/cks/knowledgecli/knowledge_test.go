package knowledgecli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	if out, err := execute(t, append(args, "review")...); err != nil || !strings.Contains(out, `"items":[]`) {
		t.Fatalf("core-only review queue: %s, %v", out, err)
	}
	packDir := filepath.Join(root, "vendor", "sample-pack")
	if err := os.MkdirAll(packDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pack := `pack_schema_version: 1
pack_id: engineering.decisions
version: 1.0.0
owner: project
scope: fixture
requires: []
concepts: [{id: business-policy, kind: rule, definition: A project rule.}]
relation_types: []
constraints: []
competency_questions: ["Which rule applies?"]
`
	if err := os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(pack), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := execute(t, append(args, "digest", "--source", "vendor/sample-pack")...); err != nil || !strings.Contains(out, `"pack_id":"engineering.decisions"`) || !strings.Contains(out, `"sha256":`) {
		t.Fatalf("registered pack digest: %s, %v", out, err)
	}
	policy := filepath.Join(root, ".cks", "knowledge", "domain", "fixture.yaml")
	if err := os.WriteFile(policy, []byte("status: proposed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, append(args, "validate")...); err == nil || !strings.Contains(err.Error(), "pack_lock_mismatch") {
		t.Fatalf("stale lock accepted: %v", err)
	}
}

func TestManifestRejectsExplicitZeroReviewApprovals(t *testing.T) {
	root := t.TempDir()
	if _, err := execute(t, "--project-root", root, "init", "--project-id", "fixture"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".cks", "knowledge", "manifest.yaml")
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(buf, []byte("min_approvals: 1"), []byte("min_approvals: 0"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, "--project-root", root, "validate"); err == nil || !strings.Contains(err.Error(), "min_approvals") {
		t.Fatalf("explicit zero approval policy accepted: %v", err)
	}
}

func TestConcurrentReviewRecordsPublishWholeBytesWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	checked, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	const count = 24
	var wg sync.WaitGroup
	errors := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := []byte(strings.Repeat(string(rune('a'+i)), 8192))
			name := fmt.Sprintf("record-%02d.yaml", i)
			if err := writeReviewRecord(dir, checked, name, body); err != nil {
				errors <- err
				return
			}
			got, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil || !bytes.Equal(got, body) {
				errors <- fmt.Errorf("record %s was partial or replaced: %v", name, err)
			}
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != count {
		t.Fatalf("temporary review file leaked: %d, %v", len(entries), err)
	}
	if err := writeReviewRecord(dir, checked, "record-00.yaml", []byte("replacement")); err == nil {
		t.Fatal("immutable review was overwritten")
	}
}
