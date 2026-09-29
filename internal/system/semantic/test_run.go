package semantic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"hash"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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
	Framework          string    `json:"framework,omitempty"`
	TestName           string    `json:"test_name,omitempty"`
	TestObserved       bool      `json:"test_observed,omitempty"`
	TestPassed         bool      `json:"test_passed,omitempty"`
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
	return a.executeLinkedTestFor(ctx, repoRoot, criterionID, testCanonicalID, argv, false)
}

// ExecuteLinkedGoTestFor resolves the reviewed CKG test anchor to one Go
// function and requires go test's JSON stream to show that exact test ran
// and passed. This is test execution proof, not a semantic Given/When/Then
// judgment.
func (a ActiveProjection) ExecuteLinkedGoTestFor(ctx context.Context, repoRoot, criterionID, testCanonicalID string) (TestRun, error) {
	return a.executeLinkedTestFor(ctx, repoRoot, criterionID, testCanonicalID, nil, true)
}

func (a ActiveProjection) executeLinkedTestFor(ctx context.Context, repoRoot, criterionID, testCanonicalID string, argv []string, exactGo bool) (TestRun, error) {
	if !exactGo && (len(argv) == 0 || argv[0] == "") {
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
	var testName string
	if exactGo {
		argv, testName, err = a.exactGoTestCommand(repoRoot, trace.Snapshot.Commit, testCanonicalID)
		if err != nil {
			return TestRun{}, err
		}
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
	if exactGo {
		report.Framework, report.TestName = "go-test-json", testName
	}
	output := &hashOutput{hash: sha256.New()}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var observer *goTestObserver
	if exactGo {
		observer = &goTestObserver{output: output, testName: testName}
		cmd.Dir, cmd.Stdout, cmd.Stderr = root, observer, observer
	} else {
		cmd.Dir, cmd.Stdout, cmd.Stderr = root, output, output
	}
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
	if exactGo {
		report.TestObserved, report.TestPassed = observer.run, observer.run && observer.pass && report.CommandPassed
		if report.CommandPassed && !report.TestPassed {
			report.RunError = "go test exited successfully without a pass event for the reviewed test"
		}
	}
	report.SnapshotConsistent, err = committedTreeClean(repoRoot, trace.Snapshot.Commit)
	if err != nil {
		report.RunError = fmt.Sprintf("post-run source check: %v", err)
		report.SnapshotConsistent = false
	}
	return report, nil
}

func (a ActiveProjection) exactGoTestCommand(repoRoot, commit, canonicalID string) ([]string, string, error) {
	var evidence *EvidenceSpan
	for i := range a.projection.Evidence {
		e := &a.projection.Evidence[i]
		if e.Kind == SourceTest && e.CanonicalID == canonicalID {
			if evidence != nil && (e.Path != evidence.Path || e.StartLine != evidence.StartLine || e.EndLine != evidence.EndLine) {
				return nil, "", fmt.Errorf("test %q has ambiguous source anchors", canonicalID)
			}
			evidence = e
		}
	}
	if evidence == nil || !strings.HasSuffix(evidence.Path, "_test.go") {
		return nil, "", fmt.Errorf("test %q has no Go test source anchor", canonicalID)
	}
	source, err := exec.Command("git", "-C", repoRoot, "show", commit+":"+evidence.Path).Output()
	if err != nil {
		return nil, "", fmt.Errorf("read reviewed Go test source: %w", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, evidence.Path, source, 0)
	if err != nil {
		return nil, "", fmt.Errorf("parse reviewed Go test source: %w", err)
	}
	var testName string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !goTestName(fn.Name.Name) ||
			fset.Position(fn.Pos()).Line < evidence.StartLine || fset.Position(fn.End()).Line > evidence.EndLine ||
			!strings.HasSuffix(canonicalID, "."+fn.Name.Name) {
			continue
		}
		if testName != "" {
			return nil, "", fmt.Errorf("test %q source anchor contains multiple Go tests", canonicalID)
		}
		testName = fn.Name.Name
	}
	if testName == "" {
		return nil, "", fmt.Errorf("test %q source anchor does not contain the reviewed Go test function", canonicalID)
	}
	packagePath := "."
	if dir := filepath.Dir(evidence.Path); dir != "." {
		packagePath = "./" + filepath.ToSlash(dir)
	}
	return []string{"go", "test", "-json", "-count=1", "-run", "^" + testName + "$", packagePath}, testName, nil
}

func goTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") || len(name) <= 4 {
		return false
	}
	c := name[4]
	return c < 'a' || c > 'z'
}

// goTestObserver hashes all output but retains only one bounded JSON line.
// A green package exit without the selected test's run/pass events is not
// accepted as exact-test execution proof (e.g. "no tests to run").
type goTestObserver struct {
	mu       sync.Mutex
	output   *hashOutput
	testName string
	pending  []byte
	run      bool
	pass     bool
}

func (o *goTestObserver) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n, err := o.output.Write(p)
	for _, b := range p[:n] {
		if b == '\n' {
			var event struct {
				Action string `json:"Action"`
				Test   string `json:"Test"`
			}
			if json.Unmarshal(o.pending, &event) == nil && event.Test == o.testName {
				if event.Action == "run" {
					o.run = true
				}
				if event.Action == "pass" {
					o.pass = true
				}
			}
			o.pending = o.pending[:0]
		} else if len(o.pending) < 1<<20 {
			o.pending = append(o.pending, b)
		}
	}
	return n, err
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
