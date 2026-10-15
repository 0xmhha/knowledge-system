package comparison

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// These are actual retained source archives with fixture identity manifests,
// not a claim of graph/vector indexing or semantic review.
func bindingFixture(t *testing.T, project, method string, purpose ...string) (collection.Version, []BindingEvidence) {
	t.Helper()
	source, root := t.TempDir(), t.TempDir()
	version := filepath.Join(root, "v1")
	if err := os.Mkdir(version, 0o700); err != nil {
		t.Fatal(err)
	}
	var err error
	version, err = filepath.EvalSymlinks(version)
	if err != nil {
		t.Fatal(err)
	}
	files := []struct{ name, body, kind string }{
		{"stake.go", "package chain\nfunc " + method + "() int { return 32 }\n", "code"},
		{"README.md", "The project names stake commitment " + method + ".\n", "document"},
		{"network.toml", "minimum_stake = 32\n", "configuration"},
		{"POLICY.md", "For fixture network only, entry requires 32 units.\n", "policy"},
		{"stake_test.go", "package chain\n// Test " + method + " entry threshold.\n", "test"},
	}
	if len(purpose) > 0 && purpose[0] == "membership" {
		files[0].body = "package chain\nfunc " + method + "(identity string) bool { return identity != \"\" }\n"
		files[1].body = "Stake registers a validator identity without committing funds.\n"
		files[2].body = "validator_registration = true\n"
		files[3].body = "Fixture policy permits identity-based validator admission; no stake commitment.\n"
		files[4].body = "package chain\n// Test validator identity admission.\n"
	}
	for _, f := range files {
		if err = os.WriteFile(filepath.Join(source, f.name), []byte(f.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	mode, commit := "snapshot-only", ""
	if len(purpose) > 0 && (purpose[0] == "committed" || purpose[0] == "working-tree") {
		mode = purpose[0]
		for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Binding fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "owned source fixture"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = source
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-10-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T00:00:00Z")
			if data, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("fixture git: %v %s", err, data)
			}
		}
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = source
		data, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		commit = strings.TrimSpace(string(data))
		if mode == "working-tree" {
			if err = os.WriteFile(filepath.Join(source, "stake.go"), []byte(files[0].body+"// owned local change\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: source, Out: version, ProjectID: project, SourceMode: mode, SourceCommit: commit})
	if err != nil {
		t.Fatal(err)
	}
	id, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"fixture"}`), "binding-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range []string{"graph", "vector"} {
		dir := filepath.Join(version, engine)
		if err = os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(map[string]any{"src_root": source, "src_commit": commit, "graph_digest": "fixture", "schema_version": "1.23", "embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "fixture", "project_id": project, "snapshot_id": captured.Identity.SnapshotID, "dataset_id": id.DatasetID, "source_mode": mode, "file_manifest_digest": captured.Identity.FileManifestDigest, "capture_policy_digest": captured.Identity.CapturePolicyDigest})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = setup.PublishCandidateIdentity(version, captured.Identity, "binding-fixture"); err != nil {
		t.Fatal(err)
	}
	v := collection.Version{Coordinates: contract.V2Coordinates{ProjectID: project, DatasetID: id.DatasetID, SnapshotID: captured.Identity.SnapshotID, SourceMode: mode, BaseCommit: commit}, VersionDir: version, Origins: []string{"repo"}}
	refs := []BindingEvidence{}
	for _, f := range files {
		line := 1
		if f.kind == "code" || f.kind == "test" {
			line = 2
		}
		actual, err := setup.ReadRetainedLines(version, "repo", f.name, line, line)
		if err != nil {
			t.Fatal(err)
		}
		c := contract.CitationV2{File: actual.File, StartLine: actual.StartLine, EndLine: actual.EndLine, CommitHash: actual.CommitHash, ProjectID: actual.ProjectID, DatasetID: actual.DatasetID, SnapshotID: actual.SnapshotID, SourceMode: actual.SourceMode, BaseCommit: actual.BaseCommit, OriginID: actual.OriginID, FileSHA256: actual.FileSHA256, ContentSHA256: actual.ContentSHA256}
		refs = append(refs, BindingEvidence{ID: "evidence-" + f.kind, Kind: f.kind, Facets: []string{"entry"}, Citation: c})
	}
	return v, refs
}
func bindingCatalog(versions ...collection.Version) collection.Catalog {
	c := collection.Catalog{SchemaVersion: 1, Versions: versions, Collections: []collection.Collection{{ID: "chains"}}}
	seen := map[string]bool{}
	for _, v := range versions {
		if !seen[v.Coordinates.ProjectID] {
			c.Projects = append(c.Projects, collection.Project{ID: v.Coordinates.ProjectID, Name: "same-name"})
			seen[v.Coordinates.ProjectID] = true
		}
		c.Collections[0].Members = append(c.Collections[0].Members, v.Coordinates)
	}
	return c
}
func bindingCleaner(t *testing.T) *sanitize.Engine {
	t.Helper()
	rules, err := config.LoadSanitizeRulesetBytes([]byte("version: 1\nrules:\n  - id: secret\n    description: fixture\n    pattern: SECRET\n    action: mask\n    severity: high\n"))
	if err != nil {
		t.Fatal(err)
	}
	cleaner, err := sanitize.New(rules)
	if err != nil {
		t.Fatal(err)
	}
	return cleaner
}
func proposedBinding(t *testing.T, s *Scheme, v collection.Version, refs []BindingEvidence, id, local string) ProjectBinding {
	t.Helper()
	resolved := s.Resolve("staking")
	if len(resolved.Candidates) != 1 {
		t.Fatal(resolved)
	}
	return ProjectBinding{ID: id, Concept: resolved.Candidates[0].Key, Coordinates: v.Coordinates, Conditions: Conditions{Network: "fixture-network", Configuration: "fixture-config", ComponentRole: "staking-network", ObservedAt: "2026-10-16T00:00:00Z"}, LocalConcept: local, Mapping: "close", Reason: "Proposed role correspondence from the cited source; not human reviewed.", Evidence: refs}
}
func TestDifferentNamesMapToSeparateProposedSourceGroups(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	a, ar := bindingFixture(t, "chain-a", "Stake")
	b, br := bindingFixture(t, "chain-b", "Bond")
	reader, err := NewMappingReader(ctx, bindingCatalog(a, b), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, test := range []struct {
		v                  collection.Version
		refs               []BindingEvidence
		id, local, foreign string
	}{{a, ar, "mapping-a", "Stake", "Bond"}, {b, br, "mapping-b", "Bond", "Stake"}} {
		result, err := reader.ReadBinding(ctx, "chains", proposedBinding(t, s, test.v, test.refs, test.id, test.local))
		if err != nil {
			t.Fatal(err)
		}
		if result.Coordinates != test.v.Coordinates || result.Concept.ConceptID != "staking" || result.LocalConcept != test.local || result.ReviewState != "proposed" || result.Verification != "archived_source_hash_and_span_only" || len(result.Evidence) != 5 || len(result.Warnings) != 3 {
			t.Fatalf("incomplete source mapping: %+v", result)
		}
		kinds := map[string]bool{}
		for _, e := range result.Evidence {
			kinds[e.Reference.Kind] = true
			if strings.Contains(e.Body, test.foreign) || e.Reference.Citation.ProjectID != test.v.Coordinates.ProjectID {
				t.Fatal("foreign implementation or citation")
			}
		}
		if len(kinds) != 5 {
			t.Fatal(kinds)
		}
	}
}
func TestBindingSchemaRejectsUnscopedOrInventedClaims(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	base := proposedBinding(t, s, v, refs, "mapping-a", "Stake")
	cases := map[string]func(*ProjectBinding){
		"pack drift":       func(b *ProjectBinding) { b.Concept.Digest = strings.Repeat("a", 64) },
		"version drift":    func(b *ProjectBinding) { b.Concept.Version = "2.0.0" },
		"unknown concept":  func(b *ProjectBinding) { b.Concept.ConceptID = "unknown" },
		"unknown mapping":  func(b *ProjectBinding) { b.Mapping = "identical-entity" },
		"no network":       func(b *ProjectBinding) { b.Conditions.Network = "" },
		"no config":        func(b *ProjectBinding) { b.Conditions.Configuration = "" },
		"no role":          func(b *ProjectBinding) { b.Conditions.ComponentRole = "" },
		"no time":          func(b *ProjectBinding) { b.Conditions.ObservedAt = "" },
		"no reason":        func(b *ProjectBinding) { b.Reason = "" },
		"no refs":          func(b *ProjectBinding) { b.Evidence = nil },
		"foreign project":  func(b *ProjectBinding) { b.Evidence[0].Citation.ProjectID = "foreign" },
		"foreign snapshot": func(b *ProjectBinding) { b.Evidence[0].Citation.SnapshotID = strings.Repeat("a", 64) },
		"unknown origin":   func(b *ProjectBinding) { b.Evidence[0].Citation.OriginID = "arbitrary" },
		"escaping path":    func(b *ProjectBinding) { b.Evidence[0].Citation.File = "../secret" },
		"non hash":         func(b *ProjectBinding) { b.Evidence[0].Citation.FileSHA256 = "not-a-hash" },
		"excess span":      func(b *ProjectBinding) { b.Evidence[0].Citation.EndLine = 300 },
		"duplicate refs":   func(b *ProjectBinding) { b.Evidence = append(b.Evidence, b.Evidence[0]) },
		"unknown kind":     func(b *ProjectBinding) { b.Evidence[0].Kind = "measured-TPS" },
		"unknown facet":    func(b *ProjectBinding) { b.Evidence[0].Facets = []string{"quorum"} },
		"duplicate facet":  func(b *ProjectBinding) { b.Evidence[0].Facets = []string{"entry", "entry"} },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var b ProjectBinding
			if err = json.Unmarshal(data, &b); err != nil {
				t.Fatal(err)
			}
			edit(&b)
			if _, err = s.BindingDigest(b); !errors.Is(err, ErrBindingInvalid) {
				t.Fatalf("accepted invalid binding: %v", err)
			}
		})
	}
	doc := BindingDocument{SchemaVersion: 1, Bindings: []ProjectBinding{base}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecodeBindings(data); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{[]byte(strings.Replace(string(data), `"mapping":"close"`, `"mapping":"close","review_state":"verified"`, 1)), []byte(strings.Replace(string(data), `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1))} {
		if _, err = s.DecodeBindings(bad); err == nil {
			t.Fatal("invented approval or duplicate key accepted")
		}
	}
	doc.Bindings = append(doc.Bindings, base)
	data, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecodeBindings(data); err == nil {
		t.Fatal("duplicate binding accepted")
	}
}
func TestForgedHashAndWrongCollectionReturnNoEvidence(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	reader, err := NewMappingReader(context.Background(), bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	b := proposedBinding(t, s, v, refs, "mapping-a", "Stake")
	b.Evidence[0].Citation.ContentSHA256 = strings.Repeat("0", 64)
	if got, err := reader.ReadBinding(context.Background(), "chains", b); !errors.Is(err, ErrBindingEvidence) || len(got.Evidence) != 0 {
		t.Fatalf("forged span accepted: %+v %v", got, err)
	}
	b = proposedBinding(t, s, v, refs, "mapping-a", "Stake")
	if got, err := reader.ReadBinding(context.Background(), "unselected", b); err == nil || len(got.Evidence) != 0 {
		t.Fatal("unselected collection admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = reader.ReadBinding(ctx, "chains", b); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = reader.ReadBinding(context.Background(), "chains", b); !errors.Is(err, collection.ErrClosed) {
		t.Fatal(err)
	}
}
func TestMappingAnnotationsAreSanitizedAndDigestTracksInterpretation(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	reader, err := NewMappingReader(context.Background(), bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	b := proposedBinding(t, s, v, refs, "mapping-a", "Stake SECRET")
	b.Reason = "SECRET reason"
	b.Conditions.Network = "SECRET-network"
	before, err := s.BindingDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	result, err := reader.ReadBinding(context.Background(), "chains", b)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "SECRET") {
		t.Fatal("annotation secret leaked")
	}
	b.Mapping = "related"
	after, err := s.BindingDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("mapping change did not invalidate digest")
	}
	b.Evidence[0].Facets[0] = "exit"
	if result.Evidence[0].Reference.Facets[0] != "entry" {
		t.Fatal("caller changed returned binding")
	}
}

func TestSameLocalNameDoesNotChooseTheSameCommonConcept(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	a, ar := bindingFixture(t, "chain-a", "Stake")
	c, cr := bindingFixture(t, "chain-c", "Stake", "membership")
	reader, err := NewMappingReader(ctx, bindingCatalog(a, c), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	staking := proposedBinding(t, s, a, ar, "mapping-a", "Stake")
	validator := proposedBinding(t, s, c, cr, "mapping-c", "Stake")
	validator.Concept = s.Resolve("validator").Candidates[0].Key
	validator.Conditions.ComponentRole = "validator-network"
	validator.Reason = "Proposed validator membership role from explicit identity-only source; same local label does not imply stake commitment."
	for i := range validator.Evidence {
		validator.Evidence[i].Facets = []string{"admission"}
	}
	first, err := reader.ReadBinding(ctx, "chains", staking)
	if err != nil {
		t.Fatal(err)
	}
	second, err := reader.ReadBinding(ctx, "chains", validator)
	if err != nil {
		t.Fatal(err)
	}
	if first.LocalConcept != second.LocalConcept || first.Concept.ConceptID == second.Concept.ConceptID || first.Coordinates == second.Coordinates || !strings.Contains(second.Evidence[0].Body, "identity") {
		t.Fatalf("same name collapsed different roles: %+v %+v", first, second)
	}
}
func TestBindingReadsRetainedSourceAndRejectsArchiveCorruption(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	reader, err := NewMappingReader(context.Background(), bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	b := proposedBinding(t, s, v, refs, "mapping-a", "Stake")
	data, err := os.ReadFile(filepath.Join(v.VersionDir, "graph/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Root string `json:"src_root"`
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(manifest.Root, "stake.go"), []byte("package chain\nfunc Stake() int { return 999 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := reader.ReadBinding(context.Background(), "chains", b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Evidence[0].Body, "return 32") || strings.Contains(result.Evidence[0].Body, "999") {
		t.Fatal("read mutable checkout")
	}
	// Only this owned corruption fixture changes the immutable blob permission.
	if err = os.Chmod(filepath.Join(v.VersionDir, "sources/blobs", refs[0].Citation.FileSHA256), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(v.VersionDir, "sources/blobs", refs[0].Citation.FileSHA256), []byte("corrupt retained source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err = reader.ReadBinding(context.Background(), "chains", b)
	if err == nil || len(result.Evidence) != 0 {
		t.Fatalf("corrupt source admitted: %+v %v", result, err)
	}
}

func TestBindingsKeepCommittedAndWorkingTreeCitationSemantics(t *testing.T) {
	for _, mode := range []string{"committed", "working-tree"} {
		t.Run(mode, func(t *testing.T) {
			s, err := LoadScheme(shippedRoot)
			if err != nil {
				t.Fatal(err)
			}
			v, refs := bindingFixture(t, "chain-a", "Stake", mode)
			reader, err := NewMappingReader(context.Background(), bindingCatalog(v), s, bindingCleaner(t))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := reader.Close(); err != nil {
					t.Error(err)
				}
			}()
			b := proposedBinding(t, s, v, refs, "mapping-a", "Stake")
			result, err := reader.ReadBinding(context.Background(), "chains", b)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range result.Evidence {
				c := e.Reference.Citation
				if c.BaseCommit != v.Coordinates.BaseCommit || c.BaseCommit == "" || c.SourceMode != mode {
					t.Fatal("base identity lost")
				}
				if mode == "committed" && c.CommitHash != c.BaseCommit || mode == "working-tree" && c.CommitHash != "" {
					t.Fatal("citation source mode mislabeled")
				}
			}
		})
	}
}

func TestBindingIDsAreScopedByProjectVersionAndConditions(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	a, ar := bindingFixture(t, "chain-a", "Stake")
	b, br := bindingFixture(t, "chain-b", "Stake")
	first := proposedBinding(t, s, a, ar, "staking", "Stake")
	second := proposedBinding(t, s, b, br, "staking", "Stake")
	variant := first
	variant.Conditions.Network = "another-network"
	d := BindingDocument{SchemaVersion: 1, Bindings: []ProjectBinding{first, second, variant}}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := s.DecodeBindings(data)
	if err != nil || len(decoded.Bindings) != 3 {
		t.Fatalf("same local ID collided across scopes: %v", err)
	}
	firstDigest, err := s.BindingDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := s.BindingDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	variantDigest, err := s.BindingDigest(variant)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest == secondDigest || firstDigest == variantDigest {
		t.Fatal("scope omitted from mapping digest")
	}
	equivalent := first
	equivalent.Conditions.ObservedAt = "2026-10-16T09:00:00+09:00"
	d.Bindings = append(d.Bindings, equivalent)
	data, err = json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecodeBindings(data); err == nil {
		t.Fatal("same instant disguised duplicate scope")
	}
}
