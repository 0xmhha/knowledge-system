package comparison

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func emptyMappingReviews() MappingReviewDocument {
	return MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: 1}, Records: []MappingReview{}}
}
func coverageQuery(t *testing.T, s *Scheme, v collection.Version, b ProjectBinding) CoverageRequest {
	t.Helper()
	return CoverageRequest{Scope: ProjectDomainScope{Coordinates: v.Coordinates, Industry: "blockchain", Conditions: b.Conditions}, Concept: s.Resolve("staking").Candidates[0].Key, Facet: "entry", RequiredCapability: "retained-source"}
}

// Raw language/document fixtures verify archive access only, never AST/indexing.
func coverageRawFixture(t *testing.T, project string, files map[string]string) (collection.Version, []BindingEvidence) {
	t.Helper()
	source, dataset := t.TempDir(), t.TempDir()
	version := filepath.Join(dataset, "v1")
	if err := os.Mkdir(version, 0o700); err != nil {
		t.Fatal(err)
	}
	var err error
	version, err = filepath.EvalSymlinks(version)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for name, body := range files {
		names = append(names, name)
		if err = os.WriteFile(filepath.Join(source, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(names)
	capture, err := setup.CaptureSource(setup.CaptureOptions{Root: source, Out: version, ProjectID: project, SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := setup.NewDatasetIdentity(capture.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"fixture"}`), "binding-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range []string{"graph", "vector"} {
		dir := filepath.Join(version, engine)
		if err = os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(map[string]any{"src_root": source, "src_commit": "", "graph_digest": "fixture", "schema_version": "1.23", "embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "fixture", "project_id": project, "snapshot_id": capture.Identity.SnapshotID, "dataset_id": identity.DatasetID, "source_mode": "snapshot-only", "file_manifest_digest": capture.Identity.FileManifestDigest, "capture_policy_digest": capture.Identity.CapturePolicyDigest})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = setup.PublishCandidateIdentity(version, capture.Identity, "binding-fixture"); err != nil {
		t.Fatal(err)
	}
	v := collection.Version{Coordinates: contract.V2Coordinates{ProjectID: project, DatasetID: identity.DatasetID, SnapshotID: capture.Identity.SnapshotID, SourceMode: "snapshot-only"}, VersionDir: version, Origins: []string{"repo"}}
	refs := []BindingEvidence{}
	for i, name := range names {
		end := strings.Count(files[name], "\n")
		if !strings.HasSuffix(files[name], "\n") {
			end++
		}
		e, err := setup.ReadRetainedLines(version, "repo", name, 1, end)
		if err != nil {
			t.Fatal(err)
		}
		kind := "document"
		if strings.HasSuffix(name, ".rs") {
			kind = "code"
		}
		refs = append(refs, BindingEvidence{ID: "source-" + string(rune('a'+i)), Kind: kind, Facets: []string{"entry"}, Citation: contract.CitationV2{File: e.File, StartLine: e.StartLine, EndLine: e.EndLine, CommitHash: e.CommitHash, ProjectID: e.ProjectID, DatasetID: e.DatasetID, SnapshotID: e.SnapshotID, SourceMode: e.SourceMode, BaseCommit: e.BaseCommit, OriginID: e.OriginID, FileSHA256: e.FileSHA256, ContentSHA256: e.ContentSHA256}})
	}
	return v, refs
}
func TestCoverageSeparatesCollectionUnknownCapabilityAndScope(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	b := proposedBinding(t, s, v, refs, "staking", "Stake")
	reader, err := NewMappingReader(ctx, bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	at := reviewInstant(t, "2026-10-17T00:00:00Z")
	cases := []struct {
		name    string
		edit    func(*CoverageRequest, *BindingDocument)
		state   string
		archive bool
	}{
		{"no mapping collected", func(q *CoverageRequest, d *BindingDocument) { d.Bindings = nil }, "not_collected", true},
		{"unreviewed is unknown by default", func(q *CoverageRequest, d *BindingDocument) {}, "unknown", true},
		{"explicit candidate overlap", func(q *CoverageRequest, d *BindingDocument) { q.IncludeProposed = true }, "similar", true},
		{"explicit candidate exact is not entity identity", func(q *CoverageRequest, d *BindingDocument) {
			q.IncludeProposed = true
			d.Bindings[0].Mapping = "exact"
		}, "exact", true},
		{"missing facet is unknown not absent", func(q *CoverageRequest, d *BindingDocument) { q.IncludeProposed = true; q.Facet = "exit" }, "unknown", true},
		{"another network stays uncollected", func(q *CoverageRequest, d *BindingDocument) { q.Scope.Conditions.Network = "another-network" }, "not_collected", true},
		{"another configuration stays uncollected", func(q *CoverageRequest, d *BindingDocument) { q.Scope.Conditions.Configuration = "another-config" }, "not_collected", true},
		{"nonselected industry unknown", func(q *CoverageRequest, d *BindingDocument) { q.Scope.Industry = "" }, "unknown", false},
		{"different industry incomparable", func(q *CoverageRequest, d *BindingDocument) { q.Scope.Industry = "ecommerce" }, "incomparable", false},
		{"outside role incomparable", func(q *CoverageRequest, d *BindingDocument) { q.Scope.Conditions.ComponentRole = "sequencer" }, "incomparable", false},
		{"Rust AST unsupported by reader", func(q *CoverageRequest, d *BindingDocument) { q.RequiredCapability = "rust-ast" }, "unsupported", false},
		{"runtime measurement unsupported", func(q *CoverageRequest, d *BindingDocument) { q.RequiredCapability = "runtime-measurement" }, "unsupported", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			q := coverageQuery(t, s, v, b)
			copyB := b
			copyB.Evidence = append([]BindingEvidence(nil), b.Evidence...)
			d := BindingDocument{SchemaVersion: 2, Bindings: []ProjectBinding{copyB}}
			test.edit(&q, &d)
			got, err := reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != test.state || got.ArchiveVerified != test.archive || got.EntityIdentity {
				t.Fatalf("wrong classification %+v", got)
			}
			if got.State == "not_applicable" {
				t.Fatal("absence inferred without cited declaration")
			}
			for _, mapping := range got.Mappings {
				for _, e := range mapping.Binding.Evidence {
					if !hasFacet(e.Reference, q.Facet) {
						t.Fatal("unrequested facet leaked")
					}
				}
			}
		})
	}
}
func TestApplicabilityDeclarationRequiresVersionedSourceAndReview(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := coverageRawFixture(t, "chain-poa", map[string]string{"POLICY.md": "This fixture network does not implement staking. Validator identity is configured directly.\n"})
	b := proposedBinding(t, s, v, refs, "staking-absence", "Staking applicability")
	b.Mapping = "related"
	b.Applicability = "not_applicable"
	b.Conditions.ComponentRole = "validator-network"
	d := BindingDocument{SchemaVersion: 2, Bindings: []ProjectBinding{b}}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecodeBindings(data); err != nil {
		t.Fatal(err)
	}
	d.SchemaVersion = 1
	data, err = json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecodeBindings(data); !errors.Is(err, ErrBindingInvalid) {
		t.Fatal("new declaration silently accepted in schema1")
	}
	d.SchemaVersion = 2
	reader, err := NewMappingReader(ctx, bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	q := coverageQuery(t, s, v, b)
	at := reviewInstant(t, "2026-10-17T00:00:00Z")
	got, err := reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
	if err != nil {
		t.Fatal(err)
	}
	if got.State == "not_applicable" || len(got.Mappings) != 0 {
		t.Fatal("unreviewed absence declared definitive by default")
	}
	q.IncludeProposed = true
	got, err = reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "not_applicable" || !got.UnreviewedCandidatesIncluded || len(got.Mappings) != 1 || !strings.Contains(got.Mappings[0].Binding.Evidence[0].Body, "does not implement staking") {
		t.Fatal("explicit sourced declaration not available as candidate")
	}
	review := fixtureMappingReview(t, s, b, "approve-declaration", "controlled-reviewer", "verified", "2026-10-16T01:00:00Z")
	reviews := emptyMappingReviews()
	reviews.Records = []MappingReview{review}
	q.IncludeProposed = false
	got, err = reader.Coverage(ctx, "chains", q, d, reviews, at)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "not_applicable" || got.UnreviewedCandidatesIncluded {
		t.Fatal("exact reviewed declaration not applied")
	}
	changed := b
	changed.Applicability = "unknown"
	oldDigest, err := s.BindingDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	newDigest, err := s.BindingDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if oldDigest == newDigest {
		t.Fatal("applicability change retained approved digest")
	}
	codeOnly := b
	codeOnly.Evidence = append([]BindingEvidence(nil), b.Evidence...)
	codeOnly.Evidence[0].Kind = "code"
	if _, err = s.BindingDigest(codeOnly); !errors.Is(err, ErrBindingInvalid) {
		t.Fatal("code-search-only absence declaration accepted")
	}
}
func TestCoverageWithinProjectConflictDoesNotFlattenProjectDifferences(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	a, ar := coverageRawFixture(t, "chain-a", map[string]string{"POLICY-POS.md": "Staking entry is supported in this fixture configuration.\n", "POLICY-NEG.md": "Staking entry is not applicable in this same fixture configuration.\n"})
	b, br := coverageRawFixture(t, "chain-b", map[string]string{"POLICY.md": "Staking is not applicable in the second fixture project.\n"})
	var posRef, negRef BindingEvidence
	for _, e := range ar {
		if strings.Contains(e.Citation.File, "POS") {
			posRef = e
		} else {
			negRef = e
		}
	}
	positive := proposedBinding(t, s, a, []BindingEvidence{posRef}, "staking-positive", "Staking")
	negative := proposedBinding(t, s, a, []BindingEvidence{negRef}, "staking-negative", "Staking applicability")
	negative.Mapping = "related"
	negative.Applicability = "not_applicable"
	other := proposedBinding(t, s, b, br, "staking-negative", "Staking applicability")
	other.Mapping = "related"
	other.Applicability = "not_applicable"
	reader, err := NewMappingReader(ctx, bindingCatalog(a, b), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	d := BindingDocument{SchemaVersion: 2, Bindings: []ProjectBinding{positive, negative, other}}
	q := coverageQuery(t, s, a, positive)
	q.IncludeProposed = true
	at := reviewInstant(t, "2026-10-17T00:00:00Z")
	got, err := reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "conflict" || len(got.Mappings) != 2 {
		t.Fatal("same-project contradiction hidden")
	}
	d.Bindings = []ProjectBinding{positive, other}
	first, err := reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
	if err != nil {
		t.Fatal(err)
	}
	q.Scope.Coordinates = b.Coordinates
	second, err := reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != "similar" || second.State != "not_applicable" || first.Scope.Coordinates == second.Scope.Coordinates {
		t.Fatal("different projects became a within-project conflict")
	}
}
func TestRustTextAndL2OrderingRemainDistinctFromUnsupportedASTOrConsensus(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := coverageRawFixture(t, "rollup-fixture", map[string]string{"sequence.rs": "pub fn choose_next_transaction() -> u64 { 7 }\n"})
	b := proposedBinding(t, s, v, refs, "ordering", "Transaction sequencing")
	b.Concept = s.Resolve("sequencing").Candidates[0].Key
	b.Conditions.ComponentRole = "sequencer"
	b.Evidence[0].Facets = []string{"ordering"}
	reader, err := NewMappingReader(ctx, bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	q := coverageQuery(t, s, v, b)
	q.Concept = b.Concept
	q.Facet = "ordering"
	q.IncludeProposed = true
	d := BindingDocument{SchemaVersion: 2, Bindings: []ProjectBinding{b}}
	at := reviewInstant(t, "2026-10-17T00:00:00Z")
	got, err := reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "similar" || !strings.Contains(got.Mappings[0].Binding.Evidence[0].Body, "choose_next_transaction") {
		t.Fatal("unsupported AST prevented valid retained text read")
	}
	q.RequiredCapability = "rust-ast"
	unsupported, err := reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
	if err != nil || unsupported.State != "unsupported" || len(unsupported.Mappings) != 0 {
		t.Fatal("reader claimed Rust AST support")
	}
	q.RequiredCapability = "retained-source"
	q.Concept = s.Resolve("consensus").Candidates[0].Key
	q.Facet = "quorum"
	consensus, err := reader.Coverage(ctx, "chains", q, d, emptyMappingReviews(), at)
	if err != nil || consensus.State != "incomparable" || len(consensus.Mappings) != 0 {
		t.Fatal("sequencer treated as L1 consensus")
	}
}
func TestSchema1DigestAndCrossScopeRejectionsStayStable(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	b := proposedBinding(t, s, v, refs, "staking", "Stake")
	type legacyBinding struct {
		ID           string                 `json:"id"`
		Concept      ConceptKey             `json:"concept"`
		Coordinates  contract.V2Coordinates `json:"coordinates"`
		Conditions   Conditions             `json:"conditions"`
		LocalConcept string                 `json:"local_concept"`
		Mapping      string                 `json:"mapping"`
		Reason       string                 `json:"reason"`
		Evidence     []BindingEvidence      `json:"evidence"`
	}
	data, err := json.Marshal(legacyBinding{b.ID, b.Concept, b.Coordinates, b.Conditions, b.LocalConcept, b.Mapping, b.Reason, b.Evidence})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(append([]byte("cks.project-concept-binding.v1\x00"), data...))
	current, err := s.BindingDigest(b)
	if err != nil || current != hex.EncodeToString(sum[:]) {
		t.Fatal("old schema1 binding digest changed")
	}
	reader, err := NewMappingReader(context.Background(), bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	q := coverageQuery(t, s, v, b)
	q.Scope.Coordinates.ProjectID = "unregistered"
	d := BindingDocument{SchemaVersion: 1, Bindings: []ProjectBinding{b}}
	got, err := reader.Coverage(context.Background(), "chains", q, d, emptyMappingReviews(), reviewInstant(t, "2026-10-17T00:00:00Z"))
	if err == nil || len(got.Mappings) != 0 {
		t.Fatal("foreign scope became an unknown or fallback project")
	}
	q = coverageQuery(t, s, v, b)
	q.Facet = "not-a-facet"
	if _, err = reader.Coverage(context.Background(), "chains", q, d, emptyMappingReviews(), reviewInstant(t, "2026-10-17T00:00:00Z")); !errors.Is(err, ErrBindingInvalid) {
		t.Fatal("unknown facet accepted")
	}
}
func TestCoverageDiscardsEvidenceWhenRetainedArchiveBecomesUnavailable(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	b := proposedBinding(t, s, v, refs, "staking", "Stake")
	reader, err := NewMappingReader(ctx, bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	blob := filepath.Join(v.VersionDir, "sources/blobs", refs[0].Citation.FileSHA256)
	if err = os.Chmod(blob, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(blob, []byte("corrupt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	q := coverageQuery(t, s, v, b)
	q.IncludeProposed = true
	got, err := reader.Coverage(ctx, "chains", q, BindingDocument{SchemaVersion: 2, Bindings: []ProjectBinding{b}}, emptyMappingReviews(), reviewInstant(t, "2026-10-17T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "unavailable" || len(got.Mappings) != 0 || got.ArchiveVerified {
		t.Fatal("unavailable archive yielded healthy evidence")
	}
}

func TestDeclaredUnknownAndRejectedReviewsNeverBecomeFeatureAbsence(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := coverageRawFixture(t, "chain-undecided", map[string]string{"POLICY.md": "The staking withdrawal policy is not decided in this fixture scope.\n"})
	refs[0].Facets = []string{"exit"}
	b := proposedBinding(t, s, v, refs, "staking-policy-status", "Staking withdrawal applicability")
	b.Mapping = "related"
	b.Applicability = "unknown"
	reader, err := NewMappingReader(ctx, bindingCatalog(v), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	q := coverageQuery(t, s, v, b)
	q.Facet = "exit"
	q.IncludeProposed = true
	d := BindingDocument{SchemaVersion: 2, Bindings: []ProjectBinding{b}}
	at := reviewInstant(t, "2026-10-17T00:00:00Z")
	approve := fixtureMappingReview(t, s, b, "approve-unknown", "controlled-a", "verified", "2026-10-16T01:00:00Z")
	reject := fixtureMappingReview(t, s, b, "reject-unknown", "controlled-b", "rejected", "2026-10-16T02:00:00Z")
	withdraw := fixtureMappingReview(t, s, b, "withdraw-unknown", "controlled-a", "withdrawn", "2026-10-16T03:00:00Z")
	cases := []struct {
		name    string
		records []MappingReview
		count   int
	}{{"proposed undecided", nil, 1}, {"reviewed undecided", []MappingReview{approve}, 1}, {"rejected is not absence", []MappingReview{reject}, 0}, {"withdrawn is not absence", []MappingReview{approve, withdraw}, 0}, {"disputed is not absence", []MappingReview{approve, reject}, 0}}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			reviews := emptyMappingReviews()
			reviews.Records = test.records
			result, err := reader.Coverage(ctx, "chains", q, d, reviews, at)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != "unknown" || len(result.Mappings) != test.count {
				t.Fatalf("review or uncertainty became feature absence: %+v", result)
			}
			if len(result.Mappings) > 0 && !strings.Contains(result.Mappings[0].Binding.Evidence[0].Body, "not decided") {
				t.Fatal("missing declared uncertainty source")
			}
		})
	}
}
