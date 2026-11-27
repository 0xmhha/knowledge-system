// Package buildpipe — temporal.go wires the git history into the build:
//
//   - The line history of every File node and declaration
//     (lineHistoryNodeTypes) is followed back through HEAD's history once
//     (temporal.TrackLines): which commits changed its lines, and which
//     commit last changed each of its lines today.
//   - One Commit node per commit those facts name, with its author, author
//     and commit times, whole subject and pull request number.
//   - changed_in: node → each commit that changed the node's lines.
//   - blame: node → each commit that last changed some of its lines today.
//   - The hunk graph of the newest commits (temporal_hunks.go), whose
//     modifies edges follow the same rule.
//   - The pull request breadcrumbs (node_prs) of the same nodes.
//
// Skips silently (no error) when srcRoot isn't inside a git checkout, so
// non-git source trees still build cleanly without temporal edges.
package buildpipe

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/graph/filterlist"
	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/graph/temporal"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// HistoryOptions configures the history pass of a build.
//   - Depth caps the changed_in edges kept per node, newest first; 0 keeps
//     every commit that changed the node.
//   - Spaces says which repository a pull request number belongs to; an
//     empty Repo is read from the origin remote when it is a GitHub URL.
type HistoryOptions struct {
	Depth  int
	Spaces temporal.NumberSpaces
}

// lineHistoryNodeTypes are the node kinds whose line history and blame the
// build records: files, and the declarations an agent asks "how and why did
// this change?" about — callables and type declarations. Sub-symbol nodes
// (fields, variables, parameters) and statements get none.
var lineHistoryNodeTypes = map[types.NodeType]bool{
	types.NodeFile:        true,
	types.NodeFunction:    true,
	types.NodeMethod:      true,
	types.NodeStruct:      true,
	types.NodeInterface:   true,
	types.NodeConstructor: true,
	types.NodeModifier:    true,
	types.NodeContract:    true,
	types.NodeEvent:       true,
	types.NodeClass:       true,
	types.NodeTypeAlias:   true,
	types.NodeEnum:        true,
}

// recordedRule says which walked commits get a Commit node.
const recordedRule = "a commit is recorded when it changed lines that a file or declaration in scope holds today (its line history), or its hunks are stored; " +
	"a commit that changed only files outside the scope, or only lines no longer present, is walked but not recorded"

// renamesRule is how the history follows renamed files.
const renamesRule = "followed (git rename detection); when a file is added as a copy of another (git copy detection), a declaration goes on into the source only if at least half of its code lines (not blank, not only a comment) are unchanged there, and the commits that adds beyond git log -L are marked copy and past_copy; lines moved within or between files are not traced"

// temporalResult is what the history pass hands the rest of the build.
type temporalResult struct {
	hunkBlobs map[string][]byte
	prRefs    map[string][]types.PRRef
	record    *persist.HistoryRecord
}

