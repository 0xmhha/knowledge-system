package mcphandlers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

const defaultChangeHistoryK = 20

// historyKey and historyUnavailable are the key and the text that
// change_history and evidence_for_intent add to their answer when the graph
// was built from a source without git history. Both tools read only what
// the build took from git (commits, commit hunks), so without the key an
// empty answer could not be told from "no change matched".
const (
	historyKey         = "history"
	historyUnavailable = "not recorded: the graph was built from a source without git history, so no commit or pull request is known for any symbol"
)

// Keys of a change_history answer.
const (
	keyCommits      = "commits"
	keyCommitsTotal = "commits_total"
	keyWithPR       = "commits_with_pr"
	keyCut          = "cut"
	keyCoverage     = "coverage"
	keyNote         = "note"
	keyPRs          = "prs"
)

// historyNote returns historyUnavailable when the graph records no git
// history, "" otherwise. A build records the commit of its source in the
// manifest (src_commit) when the source root is a git checkout with
// commits, and it reads commits and pull requests only then: an empty
// src_commit means none were read. A manifest that cannot be read says
// nothing either way, so no note is given.
func historyNote(reader store.Reader) string {
	m, err := store.GetManifest(reader)
	if err != nil || m.CommitHash != "" {
		return ""
	}
	return historyUnavailable
}

// changeHistoryDescription is the change_history tool description.
const changeHistoryDescription = "Change history of a symbol or file: the commits that changed its lines (the commits git log -L lists for them, renames followed), newest first, " +
	"each with sha, author, author_time, committer_time, the whole subject and, when the subject names one, its pull request (number, repo, upstream). " +
	"Each commit also gives path and line: where the symbol was in that commit (the file's name then, after a rename). Commits without a pull request number are listed too; commits_total counts all of them, commits_with_pr those with one, and cut says what a limit left out and how to get it. " +
	"History is recorded for files and declarations (functions, methods, types, contracts); a field, variable or statement has none, and the note names its declaration. " +
	"A file's history lists the commits that changed lines it holds today (git log -L over the whole file), not every commit that touched it (git log --follow). " +
	"Every answer that resolves to a symbol or file carries merge_rule (which commits of a merged history count) and coverage; a declaration whose file was created as a copy of another, with its own lines (including the line that names it) unchanged in the copy, has the commit that made the copy marked copy: true. " + copyHistoryNote + " " +
	"When branches were merged, the list can differ from git log -L for about 1 symbol in 200 on this repository: a commit on a merged branch whose change did not reach the merged result, or a second copy of a commit that also landed on another branch, may be listed extra or left out; merge_rule says so in every answer and check gives the git log -L command for the symbol. " +
	"qname may be a full qualified_name or a bare short name (suffix-resolved; an ambiguous bare name returns candidates). " +
	"Optional `cutoff` (RFC3339) returns only commits made strictly before it; `k` caps the count (default 20). " +
	"Use this to answer \"when/why did this code change\" without reading git. " +
	"When the graph was built from a source without git history the answer has history: \"" + historyUnavailable + "\"."

// RegisterChangeHistory wires the change_history tool: the commits that
// changed a symbol's lines, read from its changed_in edges and their Commit
// nodes, with the pull request breadcrumbs (Reader.GetNodePRs) beside them.
func RegisterChangeHistory(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("change_history",
		mcp.WithDescription(changeHistoryDescription),
		mcp.WithString("qname", mcp.Required()),
		mcp.WithString("cutoff"),
		mcp.WithNumber("k", mcp.DefaultNumber(defaultChangeHistoryK)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetString("qname", "")
		k := int(req.GetFloat("k", defaultChangeHistoryK))
		var cutoff time.Time
		if cs := req.GetString("cutoff", ""); cs != "" {
			if t, err := time.Parse(time.RFC3339, cs); err == nil {
				cutoff = t
			}
		}
		res, err := changeHistory(reader, q, cutoff, k)
		if err != nil {
			return nil, err
		}
		if note := historyNote(reader); note != "" {
			res[historyKey] = note
		}
		return textResult(res), nil
	})
}

