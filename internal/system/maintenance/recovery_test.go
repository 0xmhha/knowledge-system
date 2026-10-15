package maintenance

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

// Synthetic history isolates the service join/recovery policy. It does not
// claim a real DB publication: setup's separate process-kill tests exercise
// the actual producer, and E is the real public end-to-end acceptance scope.
func recoveryFixture(t *testing.T, kind string, cancelRequested bool) (Profile, journalRecord, *setup.ManagedPublication) {
	t.Helper()
	profile, old := durableServiceFixture(t, false)
	options, err := setup.LoadConfigStrict(profile.SetupConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(options.Out, journalDirectory, old.Status.JobID+".json")); err != nil {
		t.Fatal(err)
	}
	source, err := setup.SnapshotSourceIdentityWithLimits(context.Background(), options.Src, profile.ProjectID, "snapshot-only", "", setup.CaptureLimits{})
	if err != nil {
		t.Fatal(err)
	}
	model := json.RawMessage(`{"provider":"owned-synthetic","model":"recovery-join","dimensions":8}`)
	inputDigest := old.Request.InputDigest
	base := Base{State: "absent"}
	if kind == "actual-held" {
		retained, previous := retainedHeldFixture(t, options.Out)
		source, model, inputDigest = retained.Source, retained.EmbeddingIdentity, retained.InputDigest
		coordinates := coordinates(previous)
		base = Base{State: "pinned", Version: "v1", Coordinates: &coordinates}
	}
	var object any
	if json.Unmarshal(model, &object) != nil {
		t.Fatal("model")
	}
	modelDigest, err := Digest(object, "cks.dataset-embedding.v1")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := setup.NewDatasetIdentity(source, model, inputDigest)
	if err != nil {
		t.Fatal(err)
	}
	record := cloneRecord(old)
	record.Request.SourcePin = SourcePin{source.SourceMode, source.SnapshotID, source.SourceCommit}
	record.Request.Base = base
	record.Request.InputDigest = inputDigest
	record.Profile.InputDigest = inputDigest
	record.Request.Activation = "activate_after_gate"
	if strings.HasPrefix(kind, "held-") || kind == "actual-held" {
		record.Request.Activation = "hold_for_review"
		kind = strings.TrimPrefix(kind, "held-")
	}
	record.Request.EmbeddingDigest = modelDigest
	record.Profile.SourcePin = record.Request.SourcePin
	record.Profile.ObservedBase = record.Request.Base
	record.Profile.EmbeddingDigest = modelDigest
	record.Profile.ExistingVersions = []string{}
	record.ExpectedDatasetID = identity.DatasetID
	record.Status.RequestDigest, err = Digest(record.Request, "cks.dataset-build.v1")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Second)
	record.CreatedAt = now.Format(time.RFC3339Nano)
	record.DeadlineAt = now.Add(time.Duration(record.Request.DeadlineSeconds) * time.Second).Format(time.RFC3339Nano)
	store, err := openJournal(filepath.Join(options.Out, journalDirectory), profile.ProjectID, profile.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	record.Status.State, record.Status.Phase, record.Status.Revision = "running", "publish", 2
	record.Status.BuildComplete = true
	record.Status.Candidate = &Candidate{Version: record.Request.NewVersion, Coordinates: coordinates(identity), InputDigest: record.Request.InputDigest, EmbeddingDigest: modelDigest}
	for _, gate := range record.Profile.RequiredGates {
		record.Status.Gates[gate] = "passed"
	}
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	if cancelRequested {
		record.Status.State = "cancel_requested"
		record.Status.Revision++
		record.CancelRequested = true
		if err := store.save(record); err != nil {
			t.Fatal(err)
		}
	}
	if kind == "none" || kind == "actual-held" {
		return profile, record, nil
	}
	proof := &setup.ManagedPublication{SchemaVersion: 1, OperationID: record.Status.JobID, ProjectID: profile.ProjectID, Version: record.Request.NewVersion,
		Identity: identity, BeganAt: now.Format(time.RFC3339Nano), ConfirmedAt: time.Now().UTC().Format(time.RFC3339Nano), Outcome: kind}
	return profile, record, proof
}