// emitTemporalEdges adds Commit and Hunk nodes and the history edges
// (changed_in, blame, has_hunk, adjacent, modifies) to g, and returns the
// hunk patch blobs, the pull request breadcrumbs per node and the account of
// the history read.
//
// The filter is the build-scope list the discovery pass applied: history is
// recorded for nodes of files it allows (nil allows every file). Hunks of
// files it excludes are dropped too.
//
// Behavior:
//   - srcRoot not in a git checkout → no-op, zero result, nil error.
//   - Otherwise: append nodes/edges in place. Caller MUST re-run
//     graph.Validate after to catch any dangling refs introduced.
func emitTemporalEdges(g *graph.Graph, srcRoot string, log *slog.Logger, opt HistoryOptions,
	filter *filterlist.FilterList) (temporalResult, error) {
	repoRoot, srcRel, ok := gitRepoRel(srcRoot)
	if !ok {
		log.Debug("temporal: srcRoot is not a git checkout; no history recorded", "src", srcRoot)
		return temporalResult{}, nil
	}
	ctx := context.Background()
	items, nodeOf := historyItems(g.Nodes, srcRel, filter)
	if len(items) == 0 {
		return temporalResult{}, nil
	}
	boundaries := shallowBoundaries(repoRoot)
	started := time.Now()
	log.Info("temporal: following line history", "nodes", len(items), "files", countFiles(items))
	hist, err := temporal.TrackLines(ctx, repoRoot, historyPathspec(items), items, boundaries)
	if err != nil {
		return temporalResult{}, fmt.Errorf("temporal line history: %w", err)
	}
	log.Info("temporal: line history followed", "commits", len(hist.Order),
		"nodes_with_history", len(hist.Touches), "seconds", time.Since(started).Seconds())
	if len(hist.Order) == 0 {
		return temporalResult{}, nil
	}
	spaces := opt.Spaces
	if spaces.Repo == "" {
		spaces.Repo = scanGitRemoteRepo(repoRoot)
	}
	upstream, err := temporal.UpstreamCommits(ctx, repoRoot, spaces.UpstreamBase)
	if err != nil {
		return temporalResult{}, fmt.Errorf("temporal number spaces: %w", err)
	}
	cm := newCommitMinter(hist, spaces, upstream, srcRel)

	changedIn, blame := historyEdges(hist, nodeOf, opt.Depth, srcRel, cm)

	hunkStarted := time.Now()
	hunks, err := temporal.LoadHunks(repoRoot, 0)
	if err != nil {
		return temporalResult{}, fmt.Errorf("temporal LoadHunks: %w", err)
	}
	relHunks, _ := remapHunksToSrcRel(hunks, srcRel)
	relHunks = scopeHunksToFilter(relHunks, filter)
	relHunks = slices.DeleteFunc(relHunks, func(h temporal.HunkInfo) bool { return cm.id(h.SHA) == "" })
	window := hunkWindow(relHunks, hist)

	g.Nodes = append(g.Nodes, cm.nodes...)
	g.Edges = append(g.Edges, changedIn...)
	g.Edges = append(g.Edges, blame...)
	hunkNodes, hasHunk, adjacent, hunkBlobs := buildHunkNodes(relHunks, "", cm.ids, subjectsOf(hist), types.ConfExtracted)
	g.Nodes = append(g.Nodes, hunkNodes...)
	g.Edges = append(g.Edges, hasHunk...)
	g.Edges = append(g.Edges, adjacent...)
	modifies := modifiesEdges(relHunks, hist, nodeOf, srcRel)
	g.Edges = append(g.Edges, modifies...)

	// Hunks of commits reachable only via reflog or fsck-unreachable
	// (force-pushed-away history) come as AMBIGUOUS Commit + Hunk nodes for
	// a recovery workflow. No line history reaches them, so they get no
	// modifies edges.
	recorded := len(cm.nodes)
	unreachableBlobs, lostCommits, lostHunks, err := emitUnreachableHunkGraph(ctx, g, srcRoot, srcRel, repoRoot, cm, filter)
	if err != nil {
		return temporalResult{}, fmt.Errorf("temporal unreachable hunk graph: %w", err)
	}
	maps.Copy(hunkBlobs, unreachableBlobs)

	log.Info("temporal G6 emitted",
		"commit_nodes", recorded+lostCommits,
		"unreachable_commit_nodes", lostCommits,
		"hunk_nodes", len(hunkNodes)+lostHunks,
		"unreachable_hunk_nodes", lostHunks,
		"changed_in_edges", len(changedIn),
		"blame_edges", len(blame),
		"modifies_edges", len(modifies),
		"hunk_edges", len(hasHunk)+len(adjacent),
		"hunk_blobs", len(hunkBlobs),
		"depth_per_node", opt.Depth,
		"hunk_seconds", time.Since(hunkStarted).Seconds())
	return temporalResult{
		hunkBlobs: hunkBlobs,
		prRefs:    prRefsFromHistory(hist, nodeOf, cm),
		record:    historyRecord(hist, opt.Depth, window, spaces, recorded, lostCommits),
	}, nil
}

// historyItems lists the nodes whose history is followed, with their
// repository-relative paths, and maps item IDs back to the nodes.
func historyItems(nodes []types.Node, srcRel string, filter *filterlist.FilterList) ([]temporal.Item, map[string]types.Node) {
	prefix := srcRelPrefix(srcRel)
	var items []temporal.Item
	nodeOf := map[string]types.Node{}
	for _, n := range nodes {
		if !lineHistoryNodeTypes[n.Type] || n.FilePath == "" || n.StartLine < 1 {
			continue
		}
		if filter != nil && !filter.Allow(n.FilePath) {
			continue
		}
		it := temporal.Item{ID: n.ID, Path: prefix + n.FilePath, Start: n.StartLine, End: n.EndLine, Name: simpleName(n.Name)}
		if n.Type == types.NodeFile {
			it.Start, it.End, it.Name = 1, 0, "" // the whole file, whatever its node's span
		} else if n.EndLine < n.StartLine {
			continue
		}
		items = append(items, it)
		nodeOf[n.ID] = n
	}
	return items, nodeOf
}

