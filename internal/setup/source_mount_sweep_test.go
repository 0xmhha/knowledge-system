package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func TestReadTreeSweepRemovesOnlyCopiesOfExitedOwners(t *testing.T) {
	version, _ := retainedVersionForRequest(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	dead, live := exitedPID(t), os.Getppid()
	mk := func(name string, dir bool) string {
		path := filepath.Join(tmp, name)
		if dir {
			if err := os.MkdirAll(filepath.Join(path, "src"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "src", "a.go"), []byte("package a\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	stale := mk(fmt.Sprintf("cks-read-source-%d-111", dead), true)
	kept := []string{
		mk(fmt.Sprintf("cks-read-source-%d-222", live), true),  // live owner
		mk("cks-read-source-1081779747", true),                 // legacy name without owner
		mk(fmt.Sprintf("cks-read-source-%d-333", dead), false), // not a directory
		mk("other-tree", true),
	}
	root, cleanup, err := MaterializeRetainedReadTree(version)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("copy of an exited owner was kept: %v", err)
	}
	for _, path := range kept {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("sweep removed %s: %v", path, err)
		}
	}
	if !strings.HasPrefix(filepath.Base(root), fmt.Sprintf("cks-read-source-%d-", os.Getpid())) || filepath.Dir(root) != filepath.Clean(tmp) {
		t.Fatalf("read tree is not owner-tagged under TMPDIR: %s", root)
	}
	cleanup()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("cleanup kept the read tree: %v", err)
	}
}
