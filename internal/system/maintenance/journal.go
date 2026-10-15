package maintenance

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// A durable record binds the admitted request and original inspection, rather
// than reinterpreting a past job using the project's mutable current inputs.
// The public service is not switched to this store until recovery is wired.
type journalRecord struct {
	SchemaVersion     int            `json:"schema_version"`
	Request           BuildRequest   `json:"request"`
	Profile           ProfileContext `json:"profile"`
	ExpectedDatasetID string         `json:"expected_dataset_id"`
	CreatedAt         string         `json:"created_at"`
	DeadlineAt        string         `json:"deadline_at"`
	// Optional for compatibility with private A/B records. Once set, this
	// intent survives the public status changing to recovering.
	CancelRequested   bool      `json:"cancel_requested,omitempty"`
	ExecutionProtocol string    `json:"execution_protocol,omitempty"`
	Status            JobStatus `json:"status"`
}

type journalEnvelope struct {
	Record   journalRecord `json:"record"`
	Checksum string        `json:"checksum"`
}

type journalOwner struct {
	SchemaVersion int    `json:"schema_version"`
	ProjectID     string `json:"project_id"`
	ProfileID     string `json:"profile_id"`
}

type journalStore struct {
	mu       sync.Mutex
	root     string
	owner    journalOwner
	lock     *os.File
	poisoned bool
	records  map[string]journalRecord
	keys     map[string]string
	// Test-only fault injection at the persistence boundary; never configured
	// by public requests, operator profiles or environment variables.
	fault func(string) error
}

func validateRecord(r journalRecord) error {
	if r.ExecutionProtocol != "" && r.ExecutionProtocol != "owner-socket-mutation-lease-v1" && r.ExecutionProtocol != "owner-socket-mutation-lease-staging-v2" {
		return reject("reconcile_required")
	}
	created, cerr := time.Parse(time.RFC3339Nano, r.CreatedAt)
	deadline, derr := time.Parse(time.RFC3339Nano, r.DeadlineAt)
	if r.SchemaVersion != 1 || cerr != nil || derr != nil || deadline.Sub(created) != time.Duration(r.Request.DeadlineSeconds)*time.Second ||
		!hashPattern.MatchString(r.ExpectedDatasetID) || !validJournalProfile(r.Profile) || checkRequest(r.Request, r.Profile, true) != nil ||
		ValidateStatus(r.Status, r.Request, r.Profile) != nil ||
		(r.Status.Candidate != nil && r.Status.Candidate.Coordinates.DatasetID != r.ExpectedDatasetID) {
		return reject("reconcile_required")
	}
	if r.CancelRequested && slices.Contains([]string{"queued", "running", "succeeded", "held_for_review"}, r.Status.State) {
		return reject("reconcile_required")
	}
	return nil
}

func validJournalProfile(p ProfileContext) bool {
	if p.ExistingVersions == nil || len(p.ExistingVersions) > 10000 || len(p.AllowedActivation) < 1 || len(p.AllowedActivation) > 2 ||
		len(p.RequiredGates) < 4 || len(p.RequiredGates) > 6 || p.TimeoutMaxSeconds < 10 || p.TimeoutMaxSeconds > 86400 {
		return false
	}
	for _, group := range [][]string{p.ExistingVersions, p.AllowedActivation, p.RequiredGates} {
		seen := map[string]bool{}
		for _, name := range group {
			if seen[name] {
				return false
			}
			seen[name] = true
		}
	}
	for _, v := range p.ExistingVersions {
		if !validVersion(v) {
			return false
		}
	}
	for _, v := range p.AllowedActivation {
		if v != "hold_for_review" && v != "activate_after_gate" {
			return false
		}
	}
	for _, v := range p.RequiredGates {
		if !slices.Contains(gateNames, v) {
			return false
		}
	}
	return true
}

