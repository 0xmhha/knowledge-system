package buildpipe

import (
	"fmt"
	"log/slog"
	"path"
	"sort"

	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/logx"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"golang.org/x/tools/go/packages"
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

// logGoLoad records what the go command reported while loading and
// type-checking the Go packages (area load) and returns the number of errors.
// Errors are a WARN: a package that fails to load still gives nodes, but
// references into it lose their targets, which shows up later only as more
// unresolved references. With a build scope, the outside modules loaded are
// compared with the scope's: another version is a WARN (the graph describes
// other code than the scope), another directory is counted (another module
// cache, the same code when both caches are intact).
func logGoLoad(log *slog.Logger, pkgs []*packages.Package, b *detect.GoBuild, seconds float64) int {
	l := log.With(logx.AreaKey, "load")
	p := detect.SummarizeLoad(pkgs, summarySamples)
	attrs := []any{"language", "go", "packages", p.Packages, "packages_with_errors", p.WithErrors,
		"errors", p.Errors(), "seconds", seconds}
	kinds := make([]string, 0, len(p.ByKind))
	for k := range p.ByKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		attrs = append(attrs, "errors_"+k, p.ByKind[k])
	}
	var mc detect.ModuleCheck
	if want := b.ScopeModules(); len(want) > 0 {
		mc = detect.CheckModules(pkgs, want)
		attrs = append(attrs, "scope_modules", len(want), "modules_matched", mc.Matched,
			"modules_other_version", len(mc.OtherVersion), "modules_other_dir", len(mc.OtherDir),
			"modules_not_loaded", len(mc.NotLoaded))
	}
	if p.Errors() > 0 || len(mc.OtherVersion) > 0 {
		attrs = append(attrs, "samples", p.Samples)
		if len(mc.OtherVersion) > 0 {
			attrs = append(attrs, "other_version", firstN(mc.OtherVersion, summarySamples))
		}
		if p.Errors() > 0 && len(mc.OtherDir) > 0 {
			attrs = append(attrs, "other_dir", firstN(mc.OtherDir, summarySamples))
		}
		l.Warn("go packages loaded with problems", attrs...)
	} else {
		l.Info("go packages loaded", attrs...)
	}
	if len(mc.OtherDir) > 0 || len(mc.NotLoaded) > 0 {
		l.Debug("go modules differ from the scope", "other_dir", firstN(mc.OtherDir, summarySamples),
			"not_loaded", firstN(mc.NotLoaded, summarySamples))
	}
	return p.Errors()
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// goLoadErrors sums the load errors of the resolved graphs for the manifest.
func goLoadErrors(parts []*parse.ResolvedGraph) int {
	n := 0
	for _, rg := range parts {
		if rg != nil {
			n += rg.LoadErrors
		}
	}
	return n
}
