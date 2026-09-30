package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Blue-green reindex orchestration (reindex-migration-design §4/§5).
//
// Layout: a dataset root holds immutable version directories and a `current`
// symlink that points at the active one:
//
//	<dataset>/<version>/{graph,vector}   # a completed, aligned build
//	<dataset>/current -> <version>       # what serving resolves
//
// Reindex builds a NEW version, runs the pre-promote gate suite, and only then
// flips `current` atomically. A crash mid-build leaves `current` on the old
// version (the live index is never partial); rollback re-points it. The fused
// server resolves `current` once at startup and pins it, so a reader sees the
// old or the new target, never a missing one — serving restart (adopting the
// new version) is the instance-level blue-green step (design P5), out of scope
// here.

// validateVersion rejects version labels that are not a single, safe directory
// name. The label becomes both a build subdirectory (dataset/<version>) and a
// promote symlink target, so a value like "current", "..", or "a/b" would
// escape the dataset root or build straight through the live `current` symlink,
// corrupting the served index before the gate runs.
func validateVersion(version string) error {
	switch {
	case version == "":
		return fmt.Errorf("reindex: version is required")
	case version == "current":
		return fmt.Errorf("reindex: version %q is reserved (it is the live pointer)", version)
	case version != filepath.Base(version) || strings.ContainsRune(version, filepath.Separator):
		return fmt.Errorf("reindex: version %q must be a single path element", version)
	case strings.HasPrefix(version, "."):
		return fmt.Errorf("reindex: version %q must not start with a dot", version)
	}
	return nil
}

// NewVersion returns a fresh, sortable version label for a reindex when the
// caller does not pin one: "v<unix-seconds>". Monotonic across reindexes of a
// dataset, so version directories order chronologically.
func NewVersion() string {
	return fmt.Sprintf("v%d", time.Now().UTC().Unix())
}

// reindexLock is a dataset-level advisory lock serializing coordinated
// reindexes (design §5.3). Reads (serving) are unaffected — they go through the
// atomic `current` swap, not the lock.
type reindexLock struct{ path string }

// reindexLockStaleAfter is the age past which a held lock is treated as
// abandoned even if its owner PID still exists (guards against PID reuse). A
// real reindex completes in minutes, so this ceiling never steals a live build.
const reindexLockStaleAfter = 6 * time.Hour

func acquireReindexLock(dataset string) (*reindexLock, error) {
	if err := os.MkdirAll(dataset, 0o755); err != nil {
		return nil, fmt.Errorf("reindex: prepare dataset dir: %w", err)
	}
	p := filepath.Join(dataset, ".reindex.lock")
	if l, err := createReindexLock(p); err == nil {
		return l, nil
	} else if !os.IsExist(err) {
		return nil, fmt.Errorf("reindex: acquire lock: %w", err)
	}
	// The lock exists. Reclaim it only if the previous holder crashed (its PID is
	// gone) or it is older than the staleness ceiling; otherwise a reindex is
	// genuinely in progress and we refuse.
	reason, stale := reindexLockStale(p)
	if !stale {
		return nil, fmt.Errorf("reindex: another reindex is in progress (holds %s) — wait for it to finish", p)
	}
	_ = os.Remove(p)
	l, err := createReindexLock(p)
	if err != nil {
		return nil, fmt.Errorf("reindex: reclaimed a stale lock (%s) but could not re-acquire %s: %w", reason, p, err)
	}
	return l, nil
}

// createReindexLock creates the lock file exclusively, stamping the owner PID
// and acquire time so a later contender can judge staleness.
func createReindexLock(p string) (*reindexLock, error) {
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), time.Now().UTC().Unix())
	_ = f.Close()
	return &reindexLock{path: p}, nil
}

// reindexLockStale reports whether the lock at p is abandoned: unreadable or
// malformed, its owner PID no longer alive, or held past reindexLockStaleAfter.
func reindexLockStale(p string) (reason string, stale bool) {
	buf, err := os.ReadFile(p)
	if err != nil {
		return "lock unreadable", true
	}
	lines := strings.SplitN(strings.TrimSpace(string(buf)), "\n", 2)
	pid, _ := strconv.Atoi(strings.TrimSpace(lines[0]))
	if pid <= 0 {
		return "lock malformed", true
	}
	if syscall.Kill(pid, 0) != nil {
		return fmt.Sprintf("owner pid %d is gone", pid), true
	}
	if len(lines) > 1 {
		if ts, perr := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64); perr == nil {
			if age := time.Duration(time.Now().UTC().Unix()-ts) * time.Second; age > reindexLockStaleAfter {
				return fmt.Sprintf("held %s (exceeds %s)", age, reindexLockStaleAfter), true
			}
		}
	}
	return "", false
}

