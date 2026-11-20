package mcphandlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// search_code is grep over the graph: a regular expression over the stored
// source of every declaration, answered per declaration instead of per line,
// ranked, counted, and joined with the graph (the calls and value uses on each
// matched line, and the callers, callees or users of each hit).
//
// Why per declaration: an agent greps to find the function to look at next;
// grep's line list makes it read and group the lines itself and then ask for
// the callers in another round. Here one call returns the functions, the
// lines that matched in each, which alternatives of the pattern they matched
// (a function that matches several is likelier the place), and the next hop.
//
// The index is the graph's own blobs, so it covers the build's files as of the
// last build: not the working tree, not packages outside the build scope.

const (
	searchCodeMaxPattern  = 1000
	searchCodeLinesPerHit = 3 // search_text's pattern fallback only
	searchCodeExpandCap   = 10
)

// searchCodePageBytes bounds one search_code response. search_code returns
// every matched declaration and every matched line; a result larger than
// this is sent in parts (page). Claude Code replaces an MCP tool result over
// 25,000 tokens (MAX_MCP_OUTPUT_TOKENS) with an error and a file path the
// agent may not be able to read. In agent runs (Claude Code 2.1.326) results
// of 41,252 characters reached the agent and results of 58,996 characters
// and more did not, so a part stays under 40,000 bytes.
var searchCodePageBytes = 40000

// searchCodeDir is where a result too large for one response is saved as a
// file; the response then holds only the file's path (see fileSearchCode).
// Empty keeps the index response.
var searchCodeDir string

// SetSearchCodeDir sets the folder for search_code results saved as files.
// The agent must be allowed to read it (Claude Code: --add-dir).
func SetSearchCodeDir(dir string) { searchCodeDir = dir }

// searchCodeMaxResultChars, when positive, is sent on the search_code tool
// as _meta["anthropic/maxResultSizeChars"]: Claude Code 2.1.326 reads it
// per tool to decide when a result is saved to a file with a 2 KB preview
// instead of being given to the model (a 94,056-character result was saved
// that way even with MAX_MCP_OUTPUT_TOKENS raised). 0 sends nothing.
var searchCodeMaxResultChars int

// SetSearchCodeMaxResultChars sets the result size search_code declares to
// the client; 0 or less declares none.
func SetSearchCodeMaxResultChars(n int) { searchCodeMaxResultChars = max(n, 0) }

// searchCodeNameAsBoth answers in=name with in=both: name matches first,
// then body matches. Agents that started with in=name got only declarations
// whose name matches, and in recorded sessions those starts went with low
// recall (sym-77: 16 sessions 0.22 against 0.65 for body starts); replaying
// the recorded in=name calls as in=both put 24 instead of 8 cores in the
// result (runs/20261119-ks-1e1003c0-offline2/d16).
var searchCodeNameAsBoth bool

// searchCodeReasons enables the reasons argument: words for why the pattern
// is searched. The graph neighbours of the hits whose source or name holds
// one of them are listed after the hits, under related (offline: present
// cores 74 to 168 on the tune split, 22 to 45 on holdout, with the hits'
// ranks unchanged; runs/20261119-ks-1e1003c0-offline2/d18).
var searchCodeReasons bool

// SetSearchCodeOptions turns the two options on or off.
func SetSearchCodeOptions(nameAsBoth, reasons bool) {
	searchCodeNameAsBoth, searchCodeReasons = nameAsBoth, reasons
}

// SetSearchCodePageBytes changes the part size for clients whose output limit
// differs (MAX_MCP_OUTPUT_TOKENS) and for runs that need results split. A
// value of 0 or less keeps the current size.
func SetSearchCodePageBytes(n int) {
	if n > 0 {
		searchCodePageBytes = n
	}
}

// searchCodeKinds are the declarations whose source is searched. A matched
// line belongs to the innermost one that contains it (a Solidity function,
// not its contract).
var searchCodeKinds = map[types.NodeType]bool{
	types.NodeFunction: true, types.NodeMethod: true, types.NodeStruct: true, types.NodeInterface: true,
	types.NodeTypeAlias: true, types.NodeConstant: true, types.NodeVariable: true, types.NodeContract: true,
	types.NodeModifier: true, types.NodeEvent: true, types.NodeMapping: true, types.NodeEnum: true,
}

type codeDecl struct {
	node  types.Node
	lines []string
	test  bool
}

type codeIndex struct {
	decls  []codeDecl
	byFile map[string][]int // indexes into decls
	idOnce sync.Once
	byID   map[string]int // node id -> index into decls, see byNodeID
}

// buildCodeIndex loads every searchable declaration and its source.
// Function-local variables (no canonical id) are left out: their text is in
// their function.
func buildCodeIndex(reader store.Reader) (*codeIndex, error) {
	idx := &codeIndex{byFile: map[string][]int{}}
	seenFile := map[string]bool{}
	for _, lang := range []string{"go", "sol", "ts", "js"} {
		files, err := reader.DistinctFilePaths(lang)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if seenFile[f] {
				continue
			}
			seenFile[f] = true
			nodes, err := reader.NodesByFilePath(f)
			if err != nil {
				return nil, err
			}
			blobs, err := reader.BlobsByFilePath(f)
			if err != nil {
				return nil, err
			}
			for _, n := range nodes {
				if !searchCodeKinds[n.Type] || (n.Type == types.NodeVariable && n.CanonicalID == "") {
					continue
				}
				src, ok := blobs[n.ID]
				if !ok || len(src) == 0 {
					continue
				}
				idx.byFile[f] = append(idx.byFile[f], len(idx.decls))
				idx.decls = append(idx.decls, codeDecl{node: n, lines: strings.Split(string(src), "\n"),
					test: isTestFile(f)})
			}
		}
	}
	return idx, nil
}

// owner returns the innermost declaration of file f that spans line.
func (x *codeIndex) owner(f string, line int) int {
	best, span := -1, 0
	for _, i := range x.byFile[f] {
		n := x.decls[i].node
		if line < n.StartLine || line > n.EndLine {
			continue
		}
		if s := n.EndLine - n.StartLine; best < 0 || s < span {
			best, span = i, s
		}
	}
	return best
}

// codeIndexes holds one lazily built index per reader, shared by search_code
// and search_text's pattern fallback. A graph opened read-only does not
// change while the server runs.
var codeIndexes sync.Map // store.Reader -> *lazyCodeIndex

type lazyCodeIndex struct {
	once sync.Once
	idx  *codeIndex
	err  error
}

