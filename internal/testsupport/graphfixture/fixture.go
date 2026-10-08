// Package graphfixture creates a minimal CKG database for cross-engine tests.
// Production system packages keep their graph dependency read-only.
package graphfixture

import (
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func Create(path, sourceRoot, projectID, datasetID, snapshotID, graphDigest string, nodes []types.Node) error {
	graph, err := persist.Open(path)
	if err != nil {
		return err
	}
	defer graph.Close()
	if err := graph.Migrate(); err != nil {
		return err
	}
	if err := graph.SetManifest(persist.Manifest{SchemaVersion: "1.23", SrcRoot: sourceRoot,
		SourceMode: "snapshot-only", ProjectID: projectID, DatasetID: datasetID,
		SnapshotID: snapshotID, GraphDigest: graphDigest}); err != nil {
		return err
	}
	return graph.InsertNodes(nodes)
}
