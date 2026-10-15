package maintenance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These service tests deliberately use a changed current source/profile and
// no working engine. A replay must use its admitted record before inspecting
// mutable source/model pins; it must never start another build.
func durableServiceFixture(t *testing.T, terminal bool) (Profile, journalRecord) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, output := filepath.Join(root, "source"), filepath.Join(root, "dataset")
	if os.Mkdir(source, 0o700) != nil || os.Mkdir(output, 0o700) != nil {
		t.Fatal("prepare owned roots")
	}
	if os.WriteFile(filepath.Join(source, "changed.go"), []byte("package changed\n"), 0o600) != nil {
		t.Fatal("write owned changed source")
	}
	r := queuedRecord(t)
	if err := claimRoot(output, r.Request.ProjectID); err != nil {
		t.Fatal(err)
	}
	store, err := openJournal(filepath.Join(output, journalDirectory), r.Request.ProjectID, r.Request.ProfileID, true)
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	if err := store.save(r); err != nil {
		t.Fatal(err)
	}
	if terminal {
		r.Status.State, r.Status.Phase, r.Status.Revision = "running", "capture", 2
		if err := store.save(r); err != nil {
			t.Fatal(err)
		}
		r.Status.State, r.Status.Phase, r.Status.Terminal, r.Status.Revision = "interrupted", "finished", true, 3
		r.Status.Error = &Error{Code: "interrupted", Phase: "capture"}
		if err := store.save(r); err != nil {
			t.Fatal(err)
		}
	}
	binary, err := filepath.EvalSymlinks(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "setup.yaml")
	// All paths belong to this fixture and use the resolved temporary root.
	text := "project_id: " + r.Request.ProjectID + "\nsource_mode: snapshot-only\nsrc: " + source + "\nout: " + output +
		"\ngraph_bin: " + binary + "\nvector_bin: " + binary + "\ncks_bin: " + binary + "\nembedder: mock\nmodel_name: mock\n"
	if err := os.WriteFile(config, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return Profile{ID: r.Request.ProfileID, ProjectID: r.Request.ProjectID, SetupConfig: config, AllowedActivation: []string{"hold_for_review"}, TimeoutSeconds: 600}, r
}

func reference(r journalRecord) JobReference {
	return JobReference{SchemaVersion: 1, ProjectID: r.Request.ProjectID, ProfileID: r.Request.ProfileID, JobID: r.Status.JobID}
}

func TestPrivateDurableServiceReloadsTerminalAndReplaysBeforeChangedPins(t *testing.T) {
	profile, record := durableServiceFixture(t, true)
	service, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	status, err := service.Status(reference(record))
	if err != nil || !equalJSON(status, record.Status) {
		t.Fatal("terminal record changed on restart")
	}
	replayed, err := service.Build(context.Background(), record.Request)
	if err != nil || !equalJSON(replayed, record.Status) {
		t.Fatal("replay inspected changed source or tried the fake engine")
	}
	conflict := record.Request
	conflict.NewVersion = "different-version"
	if _, err := service.Build(context.Background(), conflict); ErrorResult(err).Code != "idempotency_conflict" {
		t.Fatal("conflicting replay accepted")
	}
	foreign := reference(record)
	foreign.ProjectID = "project-b"
	if _, err := service.Status(foreign); ErrorResult(err).Code != "job_unknown" {
		t.Fatal("foreign project read durable job")
	}
	if result, err := service.Cancel(reference(record)); err != nil || result.Decision != "already_terminal" {
		t.Fatal("restart changed terminal cancellation behavior")
	}
}

func TestPrivateDurableServiceKeepsUnsettledJobsRecovering(t *testing.T) {
	profile, record := durableServiceFixture(t, false)
	service, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.Status(reference(record))
	if err != nil || status.State != "recovering" || status.Phase != "reconcile" || status.Terminal || status.Activated {
		t.Fatal("restart guessed interrupted/success before ownership/publication checks")
	}
	if err := ValidateStatus(status, record.Request, record.Profile); err != nil {
		t.Fatal(err)
	}
	result, err := service.Cancel(reference(record))
	if err != nil || result.Decision != "reconcile_required" || !equalJSON(result.Status, status) {
		t.Fatal("recovery was falsely acknowledged as canceled")
	}
	if len(service.busy) != 1 {
		t.Fatal("unsettled root allowed new writer")
	}
	service.Close()
	again, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	loaded, err := again.Status(reference(record))
	if err != nil || !equalJSON(loaded, status) {
		t.Fatal("recovery restart rewound or mutated the recorded status")
	}
}

