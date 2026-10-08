package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Exercise the registered public JSON-RPC surface, not just a helper guard.
func publicMaintenanceCall(t *testing.T, d Deps, name string, args map[string]any) map[string]any {
	t.Helper()
	s := mcpserver.NewMCPServer("maintenance-test", "test")
	if err := Register(s, d); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	response := s.HandleMessage(context.Background(), raw)
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	result, ok := wire["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing public tool result: %s", encoded)
	}
	return result
}

func maintenanceTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			out[rel] = "link:" + target
			return err
		}
		data, err := os.ReadFile(path)
		out[rel] = fmt.Sprintf("%x", sha256.Sum256(data))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPublicMaintenanceRejectsVersionedWrites(t *testing.T) {
	for _, variant := range []string{"pinned", "missing_identity", "mixed_pin", "current_alias", "without_server_pin"} {
		for _, mode := range []string{"incremental", "full"} {
			t.Run(variant+"/"+mode, func(t *testing.T) {
				version := newPinnedV2ErrorFixture(t)
				dataset := filepath.Dir(version)
				if err := os.Symlink(filepath.Base(version), filepath.Join(dataset, "current")); err != nil {
					t.Fatal(err)
				}
				if variant == "missing_identity" {
					if err := os.Remove(filepath.Join(version, "dataset-identity.json")); err != nil {
						t.Fatal(err)
					}
				}
				if variant == "mixed_pin" {
					mutatePinnedManifest(t, version, "project_id", "other-project")
				}
				out := version
				if variant == "current_alias" {
					out = filepath.Join(dataset, "current")
				}
				f := newFixture(t, nil)
				f.deps.EvidenceVersionDir = version
				if variant == "without_server_pin" {
					f.deps.EvidenceVersionDir = ""
				}
				f.deps.Index = IndexConfig{CKGBinary: "graph", CKVBinary: "vector", CKGDataPath: filepath.Join(out, "graph", "graph.db"), CKVDataPath: filepath.Join(out, "vector"), DomainProjectDir: "must-not-export", DomainCorpusDir: filepath.Join(version, "corpus")}
				calls := withStubRunner(t, "")
				before := maintenanceTree(t, dataset)
				result := publicMaintenanceCall(t, f.deps, ToolNameOpsIndex, map[string]any{"mode": mode})
				if result["isError"] != true {
					t.Fatalf("versioned writer accepted: %+v", result)
				}
				encoded, _ := json.Marshal(result)
				if !strings.Contains(string(encoded), "versioned_setup_required") || strings.Contains(string(encoded), version) {
					t.Fatalf("missing safe migration error: %s", encoded)
				}
				if len(*calls) != 0 || !reflect.DeepEqual(before, maintenanceTree(t, dataset)) {
					t.Fatal("rejected writer mutated current, identity, corpus or retained bytes")
				}
			})
		}
	}
}

func TestPublicAsyncMaintenanceCannotBypassPinnedGuard(t *testing.T) {
	version := newPinnedV2ErrorFixture(t)
	dataset := filepath.Dir(version)
	if err := os.Symlink(filepath.Base(version), filepath.Join(dataset, "current")); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t, nil)
	f.deps.Setup.Jobs = setup.NewJobs(nil)
	t.Cleanup(f.deps.Setup.Jobs.Shutdown)
	before := maintenanceTree(t, dataset)
	for _, tc := range []struct{ name, out, ver string }{
		{ToolNameOpsSetup, version, ""},
		{ToolNameOpsSetup, filepath.Join(dataset, "current"), ""},
		{ToolNameOpsReindex, dataset, filepath.Base(version)},
		{ToolNameOpsReindex, dataset, "new-unpinned"},
	} {
		result := publicMaintenanceCall(t, f.deps, tc.name, map[string]any{"src": t.TempDir(), "out": tc.out, "version": tc.ver})
		if result["isError"] != true {
			t.Fatalf("async bypass accepted (%s): %+v", tc.name, result)
		}
		if !reflect.DeepEqual(before, maintenanceTree(t, dataset)) {
			t.Fatal("async rejection mutated dataset")
		}
	}
}

func TestPublicFlatLegacyMaintenanceRemainsAvailable(t *testing.T) {
	for _, mode := range []string{"incremental", "full"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			graph, vector := filepath.Join(root, "graph"), filepath.Join(root, "vector")
			writeManifests(t, graph, vector, "abc", "same", "abc", "same")
			f := newFixture(t, nil)
			f.deps.Index = IndexConfig{CKGBinary: "graph", CKVBinary: "vector", CKGDataPath: filepath.Join(graph, "graph.db"), CKVDataPath: vector}
			calls := withStubRunner(t, "")
			result := publicMaintenanceCall(t, f.deps, ToolNameOpsIndex, map[string]any{"mode": mode})
			if result["isError"] == true || len(*calls) != 2 {
				t.Fatalf("flat legacy rejected: %+v %v", result, *calls)
			}
			content, ok := result["structuredContent"].(map[string]any)
			if !ok || content["mode"] != mode || content["alignment"].(map[string]any)["ok"] != true {
				t.Fatalf("legacy response changed: %+v", result)
			}
		})
	}
}