func putSyntheticHistory(t *testing.T, profile Profile, proof setup.ManagedPublication) {
	t.Helper()
	options, err := setup.LoadConfigStrict(profile.SetupConfig)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(options.Out, ".maintenance-publications")
	if err := os.Mkdir(root, 0o700); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	body, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	// Independently encode the documented one-field length-prefixed domain hash.
	h := sha256.New()
	_, _ = h.Write([]byte("cks.managed-publication.v1\x00"))
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(body)))
	_, _ = h.Write(length[:])
	_, _ = h.Write(body)
	raw, err := json.Marshal(map[string]any{"proof": proof, "checksum": hex.EncodeToString(h.Sum(nil))})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, proof.OperationID+".json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPublicationReceiptJoinsExactJobSourceModelAndBase(t *testing.T) {
	_, record, proof := recoveryFixture(t, "committed", false)
	good, err := publicationFromProof(record.Request, record.Status.JobID, record.ExpectedDatasetID, record.Status.Candidate, proof)
	if err != nil || good == nil {
		t.Fatal(err)
	}
	status := cloneStatus(record.Status)
	status.State, status.Phase, status.Terminal, status.Activated = "succeeded", "finished", true, true
	status.Publication = good
	if err := ValidateStatus(status, record.Request, record.Profile); err != nil {
		t.Fatal("receipt violates independent contract fixtures", err)
	}
	for name, mutate := range map[string]func(*setup.ManagedPublication){
		"foreign-job":     func(p *setup.ManagedPublication) { p.OperationID = "foreign" },
		"foreign-project": func(p *setup.ManagedPublication) { p.ProjectID = "foreign" },
		"other-version":   func(p *setup.ManagedPublication) { p.Version = "other" },
		"other-dataset":   func(p *setup.ManagedPublication) { p.Identity.DatasetID = strings.Repeat("0", 64) },
		"stale-input":     func(p *setup.ManagedPublication) { p.Identity.InputDigest = strings.Repeat("0", 64) },
		"other-model": func(p *setup.ManagedPublication) {
			p.Identity.EmbeddingIdentity = json.RawMessage(`{"provider":"different"}`)
		},
		"foreign-snapshot": func(p *setup.ManagedPublication) { p.Identity.Source.SnapshotID = strings.Repeat("0", 64) },
		"unexpected-base": func(p *setup.ManagedPublication) {
			p.PreviousVersion = "old"
			previous := p.Identity
			p.PreviousIdentity = &previous
		},
		"pending": func(p *setup.ManagedPublication) { p.Outcome = "pending" },
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(proof)
			if err != nil {
				t.Fatal(err)
			}
			var bad setup.ManagedPublication
			if json.Unmarshal(raw, &bad) != nil {
				t.Fatal("clone")
			}
			mutate(&bad)
			if _, err := publicationFromProof(record.Request, record.Status.JobID, record.ExpectedDatasetID, record.Status.Candidate, &bad); ErrorResult(err).Code != "reconcile_required" {
				t.Fatal("foreign/incomplete proof accepted")
			}
		})
	}
}