// Caller must establish ownership under the dataset mutation lock before
// create=true. Opening an absent store with create=false has no write effects.
func openJournal(root, project, profile string, create bool) (*journalStore, error) {
	if !identifier.MatchString(project) || !identifier.MatchString(profile) || safeDirectory(root, true) != nil {
		return nil, reject("reconcile_required")
	}
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if !create {
			return nil, nil
		}
		if err := os.Mkdir(root, 0o700); err != nil {
			return nil, reject("reconcile_required")
		}
		if err := syncJournalDir(filepath.Dir(root)); err != nil {
			return nil, err
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, reject("reconcile_required")
	}
	lockPath := filepath.Join(root, ".registry.lock")
	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, reject("reconcile_required")
	}
	lock := os.NewFile(uintptr(fd), lockPath)
	var stat unix.Stat_t
	if unix.Fstat(fd, &stat) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 {
		_ = lock.Close()
		return nil, reject("reconcile_required")
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, reject("writer_busy")
	}
	store := &journalStore{root: root, owner: journalOwner{1, project, profile}, lock: lock, records: map[string]journalRecord{}, keys: map[string]string{}}
	fail := func() (*journalStore, error) { store.close(); return nil, reject("reconcile_required") }
	ownerPath := filepath.Join(root, ".owner.json")
	raw, err := readJournalFile(ownerPath)
	if os.IsNotExist(err) && create {
		entries, readErr := os.ReadDir(root)
		if readErr != nil || len(entries) != 1 || entries[0].Name() != ".registry.lock" {
			return fail()
		}
		if store.writeAtomic(".owner.json", store.owner) != nil {
			return fail()
		}
	} else {
		var owner journalOwner
		if err != nil || Decode(raw, &owner) != nil || owner != store.owner {
			return fail()
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fail()
	}
	for _, entry := range entries {
		if entry.Name() == ".registry.lock" || entry.Name() == ".owner.json" || strings.HasPrefix(entry.Name(), ".record.tmp-") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !strings.HasSuffix(entry.Name(), ".json") || !identifier.MatchString(id) || len(store.records) >= 128 {
			return fail()
		}
		raw, err := readJournalFile(filepath.Join(root, entry.Name()))
		var envelope journalEnvelope
		if err != nil || Decode(raw, &envelope) != nil || validateRecord(envelope.Record) != nil {
			return fail()
		}
		digest, err := Digest(envelope.Record, "cks.dataset-journal.v1")
		r := envelope.Record
		if err != nil || digest != envelope.Checksum || r.Status.JobID != id || r.Request.ProjectID != project || r.Request.ProfileID != profile || store.keys[key(r.Request)] != "" {
			return fail()
		}
		store.records[id], store.keys[key(r.Request)] = r, id
	}
	return store, nil
}

func readJournalFile(path string) ([]byte, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() { _ = file.Close() }()
	var stat unix.Stat_t
	if unix.Fstat(fd, &stat) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 || stat.Size > 64<<10 {
		return nil, reject("reconcile_required")
	}
	data, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err == nil && len(data) > 64<<10 {
		err = reject("reconcile_required")
	}
	return data, err
}

func syncJournalDir(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return file.Sync()
}

func (s *journalStore) writeAtomic(name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 64<<10 {
		return reject("reconcile_required")
	}
	file, err := os.CreateTemp(s.root, ".record.tmp-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if s.fault != nil {
		if err := s.fault("before-rename"); err != nil {
			return err
		}
	}
	if err := os.Rename(file.Name(), filepath.Join(s.root, name)); err != nil {
		return err
	}
	if s.fault != nil {
		if err := s.fault("after-rename"); err != nil {
			return err
		}
	}
	return syncJournalDir(s.root)
}

func cloneRecord(value journalRecord) journalRecord {
	raw, _ := json.Marshal(value)
	var result journalRecord
	_ = json.Unmarshal(raw, &result)
	return result
}

func (s *journalStore) save(value journalRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil || s.poisoned || validateRecord(value) != nil || value.Request.ProjectID != s.owner.ProjectID || value.Request.ProfileID != s.owner.ProfileID {
		return reject("reconcile_required")
	}
	id := value.Status.JobID
	if existing := s.keys[key(value.Request)]; existing != "" && existing != id {
		return reject("idempotency_conflict")
	}
	if old, exists := s.records[id]; exists {
		if (old.CancelRequested && !value.CancelRequested) ||
			(!old.CancelRequested && value.CancelRequested && old.Status.State != "cancel_requested" && value.Status.State != "cancel_requested") {
			return reject("reconcile_required")
		}
		if !equalJSON(old.Request, value.Request) || !equalJSON(old.Profile, value.Profile) || old.ExpectedDatasetID != value.ExpectedDatasetID ||
			old.CreatedAt != value.CreatedAt || old.DeadlineAt != value.DeadlineAt || old.ExecutionProtocol != value.ExecutionProtocol || validateJournalUpdate(old.Status, value.Status) != nil {
			return reject("reconcile_required")
		}
		if equalJSON(old, value) {
			return nil
		}
	} else if len(s.records) >= 128 || value.Status.State != "queued" || value.Status.Revision != 1 {
		return reject("unsupported_capability")
	}
	digest, err := Digest(value, "cks.dataset-journal.v1")
	if err == nil {
		err = s.writeAtomic(id+".json", journalEnvelope{Record: value, Checksum: digest})
	}
	if err != nil {
		// A rename may already have happened. Never acknowledge another write
		// using a stale in-memory view. Retain exclusive ownership until the
		// runtime stops the job and closes the store; then reopen/reconcile.
		s.poisoned = true
		return reject("reconcile_required")
	}
	s.records[id], s.keys[key(value.Request)] = cloneRecord(value), id
	return nil
}

func (s *journalStore) list() ([]journalRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil || s.poisoned {
		return nil, reject("reconcile_required")
	}
	ids := make([]string, 0, len(s.records))
	for id := range s.records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]journalRecord, 0, len(ids))
	for _, id := range ids {
		result = append(result, cloneRecord(s.records[id]))
	}
	return result, nil
}

func (s *journalStore) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock != nil {
		_ = s.lock.Close() // Closing the owning descriptor releases flock.
		s.lock = nil
	}
}