func codeIndexFor(reader store.Reader) (*codeIndex, error) {
	// Each Register* wraps the reader itself; key on the store underneath so
	// the tools of one server share one index.
	key := reader
	if w, ok := reader.(*llmSafeReader); ok {
		key = w.Reader
	}
	v, _ := codeIndexes.LoadOrStore(key, &lazyCodeIndex{})
	l := v.(*lazyCodeIndex)
	l.once.Do(func() { l.idx, l.err = buildCodeIndex(reader) })
	return l.idx, l.err
}

type codeHit struct {
	decl    int
	lines   []int // absolute line numbers, ascending
	terms   map[string]bool
	inName  bool
	matches int
}

// RegisterSearchCode wires search_code (production code only) and
// search_test_code (test code only). Production and test code are read for
// different reasons, and leaving tests out of search_code keeps its results
// smaller (user decision D2).
func RegisterSearchCode(s *server.MCPServer, reader store.Reader) {
	registerCodeSearch(s, reader, "search_code", codeSearchSide{tests: "exclude"},
		"grep over the production code of the graph (test files are not searched; use search_test_code for them): a regular expression (RE2; alternatives with |) over the source of every function, method, type, constant and variable, "+
			"answered per declaration: its qname, file, lines, the matched lines with line numbers, and the distinct texts it matched (lowercased). "+
			"Order: a match in the name first, then more matches, then wider use in the repository (PageRank); declarations equal on these keep no meaningful order. ")
	registerCodeSearch(s, reader, "search_test_code", codeSearchSide{tests: "only"},
		"grep over the test code of the graph (_test.go files and testdata): a regular expression (RE2; alternatives with |) over the source of every test function, helper, type, constant and variable, "+
			"answered per declaration: its qname, file, lines, the matched lines with line numbers, and the distinct texts it matched (lowercased). "+
			"Order: a match in the name first, then more matches, then wider use in the repository (PageRank). ")
}

// codeSearchSide is which code a code search covers.
type codeSearchSide struct {
	tests string // "exclude" (production only) or "only" (tests only)
}

func registerCodeSearch(s *server.MCPServer, reader store.Reader, name string, side codeSearchSide, about string) {
	reader = safeReader(reader)
	params := []mcp.ToolOption{
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDescription(about +
			"A result too large for one response comes as an index: every matched declaration in one short row with its matched line numbers and the uses on them, without code; ask for the code of the ones you choose with ids=[qname, ...] (the server guide explains every key). " +
			"Each matched line lists the calls and value uses the graph resolved on it (resolve), and expand=[\"callers\",\"callees\",\"references\"] attaches the next hop. " +
			"Use it as you would grep: enumerate the names and code shapes you expect (\"Flush|flush|Writer\", \"sql\\\\.Open\\\\(\", \"func \\\\(.*\\\\) \\\\w*Render\\\\w*\\\\(\"). " +
			"Covers the build's files as of the last graph build; packages outside the build and uncommitted edits are not searched."),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("RE2 regular expression; | separates alternatives. With fixed=true, a literal string.")),
		mcp.WithBoolean("fixed", mcp.Description("treat pattern as a literal string")),
		mcp.WithBoolean("ignore_case", mcp.Description("case-insensitive match")),
		mcp.WithBoolean("word", mcp.Description("match whole words only")),
		mcp.WithString("in", mcp.Description("body (default: declaration source, signature included), name (qualified name or the declaration's first line, e.g. \"func (t *Template) Render(\"), or both")),
		mcp.WithString("path", mcp.Description("only files under this path prefix, or matching this glob (* and **)")),
		mcp.WithArray("expand", mcp.Description("attach for each hit: callers, callees, references (who reads or writes a value)"), mcp.WithStringItems()),
		mcp.WithBoolean("resolve", mcp.Description("list the calls and value uses on each matched line (default true)")),
		mcp.WithNumber("context", mcp.Description("lines of context around each matched line (0 to 3, default 0)")),
		mcp.WithArray("ids", mcp.Description("qname (or canonical_id) of declarations to return in full: their matched lines with code, uses and expansion. Use it after a result that came as an index"), mcp.WithStringItems()),
		mcp.WithNumber("page", mcp.Description("part to return when a result asked for with ids is still too large for one response (default 1)")),
	}
	if searchCodeReasons {
		params = append(params, mcp.WithArray("reasons", mcp.Description("words for why you search: the behaviour or mechanism the problem is about, as words or identifier parts (\"retry\", \"expire\", \"cache\"). Declarations next to the hits in the graph (callers, callees, values read or written) whose source or name holds one of them are listed after the hits, under related"), mcp.WithStringItems()))
	}
	tool := nsTool(name, params...)
	if searchCodeMaxResultChars > 0 {
		tool.Meta = &mcp.Meta{AdditionalFields: map[string]any{"anthropic/maxResultSizeChars": searchCodeMaxResultChars}}
	}
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		index, err := codeIndexFor(reader)
		if err != nil {
			return nil, err
		}
		in := req.GetString("in", "body")
		if searchCodeNameAsBoth && in == "name" {
			in = "both"
		}
		var reasons []string
		if searchCodeReasons {
			reasons = req.GetStringSlice("reasons", nil)
		}
		out, err := searchCode(reader, index, searchCodeArgs{
			pattern: req.GetString("pattern", ""), fixed: req.GetBool("fixed", false),
			ignoreCase: req.GetBool("ignore_case", false), word: req.GetBool("word", false),
			in: in, path: req.GetString("path", ""), tests: side.tests, reasons: reasons,
			expand: req.GetStringSlice("expand", nil), resolve: req.GetBool("resolve", true),
			context: int(req.GetFloat("context", 0)), only: req.GetStringSlice("ids", nil), trace: traceFrom(ctx),
		})
		if err != nil {
			return textResult(map[string]any{"pattern": req.GetString("pattern", ""), "error": err.Error(), "hits": []any{}}), nil
		}
		if len(req.GetStringSlice("ids", nil)) == 0 {
			if res, ok := fileSearchCode(out, traceFrom(ctx), name); ok {
				return textResult(res), nil
			}
			if res, ok := indexSearchCode(out, traceFrom(ctx), name); ok {
				return textResult(res), nil
			}
		}
		return textResult(pageSearchCode(out, int(req.GetFloat("page", 1)), traceFrom(ctx), name)), nil
	})
}

