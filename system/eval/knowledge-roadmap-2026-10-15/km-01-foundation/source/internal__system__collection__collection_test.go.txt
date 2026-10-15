package collection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func fixture(t *testing.T, project, content string, committed bool) Version {
	t.Helper()
	root, dataset := t.TempDir(), t.TempDir()
	dir := filepath.Join(dataset, "v1")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var err error
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	mode, commit := "snapshot-only", ""
	if committed {
		for _, args := range [][]string{{"init", "-q"}, {"add", "code.go"},
			{"-c", "user.name=Collection fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "test fixture"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("fixture git: %v: %s", err, output)
			}
		}
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = root
		output, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		mode, commit = "committed", strings.TrimSpace(string(output))
	}
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: root, Out: dir, ProjectID: project, SourceMode: mode, SourceCommit: commit})
	if err != nil {
		t.Fatal(err)
	}
	id, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"fixture"}`), "fixture-input")
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range []string{"graph", "vector"} {
		engineDir := filepath.Join(dir, engine)
		if err := os.Mkdir(engineDir, 0o700); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(map[string]any{"src_root": root, "src_commit": commit, "graph_digest": "fixture", "schema_version": "1.23",
			"embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "fixture",
			"project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID,
			"dataset_id": id.DatasetID, "source_mode": mode, "file_manifest_digest": captured.Identity.FileManifestDigest,
			"capture_policy_digest": captured.Identity.CapturePolicyDigest})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(engineDir, "manifest.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := setup.PublishCandidateIdentity(dir, captured.Identity, "fixture-input"); err != nil {
		t.Fatal(err)
	}
	return Version{Coordinates: contract.V2Coordinates{ProjectID: project, DatasetID: id.DatasetID,
		SnapshotID: captured.Identity.SnapshotID, SourceMode: mode, BaseCommit: commit}, VersionDir: dir, Origins: []string{"repo"}}
}

func cleaner(t *testing.T) *sanitize.Engine {
	t.Helper()
	rules, err := config.LoadSanitizeRulesetBytes([]byte("version: 1\nrules:\n  - id: secret\n    description: fixture\n    pattern: SECRET\n    action: mask\n    severity: high\n"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := sanitize.New(rules)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func catalog(versions ...Version) Catalog {
	c := Catalog{SchemaVersion: SchemaVersion, Versions: versions, Collections: []Collection{{ID: "chains"}}}
	seen := map[string]bool{}
	for _, v := range versions {
		if !seen[v.Coordinates.ProjectID] {
			c.Projects = append(c.Projects, Project{ID: v.Coordinates.ProjectID, Name: "same-name", SourceURI: "https://example.invalid/shared"})
			seen[v.Coordinates.ProjectID] = true
		}
		c.Collections[0].Members = append(c.Collections[0].Members, v.Coordinates)
	}
	return c
}

func open(t *testing.T, c Catalog) *Session {
	t.Helper()
	s, err := Open(context.Background(), c, cleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func request(s *Session, target contract.V2Coordinates) Request {
	return Request{CatalogDigest: s.Digest(), CollectionID: "chains", Mode: Implementation, Query: "same query", Target: &target}
}

func fetcher(t *testing.T) Fetcher {
	t.Helper()
	c := cleaner(t)
	return func(ctx context.Context, v Version, query string) (contract.EvidencePackV2, error) {
		return evidencev2.Build(ctx, v.VersionDir, query, []contract.Citation{{File: "code.go", StartLine: 1, EndLine: 1}}, c)
	}
}

func TestSameCommitForksAndNamesRemainSeparate(t *testing.T) {
	a, b := fixture(t, "project-a", "package same\n", true), fixture(t, "project-b", "package same\n", true)
	if a.Coordinates.BaseCommit != b.Coordinates.BaseCommit || a.Coordinates.SnapshotID == b.Coordinates.SnapshotID {
		t.Fatal("fixture must have identical commits/code and different project snapshots")
	}
	c := catalog(a, b)
	c.Projects[1].UpstreamProjectID = "project-a"
	s := open(t, c)
	r := Request{CatalogDigest: s.Digest(), CollectionID: "chains", Mode: Comparison, Query: "same query",
		Participants: []contract.V2Coordinates{a.Coordinates, b.Coordinates}}
	out, err := s.Read(context.Background(), r, fetcher(t))
	if err != nil || len(out.Groups) != 2 || out.Groups[0].Coordinates != a.Coordinates || out.Groups[1].Coordinates != b.Coordinates {
		t.Fatalf("fork groups mixed: %+v %v", out, err)
	}
	if out.Groups[0].Evidence.Citations[0].KeyV2() == out.Groups[1].Evidence.Citations[0].KeyV2() {
		t.Fatal("same code merged across projects")
	}
}

func TestImplementationDispatchesOnlyExplicitTarget(t *testing.T) {
	a, b := fixture(t, "a", "package target\n", false), fixture(t, "b", "package foreign\n", false)
	s := open(t, catalog(a, b))
	calls := 0
	fetch := fetcher(t)
	out, err := s.Read(context.Background(), request(s, a.Coordinates), func(ctx context.Context, v Version, q string) (contract.EvidencePackV2, error) {
		calls++
		if v.Coordinates != a.Coordinates {
			t.Fatal("foreign project dispatched")
		}
		return fetch(ctx, v, q)
	})
	if err != nil || calls != 1 || len(out.Groups) != 1 || out.Groups[0].Evidence.Bodies[0].Text != "package target\n" {
		t.Fatalf("implementation isolation: %+v %v", out, err)
	}
}

func TestTwoVersionsAndWarmProviderCannotCrossTargets(t *testing.T) {
	a, b := fixture(t, "same", "package before\n", false), fixture(t, "same", "package after\n", false)
	s := open(t, catalog(a, b))
	fetch := fetcher(t)
	first, err := s.Read(context.Background(), request(s, a.Coordinates), fetch)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a warm provider cache incorrectly keyed only by query text.
	out, err := s.Read(context.Background(), request(s, b.Coordinates), func(context.Context, Version, string) (contract.EvidencePackV2, error) {
		return first.Groups[0].Evidence, nil
	})
	if !errors.Is(err, ErrEvidenceMismatch) || len(out.Groups) != 0 {
		t.Fatal("warm foreign-version cache response escaped")
	}
	out, err = s.Read(context.Background(), request(s, b.Coordinates), fetch)
	if err != nil || out.Groups[0].Evidence.Bodies[0].Text != "package after\n" {
		t.Fatalf("version selection failed: %v", err)
	}
	left, _ := s.ScopeKey(request(s, a.Coordinates))
	right, _ := s.ScopeKey(request(s, b.Coordinates))
	if left == right {
		t.Fatal("routing namespace merged versions")
	}
}

func TestRequestRejectionsNeverDispatch(t *testing.T) {
	a, b := fixture(t, "a", "package a\n", false), fixture(t, "b", "package b\n", false)
	s := open(t, catalog(a, b))
	base := request(s, a.Coordinates)
	cases := []struct {
		name string
		edit func(*Request)
		want error
	}{
		{"missing target", func(r *Request) { r.Target = nil }, ErrTargetRequired},
		{"missing catalog pin", func(r *Request) { r.CatalogDigest = "" }, ErrStaleCatalog},
		{"stale catalog", func(r *Request) { r.CatalogDigest = strings.Repeat("0", 64) }, ErrStaleCatalog},
		{"missing collection", func(r *Request) { r.CollectionID = "unknown" }, ErrScopeMismatch},
		{"wrong dataset", func(r *Request) { r.Target.DatasetID = strings.Repeat("f", 64) }, ErrScopeMismatch},
		{"wrong commit", func(r *Request) { r.Target.BaseCommit = strings.Repeat("a", 40) }, ErrScopeMismatch},
		{"foreign project", func(r *Request) { r.Target.ProjectID = "foreign" }, ErrScopeMismatch},
		{"implicit current", func(r *Request) { r.Target.DatasetID = "current" }, ErrScopeMismatch},
		{"empty query", func(r *Request) { r.Query = " " }, ErrScopeMismatch},
		{"unsupported mode", func(r *Request) { r.Mode = "auto" }, ErrUnsupportedMode},
		{"implementation with references", func(r *Request) { r.Participants = []contract.V2Coordinates{b.Coordinates} }, ErrScopeMismatch},
		{"comparison with patch target", func(r *Request) {
			r.Mode = Comparison
			r.Participants = []contract.V2Coordinates{a.Coordinates, b.Coordinates}
		}, ErrScopeMismatch},
		{"comparison without participants", func(r *Request) { r.Mode = Comparison; r.Target = nil }, ErrScopeMismatch},
		{"duplicate comparison member", func(r *Request) {
			r.Mode, r.Target = Comparison, nil
			r.Participants = []contract.V2Coordinates{a.Coordinates, a.Coordinates}
		}, ErrScopeMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			coords := *base.Target
			r.Target = &coords
			tc.edit(&r)
			out, err := s.Read(context.Background(), r, func(context.Context, Version, string) (contract.EvidencePackV2, error) {
				t.Fatal("rejected request called a backend")
				return contract.EvidencePackV2{}, nil
			})
			if !errors.Is(err, tc.want) || len(out.Groups) != 0 {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCatalogRoundtripRestartAndCallerMutation(t *testing.T) {
	a, b := fixture(t, "a", "package a\n", false), fixture(t, "b", "package b\n", false)
	c := catalog(a, b)
	s := open(t, c)
	raw, err := c.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(bytes.NewReader(stored))
	if err != nil {
		t.Fatal(err)
	}
	reopened := open(t, loaded)
	if s.Digest() != reopened.Digest() {
		t.Fatal("restart changed catalog pin")
	}
	loaded.Versions[0].Origins[0] = "knowledge:foreign.code"
	loaded.Collections[0].Members[0] = b.Coordinates
	out, err := reopened.Read(context.Background(), request(reopened, a.Coordinates), fetcher(t))
	if err != nil || out.Groups[0].Coordinates != a.Coordinates {
		t.Fatal("caller mutated an opened catalog")
	}
	if _, err := s.ScopeKey(request(s, a.Coordinates)); !errors.Is(err, ErrClosed) {
		t.Fatal("closed session accepted a request")
	}
}

func TestCatalogMembershipDigestChangesAndOrderDoesNot(t *testing.T) {
	a, b := fixture(t, "a", "package a\n", false), fixture(t, "b", "package b\n", false)
	c := catalog(a, b)
	before, err := c.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	c.Projects[0], c.Projects[1] = c.Projects[1], c.Projects[0]
	c.Versions[0], c.Versions[1] = c.Versions[1], c.Versions[0]
	c.Collections[0].Members[0], c.Collections[0].Members[1] = c.Collections[0].Members[1], c.Collections[0].Members[0]
	after, err := c.CanonicalBytes()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("array order changed membership identity")
	}
	c.Collections[0].Members = c.Collections[0].Members[:1]
	changed, err := c.CanonicalBytes()
	if err != nil || bytes.Equal(before, changed) {
		t.Fatal("membership change kept old catalog identity")
	}
}

func TestDuplicateRegistrationAndForkErrors(t *testing.T) {
	a, b := fixture(t, "a", "package a\n", false), fixture(t, "b", "package b\n", false)
	cases := map[string]func(*Catalog){
		"duplicate project": func(c *Catalog) { c.Projects = append(c.Projects, c.Projects[0]) },
		"duplicate version": func(c *Catalog) { c.Versions = append(c.Versions, c.Versions[0]) },
		"rebound dataset":   func(c *Catalog) { c.Versions[1].Coordinates = c.Versions[0].Coordinates },
		"shared directory":  func(c *Catalog) { c.Versions[1].VersionDir = c.Versions[0].VersionDir },
		"duplicate member": func(c *Catalog) {
			c.Collections[0].Members = append(c.Collections[0].Members, c.Collections[0].Members[0])
		},
		"unknown upstream": func(c *Catalog) { c.Projects[1].UpstreamProjectID = "unknown" },
		"fork cycle":       func(c *Catalog) { c.Projects[0].UpstreamProjectID = "b"; c.Projects[1].UpstreamProjectID = "a" },
		"duplicate origin": func(c *Catalog) { c.Versions[0].Origins = []string{"repo", "repo"} },
		"bad origin":       func(c *Catalog) { c.Versions[0].Origins = []string{"repo", "docs:corpus.01"} },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			c := catalog(a, b)
			edit(&c)
			if _, err := c.CanonicalBytes(); !errors.Is(err, ErrCatalogInvalid) {
				t.Fatalf("invalid registration allowed: %v", err)
			}
		})
	}
}

func TestStrictCatalogDecode(t *testing.T) {
	a := fixture(t, "a", "package a\n", false)
	raw, err := catalog(a).CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{
		append(append([]byte(nil), raw...), []byte(" {}")...),
		bytes.Replace(raw, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		bytes.Replace(raw, []byte(`"schema_version":1`), []byte(`"schema_version":1,"unknown":true`), 1),
		bytes.Repeat([]byte(" "), MaxCatalogBytes+1),
	} {
		if _, err := Decode(bytes.NewReader(invalid)); !errors.Is(err, ErrCatalogInvalid) {
			t.Fatalf("unsafe catalog decoded: %v", err)
		}
	}
}

func TestScopedLocalSymbolKeysKeepProjectVersionAndOrigin(t *testing.T) {
	a, b := fixture(t, "a", "package same\n", false), fixture(t, "b", "package same\n", false)
	ref := ScopedRef{Coordinates: a.Coordinates, OriginID: "repo", Path: "code.go", LocalID: "example.invalid/module.Same"}
	first, err := ref.Key()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{first: true}
	for _, edit := range []func(*ScopedRef){
		func(r *ScopedRef) { r.Coordinates = b.Coordinates },
		func(r *ScopedRef) { r.Coordinates.DatasetID = strings.Repeat("a", 64) },
		func(r *ScopedRef) { r.OriginID = "docs:corpus.0" },
		func(r *ScopedRef) { r.OriginID = "knowledge:sample.code" },
	} {
		changed := ref
		edit(&changed)
		key, err := changed.Key()
		if err != nil || seen[key] {
			t.Fatal("scoped symbol references collided")
		}
		seen[key] = true
	}
	for _, path := range []string{"../code.go", "/code.go", "a/../code.go", `a\code.go`} {
		ref.Path = path
		if _, err := ref.Key(); !errors.Is(err, ErrScopeMismatch) {
			t.Fatal("non-source-relative path accepted")
		}
	}
}

func TestForeignBodyAndHashesRejectedEvenWhenRestamped(t *testing.T) {
	a := fixture(t, "a", "package target // SECRET\n", false)
	s := open(t, catalog(a))
	for _, edit := range []func(*contract.EvidencePackV2){
		func(p *contract.EvidencePackV2) { p.Bodies[0].Text = "package foreign\n" },
		func(p *contract.EvidencePackV2) {
			p.Metadata.Ontology = &contract.OntologyDiagnostic{Mode: "concept_text", State: "active",
				TextSources: []contract.OntologyTextSource{{ProjectID: "foreign-project", File: "code.go", StartLine: 1, EndLine: 1}}}
		},
		func(p *contract.EvidencePackV2) {
			p.Citations[0].FileSHA256 = strings.Repeat("0", 64)
			p.Bodies[0].Citation = p.Citations[0]
		},
		func(p *contract.EvidencePackV2) {
			p.Citations[0].ContentSHA256 = strings.Repeat("0", 64)
			p.Bodies[0].Citation = p.Citations[0]
		},
	} {
		fetch := fetcher(t)
		out, err := s.Read(context.Background(), request(s, a.Coordinates), func(ctx context.Context, v Version, q string) (contract.EvidencePackV2, error) {
			p, err := fetch(ctx, v, q)
			if err != nil {
				return p, err
			}
			edit(&p)
			return p, evidencev2.Stamp(&p)
		})
		if !errors.Is(err, ErrEvidenceMismatch) || len(out.Groups) != 0 {
			t.Fatal("restamped foreign evidence escaped")
		}
	}
	out, err := s.Read(context.Background(), request(s, a.Coordinates), fetcher(t))
	if err != nil || strings.Contains(out.Groups[0].Evidence.Bodies[0].Text, "SECRET") {
		t.Fatalf("legitimate sanitized source rejected: %v", err)
	}
}

func TestCurrentAliasesAndMismatchedArchivesRejected(t *testing.T) {
	a, b := fixture(t, "a", "package a\n", false), fixture(t, "b", "package b\n", false)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(a.VersionDir, alias); err != nil {
		t.Fatal(err)
	}
	c := catalog(a)
	c.Versions[0].VersionDir = alias
	if _, err := Open(context.Background(), c, cleaner(t)); !errors.Is(err, ErrDatasetMismatch) {
		t.Fatal("mutable alias accepted")
	}
	c = catalog(a)
	c.Versions[0].VersionDir = b.VersionDir
	if _, err := Open(context.Background(), c, cleaner(t)); !errors.Is(err, ErrDatasetMismatch) {
		t.Fatal("coordinates relabeled a foreign archive")
	}
	c = catalog(a)
	c.Versions[0].Origins = []string{"repo", "docs:corpus.0"}
	if _, err := Open(context.Background(), c, cleaner(t)); !errors.Is(err, ErrDatasetMismatch) {
		t.Fatal("uncollected source origin accepted")
	}
}

func TestComparisonFailureDiscardsEarlierGroup(t *testing.T) {
	a, b := fixture(t, "a", "package a\n", false), fixture(t, "b", "package b\n", false)
	s := open(t, catalog(a, b))
	fetch := fetcher(t)
	r := Request{CatalogDigest: s.Digest(), CollectionID: "chains", Mode: Comparison, Query: "same query",
		Participants: []contract.V2Coordinates{a.Coordinates, b.Coordinates}}
	calls := 0
	out, err := s.Read(context.Background(), r, func(ctx context.Context, v Version, q string) (contract.EvidencePackV2, error) {
		calls++
		if calls == 2 {
			return contract.EvidencePackV2{}, errors.New("fixture backend failure")
		}
		return fetch(ctx, v, q)
	})
	if !errors.Is(err, ErrFetchFailed) || calls != 2 || len(out.Groups) != 0 {
		t.Fatal("partial comparison was published implicitly")
	}
}

func TestConcurrentTargetReadsAndClose(t *testing.T) {
	a, b := fixture(t, "a", "package a\n", false), fixture(t, "b", "package b\n", false)
	s := open(t, catalog(a, b))
	fetch := fetcher(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			v := []Version{a, b}[i%2]
			out, err := s.Read(context.Background(), request(s, v.Coordinates), fetch)
			if err != nil || len(out.Groups) != 1 || !reflect.DeepEqual(out.Groups[0].Coordinates, v.Coordinates) {
				t.Errorf("concurrent scope mixed: %v", err)
			}
		})
	}
	wg.Wait()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(context.Background(), request(s, a.Coordinates), fetch); !errors.Is(err, ErrClosed) {
		t.Fatal("read after close accepted")
	}
}

func TestCanceledReadDoesNotDispatch(t *testing.T) {
	a := fixture(t, "a", "package a\n", false)
	s := open(t, catalog(a))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := s.Read(ctx, request(s, a.Coordinates), func(context.Context, Version, string) (contract.EvidencePackV2, error) {
		t.Fatal("canceled read dispatched")
		return contract.EvidencePackV2{}, nil
	})
	if !errors.Is(err, context.Canceled) || len(out.Groups) != 0 {
		t.Fatal("cancellation did not stop publication")
	}
}

func TestArchiveMutationDuringLaterComparisonDiscardsAll(t *testing.T) {
	a, b := fixture(t, "a", "package a\n", false), fixture(t, "b", "package b\n", false)
	s := open(t, catalog(a, b))
	fetch := fetcher(t)
	r := Request{CatalogDigest: s.Digest(), CollectionID: "chains", Mode: Comparison, Query: "same query",
		Participants: []contract.V2Coordinates{a.Coordinates, b.Coordinates}}
	out, err := s.Read(context.Background(), r, func(ctx context.Context, v Version, q string) (contract.EvidencePackV2, error) {
		if v.Coordinates == b.Coordinates {
			blobs, err := os.ReadDir(filepath.Join(a.VersionDir, "sources", "blobs"))
			if err != nil || len(blobs) != 1 {
				t.Fatal("source fixture inventory differs")
			}
			blob := filepath.Join(a.VersionDir, "sources", "blobs", blobs[0].Name())
			// The fixture simulates archive damage, including an illicit write
			// to the ordinarily read-only retained blob. Never an engine write.
			if err := os.Chmod(blob, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(blob, []byte("changed archive\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return fetch(ctx, v, q)
	})
	if !errors.Is(err, ErrDatasetMismatch) || len(out.Groups) != 0 {
		t.Fatal("earlier group's changed archive was published")
	}
}

func TestCloseWaitsForInFlightRead(t *testing.T) {
	a := fixture(t, "a", "package a\n", false)
	s := open(t, catalog(a))
	entered, release, finished, closed := make(chan struct{}), make(chan struct{}), make(chan error, 1), make(chan error, 1)
	fetch := fetcher(t)
	go func() {
		_, err := s.Read(context.Background(), request(s, a.Coordinates), func(ctx context.Context, v Version, q string) (contract.EvidencePackV2, error) {
			close(entered)
			<-release
			return fetch(ctx, v, q)
		})
		finished <- err
	}()
	<-entered
	go func() { closed <- s.Close() }()
	select {
	case <-closed:
		close(release)
		t.Fatal("close released leases before the active request")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}

func TestReaderLeaseAndExistingRollbackStayCompatible(t *testing.T) {
	a := fixture(t, "a", "package a\n", false)
	if err := os.WriteFile(filepath.Join(a.VersionDir, "reader-protocol.json"), []byte("{\"protocol\":\"reader-flock-v1\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dataset := filepath.Dir(a.VersionDir)
	if err := setup.Rollback(dataset, filepath.Base(a.VersionDir)); err != nil {
		t.Fatal(err)
	}
	s := open(t, catalog(a))
	if len(s.leases) != 1 || s.leases[0] == nil {
		t.Fatal("reader protocol did not acquire a lease")
	}
	plan, err := setup.PlanGC(context.Background(), dataset, setup.GCPolicy{KeepRecent: 2}, time.Now())
	if err != nil || len(plan.Versions) != 1 {
		t.Fatalf("existing GC failed: %v", err)
	}
	pinned := false
	for _, reason := range plan.Versions[0].Reasons {
		pinned = pinned || reason == "pinned_reader"
	}
	if !pinned || plan.Versions[0].Delete {
		t.Fatal("collection reader not protected from GC")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := setup.Rollback(dataset, filepath.Base(a.VersionDir)); err != nil {
		t.Fatal(err)
	}
	pointer, err := os.Readlink(filepath.Join(dataset, "current"))
	if err != nil || pointer != "v1" {
		t.Fatal("existing current/rollback changed")
	}
	pack, err := fetcher(t)(context.Background(), a, "legacy single-project caller")
	if err != nil || pack.Coordinates != a.Coordinates {
		t.Fatal("existing single-project retained read changed")
	}
}
