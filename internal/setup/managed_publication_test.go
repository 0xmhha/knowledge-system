package setup

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func managedTestRoot(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	old := durableTestVersion(t, root, "old")
	durableTestVersion(t, root, "candidate")
	if _, err := Promote(root, "old"); err != nil {
		t.Fatal(err)
	}
	owner := []byte(`{"schema_version":1,"project_id":"durability-fixture"}`)
	if err := os.WriteFile(filepath.Join(root, ".maintenance-project.json"), owner, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, old
}
func managedPromoteWithID(ctx context.Context, root, version, id string) (string, error) {
	ctx = context.WithValue(ctx, managedPublicationKey{}, id)
	return withPromotionLock(root, func() (string, error) { return promoteLockedContext(ctx, root, version, false) })
}

func TestManagedPublicationReceiptSurvivesLaterTransitionAndRollback(t *testing.T) {
	root, old := managedTestRoot(t)
	before := durabilityTree(t, old)
	if previous, err := managedPromoteWithID(context.Background(), root, "candidate", "job-owned-1"); err != nil || previous != "old" {
		t.Fatal("managed publication failed", err)
	}
	proof, err := ReadManagedPublication(root, "job-owned-1")
	if err != nil || proof == nil || proof.Outcome != "committed" || proof.Version != "candidate" || proof.PreviousVersion != "old" {
		t.Fatal("missing checked publication history", err)
	}
	raw, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	durableTestVersion(t, root, "later")
	if _, err := Promote(root, "later"); err != nil {
		t.Fatal("ordinary writer did not participate", err)
	}
	if err := Rollback(root, "old"); err != nil {
		t.Fatal(err)
	}
	after, err := ReconcileManagedPublication(context.Background(), root, "job-owned-1")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(after)
	if err != nil || string(encoded) != string(raw) {
		t.Fatal("later transition rewrote historical receipt")
	}
	assertDurabilityCurrent(t, root, "old")
	if got := durabilityTree(t, old); len(before) != len(got) {
		t.Fatal("old immutable input changed")
	} else {
		for p, h := range before {
			if got[p] != h {
				t.Fatal("old immutable input changed")
			}
		}
	}
}

func TestManagedPublicationBeforeAndAfterRenameIOFailure(t *testing.T) {
	for _, stage := range []string{"before-rename", "after-rename", "current-parent", "publication-history"} {
		t.Run(stage, func(t *testing.T) {
			root, _ := managedTestRoot(t)
			id := "job-io-" + stage
			durabilityFault = func(at, path string) error {
				if at == stage {
					return errors.New("owned injected persistence failure")
				}
				return nil
			}
			_, err := managedPromoteWithID(context.Background(), root, "candidate", id)
			durabilityFault = nil
			t.Cleanup(func() { durabilityFault = nil })
			if err == nil {
				t.Fatal("fault did not reach publication")
			}
			proof, err := ReconcileManagedPublication(context.Background(), root, id)
			if err != nil || proof == nil {
				t.Fatal("could not settle operation", err)
			}
			want, version := "committed", "candidate"
			if stage == "before-rename" {
				want, version = "not_committed", "old"
			}
			if proof.Outcome != want {
				t.Fatalf("outcome %s want %s", proof.Outcome, want)
			}
			assertDurabilityCurrent(t, root, version)
			if _, err := os.Lstat(filepath.Join(root, managedPublicationDirectory, "pending.json")); !os.IsNotExist(err) {
				t.Fatal("settled pending record left ambiguous")
			}
		})
	}
}

func TestOrdinaryManagedWriterSettlesPreviousCrashBeforeAdvancing(t *testing.T) {
	root, _ := managedTestRoot(t)
	durabilityFault = func(stage, path string) error {
		if stage == "after-rename" {
			return errors.New("owned failure")
		}
		return nil
	}
	_, err := managedPromoteWithID(context.Background(), root, "candidate", "job-before-cli")
	durabilityFault = nil
	t.Cleanup(func() { durabilityFault = nil })
	if !errors.Is(err, ErrDurabilityUncertain) {
		t.Fatal("expected uncertain rename", err)
	}
	durableTestVersion(t, root, "later")
	if _, err := Promote(root, "later"); err != nil {
		t.Fatal(err)
	}
	proof, err := ReadManagedPublication(root, "job-before-cli")
	if err != nil || proof == nil || proof.Outcome != "committed" {
		t.Fatal("next CLI writer discarded previous success", err)
	}
	assertDurabilityCurrent(t, root, "later")
}

func TestManagedPublicationRefusesUnexpectedPointerAndMetadataDamage(t *testing.T) {
	for _, damage := range []string{"unexpected-pointer", "checksum", "case-alias", "linked-pending"} {
		t.Run(damage, func(t *testing.T) {
			root, _ := managedTestRoot(t)
			durabilityFault = func(stage, path string) error {
				if stage == "before-rename" {
					return errors.New("owned stop")
				}
				return nil
			}
			_, err := managedPromoteWithID(context.Background(), root, "candidate", "job-damaged")
			durabilityFault = nil
			t.Cleanup(func() { durabilityFault = nil })
			if err == nil {
				t.Fatal("fault not reached")
			}
			path := filepath.Join(root, managedPublicationDirectory, "pending.json")
			switch damage {
			case "unexpected-pointer":
				durableTestVersion(t, root, "untracked")
				if os.Remove(filepath.Join(root, "current")) != nil || os.Symlink("untracked", filepath.Join(root, "current")) != nil {
					t.Fatal("owned unsupported mutation")
				}
			case "checksum":
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				raw = []byte(strings.Replace(string(raw), `"job-damaged"`, `"job-forged"`, 1))
				if os.WriteFile(path, raw, 0o600) != nil {
					t.Fatal("owned damage")
				}
			case "case-alias":
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				raw = []byte(strings.Replace(string(raw), `"operation_id"`, `"OPERATION_ID"`, 1))
				if os.WriteFile(path, raw, 0o600) != nil {
					t.Fatal("owned case alias")
				}
			case "linked-pending":
				other := filepath.Join(t.TempDir(), "pending")
				if os.Rename(path, other) != nil || os.Symlink(other, path) != nil {
					t.Fatal("owned linked metadata")
				}
			}
			prior, _ := os.Readlink(filepath.Join(root, "current"))
			if _, err := Promote(root, "candidate"); !errors.Is(err, ErrPublicationUnresolved) {
				t.Fatal("unsettled metadata permitted a new writer", err)
			}
			current, _ := os.Readlink(filepath.Join(root, "current"))
			if current != prior {
				t.Fatal("failed reconciliation changed pointer")
			}
		})
	}
}

func TestManagedPublicationCrashHelper(t *testing.T) {
	if os.Getenv("KS_MANAGED_PUBLICATION_HELPER") != "1" {
		return
	}
	args := strings.Split(os.Getenv("KS_MANAGED_PUBLICATION_CASE"), "|")
	if len(args) != 3 {
		t.Fatal("invalid helper case")
	}
	root, stage, ready := args[0], args[1], args[2]
	durabilityFault = func(at, path string) error {
		if at != stage {
			return nil
		}
		if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
			t.Fatal(err)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	_, err := managedPromoteWithID(context.Background(), root, "candidate", "job-crash")
	t.Fatalf("helper did not stop: %v", err)
}

func TestManagedPublicationRealProcessKillRecovery(t *testing.T) {
	for _, stage := range []string{"publication-intent", "before-rename", "after-rename", "current-parent", "publication-history"} {
		t.Run(stage, func(t *testing.T) {
			root, old := managedTestRoot(t)
			before := durabilityTree(t, old)
			marker := filepath.Join(t.TempDir(), "ready")
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestManagedPublicationCrashHelper$")
			cmd.Env = append(os.Environ(), "KS_MANAGED_PUBLICATION_HELPER=1", "KS_MANAGED_PUBLICATION_CASE="+strings.Join([]string{root, stage, marker}, "|"))
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			})
			until := time.Now().Add(12 * time.Second)
			for {
				if _, err := os.Stat(marker); err == nil {
					break
				}
				if time.Now().After(until) {
					t.Fatal("child did not reach controlled boundary")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if cmd.Process.Kill() != nil {
				t.Fatal("kill helper")
			}
			if err := cmd.Wait(); err == nil {
				t.Fatal("helper did not lose owner")
			}
			proof, err := ReconcileManagedPublication(context.Background(), root, "job-crash")
			if err != nil || proof == nil {
				t.Fatal("missing recovered publication decision", err)
			}
			outcome, version := "committed", "candidate"
			if stage == "publication-intent" || stage == "before-rename" {
				outcome, version = "not_committed", "old"
			}
			if proof.Outcome != outcome {
				t.Fatalf("%s outcome %s", stage, proof.Outcome)
			}
			assertDurabilityCurrent(t, root, version)
			for path, h := range before {
				if durabilityTree(t, old)[path] != h {
					t.Fatal("crash changed old retained bytes")
				}
			}
		})
	}
}
