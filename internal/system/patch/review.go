package patch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

func reviewDir(dataset, patchID string) (string, error) {
	path, err := registryPath(dataset, patchID)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(path, ".json"), nil
}

func criterionInProjection(p semantic.Projection, id string) (semantic.Requirement, semantic.AcceptanceCriterion, error) {
	for _, r := range p.Requirements {
		for _, c := range r.AcceptanceCriteria {
			if c.ID == id {
				return r, c, nil
			}
		}
	}
	return semantic.Requirement{}, semantic.AcceptanceCriterion{}, fmt.Errorf("criterion %q is absent from candidate specification", id)
}

func loadRun(dataset, patchID, criterionID string) (semantic.TestRun, error) {
	dir, err := reviewDir(dataset, patchID)
	if err != nil {
		return semantic.TestRun{}, err
	}
	if !safeID.MatchString(criterionID) {
		return semantic.TestRun{}, fmt.Errorf("invalid criterion ID")
	}
	var run semantic.TestRun
	if err := readJSON(filepath.Join(dir, "runs", criterionID+".json"), &run); err != nil {
		return semantic.TestRun{}, err
	}
	return run, nil
}

func validateRun(p semantic.Projection, criterionID string, run semantic.TestRun) error {
	if run.Snapshot != p.Snapshot || run.CriterionID != criterionID || run.TestCanonicalID == "" ||
		!run.CommandPassed || !run.SnapshotConsistent || run.ExitCode != 0 || len(run.CheckedAssertions) == 0 ||
		len(run.AcceptedAssertions) != 0 || run.CommandSHA256 == "" || run.OutputSHA256 == "" {
		return fmt.Errorf("test execution is absent, failed, stale, or not bound to a reviewed CHECKED_BY link")
	}
	if run.Framework == "go-test-json" && (!run.TestObserved || !run.TestPassed) {
		return fmt.Errorf("selected Go test did not run and pass")
	}
	if _, _, err := criterionInProjection(p, criterionID); err != nil {
		return err
	}
	linked := false
	for _, assertion := range p.Assertions {
		if assertion.Predicate == semantic.PredicateCheckedBy && assertion.Status == semantic.StatusVerified &&
			assertion.SubjectID == criterionID && assertion.ObjectID == run.TestCanonicalID {
			for _, id := range run.CheckedAssertions {
				if id == assertion.ID {
					linked = true
				}
			}
		}
	}
	if !linked {
		return fmt.Errorf("test execution does not match a reviewed criterion link")
	}
	return nil
}

// RecordRun imports one immutable cks semantic test report. A command exit 0
// is retained as execution evidence only; it never approves a criterion.
func RecordRun(ctx context.Context, dataset, patchID, storePath, reportPath string) (semantic.TestRun, error) {
	a, err := Load(dataset, patchID)
	if err != nil {
		return semantic.TestRun{}, err
	}
	p, err := CandidateProjection(ctx, dataset, a, storePath)
	if err != nil {
		return semantic.TestRun{}, err
	}
	var run semantic.TestRun
	if err := readJSON(reportPath, &run); err != nil {
		return semantic.TestRun{}, err
	}
	if err := validateRun(p, run.CriterionID, run); err != nil {
		return semantic.TestRun{}, err
	}
	dir, _ := reviewDir(dataset, patchID)
	if err := writeImmutable(filepath.Join(dir, "runs", run.CriterionID+".json"), run); err != nil {
		return semantic.TestRun{}, err
	}
	return run, nil
}

// Decide records one person's criterion judgment against the exact archived
// spec evidence. Approved requires a separately recorded passing test run.
func Decide(ctx context.Context, dataset, patchID, storePath, decisionID, criterionID, reviewer, outcome, reason string) (Decision, error) {
	if !safeID.MatchString(decisionID) || !safeID.MatchString(criterionID) ||
		strings.TrimSpace(reviewer) == "" || strings.TrimSpace(reason) == "" ||
		(outcome != "approved" && outcome != "rejected") {
		return Decision{}, fmt.Errorf("invalid criterion review fields")
	}
	a, err := Load(dataset, patchID)
	if err != nil {
		return Decision{}, err
	}
	p, err := CandidateProjection(ctx, dataset, a, storePath)
	if err != nil {
		return Decision{}, err
	}
	r, c, err := criterionInProjection(p, criterionID)
	if err != nil {
		return Decision{}, err
	}
	if outcome == "approved" {
		run, err := loadRun(dataset, patchID, criterionID)
		if err != nil {
			return Decision{}, err
		}
		if err := validateRun(p, criterionID, run); err != nil {
			return Decision{}, err
		}
	}
	var evidenceSHA string
	for _, e := range p.Evidence {
		if e.ID == c.EvidenceID {
			evidenceSHA = e.ContentSHA256
			break
		}
	}
	if evidenceSHA == "" {
		return Decision{}, fmt.Errorf("criterion source evidence is missing")
	}
	d := Decision{DecisionID: decisionID, PatchID: patchID, CriterionID: criterionID, SpecVersion: r.Version,
		ProjectID: a.ProjectID, DatasetID: a.ResultDatasetID, SnapshotID: a.ResultSnapshotID,
		EvidenceSHA256: evidenceSHA, ReviewedBy: reviewer, Outcome: outcome, Reason: reason, CreatedAt: time.Now().UTC()}
	dir, _ := reviewDir(dataset, patchID)
	if err := writeImmutable(filepath.Join(dir, "decisions", decisionID+".json"), d); err != nil {
		return Decision{}, err
	}
	return d, nil
}