func (l *reindexLock) release() {
	if l != nil {
		_ = os.Remove(l.path)
	}
}

// Promote atomically points <dataset>/current at <version> (a temp relative
// symlink renamed over `current`, so a concurrent reader resolving `current`
// sees the old or the new target, never a missing one). It returns the version
// `current` pointed at before the swap ("" if none) so the caller can record a
// rollback target. The version directory must already exist.
func Promote(dataset, version string) (prev string, err error) {
	if err := validateVersion(version); err != nil {
		return "", err
	}
	vdir := filepath.Join(dataset, version)
	fi, err := os.Stat(vdir)
	if err != nil || !fi.IsDir() {
		return "", fmt.Errorf("promote: version %q not found under %s", version, dataset)
	}
	targetIdentity, err := verifyVersionIdentityIfPresent(vdir)
	if err != nil {
		return "", fmt.Errorf("promote: target identity: %w", err)
	}
	current := filepath.Join(dataset, "current")
	if t, rerr := os.Readlink(current); rerr == nil {
		if err := validateVersion(t); err != nil {
			return "", fmt.Errorf("promote: unsafe current target: %w", err)
		}
		prev = t
		if targetIdentity != nil {
			currentIdentity, err := verifyVersionIdentityIfPresent(filepath.Join(dataset, t))
			if err != nil {
				return prev, fmt.Errorf("promote: current identity: %w", err)
			}
			if currentIdentity != nil && currentIdentity.Source.ProjectID != targetIdentity.Source.ProjectID {
				return prev, fmt.Errorf("promote: cannot mix project IDs in one dataset root")
			}
		}
	}
	tmp := filepath.Join(dataset, fmt.Sprintf(".current.tmp-%d", os.Getpid()))
	_ = os.Remove(tmp)
	if err := os.Symlink(version, tmp); err != nil { // relative target: the version name
		return prev, fmt.Errorf("promote: stage symlink: %w", err)
	}
	if err := os.Rename(tmp, current); err != nil {
		_ = os.Remove(tmp)
		return prev, fmt.Errorf("promote: swap current: %w", err)
	}
	return prev, nil
}

// Rollback re-points current at a prior version (the same atomic swap).
func Rollback(dataset, version string) error {
	_, err := Promote(dataset, version)
	return err
}

// GateOptions parameterizes the pre-promote gate suite.
type GateOptions struct {
	// GraphBin is the graph CLI for the validate/audit gates (default "ckg").
	GraphBin string
	// Src is the source tree; when set the (soft) ckg audit gate runs.
	Src string
	// MinCanonicalRatio is the floor for canonical_id coverage
	// (CanonicalCount/SymbolCount). Zero disables the check.
	MinCanonicalRatio float64
	// TestCommand is an optional argv run against the pinned, clean source
	// after the candidate indexes pass structural gates and before promotion.
	TestCommand []string
	// ExpectedSourceCommit, when set, enforces committed mode at promotion.
	// A dirty or moved source tree, or an index from another commit, fails.
	ExpectedSourceCommit string
	// ExpectedSourceSnapshot pins the exact tracked bytes at build start.
	// Empty means a legacy commit-only build.
	ExpectedSourceSnapshot SourceIdentity
	ExpectedInputDigest    string
	ExpectedDatasetID      string
}

// gateVecManifest is the read-only projection of the vector manifest the gate
// needs (counts for the chunk/canonical gates).
type gateVecManifest struct {
	ChunkCount     int `json:"chunk_count"`
	SymbolCount    int `json:"symbol_count"`
	CanonicalCount int `json:"canonical_count"`
}

