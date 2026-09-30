//go:build darwin || linux

package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCapturedOpenRefusesLinkedDirectoryAndFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "real", "code.go"), []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := openCapturedNoFollow(root, "real/code.go")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	for link, target := range map[string]struct{ target, requested string }{
		"linked-dir":  {"real", "linked-dir/code.go"},
		"linked-file": {"real/code.go", "linked-file"},
	} {
		if err := os.Symlink(target.target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
		if file, err := openCapturedNoFollow(root, target.requested); err == nil {
			file.Close()
			t.Fatalf("linked path %q was opened", target.requested)
		}
	}
}
