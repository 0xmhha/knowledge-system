package maintenance

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func queuedRecord(t *testing.T) journalRecord {
	t.Helper()
	for _, fixture := range designFixtures(t) {
		if fixture.ID != "ST-valid-queued" {
			continue
		}
		var data struct {
			Request BuildRequest
			Profile ProfileContext
			Status  JobStatus
		}
		if err := json.Unmarshal(fixture.Data, &data); err != nil {
			t.Fatal(err)
		}
		return journalRecord{SchemaVersion: 1, Request: data.Request, Profile: data.Profile, Status: data.Status,
			ExpectedDatasetID: strings.Repeat("8", 64), CreatedAt: "2026-10-15T00:00:00Z", DeadlineAt: "2026-10-15T00:10:00Z"}
	}
	t.Fatal("missing queued fixture")
	return journalRecord{}
}

func newJournal(t *testing.T) (*journalStore, string, journalRecord) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "jobs")
	r := queuedRecord(t)
	s, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.close)
	return s, root, r
}

func TestJournalReadonlyMissingAndForeignOwnership(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "missing")
	s, err := openJournal(root, "project-a", "profile-a", false)
	if err != nil || s != nil {
		t.Fatalf("missing read changed behavior: %v", err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("read-only open created state")
	}
	store, root, r := newJournal(t)
	store.close()
	for _, owner := range [][2]string{{"project-b", r.Request.ProfileID}, {r.Request.ProjectID, "different"}} {
		if _, err := openJournal(root, owner[0], owner[1], false); ErrorResult(err).Code != "reconcile_required" {
			t.Fatal("foreign journal owner accepted")
		}
	}
}

func TestJournalFlockPersistsUntilOwnerCloses(t *testing.T) {
	s, root, r := newJournal(t)
	if _, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, false); ErrorResult(err).Code != "writer_busy" {
		t.Fatal("concurrent store acquired OS owner lock")
	}
	s.close()
	reopened, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, false)
	if err != nil {
		t.Fatal(err)
	}
	reopened.close()
}

func TestJournalRoundtripReplayAndDetachedRecords(t *testing.T) {
	s, root, record := newJournal(t)
	if err := s.save(record); err != nil {
		t.Fatal(err)
	}
	rows, err := s.list()
	if err != nil || len(rows) != 1 || !equalJSON(rows[0], record) {
		t.Fatal("record did not roundtrip")
	}
	rows[0].Status.Gates["graph"] = "failed"
	rows[0].Profile.RequiredGates[0] = "foreign"
	if err := s.save(record); err != nil {
		t.Fatal("identical write not idempotent")
	}
	s.close()
	reopened, err := openJournal(root, record.Request.ProjectID, record.Request.ProfileID, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	loaded, err := reopened.list()
	if err != nil || len(loaded) != 1 || !equalJSON(loaded[0], record) {
		t.Fatal("restart or caller mutation changed record")
	}
	other := cloneRecord(record)
	other.Status.JobID = "another-job"
	if err := reopened.save(other); ErrorResult(err).Code != "idempotency_conflict" {
		t.Fatal("duplicate scoped key created a different job")
	}
}

func TestJournalRejectsRewindTerminalMutationAndFalseSuccess(t *testing.T) {
	s, _, r := newJournal(t)
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	r.Status.State, r.Status.Phase, r.Status.Revision = "running", "capture", 2
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	bad := cloneRecord(r)
	bad.Status.Phase = "gate" // Same revision cannot describe a new body.
	if err := s.save(bad); ErrorResult(err).Code != "reconcile_required" {
		t.Fatal("same revision changed status")
	}
	r.Status.State, r.Status.Phase, r.Status.Terminal, r.Status.Revision = "interrupted", "finished", true, 3
	r.Status.Error = &Error{Code: "interrupted", Phase: "capture"}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	bad = cloneRecord(r)
	bad.Status.Revision++
	if err := s.save(bad); ErrorResult(err).Code != "reconcile_required" {
		t.Fatal("terminal state mutated")
	}
	bad = queuedRecord(t)
	bad.Status.State, bad.Status.Phase, bad.Status.Terminal, bad.Status.Activated = "succeeded", "finished", true, true
	if validateRecord(bad) == nil {
		t.Fatal("success without gates/candidate/receipt accepted")
	}
}

func TestJournalPersistenceFailureRequiresReopen(t *testing.T) {
	for _, stage := range []string{"before-rename", "after-rename"} {
		t.Run(stage, func(t *testing.T) {
			s, root, r := newJournal(t)
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			r.Status.State, r.Status.Phase, r.Status.Revision = "running", "capture", 2
			s.fault = func(at string) error {
				if at == stage {
					return errors.New("owned simulated IO failure")
				}
				return nil
			}
			if err := s.save(r); ErrorResult(err).Code != "reconcile_required" {
				t.Fatal("failed write acknowledged")
			}
			if _, err := s.list(); err == nil {
				t.Fatal("uncertain stale memory was served")
			}
			if _, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, false); ErrorResult(err).Code != "writer_busy" {
				t.Fatal("uncertain store released owner prematurely")
			}
			s.close()
			reopened, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, false)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.close()
			rows, err := reopened.list()
			want := 1
			if stage == "after-rename" {
				want = 2
			}
			if err != nil || len(rows) != 1 || rows[0].Status.Revision != want {
				t.Fatal("reopen did not settle old/new atomic record")
			}
		})
	}
}

