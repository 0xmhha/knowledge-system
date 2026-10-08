package semantic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	graphstore "github.com/0xmhha/knowledge-system/pkg/graph/store"
)

// CKG is built through its public CLI at two real committed snapshots. CKV
// is a coordinate-only control: no vector DB, chunk links or embeddings.
// All meanings are proposed and the active pointer belongs only to this fixture.
func TestForeignCodeAnchorPromotionPreservesActiveDataset(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	moduleRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	ckg := filepath.Join(t.TempDir(), "ckg")
	anchorPromotionCommand(t, ctx, moduleRoot, "go", "build", "-o", ckg, "./cmd/graph")
	source, dataset := t.TempDir(), t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/anchor-promotion\n\ngo 1.25.0\n")
	write("alpha.go", "package anchor\nfunc Alpha() int { return 1 }\n")
	write("ontology.yaml", "version: 1\nproject_id: anchor-promotion-control\ndomain: owned promotion control\ncompetency_questions: [\"Which captured Alpha backs this relationship?\"]\nconcepts:\n  - id: alpha\n    kind: rule\n    definition: The captured Alpha implementation.\n    includes: [Alpha]\n    excludes: [Other symbols]\n    terms: [{lang: en, value: alpha, preferred: true}]\n    status: proposed\n")
	anchorPromotionCommand(t, ctx, source, "git", "init", "-q")
	commit := func() string {
		t.Helper()
		anchorPromotionCommand(t, ctx, source, "git", "add", ".")
		anchorPromotionCommand(t, ctx, source, "git", "-c", "user.name=Owned Anchor Control", "-c", "user.email=fixture@local.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "owned source control")
		return strings.TrimSpace(anchorPromotionCommand(t, ctx, source, "git", "rev-parse", "HEAD"))
	}
	baseVersion := filepath.Join(dataset, "base")
	base := anchorPromotionVersion(t, ctx, ckg, source, baseVersion, commit())
	write("alpha.go", "package anchor\nfunc Alpha() int { return 2 }\n")
	candidateVersion := filepath.Join(dataset, "candidate")
	candidate := anchorPromotionVersion(t, ctx, ckg, source, candidateVersion, commit())
	if base.Snapshot == candidate.Snapshot || base.Evidence[len(base.Evidence)-1].CanonicalID != candidate.Evidence[len(candidate.Evidence)-1].CanonicalID {
		t.Fatal("controls need distinct snapshots with the same actual Alpha canonical ID")
	}
	storePath := filepath.Join(t.TempDir(), "semantic.sqlite")
	store, err := OpenStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	baseGraph, baseVector := filepath.Join(baseVersion, "graph"), filepath.Join(baseVersion, "vector")
	candidateGraph, candidateVector := filepath.Join(candidateVersion, "graph"), filepath.Join(candidateVersion, "vector")
	if err := store.PutAlignedRetained(ctx, base, source, baseGraph, baseVector, baseVersion); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateAlignedRetained(ctx, base.Snapshot.ProjectID, base.Snapshot.DatasetID, source, baseGraph, baseVector, baseVersion); err != nil {
		t.Fatal(err)
	}
	assertUnchanged := func(t *testing.T) {
		t.Helper()
		current, err := store.Current(ctx, base.Snapshot.ProjectID)
		if err != nil || !reflect.DeepEqual(current, base) {
			t.Fatalf("rejected promotion changed active projection: %+v %v", current, err)
		}
		retained, err := store.Load(ctx, base.Snapshot.ProjectID, base.Snapshot.DatasetID)
		if err != nil || !reflect.DeepEqual(retained, base) {
			t.Fatalf("rejected promotion changed original projection: %+v %v", retained, err)
		}
	}
	// Deliberately keep candidate JSON coordinates consistent while swapping
	// only the actual DB. This forces the real canonical-anchor gate to run.
	staleGraph := t.TempDir()
	for _, file := range []struct{ from, to string }{
		{filepath.Join(baseGraph, "graph.db"), filepath.Join(staleGraph, "graph.db")},
		{filepath.Join(candidateGraph, "manifest.json"), filepath.Join(staleGraph, "manifest.json")},
	} {
		body, err := os.ReadFile(file.from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file.to, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateDatasetAlignment(candidate, source, staleGraph, candidateVector); err != nil {
		t.Fatalf("JSON-coordinate positive control failed: %v", err)
	}
	if err := ValidateCodeAnchors(candidate, staleGraph); !errors.Is(err, ErrCodeAnchorSnapshotMismatch) {
		t.Fatalf("actual old graph DB was not rejected: %v", err)
	}
	clone := func(t *testing.T) Projection {
		body, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		var p Projection
		if err := json.Unmarshal(body, &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// gate names the typed rejection that must fire, so each case is
	// attributed to the projection check or the actual CKG anchor check
	// rather than to a shared message substring.
	for _, tc := range []struct {
		name, want string
		gate       error
		edit       func(*Projection)
		graph      string
	}{
		{"foreign-evidence-snapshot", "crosses", ErrEvidenceSnapshotMismatch, func(p *Projection) { p.Evidence[len(p.Evidence)-1] = base.Evidence[len(base.Evidence)-1] }, candidateGraph},
		{"relabelled-foreign-source-bytes", "digest mismatch", nil, func(p *Projection) {
			p.Evidence[len(p.Evidence)-1] = base.Evidence[len(base.Evidence)-1]
			p.Evidence[len(p.Evidence)-1].Snapshot = p.Snapshot
		}, candidateGraph},
		{"stale-actual-graph-with-matching-JSON", "crosses", ErrCodeAnchorSnapshotMismatch, func(*Projection) {}, staleGraph},
		{"missing-actual-canonical", "not found", nil, func(p *Projection) {
			p.Evidence[len(p.Evidence)-1].CanonicalID = "example.test/anchor-promotion.Missing"
			p.Assertions[0].ObjectID = p.Evidence[len(p.Evidence)-1].CanonicalID
		}, candidateGraph},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := clone(t)
			tc.edit(&p)
			for _, put := range []func() error{
				func() error { return store.PutAligned(ctx, p, source, tc.graph, candidateVector) },
				func() error {
					return store.PutAlignedRetained(ctx, p, source, tc.graph, candidateVector, candidateVersion)
				},
			} {
				err := put()
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("expected %q before persistence, got %v", tc.want, err)
				}
				for _, gate := range []error{ErrEvidenceSnapshotMismatch, ErrCodeAnchorSnapshotMismatch} {
					if errors.Is(err, gate) != (gate == tc.gate) {
						t.Fatalf("rejection attributed to wrong gate: want %v, got %v", tc.gate, err)
					}
				}
				if _, err := store.Load(ctx, p.Snapshot.ProjectID, p.Snapshot.DatasetID); !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("rejected candidate was persisted: %v", err)
				}
				assertUnchanged(t)
			}
			t.Log("both promotion writes refused; candidate absent and previous active projection unchanged")
		})
	}
	t.Run("activation-rechecks-actual-graph", func(t *testing.T) {
		// A valid candidate may be stored without becoming current.
		if err := store.PutAlignedRetained(ctx, candidate, source, candidateGraph, candidateVector, candidateVersion); err != nil {
			t.Fatal(err)
		}
		assertUnchanged(t)
		for _, activate := range []func() error{
			func() error {
				return store.ActivateAligned(ctx, candidate.Snapshot.ProjectID, candidate.Snapshot.DatasetID, source, staleGraph, candidateVector)
			},
			func() error {
				return store.ActivateAlignedRetained(ctx, candidate.Snapshot.ProjectID, candidate.Snapshot.DatasetID, source, staleGraph, candidateVector, candidateVersion)
			},
		} {
			if err := activate(); !errors.Is(err, ErrCodeAnchorSnapshotMismatch) || errors.Is(err, ErrEvidenceSnapshotMismatch) {
				t.Fatalf("stale canonical DB activation not refused by the CKG anchor gate: %v", err)
			}
			assertUnchanged(t)
		}
		t.Log("valid candidate stored; both stale-DB activations refused and previous active projection unchanged")
	})
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	ro, err := OpenStoreReadOnly(ctx, storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	current, err := ro.Current(ctx, base.Snapshot.ProjectID)
	if err != nil || !reflect.DeepEqual(current, base) {
		t.Fatalf("reopen lost previous active projection: %+v %v", current, err)
	}
}

func anchorPromotionCommand(t *testing.T, ctx context.Context, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	body, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, body)
	}
	return string(body)
}

