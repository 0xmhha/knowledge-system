package semantic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

// TestRun records a command against a reviewed trace and a clean committed
// source tree. CommandPassed never asserts that a specific criterion passed:
// the runner does not interpret framework output or modify semantic status.
type TestRun struct {
	Snapshot           Snapshot  `json:"snapshot"`
	CriterionID        string    `json:"criterion_id"`
	TestCanonicalID    string    `json:"test_canonical_id"`
	TestedByAssertions []string  `json:"tested_by_assertions"`
	AcceptedAssertions []string  `json:"accepted_by_assertions"`
	Executable         string    `json:"executable"`
	CommandSHA256      string    `json:"command_sha256"`
	StartedAt          time.Time `json:"started_at"`
	DurationMillis     int64     `json:"duration_ms"`
	GOOS               string    `json:"goos"`
	GOARCH             string    `json:"goarch"`
	GoRuntime          string    `json:"go_runtime"`
	ExitCode           int       `json:"exit_code"`
	CommandPassed      bool      `json:"command_passed"`
	SnapshotConsistent bool      `json:"snapshot_consistent"`
	OutputSHA256       string    `json:"output_sha256"`
	OutputBytes        int64     `json:"output_bytes"`
	RunError           string    `json:"run_error,omitempty"`
}

type hashOutput struct {
	mu   sync.Mutex
	hash hash.Hash
	n    int64
}

func (w *hashOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.hash.Write(p)
	w.n += int64(n)
	return n, err
}

// ExecuteLinkedTest runs argv directly, without a shell, after checking a
// reviewed path and source cleanliness. It never promotes a dataset. A failed
// command is returned as a report so the caller can persist the failure.
func (a ActiveProjection) ExecuteLinkedTest(ctx context.Context, repoRoot, criterionID string, argv []string) (TestRun, error) {
	return a.ExecuteLinkedTestFor(ctx, repoRoot, criterionID, "", argv)
}

// ExecuteLinkedTestFor binds a command record to one reviewed test symbol.
// An omitted symbol is accepted only when the criterion has exactly one
// reviewed test target. The command's success still is not criterion proof.
func (a ActiveProjection) ExecuteLinkedTestFor(ctx context.Context, repoRoot, criterionID, testCanonicalID string, argv []string) (TestRun, error) {
	if len(argv) == 0 || argv[0] == "" {
		return TestRun{}, fmt.Errorf("test run needs a command after --")
	}
	trace, err := a.Trace()
	if err != nil {
		return TestRun{}, err
	}
	linkedTests := map[string]map[string]bool{}
	for _, requirement := range trace.Requirements {
		if requirement.State != TraceLinked {
			continue
		}
		for _, path := range requirement.Paths {
			for _, id := range path.AcceptedCriterionIDs {
				if id == criterionID {
					if linkedTests[path.TestCanonicalID] == nil {
						linkedTests[path.TestCanonicalID] = map[string]bool{}
					}
					linkedTests[path.TestCanonicalID][path.TestedByAssertion] = true
				}
			}
		}
	}
	if len(linkedTests) == 0 {
		return TestRun{}, fmt.Errorf("criterion %q has no fully reviewed code/test path", criterionID)
	}
	if testCanonicalID == "" {
		if len(linkedTests) != 1 {
			return TestRun{}, fmt.Errorf("criterion %q has %d reviewed tests; select --test-canonical-id", criterionID, len(linkedTests))
		}
		for id := range linkedTests {
			testCanonicalID = id
		}
	}
	assertions, ok := linkedTests[testCanonicalID]
	if !ok {
		return TestRun{}, fmt.Errorf("test %q is not a reviewed target for criterion %q", testCanonicalID, criterionID)
	}
	testedBy := make([]string, 0, len(assertions))
	for id := range assertions {
		testedBy = append(testedBy, id)
	}
	sort.Strings(testedBy)
	var acceptedBy []string
	for _, edge := range a.projection.Assertions {
		if edge.Status == StatusVerified && edge.Predicate == PredicateAcceptedBy &&
			edge.SubjectID == criterionID && edge.ObjectID == testCanonicalID {
			acceptedBy = append(acceptedBy, edge.ID)
		}
	}
	sort.Strings(acceptedBy)
	if len(acceptedBy) == 0 {
		return TestRun{}, fmt.Errorf("criterion %q has no reviewed acceptance assertion for test %q", criterionID, testCanonicalID)
	}
	clean, err := committedTreeClean(repoRoot, trace.Snapshot.Commit)
	if err != nil {
		return TestRun{}, err
	}
	if !clean {
		return TestRun{}, fmt.Errorf("test run requires a clean committed source tree")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return TestRun{}, err
	}
	commandBytes, _ := json.Marshal(argv)
	commandHash := sha256.Sum256(commandBytes)
	report := TestRun{Snapshot: trace.Snapshot, CriterionID: criterionID,
		TestCanonicalID: testCanonicalID, TestedByAssertions: testedBy, AcceptedAssertions: acceptedBy,
		Executable: filepath.Base(argv[0]), CommandSHA256: hex.EncodeToString(commandHash[:]),
		StartedAt: time.Now().UTC(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		GoRuntime: runtime.Version(), ExitCode: -1}
	output := &hashOutput{hash: sha256.New()}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = root, output, output
	start := time.Now()
	runErr := cmd.Run()
	report.DurationMillis = time.Since(start).Milliseconds()
	report.OutputSHA256 = hex.EncodeToString(output.hash.Sum(nil))
	report.OutputBytes = output.n
	if runErr == nil {
		report.ExitCode, report.CommandPassed = 0, true
	} else {
		if exit, ok := runErr.(*exec.ExitError); ok {
			report.ExitCode = exit.ExitCode()
		}
		report.RunError = runErr.Error()
	}
	report.SnapshotConsistent, err = committedTreeClean(repoRoot, trace.Snapshot.Commit)
	if err != nil {
		report.RunError = fmt.Sprintf("post-run source check: %v", err)
		report.SnapshotConsistent = false
	}
	return report, nil
}

func committedTreeClean(repoRoot, expectedCommit string) (bool, error) {
	head, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return false, fmt.Errorf("inspect test HEAD: %w", err)
	}
	if string(bytes.TrimSpace(head)) != expectedCommit {
		return false, nil
	}
	cmd := exec.Command("git", "-C", repoRoot, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("inspect test source: %w", err)
	}
	if len(out) > 0 {
		return false, nil
	}
	ignored, err := setup.CountIgnoredIndexable(repoRoot)
	if err != nil {
		return false, err
	}
	return ignored == 0, nil
}
