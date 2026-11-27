package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

func newExportStaticCmd() *cobra.Command {
	var graph, out string
	cmd := &cobra.Command{
		Use:   "export-static",
		Short: "Export graph as chunked JSON for static hosting",
		Long: `Export the graph as files for static hosting:

  manifest.json             the build manifest
  hierarchy/pkg_tree.json   the package hierarchy
  hierarchy/topic_tree.json the topic clusters
  nodes/chunk_NNNN.json     nodes, 5,000 per file
  edges/chunk_NNNN.json     edges, 10,000 per file
  blobs/<node id>.txt       each node's source text; a Hunk node's patch,
                            stored gzipped in graph.db, is written decompressed

The bundle holds data only. The dashboard pages that read it are not part
of ckg: the separate cks binary of this repository adds them with
cks viewer export --out DIR.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cleanup, err := newLogger(rootVerbose, rootLogFile)
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}
			defer cleanup()

			db, err := graphDB(graph)
			if err != nil {
				return err
			}
			store, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()

			manifest, err := store.GetManifest()
			if err != nil {
				return fmt.Errorf("read manifest: %w", err)
			}
			if err := legacyGraphExportAllowed(manifest); err != nil {
				return err
			}

			// Chunk sizes match spec §6.6: 5k nodes, 10k edges per chunk —
			// small enough to stream incrementally, big enough to keep
			// manifest/directory overhead bounded on repos with ~100k nodes.
			if err := store.ExportChunked(out, 5000, 10000); err != nil {
				return fmt.Errorf("export chunked: %w", err)
			}

			_, _ = fmt.Fprintln(os.Stderr, exportStaticDoneMessage(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory (required)")
	cmd.Flags().StringVar(&out, "out", "", "output directory (required)")
	_ = cmd.MarkFlagRequired("graph")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

// exportStaticDoneMessage is what export-static reports when it is done.
func exportStaticDoneMessage(out string) string {
	return fmt.Sprintf("ckg: exported static graph data to %s (data only; the dashboard pages come from the separate cks binary: cks viewer export --out %s)", out, out)
}