type searchCodeArgs struct {
	pattern           string
	fixed, ignoreCase bool
	word, resolve     bool
	in, path, tests   string
	expand            []string
	context           int
	limit             int        // declarations to return; 0 returns all
	linesPerHit       int        // matched lines shown per declaration; 0 shows all
	only              []string   // canonical ids (or qnames) to return in full; empty returns all
	reasons           []string   // words for why the pattern is searched; adds related neighbours
	trace             *callTrace // nil unless the call is traced
}

func compileSearchPattern(a searchCodeArgs) (*regexp.Regexp, error) {
	if a.pattern == "" {
		return nil, fmt.Errorf("pattern is empty")
	}
	if len(a.pattern) > searchCodeMaxPattern {
		return nil, fmt.Errorf("pattern longer than %d characters", searchCodeMaxPattern)
	}
	p := a.pattern
	if a.fixed {
		p = regexp.QuoteMeta(p)
	}
	if a.word {
		p = `\b(?:` + p + `)\b`
	}
	if a.ignoreCase {
		p = `(?i)` + p
	}
	rx, err := regexp.Compile(p)
	if err != nil {
		return nil, fmt.Errorf("pattern is not a valid RE2 expression (Go regexp syntax; no lookaround or backreferences): %v", err)
	}
	return rx, nil
}

func pathFilter(p string) func(string) bool {
	if p == "" {
		return func(string) bool { return true }
	}
	if !strings.ContainsAny(p, "*?[") {
		p = strings.TrimPrefix(p, "./")
		return func(f string) bool { return strings.HasPrefix(f, p) }
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == '*' && i+1 < len(p) && p[i+1] == '*':
			b.WriteString(".*")
			i++
			if i+1 < len(p) && p[i+1] == '/' {
				i++
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	rx := regexp.MustCompile(b.String())
	return rx.MatchString
}

func searchCode(reader store.Reader, idx *codeIndex, a searchCodeArgs) (map[string]any, error) {
	rx, err := compileSearchPattern(a)
	if err != nil {
		return nil, err
	}
	a.context = max(0, min(a.context, 3))
	a.trace.add("search_code.pattern", map[string]any{"regex": rx.String(), "alternatives": topLevelAlternatives(a.pattern),
		"in": a.in, "path": a.path, "tests": a.tests, "limit": a.limit, "context": a.context, "resolve": a.resolve, "expand": a.expand})
	inFile := pathFilter(a.path)
	skippedPath, skippedTests, nestedLines := 0, 0, 0
	hits := map[int]*codeHit{}
	hit := func(i int) *codeHit {
		h, ok := hits[i]
		if !ok {
			h = &codeHit{decl: i, terms: map[string]bool{}}
			hits[i] = h
		}
		return h
	}
	totalMatches := 0
	for i, d := range idx.decls {
		n := d.node
		if !inFile(n.FilePath) {
			skippedPath++
			continue
		}
		if a.tests == "exclude" && d.test || a.tests == "only" && !d.test {
			skippedTests++
			continue
		}
		if a.in == "name" || a.in == "both" {
			// The name is the qualified name or the declaration's first
			// line, so "func .*Round\w*\(" works as it does in grep.
			m := rx.FindString(n.QualifiedName)
			if m == "" && len(d.lines) > 0 {
				m = rx.FindString(d.lines[0])
			}
			if m != "" {
				h := hit(i)
				h.inName = true
				h.terms[strings.ToLower(m)] = true
				h.matches++
				totalMatches++
			}
		}
		if a.in == "name" {
			continue
		}
		for k, line := range d.lines {
			ms := rx.FindAllString(line, -1)
			if len(ms) == 0 {
				continue
			}
			abs := n.StartLine + k
			// A line inside a nested declaration belongs to that one.
			owner := idx.owner(n.FilePath, abs)
			if owner >= 0 && owner != i {
				nestedLines++
				continue
			}
			h := hit(i)
			h.lines = append(h.lines, abs)
			h.matches += len(ms)
			totalMatches += len(ms)
			for _, m := range ms {
				h.terms[strings.ToLower(m)] = true
			}
		}
	}
	// hits is a map; start from index order so ties keep it.
	ranked := make([]*codeHit, 0, len(hits))
	for _, h := range hits {
		ranked = append(ranked, h)
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].decl < ranked[j].decl })
	// Hits equal on every key below stay in index order (the order the
	// store lists files, then line): their order carries no evidence, and
	// sorting them by name put names starting with "a" first (the name
	// decided 1,028 of 8,150 adjacent orders in a replay of 327 recorded
	// calls). The stable sort keeps that order the same on every call.
	sort.SliceStable(ranked, func(i, j int) bool {
		return rankBefore(idx.decls[ranked[i].decl], idx.decls[ranked[j].decl], ranked[i], ranked[j])
	})
	if a.tests == "include" {
		// Only search_text's pattern fallback still mixes tests in; they go
		// after production code there. search_code and search_test_code
		// each search one side.
		sort.SliceStable(ranked, func(i, j int) bool {
			return !idx.decls[ranked[i].decl].test && idx.decls[ranked[j].decl].test
		})
	}
	files := map[string]bool{}
	for _, h := range ranked {
		files[idx.decls[h.decl].node.FilePath] = true
	}
	if a.trace.on() {
		a.trace.add("search_code.scan", map[string]any{"declarations": len(idx.decls), "skipped_by_path": skippedPath,
			"skipped_by_tests": skippedTests, "lines_owned_by_nested_declaration": nestedLines,
			"matched_declarations": len(ranked), "matched_files": len(files), "total_matches": totalMatches})
		a.trace.add("search_code.rank", traceRanking(idx, ranked, keptBy(a.limit, len(ranked))))
	}
	out := map[string]any{"pattern": a.pattern, "total_declarations": len(ranked), "total_files": len(files),
		"total_matches": totalMatches}
	if a.limit > 0 && len(ranked) > a.limit {
		ranked = ranked[:a.limit]
		out["truncated"] = true
	}
	if len(a.only) > 0 {
		// Details of declarations the agent chose from an index response:
		// the same matched lines, uses and expansion, for those only.
		want := map[string]bool{}
		for _, id := range a.only {
			want[id] = true
		}
		found := map[string]bool{}
		kept := make([]*codeHit, 0, len(a.only))
		for _, h := range ranked {
			n := idx.decls[h.decl].node
			for _, k := range []string{n.CanonicalID, n.QualifiedName} {
				if k != "" && want[k] {
					found[k] = true
					kept = append(kept, h)
					break
				}
			}
		}
		ranked = kept
		var missing []string
		for _, id := range a.only {
			if !found[id] {
				missing = append(missing, id)
			}
		}
		if missing != nil {
			out["ids_not_matched"] = missing
		}
		a.trace.add("search_code.ids", map[string]any{"asked": a.only, "returned": len(kept), "not_matched": missing})
	}
	ids := make([]string, len(ranked))
	for i, h := range ranked {
		ids[i] = idx.decls[h.decl].node.ID
	}
	var lineUses map[string]map[int][]string
	var lineExt map[string]map[int][]types.ExternalPackage
	if (a.resolve || a.trace.on()) && len(ids) > 0 {
		lineUses, lineExt = usesByLineExt(reader, ids)
	}
	shownExt := map[string]types.ExternalPackage{}
	var views []map[string]any
	rows := make([]map[string]any, 0, len(ranked))
	for _, h := range ranked {
		d := idx.decls[h.decl]
		n := d.node
		terms := make([]string, 0, len(h.terms))
		for t := range h.terms {
			terms = append(terms, t)
		}
		sort.Strings(terms)
		row := map[string]any{"qname": n.QualifiedName, "type": string(n.Type), "file": n.FilePath,
			"start_line": n.StartLine, "end_line": n.EndLine, "matches": h.matches, "matched": terms}
		if n.CanonicalID != "" {
			row["canonical_id"] = n.CanonicalID
		}
		if n.Signature != "" {
			row["signature"] = n.Signature
		}
		if d.test {
			row["test"] = true
		}
		if h.inName {
			row["name_match"] = true
		}
		var shown []map[string]any
		for k, ln := range h.lines {
			if a.linesPerHit > 0 && k == a.linesPerHit {
				row["more_lines"] = len(h.lines) - k
				break
			}
			from, to := max(ln-a.context, n.StartLine), min(ln+a.context, n.EndLine)
			var text []string
			for l := from; l <= to; l++ {
				if off := l - n.StartLine; off >= 0 && off < len(d.lines) {
					text = append(text, strings.TrimRight(d.lines[off], " \t"))
				}
			}
			m := map[string]any{"line": ln, "text": strings.Join(text, "\n")}
			if uses := lineUses[n.ID][ln]; a.resolve && len(uses) > 0 {
				m["uses"] = uses
				for _, ep := range lineExt[n.ID][ln] {
					shownExt[ep.Path] = ep
				}
			}
			shown = append(shown, m)
		}
		if shown != nil {
			row["lines"] = shown
		}
		rows = append(rows, row)
		if a.trace.on() {
			views = append(views, traceHitView(d, h, a, lineUses[n.ID]))
		}
	}
	if a.trace.on() {
		a.trace.add("search_code.view", map[string]any{"hits": views, "collapsed_uses": collapsedUses(reader, ids)})
	}
	if len(a.expand) > 0 {
		var cut []map[string]any
		for i, h := range ranked {
			for kind, total := range attachExpansion(reader, rows[i], idx.decls[h.decl].node, a.expand) {
				if total > searchCodeExpandCap {
					cut = append(cut, map[string]any{"qname": rows[i]["qname"], "kind": kind, "total": total, "shown": searchCodeExpandCap})
				}
			}
		}
		a.trace.add("search_code.expand", map[string]any{"cut": cut})
	}
	out["hits"] = rows
	if len(a.reasons) > 0 && len(a.only) == 0 && len(ids) > 0 {
		related, words := reasonNeighbours(reader, idx, ids, a.reasons)
		out["related"] = related
		a.trace.add("search_code.related", map[string]any{"reasons": words, "related": len(related)})
	}
	if len(shownExt) > 0 {
		out["packages"] = externalPackageList(shownExt)
	}
	if len(rows) == 0 {
		out["note"] = "no declaration matches; the index holds the build's declarations, not comments outside them, imports or files outside the build"
		// Say whether the filters, not the pattern, emptied the result.
		if a.path != "" || a.tests != "include" || a.in == "name" {
			wide := a
			wide.path, wide.tests, wide.in, wide.expand, wide.resolve, wide.limit, wide.trace = "", "include", "both", nil, false, 1, nil
			if res, err := searchCode(reader, idx, wide); err == nil {
				if n := res["total_declarations"].(int); n > 0 {
					out["without_filters"] = n
					out["note"] = fmt.Sprintf("no declaration matches with these filters; %d match the pattern with in=both and no path or tests filter", n)
				}
			}
		}
	}
	return out, nil
}

