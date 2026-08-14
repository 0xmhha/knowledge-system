package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleOpsIndex_DomainExportFailureAborts(t *testing.T) {
	// A non-existent DomainProjectDir causes inventory.LoadProject to fail.
	// The handler must abort (not continue to ckv/ckg) and surface the
	// failure in resp.CKV.Error, containing "domain export".
	withStubRunner(t, "") // no real subprocess should fire
	d := Deps{Index: IndexConfig{
		CKVBinary:        "echo",
		CKGBinary:        "",
		DomainProjectDir: "/nonexistent/cks-domain-project",
		DomainCorpusDir:  t.TempDir(),
	}}
	res, err := handleOpsIndex(context.Background(), d, callToolReq(map[string]any{"mode": "full"}))
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	var resp opsIndexResponse
	if decErr := decodeStructured(res, &resp); decErr != nil {
		t.Fatalf("decode structured response: %v", decErr)
	}
	if !strings.Contains(resp.CKV.Error, "domain export") {
		t.Errorf("CKV.Error = %q; want substring \"domain export\"", resp.CKV.Error)
	}
}

func TestCKVIndexArgs_FullIncludesDocs(t *testing.T) {
	ic := IndexConfig{
		CKVDataPath:     "./ckv-stablenet",
		SourceRoot:      "/src",
		EmbedModel:      "bge-m3",
		DomainCorpusDir: "generated/domain-corpus/go-stablenet",
	}
	args := ckvIndexArgs(ic, "full", "")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--docs generated/domain-corpus/go-stablenet") {
		t.Errorf("full build args missing --docs: %v", args)
	}
}

func TestCKVIndexArgs_IncrementalOmitsDocs(t *testing.T) {
	ic := IndexConfig{CKVDataPath: "./ckv-stablenet", DomainCorpusDir: "generated/corpus"}
	args := ckvIndexArgs(ic, "incremental", "")
	if strings.Contains(strings.Join(args, " "), "--docs") {
		t.Errorf("incremental (reindex) must not pass --docs: %v", args)
	}
}

type capturedRun struct {
	name string
	args []string
	env  []string
}

// withStubRunner swaps indexRunner for the duration of a test, capturing each
// invocation and returning failFor's error for the named binary.
func withStubRunner(t *testing.T, failFor string) *[]capturedRun {
	t.Helper()
	var calls []capturedRun
	orig := indexRunner
	indexRunner = func(_ context.Context, name string, args, env []string) error {
		calls = append(calls, capturedRun{name: name, args: args, env: env})
		if failFor != "" && name == failFor {
			return errors.New("boom")
		}
		return nil
	}
	t.Cleanup(func() { indexRunner = orig })
	return &calls
}

func TestHandleOpsIndex_UnconfiguredErrors(t *testing.T) {
	res, err := handleOpsIndex(context.Background(), Deps{}, callToolReq(map[string]any{"mode": "incremental"}))
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError result when no index binaries configured, got %+v", res)
	}
}

func TestHandleOpsIndex_BadModeErrors(t *testing.T) {
	res, _ := handleOpsIndex(context.Background(), Deps{Index: IndexConfig{CKVBinary: "ckv"}},
		callToolReq(map[string]any{"mode": "sideways"}))
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for invalid mode")
	}
}

func TestHandleOpsIndex_IncrementalRunsBothBackends(t *testing.T) {
	calls := withStubRunner(t, "")
	d := Deps{Index: IndexConfig{
		CKVBinary: "ckv-bin", CKGBinary: "ckg-bin",
		CKVDataPath: "/d/ckv", CKGDataPath: "/d/ckg/graph.db",
		SourceRoot: "/src", EmbedModel: "bge-m3", OllamaURL: "http://h:1",
	}}
	res, err := handleOpsIndex(context.Background(), d,
		callToolReq(map[string]any{"mode": "incremental", "since_commit": "abc123"}))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Errorf("unexpected IsError on success: %+v", res)
	}
	if len(*calls) != 2 {
		t.Fatalf("want 2 backend runs, got %d: %+v", len(*calls), *calls)
	}
	ckv, ckg := (*calls)[0], (*calls)[1]
	if ckv.name != "ckv-bin" {
		t.Errorf("first run = %q, want ckv-bin", ckv.name)
	}
	joined := strings.Join(ckv.args, " ")
	if !strings.Contains(joined, "reindex") || !strings.Contains(joined, "--src /src") ||
		!strings.Contains(joined, "--out /d/ckv") ||
		!strings.Contains(joined, "--since abc123") || !strings.Contains(joined, "--model-name=bge-m3") {
		t.Errorf("ckv reindex args wrong: %v", ckv.args)
	}
	if len(ckv.env) == 0 || !strings.Contains(ckv.env[0], "CKV_OLLAMA_ENDPOINT=http://h:1") {
		t.Errorf("ckv env missing ollama endpoint: %v", ckv.env)
	}
	if ckg.name != "ckg-bin" || !strings.Contains(strings.Join(ckg.args, " "), "build --src /src --out /d/ckg --force") {
		t.Errorf("ckg build args wrong: %v", ckg.args)
	}
}