// Gate runs the pre-promote checks against a built version directory
// (<dataset>/<version>). It returns an error the moment a HARD gate fails, so
// the caller leaves `current` unchanged. The ckg-audit gate is soft (a warning,
// not a failure). Gates: ckg validate (structural), verify-align
// (commit+digest+schema≥1.19), vector chunk_count>0, canonical_id coverage,
// ckg audit (soft).
func Gate(ctx context.Context, dataset, version string, o GateOptions, r Runner, emit func(Event)) error {
	warn := func(msg string) {
		if emit != nil {
			emit(Event{Time: time.Now().UTC(), Step: "reindex-gate", Type: "warning", Message: msg})
		}
	}
	graphBin := o.GraphBin
	if graphBin == "" {
		graphBin = "ckg"
	}
	vdir := filepath.Join(dataset, version)
	graphDir := filepath.Join(vdir, "graph")
	vectorDir := filepath.Join(vdir, "vector")

	// 1. Structural graph validation (hard).
	if err := r.Run(ctx, Step{
		ID: "reindex-gate", Title: "gate: ckg validate",
		Cmd: []string{graphBin, "validate", "--graph", graphDir},
	}, emit); err != nil {
		return fmt.Errorf("gate: ckg validate failed: %w", err)
	}

	// 2. Coordinate alignment (hard): same commit, matching digest pin, schema>=1.19.
	if err := VerifyAlignment(graphDir, vectorDir, emit); err != nil {
		return fmt.Errorf("gate: %w", err)
	}
	if o.ExpectedSourceSnapshot.SnapshotID != "" {
		if err := VerifyCandidateIdentity(vdir, o.ExpectedSourceSnapshot, o.ExpectedInputDigest); err != nil {
			return fmt.Errorf("gate: %w", err)
		}
		var candidate DatasetIdentity
		if err := readJSON(filepath.Join(vdir, "dataset-identity.json"), &candidate); err != nil || candidate.DatasetID != o.ExpectedDatasetID {
			return fmt.Errorf("gate: candidate dataset ID differs from pre-build identity: %v", err)
		}
	}

	// 3 & 4. Vector chunk count + canonical_id coverage (hard).
	var vm gateVecManifest
	if err := readJSON(filepath.Join(vectorDir, "manifest.json"), &vm); err != nil {
		return fmt.Errorf("gate: read vector manifest: %w", err)
	}
	if vm.ChunkCount <= 0 {
		return fmt.Errorf("gate: vector index has %d chunks — refusing to promote an empty index", vm.ChunkCount)
	}
	if o.MinCanonicalRatio > 0 {
		if vm.SymbolCount == 0 {
			return fmt.Errorf("gate: canonical coverage unverifiable — vector manifest reports 0 symbol chunks")
		}
		ratio := float64(vm.CanonicalCount) / float64(vm.SymbolCount)
		if ratio < o.MinCanonicalRatio {
			return fmt.Errorf("gate: canonical_id coverage %.1f%% (%d/%d) < %.1f%% — vector<->graph join too sparse",
				ratio*100, vm.CanonicalCount, vm.SymbolCount, o.MinCanonicalRatio*100)
		}
	} else if emit != nil {
		// The gate is advertised but disabled at ratio 0; surface that so an
		// operator does not assume canonical coverage was checked.
		emit(Event{Step: "reindex-gate", Type: "warning",
			Message: "canonical_id coverage gate disabled (min_canonical_ratio=0) — join density not verified"})
	}

	// 5. File-set audit (soft): a mismatch is surfaced, not fatal.
	if o.Src != "" {
		if err := r.Run(ctx, Step{
			ID: "reindex-gate", Title: "gate: ckg audit",
			Cmd: []string{graphBin, "audit", "--src", o.Src, "--graph", graphDir},
		}, emit); err != nil {
			warn(fmt.Sprintf("ckg audit reported issues (soft gate, not blocking promote): %v", err))
		}
	}
	if len(o.TestCommand) > 0 {
		if err := runTestGate(ctx, vdir, o.Src, o.TestCommand); err != nil {
			return fmt.Errorf("gate: test command failed: %w", err)
		}
	}
	if o.ExpectedSourceCommit != "" {
		var graph graphManifest
		if err := readJSON(filepath.Join(graphDir, "manifest.json"), &graph); err != nil {
			return fmt.Errorf("gate: read committed source coordinate: %w", err)
		}
		if graph.SrcCommit != o.ExpectedSourceCommit {
			return fmt.Errorf("gate: candidate source commit differs from the pre-build commit")
		}
		clean, err := testGateSourceClean(o.Src, o.ExpectedSourceCommit)
		if err != nil {
			return fmt.Errorf("gate: verify committed source: %w", err)
		}
		if !clean {
			return fmt.Errorf("gate: source changed during build; current left unchanged")
		}
	}
	if o.ExpectedSourceSnapshot.SnapshotID != "" {
		current, err := CommittedSourceIdentity(o.Src, o.ExpectedSourceSnapshot.ProjectID, o.ExpectedSourceSnapshot.SourceCommit)
		if err != nil || current != o.ExpectedSourceSnapshot {
			return fmt.Errorf("gate: source snapshot changed during build; current left unchanged: %v", err)
		}
	}
	return nil
}