// usesByLine maps node id -> line -> "calls X" / "reads X" / "writes X" for
// the resolved edges that start at each node. Calls and value uses have one
// edge per line, so every line that calls or uses something has its entry.
func usesByLine(reader store.Reader, ids []string) map[string]map[int][]string {
	out, _ := usesByLineExt(reader, ids)
	return out
}

// usesByLineExt is usesByLine plus, per node and line, the packages of the
// External targets (schema 1.36) named on that line, so a response can list
// each package it mentions once. An External target is named by its package
// name ("time.AfterFunc"); when two packages of the targets share a name
// (math/rand, crypto/rand) both are named by import path instead.
func usesByLineExt(reader store.Reader, ids []string) (map[string]map[int][]string, map[string]map[int][]types.ExternalPackage) {
	edges, err := reader.QueryEdgesForNodes(ids)
	if err != nil {
		return nil, nil
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var dsts []string
	for _, e := range edges {
		if want[e.Src] && e.Line > 0 {
			dsts = append(dsts, e.Dst)
		}
	}
	targets, err := reader.NodesByIDs(dsts)
	if err != nil {
		return nil, nil
	}
	name := map[string]string{}
	ext := map[string]types.ExternalPackage{}
	pathsByName := map[string]map[string]bool{}
	for _, n := range targets {
		name[n.ID] = n.QualifiedName
		if n.Type == types.NodeExternal && n.External != nil {
			ext[n.ID] = *n.External
			if pathsByName[n.External.Name] == nil {
				pathsByName[n.External.Name] = map[string]bool{}
			}
			pathsByName[n.External.Name][n.External.Path] = true
		}
	}
	for id, ep := range ext {
		if len(pathsByName[ep.Name]) > 1 {
			name[id] = ep.Path + strings.TrimPrefix(name[id], ep.Name)
		}
	}
	runs := dynamicTargets(reader, edges, want)
	out := map[string]map[int][]string{}
	outExt := map[string]map[int][]types.ExternalPackage{}
	seen := map[string]bool{}
	for _, e := range edges {
		if !want[e.Src] || e.Line == 0 || name[e.Dst] == "" {
			continue
		}
		// A call through a function value is stored as an invokes edge
		// back to its own function; dispatches_to (schema 1.31) names what
		// the value can hold, on the same line.
		if e.Type == types.EdgeInvokes && e.Src == e.Dst {
			k := fmt.Sprint(e.Src, e.Line, "fv")
			if !seen[k] {
				seen[k] = true
				if out[e.Src] == nil {
					out[e.Src] = map[int][]string{}
				}
				u := "calls a function value (target not linked)"
				if r := runs[fmt.Sprint(e.Src, "|", e.Line)]; r != "" {
					u = "calls a function value that may run " + r
				}
				out[e.Src][e.Line] = append(out[e.Src][e.Line], u)
			}
			continue
		}
		if e.Type == types.EdgeDispatchesTo {
			continue // shown with the call it belongs to
		}
		verb := ""
		switch e.Type {
		case types.EdgeInvokes:
			verb = "calls "
			if r := runs[e.Dst]; r != "" && e.DispatchKind == "interface_method" {
				u := verb + name[e.Dst] + " (may run " + r + ")"
				k := fmt.Sprint(e.Src, e.Line, u)
				if !seen[k] {
					seen[k] = true
					if out[e.Src] == nil {
						out[e.Src] = map[int][]string{}
					}
					out[e.Src][e.Line] = append(out[e.Src][e.Line], u)
				}
				continue
			}
		case types.EdgeCalls:
			verb = "calls "
		case types.EdgeReadsField:
			verb = "reads "
		case types.EdgeWritesField:
			verb = "writes "
		case types.EdgeReferences:
			verb = "reads "
			if e.DispatchKind == "write" {
				verb = "writes "
			}
		default:
			continue
		}
		u := verb + name[e.Dst]
		k := fmt.Sprint(e.Src, e.Line, u)
		if seen[k] {
			continue
		}
		seen[k] = true
		if out[e.Src] == nil {
			out[e.Src] = map[int][]string{}
		}
		out[e.Src][e.Line] = append(out[e.Src][e.Line], u)
		if ep, ok := ext[e.Dst]; ok {
			if outExt[e.Src] == nil {
				outExt[e.Src] = map[int][]types.ExternalPackage{}
			}
			outExt[e.Src][e.Line] = append(outExt[e.Src][e.Line], ep)
		}
	}
	for _, m := range out {
		for l := range m {
			sort.Strings(m[l])
		}
	}
	return out, outExt
}

// externalPackageList renders the packages of the External targets a
// response names, once each, sorted by import path.
func externalPackageList(m map[string]types.ExternalPackage) []map[string]any {
	paths := make([]string, 0, len(m))
	for p := range m {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]map[string]any, 0, len(paths))
	for _, p := range paths {
		ep := m[p]
		row := map[string]any{"name": ep.Name, "path": ep.Path, "module": ep.Module}
		if ep.Version != "" {
			row["version"] = ep.Version
		}
		out = append(out, row)
	}
	return out
}