func TestHandleOpsIndex_CKGPolicyFileForwarded(t *testing.T) {
	calls := withStubRunner(t, "")
	d := Deps{Index: IndexConfig{
		CKGBinary: "ckg-bin", CKGDataPath: "/d/ckg/graph.db", SourceRoot: "/src",
		CKGPolicyFile: "/p/policy.yaml",
	}}
	if _, err := handleOpsIndex(context.Background(), d, callToolReq(map[string]any{"mode": "full"})); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join((*calls)[0].args, " ")
	if !strings.Contains(joined, "build --src /src --out /d/ckg --force") ||
		!strings.Contains(joined, "--policy-file /p/policy.yaml") {
		t.Errorf("ckg build should forward --policy-file: %v", (*calls)[0].args)
	}
}

func TestHandleOpsIndex_CKGNoPolicyFileOmitsFlag(t *testing.T) {
	calls := withStubRunner(t, "")
	d := Deps{Index: IndexConfig{CKGBinary: "ckg-bin", CKGDataPath: "/d/ckg", SourceRoot: "/src"}}
	if _, err := handleOpsIndex(context.Background(), d, callToolReq(map[string]any{"mode": "full"})); err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join((*calls)[0].args, " "); strings.Contains(joined, "--policy-file") {
		t.Errorf("ckg build must omit --policy-file when unset: %v", (*calls)[0].args)
	}
}

func TestHandleOpsIndex_FullUsesBuildWithSrc(t *testing.T) {
	calls := withStubRunner(t, "")
	d := Deps{Index: IndexConfig{CKVBinary: "ckv-bin", CKVDataPath: "/d/ckv", SourceRoot: "/src", EmbedModel: "bge-m3"}}
	if _, err := handleOpsIndex(context.Background(), d, callToolReq(map[string]any{"mode": "full"})); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join((*calls)[0].args, " ")
	if !strings.Contains(joined, "build --src /src --out /d/ckv") {
		t.Errorf("full mode should `build --src`: %v", (*calls)[0].args)
	}
}