// Reindex runs one coordinated blue-green cycle: acquire the dataset lock,
// build a new version directory, gate it, and — only on success — promote it.
// o.Out is the dataset root; the new build lands in o.Out/<version>. On gate
// failure the version dir is kept for diagnosis and `current` is left untouched.
func Reindex(ctx context.Context, o Options, version string, gopt GateOptions, r Runner, emit func(Event)) error {
	if err := validateVersion(version); err != nil {
		return err
	}
	dataset := o.Out
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		return err
	}
	defer lock.release()

	// Build into the version directory (o.Out/<version>/{graph,vector}).
	vo := o
	vo.Out = filepath.Join(dataset, version)
	var captured CapturedSource
	var buildCleanup func() error
	defer func() {
		if buildCleanup != nil {
			_ = buildCleanup()
		}
	}()
	if gopt.ExpectedSourceSnapshot.SnapshotID != "" {
		if _, err := os.Lstat(vo.Out); err == nil {
			return fmt.Errorf("reindex: pinned version %q already exists; choose a new version", version)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("reindex: inspect version %q: %w", version, err)
		}
		captured, err = CaptureSource(CaptureOptions{Root: o.Src, Out: vo.Out,
			ProjectID: o.ProjectID, SourceMode: "committed", SourceCommit: gopt.ExpectedSourceSnapshot.SourceCommit})
		if err != nil {
			return fmt.Errorf("reindex: retain source bytes: %w", err)
		}
		if captured.Identity != gopt.ExpectedSourceSnapshot {
			return fmt.Errorf("reindex: captured source differs from pre-build identity")
		}
		buildRoot := filepath.Join(vo.Out, ".build-source")
		buildCleanup, err = captured.MaterializeBuildTree(buildRoot)
		if err != nil {
			return fmt.Errorf("reindex: materialize captured source: %w", err)
		}
		if err := captured.VerifyAgainst(buildRoot); err != nil {
			return fmt.Errorf("reindex: staged source differs from capture: %w", err)
		}
		vo.Src = buildRoot
		vo.LogicalSrcRoot = o.Src
	}
	plan, err := BuildPlan(vo)
	if err != nil {
		return fmt.Errorf("reindex: plan: %w", err)
	}
	if err := Execute(ctx, plan, r, emit); err != nil {
		return fmt.Errorf("reindex: build version %s: %w", version, err)
	}
	if captured.Identity.SnapshotID != "" {
		if err := captured.VerifyBlobs(); err != nil {
			return fmt.Errorf("reindex: %w", err)
		}
		if err := captured.VerifyAgainst(vo.Src); err != nil {
			return fmt.Errorf("reindex: staged source changed during build: %w", err)
		}
		if err := VerifyEngineInputs(vo.Out, captured); err != nil {
			return fmt.Errorf("reindex: %w", err)
		}
	}
	if gopt.ExpectedSourceSnapshot.SnapshotID != "" {
		current, err := CommittedSourceIdentity(o.Src, o.ProjectID, gopt.ExpectedSourceSnapshot.SourceCommit)
		if err != nil || current != gopt.ExpectedSourceSnapshot {
			return fmt.Errorf("reindex: source changed during build: %v", err)
		}
		inputs, err := ConfiguredInputDigest(o)
		if err != nil || inputs != gopt.ExpectedInputDigest {
			return fmt.Errorf("reindex: build inputs changed during build: %v", err)
		}
		identity, err := PublishCandidateIdentity(vo.Out, current, inputs)
		if err != nil {
			return fmt.Errorf("reindex: publish candidate identity: %w", err)
		}
		if identity.DatasetID != gopt.ExpectedDatasetID {
			return fmt.Errorf("reindex: embedding identity changed during build; current left unchanged")
		}
	}

	if gopt.GraphBin == "" {
		gopt.GraphBin = o.GraphBin
	}
	if gopt.Src == "" {
		gopt.Src = o.Src
	}
	if err := Gate(ctx, dataset, version, gopt, r, emit); err != nil {
		return fmt.Errorf("reindex: %w (current left unchanged; version %s kept for diagnosis)", err, version)
	}
	if captured.Identity.SnapshotID != "" {
		if err := captured.VerifyBlobs(); err != nil {
			return fmt.Errorf("reindex: %w (current left unchanged)", err)
		}
		if err := captured.VerifyAgainst(vo.Src); err != nil {
			return fmt.Errorf("reindex: staged source changed before promotion: %w", err)
		}
	}
	if gopt.ExpectedSourceSnapshot.SnapshotID != "" {
		inputs, err := ConfiguredInputDigest(o)
		if err != nil || inputs != gopt.ExpectedInputDigest {
			return fmt.Errorf("reindex: build inputs changed before promotion; current left unchanged: %v", err)
		}
	}
	if buildCleanup != nil {
		if err := buildCleanup(); err != nil {
			return fmt.Errorf("reindex: remove temporary build tree: %w", err)
		}
		buildCleanup = nil
	}

	prev, err := Promote(dataset, version)
	if err != nil {
		return err
	}
	if emit != nil {
		msg := fmt.Sprintf("promoted %s → current", version)
		if prev != "" {
			msg += fmt.Sprintf(" (was %s; rollback: knowledge-setup --rollback %s)", prev, prev)
		}
		emit(Event{Time: time.Now().UTC(), Step: "reindex-promote", Type: "done", Message: msg})
	}
	return nil
}
