package maintenance

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

func proofMatchesRequest(r BuildRequest, jobID, expectedDataset string, candidate *Candidate, proof *setup.ManagedPublication) error {
	if proof == nil || proof.OperationID != jobID || proof.ProjectID != r.ProjectID || proof.Version != r.NewVersion ||
		proof.Identity.DatasetID != expectedDataset || proof.Identity.InputDigest != r.InputDigest || candidate == nil || candidate.Coordinates != coordinates(proof.Identity) ||
		candidate.Version != r.NewVersion || candidate.InputDigest != r.InputDigest || candidate.EmbeddingDigest != r.EmbeddingDigest {
		return reject("reconcile_required")
	}
	c := coordinates(proof.Identity)
	if c.ProjectID != r.ProjectID || c.SnapshotID != r.SourcePin.SnapshotID || c.SourceMode != r.SourcePin.Mode || c.BaseCommit != r.SourcePin.BaseCommit {
		return reject("reconcile_required")
	}
	var model any
	if json.Unmarshal(proof.Identity.EmbeddingIdentity, &model) != nil {
		return reject("reconcile_required")
	}
	modelDigest, err := Digest(model, "cks.dataset-embedding.v1")
	if err != nil || modelDigest != r.EmbeddingDigest {
		return reject("reconcile_required")
	}
	if r.Base.State == "absent" {
		if proof.PreviousVersion != "" || proof.PreviousIdentity != nil {
			return reject("reconcile_required")
		}
	} else if proof.PreviousVersion != r.Base.Version || proof.PreviousIdentity == nil || r.Base.Coordinates == nil || coordinates(*proof.PreviousIdentity) != *r.Base.Coordinates {
		return reject("reconcile_required")
	}
	return nil
}

func publicationFromProof(r BuildRequest, jobID, expectedDataset string, candidate *Candidate, proof *setup.ManagedPublication) (*Publication, error) {
	if proof == nil || proof.Outcome != "committed" {
		return nil, reject("reconcile_required")
	}
	if err := proofMatchesRequest(r, jobID, expectedDataset, candidate, proof); err != nil {
		return nil, err
	}
	body := map[string]any{"base": r.Base, "version": r.NewVersion, "coordinates": candidate.Coordinates, "published_at": proof.ConfirmedAt}
	receipt, err := Digest(body, "cks.dataset-publication.v1")
	if err != nil {
		return nil, err
	}
	return &Publication{Base: r.Base, Version: r.NewVersion, Coordinates: candidate.Coordinates, PublishedAt: proof.ConfirmedAt, ReceiptDigest: receipt}, nil
}

// Caller holds Service.mu before adding a waiter. Close sets closed under the
// same mutex before Wait, so a late job cannot add a recovery task after Wait.
func (s *Service) startRecoveryLocked(p registered, j *job) {
	if s.closed || !s.durable {
		return
	}
	j.mu.Lock()
	if j.recoveryStarted || j.persistenceFailed || j.record == nil || j.status.State != "recovering" {
		j.mu.Unlock()
		return
	}
	j.recoveryStarted = true
	j.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
			done, _ := s.recoverDurableJobOnce(ctx, p, j, false)
			cancel()
			if done {
				s.mu.Lock()
				delete(s.busy, p.out)
				s.mu.Unlock()
				return
			}
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// C can settle checked publication/held candidates whose persisted build/gates
// completed. Missing publication is NOT proof that a pre-gate child stopped;
// D must establish quiescence before allowInterrupted may be true for it.
func (s *Service) recoverDurableJobOnce(ctx context.Context, p registered, j *job, allowInterrupted bool) (bool, error) {
	done := false
	err := setup.WithReconciledManagedPublication(ctx, p.out, j.requestID(), func(proof *setup.ManagedPublication) error {
		j.mu.Lock()
		defer j.mu.Unlock()
		before := cloneStatus(j.status)
		defer func() {
			if !done && !j.persistenceFailed {
				j.status = before
			}
		}()
		if j.persistenceFailed || j.record == nil {
			return reject("reconcile_required")
		}
		if j.status.Terminal {
			done = true
			return nil
		}
		if j.status.State != "recovering" {
			return reject("reconcile_required")
		}
		record := j.record
		if proof != nil {
			if !record.Status.BuildComplete || record.Status.Candidate == nil {
				return reject("reconcile_required")
			}
			switch proof.Outcome {
			case "committed":
				if record.CancelRequested || record.Request.Activation != "activate_after_gate" {
					return reject("reconcile_required")
				}
				publication, err := publicationFromProof(record.Request, j.status.JobID, record.ExpectedDatasetID, record.Status.Candidate, proof)
				if err != nil {
					return err
				}
				j.status.Publication = publication
				j.status.State, j.status.Activated = "succeeded", true
			case "not_committed":
				if err := proofMatchesRequest(record.Request, j.status.JobID, record.ExpectedDatasetID, record.Status.Candidate, proof); err != nil {
					return err
				}
				setInterruptedOutcome(j, record.CancelRequested)
			default:
				return reject("reconcile_required")
			}
		} else if record.Status.BuildComplete && record.CancelRequested {
			setInterruptedOutcome(j, true)
		} else if record.Status.BuildComplete && record.Request.Activation == "hold_for_review" {
			identity, err := setup.InspectVersionIdentityContext(ctx, filepath.Join(p.out, record.Request.NewVersion))
			if err != nil || identity == nil || identity.DatasetID != record.ExpectedDatasetID || record.Status.Candidate == nil || coordinates(*identity) != record.Status.Candidate.Coordinates {
				return reject("reconcile_required")
			}
			raw, err := readJournalFile(filepath.Join(p.out, record.Request.NewVersion, "review-hold.json"))
			if err != nil {
				return err
			}
			var hold struct {
				ProjectID   string `json:"project_id"`
				DatasetID   string `json:"dataset_id"`
				SnapshotID  string `json:"snapshot_id"`
				BaseVersion string `json:"base_version"`
			}
			if Decode(raw, &hold) != nil || hold.ProjectID != record.Request.ProjectID || hold.DatasetID != record.ExpectedDatasetID || hold.SnapshotID != record.Request.SourcePin.SnapshotID || hold.BaseVersion != record.Request.Base.Version {
				return reject("reconcile_required")
			}
			j.status.State = "held_for_review"
		} else if record.Status.BuildComplete || allowInterrupted {
			setInterruptedOutcome(j, record.CancelRequested)
		} else {
			return nil
		}
		j.status.Phase, j.status.Terminal = "finished", true
		j.status.Revision++
		if err := ValidateStatus(j.status, record.Request, record.Profile); err != nil {
			return err
		}
		if err := j.persistLocked(); err != nil {
			return err
		}
		done = true
		return nil
	})
	return done, err
}

func setInterruptedOutcome(j *job, cancelRequested bool) {
	state := "interrupted"
	if cancelRequested {
		state = "canceled"
	}
	j.status.State = state
	j.status.Error = &Error{Code: state, Phase: "reconcile"}
}

func (j *job) requestID() string { j.mu.Lock(); defer j.mu.Unlock(); return j.status.JobID }