func TestPrivateRecoveryUsesHistoricalPublicationWithoutInventingServingAdoption(t *testing.T) {
	profile, record, proof := recoveryFixture(t, "committed", false)
	putSyntheticHistory(t, profile, *proof)
	service, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	until := time.Now().Add(5 * time.Second)
	var status JobStatus
	for {
		status, err = service.Status(reference(record))
		if err != nil {
			t.Fatal(err)
		}
		if status.Terminal {
			break
		}
		if time.Now().After(until) {
			t.Fatal("history was not recovered")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status.State != "succeeded" || !status.Activated || status.Publication == nil || status.ServingAdoption != "not_observed" {
		t.Fatal("wrong recovered status")
	}
	if err := ValidateStatus(status, record.Request, record.Profile); err != nil {
		t.Fatal(err)
	}
	options, err := setup.LoadConfigStrict(profile.SetupConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(options.Out, "current")); !os.IsNotExist(err) {
		t.Fatal("history lookup moved or invented current")
	}
	replayed, err := service.Build(context.Background(), record.Request)
	if err != nil || !equalJSON(replayed, status) {
		t.Fatal("recovered success reran instead of replaying")
	}
}

func TestPrivateRecoveryDistinguishesNotCommittedCanceledAndInterrupted(t *testing.T) {
	for _, requested := range []bool{false, true} {
		t.Run(map[bool]string{false: "interrupted", true: "canceled"}[requested], func(t *testing.T) {
			profile, record, proof := recoveryFixture(t, "not_committed", requested)
			putSyntheticHistory(t, profile, *proof)
			service, err := newDurableService([]Profile{profile}, os.Args[0])
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			until := time.Now().Add(5 * time.Second)
			for {
				status, err := service.Status(reference(record))
				if err != nil {
					t.Fatal(err)
				}
				if status.Terminal {
					want := "interrupted"
					if requested {
						want = "canceled"
					}
					if status.State != want || status.Activated || status.Publication != nil {
						t.Fatal("wrong nonpublication recovery")
					}
					if ValidateStatus(status, record.Request, record.Profile) != nil {
						t.Fatal("invalid recovered state")
					}
					break
				}
				if time.Now().After(until) {
					t.Fatal("nonpublication recovery did not finish")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

func TestPrivateRecoveryRefusesCommittedHistoryContradictingAcceptedCancel(t *testing.T) {
	profile, record, proof := recoveryFixture(t, "committed", true)
	putSyntheticHistory(t, profile, *proof)
	service, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	time.Sleep(600 * time.Millisecond)
	status, err := service.Status(reference(record))
	if err != nil || status.State != "recovering" || status.Terminal || status.Activated {
		t.Fatal("accepted cancellation was silently changed to success")
	}
}

func TestCancelIntentSurvivesLegacyRecordAndMultipleRecoveryRestarts(t *testing.T) {
	profile, record := durableServiceFixture(t, false)
	options, err := setup.LoadConfigStrict(profile.SetupConfig)
	if err != nil {
		t.Fatal(err)
	}
	store, err := openJournal(filepath.Join(options.Out, journalDirectory), profile.ProjectID, profile.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	record.Status.State = "cancel_requested"
	record.Status.Revision++
	// No optional flag: emulate the earlier private A/B record format.
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	store.close()
	for i := 0; i < 2; i++ {
		service, err := newDurableService([]Profile{profile}, os.Args[0])
		if err != nil {
			t.Fatal(err)
		}
		j := service.jobs[record.Status.JobID]
		j.mu.Lock()
		kept := j.record.CancelRequested
		j.mu.Unlock()
		if !kept {
			service.Close()
			t.Fatal("recovering erased accepted cancellation intent")
		}
		status, err := service.Status(reference(record))
		if err != nil || status.State != "recovering" || status.Terminal {
			service.Close()
			t.Fatal("unconfirmed child outcome fabricated")
		}
		service.Close()
	}
}

func TestAcceptedCancelOfBuiltHeldCandidateCannotRecoverAsHeld(t *testing.T) {
	profile, record, _ := recoveryFixture(t, "held-none", true)
	service, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	until := time.Now().Add(5 * time.Second)
	for {
		status, err := service.Status(reference(record))
		if err != nil {
			t.Fatal(err)
		}
		if status.Terminal {
			if status.State != "canceled" || status.Activated || status.Publication != nil {
				t.Fatal("accepted cancel was converted to held/success")
			}
			break
		}
		if time.Now().After(until) {
			t.Fatal("built candidate cancellation did not settle")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// This immutable fixture contains actual CKG/CKV SQLite databases and retained
// source from KM-02.2; only the selected held version is copied into owned tmp.
func retainedHeldFixture(t *testing.T, output string) (setup.DatasetIdentity, setup.DatasetIdentity) {
	t.Helper()
	file, err := os.Open("../../../system/eval/knowledge-roadmap-2026-10-15/km-02-public-maintenance/fixture.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = compressed.Close() }()
	archive := tar.NewReader(compressed)
	var selected, previous setup.DatasetIdentity
	const prefix = "project-a/dataset/v2/"
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == "project-a/dataset/v1/dataset-identity.json" {
			raw, err := io.ReadAll(io.LimitReader(archive, 64<<10))
			if err != nil || json.Unmarshal(raw, &previous) != nil {
				t.Fatal("previous fixture identity")
			}
			continue
		}
		if !strings.HasPrefix(header.Name, prefix) {
			continue
		}
		relative := strings.TrimPrefix(header.Name, prefix)
		if header.Typeflag == tar.TypeDir {
			relative = strings.TrimSuffix(relative, "/")
		}
		if relative == "" {
			continue
		}
		if filepath.IsAbs(relative) || filepath.Clean(relative) != relative || relative == ".." || strings.HasPrefix(relative, "../") {
			t.Fatal("unsafe fixture path")
		}
		target := filepath.Join(output, "v2", relative)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				t.Fatal(err)
			}
		case tar.TypeReg:
			if header.Size > 16<<20 {
				t.Fatal("oversized fixture file")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(io.LimitReader(archive, (16<<20)+1))
			if err != nil || int64(len(raw)) != header.Size {
				t.Fatal("incomplete fixture file")
			}
			if err := os.WriteFile(target, raw, os.FileMode(header.Mode)&0o777); err != nil {
				t.Fatal(err)
			}
			if relative == "dataset-identity.json" && json.Unmarshal(raw, &selected) != nil {
				t.Fatal("held fixture identity")
			}
		default:
			t.Fatal("fixture contains a linked/special selected artifact")
		}
	}
	if selected.DatasetID == "" || previous.DatasetID == "" {
		t.Fatal("actual retained identities missing")
	}
	return selected, previous
}

func TestPrivateRecoveryVerifiesActualRetainedHeldDatabase(t *testing.T) {
	profile, record, _ := recoveryFixture(t, "actual-held", false)
	service, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	until := time.Now().Add(10 * time.Second)
	for {
		status, err := service.Status(reference(record))
		if err != nil {
			t.Fatal(err)
		}
		if status.Terminal {
			if status.State != "held_for_review" || !status.BuildComplete || status.Activated || status.Publication != nil {
				t.Fatal("held candidate falsely adopted or lost")
			}
			if ValidateStatus(status, record.Request, record.Profile) != nil {
				t.Fatal("invalid held recovery")
			}
			break
		}
		if time.Now().After(until) {
			t.Fatal("actual retained held candidate did not recover")
		}
		time.Sleep(10 * time.Millisecond)
	}
	options, err := setup.LoadConfigStrict(profile.SetupConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(options.Out, "current")); !os.IsNotExist(err) {
		t.Fatal("held recovery activated current")
	}
}