// dynamicTargets names, from dispatches_to edges (schema 1.31), what each
// dynamic call can run: keyed by interface method id for the interface
// calls among edges, and by "<src>|<line>" for function-value calls of the
// wanted sources. At most dispatchNameCap names, then "and N more".
func dynamicTargets(reader store.Reader, edges []types.Edge, want map[string]bool) map[string]string {
	if !hasDispatch(reader) {
		return nil
	}
	var ifaces []string
	seen := map[string]bool{}
	for _, e := range edges {
		if want[e.Src] && e.Type == types.EdgeInvokes && e.DispatchKind == "interface_method" && !seen[e.Dst] {
			seen[e.Dst] = true
			ifaces = append(ifaces, e.Dst)
		}
	}
	all := append([]types.Edge{}, edges...)
	if len(ifaces) > 0 {
		if more, err := reader.QueryEdgesForNodes(ifaces); err == nil {
			all = append(all, more...)
		}
	}
	lists := map[string][]string{}
	byType := map[string][]string{} // type-hierarchy targets of an interface method
	var dsts []string
	for _, e := range all {
		if e.Type != types.EdgeDispatchesTo {
			continue
		}
		switch {
		case e.DispatchKind == "interface_method" && seen[e.Src]:
			lists[e.Src] = append(lists[e.Src], e.Dst)
		case e.DispatchKind == "type_hierarchy" && seen[e.Src]:
			byType[e.Src] = append(byType[e.Src], e.Dst)
		case e.DispatchKind != "interface_method" && e.DispatchKind != "type_hierarchy" && want[e.Src] && e.Line > 0:
			k := fmt.Sprint(e.Src, "|", e.Line)
			lists[k] = append(lists[k], e.Dst)
		default:
			continue
		}
		dsts = append(dsts, e.Dst)
	}
	if len(lists) == 0 && len(byType) == 0 {
		return nil
	}
	ns, err := reader.NodesByIDs(dsts)
	if err != nil {
		return nil
	}
	name := map[string]string{}
	for _, n := range ns {
		name[n.ID] = n.QualifiedName
	}
	join := func(ids []string) string {
		var names []string
		dup := map[string]bool{}
		for _, id := range ids {
			if q := name[id]; q != "" && !dup[q] {
				dup[q] = true
				names = append(names, q)
			}
		}
		sort.Strings(names)
		if len(names) > dispatchNameCap {
			names = append(names[:dispatchNameCap], fmt.Sprintf("and %d more", len(names)-dispatchNameCap))
		}
		return strings.Join(names, ", ")
	}
	out := map[string]string{}
	for k, ids := range lists {
		out[k] = join(ids)
	}
	// The type hierarchy's candidates come after the value flow's. Next to
	// value-flow targets they are only counted ("A, B; 3 more by type"): on
	// a wide interface they are mostly unrelated types. With no value-flow
	// target they are named ("by type C, D").
	for k, ids := range byType {
		uniq := map[string]bool{}
		for _, id := range ids {
			if q := name[id]; q != "" {
				uniq[q] = true
			}
		}
		if len(uniq) == 0 {
			continue
		}
		if out[k] != "" {
			out[k] += fmt.Sprintf("; %d more by type", len(uniq))
		} else {
			out[k] = "by type " + join(ids)
		}
	}
	return out
}

// dispatchNameCap bounds the implementations named on one call line.
const dispatchNameCap = 4

