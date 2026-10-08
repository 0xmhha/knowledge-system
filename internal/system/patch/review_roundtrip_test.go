package patch

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	graphstore "github.com/0xmhha/knowledge-system/pkg/graph/store"
)

// All reviewed records below are owned integration controls, not human
// judgments. CKG is built through the public CLI; CKV is a coordinate-only
// fixture with no declared chunk links, embeddings or retrieval claims.
func TestExactRunnerReportRoundTripDoesNotApproveOrPromote(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	moduleRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	ckg := filepath.Join(t.TempDir(), "ckg")
	roundtripCommand(t, ctx, moduleRoot, "go", "build", "-o", ckg, "./cmd/graph")
	for _, pkg := range []string{"", "nested"} {
		name := pkg
		if name == "" {
			name = "root"
		}
		t.Run(name, func(t *testing.T) {
			source, dataset, records := t.TempDir(), t.TempDir(), t.TempDir()
			write := func(path, content string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(source, "go.mod"), "module example.test/proof\n\ngo 1.25.0\n")
			codePath, testPath := filepath.Join(pkg, "main.go"), filepath.Join(pkg, "main_test.go")
			write(filepath.Join(source, codePath), "package proof\nfunc Alpha() int { return 0 }\n")
			write(filepath.Join(source, testPath), "package proof\nimport \"testing\"\nfunc TestAlpha(t *testing.T) { if Alpha() != 1 { t.Fatal(\"Alpha must return one\") } }\n")
			ontology := "version: 1\nproject_id: roundtrip-control\ndomain: integration control\ncompetency_questions: [\"Does Alpha return one?\"]\nconcepts:\n  - id: feature\n    kind: rule\n    definition: Alpha returns one.\n    includes: [Alpha result]\n    excludes: [Other behavior]\n    terms: [{lang: en, value: feature, preferred: true}]\n    status: verified\n    reviewed_by: unit-fixture\n"
			spec := "version: 1\nproject_id: roundtrip-control\nrequirements:\n  - id: R\n    version: 1\n    title: Alpha control\n    statement: Alpha returns one.\n    concept_ids: [feature]\n    status: verified\n    reviewed_by: unit-fixture\n    acceptance_criteria:\n      - id: C\n        given: Alpha\n        when: Called\n        then: Returns one\n"
			write(filepath.Join(source, "ontology.yaml"), ontology)
			write(filepath.Join(source, "spec.yaml"), spec)
			roundtripCommand(t, ctx, source, "git", "init", "-q")
			commit := func() string {
				t.Helper()
				roundtripCommand(t, ctx, source, "git", "add", ".")
				roundtripCommand(t, ctx, source, "git", "-c", "user.name=Unit Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "-qm", "owned control")
				return strings.TrimSpace(roundtripCommand(t, ctx, source, "git", "rev-parse", "HEAD"))
			}
			roundtripVersion(t, ctx, ckg, source, dataset, "base", commit())
			if err := os.Symlink("base", filepath.Join(dataset, "current")); err != nil {
				t.Fatal(err)
			}
			write(filepath.Join(source, codePath), "package proof\nfunc Alpha() int { return 1 }\n")
			identity := roundtripVersion(t, ctx, ckg, source, dataset, "candidate", commit())
			vdir := filepath.Join(dataset, "candidate")
			roundtripJSON(t, filepath.Join(vdir, "review-hold.json"), hold{ProjectID: identity.Source.ProjectID, DatasetID: identity.DatasetID, SnapshotID: identity.Source.SnapshotID, BaseVersion: "base"})
			attempt, err := Register(dataset, "candidate", "control-change")
			if err != nil || len(attempt.ChangedFiles) != 1 || attempt.ChangedFiles[0].Path != filepath.ToSlash(codePath) {
				t.Fatalf("register exact source change: %+v %v", attempt, err)
			}
			snapshot := semantic.Snapshot{ProjectID: identity.Source.ProjectID, DatasetID: identity.DatasetID, SnapshotID: identity.Source.SnapshotID, SourceMode: "committed", Commit: identity.Source.SourceCommit}
			p, _, err := semantic.ExtractOntology(snapshot, "ontology.yaml", []byte(ontology))
			if err != nil {
				t.Fatal(err)
			}
			sp, _, err := semantic.ExtractSpec(snapshot, "spec.yaml", []byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			p.Requirements = sp.Requirements
			p.Evidence = append(p.Evidence, sp.Evidence...)
			canonicalPrefix := "example.test/proof"
			if pkg != "" {
				canonicalPrefix += "/" + pkg
			}
			codeID, testID := canonicalPrefix+".Alpha", canonicalPrefix+".TestAlpha"
			graphDir, vectorDir := filepath.Join(vdir, "graph"), filepath.Join(vdir, "vector")
			reader, err := graphstore.OpenReadOnly(filepath.Join(graphDir, "graph.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			for _, anchor := range []struct {
				id, canonical string
				kind          semantic.SourceKind
			}{{"code", codeID, semantic.SourceCode}, {"test", testID, semantic.SourceTest}} {
				n, found, err := reader.FindByCanonicalID(anchor.canonical)
				if err != nil || !found {
					t.Fatalf("real CKG anchor %q: found=%v err=%v", anchor.canonical, found, err)
				}
				body, err := os.ReadFile(filepath.Join(source, n.FilePath))
				if err != nil {
					t.Fatal(err)
				}
				lines := strings.SplitAfter(string(body), "\n")
				digest := sha256.Sum256([]byte(strings.Join(lines[n.StartLine-1:n.EndLine], "")))
				p.Evidence = append(p.Evidence, semantic.EvidenceSpan{ID: anchor.id, Snapshot: snapshot, Kind: anchor.kind, Path: n.FilePath, StartLine: n.StartLine, EndLine: n.EndLine, ContentSHA256: hex.EncodeToString(digest[:]), CanonicalID: n.CanonicalID, Extractor: "ckg-ast-v1"})
			}
			edge := func(id, predicate, subject, object string, evidence ...string) semantic.Assertion {
				return semantic.Assertion{ID: id, Predicate: predicate, SubjectID: subject, ObjectID: object, EvidenceIDs: evidence, Status: semantic.StatusVerified, ReviewedBy: "unit-fixture"}
			}
			p.Assertions = []semantic.Assertion{
				edge("impl", semantic.PredicateImplementedBy, "feature", codeID, p.Concepts[0].EvidenceID, "code"),
				edge("tested-1", semantic.PredicateTestedBy, codeID, testID, "code", "test"),
				edge("tested-2", semantic.PredicateTestedBy, codeID, testID, "code", "test"),
				edge("checked-1", semantic.PredicateCheckedBy, "C", testID, p.Requirements[0].AcceptanceCriteria[0].EvidenceID, "test"),
				edge("checked-2", semantic.PredicateCheckedBy, "C", testID, p.Requirements[0].AcceptanceCriteria[0].EvidenceID, "test"),
			}
			storePath := filepath.Join(records, "semantic.sqlite")
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if err := store.PutAlignedRetained(ctx, p, source, graphDir, vectorDir, vdir); err != nil {
				t.Fatal(err)
			}
			active, err := store.LoadAlignedRetained(ctx, snapshot.ProjectID, snapshot.DatasetID, source, graphDir, vectorDir, vdir)
			if err != nil {
				t.Fatal(err)
			}
			run, err := active.ExecuteLinkedGoTestFor(ctx, source, "C", testID)
			if err != nil || !run.TestObserved || !run.TestPassed || !run.CommandPassed || !run.SnapshotConsistent || run.OutputBytes == 0 || run.TestName != "TestAlpha" || len(run.TestedByAssertions) != 2 || len(run.CheckedAssertions) != 2 {
				t.Fatalf("production exact runner: %+v %v", run, err)
			}
			if _, err := store.Current(ctx, snapshot.ProjectID); err != sql.ErrNoRows {
				t.Fatalf("runner activated semantic current: %v", err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			report := filepath.Join(records, "report.json")
			bad := run
			bad.TestedByAssertions = []string{"foreign", "tested-2"}
			roundtripJSON(t, report, bad)
			if _, err := RecordRun(ctx, dataset, attempt.PatchID, storePath, report); err == nil {
				t.Fatal("public report import accepted foreign proof ID")
			}
			runPath := filepath.Join(dataset, ".patches", attempt.PatchID, "runs", "C.json")
			if _, err := os.Stat(runPath); !os.IsNotExist(err) {
				t.Fatalf("rejected import wrote a run: %v", err)
			}
			roundtripJSON(t, report, run)
			for i := 0; i < 2; i++ {
				imported, err := RecordRun(ctx, dataset, attempt.PatchID, storePath, report)
				if err != nil || !reflect.DeepEqual(imported, run) {
					t.Fatalf("exact report import/retry: %+v %v", imported, err)
				}
			}
			persisted, err := loadRun(dataset, attempt.PatchID, "C")
			if err != nil || !reflect.DeepEqual(persisted, run) {
				t.Fatalf("persisted report differs: %+v %v", persisted, err)
			}
			before, err := os.ReadFile(runPath)
			if err != nil {
				t.Fatal(err)
			}
			bad = run
			bad.OutputSHA256 = strings.Repeat("0", 64)
			if err := validateRun(p, "C", bad); err != nil {
				t.Fatalf("immutable replacement control must pass metadata checks: %v", err)
			}
			roundtripJSON(t, report, bad)
			if _, err := RecordRun(ctx, dataset, attempt.PatchID, storePath, report); err == nil {
				t.Fatal("public report import overwrote immutable run")
			}
			after, err := os.ReadFile(runPath)
			if err != nil || string(before) != string(after) {
				t.Fatalf("rejected replacement changed run bytes: %v", err)
			}
			if current, err := os.Readlink(filepath.Join(dataset, "current")); err != nil || current != "base" {
				t.Fatalf("report import changed current: %q %v", current, err)
			}
			loaded, err := Load(dataset, attempt.PatchID)
			if err != nil || loaded.State != "unconfirmed" {
				t.Fatalf("report import accepted patch: %+v %v", loaded, err)
			}
			if _, err := os.Stat(filepath.Join(dataset, ".patches", attempt.PatchID, "decisions")); !os.IsNotExist(err) {
				t.Fatalf("report import fabricated decisions: %v", err)
			}
			for _, path := range []string{filepath.Join(vdir, "review-release.json"), filepath.Join(vdir, "test-gate.json")} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("unexpected promotion marker %s: %v", path, err)
				}
			}
			check, err := semantic.OpenStore(storePath)
			if err != nil {
				t.Fatal(err)
			}
			defer check.Close()
			if _, err := check.Current(ctx, snapshot.ProjectID); err != sql.ErrNoRows {
				t.Fatalf("import activated semantic current: %v", err)
			}
			t.Logf("owned control: package=%q real CKG canonical=%q exact observed/pass=true tested=2 checked=2 import/retry=true bad/replacement refused=true human decisions=0 current=base semantic current=absent output_sha=%s", pkg, testID, run.OutputSHA256)
		})
	}
}

func roundtripCommand(t *testing.T, ctx context.Context, dir, executable string, argv ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, executable, argv...)
	cmd.Dir = dir
	body, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", executable, argv, err, body)
	}
	return string(body)
}

func roundtripJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func roundtripVersion(t *testing.T, ctx context.Context, ckg, source, dataset, version, commit string) setup.DatasetIdentity {
	t.Helper()
	vdir := filepath.Join(dataset, version)
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: source, Out: vdir, ProjectID: "roundtrip-control", SourceMode: "committed", SourceCommit: commit})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "coordinate-only-control")
	if err != nil {
		t.Fatal(err)
	}
	graphDir := filepath.Join(vdir, "graph")
	roundtripCommand(t, ctx, source, ckg, "build", "--src", source, "--out", graphDir, "--lang", "go", "--no-cache", "--fail-on-parse-errors", "--project-id", captured.Identity.ProjectID, "--snapshot-id", captured.Identity.SnapshotID, "--dataset-id", expected.DatasetID, "--source-mode", "committed", "--file-manifest-digest", captured.Identity.FileManifestDigest, "--capture-policy-digest", captured.Identity.CapturePolicyDigest)
	var graph struct {
		GraphDigest string `json:"graph_digest"`
	}
	if err := readJSONLoose(filepath.Join(graphDir, "manifest.json"), &graph); err != nil {
		t.Fatal(err)
	}
	vectorDir := filepath.Join(vdir, "vector")
	if err := os.Mkdir(vectorDir, 0o700); err != nil {
		t.Fatal(err)
	}
	roundtripJSON(t, filepath.Join(vectorDir, "manifest.json"), map[string]any{
		"src_root": source, "src_commit": commit, "source_mode": "committed",
		"embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space",
		"project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID, "dataset_id": expected.DatasetID,
		"file_manifest_digest": captured.Identity.FileManifestDigest, "capture_policy_digest": captured.Identity.CapturePolicyDigest,
		"sources": map[string]any{"ckg": map[string]any{"graph_digest": graph.GraphDigest, "src_commit": commit}},
	})
	identity, err := setup.PublishCandidateIdentity(vdir, captured.Identity, "coordinate-only-control")
	if err != nil {
		t.Fatal(err)
	}
	return identity
}