func anchorPromotionVersion(t *testing.T, ctx context.Context, ckg, source, version, commit string) Projection {
	t.Helper()
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: source, Out: version, ProjectID: "anchor-promotion-control", SourceMode: "committed", SourceCommit: commit})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "coordinate-only-anchor-control")
	if err != nil {
		t.Fatal(err)
	}
	graphDir, vectorDir := filepath.Join(version, "graph"), filepath.Join(version, "vector")
	anchorPromotionCommand(t, ctx, source, ckg, "build", "--src", source, "--out", graphDir, "--lang", "go", "--no-cache", "--fail-on-parse-errors", "--project-id", captured.Identity.ProjectID, "--snapshot-id", captured.Identity.SnapshotID, "--dataset-id", identity.DatasetID, "--source-mode", "committed", "--file-manifest-digest", captured.Identity.FileManifestDigest, "--capture-policy-digest", captured.Identity.CapturePolicyDigest)
	var graph graphCoordinates
	if err := readCoordinates(filepath.Join(graphDir, "manifest.json"), &graph); err != nil {
		t.Fatal(err)
	}
	writeCoordinates(t, vectorDir, map[string]any{
		"project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID, "dataset_id": identity.DatasetID,
		"file_manifest_digest": captured.Identity.FileManifestDigest, "capture_policy_digest": captured.Identity.CapturePolicyDigest,
		"source_mode": "committed", "src_root": source, "src_commit": commit,
		"embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space",
		"sources": map[string]any{"ckg": map[string]any{"graph_digest": graph.GraphDigest, "src_commit": commit}},
	})
	identity, err = setup.PublishCandidateIdentity(version, captured.Identity, "coordinate-only-anchor-control")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{ProjectID: captured.Identity.ProjectID, DatasetID: identity.DatasetID, SnapshotID: captured.Identity.SnapshotID, SourceMode: "committed", Commit: commit}
	body, err := os.ReadFile(filepath.Join(source, "ontology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := ExtractOntology(snapshot, "ontology.yaml", body)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := graphstore.OpenReadOnly(filepath.Join(graphDir, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	nodes, err := reader.AllNodes()
	if err != nil {
		t.Fatal(err)
	}
	anchors := 0
	for _, node := range nodes {
		if node.Name != "Alpha" || node.FilePath != "alpha.go" || node.CanonicalID == "" {
			continue
		}
		anchors++
		proof, err := setup.ReadRetainedLines(version, "repo", node.FilePath, node.StartLine, node.EndLine)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(proof.Text))
		evidence := EvidenceSpan{ID: "alpha-code", Snapshot: snapshot, Kind: SourceCode, Path: node.FilePath, StartLine: node.StartLine, EndLine: node.EndLine, ContentSHA256: hex.EncodeToString(sum[:]), CanonicalID: node.CanonicalID, Extractor: "actual-ckg-control"}
		if err := AttachCodeEvidence(&p, reader, evidence); err != nil {
			t.Fatal(err)
		}
		p.Assertions = append(p.Assertions, Assertion{ID: "alpha-implementation", Predicate: PredicateImplementedBy, SubjectID: "alpha", ObjectID: node.CanonicalID, EvidenceIDs: []string{p.Concepts[0].EvidenceID, evidence.ID}, Status: StatusProposed})
	}
	if anchors != 1 {
		t.Fatalf("wanted exactly one actual Alpha canonical, got %d", anchors)
	}
	if err := p.ValidateRetainedSources(version); err != nil {
		t.Fatal(err)
	}
	return p
}