func TestJournalRejectsCorruptAndRehashedFalseState(t *testing.T) {
	for _, restamp := range []bool{false, true} {
		t.Run(map[bool]string{false: "checksum", true: "semantic"}[restamp], func(t *testing.T) {
			s, root, r := newJournal(t)
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.close()
			path := filepath.Join(root, r.Status.JobID+".json")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var envelope journalEnvelope
			if json.Unmarshal(raw, &envelope) != nil {
				t.Fatal("bad saved envelope")
			}
			envelope.Record.Status.Activated = true
			if restamp {
				envelope.Checksum, err = Digest(envelope.Record, "cks.dataset-journal.v1")
				if err != nil {
					t.Fatal(err)
				}
			}
			raw, err = json.Marshal(envelope)
			if err != nil || os.WriteFile(path, raw, 0o600) != nil {
				t.Fatal("could not edit owned test state")
			}
			if _, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, false); ErrorResult(err).Code != "reconcile_required" {
				t.Fatal("corrupt state accepted")
			}
		})
	}
}

func TestJournalRejectsSymlinksAndHardlinkedRecords(t *testing.T) {
	for _, name := range []string{".registry.lock", ".owner.json", "job-a-001.json"} {
		t.Run(name, func(t *testing.T) {
			s, root, r := newJournal(t)
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.close()
			path := filepath.Join(root, name)
			other := filepath.Join(t.TempDir(), "foreign")
			if os.Rename(path, other) != nil || os.Symlink(other, path) != nil {
				t.Fatal("could not prepare owned symlink")
			}
			if _, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, false); err == nil {
				t.Fatal("symlink accepted")
			}
			if os.Remove(path) != nil || os.Link(other, path) != nil {
				t.Fatal("could not prepare hardlink")
			}
			if _, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, false); err == nil {
				t.Fatal("hardlink accepted")
			}
		})
	}
}

func TestJournalCrashHelper(t *testing.T) {
	stage := os.Getenv("KS_JOURNAL_TEST_HELPER_STAGE")
	if stage == "" {
		return
	}
	r := queuedRecord(t)
	s, err := openJournal(os.Getenv("KS_JOURNAL_TEST_HELPER_ROOT"), r.Request.ProjectID, r.Request.ProfileID, false)
	if err != nil {
		t.Fatal(err)
	}
	s.fault = func(at string) error {
		if at == stage {
			os.Exit(81) // Simulate an ungraceful owner loss at an exact write boundary.
		}
		return nil
	}
	r.Status.State, r.Status.Phase, r.Status.Revision = "running", "capture", 2
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash fault was not reached")
}

func TestJournalProcessLossLeavesOneWholeRecordAndReleasesLock(t *testing.T) {
	for _, stage := range []string{"before-rename", "after-rename"} {
		t.Run(stage, func(t *testing.T) {
			s, root, r := newJournal(t)
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.close()
			cmd := exec.Command(os.Args[0], "-test.run=^TestJournalCrashHelper$")
			cmd.Env = append(os.Environ(), "KS_JOURNAL_TEST_HELPER_STAGE="+stage, "KS_JOURNAL_TEST_HELPER_ROOT="+root)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 81 {
				t.Fatalf("helper did not exit at fault: %v %s", err, output)
			}
			reopened, err := openJournal(root, r.Request.ProjectID, r.Request.ProfileID, false)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.close()
			rows, err := reopened.list()
			want := 1
			if stage == "after-rename" {
				want = 2
			}
			if err != nil || len(rows) != 1 || rows[0].Status.Revision != want {
				t.Fatal("crash exposed partial or missing state")
			}
		})
	}
}

func TestJournalBoundsRecordsWithoutEvictingReplay(t *testing.T) {
	s, _, original := newJournal(t)
	for i := 0; i < 128; i++ {
		r := cloneRecord(original)
		r.Request.IdempotencyKey = fmt.Sprintf("owned-build-key-%04d", i)
		r.Request.NewVersion = fmt.Sprintf("v-new-%04d", i)
		r.Status.JobID = fmt.Sprintf("owned-job-%04d", i)
		r.Status.TargetVersion = r.Request.NewVersion
		var err error
		r.Status.RequestDigest, err = Digest(r.Request, "cks.dataset-build.v1")
		if err != nil || s.save(r) != nil {
			t.Fatalf("admission %d failed", i)
		}
	}
	if err := s.save(original); ErrorResult(err).Code != "unsupported_capability" {
		t.Fatal("record bound exceeded")
	}
	rows, err := s.list()
	if err != nil || len(rows) != 128 {
		t.Fatal("bound evicted admitted jobs")
	}
}