// attachExpansion adds the requested next hop of one hit to its row and
// returns, per kind, how many neighbours there were before the cap.
func attachExpansion(reader store.Reader, row map[string]any, n types.Node, kinds []string) map[string]int {
	seed := seedRef{QName: n.QualifiedName, IDs: []string{n.ID}}
	totals := map[string]int{}
	brief := func(kind string, ns []types.Node, self string) []map[string]any {
		var out []map[string]any
		for _, x := range ns {
			if x.ID == self {
				continue
			}
			totals[kind]++
			if len(out) == searchCodeExpandCap {
				continue
			}
			out = append(out, map[string]any{"qname": x.QualifiedName, "file": x.FilePath, "line": x.StartLine})
		}
		return out
	}
	for _, k := range kinds {
		switch k {
		case "callers":
			if ns, _, err := reverseCallersUnion(reader, seed, 1); err == nil {
				row["callers"] = brief(k, ns, n.ID)
			}
		case "callees":
			if ns, es, err := seed.neighborhood(reader, 1, false, callEdgeTypes...); err == nil {
				ns, _, _ = withDispatchTargets(reader, ns, es)
				row["callees"] = brief(k, ns, n.ID)
			}
		case "references":
			if isValueNode(n.Type) {
				if out, err := valueUsers(reader, seed, "all", searchCodeExpandCap); err == nil {
					row["references"] = out["users"]
					if t, ok := out["total"].(int); ok {
						totals[k] = t
					}
				}
			}
		}
	}
	return totals
}

// keptBy is how many of n ranked hits a limit keeps (0 keeps all).
func keptBy(limit, n int) int {
	if limit <= 0 || limit > n {
		return n
	}
	return limit
}

// pageSearchCode returns part page of a search_code result. A result that
// fits in searchCodePageBytes is returned whole and unchanged. A larger one
// is split between hits, in rank order, and each part starts with a "page"
// header: which part it is, how many there are, which hits it holds, and the
// call that returns the next part. The header is written first so that it is
// the first thing the reader sees, before any hit.
func pageSearchCode(out map[string]any, page int, t *callTrace, tool string) any {
	rows, _ := out["hits"].([]map[string]any)
	rest := make(map[string]any, len(out))
	for k, v := range out {
		if k != "hits" {
			rest[k] = v
		}
	}
	base, _ := json.Marshal(rest)
	var parts [][2]int // [from, to) indexes into rows
	from, size := 0, len(base)
	for i, r := range rows {
		b, _ := json.Marshal(r)
		if i > from && size+len(b)+1 > searchCodePageBytes {
			parts = append(parts, [2]int{from, i})
			from, size = i, len(base)
		}
		size += len(b) + 1
	}
	parts = append(parts, [2]int{from, len(rows)})
	t.add("search_code.page", map[string]any{"parts": len(parts), "requested": page, "budget_bytes": searchCodePageBytes})
	if page < 1 {
		page = 1
	}
	if page > len(parts) {
		return map[string]any{"pattern": out["pattern"], "hits": []any{},
			"error": fmt.Sprintf("page %d does not exist: this result has %d part(s)", page, len(parts))}
	}
	if len(parts) == 1 {
		return out
	}
	p := parts[page-1]
	rest["hits"] = rows[p[0]:p[1]]
	header := map[string]any{"number": page, "of": len(parts),
		"declarations": fmt.Sprintf("%d-%d of %d", p[0]+1, p[1], len(rows))}
	if page < len(parts) {
		header["next"] = page + 1
		header["get_next"] = fmt.Sprintf("This is part %d of %d of one result, and the result is not complete until all %d parts are read. "+
			"Call %s again with the same arguments and page=%d for part %d.", page, len(parts), len(parts), ToolName(tool), page+1, page+1)
	} else {
		header["get_next"] = fmt.Sprintf("This is the last part (%d of %d). Parts 1 to %d together hold all %d declarations.", page, len(parts), len(parts), len(rows))
	}
	hb, _ := json.Marshal(header)
	rb, _ := json.Marshal(rest)
	raw := append([]byte(`{"page":`), hb...)
	raw = append(raw, ',')
	raw = append(raw, rb[1:]...)
	return json.RawMessage(raw)
}

// indexSearchCode answers a result larger than one response with an index
// instead of parts: in agent runs no agent asked for part 2 of a split result
// (0 of 20, with and without the parts rule in the server guide), so what
// was not in the first response never reached the agent. The index holds
// every matched declaration in rank order, each with its matched line
// numbers and the calls and value uses on them but no code text; the agent
// asks for the code of the ones it chooses with ids.
//
// When even the index does not fit, the rows at the end of the ranking lose
// their lines (name, file, span, matched texts stay), and if that is not
// enough the rest is counted per file, so no declaration disappears without
// a number. It returns ok=false when the full result fits.
func indexSearchCode(out map[string]any, t *callTrace, tool string) (any, bool) {
	full, _ := json.Marshal(out)
	if len(full) <= searchCodePageBytes {
		return nil, false
	}
	rows, _ := out["hits"].([]map[string]any)
	rest := map[string]any{}
	for k, v := range out {
		if k != "hits" {
			rest[k] = v
		}
	}
	// A qname names the row for ids; canonical_id is added only where two
	// rows share a qname.
	seenQ := map[any]int{}
	for _, r := range rows {
		seenQ[r["qname"]]++
	}
	withLines := make([]map[string]any, len(rows))
	nameOnly := make([]map[string]any, len(rows))
	for i, r := range rows {
		dup := seenQ[r["qname"]] > 1
		withLines[i], nameOnly[i] = indexRow(r, true, dup), indexRow(r, false, dup)
	}
	sizes := func(rs []map[string]any) []int {
		out := make([]int, len(rs))
		for i, r := range rs {
			b, _ := json.Marshal(r)
			out[i] = len(b) + 1
		}
		return out
	}
	sw, sn := sizes(withLines), sizes(nameOnly)
	// Rows 0..k-1 with lines, k..m-1 names only, m.. counted per file. Names
	// come first: as many rows as fit get a name, then rows from the top of
	// the ranking get their lines while the budget lasts. The estimate
	// leaves room for the header; the rendered response is then checked
	// against the budget and rows give up lines, then names, until it fits.
	base, _ := json.Marshal(rest)
	budget := searchCodePageBytes - 1200
	total, m := len(base), 0
	for m < len(rows) && total+sn[m] <= budget {
		total += sn[m]
		m++
	}
	k := 0
	for k < m && total-sn[k]+sw[k] <= budget {
		total += sw[k] - sn[k]
		k++
	}
	raw, hdr := renderIndex(rows, withLines, nameOnly, rest, k, m, tool)
	for len(raw) > searchCodePageBytes && m > 0 {
		if k > 0 {
			k--
		} else {
			m--
		}
		raw, hdr = renderIndex(rows, withLines, nameOnly, rest, k, m, tool)
	}
	t.add("search_code.index", map[string]any{"declarations": len(rows), "with_lines": hdr["with_lines"], "names_only": m - k,
		"not_listed": len(rows) - m, "budget_bytes": searchCodePageBytes, "full_bytes": len(full), "bytes": len(raw)})
	return json.RawMessage(raw), true
}

