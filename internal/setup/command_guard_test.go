package setup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func guardianContext(parent context.Context) context.Context {
	return context.WithValue(parent, commandGuardianKey{}, os.Args[0])
}
func waitOwnedMarker(t *testing.T, path string) {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(until) {
			t.Fatal("owned command did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func assertWriterEventuallyFree(t *testing.T, root string) {
	t.Helper()
	until := time.Now().Add(4 * time.Second)
	for {
		lock, err := acquireReindexLock(root)
		if err == nil {
			lock.release()
			return
		}
		if !errors.Is(err, ErrDatasetWriterBusy) || time.Now().After(until) {
			t.Fatal("owned writer was not released", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestGuardianPreservesExactArgvOutputAndWorkingDirectory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	root := t.TempDir()
	cmd := setupCommandContext(guardianContext(ctx), "/bin/sh", "-c", `printf '%s\n' "$1"; pwd`, "owned", "literal space; $(not-executed)")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "literal space; $(not-executed)\n"+root+"\n" {
		t.Fatalf("argv/workdir changed: %q", raw)
	}
}

func TestGuardianCancellationStopsSameGroupLateWriter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	root := t.TempDir()
	ready, late := filepath.Join(root, "ready"), filepath.Join(root, "late")
	cmd := setupCommandContext(guardianContext(ctx), "/bin/sh", "-c", `printf ready > "$1"; (sleep 1; printf orphan > "$2") & wait`, "owned", ready, late)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitOwnedMarker(t, ready)
	cancel()
	if err := cmd.Wait(); err == nil {
		t.Fatal("canceled owner returned success")
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := os.Stat(late); !os.IsNotExist(err) {
		t.Fatal("canceled descendant wrote after completion")
	}
}

func TestInheritedWriterLeaseSurvivesParentDescriptorClose(t *testing.T) {
	root := t.TempDir()
	lock, err := acquireReindexLock(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = context.WithValue(guardianContext(ctx), mutationLeaseKey{}, lock.file)
	ready := filepath.Join(t.TempDir(), "ready")
	cmd := setupCommandContext(ctx, "/bin/sh", "-c", `printf ready > "$1"; sleep 5`, "owned", ready)
	if err := cmd.Start(); err != nil {
		lock.release()
		t.Fatal(err)
	}
	waitOwnedMarker(t, ready)
	lock.release()
	if next, err := acquireReindexLock(root); !errors.Is(err, ErrDatasetWriterBusy) {
		if next != nil {
			next.release()
		}
		cancel()
		_ = cmd.Wait()
		t.Fatal("closing parent unlocked an active child writer")
	}
	cancel()
	_ = cmd.Wait()
	assertWriterEventuallyFree(t, root)
}

func TestGuardianOwnerProcessHelper(t *testing.T) {
	if os.Getenv("KS_GUARDIAN_OWNER_HELPER") != "1" {
		return
	}
	root := os.Getenv("KS_GUARDIAN_OWNER_ROOT")
	ready := os.Getenv("KS_GUARDIAN_OWNER_READY")
	late := os.Getenv("KS_GUARDIAN_OWNER_LATE")
	lock, err := acquireReindexLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx = context.WithValue(guardianContext(ctx), mutationLeaseKey{}, lock.file)
	cmd := setupCommandContext(ctx, "/bin/sh", "-c", `printf ready > "$1"; (sleep 2; printf orphan > "$2") & wait`, "owned", ready, late)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("owner should have been killed before its command finished")
}

func TestGuardianActualOwnerKillStopsWriterAndReleasesLease(t *testing.T) {
	root := t.TempDir()
	evidence := t.TempDir()
	ready, late := filepath.Join(evidence, "ready"), filepath.Join(evidence, "late")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	owner := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGuardianOwnerProcessHelper$")
	owner.Env = append(os.Environ(), "KS_GUARDIAN_OWNER_HELPER=1", "KS_GUARDIAN_OWNER_ROOT="+root, "KS_GUARDIAN_OWNER_READY="+ready, "KS_GUARDIAN_OWNER_LATE="+late)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if owner.ProcessState == nil {
			_ = owner.Process.Kill()
			_ = owner.Wait()
		}
	})
	waitOwnedMarker(t, ready)
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Wait(); err == nil {
		t.Fatal("owner did not lose process")
	}
	assertWriterEventuallyFree(t, root)
	time.Sleep(2100 * time.Millisecond)
	if _, err := os.Stat(late); !os.IsNotExist(err) {
		t.Fatal("owner loss left a live late writer")
	}
}

func TestGuardianAbsoluteDeadlineWorksWhileOwnerSocketStaysOpen(t *testing.T) {
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	parent, child := os.NewFile(uintptr(pair[0]), "owner"), os.NewFile(uintptr(pair[1]), "guardian")
	defer func() { _ = parent.Close(); _ = child.Close() }()
	root := t.TempDir()
	late := filepath.Join(root, "late")
	payload := guardedCommand{SchemaVersion: 1, Argv: []string{"/bin/sh", "-c", `sleep 1; printf late > "$1"`, "owned", late}, DeadlineUnixNano: time.Now().Add(200 * time.Millisecond).UnixNano()}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "__dataset-step", base64.RawURLEncoding.EncodeToString(raw))
	cmd.ExtraFiles = []*os.File{child}
	start := time.Now()
	if err := cmd.Run(); err == nil {
		t.Fatal("expired deadline reported success")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("guardian ignored absolute deadline")
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := os.Stat(late); !os.IsNotExist(err) {
		t.Fatal("deadline left a late writer")
	}
}

func TestDerivedFreshVerificationHonorsContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := VerifyDerivedFreshContext(guardianContext(ctx), "owned", "controlled slow renderer", []string{"/bin/sh", "-c", "sleep 5", "owned"}, filepath.Join(t.TempDir(), "committed"), func(Event) {})
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatal("derived verification ignored caller deadline")
	}
}
