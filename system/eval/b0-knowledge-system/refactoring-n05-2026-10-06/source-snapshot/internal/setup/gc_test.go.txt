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

func gcVersion(t *testing.T, dataset, version string) string {
	t.Helper()
	path := durableTestVersion(t, dataset, version)
	if err := prepareReaderProtocol(dataset, version); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	return path
}

func gcDataset(t *testing.T) string {
	t.Helper()
	dataset := t.TempDir()
	for _, v := range []string{"v1", "v2", "v3", "v4", "v5", "v6"} {
		gcVersion(t, dataset, v)
	}
	if _, err := Promote(dataset, "v6"); err != nil {
		t.Fatal(err)
	}
	return dataset
}

func gcPlan(t *testing.T, dataset string) GCPlan {
	t.Helper()
	plan, err := PlanGC(context.Background(), dataset, GCPolicy{KeepRecent: 2, MinAge: time.Hour}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestGCDryRunDeterministicLegacyRollbackAndCapacity(t *testing.T) {
	dataset := gcDataset(t)
	legacy := durableTestVersion(t, dataset, "legacy")
	past := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(legacy, past, past); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "v6"); err != nil {
		t.Fatal(err)
	}
	policy := GCPolicy{KeepRecent: 2, MinAge: time.Hour, MaxBytes: 1, ProtectVersions: []string{"v3"}}
	at := time.Now()
	a, err := PlanGC(context.Background(), dataset, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PlanGC(context.Background(), dataset, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest != b.Digest || !a.OverBudget || a.RetainedBytes <= 1 {
		t.Fatalf("nondeterministic capacity plan: %+v", a)
	}
	for _, v := range a.Versions {
		if v.Version == "legacy" || v.Version == "v1" || v.Version == "v3" || v.Version == "v6" {
			if v.Delete {
				t.Fatalf("protected %s selected", v.Version)
			}
		}
	}
	if err := ApplyGC(context.Background(), dataset, a); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"legacy", "v1", "v3", "v6"} {
		if _, err := os.Stat(filepath.Join(dataset, v)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGCRejectsStaleReferenceAndBytePlans(t *testing.T) {
	for _, change := range []string{"reader", "pointer", "bytes"} {
		t.Run(change, func(t *testing.T) {
			dataset := gcDataset(t)
			plan := gcPlan(t, dataset)
			switch change {
			case "reader":
				lease, err := PinVersionReader(filepath.Join(dataset, "v1"))
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
			case "pointer":
				if _, err := Promote(dataset, "v1"); err != nil {
					t.Fatal(err)
				}
			case "bytes":
				if err := os.WriteFile(filepath.Join(dataset, "v1", "extra.json"), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := ApplyGC(context.Background(), dataset, plan); err == nil {
				t.Fatal("stale plan executed")
			}
			for _, v := range plan.Versions {
				if _, err := os.Stat(filepath.Join(dataset, v.Version)); err != nil {
					t.Fatal("stale rejection removed version", err)
				}
			}
		})
	}
}

func TestGCInterruptedTrashRecoveryAndCancellation(t *testing.T) {
	for _, stage := range []string{"before-trash-rename", "after-trash-rename", "before-trash-delete", "after-trash-file", "cancel-after-file"} {
		t.Run(stage, func(t *testing.T) {
			dataset := gcDataset(t)
			plan := gcPlan(t, dataset)
			currentBytes := durabilityTree(t, filepath.Join(dataset, "v6"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			hit := false
			gcFault = func(got, _ string) error {
				if got == stage || (stage == "cancel-after-file" && got == "after-trash-file") {
					hit = true
					if stage == "cancel-after-file" {
						cancel()
						return nil
					}
					return fmt.Errorf("injected %s", stage)
				}
				return nil
			}
			t.Cleanup(func() { gcFault = nil })
			err := ApplyGC(ctx, dataset, plan)
			gcFault = nil
			if !hit || err == nil {
				t.Fatal("GC did not stop at requested boundary", err)
			}
			assertDurabilityCurrent(t, dataset, "v6")
			if fmt.Sprint(currentBytes) != fmt.Sprint(durabilityTree(t, filepath.Join(dataset, "v6"))) {
				t.Fatal("interruption changed current artifacts")
			}
			pending := gcPlan(t, dataset)
			if pending.PendingTrashEntries == 0 || pending.PendingTrashBytes == 0 || pending.ReclaimBytes != 0 {
				t.Fatal("pending trash was omitted from capacity/protection status")
			}
			if err := ApplyGC(context.Background(), dataset, pending); err == nil {
				t.Fatal("new apply bypassed pending GC recovery")
			}
			if err := ResumeGC(context.Background(), dataset); err != nil {
				t.Fatal(err)
			}
			if err := ApplyGC(context.Background(), dataset, gcPlan(t, dataset)); err != nil {
				t.Fatal(err)
			}
			assertDurabilityCurrent(t, dataset, "v6")
			entries, err := os.ReadDir(filepath.Join(dataset, ".gc-trash"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("trash not recovered: %v %v", entries, err)
			}
		})
	}
}

func TestGCUnknownTrashIsReportedAndNeverDeleted(t *testing.T) {
	dataset := gcDataset(t)
	path := filepath.Join(dataset, ".gc-trash")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(path, "unknown"), 0700); err != nil {
		t.Fatal(err)
	}
	plan := gcPlan(t, dataset)
	if plan.PendingTrashEntries != 1 || plan.ReclaimBytes != 0 {
		t.Fatal("unknown trash not protected")
	}
	if err := ResumeGC(context.Background(), dataset); err == nil {
		t.Fatal("unknown trash reported as recovered")
	}
	if _, err := os.Stat(filepath.Join(path, "unknown")); err != nil {
		t.Fatal("unknown directory deleted")
	}
}

func TestGCCorruptedCurrentCannotDiscardRollbackCandidates(t *testing.T) {
	dataset := gcDataset(t)
	if err := os.WriteFile(filepath.Join(dataset, "v6", "dataset-identity.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanGC(context.Background(), dataset, GCPolicy{KeepRecent: 2}, time.Now()); err == nil {
		t.Fatal("corrupt current allowed GC planning")
	}
	if err := ResumeGC(context.Background(), dataset); err == nil {
		t.Fatal("corrupt current allowed recovery deletion")
	}
	for _, v := range []string{"v1", "v2", "v3", "v4", "v5"} {
		if _, err := os.Stat(filepath.Join(dataset, v)); err != nil {
			t.Fatal("rollback candidate discarded", err)
		}
	}
}

func TestGCInvalidReleaseAndPendingIntentRemainProtected(t *testing.T) {
	dataset := gcDataset(t)
	holdDurabilityVersion(t, dataset, "v1")
	if err := os.WriteFile(filepath.Join(dataset, "v1", "review-release.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataset, "v2", "review-intent.json"), []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-48 * time.Hour)
	for _, v := range []string{"v1", "v2"} {
		if err := os.Chtimes(filepath.Join(dataset, v), past, past); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range gcPlan(t, dataset).Versions {
		if v.Version == "v1" || v.Version == "v2" {
			if v.Delete || !strings.Contains(strings.Join(v.Reasons, ","), "review_hold") {
				t.Fatal("unreleased review reference selected")
			}
		}
	}
}

func TestGCRecoveryRejectsForeignTrash(t *testing.T) {
	dataset := gcDataset(t)
	plan := gcPlan(t, dataset)
	gcFault = func(stage, _ string) error {
		if stage == "after-trash-rename" {
			return fmt.Errorf("stop")
		}
		return nil
	}
	t.Cleanup(func() { gcFault = nil })
	if err := ApplyGC(context.Background(), dataset, plan); err == nil {
		t.Fatal("injection missing")
	}
	gcFault = nil
	entries, err := os.ReadDir(filepath.Join(dataset, ".gc-trash"))
	if err != nil {
		t.Fatal(err)
	}
	var trash string
	for _, entry := range entries {
		if entry.IsDir() {
			trash = filepath.Join(dataset, ".gc-trash", entry.Name())
			break
		}
	}
	if trash == "" {
		t.Fatal("missing trash")
	}
	if err := os.WriteFile(filepath.Join(trash, "foreign.txt"), []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ResumeGC(context.Background(), dataset); err == nil {
		t.Fatal("foreign trash deleted")
	}
	if _, err := os.Stat(filepath.Join(trash, "foreign.txt")); err != nil {
		t.Fatal(err)
	}
	assertDurabilityCurrent(t, dataset, "v6")
}

func TestGCMutationBlocksReaderAdmission(t *testing.T) {
	dataset := gcDataset(t)
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	if lease, err := PinVersionReader(filepath.Join(dataset, "v1")); err == nil {
		lease.Close()
		t.Fatal("reader admitted during GC/build")
	}
	if _, err := PlanGC(context.Background(), dataset, GCPolicy{KeepRecent: 2}, time.Now()); err == nil {
		t.Fatal("GC bypassed dataset mutation lock")
	}
}

func TestGCIdempotentPromotionPreservesActualRollbackReference(t *testing.T) {
	dataset := gcDataset(t)
	if _, err := Promote(dataset, "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "v6"); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "v6"); err != nil {
		t.Fatal(err)
	}
	for _, v := range gcPlan(t, dataset).Versions {
		if v.Version == "v1" && (v.Delete || !strings.Contains(strings.Join(v.Reasons, ","), "rollback_target")) {
			t.Fatal("noop promotion lost the actual rollback target")
		}
	}
}

func TestGCProcessHelper(t *testing.T) {
	if os.Getenv("CKS_N05_GC_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) != 4 {
		os.Exit(31)
	}
	dataset, ready, operation := args[1], args[2], args[3]
	if operation == "reader" {
		lease, err := PinVersionReader(filepath.Join(dataset, "v1"))
		if err != nil {
			os.Exit(32)
		}
		defer lease.Close()
		if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
			os.Exit(33)
		}
		for {
			time.Sleep(time.Second)
		}
	}
	plan, err := PlanGC(context.Background(), dataset, GCPolicy{KeepRecent: 2, MinAge: time.Hour}, time.Now())
	if err != nil {
		os.Exit(34)
	}
	gcFault = func(stage, _ string) error {
		if stage == "after-trash-file" {
			if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
				os.Exit(35)
			}
			for {
				time.Sleep(time.Second)
			}
		}
		return nil
	}
	if err := ApplyGC(context.Background(), dataset, plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(36)
	}
	os.Exit(37)
}

func TestGCReaderCrashAndPartialDeleteProcessRecovery(t *testing.T) {
	for _, operation := range []string{"reader", "delete"} {
		t.Run(operation, func(t *testing.T) {
			dataset := gcDataset(t)
			ready := filepath.Join(t.TempDir(), "ready")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGCProcessHelper$", "--", dataset, ready, operation)
			cmd.Env = append(os.Environ(), "CKS_N05_GC_HELPER=1")
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					cmd.Process.Kill()
					cmd.Wait()
				}
			})
			deadline := time.Now().Add(10 * time.Second)
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("helper did not reach boundary")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if operation == "reader" {
				for _, v := range gcPlan(t, dataset).Versions {
					if v.Version == "v1" && !strings.Contains(strings.Join(v.Reasons, ","), "pinned_reader") {
						t.Fatal("separate live reader not protected")
					}
				}
			}
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err == nil {
				t.Fatal("helper not killed")
			}
			if err := ResumeGC(context.Background(), dataset); err != nil {
				t.Fatal(err)
			}
			plan := gcPlan(t, dataset)
			if operation == "reader" {
				for _, v := range plan.Versions {
					if v.Version == "v1" && !v.Delete {
						t.Fatal("dead reader ownership not reclaimed")
					}
				}
			}
			if err := ApplyGC(context.Background(), dataset, plan); err != nil {
				t.Fatal(err)
			}
			assertDurabilityCurrent(t, dataset, "v6")
		})
	}
}

func TestGCProtectsCurrentReaderAndReviewedHold(t *testing.T) {
	dataset := t.TempDir()
	for _, v := range []string{"v1", "v2", "v3", "v4", "v5", "v6"} {
		gcVersion(t, dataset, v)
	}
	if _, err := Promote(dataset, "v6"); err != nil {
		t.Fatal(err)
	}
	lease, err := PinVersionReader(filepath.Join(dataset, "v1"))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	holdDurabilityVersion(t, dataset, "v2")
	policy := GCPolicy{KeepRecent: 2, MinAge: time.Hour}
	plan, err := PlanGC(context.Background(), dataset, policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range plan.Versions {
		if (item.Version == "v1" || item.Version == "v2" || item.Version == "v6") && item.Delete {
			t.Fatalf("protected %s selected", item.Version)
		}
	}
	if err := ApplyGC(context.Background(), dataset, plan); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"v1", "v2", "v6"} {
		if _, err := os.Stat(filepath.Join(dataset, v)); err != nil {
			t.Fatal(err)
		}
	}
	assertDurabilityCurrent(t, dataset, "v6")
}