// renderIndex renders an index response: rows 0..k-1 with their lines,
// k..m-1 by name, the rest counted per file. with_lines in the header counts
// the rows that do carry matched lines (a row matched in its name only has
// none).
func renderIndex(rows, withLines, nameOnly []map[string]any, base map[string]any, k, m int, tool string) ([]byte, map[string]any) {
	rest := make(map[string]any, len(base)+2)
	for key, v := range base {
		rest[key] = v
	}
	index := make([]map[string]any, 0, m)
	index = append(index, withLines[:k]...)
	index = append(index, nameOnly[k:m]...)
	rest["hits"] = index
	carry := 0
	for _, r := range withLines[:k] {
		if r["at"] != nil {
			carry++
		}
	}
	header := map[string]any{"declarations": len(rows), "with_lines": carry}
	if m > k {
		header["names_only"] = m - k
	}
	text := fmt.Sprintf("This result is too large for one response, so it is an index: all %d matched declarations in rank order, "+
		"each with its matched line numbers and the calls and uses on them, without the code. "+
		"To see the code of the declarations you choose, call %s with the same pattern and ids=[their qname, ...].", len(rows), ToolName(tool))
	if m > k {
		text = fmt.Sprintf("This result is too large for one response, so it is an index of the %d matched declarations in rank order: "+
			"the first %d with their matched line numbers and the calls and uses on them, the next %d by name, file and lines only, without the code. "+
			"To see the code of the declarations you choose, call %s with the same pattern and ids=[their qname, ...].", len(rows), k, m-k, ToolName(tool))
	}
	if m < len(rows) {
		perFile := map[string]int{}
		for _, r := range rows[m:] {
			f, _ := r["file"].(string)
			perFile[f]++
		}
		rest["not_listed"] = perFile
		header["not_listed"] = len(rows) - m
		text += fmt.Sprintf(" %d more declarations do not fit even as names; not_listed counts them per file. Narrow the pattern or add path to see them.", len(rows)-m)
	}
	header["read_me"] = text
	hb, _ := json.Marshal(header)
	rb, _ := json.Marshal(rest)
	raw := append([]byte(`{"index":`), hb...)
	raw = append(raw, ',')
	raw = append(raw, rb[1:]...)
	return raw, header
}

// indexRow is one hit as an index row, kept short so more rows fit: qname,
// type, file, span ("start-end"), matched texts and count, the flags, and
// with lines, at (the matched line numbers) and uses (line -> what the graph
// resolved on it, only for lines that have any). canonical_id is kept when
// another row has the same qname, so ids can tell them apart.
func indexRow(r map[string]any, lines, withID bool) map[string]any {
	row := map[string]any{}
	for _, k := range []string{"qname", "type", "file", "matched", "matches", "test", "name_match"} {
		if v, ok := r[k]; ok {
			row[k] = v
		}
	}
	row["span"] = fmt.Sprintf("%v-%v", r["start_line"], r["end_line"])
	if withID {
		if v, ok := r["canonical_id"]; ok {
			row["canonical_id"] = v
		}
	}
	if !lines {
		return row
	}
	ls, _ := r["lines"].([]map[string]any)
	at := make([]any, 0, len(ls))
	uses := map[string]any{}
	for _, l := range ls {
		at = append(at, l["line"])
		if u, ok := l["uses"]; ok {
			uses[fmt.Sprint(l["line"])] = u
		}
	}
	if len(at) > 0 {
		row["at"] = at
	}
	if len(uses) > 0 {
		row["uses"] = uses
	}
	return row
}

// fileSearchCode saves a result larger than one response as a JSON file in
// searchCodeDir and answers with the file's path only: no hit is in the
// response, so the agent cannot take a part for the whole. In agent runs
// neither parts (0 of 20 asked for part 2) nor an index (0 of 6 asked for
// details) got the rest of a large result to the agent. The file is indented
// so that a file reader shows it line by line (one line of compact JSON is
// cut by readers that limit line length). It returns ok=false when the
// result fits, when no folder is set, or when the file cannot be written; the
// caller then answers with an index.
func fileSearchCode(out map[string]any, t *callTrace, tool string) (any, bool) {
	if searchCodeDir == "" {
		return nil, false
	}
	full, _ := json.Marshal(out)
	if len(full) <= searchCodePageBytes {
		return nil, false
	}
	body, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return nil, false
	}
	if err := os.MkdirAll(searchCodeDir, 0o755); err != nil {
		t.add("search_code.file", map[string]any{"error": err.Error()})
		return nil, false
	}
	sum := sha256.Sum256(full)
	path := filepath.Join(searchCodeDir, fmt.Sprintf("search_code-%x.json", sum[:6]))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.add("search_code.file", map[string]any{"error": err.Error()})
		return nil, false
	}
	rows, _ := out["hits"].([]map[string]any)
	lines := bytes.Count(body, []byte("\n")) + 1
	text := fmt.Sprintf("This result is too large for one response, so none of it is here: all %d matched declarations "+
		"(%d bytes, %d lines of indented JSON) are saved in the file %s. Read that file before you draw any conclusion from this search; "+
		"when the reader says the file is too large, read it in parts with offset and limit until you reach the end. "+
		"It has the keys of a full %s result, which the server guide explains. "+
		"For the code of a few declarations you can also call %s with the same pattern and ids=[their qname, ...].",
		len(rows), len(body), lines, path, ToolName(tool), ToolName(tool))
	t.add("search_code.file", map[string]any{"path": path, "declarations": len(rows), "bytes": len(body), "lines": lines,
		"budget_bytes": searchCodePageBytes})
	return map[string]any{"file": map[string]any{"path": path, "declarations": len(rows), "bytes": len(body),
		"lines": lines, "read_me": text}}, true
}