// historyCommit is one commit of a change_history answer.
type historyCommit struct {
	SHA           string `json:"sha"`
	Author        string `json:"author"`
	AuthorTime    string `json:"author_time"`
	CommitterTime string `json:"committer_time"`
	Subject       string `json:"subject"`
	Path          string `json:"path"`
	Line          int    `json:"line"`
	// PastCopy is set on a commit older than the copy that created the
	// file: git log -L does not follow every copy, so it may not list it.
	PastCopy bool `json:"past_copy,omitempty"`
	// Copy is set on the commit that created the file as a copy of another.
	Copy       bool       `json:"copy,omitempty"`
	PR         *historyPR `json:"pr,omitempty"`
	commitTime int64
}

// historyPR is the pull request a commit subject names, in its number space.
// Every entry carries every key: Repo is unknownRepo when the repository is
// not known; Ported lists the earlier numbers of a port's subject, pull
// requests of PortedRepo.
type historyPR struct {
	Number      int    `json:"number"`
	Repo        string `json:"repo"`
	Upstream    bool   `json:"upstream"`
	Ported      []int  `json:"ported"`
	PortedRepo  string `json:"ported_repo"`
	Reverts     []int  `json:"reverts"`
	RevertsRepo string `json:"reverts_repo"`
}

// Reliability of the history through copies, as measured on the go-stablenet
// fork of go-ethereum (15,110 commits) on 2026-11-27, against git log -L and
// by reading the diffs: of the commits under before_copy that were read,
// about 3 in 4 changed the symbol; of 1,078 symbols whose history runs
// through a copy, 3 missed commits behind a copy followed by a rename.
// They describe that repository, not a general rate.
const copyHistoryNote = "At a copy the main list can be shorter than git log -L. " +
	"before_copy continues the history through the copy into the source file: it can hold commits git log -L lists as well as older ones it does not, " +
	"and on the go-stablenet repository (measured 2026-11-27) about 3 in 4 of its commits changed this symbol, so read a commit's diff before relying on it. " +
	"A symbol whose history runs through a copy followed by a rename can miss a few commits (3 of 1,078 such symbols there)."

// mergeRule is the per-answer statement of which commits of a merged
// history count, and how far that is from git log -L.
const mergeRule = "how far to trust this list: on this repository about 1 symbol in 200 differs from git log -L, where branches were merged: " +
	"a commit made on a branch whose change did not survive the merge (for example a change and its revert), or a duplicate of a commit that was also applied on another branch, may be listed extra or be missing. " +
	"A commit marked copy created the file as a copy of another file. " + copyHistoryNote + " " +
	"To compare one symbol with git, run the command in check"

// keyCheck is the key of the git command that lists a symbol's commits.
const keyCheck = "check"

// checkCommand prints the git log -L command that lists the commits of n.
func checkCommand(reader store.Reader, n types.Node) string {
	start, end := n.StartLine, n.EndLine
	if n.Type == types.NodeFile {
		start, end = 1, fileLineCount(reader, n.FilePath)
	}
	return fmt.Sprintf("git log --format='%%H %%s' --no-patch -L%d,%d:%s", start, end, n.FilePath)
}

// fileLineCount counts the lines of a file of the source root; when the file
// cannot be read, the last line any node of the file reaches.
func fileLineCount(reader store.Reader, path string) int {
	if man, err := reader.GetManifest(); err == nil && man.SrcRoot != "" {
		if b, err := os.ReadFile(filepath.Join(man.SrcRoot, filepath.FromSlash(path))); err == nil {
			n := bytes.Count(b, []byte{'\n'})
			if len(b) > 0 && b[len(b)-1] != '\n' {
				n++
			}
			return n
		}
	}
	last := 1
	if inFile, err := reader.NodesByFilePath(path); err == nil {
		for _, x := range inFile {
			last = max(last, x.EndLine)
		}
	}
	return last
}

// keyBeforeCopy is the key of the tool's extension beyond git log -L.
const keyBeforeCopy = "before_copy"

