package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func collectionFixture(t *testing.T) (*fixture, collection.Catalog, *sanitize.Engine) {
	t.Helper()
	f := newFixture(t, nil)
	dir, err := filepath.EvalSymlinks(newPinnedV2ErrorFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	id, err := setup.InspectVersionIdentity(dir)
	if err != nil || id == nil {
		t.Fatal(err)
	}
	coords := contract.V2Coordinates{ProjectID: id.Source.ProjectID, DatasetID: id.DatasetID,
		SnapshotID: id.Source.SnapshotID, SourceMode: id.Source.SourceMode, BaseCommit: id.Source.SourceCommit}
	cleaner, err := sanitize.New(f.ruleset)
	if err != nil {
		t.Fatal(err)
	}
	f.deps.EvidenceVersionDir, f.deps.EvidenceSanitizer = dir, cleaner
	catalog := collection.Catalog{SchemaVersion: 1, Projects: []collection.Project{{ID: coords.ProjectID, Name: "fixture"}},
		Versions:    []collection.Version{{Coordinates: coords, VersionDir: dir, Origins: []string{"repo"}}},
		Collections: []collection.Collection{{ID: "fixture", Members: []contract.V2Coordinates{coords}}}}
	return f, catalog, cleaner
}

func TestCollectionToolsAreOptInAndReadOnly(t *testing.T) {
	f, catalog, cleaner := collectionFixture(t)
	srv := mcpserver.NewMCPServer("collection-test", "test")
	if err := Register(srv, f.deps); err != nil {
		t.Fatal(err)
	}
	if srv.GetTool(ToolNameCollectionContext) != nil || srv.GetTool(ToolNameCollectionStatus) != nil {
		t.Fatal("default tools changed without opt-in")
	}
	service, err := NewCollectionService(context.Background(), catalog, cleaner,
		func(context.Context, collection.Version) (Deps, func(), error) { return f.deps, nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	f.deps.Collection = service
	configured := mcpserver.NewMCPServer("collection-test", "test")
	if err := Register(configured, f.deps); err != nil {
		t.Fatal(err)
	}
	if len(configured.ListTools()) != len(srv.ListTools())+2 {
		t.Fatal("opt-in must add exactly two collection tools")
	}
	for _, name := range []string{ToolNameCollectionContext, ToolNameCollectionStatus} {
		tool := configured.GetTool(name)
		if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
			t.Fatal("missing read-only annotation")
		}
	}
	result := publicMaintenanceCall(t, f.deps, ToolNameCollectionStatus, map[string]any{})
	raw, err := json.Marshal(result)
	if err != nil || strings.Contains(string(raw), catalog.Versions[0].VersionDir) || strings.Contains(string(raw), "version_dir") {
		t.Fatal("status exposed local paths")
	}
}

func TestCollectionServiceUsesExistingV2AndClosesOnce(t *testing.T) {
	f, catalog, cleaner := collectionFixture(t)
	closed := 0
	service, err := NewCollectionService(context.Background(), catalog, cleaner,
		func(context.Context, collection.Version) (Deps, func(), error) {
			return f.deps, func() { closed++ }, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	r := collection.Request{CatalogDigest: service.session.Digest(), CollectionID: "fixture", Mode: collection.Implementation,
		Query: "Alpha", Target: &catalog.Versions[0].Coordinates}
	out, err := service.read(context.Background(), r)
	if err != nil || len(out.Groups) != 1 || out.Groups[0].Coordinates != catalog.Versions[0].Coordinates {
		t.Fatalf("v2 route failed: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil || closed != 1 {
		t.Fatal("backend close not idempotent")
	}
	if _, err := service.read(context.Background(), r); !errors.Is(err, collection.ErrClosed) {
		t.Fatal("closed service read succeeded")
	}
}

func TestCollectionFactoryFailureClosesOpenedBackend(t *testing.T) {
	f, catalog, cleaner := collectionFixture(t)
	closed := false
	_, err := NewCollectionService(context.Background(), catalog, cleaner,
		func(context.Context, collection.Version) (Deps, func(), error) {
			return f.deps, func() { closed = true }, errors.New("internal /private/path must not escape")
		})
	if !errors.Is(err, collection.ErrDatasetMismatch) || !closed || strings.Contains(err.Error(), "/private/") {
		t.Fatal("failed startup leaked resources/details")
	}
}

func TestCollectionPublicUnknownAndMissingTargetsAreRejected(t *testing.T) {
	f, catalog, cleaner := collectionFixture(t)
	service, err := NewCollectionService(context.Background(), catalog, cleaner,
		func(context.Context, collection.Version) (Deps, func(), error) { return f.deps, nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	f.deps.Collection = service
	for _, arguments := range []map[string]any{
		{"catalog_digest": service.session.Digest(), "collection_id": "fixture", "mode": "implementation", "query": "Alpha"},
		{"catalog_digest": service.session.Digest(), "collection_id": "fixture", "mode": "implementation", "query": "Alpha", "config_path": "/private/unregistered"},
	} {
		response := publicMaintenanceCall(t, f.deps, ToolNameCollectionContext, arguments)
		if response["isError"] != true {
			t.Fatal("malformed public query was accepted")
		}
	}
}
