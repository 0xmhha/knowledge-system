package evidencev2

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func testCleaner(t *testing.T) *sanitize.Engine {
	t.Helper()
	rules, err := config.LoadSanitizeRulesetBytes([]byte("version: 1\nrules:\n  - id: secret\n    description: test\n    pattern: SECRET\n    action: drop\n    severity: high\n"))
	if err != nil {
		t.Fatal(err)
	}
	cleaner, err := sanitize.New(rules)
	if err != nil {
		t.Fatal(err)
	}
	return cleaner
}

func testVersion(t *testing.T, content string) (string, string) {
	t.Helper()
	root, version := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: root, Out: version, ProjectID: "p", SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "inputs")
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"graph", "vector"} {
		dir := filepath.Join(version, side)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := map[string]any{"src_root": root, "graph_digest": "graph", "schema_version": "1.23",
			"embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space",
			"project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID,
			"dataset_id": identity.DatasetID, "source_mode": captured.Identity.SourceMode,
			"file_manifest_digest":  captured.Identity.FileManifestDigest,
			"capture_policy_digest": captured.Identity.CapturePolicyDigest}
		buf, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), buf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := setup.PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	return root, version
}

func TestBuildUsesRetainedBytesAndV2Tuple(t *testing.T) {
	root, version := testVersion(t, "past\r\nreason\n")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pack, err := Build(context.Background(), version, "why", []contract.Citation{{File: "README.md", StartLine: 2, EndLine: 2}}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	if pack.FormatVersion != 2 || pack.Coordinates.SourceMode != "snapshot-only" || len(pack.Bodies) != 1 || pack.Bodies[0].Text != "reason\n" || pack.Citations[0].CommitHash != "" {
		t.Fatalf("incorrect retained v2 evidence: %+v", pack)
	}
	if err := Verify(pack); err != nil {
		t.Fatal(err)
	}
	pack.Bodies[0].Text = "tampered"
	if err := Verify(pack); err == nil {
		t.Fatal("tampered body kept v2 integrity")
	}
}

func TestV2IntegrityCoversOptionalOntologyFallback(t *testing.T) {
	_, version := testVersion(t, "original\n")
	pack, err := Build(context.Background(), version, "Alpha", nil, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	if pack.Metadata.Ontology != nil {
		t.Fatal("default v2 enabled ontology")
	}
	pack.Metadata.Ontology = &contract.OntologyDiagnostic{Mode: "relations", State: "unavailable", Reason: "store_or_dataset_missing"}
	if err := Stamp(&pack); err != nil {
		t.Fatal(err)
	}
	if err := Verify(pack); err != nil {
		t.Fatal(err)
	}
	pack.Metadata.Ontology.State = "active"
	if err := Verify(pack); err == nil {
		t.Fatal("tampered ontology state kept valid integrity")
	}
}

func TestBuildDropsSecretAndRejectsForeignCommit(t *testing.T) {
	_, version := testVersion(t, "SECRET=bad\n")
	pack, err := Build(context.Background(), version, "inspect", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}}, testCleaner(t))
	if err != nil || pack.EvidenceState != "partial" || len(pack.Bodies) != 0 {
		t.Fatalf("secret escaped: %+v %v", pack, err)
	}
	_, err = Build(context.Background(), version, "inspect", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1, CommitHash: strings.Repeat("a", 40)}}, testCleaner(t))
	if err == nil || !strings.Contains(err.Error(), "snapshot_mismatch") {
		t.Fatalf("foreign commit accepted: %v", err)
	}
}

func TestCanonicalJCSStringsAndSortedKeys(t *testing.T) {
	value := map[string]any{"z": "€\u2028<\n", "a": 1}
	got, err := canonicalBytes(value)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"a\":1,\"z\":\"€\u2028<\\n\"}"
	if string(got) != want {
		t.Fatalf("canonical bytes %q, want %q", got, want)
	}
	if _, err := canonicalBytes(map[string]any{"n": 1.5}); err == nil {
		t.Fatal("unsupported float silently hashed")
	}
	if _, err := canonicalBytes(map[string]any{"body": string([]byte{0xff})}); err == nil {
		t.Fatal("invalid UTF-8 silently normalized")
	}
}

func TestV2IntegrityMatchesIndependentJSGolden(t *testing.T) {
	pack := contract.EvidencePackV2{FormatVersion: 2,
		Coordinates: contract.V2Coordinates{ProjectID: "p", DatasetID: strings.Repeat("a", 64),
			SnapshotID: strings.Repeat("b", 64), SourceMode: "snapshot-only"},
		Query: "why", Citations: []contract.CitationV2{}, Bodies: []contract.BodyV2{},
		GraphNeighbors: []any{}, EvidenceState: "complete",
		Metadata: contract.V2Metadata{IntegrityHashAlgo: "sha256-v2"}}
	if err := Stamp(&pack); err != nil {
		t.Fatal(err)
	}
	if want := "d725ea84b43532b833751a95b41a09fbf954982aeb2abe6840f89a81c25817b4"; pack.Metadata.IntegrityHash != want {
		t.Fatalf("v2 JCS golden changed: %s, want %s", pack.Metadata.IntegrityHash, want)
	}
}