// beforeCopy lists the commits older than the copy that created the file,
// made to the copy's source: git log -L does not list them unless it follows
// the copy, so they are kept out of the main list.
type beforeCopy struct {
	Count      int             `json:"count"`
	CopyCommit string          `json:"copy_commit,omitempty"`
	SourcePath string          `json:"source_path,omitempty"`
	Commits    []historyCommit `json:"commits"`
}

// splitBeforeCopy moves the past_copy commits out of the main list.
func splitBeforeCopy(all []historyCommit) ([]historyCommit, beforeCopy) {
	var main []historyCommit
	var b beforeCopy
	for _, c := range all {
		switch {
		case c.PastCopy:
			b.Commits = append(b.Commits, c)
			if b.SourcePath == "" {
				b.SourcePath = c.Path
			}
		case c.Copy && b.CopyCommit == "":
			b.CopyCommit = c.SHA
			main = append(main, c)
		default:
			main = append(main, c)
		}
	}
	b.Count = len(b.Commits)
	if main == nil {
		main = []historyCommit{}
	}
	return main, b
}

// unknownRepo names a number space whose repository the build was not told.
const unknownRepo = "unknown (set history.repo in the scope profile)"

// fileHistoryMeaning is what a File node's history lists.
const fileHistoryMeaning = "a file's history is the commits that changed lines the file holds today (git log -L1,<last line>, renames followed): " +
	"a commit all of whose lines were later replaced or removed is not listed; git log --follow lists every commit that touched the file"

// keyMeaning and keyPromoted are further keys of a change_history answer.
const (
	keyMergeRule = "merge_rule"
	keyMeaning   = "meaning"
	keyPromoted  = "promoted"
)

// historyCut says what a limit left out of the answer.
type historyCut struct {
	By      string `json:"by"`
	Kept    int    `json:"kept"`
	Total   int    `json:"total"`
	GetRest string `json:"get_rest"`
}

// changeHistory resolves qname and lists the commits that changed the
// resolved node(s), newest first, capped at k. Seed resolution mirrors the
// traversal tools — bare names resolve by suffix, multi-match returns
// `ambiguous` + candidates, no match returns `not_found`.
func changeHistory(reader store.Reader, qname string, cutoff time.Time, k int) (map[string]any, error) {
	in := classifyHistoryInput(reader, qname)
	if in.note != "" {
		return map[string]any{
			"seed_qname": qname, "not_found": true, keyCommits: []historyCommit{}, keyPRs: []map[string]any{},
			"input": in.kind, keyNote: in.note,
		}, nil
	}
	qname = in.qname
	resolved, cands, ambiguous, ok := resolveSeed(reader, qname, "")
	if miss := seedMiss(qname, resolved, cands, ambiguous, ok); miss != nil {
		// A history answer has commits where a traversal has nodes and edges.
		delete(miss, "nodes")
		delete(miss, "edges")
		miss[keyCommits] = []historyCommit{}
		miss[keyPRs] = []map[string]any{}
		if ambiguous {
			miss["input"] = inputAmbiguous
			miss[keyNote] = "taken for a symbol name that several nodes carry: call change_history again with one of the candidates"
		} else {
			miss["input"] = inputSymbol
			miss[keyNote] = "taken for a symbol name, which no node of the graph has: use find_symbol to look for it, or give a file path relative to the source root"
		}
		return miss, nil
	}
	nodes, err := resolved.nodes(reader)
	if err != nil {
		return nil, err
	}
	man, _ := reader.GetManifest() // a missing manifest only loses the coverage text
	all, err := nodeCommits(reader, nodes, cutoff)
	if err != nil {
		return nil, err
	}
	commits, before := splitBeforeCopy(all)
	prs, err := nodePRs(reader, nodes, cutoff)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		keyCommitsTotal: len(commits),
		keyWithPR:       countWithPR(commits),
		keyCoverage:     historyCoverage(man.History),
		keyMergeRule:    mergeRule,
	}
	if len(nodes) > 0 && nodes[0].StartLine > 0 {
		out[keyCheck] = checkCommand(reader, nodes[0])
	}
	var cuts []historyCut
	if d := depthCap(man.History); d > 0 && len(commits) >= d {
		cuts = append(cuts, historyCut{By: "temporal_depth", Kept: d, Total: len(commits),
			GetRest: fmt.Sprintf("the build kept the newest %d commits per node (ckg build --temporal-depth %d); older ones are not recorded: rebuild with --temporal-depth 0 to record all", d, d)})
	}
	if k > 0 && len(commits) > k {
		cuts = append(cuts, historyCut{By: "k", Kept: k, Total: len(commits),
			GetRest: fmt.Sprintf("call change_history again with k=%d for all of them", len(commits))})
		commits = commits[:k]
	}
	if k > 0 && len(prs) > k {
		prs = prs[:k]
	}
	if len(cuts) > 0 {
		out[keyCut] = cuts
	}
	out[keyCommits] = commits
	out[keyPRs] = prs
	if len(nodes) > 0 && nodes[0].Type == types.NodeFile {
		out[keyMeaning] = fileHistoryMeaning
	}
	if decl, ok := promotedFrom(reader, nodes); ok {
		out[keyPromoted] = fmt.Sprintf("this is a method promoted from the embedded %s, which is declared on these lines; the history is that declaration's", persist.PrintedName(reader, decl))
	}
	if len(before.Commits) > 0 {
		out[keyBeforeCopy] = before
	}
	if len(commits) == 0 {
		out[keyNote] = emptyHistoryNote(reader, nodes, man.History, cutoff)
	}
	return resolved.result(out), nil
}

