package comparison

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runtimeFixture(t *testing.T) (*ComparisonRuntime, string, ComparisonRequest, collectionCatalogFixture) {
	t.Helper()
	a, ar := bindingFixture(t, "chain-a", "Stake")
	b, br := bindingFixture(t, "chain-b", "Bond")
	catalog := bindingCatalog(a, b)
	schemeRoot := copyScheme(t)
	schemeRoot, errCanonical := filepath.EvalSymlinks(schemeRoot)
	if errCanonical != nil {
		t.Fatal(errCanonical)
	}
	scheme, err := LoadScheme(schemeRoot)
	if err != nil {
		t.Fatal(err)
	}
	first := proposedBinding(t, scheme, a, ar, "staking-a", "Stake")
	second := proposedBinding(t, scheme, b, br, "staking-b", "Bond")
	dir := t.TempDir()
	dir, errCanonical = filepath.EvalSymlinks(dir)
	if errCanonical != nil {
		t.Fatal(errCanonical)
	}
	var errWrite error
	write := func(name string, value any) string {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err = os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	catalogPath := write("catalog.json", catalog)
	bindingsPath := write("bindings.json", BindingDocument{SchemaVersion: 2, Bindings: []ProjectBinding{first, second}})
	rulesPath := filepath.Join(dir, "rules.yaml")
	errWrite = os.WriteFile(rulesPath, []byte("version: 1\nrules:\n  - id: secret\n    description: fixture\n    pattern: SECRET\n    action: mask\n    severity: high\n"), 0o600)
	if errWrite != nil {
		t.Fatal(errWrite)
	}
	scopes := []ProjectDomainScope{{Coordinates: a.Coordinates, Industry: "blockchain", Conditions: first.Conditions}, {Coordinates: b.Coordinates, Industry: "blockchain", Conditions: second.Conditions}}
	profilePath := write("profile.json", ComparisonProfileDocument{SchemaVersion: 1, Name: "comparison-test", CatalogPath: catalogPath, SchemeRoot: schemeRoot, BindingsPath: bindingsPath, SanitizeRulesPath: rulesPath, Scopes: scopes})
	runtime, err := OpenComparisonRuntime(context.Background(), profilePath)
	if err != nil {
		t.Fatal(err)
	}
	runtime.now = func() time.Time { return protocolNow(t) }
	t.Cleanup(func() { _ = runtime.Close() })
	ids := []string{}
	for _, scope := range scopes {
		id, err := ComparisonScopeID(scope)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	q := ComparisonRequest{SchemaVersion: 1, ProfileDigest: runtime.protocol.ProfileDigest(), CatalogDigest: runtime.protocol.CatalogDigest(), CollectionID: "chains", Mode: "comparison", Participants: ids, Concept: first.Concept, Facets: []string{"entry", "exit"}, AsOf: "2026-10-17T00:00:00Z", Budget: DefaultComparisonBudget(), IncludeProposed: true}
	return runtime, profilePath, q, collectionCatalogFixture{BindingsPath: bindingsPath, Versions: []collection.Version{a, b}}
}

type collectionCatalogFixture struct {
	BindingsPath string
	Versions     []collection.Version
}

func executeFixture(t *testing.T, r *ComparisonRuntime, q ComparisonRequest) ComparisonBundle {
	t.Helper()
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Extract(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRuntimeLoadsPinnedInputsAndReturnsProjectFacetMatrix(t *testing.T) {
	r, _, q, _ := runtimeFixture(t)
	description, err := r.DescribeRequest(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if description.State != "configured" || description.SourceReadiness != "not_checked" || description.WriteEnabled || len(description.Scopes) != 2 || len(description.Concepts) != 8 {
		t.Fatal("configured metadata became serving readiness")
	}
	result := executeFixture(t, r, q)
	if result.Mode != "comparison" || len(result.Groups) != 2 || result.State != "partial" || result.UsedBodyBytes == 0 || result.UsedCitations != 10 || len(result.Candidates) == 0 {
		t.Fatalf("incomplete matrix %+v", result)
	}
	for i, group := range result.Groups {
		if len(group.Cells) != 2 || group.Cells[0].State != "similar" || group.Cells[1].State != "unknown" {
			t.Fatal("missing facet hidden")
		}
		want := "Stake"
		foreign := "Bond"
		if i == 1 {
			want, foreign = foreign, want
		}
		for _, m := range group.Cells[0].Mappings {
			if m.Binding.ReviewState != "proposed" {
				t.Fatal("invented human review")
			}
			for _, e := range m.Binding.Evidence {
				if strings.Contains(e.Body, foreign) || e.Reference.Citation.ProjectID != group.Participant.Scope.Coordinates.ProjectID {
					t.Fatal("foreign source context")
				}
			}
		}
		if !strings.Contains(group.Cells[0].Mappings[0].Binding.Evidence[0].Body, want) {
			t.Fatal("actual source body missing")
		}
	}
	encoded, err := json.Marshal(description)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "version_dir") || strings.Contains(string(encoded), "catalog_path") {
		t.Fatal("local configuration paths exposed")
	}
}
func TestRuntimeKeepsInputSnapshotAndRejectsRestartStalePins(t *testing.T) {
	r, path, q, fixture := runtimeFixture(t)
	before := executeFixture(t, r, q)
	if err := os.WriteFile(fixture.BindingsPath, []byte(`{"schema_version":2,"bindings":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	after := executeFixture(t, r, q)
	if before.UsedBodyBytes != after.UsedBodyBytes || before.ProfileDigest != after.ProfileDigest {
		t.Fatal("running profile silently reloaded")
	}
	fresh, err := OpenComparisonRuntime(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fresh.Close() }()
	fresh.now = r.now
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fresh.Extract(context.Background(), data); !errors.Is(err, ErrComparisonStale) {
		t.Fatal("changed configuration accepted old request pins")
	}
}
func TestRuntimeBudgetAndSourceFailureDoNotChangeParticipantSelection(t *testing.T) {
	r, _, q, fixture := runtimeFixture(t)
	q.Budget.MaxBodyBytes = 1
	small := executeFixture(t, r, q)
	if small.UsedBodyBytes != 0 || small.UsedCitations != 0 || small.Groups[0].Cells[0].State != "budget_exceeded" || len(small.Groups[0].Cells[0].Mappings) != 0 {
		t.Fatal("budget silently returned truncated evidence")
	}
	q.Budget = DefaultComparisonBudget()
	if err := os.Remove(filepath.Join(fixture.Versions[1].VersionDir, "sources/manifest.json")); err != nil {
		t.Fatal(err)
	}
	partial := executeFixture(t, r, q)
	if partial.Groups[0].Cells[0].State != "similar" || partial.Groups[1].Cells[0].State != "unavailable" || len(partial.Groups[1].Cells[0].Mappings) != 0 || partial.Groups[0].Participant.ScopeID != q.Participants[0] || partial.Groups[1].Participant.ScopeID != q.Participants[1] {
		t.Fatal("unavailable participant blocked or replaced another project")
	}
}
func TestRuntimeRefusesSymlinkProfileAndClosedOrCanceledReads(t *testing.T) {
	r, path, q, _ := runtimeFixture(t)
	link := filepath.Join(t.TempDir(), "profile.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenComparisonRuntime(context.Background(), link); !errors.Is(err, ErrComparisonProfile) {
		t.Fatal("linked profile accepted")
	}
	if _, err := r.DescribeRequest(context.Background(), []byte(`{"config_path":"/private/elsewhere"}`)); !errors.Is(err, ErrComparisonRequest) {
		t.Fatal("description rewrote configuration")
	}
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = r.Extract(ctx, data); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Extract(context.Background(), data); !errors.Is(err, ErrComparisonClosed) {
		t.Fatal("closed runtime served source")
	}
}
