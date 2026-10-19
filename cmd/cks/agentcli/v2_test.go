package agentcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	sysmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func v2Fixture(t *testing.T, mode string) contract.EvidencePackV2 {
	t.Helper()
	commit := strings.Repeat("c", 40)
	if mode == "snapshot-only" {
		commit = ""
	}
	p := contract.EvidencePackV2{FormatVersion: 2, Query: "explain F", Coordinates: contract.V2Coordinates{ProjectID: "fixture", DatasetID: strings.Repeat("a", 64), SnapshotID: strings.Repeat("b", 64), SourceMode: mode, BaseCommit: commit}, Citations: []contract.CitationV2{}, Bodies: []contract.BodyV2{}, GraphNeighbors: []any{}, EvidenceState: "complete", Metadata: contract.V2Metadata{IntegrityHashAlgo: "sha256-v2"}}
	if err := evidencev2.Stamp(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func v2Mock(pack contract.EvidencePackV2) *mockMCPClient {
	return &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{sysmcp.ToolNameGetForTaskV2: structuredResult(pack)}}
}

func requirePublicCode(t *testing.T, err error, code string) {
	t.Helper()
	var public interface{ PublicCode() string }
	if !errors.As(err, &public) || public.PublicCode() != code {
		t.Fatalf("public code: %v; want %s", err, code)
	}
}

func TestFetchV2SourceModesPreserveCoordinatesAndHash(t *testing.T) {
	for _, mode := range []string{"committed", "working-tree", "snapshot-only"} {
		t.Run(mode, func(t *testing.T) {
			want := v2Fixture(t, mode)
			m := v2Mock(want)
			got, err := (&Agent{client: m}).FetchV2(context.Background(), want.Query, V2Options{})
			if err != nil || got.Coordinates != want.Coordinates || got.Metadata.IntegrityHash != want.Metadata.IntegrityHash {
				t.Fatalf("v2 roundtrip: %+v %v", got, err)
			}
			if len(m.calls) != 1 || m.calls[0].name != sysmcp.ToolNameGetForTaskV2 {
				t.Fatal("legacy tool selected", m.calls)
			}
		})
	}
}

func TestFetchV2ForwardsOptionalArguments(t *testing.T) {
	p := v2Fixture(t, "committed")
	m := v2Mock(p)
	opts := V2Options{ExplainOntology: true, IncludeKnowledge: true, KnowledgeAsOf: "2026-10-20", KnowledgeSubsystem: "core"}
	if _, err := (&Agent{client: m}).FetchV2(context.Background(), p.Query, opts); err != nil {
		t.Fatal(err)
	}
	a := m.calls[0].args
	if a["prompt"] != p.Query || a["explain_ontology"] != true || a["include_knowledge"] != true || a["knowledge_as_of"] != opts.KnowledgeAsOf || a["knowledge_subsystem"] != "core" {
		t.Fatal(a)
	}
}

func TestFetchV2TextFallbackAndPartialState(t *testing.T) {
	p := v2Fixture(t, "working-tree")
	p.EvidenceState = "partial"
	if err := evidencev2.Stamp(&p); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	m := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{sysmcp.ToolNameGetForTaskV2: mcpgo.NewToolResultText(string(raw))}}
	got, err := (&Agent{client: m}).FetchV2(context.Background(), p.Query, V2Options{})
	if err != nil || got.EvidenceState != "partial" || got.Metadata.IntegrityHash != p.Metadata.IntegrityHash {
		t.Fatalf("partial/text lost: %+v %v", got, err)
	}
}

func TestFetchV2KeepsTypedFailure(t *testing.T) {
	r := mcpgo.NewToolResultError("backend message")
	r.StructuredContent = map[string]string{"code": "source_missing"}
	m := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{sysmcp.ToolNameGetForTaskV2: r}}
	_, err := (&Agent{client: m}).FetchV2(context.Background(), "explain F", V2Options{})
	requirePublicCode(t, err, "source_missing")
}

func TestFetchV2RefusesLegacyDTO(t *testing.T) {
	m := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{sysmcp.ToolNameGetForTaskV2: structuredResult(evidencePack{Query: "explain F"})}}
	_, err := (&Agent{client: m}).FetchV2(context.Background(), "explain F", V2Options{})
	requirePublicCode(t, err, "v2_evidence_failed")
}

func TestFetchV2InvalidOptionsNeverCallBackend(t *testing.T) {
	for _, opts := range []V2Options{{Intent: "not-an-intent"}, {IncludeKnowledge: true}, {KnowledgeAsOf: "2026-10-20"}, {IncludeKnowledge: true, KnowledgeAsOf: "2026-02-30", KnowledgeSubsystem: "core"}} {
		m := &mockMCPClient{}
		if _, err := (&Agent{client: m}).FetchV2(context.Background(), "explain F", opts); err == nil || len(m.calls) != 0 {
			t.Fatal("invalid options called backend", opts, err, m.calls)
		}
	}
}

