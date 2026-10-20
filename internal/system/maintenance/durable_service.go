package maintenance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

const journalDirectory = ".maintenance-jobs"

// persistLocked is called with the job mutex held, before acknowledging a
// state change. A failed/uncertain disk write cannot expose stale memory as a
// terminal result. Keep the writer owner until execution stops and Close runs.
func (j *job) persistLocked() error {
	if j.journal == nil {
		return nil
	}
	if j.persistenceFailed || j.record == nil {
		return reject("reconcile_required")
	}
	next := cloneRecord(*j.record)
	next.Status = cloneStatus(j.status)
	if j.status.State == "cancel_requested" {
		next.CancelRequested = true
	}
	if err := j.journal.save(next); err != nil {
		j.status = cloneStatus(j.record.Status)
		if !j.status.Terminal {
			j.status.State, j.status.Phase = "recovering", "reconcile"
			j.status.Revision++
		}
		j.persistenceFailed = true
		j.cancel()
		return reject("reconcile_required")
	}
	*j.record = next
	return nil
}

// loadJournals never restarts an interrupted build. Nonterminal records remain
// recovering until the publication and live-child ownership checks are wired.
func (s *Service) loadJournals() error {
	ids := make([]string, 0, len(s.profiles))
	for id := range s.profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := s.profiles[id]
		root := filepath.Join(p.out, journalDirectory)
		if _, err := os.Lstat(root); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return reject("reconcile_required")
		}
		owned, err := ownsRoot(p.out, p.spec.ProjectID)
		if err != nil || !owned {
			return reject("project_mismatch")
		}
		store, err := openJournal(root, p.spec.ProjectID, p.spec.ID, false)
		if err != nil {
			return err
		}
		if store == nil {
			return reject("reconcile_required")
		}
		s.journals[p.out] = store
		rows, err := store.list()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if s.jobs[row.Status.JobID] != nil || s.keys[key(row.Request)] != "" {
				return reject("reconcile_required")
			}
			r := cloneRecord(row)
			if r.Status.State == "cancel_requested" {
				r.CancelRequested = true
			}
			j := &job{request: r.Request, status: cloneStatus(r.Status), cancel: func() {}, journal: store, record: &r}
			if !j.status.Terminal {
				if s.busy[p.out] {
					return reject("reconcile_required")
				}
				if j.status.State != "recovering" {
					j.status.State, j.status.Phase = "recovering", "reconcile"
					j.status.Revision++
					if err := j.persistLocked(); err != nil {
						return err
					}
				}
				s.busy[p.out] = true
			}
			s.jobs[r.Status.JobID], s.keys[key(r.Request)] = j, r.Status.JobID
		}
	}
	return nil
}

// Caller owns Service.mu. Creating the registry uses the existing dataset
// mutation lock and repeats all admission pins before claiming a new root.
func (s *Service) ensureJournal(ctx context.Context, p registered, prepared prepared, request BuildRequest) (*journalStore, error) {
	if store := s.journals[p.out]; store != nil {
		return store, nil
	}
	var store *journalStore
	err := setup.WithDatasetMutationLock(ctx, p.out, func(ctx context.Context) error {
		current, err := prepare(ctx, p, s.self)
		if err != nil {
			return err
		}
		if err := checkRequest(request, current.view, true); err != nil {
			return err
		}
		if !equalJSON(current.identity, prepared.identity) {
			return reject("source_stale")
		}
		if err := claimRoot(p.out, p.spec.ProjectID); err != nil {
			return err
		}
		var openErr error
		store, openErr = openJournal(filepath.Join(p.out, journalDirectory), p.spec.ProjectID, p.spec.ID, true)
		return openErr
	})
	if err != nil {
		if store != nil {
			store.close()
		}
		if errors.Is(err, setup.ErrDatasetWriterBusy) {
			return nil, reject("writer_busy")
		}
		return nil, err
	}
	s.journals[p.out] = store
	return store, nil
}