func readDecisions(dataset, patchID string) ([]Decision, error) {
	dir, err := reviewDir(dataset, patchID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(dir, "decisions"))
	if err != nil {
		return nil, err
	}
	result := make([]Decision, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || !safeID.MatchString(strings.TrimSuffix(entry.Name(), ".json")) {
			return nil, fmt.Errorf("invalid patch decision entry")
		}
		var d Decision
		if err := readJSON(filepath.Join(dir, "decisions", entry.Name()), &d); err != nil {
			return nil, err
		}
		if d.DecisionID+".json" != entry.Name() {
			return nil, fmt.Errorf("patch decision ID differs from file name")
		}
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DecisionID < result[j].DecisionID })
	return result, nil
}

func verifyTestGate(versionDir string, a Attempt) error {
	var report struct {
		SnapshotID         string `json:"snapshot_id"`
		CommandPassed      bool   `json:"command_passed"`
		SnapshotConsistent bool   `json:"snapshot_consistent"`
		ExitCode           int    `json:"exit_code"`
		CommandSHA256      string `json:"command_sha256"`
		OutputSHA256       string `json:"output_sha256"`
	}
	if err := readJSONLoose(filepath.Join(versionDir, "test-gate.json"), &report); err != nil {
		return err
	}
	if report.SnapshotID != a.ResultSnapshotID || !report.CommandPassed || !report.SnapshotConsistent || report.ExitCode != 0 ||
		len(report.CommandSHA256) != 64 || len(report.OutputSHA256) != 64 {
		return fmt.Errorf("candidate test gate did not pass on the patch snapshot")
	}
	return nil
}

// Promote requires an unchanged active base, a passing candidate test gate,
// a current semantic trace and a separate approved human decision plus test
// execution for every criterion. Any rejection holds the candidate.
func Promote(ctx context.Context, dataset, patchID, storePath string) (string, error) {
	a, err := Load(dataset, patchID)
	if err != nil {
		return "", err
	}
	current, err := os.Readlink(filepath.Join(dataset, "current"))
	if err != nil || current != a.BaseVersion {
		return "", fmt.Errorf("patch base is no longer current")
	}
	vdir := filepath.Join(dataset, a.ResultVersion)
	if err := verifyTestGate(vdir, a); err != nil {
		return "", err
	}
	p, err := CandidateProjection(ctx, dataset, a, storePath)
	if err != nil {
		return "", err
	}
	trace, err := p.Trace()
	if err != nil {
		return "", err
	}
	if len(trace.Requirements) == 0 {
		return "", fmt.Errorf("patch candidate has no reviewed requirements")
	}
	for _, r := range trace.Requirements {
		if r.State != semantic.TraceLinked {
			return "", fmt.Errorf("requirement %q is %s, not linked", r.RequirementID, r.State)
		}
	}
	decisions, err := readDecisions(dataset, patchID)
	if err != nil {
		return "", err
	}
	byCriterion := map[string]Decision{}
	for _, d := range decisions {
		if d.PatchID != patchID || d.ProjectID != a.ProjectID || d.DatasetID != a.ResultDatasetID || d.SnapshotID != a.ResultSnapshotID ||
			byCriterion[d.CriterionID].DecisionID != "" || d.Outcome != "approved" || d.ReviewedBy == "" || d.Reason == "" {
			return "", fmt.Errorf("patch has rejected, duplicate, or foreign criterion decision")
		}
		byCriterion[d.CriterionID] = d
	}
	criteria := 0
	for _, r := range p.Requirements {
		for _, c := range r.AcceptanceCriteria {
			criteria++
			d, ok := byCriterion[c.ID]
			if !ok || d.SpecVersion != r.Version {
				return "", fmt.Errorf("criterion %q lacks current human approval", c.ID)
			}
			evidenceSHA := ""
			for _, e := range p.Evidence {
				if e.ID == c.EvidenceID {
					evidenceSHA = e.ContentSHA256
					break
				}
			}
			if evidenceSHA == "" || d.EvidenceSHA256 != evidenceSHA {
				return "", fmt.Errorf("criterion %q approval evidence differs", c.ID)
			}
			run, err := loadRun(dataset, patchID, c.ID)
			if err != nil {
				return "", err
			}
			if err := validateRun(p, c.ID, run); err != nil {
				return "", err
			}
		}
	}
	if len(byCriterion) != criteria {
		return "", fmt.Errorf("patch has decisions for foreign criteria")
	}
	data, err := json.Marshal(decisions)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return setup.PromoteReviewedCandidate(dataset, a.ResultVersion, patchID, hex.EncodeToString(sum[:]))
}
