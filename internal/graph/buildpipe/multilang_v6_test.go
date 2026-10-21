package buildpipe_test

import (
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestPipelineMultilangV6Markers runs the full
// buildpipe.Run cold-rebuild pipeline over a multi-language
// fixture (Sol + TS + Go) and verifies the round-trip contract
// for both:
//
//   - Identity surface: qualified name, file path, language stamp,
//     cross-language binds_to edges (linker output).
//   - Marker surface: HasFunctionPointerCall, IsFunctionTyped,
//     HasExternalCall, and the rest survive the SQLite write-
//     then-read cycle via the nodes.attrs JSON-blob column added
//     in schema 1.11. This was originally a documented gap; once
//     the column landed this test gained the marker assertions.
func TestPipelineMultilangV6Markers(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	_, err := buildpipe.Run(buildpipe.Options{
		SrcRoot:    "testdata/multilang_v6_markers",
		OutDir:     out,
		Languages:  []string{"auto"},
		CKGVersion: "test",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	store, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer func() { _ = store.Close() }()

	nodes, err := store.AllNodes()
	if err != nil {
		t.Fatalf("AllNodes: %v", err)
	}

	// Sol Wallet and TS Wallet share the bare name "Wallet" — the
	// cross-language binder relies on that homonymy. Index Sol nodes
	// separately so subsequent assertions can target the contract
	// without colliding with the TS class.
	solByQName := map[string]types.Node{}
	for _, n := range nodes {
		if n.Language == "sol" {
			solByQName[n.QualifiedName] = n
		}
	}

	// (a) Sol contract + its three functions all round-trip with
	// their qnames and file paths intact.
	for _, qn := range []string{
		"Wallet", "Wallet.trigger", "Wallet.relay", "Wallet.plain",
	} {
		n, ok := solByQName[qn]
		if !ok {
			t.Errorf("missing Sol node %q in persisted graph", qn)
			continue
		}
		if n.FilePath == "" {
			t.Errorf("%s: empty FilePath", qn)
		}
	}

	// (a.1) Marker round-trip. The Sol parser stamps
	// HasFunctionPointerCall on Wallet.trigger and
	// HasExternalCall on Wallet.relay. The fn-typed
	// state-var onAction lights up IsFunctionTyped on the
	// corresponding NodeField. All three markers MUST survive the
	// SQLite write-then-read cycle now that schema 1.9 (nodes.attrs)
	// is in place.
	if n, ok := solByQName["Wallet.trigger"]; ok {
		if !n.HasFunctionPointerCall {
			t.Errorf("Wallet.trigger HasFunctionPointerCall: got false, want true (after attrs roundtrip)")
		}
	}
	if n, ok := solByQName["Wallet.relay"]; ok {
		if !n.HasExternalCall {
			t.Errorf("Wallet.relay HasExternalCall: got false, want true (after attrs roundtrip)")
		}
	}
	if n, ok := solByQName["Wallet.onAction"]; ok {
		if !n.IsFunctionTyped {
			t.Errorf("Wallet.onAction IsFunctionTyped: got false, want true (after attrs roundtrip)")
		}
	}

	// (b) Cross-language binding. Sol Wallet + TS Wallet → linker
	// emits at least one binds_to edge.
	bindsTo, err := store.QueryEdgesByType("binds_to")
	if err != nil {
		t.Fatalf("QueryEdgesByType: %v", err)
	}
	if len(bindsTo) == 0 {
		t.Errorf("expected >=1 binds_to edge between Sol Wallet and TS Wallet, got 0")
	}

	// (c) Pipeline produced nodes from Sol and TS (Go optional —
	// the fixture has one Go file but buildpipe's auto discovery
	// may skip it if go.mod constraints reject the in-test path).
	langs := map[string]bool{}
	for _, n := range nodes {
		if n.Language != "" {
			langs[n.Language] = true
		}
	}
	if !langs["sol"] {
		t.Errorf("no Sol nodes persisted (langs=%v)", langs)
	}
	if !langs["ts"] {
		t.Errorf("no TS nodes persisted (langs=%v)", langs)
	}
}