// rankBefore is the search_code order: a match in the name, then more
// matches, then a higher PageRank. Nothing after PageRank: hits equal on
// all of these are ties.
//
// Two keys were dropped by user decision (2026-11-19). Production before
// tests: search_code no longer returns tests (search_test_code does). More
// distinct matched strings: a declaration that names more of the words the
// model guessed is not the more important one; how often the words occur
// says more about how much of its work is about them.
func rankBefore(da, db codeDecl, a, b *codeHit) bool {
	if a.inName != b.inName {
		return a.inName
	}
	if a.matches != b.matches {
		return a.matches > b.matches
	}
	return da.node.PageRank > db.node.PageRank
}

// reasonEdges link a declaration to its graph neighbours for reasons:
// calls either way and value reads and writes either way.
var reasonEdges = map[types.EdgeType]bool{
	types.EdgeCalls: true, types.EdgeInvokes: true, types.EdgeReadsField: true,
	types.EdgeWritesField: true, types.EdgeReferences: true,
}

// reasonWords lowercases the reasons, drops words shorter than 3 characters
// and duplicates.
func reasonWords(reasons []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range reasons {
		w := strings.ToLower(strings.TrimSpace(r))
		if len(w) < 3 || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// wordsIn returns the reason words text holds, case-insensitively. A reason
// of several words ("round change") also matches the identifier forms
// roundChange and round_change, which plain substring matching missed: in the
// I3 agent run (runs/20261120-ks-i3opts) "round change" matched 0 rows.
func wordsIn(text string, words []string) []string {
	t := strings.ToLower(text)
	var got []string
	for _, w := range words {
		for _, form := range reasonForms(w) {
			if strings.Contains(t, form) {
				got = append(got, w)
				break
			}
		}
	}
	return got
}

// reasonForms is a lowercased reason as written, and for a reason of several
// words (split on spaces, hyphens and underscores) also joined with nothing
// and with underscores.
func reasonForms(w string) []string {
	parts := strings.FieldsFunc(w, func(r rune) bool { return r == ' ' || r == '-' || r == '_' })
	if len(parts) < 2 {
		return []string{w}
	}
	return []string{w, strings.Join(parts, " "), strings.Join(parts, ""), strings.Join(parts, "_")}
}

// reasonNeighbours lists the declarations next to the hits in the graph
// (production code, not hits themselves) whose source or qualified name holds
// a reason word. Each row says which words it holds and through which hit it
// was reached. Order: more distinct reason words in the declaration's source
// plus in the names of its own neighbours, then higher PageRank. The hits are
// not reordered: related comes after them.
func reasonNeighbours(reader store.Reader, idx *codeIndex, hitIDs []string, reasons []string) ([]map[string]any, []string) {
	words := reasonWords(reasons)
	out := []map[string]any{}
	if len(words) == 0 {
		return out, words
	}
	byID := idx.byNodeID()
	isHit := make(map[string]bool, len(hitIDs))
	for _, id := range hitIDs {
		isHit[id] = true
	}
	edges, err := reader.QueryEdgesForNodes(hitIDs)
	if err != nil {
		return out, words
	}
	type cand struct {
		decl int
		via  []map[string]any
	}
	cands := map[string]*cand{}
	var order []string
	addVia := func(other, hit string, t types.EdgeType, outward bool) {
		if isHit[other] {
			return
		}
		di, ok := byID[other]
		if !ok || idx.decls[di].test {
			return
		}
		c, ok := cands[other]
		if !ok {
			c = &cand{decl: di}
			cands[other] = c
			order = append(order, other)
		}
		dir := "from"
		if outward {
			dir = "to"
		}
		c.via = append(c.via, map[string]any{"hit": idx.decls[byID[hit]].node.QualifiedName, "edge": string(t), "dir": dir})
	}
	for _, e := range edges {
		if !reasonEdges[e.Type] || e.Src == e.Dst {
			continue
		}
		if isHit[e.Src] {
			addVia(e.Dst, e.Src, e.Type, true)
		}
		if isHit[e.Dst] {
			addVia(e.Src, e.Dst, e.Type, false)
		}
	}
	if len(order) == 0 {
		return out, words
	}
	// Names of each candidate's own neighbours, for the score.
	nb := map[string][]string{}
	if es, err := reader.QueryEdgesForNodes(order); err == nil {
		var ids []string
		for _, e := range es {
			if !reasonEdges[e.Type] {
				continue
			}
			if _, ok := cands[e.Src]; ok {
				nb[e.Src] = append(nb[e.Src], e.Dst)
				ids = append(ids, e.Dst)
			}
			if _, ok := cands[e.Dst]; ok {
				nb[e.Dst] = append(nb[e.Dst], e.Src)
				ids = append(ids, e.Src)
			}
		}
		name := map[string]string{}
		if ns, err := reader.NodesByIDs(ids); err == nil {
			for _, n := range ns {
				name[n.ID] = n.QualifiedName
			}
		}
		for id, list := range nb {
			names := make([]string, 0, len(list))
			for _, x := range list {
				names = append(names, name[x])
			}
			nb[id] = names
		}
	}
	type row struct {
		id    string
		own   []string
		score int
	}
	var rows []row
	for _, id := range order {
		d := idx.decls[cands[id].decl]
		own := wordsIn(d.node.QualifiedName+"\n"+strings.Join(d.lines, "\n"), words)
		if len(own) == 0 {
			continue
		}
		near := map[string]bool{}
		for _, w := range wordsIn(strings.Join(nb[id], "\n"), words) {
			near[w] = true
		}
		rows = append(rows, row{id: id, own: own, score: len(own) + len(near)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return idx.decls[cands[rows[i].id].decl].node.PageRank > idx.decls[cands[rows[j].id].decl].node.PageRank
	})
	for _, r := range rows {
		n := idx.decls[cands[r.id].decl].node
		m := map[string]any{"qname": n.QualifiedName, "type": string(n.Type), "file": n.FilePath,
			"start_line": n.StartLine, "end_line": n.EndLine, "reason_matched": r.own, "via": cands[r.id].via}
		if n.Signature != "" {
			m["signature"] = n.Signature
		}
		if n.CanonicalID != "" {
			m["canonical_id"] = n.CanonicalID
		}
		out = append(out, m)
	}
	return out, words
}

// byNodeID maps node id to the index of its declaration, built once.
func (x *codeIndex) byNodeID() map[string]int {
	x.idOnce.Do(func() {
		x.byID = make(map[string]int, len(x.decls))
		for i, d := range x.decls {
			x.byID[d.node.ID] = i
		}
	})
	return x.byID
}
