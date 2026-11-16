package buildpipe

import (
	"fmt"
	"log/slog"
	"path"
	"sort"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/logx"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Build summary records (docs/graph/BUILD-AND-QUERY-FLOW.md, section 4): each
// counts in its stage and leaves one record at the end of it.

const summarySamples = 20

// logResolveSummary records how many references a language's Resolve bound
// and how many it dropped, by reason; samples go to DEBUG.
func logResolveSummary(log *slog.Logger, language string, rg *parse.ResolvedGraph) {
	if rg == nil {
		return
	}
	l := log.With(logx.AreaKey, "resolve")
	total := 0
	attrs := []any{"language", language}
	reasons := make([]string, 0, len(rg.Unresolved))
	for r := range rg.Unresolved {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		total += rg.Unresolved[r]
		attrs = append(attrs, "unresolved_"+r, rg.Unresolved[r])
	}
	attrs = append(attrs, "unresolved", total)
	l.Info("resolve summary", attrs...)
	for _, r := range reasons {
		l.Debug("unresolved samples", "language", language, "reason", r, "samples", rg.UnresolvedSamples[r])
	}
}

// unresolvedTotal sums the dropped references of all parts for the manifest.
func unresolvedTotal(parts []*parse.ResolvedGraph) int {
	n := 0
	for _, p := range parts {
		if p == nil {
			continue
		}
		for _, c := range p.Unresolved {
			n += c
		}
	}
	return n
}

// warnDuplicateNodeIDs counts node ids that two different declarations
// received. graph.Build keeps one node per id, so such a pair silently loses
// a node (two packages that declared the same name did, before node ids were
// keyed by file). Files of one package share their package node, which is not
// a duplicate.
func warnDuplicateNodeIDs(log *slog.Logger, parts []*parse.ResolvedGraph) int {
	first := map[string]types.Node{}
	count := 0
	var samples []string
	for _, p := range parts {
		if p == nil {
			continue
		}
		for _, n := range p.Nodes {
			prev, seen := first[n.ID]
			if !seen {
				first[n.ID] = n
				continue
			}
			if sameDeclaration(prev, n) {
				continue
			}
			count++
			if len(samples) < summarySamples {
				samples = append(samples, fmt.Sprintf("%s %s (%s) | %s %s (%s)",
					prev.Type, prev.QualifiedName, prev.FilePath, n.Type, n.QualifiedName, n.FilePath))
			}
		}
	}
	if count > 0 {
		log.With(logx.AreaKey, "graph").Warn("duplicate node ids: one node of each pair is lost",
			"count", count, "samples", samples)
	}
	return count
}

func sameDeclaration(a, b types.Node) bool {
	if a.Type != b.Type || a.QualifiedName != b.QualifiedName {
		return false
	}
	if a.FilePath == b.FilePath {
		return true
	}
	return a.Type == types.NodePackage && path.Dir(a.FilePath) == path.Dir(b.FilePath)
}

// coldReason names why a build cannot use the cache of the previous graph.
func coldReason(opt Options, old *persist.Manifest, sameGoBuild bool) string {
	switch {
	case opt.NoCache:
		return "--no-cache"
	case old == nil:
		return "no previous graph"
	case !sameGoBuild:
		return "go build configuration changed"
	case old.SchemaVersion != SchemaVersion:
		return "schema version changed"
	case old.EffectiveBuilderVersion() != opt.CKGVersion:
		return "builder version changed"
	case len(old.Files) == 0:
		return "previous graph has no file list"
	}
	return "cache not usable"
}

// logCacheDecision records which build path runs and why, with the file
// counts of an incremental or short-circuit decision.
func logCacheDecision(log *slog.Logger, path, reason string, d CacheDecisions) {
	log.With(logx.AreaKey, "cache").Info("cache decision", "path", path, "reason", reason,
		"cached", d.Hits, "changed", d.Misses, "removed", d.Removed)
}