type closeCountClient struct {
	*mockMCPClient
	count int
	hook  func()
}

func (m *closeCountClient) Close() error {
	m.count++
	if m.hook != nil {
		m.hook()
	}
	return m.mockMCPClient.Close()
}

type closeFirstWriter struct {
	client *closeCountClient
	data   bytes.Buffer
}

func (w *closeFirstWriter) Write(b []byte) (int, error) {
	if !w.client.closed {
		return 0, errors.New("output before server close")
	}
	return w.data.Write(b)
}

func TestDeliverV2ClosesOnceBeforeStdout(t *testing.T) {
	p := v2Fixture(t, "committed")
	m := &closeCountClient{mockMCPClient: v2Mock(p)}
	w := &closeFirstWriter{client: m}
	if err := deliverV2(context.Background(), &Agent{client: m}, p.Query, V2Options{}, "", w); err != nil {
		t.Fatal(err)
	}
	var got contract.EvidencePackV2
	if err := json.Unmarshal(w.data.Bytes(), &got); err != nil || evidencev2.Verify(got) != nil || m.count != 1 {
		t.Fatal("output/close", err, m.count)
	}
}

func TestDeliverV2FailedClosePublishesNothing(t *testing.T) {
	p := v2Fixture(t, "committed")
	m := &closeCountClient{mockMCPClient: v2Mock(p)}
	m.closeErr = errors.New("shutdown failed")
	var out bytes.Buffer
	err := deliverV2(context.Background(), &Agent{client: m}, p.Query, V2Options{}, "", &out)
	requirePublicCode(t, err, "service_unavailable")
	if out.Len() != 0 || m.count != 1 {
		t.Fatal("failed close published output", out.String(), m.count)
	}
}

func TestDeliverV2CancellationAtClosePublishesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := v2Fixture(t, "committed")
	m := &closeCountClient{mockMCPClient: v2Mock(p), hook: cancel}
	var out bytes.Buffer
	err := deliverV2(ctx, &Agent{client: m}, p.Query, V2Options{}, "", &out)
	requirePublicCode(t, err, "request_canceled")
	if out.Len() != 0 {
		t.Fatal("canceled output published")
	}
}

func TestDeliverV2FileIsNewAndTemporaryFilesRemoved(t *testing.T) {
	p := v2Fixture(t, "snapshot-only")
	dir := t.TempDir()
	file := filepath.Join(dir, "context.json")
	if err := deliverV2(context.Background(), &Agent{client: v2Mock(p)}, p.Query, V2Options{}, file, io.Discard); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var got contract.EvidencePackV2
	if err := json.Unmarshal(raw, &got); err != nil || evidencev2.Verify(got) != nil {
		t.Fatal("invalid output", err)
	}
	err = deliverV2(context.Background(), &Agent{client: v2Mock(p)}, p.Query, V2Options{}, file, io.Discard)
	requirePublicCode(t, err, "output_exists")
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(after, raw) {
		t.Fatal("existing output changed", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary output remains", err, entries)
	}
}

func TestDeliverV2FailedFetchPreservesExistingOutput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "context.json")
	if err := os.WriteFile(file, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &closeCountClient{mockMCPClient: &mockMCPClient{callErr: map[string]error{sysmcp.ToolNameGetForTaskV2: context.Canceled}}}
	err := deliverV2(context.Background(), &Agent{client: m}, "explain F", V2Options{}, file, io.Discard)
	requirePublicCode(t, err, "request_canceled")
	raw, err := os.ReadFile(file)
	if err != nil || string(raw) != "previous" || m.count != 1 {
		t.Fatal("failed fetch changed output or left client", err, string(raw), m.count)
	}
}

func TestAgentV2CommandValidatesBeforeServerStart(t *testing.T) {
	for _, args := range [][]string{{"--format=unknown"}, {"--explain-ontology"}, {"--format=v2", "--include-knowledge"}} {
		cmd := NewCmd()
		cmd.SetArgs(append(args, "--cks-mcp=/does-not-exist", "--prompt=explain F"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		var public interface{ PublicCode() string }
		if !errors.As(err, &public) || public.PublicCode() == "service_unavailable" {
			t.Fatal("invalid format/options reached process startup", args, err)
		}
	}
	if NewCmd().Flags().Lookup("format").DefValue != "markdown" {
		t.Fatal("legacy default changed")
	}
}