// historicalPath is a commit-time path relative to srcRoot when it lies
// under it, else relative to the repository root.
func historicalPath(repoPath, prefix string) string {
	if rel, ok := strings.CutPrefix(repoPath, prefix); ok {
		return rel
	}
	return repoPath
}

// srcRelPrefix is the repository-relative prefix of srcRoot's files.
func srcRelPrefix(srcRel string) string {
	if srcRel == "" || srcRel == "." {
		return ""
	}
	return strings.TrimSuffix(srcRel, "/") + "/"
}

// historyPathspec limits the walked history to the items' file types: one
// "*.<ext>" per extension, so a file renamed from another name of the same
// type is still followed; files without an extension are named exactly.
func historyPathspec(items []temporal.Item) []string {
	set := map[string]bool{}
	for _, it := range items {
		if ext := path.Ext(it.Path); ext != "" {
			set["*"+ext] = true
		} else {
			set[it.Path] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

func countFiles(items []temporal.Item) int {
	set := map[string]bool{}
	for _, it := range items {
		set[it.Path] = true
	}
	return len(set)
}

// commitFilePath is the FilePath of Commit nodes: they live in no file, but
// FilePath is required; the build root's path is stable across builds.
func commitFilePath(srcRel string) string {
	if srcRel == "" || srcRel == "." {
		return ".git"
	}
	return srcRel
}

// commitMinter creates each Commit node once, on first use, and remembers
// its ID.
type commitMinter struct {
	hist     *temporal.LineHistory
	spaces   temporal.NumberSpaces
	upstream map[string]bool
	filePath string
	ids      map[string]string
	nodes    []types.Node
	// upstreamMax is the largest number that merged an upstream commit:
	// how far the upstream's numbering had come at the fork point.
	upstreamMax int
}

// newCommitMinter prepares the Commit nodes of a walked history.
func newCommitMinter(hist *temporal.LineHistory, spaces temporal.NumberSpaces, upstream map[string]bool, srcRel string) *commitMinter {
	m := &commitMinter{hist: hist, spaces: spaces, upstream: upstream, filePath: commitFilePath(srcRel), ids: map[string]string{}}
	if spaces.UpstreamBase != "" {
		for sha, c := range hist.Commits {
			if upstream[sha] {
				m.upstreamMax = max(m.upstreamMax, temporal.PRNumber(c.Subject))
			}
		}
	}
	return m
}

// earlierNumberRule decides the repository of a number a subject names
// besides its own (a ported or a reverted pull request).
const earlierNumberRule = "a number a subject names besides its own (a port's or a revert's) is read in the upstream repository when the commit is upstream, " +
	"or when it is larger than every number of the upstream commits (a pull request the upstream merged after the fork point, which a port carries over); otherwise it is one of the source's own pull requests"

// earlierRepo places such a number by earlierNumberRule.
func (m *commitMinter) earlierRepo(n int, upstreamCommit bool) string {
	if m.spaces.UpstreamBase == "" {
		return m.spaces.Repo
	}
	if upstreamCommit || n > m.upstreamMax {
		return m.spaces.UpstreamRepo
	}
	return m.spaces.Repo
}

// id returns the node ID of commit sha, creating the node; "" when sha is
// not a commit of the walked history.
func (m *commitMinter) id(sha string) string {
	if id, ok := m.ids[sha]; ok {
		return id
	}
	meta, ok := m.hist.Commits[sha]
	if !ok {
		return ""
	}
	n := makeCommitNode(sha, m.commitMeta(meta), m.filePath)
	m.ids[sha] = n.ID
	m.nodes = append(m.nodes, n)
	return n.ID
}

// commitMeta is the node metadata of one walked commit, its pull request
// number placed in its number space.
func (m *commitMinter) commitMeta(c temporal.CommitMeta) types.CommitMeta {
	nums := temporal.PRNumbers(c.Subject)
	meta := types.CommitMeta{
		Author: c.Author, AuthorTime: c.AuthorTime, CommitTime: c.CommitTime,
		Subject: c.Subject, Body: bodyExcerpt(c.Body), PR: nums.Own, Ported: nums.Ported, Reverts: nums.Reverts,
		Upstream: m.upstream[c.SHA],
	}
	if meta.PR > 0 {
		meta.PRRepo = m.spaces.Repo
		if meta.Upstream {
			meta.PRRepo = m.spaces.UpstreamRepo
		}
	}
	if len(nums.Ported) > 0 {
		meta.PortedRepo = m.earlierRepo(nums.Ported[0], meta.Upstream)
	}
	if len(nums.Reverts) > 0 {
		meta.RevertsRepo = m.earlierRepo(nums.Reverts[0], meta.Upstream)
	}
	return meta
}

// historyEdges turns the line history into changed_in and blame edges.
// depth > 0 keeps the newest depth commits per node. A changed_in edge
// carries the node's place in that commit: FilePath is the file's name then
// (it differs from the node's after a rename) and Line the node's first line
// then. A blame edge carries today's file.
func historyEdges(hist *temporal.LineHistory, nodeOf map[string]types.Node, depth int, srcRel string, cm *commitMinter) (changedIn, blame []types.Edge) {
	prefix := srcRelPrefix(srcRel)
	for _, id := range slices.Sorted(maps.Keys(nodeOf)) {
		n := nodeOf[id]
		touches := hist.Touches[id]
		if depth > 0 && len(touches) > depth {
			touches = touches[:depth]
		}
		for _, tc := range touches {
			if cid := cm.id(tc.SHA); cid != "" {
				e := types.Edge{Src: id, Dst: cid, Type: types.EdgeChangedIn,
					FilePath: historicalPath(tc.Path, prefix), Line: tc.Start, Count: 1, Confidence: types.ConfExtracted}
				switch {
				case tc.Copy:
					e.DispatchKind = types.DispatchCopy
				case tc.PastCopy != "":
					e.DispatchKind = types.DispatchPastCopy
				}
				changedIn = append(changedIn, e)
			}
		}
		lines := hist.Blame[id]
		for _, sha := range slices.Sorted(maps.Keys(lines)) {
			if cid := cm.id(sha); cid != "" {
				blame = append(blame, types.Edge{Src: id, Dst: cid, Type: types.EdgeBlame,
					FilePath: n.FilePath, Count: lines[sha], Confidence: types.ConfExtracted})
			}
		}
	}
	return changedIn, blame
}

// makeCommitNode produces the Commit node of one commit. ID derives from
// (`commit:<sha>`, "git", 0) so the same commit consistently hashes to the
// same node ID across rebuilds + across srcRoots inside the same repo.
// Signature keeps the "<author time>: <subject>" form readers parse, with
// the subject whole.
func makeCommitNode(sha string, meta types.CommitMeta, filePath string) types.Node {
	qname := "commit:" + sha
	displayName := sha
	if len(displayName) > 12 {
		displayName = displayName[:12]
	}
	return types.Node{
		ID:            parse.MakeID(qname, "git", 0),
		Type:          types.NodeCommit,
		Name:          displayName,
		QualifiedName: qname,
		FilePath:      filePath,
		StartLine:     1,
		EndLine:       1,
		StartByte:     0,
		EndByte:       1,
		Language:      "git", // sentinel for meta nodes (no source language). Picked specifically so DistinctFilePaths(language=go|ts|sol) audit queries don't see commit nodes — keeps the audit's per-language file-set diff clean.
		SubKind:       "git",
		Signature:     fmt.Sprintf("%d: %s", meta.AuthorTime, meta.Subject),
		Confidence:    types.ConfExtracted,
		Commit:        &meta,
	}
}

// subjectsOf maps every walked commit to its subject.
func subjectsOf(hist *temporal.LineHistory) map[string]string {
	out := make(map[string]string, len(hist.Commits))
	for sha, c := range hist.Commits {
		out[sha] = c.Subject
	}
	return out
}

// modifiesEdges links each hunk to the declarations it changed: a line the
// hunk added, replaced or deleted (context lines do not count) lies in the
// declaration's lines as they were in the hunk's commit. That is the
// changed_in rule (temporal.LineChange.Touches) applied to one hunk, so a
// hunk only modifies a declaration whose line history holds its commit.
func modifiesEdges(hunks []temporal.HunkInfo, hist *temporal.LineHistory, nodeOf map[string]types.Node, srcRel string) []types.Edge {
	type at struct{ sha, path string }
	type declAt struct {
		id         string
		start, end int
	}
	prefix := srcRelPrefix(srcRel)
	byCommitFile := map[at][]declAt{}
	for _, id := range slices.Sorted(maps.Keys(hist.Touches)) {
		n, ok := nodeOf[id]
		if !ok || n.Type == types.NodeFile {
			continue
		}
		for _, tc := range hist.Touches[id] {
			rel := strings.TrimPrefix(tc.Path, prefix)
			byCommitFile[at{tc.SHA, rel}] = append(byCommitFile[at{tc.SHA, rel}], declAt{id, tc.Start, tc.End})
		}
	}
	var out []types.Edge
	for _, h := range hunks {
		decls := byCommitFile[at{h.SHA, h.FilePath}]
		if len(decls) == 0 || h.Binary {
			continue
		}
		changes, ok := temporal.PatchChanges(h.Patch)
		if !ok {
			continue
		}
		hunkID := makeHunkNode(h, types.ConfExtracted).ID
		for _, d := range decls {
			if touchesAny(changes, d.start, d.end) {
				out = append(out, types.Edge{Src: hunkID, Dst: d.id, Type: types.EdgeModifies,
					FilePath: h.FilePath, Count: 1, Confidence: types.ConfExtracted})
			}
		}
	}
	return out
}

func touchesAny(changes []temporal.LineChange, start, end int) bool {
	for _, c := range changes {
		if c.Touches(start, end) {
			return true
		}
	}
	return false
}

// hunkWindow describes the commits whose hunks were stored.
func hunkWindow(hunks []temporal.HunkInfo, hist *temporal.LineHistory) persist.HunkWindow {
	w := persist.HunkWindow{Limit: temporal.HunkCommitsDefault}
	seen := map[string]bool{}
	for _, h := range hunks {
		if seen[h.SHA] {
			continue
		}
		seen[h.SHA] = true
		c := hist.Commits[h.SHA]
		if w.Newest == "" || c.AuthorTime > w.NewestTime {
			w.Newest, w.NewestTime = h.SHA, c.AuthorTime
		}
		if w.Oldest == "" || c.AuthorTime < w.OldestTime {
			w.Oldest, w.OldestTime = h.SHA, c.AuthorTime
		}
	}
	w.Commits = len(seen)
	return w
}

// historyRecord is the manifest's account of the history read.
func historyRecord(hist *temporal.LineHistory, depth int, window persist.HunkWindow, spaces temporal.NumberSpaces, recorded, unreachable int) *persist.HistoryRecord {
	tracked := make([]string, 0, len(lineHistoryNodeTypes))
	for t := range lineHistoryNodeTypes {
		tracked = append(tracked, string(t))
	}
	sort.Strings(tracked)
	return &persist.HistoryRecord{
		Commits:      len(hist.Order),
		Recorded:     recorded,
		Unreachable:  unreachable,
		RecordedRule: recordedRule,
		Tracked:      tracked,
		DepthPerNode: depth,
		Renames:      renamesRule,
		Boundaries:   hist.Boundaries,
		HunkWindow:   window,
		NumberSpaces: persist.NumberSpaceDoc{
			Repo: spaces.Repo, UpstreamRepo: spaces.UpstreamRepo,
			UpstreamBase: spaces.UpstreamBase, Rule: spaces.Rule() + "; " + earlierNumberRule,
		},
	}
}

// prRefsFromHistory lists, per node, the pull requests whose commits changed
// the node's lines, newest first: the node_prs breadcrumbs. A number is kept
// once per node (node_prs is keyed by node and number); when two number
// spaces share a number the newer commit's pull request is kept.
func prRefsFromHistory(hist *temporal.LineHistory, nodeOf map[string]types.Node, cm *commitMinter) map[string][]types.PRRef {
	out := map[string][]types.PRRef{}
	for id := range nodeOf {
		for _, tc := range hist.Touches[id] {
			c := hist.Commits[tc.SHA]
			meta := cm.commitMeta(c)
			if meta.PR == 0 {
				continue
			}
			ref := types.PRRef{
				Number:      meta.PR,
				Title:       cleanPRTitle(c.Subject),
				Summary:     meta.Body,
				HeadSHA:     c.SHA,
				MergedAtUTC: time.Unix(c.CommitTime, 0).UTC(),
				Repo:        meta.PRRepo,
			}
			if len(c.Parents) >= 1 {
				ref.BaseSHA = c.Parents[0]
			}
			if len(c.Parents) >= 2 {
				ref.HeadSHA = c.Parents[1]
			}
			out[id] = append(out[id], ref)
		}
	}
	dedupePRRefsByNode(out)
	return out
}

// sortNodesByID sorts in place by ID for deterministic snapshots.
func sortNodesByID(ns []types.Node) {
	sort.Slice(ns, func(i, j int) bool { return ns[i].ID < ns[j].ID })
}

// simpleName is the last part of a node's name, the identifier its
// declaration line holds ("Method" of "Type.Method").
func simpleName(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}
