package setup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type testGateReport struct {
	SourceCommit       string    `json:"source_commit"`
	SourceMode         string    `json:"source_mode,omitempty"`
	SnapshotID         string    `json:"snapshot_id,omitempty"`
	GraphDigest        string    `json:"graph_digest"`
	Executable         string    `json:"executable"`
	CommandSHA256      string    `json:"command_sha256"`
	OutputSHA256       string    `json:"output_sha256"`
	OutputBytes        int64     `json:"output_bytes"`
	StartedAt          time.Time `json:"started_at"`
	DurationMillis     int64     `json:"duration_ms"`
	GOOS               string    `json:"goos"`
	GOARCH             string    `json:"goarch"`
	GoRuntime          string    `json:"go_runtime"`
	ExitCode           int       `json:"exit_code"`
	CommandPassed      bool      `json:"command_passed"`
	SnapshotConsistent bool      `json:"snapshot_consistent"`
	RunError           string    `json:"run_error,omitempty"`
}

type gateOutputHash struct {
	mu   sync.Mutex
	hash hash.Hash
	n    int64
}

func (w *gateOutputHash) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.hash.Write(data)
	w.n += int64(n)
	return n, err
}

// runTestGate records one explicit command against the candidate's committed
// source. It has no shell, stores no output text, and cannot promote a version.
func runTestGate(ctx context.Context, candidateDir, sourceRoot string, argv []string, pinned ...SourceIdentity) error {
	return runTestGateWithOrigins(ctx, candidateDir, sourceRoot, argv, nil, pinned...)
}

func runTestGateWithOrigins(ctx context.Context, candidateDir, sourceRoot string, argv []string, externalOrigins []CaptureOrigin, pinned ...SourceIdentity) error {
	return runTestGateWithLimits(ctx, candidateDir, sourceRoot, argv, externalOrigins, CaptureLimits{}, pinned...)
}

func runTestGateWithLimits(ctx context.Context, candidateDir, sourceRoot string, argv []string, externalOrigins []CaptureOrigin, limits CaptureLimits, pinned ...SourceIdentity) error {
	if sourceRoot == "" || len(argv) == 0 || argv[0] == "" {
		return fmt.Errorf("source and test executable are required")
	}
	var graph struct {
		SrcCommit   string `json:"src_commit"`
		GraphDigest string `json:"graph_digest"`
	}
	if err := readJSON(filepath.Join(candidateDir, "graph", "manifest.json"), &graph); err != nil {
		return err
	}
	if len(graph.SrcCommit) != 40 && !(len(pinned) == 1 && pinned[0].SourceMode == "snapshot-only" && graph.SrcCommit == "") {
		return fmt.Errorf("candidate graph has no full source commit")
	}
	check := func() (bool, error) { return testGateSourceCleanContext(ctx, sourceRoot, graph.SrcCommit) }
	if len(pinned) == 1 && pinned[0].SourceMode != "committed" {
		check = func() (bool, error) {
			current, err := SnapshotSourceIdentityWithLimits(ctx, sourceRoot, pinned[0].ProjectID,
				pinned[0].SourceMode, pinned[0].SourceCommit, limits, externalOrigins...)
			return err == nil && current == pinned[0], err
		}
	}
	clean, err := check()
	if err != nil {
		return err
	}
	if !clean {
		return fmt.Errorf("test gate requires the candidate's unchanged source snapshot")
	}
	root, err := filepath.Abs(sourceRoot)
	if err != nil {
		return err
	}
	commandJSON, _ := json.Marshal(argv)
	commandHash := sha256.Sum256(commandJSON)
	report := testGateReport{SourceCommit: graph.SrcCommit, GraphDigest: graph.GraphDigest,
		Executable: filepath.Base(argv[0]), CommandSHA256: hex.EncodeToString(commandHash[:]),
		StartedAt: time.Now().UTC(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		GoRuntime: runtime.Version(), ExitCode: -1}
	if len(pinned) == 1 {
		report.SourceMode, report.SnapshotID = pinned[0].SourceMode, pinned[0].SnapshotID
	}
	output := &gateOutputHash{hash: sha256.New()}
	command := buildCommandContext(ctx, argv[0], argv[1:]...)
	command.Dir, command.Stdout, command.Stderr = root, output, output
	start := time.Now()
	runErr := command.Run()
	report.DurationMillis = time.Since(start).Milliseconds()
	report.OutputSHA256, report.OutputBytes = hex.EncodeToString(output.hash.Sum(nil)), output.n
	if runErr == nil {
		report.CommandPassed, report.ExitCode = true, 0
	} else {
		if exit, ok := runErr.(*exec.ExitError); ok {
			report.ExitCode = exit.ExitCode()
		}
		report.RunError = runErr.Error()
	}
	report.SnapshotConsistent, err = check()
	if err != nil {
		report.SnapshotConsistent = false
		report.RunError = fmt.Sprintf("post-run source check: %v", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(candidateDir, "test-gate.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create candidate test report: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if !report.CommandPassed || !report.SnapshotConsistent {
		return fmt.Errorf("candidate test did not pass on a consistent source; see %s", filepath.Join(candidateDir, "test-gate.json"))
	}
	return nil
}

func testGateSourceClean(root, expectedCommit string) (bool, error) {
	return testGateSourceCleanContext(context.Background(), root, expectedCommit)
}

func testGateSourceCleanContext(ctx context.Context, root, expectedCommit string) (bool, error) {
	head, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return false, fmt.Errorf("inspect candidate HEAD: %w", err)
	}
	if string(bytes.TrimSpace(head)) != expectedCommit {
		return false, nil
	}
	status, err := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return false, fmt.Errorf("inspect candidate source: %w", err)
	}
	if len(status) > 0 {
		return false, nil
	}
	ignored, err := CountIgnoredIndexableContext(ctx, root)
	if err != nil {
		return false, err
	}
	return ignored == 0, nil
}