func TestPrivateDurableServiceEnforcesOneRegistryOwner(t *testing.T) {
	profile, _ := durableServiceFixture(t, true)
	first, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if second, err := newDurableService([]Profile{profile}, os.Args[0]); ErrorResult(err).Code != "writer_busy" {
		if second != nil {
			second.Close()
		}
		t.Fatal("second coordinator opened owned registry")
	}
}

func TestDurableCancelIsPersistedBeforeAcknowledgment(t *testing.T) {
	s, _, ctx := memoryJob(t)
	store, _, record := newJournal(t)
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	j := s.jobs["job-one"]
	j.status = cloneStatus(record.Status)
	j.request = record.Request
	j.journal, j.record = store, &record
	delete(s.jobs, "job-one")
	s.jobs[record.Status.JobID] = j
	result, err := s.Cancel(reference(record))
	if err != nil || result.Decision != "accepted" || ctx.Err() != context.Canceled {
		t.Fatal("durable cancel failed")
	}
	rows, err := store.list()
	if err != nil || len(rows) != 1 || !equalJSON(rows[0].Status, result.Status) || rows[0].Status.State != "cancel_requested" {
		t.Fatal("cancel acknowledged before persistence")
	}
}

func TestFailedCancelPersistenceNeverAcknowledgesOrInventsTerminal(t *testing.T) {
	s, _, ctx := memoryJob(t)
	store, _, record := newJournal(t)
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	j := s.jobs["job-one"]
	j.status, j.request, j.journal, j.record = cloneStatus(record.Status), record.Request, store, &record
	delete(s.jobs, "job-one")
	s.jobs[record.Status.JobID] = j
	store.fault = func(string) error { return os.ErrPermission }
	if _, err := s.Cancel(reference(record)); ErrorResult(err).Code != "reconcile_required" {
		t.Fatal("failed durable cancel acknowledged")
	}
	if _, err := s.Status(reference(record)); ErrorResult(err).Code != "reconcile_required" || ctx.Err() != context.Canceled {
		t.Fatal("uncertain persistence emitted a fabricated revision or kept execution alive")
	}
	s.profiles[record.Request.ProfileID] = registered{spec: Profile{ID: record.Request.ProfileID, ProjectID: record.Request.ProjectID}}
	s.keys[key(record.Request)] = record.Status.JobID
	if _, err := s.Build(context.Background(), record.Request); ErrorResult(err).Code != "reconcile_required" {
		t.Fatal("uncertain replay exposed stale status")
	}
	conflict := record.Request
	conflict.NewVersion = "different"
	if _, err := s.Build(context.Background(), conflict); ErrorResult(err).Code != "idempotency_conflict" {
		t.Fatal("uncertain storage hid a known fingerprint conflict")
	}
}

// This fixture deliberately fails graph construction. It tests admission and
// failure persistence through the real orchestration, not successful DBs or
// model quality (the latter remain the public C–E evaluation).
func durableFailingProfile(t *testing.T) (Profile, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "source")
	if os.Mkdir(src, 0o700) != nil {
		t.Fatal("source")
	}
	for name, content := range map[string]string{"go.mod": "module example.invalid/owned\n\ngo 1.26.9\n", "owned.go": "package owned\nfunc Owned()int{return 1}\n"} {
		if os.WriteFile(filepath.Join(src, name), []byte(content), 0o600) != nil {
			t.Fatal("source file")
		}
	}
	marker := filepath.Join(root, "graph-started")
	quoteShell := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'" }
	graph, vector := filepath.Join(root, "graph-fails"), filepath.Join(root, "vector-identity")
	if os.WriteFile(graph, []byte("#!/bin/sh\nprintf started > "+quoteShell(marker)+"\nexit 23\n"), 0o700) != nil {
		t.Fatal("graph fixture")
	}
	if os.WriteFile(vector, []byte("#!/bin/sh\nprintf '%s\\n' '{\"embedding_identity\":{\"provider\":\"owned-fixture\",\"model\":\"controlled-failure-only\",\"dimensions\":8}}'\n"), 0o700) != nil {
		t.Fatal("vector fixture")
	}
	binary, err := filepath.EvalSymlinks(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "setup.yaml")
	text := "project_id: owned-failure\nsource_mode: snapshot-only\nsrc: " + src + "\nout: " + filepath.Join(root, "dataset") + "\ngraph_bin: " + graph + "\nvector_bin: " + vector + "\ncks_bin: " + binary + "\nembedder: mock\nmodel_name: mock\n"
	if os.WriteFile(config, []byte(text), 0o600) != nil {
		t.Fatal("config")
	}
	return Profile{ID: "owned-default", ProjectID: "owned-failure", SetupConfig: config, AllowedActivation: []string{"activate_after_gate"}, TimeoutSeconds: 60}, marker
}

