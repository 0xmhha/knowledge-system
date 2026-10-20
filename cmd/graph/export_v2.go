package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type graphExportError string

func (e graphExportError) Error() string { return string(e) }

func graphExportMessage(e graphExportError) string {
	switch e {
	case "requires_v2":
		return "This source requires the v2 graph JSON format."
	case "reindex_required":
		return "Complete source identity is required; rebuild the graph."
	case "invalid_request":
		return "Select a supported graph export format."
	case "output_exists":
		return "The output file already exists."
	case "request_canceled":
		return "Graph export was canceled."
	case "request_timeout":
		return "Graph export timed out."
	case "durability_uncertain":
		return "The output was published but its durability could not be confirmed."
	default:
		return "Graph export failed."
	}
}

func legacyGraphExportAllowed(m persist.Manifest) error {
	switch m.SourceMode {
	case "", "committed":
		return nil
	case "working-tree", "snapshot-only":
		return graphExportError("requires_v2")
	default:
		return graphExportError("reindex_required")
	}
}

func graphCoordinates(m persist.Manifest) (contract.V2Coordinates, error) {
	c := contract.V2Coordinates{ProjectID: m.ProjectID, DatasetID: m.DatasetID,
		SnapshotID: m.SnapshotID, SourceMode: m.SourceMode, BaseCommit: m.SrcCommit}
	digest := func(s string, bytes int) bool { b, err := hex.DecodeString(s); return err == nil && len(b) == bytes }
	if c.ProjectID == "" || !digest(c.DatasetID, 32) || !digest(c.SnapshotID, 32) ||
		!digest(m.FileManifestDigest, 32) || !digest(m.CapturePolicyDigest, 32) {
		return c, graphExportError("reindex_required")
	}
	switch c.SourceMode {
	case "committed", "working-tree":
		if !digest(c.BaseCommit, 20) {
			return c, graphExportError("reindex_required")
		}
	case "snapshot-only":
		if c.BaseCommit != "" {
			return c, graphExportError("reindex_required")
		}
	default:
		return c, graphExportError("reindex_required")
	}
	return c, nil
}

type graphVisitor interface {
	VisitGraph(context.Context, func(persist.Manifest) error, func(types.Node) error, func(types.Edge) error) error
}

// writeGraphV2 is a raw graph snapshot, not an EvidencePack: graph row IDs are
// scoped by coordinates and source digests are declared manifest pins, not a
// verification of archived file bytes or a sha256-v2 evidence hash.
func writeGraphV2(ctx context.Context, out io.Writer, store graphVisitor, pretty, minimal bool) error {
	w := bufio.NewWriter(out)
	nodes, edges := 0, 0
	startedEdges := false
	write := func(s string) error { _, err := io.WriteString(w, s); return err }
	value := func(v any) error {
		var b []byte
		var err error
		if pretty {
			b, err = json.MarshalIndent(v, "  ", "  ")
		} else {
			b, err = json.Marshal(v)
		}
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	beginEdges := func() error {
		if startedEdges {
			return nil
		}
		startedEdges = true
		return write("],\n\"edges\":[")
	}
	err := store.VisitGraph(ctx, func(m persist.Manifest) error {
		c, err := graphCoordinates(m)
		if err != nil {
			return err
		}
		if err := write("{\"format\":\"ckg-graph-v2\",\"format_version\":2,\"coordinates\":"); err != nil {
			return err
		}
		if err := value(c); err != nil {
			return err
		}
		if err := write(",\"source\":"); err != nil {
			return err
		}
		if err := value(struct {
			FileManifest  string `json:"file_manifest_digest"`
			CapturePolicy string `json:"capture_policy_digest"`
		}{m.FileManifestDigest, m.CapturePolicyDigest}); err != nil {
			return err
		}
		if err := write(",\"header\":"); err != nil {
			return err
		}
		hdr := jsonGraphHeader{SchemaVersion: m.SchemaVersion, Engine: "graph", BuilderVersion: m.EffectiveBuilderVersion(), CKGVersion: m.CKGVersion, SrcRoot: m.SrcRoot, SrcCommit: m.SrcCommit, BuildTimestamp: m.BuildTimestamp, Languages: m.Languages, Stats: m.Stats}
		if err := value(hdr); err != nil {
			return err
		}
		return write(",\n\"nodes\":[")
	}, func(n types.Node) error {
		if nodes > 0 {
			if err := write(","); err != nil {
				return err
			}
		}
		if minimal {
			if err := value(toMinimalNode(n)); err != nil {
				return err
			}
		} else if err := value(n); err != nil {
			return err
		}
		nodes++
		return nil
	}, func(e types.Edge) error {
		if err := beginEdges(); err != nil {
			return err
		}
		if edges > 0 {
			if err := write(","); err != nil {
				return err
			}
		}
		if minimal {
			if err := value(toMinimalEdge(e)); err != nil {
				return err
			}
		} else if err := value(e); err != nil {
			return err
		}
		edges++
		return nil
	})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := beginEdges(); err != nil {
		return err
	}
	if err := write("],\"counts\":"); err != nil {
		return err
	}
	if err := value(struct {
		Nodes int `json:"nodes"`
		Edges int `json:"edges"`
	}{nodes, edges}); err != nil {
		return err
	}
	if err := write("}\n"); err != nil {
		return err
	}
	return w.Flush()
}

func exportGraphV2(ctx context.Context, graph, output string, pretty, minimal bool) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := publishGraphV2(ctx, output, func(w io.Writer) error {
		store, err := persist.OpenReadOnly(filepath.Join(graph, "graph.db"))
		if err != nil {
			return err
		}
		err = writeGraphV2(ctx, w, store, pretty, minimal)
		return errors.Join(err, store.Close())
	})
	if err == nil {
		return nil
	}
	var public graphExportError
	if errors.As(err, &public) {
		return public
	}
	if errors.Is(err, context.Canceled) {
		return graphExportError("request_canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return graphExportError("request_timeout")
	}
	return graphExportError("export_failed")
}

// A new, same-directory file is published only after a complete read, flush,
// database close, fsync and file close. An existing destination is preserved.
func publishGraphV2(ctx context.Context, output string, encode func(io.Writer) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".ckg-graph-v2-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if err := encode(f); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Link(name, output); err != nil {
		if errors.Is(err, os.ErrExist) {
			return graphExportError("output_exists")
		}
		return err
	}
	if err := os.Remove(name); err != nil {
		return graphExportError("durability_uncertain")
	}
	d, err := os.Open(filepath.Dir(output))
	if err != nil {
		return graphExportError("durability_uncertain")
	}
	err = d.Sync()
	closeErr := d.Close()
	if err != nil || closeErr != nil {
		return graphExportError("durability_uncertain")
	}
	return nil
}