func TestHandleOpsIndex_BackendFailureSurfacesNotError(t *testing.T) {
	// A backend exec failure is reported in the structured per-backend OK
	// field (not a transport error); the handler still returns a result.
	withStubRunner(t, "ckv-bin")
	d := Deps{Index: IndexConfig{CKVBinary: "ckv-bin", CKVDataPath: "/d/ckv", SourceRoot: "/src"}}
	res, err := handleOpsIndex(context.Background(), d, callToolReq(map[string]any{"mode": "incremental"}))
	if err != nil {
		t.Fatalf("should not return transport error on backend failure: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

// writeManifests lays out the two manifests the alignment gate reads, at the
// paths a deployment's IndexConfig points at.
func writeManifests(t *testing.T, graphDir, vectorDir, graphCommit, graphDigest, vecCommit, vecPin string) {
	t.Helper()
	for _, d := range []string{graphDir, vectorDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	graph := fmt.Sprintf(`{"src_commit":%q,"schema_version":"1.23","graph_digest":%q}`, graphCommit, graphDigest)
	if err := os.WriteFile(filepath.Join(graphDir, "manifest.json"), []byte(graph), 0o644); err != nil {
		t.Fatal(err)
	}
	vec := fmt.Sprintf(`{"src_commit":%q,"sources":{"ckg":{"graph_digest":%q}}}`, vecCommit, vecPin)
	if err := os.WriteFile(filepath.Join(vectorDir, "manifest.json"), []byte(vec), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestHandleOpsIndex_RefusesAMisalignedPair reproduces the incident this gate
// was added for: both engines exit 0, but the vector index was rebuilt against
// a graph that moved, so the pair cannot be served. Without the gate the tool
// reports a successful refresh and the failure surfaces at the next restart as
// serviceable=false — every tool down at once, hours after the call.
func TestHandleOpsIndex_RefusesAMisalignedPair(t *testing.T) {
	withStubRunner(t, "")
	root := t.TempDir()
	graphDir, vectorDir := filepath.Join(root, "graph"), filepath.Join(root, "vector")

	cases := []struct {
		name                     string
		graphCommit, graphDigest string
		vecCommit, vecPin        string
		wantAligned              bool
		wantTextContains         string
	}{
		{
			name:        "the pair agrees",
			graphCommit: "abc123", graphDigest: "d1",
			vecCommit: "abc123", vecPin: "d1",
			wantAligned: true, wantTextContains: "index refresh result",
		},
		{
			name:        "vector rebuilt alone, against a graph that moved",
			graphCommit: "abc123", graphDigest: "d1",
			vecCommit: "abc123", vecPin: "d0-stale",
			wantAligned: false, wantTextContains: "not servable",
		},
		{
			name:        "the two indexed different commits",
			graphCommit: "abc123", graphDigest: "d1",
			vecCommit: "def456", vecPin: "d1",
			wantAligned: false, wantTextContains: "not servable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeManifests(t, graphDir, vectorDir, tc.graphCommit, tc.graphDigest, tc.vecCommit, tc.vecPin)
			d := Deps{Index: IndexConfig{
				CKVBinary: "echo", CKGBinary: "echo",
				CKGDataPath: filepath.Join(graphDir, "graph.db"),
				CKVDataPath: vectorDir,
				SourceRoot:  "/src",
			}}
			res, err := handleOpsIndex(context.Background(), d, callToolReq(map[string]any{"mode": "full"}))
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			var resp opsIndexResponse
			if decErr := decodeStructured(res, &resp); decErr != nil {
				t.Fatalf("decode structured response: %v", decErr)
			}
			if !resp.CKV.OK || !resp.CKG.OK {
				t.Fatalf("both builds should have exited 0: %+v", resp)
			}
			if resp.Alignment.OK != tc.wantAligned {
				t.Errorf("Alignment.OK = %v, want %v (error %q)",
					resp.Alignment.OK, tc.wantAligned, resp.Alignment.Error)
			}
			if got := resultText(res); !strings.Contains(got, tc.wantTextContains) {
				t.Errorf("text = %q, want it to contain %q", got, tc.wantTextContains)
			}
		})
	}
}

// TestHandleOpsIndex_GraphOnlyRefreshIsStillChecked is the incident as it
// actually happened. The dataset that broke had its vector index built at
// 12:55 and its graph rebuilt at 13:00 — the graph moved and the vector's pin
// to it went stale. Nothing rebuilt the vector "alone"; its file mtime moved
// later only because the process reopened it.
//
// So the case that matters is a refresh that touches ONE engine while the
// other half of the pair already exists. Gating the check on "this deployment
// builds both" would skip exactly this.
func TestHandleOpsIndex_GraphOnlyRefreshIsStillChecked(t *testing.T) {
	withStubRunner(t, "")
	root := t.TempDir()
	graphDir, vectorDir := filepath.Join(root, "graph"), filepath.Join(root, "vector")
	// The graph rebuilt to a new digest; the vector still pins the old one.
	writeManifests(t, graphDir, vectorDir, "abc123", "digest-new", "abc123", "digest-old")

	d := Deps{Index: IndexConfig{
		CKGBinary:   "echo", // vector binary unset: this refresh rebuilds the graph only
		CKGDataPath: filepath.Join(graphDir, "graph.db"),
		CKVDataPath: vectorDir,
		SourceRoot:  "/src",
	}}
	res, err := handleOpsIndex(context.Background(), d, callToolReq(map[string]any{"mode": "full"}))
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	var resp opsIndexResponse
	if decErr := decodeStructured(res, &resp); decErr != nil {
		t.Fatalf("decode structured response: %v", decErr)
	}
	if !resp.CKG.OK {
		t.Fatalf("the graph build should have exited 0: %+v", resp)
	}
	if resp.Alignment.OK {
		t.Errorf("Alignment.OK = true, want false: the vector pins a digest the graph no longer has")
	}
	if got := resultText(res); !strings.Contains(got, "not servable") {
		t.Errorf("text = %q, want it to report the pair is not servable", got)
	}
}

// TestHandleOpsIndex_NoVectorInThisDeployment keeps the gate from inventing a
// failure where there is no pair to check.
func TestHandleOpsIndex_NoVectorInThisDeployment(t *testing.T) {
	withStubRunner(t, "")
	root := t.TempDir()
	graphDir := filepath.Join(root, "graph")
	if err := os.MkdirAll(graphDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := Deps{Index: IndexConfig{
		CKGBinary:   "echo",
		CKGDataPath: filepath.Join(graphDir, "graph.db"),
		SourceRoot:  "/src",
	}}
	res, err := handleOpsIndex(context.Background(), d, callToolReq(map[string]any{"mode": "full"}))
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if got := resultText(res); strings.Contains(got, "FAILED") {
		t.Errorf("text = %q, want no failure for a deployment with no vector index", got)
	}
}