func requestForInspection(v ProfileContext) BuildRequest {
	return BuildRequest{SchemaVersion: 1, ProjectID: v.ProjectID, ProfileID: v.ProfileID, ProfileDigest: v.ProfileDigest, SourcePin: v.SourcePin,
		InputDigest: v.InputDigest, EmbeddingDigest: v.EmbeddingDigest, Base: v.ObservedBase, NewVersion: "v-owned", Activation: "activate_after_gate",
		IdempotencyKey: "owned-failure-request-0001", DeadlineSeconds: 60}
}

func TestPrivateDurableFailurePersistsThroughActualAdmissionAndRestart(t *testing.T) {
	profile, marker := durableFailingProfile(t)
	service, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	inspection, err := service.Inspect(context.Background(), ProfileRequest{1, profile.ProjectID, profile.ID})
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Capabilities.JobPersistence != "journal_recovery_incomplete" {
		t.Fatal("private partial implementation claimed completed recovery")
	}
	request := requestForInspection(inspection.Profile)
	status, err := service.Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(45 * time.Second)
	ref := JobReference{1, request.ProjectID, request.ProfileID, status.JobID}
	for !status.Terminal {
		if time.Now().After(deadline) {
			t.Fatal("controlled failed job did not finish")
		}
		time.Sleep(10 * time.Millisecond)
		status, err = service.Status(ref)
		if err != nil {
			t.Fatal(err)
		}
	}
	if status.State != "failed" || status.Activated || status.Error == nil || status.Error.Code != "gate_failed" {
		t.Fatalf("false result: %+v", status)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("controlled engine failure was not exercised")
	}
	if _, err := os.Lstat(filepath.Join(service.profiles[profile.ID].out, "current")); !os.IsNotExist(err) {
		t.Fatal("failed build published current")
	}
	service.Close()
	again, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	replayed, err := again.Build(context.Background(), request)
	if err != nil || !equalJSON(replayed, status) {
		t.Fatal("failed job was lost or rerun on restart")
	}
}

func TestPrivateDurableAdmissionFailureCannotAcknowledgeOrRun(t *testing.T) {
	for _, stage := range []string{"before-rename", "after-rename"} {
		t.Run(stage, func(t *testing.T) {
			profile, marker := durableFailingProfile(t)
			service, err := newDurableService([]Profile{profile}, os.Args[0])
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			p := service.profiles[profile.ID]
			prepared, err := prepare(context.Background(), p, os.Args[0])
			if err != nil {
				t.Fatal(err)
			}
			request := requestForInspection(prepared.view)
			store, err := service.ensureJournal(context.Background(), p, prepared, request)
			if err != nil {
				t.Fatal(err)
			}
			store.fault = func(at string) error {
				if at == stage {
					return os.ErrPermission
				}
				return nil
			}
			status, err := service.Build(context.Background(), request)
			if ErrorResult(err).Code != "reconcile_required" || status.JobID != "" || len(service.jobs) != 0 {
				t.Fatal("failed admission acknowledged or launched job")
			}
			if _, err := os.Lstat(marker); !os.IsNotExist(err) {
				t.Fatal("failed admission ran engine")
			}
			service.Close()
			again, err := newDurableService([]Profile{profile}, os.Args[0])
			if err != nil {
				t.Fatal(err)
			}
			defer again.Close()
			want := 0
			if stage == "after-rename" {
				want = 1
			}
			if len(again.jobs) != want {
				t.Fatal("restart did not reconcile atomic admission boundary")
			}
			for _, j := range again.jobs {
				s := snapshot(j)
				if s.State != "recovering" || s.Terminal || s.Activated {
					t.Fatal("uncertain admission guessed a terminal result")
				}
			}
		})
	}
}