// nodeCommits lists the commits the nodes' changed_in edges reach, deduped
// and newest first (committer time, then author time, then sha).
func nodeCommits(reader store.Reader, nodes []types.Node, cutoff time.Time) ([]historyCommit, error) {
	ids, own := historyNodeIDs(nodes)
	edges, err := reader.QueryEdgesForNodes(ids)
	if err != nil {
		return nil, fmt.Errorf("change_history edges: %w", err)
	}
	var dst []string
	place := map[string]types.Edge{}
	for _, e := range edges {
		if _, seen := place[e.Dst]; e.Type == types.EdgeChangedIn && own[e.Src] && !seen {
			place[e.Dst] = e
			dst = append(dst, e.Dst)
		}
	}
	commitNodes, err := reader.NodesByIDs(dst)
	if err != nil {
		return nil, fmt.Errorf("change_history commits: %w", err)
	}
	out := make([]historyCommit, 0, len(commitNodes))
	for _, c := range commitNodes {
		if c.Type != types.NodeCommit || c.Commit == nil {
			continue
		}
		m := c.Commit
		if !cutoff.IsZero() && !time.Unix(m.CommitTime, 0).Before(cutoff) {
			continue
		}
		hc := historyCommit{
			SHA: strings.TrimPrefix(c.QualifiedName, "commit:"), Author: m.Author,
			AuthorTime:    time.Unix(m.AuthorTime, 0).UTC().Format(time.RFC3339),
			CommitterTime: time.Unix(m.CommitTime, 0).UTC().Format(time.RFC3339),
			Subject:       m.Subject, commitTime: m.CommitTime,
			Path: place[c.ID].FilePath, Line: place[c.ID].Line,
			PastCopy: place[c.ID].DispatchKind == types.DispatchPastCopy,
			Copy:     place[c.ID].DispatchKind == types.DispatchCopy,
		}
		if m.PR > 0 {
			hc.PR = &historyPR{Number: m.PR, Repo: orUnknown(m.PRRepo), Upstream: m.Upstream,
				Ported: m.Ported, PortedRepo: m.PortedRepo}
			hc.PR.Reverts = m.Reverts
			if hc.PR.Ported == nil {
				hc.PR.Ported = []int{}
			}
			if hc.PR.Reverts == nil {
				hc.PR.Reverts = []int{}
			}
			if len(m.Ported) > 0 {
				hc.PR.PortedRepo = orUnknown(m.PortedRepo)
			}
			if len(m.Reverts) > 0 {
				hc.PR.RevertsRepo = orUnknown(m.RevertsRepo)
			}
		}
		out = append(out, hc)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].commitTime != out[j].commitTime {
			return out[i].commitTime > out[j].commitTime
		}
		if out[i].AuthorTime != out[j].AuthorTime {
			return out[i].AuthorTime > out[j].AuthorTime
		}
		return out[i].SHA < out[j].SHA
	})
	return out, nil
}

