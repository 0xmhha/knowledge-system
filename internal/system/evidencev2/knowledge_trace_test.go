package evidencev2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/internal/testsupport/graphfixture"
	graphtypes "github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func reviewedTraceFixture(t *testing.T, statement string) (string, semantic.ActiveProjection) {
	t.Helper()
	root, version := knowledgeVersion(t, "public", statement, "public")
	identity, err := setup.InspectVersionIdentity(version)
	if err != nil || identity == nil {
		t.Fatalf("identity: %+v %v", identity, err)
	}
	snapshot := semantic.Snapshot{ProjectID: "p", DatasetID: identity.DatasetID,
		SnapshotID: identity.Source.SnapshotID, SourceMode: "snapshot-only"}
	ontologySource, err := os.ReadFile(filepath.Join(root, "ontology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	specSource, err := os.ReadFile(filepath.Join(root, "spec.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ontology, _, err := semantic.ExtractOntology(snapshot, "ontology.yaml", ontologySource)
	if err != nil {
		t.Fatal(err)
	}
	spec, _, err := semantic.ExtractSpec(snapshot, "spec.yaml", specSource)
	if err != nil {
		t.Fatal(err)
	}
	span := func(id string, kind semantic.SourceKind, path string, line int, body, canonical string) semantic.EvidenceSpan {
		t.Helper()
		sum := sha256.Sum256([]byte(body))
		return semantic.EvidenceSpan{ID: id, Snapshot: snapshot, Kind: kind, Path: path,
			StartLine: line, EndLine: line, ContentSHA256: hex.EncodeToString(sum[:]),
			CanonicalID: canonical, Extractor: "ckg-ast-v1"}
	}
	p := semantic.Projection{SchemaVersion: semantic.SchemaVersion, Snapshot: snapshot,
		Evidence: append(append(ontology.Evidence, spec.Evidence...),
			span("code", semantic.SourceCode, "main.go", 2, "func Alpha() {}\n", "pkg.Alpha"),
			span("test", semantic.SourceTest, "main_test.go", 2, "func TestAlpha() {}\n", "pkg.TestAlpha")),
		Concepts: ontology.Concepts, Requirements: spec.Requirements,
		Assertions: []semantic.Assertion{
			{ID: "impl", Predicate: semantic.PredicateImplementedBy, SubjectID: "alpha", ObjectID: "pkg.Alpha", EvidenceIDs: []string{ontology.Evidence[0].ID, "code"}, Status: semantic.StatusVerified, ReviewedBy: "reviewer"},
			{ID: "tested", Predicate: semantic.PredicateTestedBy, SubjectID: "pkg.Alpha", ObjectID: "pkg.TestAlpha", EvidenceIDs: []string{"code", "test"}, Status: semantic.StatusVerified, ReviewedBy: "reviewer"},
			{ID: "checked", Predicate: semantic.PredicateCheckedBy, SubjectID: "AC-1", ObjectID: "pkg.TestAlpha", EvidenceIDs: []string{spec.Evidence[1].ID, "test"}, Status: semantic.StatusVerified, ReviewedBy: "reviewer"},
		},
	}
	if err := graphfixture.Create(filepath.Join(version, "graph", "graph.db"), root, "p", identity.DatasetID,
		identity.Source.SnapshotID, strings.Repeat("a", 64), []graphtypes.Node{
			{ID: "1111111111111111", Type: graphtypes.NodeFunction, Name: "Alpha", QualifiedName: "pkg.Alpha", CanonicalID: "pkg.Alpha", FilePath: "main.go", StartLine: 2, EndLine: 2, StartByte: 10, EndByte: 25, Language: "go", Confidence: graphtypes.ConfExtracted},
			{ID: "2222222222222222", Type: graphtypes.NodeFunction, Name: "TestAlpha", QualifiedName: "pkg.TestAlpha", CanonicalID: "pkg.TestAlpha", FilePath: "main_test.go", StartLine: 2, EndLine: 2, StartByte: 10, EndByte: 29, Language: "go", Confidence: graphtypes.ConfExtracted},
		}); err != nil {
		t.Fatal(err)
	}
	store, err := semantic.OpenStore(filepath.Join(t.TempDir(), "semantic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.PutAlignedRetained(ctx, p, root, filepath.Join(version, "graph"), filepath.Join(version, "vector"), version); err != nil {
		t.Fatal(err)
	}
	active, err := store.LoadAlignedRetained(ctx, "p", identity.DatasetID, root, filepath.Join(version, "graph"), filepath.Join(version, "vector"), version)
	if err != nil {
		t.Fatal(err)
	}
	return version, active
}

func TestAttachVerifiedTracesRequiresAlignedCitedPath(t *testing.T) {
	version, active := reviewedTraceFixture(t, "Separate approval is required.")
	ctx := context.Background()
	base, err := Build(ctx, version, "why", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	local, err := AttachKnowledge(ctx, base, version, "2026-06-01", "transfers", testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := AttachVerifiedTraces(ctx, local, version, "2026-06-01", "transfers", active, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(got); err != nil {
		t.Fatal(err)
	}
	if err := Verify(local); err != nil {
		t.Fatalf("augmenting traces mutated the original response: %v", err)
	}
	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip contract.EvidencePackV2
	if err := json.Unmarshal(wire, &roundTrip); err != nil || Verify(roundTrip) != nil {
		t.Fatalf("v2 consumer cannot replay trace response: %v", err)
	}
	value := got.Semantic.(contract.KnowledgeSemanticV2)
	if len(value.KnowledgeContext.TraceLinks) != 1 || len(value.CodingContext.ImplementedBehavior) != 1 ||
		value.KnowledgeContext.TraceLinks[0].CodeCitation.File != "main.go" || len(got.Citations) <= len(local.Citations) {
		t.Fatalf("reviewed external path missing: %+v", value)
	}
	stale, err := AttachVerifiedTraces(ctx, local, version, "2025-01-01", "transfers", active, testCleaner(t))
	if err != nil || len(stale.Semantic.(contract.KnowledgeSemanticV2).KnowledgeContext.TraceLinks) != 0 {
		t.Fatalf("future ADR exposed: %+v %v", stale.Semantic, err)
	}
	_, foreign := reviewedTraceFixture(t, "A different policy applies.")
	mismatch, err := AttachVerifiedTraces(ctx, local, version, "2026-06-01", "transfers", foreign, testCleaner(t))
	if err != nil || len(mismatch.Semantic.(contract.KnowledgeSemanticV2).KnowledgeContext.TraceLinks) != 0 || Verify(mismatch) != nil {
		t.Fatalf("foreign projection path exposed: %+v %v", mismatch.Semantic, err)
	}
}
