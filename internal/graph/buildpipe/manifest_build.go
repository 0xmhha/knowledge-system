// Package buildpipe — manifest.go holds what every build shares around the
// parse: the discovery of the files (for the manifest's per-file entries)
// and the manifest itself.
//
// Every build is cold (ADR-0002): it parses every file and recomputes every
// artifact; the per-file entries of the manifest record what was built and
// are not read back as a cache. The incremental build was removed: on 11 of
// 13 measured edits its graph differed from a cold build of the same tree,
// and it was slower than a cold build.
package buildpipe

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/0xmhha/knowledge-system/internal/graph/cluster"
	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/internal/graph/filterlist"
	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// discoveryAll returns every discovered file in slash form with language tag.
// Output is sorted by path so cache-decision logging is deterministic.
//
// filter (optional): when non-nil, paths that fail filter.Allow are excluded
// from the result before counting. This keeps the cache-decision logic in
// sync with what runCold actually parses (--files-from is honoured by both
// paths or neither — never one and not the other).
func discoveryAll(srcRoot string, languages []string, filter *filterlist.FilterList, goBuild *detect.GoBuild) ([]DiscoveredFile, persist.Manifest, int, int, int, int, error) {
	var lc persist.Manifest // language counts only — caller fills full manifest later
	files, err := detect.Walk(srcRoot)
	if err != nil {
		return nil, lc, 0, 0, 0, 0, fmt.Errorf("detect: %w", err)
	}
	goFiles, err := detect.GoFilesFor(srcRoot, goBuild)
	if err != nil {
		return nil, lc, 0, 0, 0, 0, fmt.Errorf("detect go: %w", err)
	}
	if filter != nil {
		goFiles = filter.FilterPaths(goFiles)
		files.TS = filter.FilterPaths(files.TS)
		files.Sol = filter.FilterPaths(files.Sol)
		files.Proto = filter.FilterPaths(files.Proto)
	}
	out := make([]DiscoveredFile, 0, len(goFiles)+len(files.TS)+len(files.Sol)+len(files.Proto))
	if shouldRun("go", languages) {
		for _, p := range goFiles {
			out = append(out, DiscoveredFile{Path: filepath.ToSlash(p), Language: "go"})
		}
	}
	if shouldRun("ts", languages) {
		for _, p := range files.TS {
			out = append(out, DiscoveredFile{Path: filepath.ToSlash(p), Language: "ts"})
		}
	}
	if shouldRun("sol", languages) {
		for _, p := range files.Sol {
			out = append(out, DiscoveredFile{Path: filepath.ToSlash(p), Language: "sol"})
		}
	}
	if shouldRun("proto", languages) {
		for _, p := range files.Proto {
			out = append(out, DiscoveredFile{Path: filepath.ToSlash(p), Language: "proto"})
		}
	}
	return out, lc, len(goFiles), len(files.TS), len(files.Sol), len(files.Proto), nil
}

// buildManifestSkeleton fills the non-Files portion of the new manifest. The
// Files block is set separately so cold and incremental paths share the
// per-file population logic.
func buildManifestSkeleton(opt Options, goCount, tsCount, solCount, protoCount int,
	g *graph.Graph, pkgTree *cluster.PkgTree, parseErrs int) persist.Manifest {
	return persist.Manifest{
		SchemaVersion:       SchemaVersion,
		ProjectID:           opt.ProjectID,
		SnapshotID:          opt.SnapshotID,
		DatasetID:           opt.DatasetID,
		FileManifestDigest:  opt.FileManifestDigest,
		CapturePolicyDigest: opt.CapturePolicyDigest,
		SourceMode:          opt.SourceMode,
		CKGVersion:          opt.CKGVersion,
		BuildTimestamp:      time.Now().UTC().Format(time.RFC3339),
		SrcRoot:             opt.SrcRoot,
		GoBuild:             opt.GoBuild.Key(),
		Languages: map[string]int{
			"go": goCount, "ts": tsCount, "sol": solCount, "proto": protoCount,
		},
		Stats: map[string]int{
			"nodes":          len(g.Nodes),
			"edges":          len(g.Edges),
			"pkg_tree_edges": len(pkgTree.Edges),
		},
		GraphDigest:      ComputeGraphDigest(g.Nodes, g.Edges),
		EnrichDigest:     ComputeEnrichDigest(g.Nodes, g.Edges),
		ParseErrorsCount: parseErrs,
		ClusteringStatus: "ok",
	}
}