// nodePRs aggregates the nodes' pull request breadcrumbs, one per (repo,
// number), newest first.
func nodePRs(reader store.Reader, nodes []types.Node, cutoff time.Time) ([]map[string]any, error) {
	seen := map[string]bool{}
	prs := []map[string]any{}
	for _, n := range nodes {
		refs, err := reader.GetNodePRs(n.ID, cutoff)
		if err != nil {
			return nil, err
		}
		for _, r := range refs {
			key := fmt.Sprintf("%s#%d", r.Repo, r.Number)
			if seen[key] {
				continue
			}
			seen[key] = true
			prs = append(prs, map[string]any{
				"number":    r.Number,
				"title":     r.Title,
				"summary":   r.Summary,
				"merged_at": r.MergedAtUTC.Format(time.RFC3339),
				"repo":      r.Repo,
				"head_sha":  r.HeadSHA,
				"base_sha":  r.BaseSHA,
			})
		}
	}
	// RFC3339-UTC strings sort lexicographically == chronologically.
	sort.SliceStable(prs, func(i, j int) bool {
		return prs[i]["merged_at"].(string) > prs[j]["merged_at"].(string)
	})
	return prs, nil
}

func orUnknown(repo string) string {
	if repo == "" {
		return unknownRepo
	}
	return repo
}

func historyNodeIDs(nodes []types.Node) ([]string, map[string]bool) {
	ids := make([]string, 0, len(nodes))
	own := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
		own[n.ID] = true
	}
	return ids, own
}

func countWithPR(cs []historyCommit) int {
	n := 0
	for _, c := range cs {
		if c.PR != nil {
			n++
		}
	}
	return n
}

func depthCap(h *persist.HistoryRecord) int {
	if h == nil {
		return 0
	}
	return h.DepthPerNode
}

