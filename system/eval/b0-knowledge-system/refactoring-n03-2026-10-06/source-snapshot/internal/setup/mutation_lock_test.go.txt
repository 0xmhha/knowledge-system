package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveReindexLockCannotAgeOut(t *testing.T) {
	dataset := t.TempDir()
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	// Make the live holder older than the previous 6-hour reclaim ceiling.
	if err := os.WriteFile(filepath.Join(dataset, ".reindex.lock"), []byte(fmt.Sprintf("%d\n1\n", os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	other, err := acquireReindexLock(dataset)
	if err == nil {
		other.release()
		t.Fatal("live holder was stolen by age")
	}
}

func TestMutationLockLegacyLiveOwnerAndInode(t *testing.T) {
	dataset := t.TempDir()
	path := filepath.Join(dataset, ".reindex.lock")
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n1\n", os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if lock, err := acquireReindexLock(dataset); err == nil {
		lock.release()
		t.Fatal("old live PID stolen during migration")
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("legacy inode replaced")
	}
	if err := os.WriteFile(path, []byte("2147483000\n1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		t.Fatal(err)
	}
	lock.release()
	after, err = os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("released lock removed or replaced inode")
	}
	lock, err = acquireReindexLock(dataset)
	if err != nil {
		t.Fatal(err)
	}
	lock.release()
}

func TestMutationLockRejectsLinkedAndNonRegularFile(t *testing.T) {
	for _, kind := range []string{"symlink", "directory", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			dataset := t.TempDir()
			path := filepath.Join(dataset, ".reindex.lock")
			target := filepath.Join(t.TempDir(), "target")
			if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(target, path)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "hardlink":
				err = os.Link(target, path)
			}
			if err != nil {
				t.Fatal(err)
			}
			if lock, err := acquireReindexLock(dataset); err == nil {
				lock.release()
				t.Fatal("linked/nonregular lock accepted")
			}
			bytes, err := os.ReadFile(target)
			if err != nil || string(bytes) != "untouched" {
				t.Fatal("external lock target altered")
			}
		})
	}
}

// Each helper is a separate OS process with its own file descriptor table.
func TestMutationLockProcessHelper(t *testing.T) {
	if os.Getenv("CKS_N03_LOCK_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) != 3 {
		os.Exit(24)
	}
	lock, err := acquireReindexLock(args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(23)
	}
	defer lock.release()
	if err := os.WriteFile(args[2], []byte("holder entered"), 0600); err != nil {
		os.Exit(25)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestMutationLockSeparateProcessesAndOwnerCrash(t *testing.T) {
	dataset := t.TempDir()
	ready := filepath.Join(dataset, "ready")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	start := func(marker string) *exec.Cmd {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		t.Cleanup(cancel)
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestMutationLockProcessHelper$", "--", dataset, marker)
		cmd.Env = append(os.Environ(), "CKS_N03_LOCK_HELPER=1")
		return cmd
	}
	holder := start(ready)
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	killed := false
	t.Cleanup(func() {
		if !killed {
			_ = holder.Process.Kill()
			_ = holder.Wait()
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("holder failed to enter")
		}
		time.Sleep(10 * time.Millisecond)
	}
	path := filepath.Join(dataset, ".reindex.lock")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// An ancient record cannot invalidate this process's actual OS ownership.
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n1\n", holder.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	contenders := make([]*exec.Cmd, 8)
	for i := range contenders {
		contenders[i] = start(filepath.Join(dataset, fmt.Sprintf("contender-%d", i)))
		if err := contenders[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, cmd := range contenders {
		err := cmd.Wait()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
			t.Fatalf("concurrent process entered critical section: %v", err)
		}
	}
	if err := holder.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = holder.Wait()
	killed = true
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		t.Fatalf("OS did not recover dead holder: %v", err)
	}
	lock.release()
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("crash recovery replaced lock inode")
	}
}

func TestPointerMutationsCannotOvertakeBuild(t *testing.T) {
	dataset := t.TempDir()
	writeAlignedVersion(t, filepath.Join(dataset, "v1"))
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	for _, mutate := range []func() error{
		func() error { _, err := Promote(dataset, "v1"); return err },
		func() error { return Rollback(dataset, "v1") },
		func() error {
			_, err := PromoteReviewedCandidate(dataset, "v1", "patch", fmt.Sprintf("%064d", 1))
			return err
		},
	} {
		if err := mutate(); err == nil || !strings.Contains(err.Error(), "in progress") {
			t.Fatal("pointer writer overtook active build")
		}
	}
	if _, err := os.Lstat(filepath.Join(dataset, "current")); !os.IsNotExist(err) {
		t.Fatal("active pointer changed during build")
	}
}
