package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountIgnoredIndexableHandlesNestedGeneratedTrees(t *testing.T) {
	repo := t.TempDir()
	gateGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("node_modules/\n*.go\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gateGit(t, repo, "add", ".gitignore")
	gateGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	for name, content := range map[string]string{
		"web/viewer/node_modules/lib.ts": "export {}\n",
		"web/source/hidden.go":           "package source\n",
		"web/dist/generated.go":          "package generated\n",
	} {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	count, err := CountIgnoredIndexable(repo)
	if err != nil || count != 2 {
		t.Fatalf("ignored indexable files = %d, %v; want hidden.go and potentially indexed dist/generated.go", count, err)
	}
}