// historyCoverage says what the recorded history covers, from the manifest.
func historyCoverage(h *persist.HistoryRecord) string {
	if h == nil {
		return "unknown: the graph's manifest has no history record"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d commits of HEAD's history were walked; renames %s", h.Commits, h.Renames)
	if len(h.Boundaries) > 0 {
		fmt.Fprintf(&b, "; the clone is shallow and its history stops at %s, so older commits are missing", strings.Join(h.Boundaries, ", "))
	}
	if h.DepthPerNode > 0 {
		fmt.Fprintf(&b, "; at most %d commits are recorded per node (newest first)", h.DepthPerNode)
	}
	fmt.Fprintf(&b, ". Pull request numbers: %s", h.NumberSpaces.Rule)
	return b.String()
}

// emptyHistoryNote says why no commit is listed: the node kind has no
// recorded history, the cutoff left everything out, or no recorded commit
// changed these lines — with the file's own count, so "the history exists but
// did not touch this" is told from "nothing is recorded here".
func emptyHistoryNote(reader store.Reader, nodes []types.Node, h *persist.HistoryRecord, cutoff time.Time) string {
	if h != nil && len(nodes) > 0 && !tracksKind(h, nodes[0].Type) {
		if decl, ok := enclosingDeclaration(reader, h, nodes[0]); ok {
			name := persist.PrintedName(reader, decl)
			return fmt.Sprintf("history is recorded for %s nodes only; a %s has none. It lies in %s %s (%s:%d-%d): change_history {\"qname\": %q} returns that declaration's history",
				strings.Join(h.Tracked, ", "), nodes[0].Type, decl.Type, name, decl.FilePath, decl.StartLine, decl.EndLine, name)
		}
		return fmt.Sprintf("history is recorded for %s nodes only; a %s has none and lies in no declaration: change_history {\"qname\": %q} returns the history of its file",
			strings.Join(h.Tracked, ", "), nodes[0].Type, fileQName(reader, nodes[0].FilePath))
	}
	if !cutoff.IsZero() {
		return "no commit before the cutoff changed these lines"
	}
	if len(nodes) == 0 {
		return "no commit is recorded"
	}
	fileCommits := 0
	if fileNodes, err := reader.NodesByFilePath(nodes[0].FilePath); err == nil {
		for _, f := range fileNodes {
			if f.Type == types.NodeFile {
				if cs, err := nodeCommits(reader, []types.Node{f}, time.Time{}); err == nil {
					fileCommits = len(cs)
				}
			}
		}
	}
	note := fmt.Sprintf("no recorded commit changed these lines; %d recorded commits changed the file %s", fileCommits, nodes[0].FilePath)
	if h != nil && len(h.Boundaries) > 0 {
		note += "; the clone's history stops at its shallow boundary, so the commits that wrote these lines are older than the clone"
	}
	return note
}

func tracksKind(h *persist.HistoryRecord, t types.NodeType) bool {
	for _, k := range h.Tracked {
		if k == string(t) {
			return true
		}
	}
	return false
}

// commitQNamePrefix starts a Commit node's qualified name.
const commitQNamePrefix = "commit:"

// historyInput is what classifyHistoryInput makes of a change_history
// qname: the name to resolve, or, for an input that names no symbol or
// file of the graph, its kind and a note saying what it was taken for and
// what to call instead.
type historyInput struct {
	qname, kind, note string
}

// Input kinds a change_history answer names.
const (
	inputEmpty      = "empty"
	inputCommit     = "commit"
	inputDirectory  = "directory"
	inputFileLine   = "file_and_line"
	inputNoFile     = "file_not_in_graph"
	inputWrongCase  = "file_case_differs"
	inputNotSource  = "not_a_source_file"
	inputOutsideSrc = "outside_source_root"
	inputSymbol     = "symbol"
	inputHunk       = "hunk"
	inputBadLine    = "bad_line"
	inputAmbiguous  = "ambiguous_symbol"
)

// hunkQNamePrefix starts a Hunk node's qualified name.
const hunkQNamePrefix = "hunk:"

var (
	shaInput = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	// lineSuffixInput is "file:line" or "file@line", the line part as
	// written (it may not be a number).
	lineSuffixInput = regexp.MustCompile(`^(.+\.[A-Za-z]+)[:@]([^:@/\\]*)$`)
	// pathLike: a separator, or a final ".ext" that is not a Go
	// identifier's member access (a qualified name has no lowercase-only
	// extension right after a slash-free name of a file).
	pathLike = regexp.MustCompile(`[/\\]|\.(go|sol|ts|tsx|proto|mod|sum|md|txt|json|yaml|yml|toml|sh|c|h|s)$`)
)

// sourceExts are the file extensions the graph has File nodes for.
var sourceExts = map[string]bool{".go": true, ".sol": true, ".ts": true, ".tsx": true, ".proto": true}

// commitNote is the answer to a commit given as the input.
const commitNote = "taken for a commit: a commit has no change history of its own: change_history answers for a symbol or a file. " +
	"To see what a commit changed, call evidence_for_intent with its subject as the intent (its hit lists the commit's hunks or the declarations it changed), " +
	"or change_history on one of those declarations or files"

// classifyHistoryInput is the one place that reads a change_history input.
// A name the symbol resolution takes passes through. A path is cleaned once
// (cleanHistoryPath) and then: one file of the graph becomes its File node's
// name; "file:line" and "file@line" become the declaration holding that
// line; a commit, a directory, a path in another case, a file that is not a
// source file, a path outside the source root or one the graph does not
// hold get a note.
func classifyHistoryInput(reader store.Reader, q string) historyInput {
	t := strings.TrimSpace(q)
	switch {
	case t == "":
		return historyInput{kind: inputEmpty, note: "the input is empty: give a symbol's qualified name (pkg.Type.Method) or a file path relative to the source root"}
	case strings.HasPrefix(t, commitQNamePrefix):
		return historyInput{kind: inputCommit, note: commitNote}
	case strings.HasPrefix(t, hunkQNamePrefix):
		return historyInput{kind: inputHunk, note: "taken for a hunk: a hunk is one change of one commit and has no history of its own; " +
			"evidence_for_intent lists hunks with their commits, and change_history on a declaration the hunk modifies gives that declaration's commits"}
	}
	if _, _, ambiguous, ok := resolveSeed(reader, t, ""); ok || ambiguous {
		return historyInput{qname: t}
	}
	if shaInput.MatchString(t) {
		return historyInput{kind: inputCommit, note: commitNote}
	}
	if !pathLike.MatchString(t) && !lineSuffixInput.MatchString(t) {
		return historyInput{qname: t}
	}
	line := 0
	if m := lineSuffixInput.FindStringSubmatch(t); m != nil {
		n, err := strconv.Atoi(m[2])
		if err != nil || n < 1 {
			return historyInput{kind: inputBadLine, note: fmt.Sprintf("taken for %s at line %q: line numbers are whole numbers starting at 1 (file:line or file@line)", boundedEcho(m[1]), boundedEcho(m[2]))}
		}
		t, line = m[1], n
	}
	p, outside := cleanHistoryPath(reader, t)
	if outside != "" {
		return historyInput{kind: inputOutsideSrc, note: fmt.Sprintf("taken for the path %s, which is outside the source root %s: give a path under it, or relative to it", boundedEcho(t), outside)}
	}
	name := fileNodeName(reader, p)
	if line > 0 {
		if decl, ok := declarationAt(reader, p, line); ok {
			return historyInput{qname: persist.PrintedName(reader, decl)}
		}
		if name != "" {
			return historyInput{kind: inputFileLine, note: fmt.Sprintf("taken for line %d of %s, where no declaration with recorded history lies: change_history {\"qname\": %q} returns the file's history", line, boundedEcho(p), name)}
		}
	}
	if name != "" {
		return historyInput{qname: name}
	}
	all := allFilePaths(reader)
	for _, f := range all {
		if strings.EqualFold(f, p) {
			return historyInput{kind: inputWrongCase, note: fmt.Sprintf("taken for the file %s, which differs only in case from %s: paths are case-sensitive; call change_history {\"qname\": %q}", boundedEcho(p), f, f)}
		}
	}
	var under []string
	for _, f := range all {
		if strings.HasPrefix(f, p+"/") {
			under = append(under, f)
		}
	}
	if len(under) > 0 {
		return historyInput{kind: inputDirectory, note: fmt.Sprintf("taken for the directory %s: history is recorded per file, so call change_history on one of its %d files, for example {\"qname\": %q}", boundedEcho(p), len(under), under[0])}
	}
	if !sourceExts[path.Ext(p)] {
		return historyInput{kind: inputNotSource, note: fmt.Sprintf("taken for the file %s, which is not a source file of the graph: history is recorded for Go, Solidity, TypeScript and proto files and their declarations", boundedEcho(p))}
	}
	return historyInput{kind: inputNoFile, note: fmt.Sprintf("taken for the file %s, which is not in the graph at its source commit: a file that was deleted or renamed, or lies outside the build's scope, has no node", boundedEcho(p))}
}

// cleanHistoryPath turns any spelling of one file's path into the graph's,
// once: "\" becomes "/", an absolute path is made relative to the source
// root (symbolic links resolved on both sides), and "./", "//" and ".." are
// folded. outside is the source root when an absolute path lies outside it.
func cleanHistoryPath(reader store.Reader, p string) (clean, outside string) {
	p = strings.ReplaceAll(p, `\`, "/")
	if filepath.IsAbs(p) {
		root := ""
		if man, err := reader.GetManifest(); err == nil {
			root = man.SrcRoot
		}
		rel, ok := relativeTo(root, p)
		if !ok {
			return "", root
		}
		p = rel
	}
	p = path.Clean(p)
	if p == "." {
		p = ""
	}
	if p == ".." || strings.HasPrefix(p, "../") {
		root := "the source root"
		if man, err := reader.GetManifest(); err == nil && man.SrcRoot != "" {
			root = man.SrcRoot
		}
		return "", root
	}
	return strings.TrimPrefix(p, "./"), ""
}

// relativeTo makes absolute path p relative to root, trying the paths as
// given and with symbolic links resolved.
func relativeTo(root, p string) (string, bool) {
	if root == "" {
		return "", false
	}
	roots, ps := []string{root}, []string{p}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		roots = append(roots, r)
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		ps = append(ps, r)
	}
	for _, r := range roots {
		for _, q := range ps {
			if rel, err := filepath.Rel(r, q); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
				return filepath.ToSlash(rel), true
			}
		}
	}
	return "", false
}

// fileNodeName is the name of path's File node, "" when it has none.
func fileNodeName(reader store.Reader, path string) string {
	inFile, err := reader.NodesByFilePath(path)
	if err != nil {
		return ""
	}
	for _, f := range inFile {
		if f.Type == types.NodeFile {
			return persist.PrintedName(reader, f)
		}
	}
	return ""
}

// declarationAt is the smallest declaration with recorded history holding
// line of path.
func declarationAt(reader store.Reader, path string, line int) (types.Node, bool) {
	man, err := reader.GetManifest()
	if err != nil || man.History == nil {
		return types.Node{}, false
	}
	return enclosingDeclaration(reader, man.History, types.Node{FilePath: path, StartLine: line, EndLine: line})
}

// fileLanguages are the languages whose files have File nodes.
var fileLanguages = []string{"go", "ts", "sol", "proto"}

// allFilePaths lists the paths of the graph's File nodes, sorted.
func allFilePaths(reader store.Reader) []string {
	var paths []string
	for _, lang := range fileLanguages {
		ps, err := reader.DistinctFilePaths(lang)
		if err != nil {
			return nil
		}
		paths = append(paths, ps...)
	}
	sort.Strings(paths)
	return paths
}

// fileQName is the name change_history takes for the File node of path.
func fileQName(reader store.Reader, path string) string {
	if inFile, err := reader.NodesByFilePath(path); err == nil {
		for _, f := range inFile {
			if f.Type == types.NodeFile {
				return persist.PrintedName(reader, f)
			}
		}
	}
	return path
}

// enclosingDeclaration finds the smallest declaration with recorded history
// whose lines hold n.
func enclosingDeclaration(reader store.Reader, h *persist.HistoryRecord, n types.Node) (types.Node, bool) {
	inFile, err := reader.NodesByFilePath(n.FilePath)
	if err != nil {
		return types.Node{}, false
	}
	var best types.Node
	found := false
	for _, d := range inFile {
		if d.ID == n.ID || d.Type == types.NodeFile || !tracksKind(h, d.Type) ||
			d.StartLine > n.StartLine || d.EndLine < n.EndLine {
			continue
		}
		if !found || d.EndLine-d.StartLine < best.EndLine-best.StartLine {
			best, found = d, true
		}
	}
	return best, found
}

// promotedFrom reports the declaration a promoted method copies: a Go
// method of an embedding type is recorded on the embedded method's lines,
// without a canonical id of its own.
func promotedFrom(reader store.Reader, nodes []types.Node) (types.Node, bool) {
	if len(nodes) != 1 || nodes[0].CanonicalID != "" || nodes[0].Type != types.NodeMethod {
		return types.Node{}, false
	}
	n := nodes[0]
	inFile, err := reader.NodesByFilePath(n.FilePath)
	if err != nil {
		return types.Node{}, false
	}
	for _, d := range inFile {
		if d.ID != n.ID && d.Type == n.Type && d.CanonicalID != "" && d.StartLine == n.StartLine && d.EndLine == n.EndLine {
			return d, true
		}
	}
	return types.Node{}, false
}
