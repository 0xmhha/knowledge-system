package maintenance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

type job struct {
	mu                sync.Mutex
	status            JobStatus
	request           BuildRequest
	cancel            context.CancelFunc
	journal           *journalStore
	record            *journalRecord
	persistenceFailed bool
	recoveryStarted   bool
}

type Service struct {
	mu       sync.Mutex
	profiles map[string]registered
	jobs     map[string]*job
	keys     map[string]string
	busy     map[string]bool
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	self     string
	closed   bool
	durable  bool
	journals map[string]*journalStore
}

func New(profiles []Profile, self string) (*Service, error) {
	// Keep the public surface process-lifetime until publication and child
	// ownership recovery pass KM-02.3 C–E.
	return newService(profiles, self, false)
}

func newDurableService(profiles []Profile, self string) (*Service, error) {
	return newService(profiles, self, true)
}

func newService(profiles []Profile, self string, durable bool) (*Service, error) {
	if len(profiles) > 32 {
		return nil, reject("unsupported_capability")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{profiles: map[string]registered{}, jobs: map[string]*job{}, keys: map[string]string{}, busy: map[string]bool{}, ctx: ctx, cancel: cancel, self: self, durable: durable, journals: map[string]*journalStore{}}
	for _, spec := range profiles {
		if err := spec.Validate(); err != nil {
			cancel()
			return nil, err
		}
		if _, ok := s.profiles[spec.ID]; ok {
			cancel()
			return nil, reject("profile_unknown")
		}
		o, _, err := loadProfile(spec, self)
		if err != nil {
			cancel()
			return nil, err
		}
		for _, other := range s.profiles {
			if nested(other.out, o.Out) || nested(o.Out, other.out) || nested(other.src, o.Out) || nested(o.Out, other.src) || nested(other.out, o.Src) || nested(o.Src, other.out) {
				cancel()
				return nil, reject("project_mismatch")
			}
		}
		spec.AllowedActivation = append([]string{}, spec.AllowedActivation...)
		s.profiles[spec.ID] = registered{spec: spec, src: o.Src, out: o.Out}
	}
	if durable {
		if err := s.loadJournals(); err != nil {
			s.Close()
			return nil, err
		}
	}
	if durable {
		s.mu.Lock()
		for _, j := range s.jobs {
			s.startRecoveryLocked(s.profiles[j.request.ProfileID], j)
		}
		s.mu.Unlock()
	}
	return s, nil
}

func (s *Service) profile(project, id string) (registered, error) {
	if !identifier.MatchString(project) || !identifier.MatchString(id) {
		return registered{}, reject("invalid_request")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return registered{}, reject("operation_failed")
	}
	p, ok := s.profiles[id]
	if !ok {
		return p, reject("profile_unknown")
	}
	if p.spec.ProjectID != project {
		return p, reject("project_mismatch")
	}
	return p, nil
}

func (s *Service) Inspect(ctx context.Context, r ProfileRequest) (Inspection, error) {
	if r.SchemaVersion != 1 {
		return Inspection{}, reject("invalid_request")
	}
	p, err := s.profile(r.ProjectID, r.ProfileID)
	if err != nil {
		return Inspection{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	value, err := prepare(ctx, p, s.self)
	if err != nil {
		return Inspection{}, err
	}
	persistence := "process_lifetime"
	if s.durable {
		persistence = "journal_recovery_incomplete"
	}
	return Inspection{Profile: value.view, Capabilities: Capabilities{VersionedBuild: true, Cancel: true, JobPersistence: persistence, ServingAdoption: "not_observed", InputPreparation: "existing_reviewed_or_rendered_inputs"}}, nil
}

func snapshot(j *job) JobStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return cloneStatus(j.status)
}

func statusResult(j *job) (JobStatus, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.persistenceFailed {
		// The disk may contain either revision after an uncertain write.
		// Never emit a fabricated recovering revision that a restart could
		// rewind or replace with a different body at the same revision.
		return JobStatus{}, &Error{Code: "reconcile_required", Phase: "reconcile"}
	}
	return cloneStatus(j.status), nil
}

func replayResult(j *job, digest string) (JobStatus, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status.RequestDigest != digest {
		return JobStatus{}, reject("idempotency_conflict")
	}
	if j.persistenceFailed {
		return JobStatus{}, &Error{Code: "reconcile_required", Phase: "reconcile"}
	}
	return cloneStatus(j.status), nil
}

func cloneStatus(value JobStatus) JobStatus {
	// All fields are JSON values; detach maps/pointers so polling cannot mutate a job.
	raw, _ := json.Marshal(value)
	var copy JobStatus
	_ = json.Unmarshal(raw, &copy)
	return copy
}

func (s *Service) lookup(r JobReference) (*job, error) {
	if r.SchemaVersion != 1 || !identifier.MatchString(r.JobID) || !identifier.MatchString(r.ProfileID) || !identifier.MatchString(r.ProjectID) {
		return nil, reject("invalid_request")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[r.JobID]
	if j == nil || j.request.ProjectID != r.ProjectID || j.request.ProfileID != r.ProfileID {
		return nil, reject("job_unknown")
	}
	return j, nil
}

func (s *Service) Status(r JobReference) (JobStatus, error) {
	j, err := s.lookup(r)
	if err != nil {
		return JobStatus{}, err
	}
	return statusResult(j)
}

func (s *Service) Cancel(r JobReference) (CancelResult, error) {
	j, err := s.lookup(r)
	if err != nil {
		return CancelResult{}, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.persistenceFailed {
		return CancelResult{}, &Error{Code: "reconcile_required", Phase: "reconcile"}
	}
	decision := "accepted"
	switch {
	case j.status.State == "recovering":
		decision = "reconcile_required"
	case j.status.Activated:
		decision = "too_late"
	case j.status.Terminal:
		decision = "already_terminal"
	case j.status.State == "cancel_requested":
		decision = "already_requested"
	default:
		j.status.State = "cancel_requested"
		j.status.Revision++
		if err := j.persistLocked(); err != nil {
			return CancelResult{}, err
		}
		j.cancel()
	}
	return CancelResult{Decision: decision, Status: cloneStatus(j.status)}, nil
}

func key(r BuildRequest) string {
	return r.ProjectID + "\x00" + r.ProfileID + "\x00" + r.IdempotencyKey
}

func (s *Service) Build(ctx context.Context, request BuildRequest) (JobStatus, error) {
	if err := request.Validate(); err != nil {
		return JobStatus{}, err
	}
	p, err := s.profile(request.ProjectID, request.ProfileID)
	if err != nil {
		return JobStatus{}, err
	}
	digest, err := Digest(request, "cks.dataset-build.v1")
	if err != nil {
		return JobStatus{}, reject("invalid_request")
	}
	// Replay must precede mutable source/current checks. This registry's scope
	// is explicitly process-lifetime until the durable KM-02.3 stage.
	s.mu.Lock()
	if id := s.keys[key(request)]; id != "" {
		j := s.jobs[id]
		s.mu.Unlock()
		return replayResult(j, digest)
	}
	s.mu.Unlock()
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	prepared, err := prepare(bounded, p, s.self)
	cancel()
	if err != nil {
		return JobStatus{}, err
	}
	if err := checkRequest(request, prepared.view, true); err != nil {
		return JobStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return JobStatus{}, reject("operation_failed")
	}
	if id := s.keys[key(request)]; id != "" {
		j := s.jobs[id]
		return replayResult(j, digest)
	}
	if s.busy[p.out] {
		return JobStatus{}, reject("writer_busy")
	}
	if len(s.jobs) >= 128 {
		return JobStatus{}, reject("unsupported_capability")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return JobStatus{}, reject("operation_failed")
	}
	created := time.Now().UTC()
	deadline := created.Add(time.Duration(request.DeadlineSeconds) * time.Second)
	jobCtx, stop := context.WithDeadline(s.ctx, deadline)
	j := &job{request: request, cancel: stop, status: JobStatus{SchemaVersion: 1, JobID: hex.EncodeToString(random[:]), ProjectID: request.ProjectID, ProfileID: request.ProfileID, RequestDigest: digest, TargetVersion: request.NewVersion,
		Revision: 1, State: "queued", Phase: "accepted", ServingAdoption: "not_observed", Gates: map[string]string{"source": "not_run", "identity": "not_run", "graph": "not_run", "vector": "not_run", "knowledge": "not_configured", "semantic": "not_configured"}}}
	for _, gate := range prepared.view.RequiredGates {
		j.status.Gates[gate] = "not_run"
	}
	if s.durable {
		store, err := s.ensureJournal(ctx, p, prepared, request)
		if err != nil {
			stop()
			return JobStatus{}, err
		}
		record := journalRecord{SchemaVersion: 1, ExecutionProtocol: "owner-socket-mutation-lease-staging-v2", Request: request, Profile: prepared.view, ExpectedDatasetID: prepared.identity.DatasetID,
			CreatedAt: created.Format(time.RFC3339Nano), DeadlineAt: deadline.Format(time.RFC3339Nano), Status: cloneStatus(j.status)}
		if err := store.save(record); err != nil {
			stop()
			return JobStatus{}, err
		}
		j.journal, j.record = store, &record
	}
	s.jobs[j.status.JobID], s.keys[key(request)], s.busy[p.out] = j, j.status.JobID, true
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer stop()
		s.execute(jobCtx, p, prepared, j)
		s.mu.Lock()
		if snapshot(j).Terminal {
			delete(s.busy, p.out)
		} else {
			s.startRecoveryLocked(p, j)
		}
		s.mu.Unlock()
	}()
	return statusResult(j)
}

func (j *job) phase(phase string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status.Terminal || j.persistenceFailed || j.status.Phase == phase {
		return
	}
	j.status.Phase = phase
	j.status.Revision++
	if j.status.State != "cancel_requested" {
		j.status.State = "running"
	}
	_ = j.persistLocked()
}

func (s *Service) execute(ctx context.Context, p registered, value prepared, j *job) {
	r := j.request
	j.phase("capture")
	validate := func(ctx context.Context, checkVersion bool) error {
		now, err := prepare(ctx, p, s.self)
		if err != nil {
			return err
		}
		return checkRequest(r, now.view, checkVersion)
	}
	setCandidate := func() error {
		id, err := setup.InspectVersionIdentityContext(ctx, filepath.Join(p.out, r.NewVersion))
		if err != nil || id == nil || id.DatasetID != value.identity.DatasetID || id.Source != value.source || id.InputDigest != r.InputDigest {
			return reject("gate_failed")
		}
		j.mu.Lock()
		defer j.mu.Unlock()
		j.status.Candidate = &Candidate{Version: r.NewVersion, Coordinates: coordinates(*id), InputDigest: r.InputDigest, EmbeddingDigest: r.EmbeddingDigest}
		for _, gate := range value.view.RequiredGates {
			j.status.Gates[gate] = "passed"
		}
		j.status.BuildComplete = true
		j.status.Revision++
		return j.persistLocked()
	}
	gates := setup.GateOptions{PublicationID: j.requestID(), GraphBin: value.options.GraphBin, Src: value.options.Src, ExpectedSourceCommit: value.source.SourceCommit,
		ExpectedSourceSnapshot: value.source, ExpectedInputDigest: r.InputDigest, ExpectedDatasetID: value.identity.DatasetID, ExternalOrigins: value.options.ExternalOrigins,
		HoldForReview: r.Activation == "hold_for_review", BeforeCandidate: func(ctx context.Context) error {
			if err := validate(ctx, true); err != nil {
				return err
			}
			return claimRoot(p.out, r.ProjectID)
		}}
	if value.source.SourceMode != "committed" {
		gates.ExpectedSourceCommit = ""
	}
	gates.Publish = func(ctx context.Context, promote func() (string, error)) (string, error) {
		if err := validate(ctx, false); err != nil {
			return "", err
		}
		if err := setCandidate(); err != nil {
			return "", err
		}
		j.phase("publish")
		j.mu.Lock()
		defer j.mu.Unlock()
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if j.status.State == "cancel_requested" {
			return "", context.Canceled
		}
		previous, err := promote()
		if err != nil {
			return previous, err
		}
		proof, err := setup.ReadManagedPublication(p.out, j.status.JobID)
		if err != nil {
			return previous, errors.Join(setup.ErrDurabilityUncertain, err)
		}
		publication, err := publicationFromProof(r, j.status.JobID, value.identity.DatasetID, j.status.Candidate, proof)
		if err != nil {
			return previous, errors.Join(setup.ErrPublicationUnresolved, err)
		}
		j.status.Publication = publication
		j.status.State, j.status.Phase, j.status.Terminal, j.status.Activated = "succeeded", "finished", true, true
		j.status.Revision++
		return previous, j.persistLocked()
	}
	err := setup.Reindex(ctx, value.options, r.NewVersion, gates, setup.SubprocessRunner{GuardianBinary: value.options.CksBin}, func(event setup.Event) {
		if event.Type != "start" && event.Type != "done" {
			return
		}
		switch {
		case strings.Contains(event.Step, "gate"):
			j.phase("gate")
		case strings.Contains(event.Step, "ckv") || strings.Contains(event.Step, "vector"):
			j.phase("build_vector")
		case strings.Contains(event.Step, "ckg") || strings.Contains(event.Step, "graph"):
			j.phase("build_graph")
		}
	})
	if err == nil && r.Activation == "hold_for_review" {
		err = validate(ctx, false)
		if err == nil {
			err = setCandidate()
		}
	}
	// A stopped guardian is not sufficient if an escaped descendant still
	// retains inherited writer ownership. Reindex has released our own copy.
	var quiescenceErr error
	if err != nil && !errors.Is(err, setup.ErrDatasetWriterBusy) {
		probe, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		quiescenceErr = setup.WithDatasetMutationLock(probe, p.out, func(ctx context.Context) error {
			return setup.CleanupAbandonedBuildStaging(ctx, p.out, r.NewVersion, r.ProjectID, r.SourcePin.SnapshotID, value.identity.DatasetID, j.requestID())
		})
		cancel()
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status.Terminal || j.persistenceFailed {
		return
	}
	if errors.Is(err, setup.ErrDurabilityUncertain) || errors.Is(err, setup.ErrPublicationUnresolved) || quiescenceErr != nil {
		// current may already have changed. Publication reconciliation must
		// settle this before reporting failure/cancellation or admitting a
		// replacement writer. C–E will provide the durable proof/recovery.
		j.status.State, j.status.Phase = "recovering", "reconcile"
		j.status.Revision++
		_ = j.persistLocked()
		return
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		j.status.State = "held_for_review"
	} else {
		code := "gate_failed"
		var safe *Error
		if errors.As(err, &safe) {
			code = safe.Code
		}
		if errors.Is(err, setup.ErrDatasetWriterBusy) {
			code = "writer_busy"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			code = "deadline_exceeded"
		}
		switch {
		case errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled):
			j.status.State, code = "canceled", "canceled"
		default:
			j.status.State = "failed"
		}
		j.status.Error = &Error{Code: code, Retryable: code == "writer_busy", Phase: j.status.Phase}
	}
	j.status.Phase, j.status.Terminal = "finished", true
	j.status.Revision++
	_ = j.persistLocked()
}

func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	for _, store := range s.journals {
		store.close()
	}
}
