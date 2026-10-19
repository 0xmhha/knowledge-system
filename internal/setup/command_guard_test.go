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
	cmd := setupCommandContext(guardianContext(ctx), "/bin/sh", "-c", `printf ready > "$1"; (sleep 1 && printf orphan > "$2") & wait`, "owned", ready, late)
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
	childReady := os.Getenv("KS_GUARDIAN_OWNER_CHILD_READY")
	release := os.Getenv("KS_GUARDIAN_OWNER_RELEASE")
	lock, err := acquireReindexLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx = context.WithValue(guardianContext(ctx), mutationLeaseKey{}, lock.file)
	cmd := setupCommandContext(ctx, "/bin/sh", "-c", `(printf ready > "$2"; while [ ! -f "$3" ]; do sleep .01; done; printf orphan > "$4") & printf ready > "$1"; wait`, "owned", ready, childReady, release, late)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("owner should have been killed before its command finished")
}

func TestGuardianActualOwnerKillStopsWriterAndReleasesLease(t *testing.T) {
	root := t.TempDir()
	evidence := t.TempDir()
	ready, late := filepath.Join(evidence, "ready"), filepath.Join(evidence, "late")
	childReady, release := filepath.Join(evidence, "child-ready"), filepath.Join(evidence, "release")
	// Release only after the owner has died and the mutation lease is free.
	// A fixed sleep could write before a descheduled test gets to kill the owner.
	// Cleanup releases a stranded fixture even when an earlier assertion fails.
	t.Cleanup(func() { _ = os.WriteFile(release, []byte("cleanup"), 0o600) })
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	owner := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGuardianOwnerProcessHelper$")
	owner.Env = append(os.Environ(), "KS_GUARDIAN_OWNER_HELPER=1", "KS_GUARDIAN_OWNER_ROOT="+root, "KS_GUARDIAN_OWNER_READY="+ready, "KS_GUARDIAN_OWNER_LATE="+late, "KS_GUARDIAN_OWNER_CHILD_READY="+childReady, "KS_GUARDIAN_OWNER_RELEASE="+release)
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
	waitOwnedMarker(t, childReady)
	if _, err := os.Stat(late); !os.IsNotExist(err) {
		t.Fatal("writer crossed the unopened release gate")
	}
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Wait(); err == nil {
		t.Fatal("owner did not lose process")
	}
	assertWriterEventuallyFree(t, root)
	if err := os.WriteFile(release, []byte("after-owner-and-lease"), 0o600); err != nil {
		t.Fatal(err)
	}
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
	// A group signal can terminate sleep before its waiting shell. With ';',
	// that shell can briefly execute printf during cancellation, before it too
	// is killed. Require a successful full sleep to arm this delayed writer.
	payload := guardedCommand{SchemaVersion: 1, Argv: []string{"/bin/sh", "-c", `sleep 1 && printf late > "$1"`, "owned", late}, DeadlineUnixNano: time.Now().Add(200 * time.Millisecond).UnixNano()}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "__dataset-step", base64.RawURLEncoding.EncodeToString(raw))
	cmd.ExtraFiles = []*os.File{child}
	diagnostic, err := os.Create(filepath.Join(root, "guardian-output"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = diagnostic.Close() }()
	cmd.Stdout, cmd.Stderr = diagnostic, diagnostic
	start := time.Now()
	if err := cmd.Run(); err == nil {
		t.Fatal("expired deadline reported success")
	}
	finished := time.Now()
	if time.Since(start) > 3*time.Second {
		t.Fatal("guardian ignored absolute deadline")
	}
	returnInfo, returnErr := os.Stat(late)
	if returnErr != nil && !os.IsNotExist(returnErr) {
		t.Fatalf("inspect marker at guardian return: %v", returnErr)
	}
	time.Sleep(1100 * time.Millisecond)
	if info, err := os.Stat(late); err == nil {
		output, _ := os.ReadFile(diagnostic.Name())
		t.Fatalf("deadline writer marker: existed_at_return=%t mtime_after_return=%t guardian=%s", returnInfo != nil, info.ModTime().After(finished), output)
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect deadline writer marker: %v", err)
	}
}

func TestGuardianDeadlineStopsWriterReleasedOnlyAfterReturn(t *testing.T) {
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	parent, child := os.NewFile(uintptr(pair[0]), "owner"), os.NewFile(uintptr(pair[1]), "guardian")
	defer func() { _ = parent.Close(); _ = child.Close() }()
	root := t.TempDir()
	ready, release, late := filepath.Join(root, "ready"), filepath.Join(root, "release"), filepath.Join(root, "late")
	// Also release a faulty fixture when an earlier assertion fails.
	t.Cleanup(func() { _ = os.WriteFile(release, []byte("cleanup"), 0o600) })
	payload := guardedCommand{SchemaVersion: 1, Argv: []string{"/bin/sh", "-c", `(printf ready > "$1"; while [ ! -f "$2" ]; do sleep .01; done; printf late > "$3") & wait`, "owned", ready, release, late}, DeadlineUnixNano: time.Now().Add(3 * time.Second).UnixNano()}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "__dataset-step", base64.RawURLEncoding.EncodeToString(raw))
	cmd.ExtraFiles = []*os.File{child}
	// A regular file must not make Cmd.Wait wait for inherited output pipes;
	// otherwise a surviving writer would prevent the test from opening its gate.
	diagnostic, err := os.Create(filepath.Join(root, "guardian-output"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = diagnostic.Close() }()
	cmd.Stdout, cmd.Stderr = diagnostic, diagnostic
	start := time.Now()
	if err := cmd.Run(); err == nil {
		t.Fatal("expired deadline reported success")
	}
	if time.Since(start) > 6*time.Second {
		t.Fatal("guardian ignored gated writer deadline")
	}
	if _, err := os.Stat(ready); err != nil {
		output, _ := os.ReadFile(diagnostic.Name())
		t.Fatalf("gated writer never became ready: %v guardian=%s", err, output)
	}
	if _, err := os.Stat(late); !os.IsNotExist(err) {
		t.Fatalf("writer crossed unopened gate: %v", err)
	}
	// The owner socket stays open. Only the absolute deadline stops the
	// guardian; a surviving writer can now demonstrate a post-return write.
	if err := os.WriteFile(release, []byte("after-guardian-return"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := os.Stat(late); err == nil {
		output, _ := os.ReadFile(diagnostic.Name())
		t.Fatalf("deadline left a post-return writer: guardian=%s", output)
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect post-return writer marker: %v", err)
	}
}

func TestGuardianPastAbsoluteDeadlineCannotStartCommand(t *testing.T) {
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	parent, child := os.NewFile(uintptr(pair[0]), "owner"), os.NewFile(uintptr(pair[1]), "guardian")
	defer func() { _ = parent.Close(); _ = child.Close() }()
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	payload := guardedCommand{SchemaVersion: 1, Argv: []string{"/bin/sh", "-c", `printf late > "$1"`, "owned", marker}, DeadlineUnixNano: time.Now().Add(-time.Second).UnixNano()}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "__dataset-step", base64.RawURLEncoding.EncodeToString(raw))
	cmd.ExtraFiles = []*os.File{child}
	if cmd.Run() == nil {
		t.Fatal("past deadline succeeded")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("past deadline executed a command")
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
