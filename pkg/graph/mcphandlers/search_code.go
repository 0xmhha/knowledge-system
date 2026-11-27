package mcphandlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// indexHeaderReserve is the room an index response leaves for its header
// in each copy of the answer.
const indexHeaderReserve = 1200

// searchCodePageBytes bounds the content of one search_code response: the
// bytes of every block it carries (contentBytes), so under ContentBoth the
// two copies together stay within it. search_code returns
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

// MinSearchCodePageBytes is the smallest search_code page size a server
// accepts: the smallest index (totals, the pattern, read_me and how to
// narrow, no rows and no list entries) fits it in every content mode for
// a pattern of up to maxFloorPatternBytes bytes (2,223 bytes measured
// under both with a 200-byte pattern). A result whose smallest index is
// larger (a longer pattern) is answered with tooLargeAnswer.
const MinSearchCodePageBytes = 3000

// maxFloorPatternBytes is the pattern length MinSearchCodePageBytes holds for.
const maxFloorPatternBytes = 200

// ValidateSearchCodePageBytes refuses a page size below
// MinSearchCodePageBytes, zero and negative sizes included; a caller that
// was given no size keeps the default and does not call it.
func ValidateSearchCodePageBytes(n int) error {
	if n < MinSearchCodePageBytes {
		return fmt.Errorf("--page-bytes %d is below the smallest search_code answer the server can send; the minimum is %d", n, MinSearchCodePageBytes)
	}
	return nil
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
// not its contract). A Solidity state variable is a Field node and is decided
// by searchable.
var searchCodeKinds = map[types.NodeType]bool{
	types.NodeFunction: true, types.NodeMethod: true, types.NodeStruct: true, types.NodeInterface: true,
	types.NodeTypeAlias: true, types.NodeConstant: true, types.NodeVariable: true, types.NodeContract: true,
	types.NodeModifier: true, types.NodeEvent: true, types.NodeMapping: true, types.NodeEnum: true,
	types.NodeError: true,
}

// langSolidity and subKindStructMember tell a Solidity state variable from a
// member of a struct: both are Field nodes.
const (
	langSolidity        = "sol"
	subKindStructMember = "struct_member"
)

// searchable reports whether n is a declaration the code search answers
// with: a kind of searchCodeKinds declared in a file of the build. enclosers
// are the functions, methods and variables of n's file (store.IsEncloser).
//
// Left out, because their text is searched as part of another declaration:
// a variable declared inside a function body or inside a function literal of
// a package-level variable's initialiser (store.InFunctionBody; a type or a
// constant declared there is a hit of its own, as it is at package level),
// a struct member (a Go field or a Solidity one; a Solidity state variable is
// a declaration of its contract and is kept), and the copy of a promoted
// method, which is the declared method under another type's name. A blank
// package-level declaration (`var _ I = (*T)(nil)`) is searched like any
// other: it has no name, but its text is source text (0 of 64 such lines of
// go-stablenet were found while blank variables were left out).
func searchable(n types.Node, enclosers []types.Node) bool {
	switch n.Type {
	case types.NodeVariable:
		if store.InFunctionBody(n, enclosers) {
			return false
		}
	case types.NodeField:
		return n.Language == langSolidity && n.SubKind != subKindStructMember
	}
	_, promoted := store.PromotedFrom(n)
	return searchCodeKinds[n.Type] && !promoted
}

type codeDecl struct {
	node types.Node
	// lines are the declaration's lines, each from where the declaration
	// has it to the end of the file's line (see addFile).
	lines []string
	// header is the declaration's own first line, without what follows the
	// declaration on that line and without comments (declHeader); in=name
	// matches it.
	header string
	test   bool
	// promotedTo are the types that get this method through an embedded
	// field, by qualified name, sorted.
	promotedTo []string
	// lateCalls maps a callee's node id to the lines, after the line its
	// call starts on, that name the callee (see addLateCalls).
	lateCalls map[string][]int
	// own maps a line (offset into lines) on which a declaration inside
	// this one starts or continues to the number of leading bytes that are
	// still this declaration's; a line not in the map is all its own (see
	// markOwnership).
	own map[int]int
	// twins are the declarations that share this one's lines and text (the
	// names of `var a, b = f()` when each starts at the keyword), this one
	// included, in the order of their names; nil when there is none.
	twins []int
	// mute marks a blank name of a multi-name declaration that has named
	// ones: its text is theirs, and nothing is reported under it.
	mute bool
	// full maps a line (offset into lines) to the fullest text stored for
	// that line of the file, when this declaration starts later on the line
	// (it is inside another one). The pattern is matched against it, so that
	// the anchors for line start and line end mean the line and not the
	// declaration's part of it.
	full map[int]string
	// guard maps a line to the sides (guardLead, guardTrail) on which other
	// stored text of the file stands on the same line without being part of
	// the text matched: two declarations on one line, neither inside the
	// other. A line anchor does not match on such a side.
	guard map[int]uint8
}

// The sides of a line's stored text beyond which the source line goes on.
const (
	guardLead uint8 = 1 << iota
	guardTrail
)

// lineFiller stands for source text that is not stored, next to a line's
// stored text: with it there, the pattern's line anchors do not match where
// the source line goes on.
const lineFiller = "\x00"

// matchLine returns the matches of rx on text, a line's stored text. On a
// guarded side the source line goes on: no line anchor matches there, and a
// match does not start outside the stored text.
func matchLine(rx *regexp.Regexp, text string, guard uint8) [][]int {
	if guard == 0 {
		return rx.FindAllStringIndex(text, -1)
	}
	padded, lead := text, 0
	if guard&guardLead != 0 {
		padded, lead = lineFiller+padded, len(lineFiller)
	}
	if guard&guardTrail != 0 {
		padded += lineFiller
	}
	var out [][]int
	for _, loc := range rx.FindAllStringIndex(padded, -1) {
		from, to := loc[0]-lead, min(loc[1]-lead, len(text))
		if from < 0 || from > len(text) || from == len(text) && guard&guardTrail != 0 {
			continue
		}
		out = append(out, []int{from, to})
	}
	return out
}

type codeIndex struct {
	decls  []codeDecl
	byFile map[string][]int // indexes into decls
	idOnce sync.Once
	byID   map[string]int // node id -> index into decls, see byNodeID
	// isTest is the graph's test file rule (store.TestFileRule).
	isTest func(file string) bool
}

// buildCodeIndex loads every searchable declaration and its source.
func buildCodeIndex(reader store.Reader) (*codeIndex, error) {
	isTest, err := testFileRule(reader)
	if err != nil {
		return nil, err
	}
	idx := &codeIndex{byFile: map[string][]int{}, isTest: isTest}
	seenFile := map[string]bool{}
	var longCalls []types.Node
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
			sites, err := idx.addFile(reader, f)
			if err != nil {
				return nil, err
			}
			longCalls = append(longCalls, sites...)
		}
	}
	// One lookup for the call sites of every file: edges are indexed by
	// their ends, not by file, and a lookup per file made the index build
	// ten times slower on go-stablenet (29 s instead of 3 s).
	if err := idx.addLateCalls(reader, longCalls); err != nil {
		return nil, err
	}
	return idx, nil
}

// addFile adds the searchable declarations of file f and returns the file's
// call sites that span several lines, for addLateCalls.
func (x *codeIndex) addFile(reader store.Reader, f string) ([]types.Node, error) {
	nodes, err := reader.NodesByFilePath(f)
	if err != nil {
		return nil, err
	}
	blobs, err := reader.BlobsByFilePath(f)
	if err != nil {
		return nil, err
	}
	first := len(x.decls)
	copies := map[store.DeclSite][]string{}
	var longCalls, enclosers []types.Node
	for _, n := range nodes {
		if store.IsEncloser(n) {
			enclosers = append(enclosers, n)
		}
	}
	for _, n := range nodes {
		if _, ok := store.PromotedFrom(n); ok {
			copies[store.SiteOf(n)] = append(copies[store.SiteOf(n)], store.EmbeddingType(n))
			continue
		}
		if n.Type == types.NodeCallSite && n.EndLine > n.StartLine {
			longCalls = append(longCalls, n)
			continue
		}
		src, ok := blobs[n.ID]
		if !searchable(n, enclosers) || !ok || len(src) == 0 {
			continue
		}
		x.byFile[f] = append(x.byFile[f], len(x.decls))
		x.decls = append(x.decls, codeDecl{node: n, lines: strings.Split(string(src), "\n"), test: x.isTest(f)})
	}
	added := x.decls[first:]
	// A nested declaration's source ends where the declaration ends, so the
	// rest of its last line (the comment after an interface method or an
	// enum value) is only in the enclosing declaration's source. Each line
	// of a declaration is extended to the end of the file's line: 16 lines
	// of go-stablenet matched only in that text and were reported under no
	// declaration.
	whole := map[int]lineText{}
	for _, d := range added {
		at := d.node.StartByte
		for k, text := range d.lines {
			if l := d.node.StartLine + k; len(text) > len(whole[l].text) {
				whole[l] = lineText{text: text, at: at}
			}
			at += len(text) + 1
		}
	}
	// apart collects, per line, the stored texts that are not part of the
	// line's fullest text: another declaration on the same line that is not
	// inside the first (`type A struct{}; type B struct{}`).
	apart := map[int][]lineText{}
	for i := range added {
		d := &added[i]
		d.header = declHeader(d.lines[0])
		at := d.node.StartByte
		for k, own := range d.lines {
			l := d.node.StartLine + k
			w := whole[l]
			d.lines[k] = toLineEnd(w, own, at)
			switch {
			case d.lines[k] == w.text:
			case strings.HasSuffix(w.text, d.lines[k]):
				if d.full == nil {
					d.full = map[int]string{}
				}
				d.full[k] = w.text
			default:
				apart[l] = append(apart[l], lineText{text: own, at: at})
			}
			at += len(own) + 1
		}
		if to := copies[store.SiteOf(d.node)]; d.node.Type == types.NodeMethod && len(to) > 0 {
			sort.Strings(to)
			d.promotedTo = to
		}
	}
	if len(apart) > 0 {
		guardLines(added, whole, apart)
	}
	x.markOwnership(x.byFile[f])
	return longCalls, nil
}

// guardLines marks, on the lines that hold several stored texts side by
// side, the sides of each text beyond which the line goes on (codeDecl.guard):
// the texts of a line are ordered by where they start, and every side that
// faces another text is guarded.
func guardLines(decls []codeDecl, whole map[int]lineText, apart map[int][]lineText) {
	for l, texts := range apart {
		texts = append(texts, whole[l])
		sort.SliceStable(texts, func(a, b int) bool { return texts[a].at < texts[b].at })
		side := map[string]uint8{}
		for pos, t := range texts {
			var g uint8
			if pos > 0 {
				g |= guardLead
			}
			if pos < len(texts)-1 {
				g |= guardTrail
			}
			side[t.text] |= g
		}
		for i := range decls {
			d := &decls[i]
			k := l - d.node.StartLine
			if k < 0 || k >= len(d.lines) {
				continue
			}
			text := d.lines[k]
			if w, ok := d.full[k]; ok {
				text = w
			}
			if g := side[text]; g != 0 {
				if d.guard == nil {
					d.guard = map[int]uint8{}
				}
				d.guard[k] = g
			}
		}
	}
}

// lineText is the longest text any declaration of a file has for one line,
// and the byte offset in the file at which that text starts.
type lineText struct {
	text string
	at   int
}

// toLineEnd returns own, a declaration's part of a line that starts at byte
// offset at of the file, extended to the end of that line: the text of whole
// from where own starts. The byte offsets say where that is; when they do not
// fit (a graph without them) the first occurrence of own in whole is taken.
// own is returned unchanged when it is empty or not found in whole.
func toLineEnd(whole lineText, own string, at int) string {
	if own == "" {
		return own
	}
	if col := at - whole.at; col >= 0 && col <= len(whole.text) && strings.HasPrefix(whole.text[col:], own) {
		return whole.text[col:]
	}
	if i := strings.Index(whole.text, own); i >= 0 {
		return whole.text[i:]
	}
	return own
}

// declHeader returns the part of a declaration's first stored line that
// in=name matches besides the qualified name: the line from the
// declaration's first token, with every comment on it left out and the
// blanks before them trimmed.
//
// While a declaration's stored text ended at its last token, this line ran
// to the end of the source line for a declaration that goes on below it, and
// to its last token for one that ends on it. Since the text runs through the
// comment after the last token, that comment was matched as a name: on
// go-stablenet `timeout` returned ethapi.Backend.RPCEVMTimeout and `t`
// returned rawdb.opMax from the words of their comments. A name search
// matches names and code, so no comment is part of the header, the one on
// the first line of a longer declaration (`type T struct { // ...`)
// included. The body search still finds every comment.
func declHeader(line string) string {
	var b strings.Builder
	quote := byte(0) // the delimiter of the literal being read, 0 outside one
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			b.WriteByte(c)
			if c == '\\' && quote != '`' && i+1 < len(line) {
				i++
				b.WriteByte(line[i])
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
			b.WriteByte(c)
		case strings.HasPrefix(line[i:], lineCommentOpen):
			return strings.TrimRight(b.String(), headerBlanks)
		case strings.HasPrefix(line[i:], blockCommentOpen):
			end := strings.Index(line[i+len(blockCommentOpen):], blockCommentClose)
			if end < 0 {
				return strings.TrimRight(b.String(), headerBlanks)
			}
			i += len(blockCommentOpen) + end + len(blockCommentClose) - 1
		default:
			b.WriteByte(c)
		}
	}
	return strings.TrimRight(b.String(), headerBlanks)
}

// The comment delimiters of Go and Solidity, and the blanks trimmed where a
// comment is left out of a header (declHeader).
const (
	lineCommentOpen   = "//"
	blockCommentOpen  = "/*"
	blockCommentClose = "*/"
	headerBlanks      = " \t"
)

// markOwnership records, for the declarations of one file (indexes into
// decls, ordered by start line), which part of each line every declaration
// owns, so that a match is reported under one declaration: the one that owns
// its first character.
//
// The rule: a declaration inside another one owns its lines from where its
// text starts to the end of the line, and the indentation in front of it;
// other text in front of it on its first line stays with the enclosing
// declaration. So in
//
//	func Slot(key interface{ Bytes() []byte }) common.Hash {
//
// "func Slot(key" is found under the function and "Bytes()" under the
// interface method, and in `var a, b = f()` each name is found under its own
// declaration. Before, the innermost declaration took the whole line and the
// text in front of it was unsearchable (4 of 4 signature lines with an inline
// interface on go-stablenet; 8 of 69 names of multi-name declarations).
//
// Declarations with the same lines and the same text (twins) share a line;
// see twinOwner.
func (x *codeIndex) markOwnership(file []int) {
	for pos, i := range file {
		outer := &x.decls[i]
		// Declarations that start within outer's lines sit next to it in
		// the list: walk back over those starting on its first line, then
		// forward to the last one starting before its end.
		lo := pos
		for lo > 0 && x.decls[file[lo-1]].node.StartLine == outer.node.StartLine {
			lo--
		}
		for _, j := range file[lo:] {
			inner := &x.decls[j]
			if inner.node.StartLine > outer.node.EndLine {
				break
			}
			if j == i {
				continue
			}
			switch nesting(outer, inner) {
			case nestedTwin:
				if outer.twins == nil {
					outer.twins = []int{i}
				}
				outer.twins = append(outer.twins, j)
			case nestedInside:
				outer.yield(inner)
			}
		}
		// Twins in the order their names stand in the declaration, so
		// that "the first" is the first name written.
		sort.Slice(outer.twins, func(a, b int) bool {
			pa, pb := x.namePos(outer.twins[a]), x.namePos(outer.twins[b])
			if pa != pb {
				return pa < pb
			}
			return outer.twins[a] < outer.twins[b]
		})
		// A blank among the names (`x, _ = f()`) is no name: the line is
		// reported under the named ones and they do not list it.
		var named []int
		for _, t := range outer.twins {
			if x.decls[t].node.Name != store.BlankName {
				named = append(named, t)
			}
		}
		if len(named) > 0 && len(named) < len(outer.twins) {
			outer.mute = outer.node.Name == store.BlankName
			outer.twins = named
			if len(named) == 1 && !outer.mute {
				outer.twins = nil
			}
		}
	}
}

// namePos returns where declaration i's name first stands, as a whole
// identifier, in its first line, or the line's length when it does not.
func (x *codeIndex) namePos(i int) int {
	line, name := x.decls[i].lines[0], x.decls[i].node.Name
	for p := 0; name != "" && p < len(line); {
		at := strings.Index(line[p:], name)
		if at < 0 {
			break
		}
		at += p
		if end := at + len(name); (at == 0 || !isIdentByte(line[at-1])) && (end == len(line) || !isIdentByte(line[end])) {
			return at
		}
		p = at + 1
	}
	return len(line)
}

// The ways a declaration that starts within another one's lines relates to it.
const (
	nestedNot    = iota // not inside it (it encloses it, or only overlaps)
	nestedInside        // inside it: it takes its lines from where it starts
	nestedTwin          // same lines and same text: another name of one declaration
)

// nesting says how inner, which starts within outer's lines, relates to
// outer. With the same lines, the one whose first line starts later in the
// line is inside the other.
func nesting(outer, inner *codeDecl) int {
	o, n := outer.node, inner.node
	if n.StartLine < o.StartLine || n.EndLine > o.EndLine {
		return nestedNot
	}
	if n.StartLine != o.StartLine || n.EndLine != o.EndLine {
		return nestedInside
	}
	switch {
	case inner.lines[0] == outer.lines[0]:
		return nestedTwin
	case len(inner.lines[0]) < len(outer.lines[0]) && strings.HasSuffix(outer.lines[0], inner.lines[0]):
		return nestedInside
	}
	return nestedNot
}

// yield gives the lines of inner, a declaration inside d, to inner: all of
// each line but, on inner's first line, what stands in front of inner.
func (d *codeDecl) yield(inner *codeDecl) {
	if d.own == nil {
		d.own = map[int]int{}
	}
	for l := inner.node.StartLine; l <= inner.node.EndLine; l++ {
		k := l - d.node.StartLine
		if k < 0 || k >= len(d.lines) {
			continue
		}
		keep := 0
		if l == inner.node.StartLine && strings.HasSuffix(d.lines[k], inner.lines[0]) {
			keep = len(d.lines[k]) - len(inner.lines[0])
			// Indentation is not text in front of a declaration.
			if strings.TrimSpace(d.lines[k][:keep]) == "" {
				keep = 0
			}
		}
		if old, ok := d.own[k]; !ok || keep < old {
			d.own[k] = keep
		}
	}
}

// lineText returns line k (offset into lines) of the declaration as a
// result shows it: its own part of the line together with the indentation
// in front of it, which belongs to a nested declaration as much as the
// matches in it do (markOwnership). Other text in front of it is the
// enclosing declaration's and is left out.
func (d *codeDecl) lineText(k int) string {
	own := d.lines[k]
	if w, ok := d.full[k]; ok && strings.TrimSpace(w[:len(w)-len(own)]) == "" {
		return w
	}
	return own
}

// owns reports whether the match at bytes [from, to) of line k of declaration
// i is reported under that declaration: it starts in the part of the line the
// declaration owns and, among twins, it is this twin's.
func (x *codeIndex) owns(i, k, from, to int) bool {
	d := &x.decls[i]
	if keep, limited := d.own[k]; d.mute || limited && from >= keep {
		return false
	}
	return d.twins == nil || x.twinOwner(d, k, from, to) == i
}

// declaredWith returns the qualified names of the other declarations that
// share declaration i's lines and text: the other names of a multi-name
// declaration.
func (x *codeIndex) declaredWith(i int) []string {
	var out []string
	for _, t := range x.decls[i].twins {
		if t != i {
			out = append(out, x.decls[t].node.QualifiedName)
		}
	}
	return out
}

// twinOwner picks, among twins that share a line, the one a match on it is
// reported under: on the first line, the twin whose name the match touches,
// so that each name of `var a, b = f()` is found under its own declaration;
// otherwise the first twin, so that the line is reported once. Every match
// is counted once; each hit names the others in declared_with.
func (x *codeIndex) twinOwner(d *codeDecl, k, from, to int) int {
	if k > 0 {
		return d.twins[0]
	}
	// twins are in the order of their names, so the first that the match
	// touches is the earliest.
	for _, t := range d.twins {
		at := x.namePos(t)
		end := at + len(x.decls[t].node.Name)
		if at < len(d.lines[0]) && (from < end && to > at || from == to && from >= at && from < end) {
			return t
		}
	}
	return d.twins[0]
}

// addLateCalls records, for the call sites that span several lines, the
// later lines that name the callee. A call edge carries the line its call
// starts on, so a call written as
//
//	abi.Arguments{
//		...
//	}.Pack(a, b)
//
// was listed on the first line only and a search for "\.Pack\(" matched a
// line without it (8 calls on go-stablenet).
func (x *codeIndex) addLateCalls(reader store.Reader, sites []types.Node) error {
	if len(sites) == 0 {
		return nil
	}
	long := make([]string, len(sites))
	for i, s := range sites {
		long[i] = s.ID
	}
	edges, err := reader.QueryEdgesForNodes(long)
	if err != nil {
		return err
	}
	callees := map[string][]string{} // call site id -> callee ids
	var ids []string
	for _, e := range edges {
		if e.Type == types.EdgeSiteCalls {
			callees[e.Src] = append(callees[e.Src], e.Dst)
			ids = append(ids, e.Dst)
		}
	}
	targets, err := reader.NodesByIDs(ids)
	if err != nil {
		return err
	}
	name := make(map[string]string, len(targets))
	for _, n := range targets {
		name[n.ID] = n.Name
	}
	for _, s := range sites {
		di := x.owner(s.FilePath, s.StartLine)
		if di < 0 {
			continue
		}
		d := &x.decls[di]
		for _, callee := range callees[s.ID] {
			for l := s.StartLine + 1; l <= s.EndLine; l++ {
				if off := l - d.node.StartLine; off < len(d.lines) && namesCall(d.lines[off], name[callee]) {
					if d.lateCalls == nil {
						d.lateCalls = map[string][]int{}
					}
					d.lateCalls[callee] = append(d.lateCalls[callee], l)
				}
			}
		}
	}
	return nil
}

// namesCall reports whether line holds a call written with name: the name as
// a whole identifier followed by "(".
func namesCall(line, name string) bool {
	if name == "" {
		return false
	}
	for from := 0; ; {
		i := strings.Index(line[from:], name+"(")
		if i < 0 {
			return false
		}
		i += from
		if i == 0 || !isIdentByte(line[i-1]) {
			return true
		}
		from = i + 1
	}
}

// lateCallLines returns the lines after the first on which declaration src
// names callee dst in a call that spans several lines.
func (x *codeIndex) lateCallLines(src, dst string) []int {
	if i, ok := x.byNodeID()[src]; ok {
		return x.decls[i].lateCalls[dst]
	}
	return nil
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
		"grep over the test code of the graph (the files the build records as test code): a regular expression (RE2; alternatives with |) over the source of every test function, helper, type, constant and variable, "+
			"answered per declaration: its qname, file, lines, the matched lines with line numbers, and the distinct texts it matched (lowercased). "+
			"Order: a match in the name first, then more matches, then wider use in the repository (PageRank). "+
			"Arguments, result keys and the parts of a large result are those of search_code (its description and parameters explain them). ")
}

// readingFor is the part of a code search tool's description that explains
// its result: in full for search_code, which search_test_code points to.
func readingFor(name string) string {
	if name != "search_code" {
		return ""
	}
	return guideText(searchCodeReading)
}

// ruleFor is the full rule for search_code and a pointer to it for the
// other code search, which takes the same arguments.
func ruleFor(name, rule, short string) string {
	if name != "search_code" {
		return short
	}
	return guideText(rule)
}

// nameAsBothNote is said in the in parameter when in=name returns body
// matches too.
func nameAsBothNote() string {
	if searchCodeNameAsBoth {
		return searchCodeNameAsBothManual
	}
	return ""
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
			"A result too large for one response comes as an index: every matched declaration in one short row with its matched line numbers and the uses on them, without code; ask for the code of the ones you choose with ids=[qname, ...] (see ids). " +
			"Each matched line lists the calls and value uses the graph resolved on it (resolve), and expand=[\"callers\",\"callees\",\"references\"] attaches the next hop. " +
			"Use it as you would grep: enumerate the names and code shapes you expect (\"Flush|flush|Writer\", \"sql\\\\.Open\\\\(\", \"func \\\\(.*\\\\) \\\\w*Render\\\\w*\\\\(\"). " +
			"Covers the build's files as of the last graph build; packages outside the build and uncommitted edits are not searched." + readingFor(name)),
		mcp.WithString("pattern", mcp.Required(), mcp.Description(ruleFor(name, searchCodePatternRule, "RE2 regular expression; | separates alternatives. With fixed=true, a literal string. What is searched is as in search_code (its pattern parameter)."))),
		mcp.WithBoolean("fixed", mcp.Description("treat pattern as a literal string")),
		mcp.WithBoolean("ignore_case", mcp.Description("case-insensitive match")),
		mcp.WithBoolean("word", mcp.Description("match whole words only")),
		mcp.WithString("in", mcp.Description("body (default: declaration source, signature included), name (the qualified name, or the declaration's first line from its first token with its comments left out, e.g. \"func (t *Template) Render(\"; never comment text), or both."+nameAsBothNote())),
		mcp.WithString("path", mcp.Description("only files under this path prefix, or matching this glob (* and **)")),
		mcp.WithArray("expand", mcp.Description("attach for each hit: callers, callees, references (who reads or writes a value); at most 10 of each, callers and callees the first 10 by qualified name, then file and line, references in the order find_references gives; when there are more, the hit has callers_total, callees_total or references_total. The counted set is exactly what find_callers or find_callees with qname=<the hit's canonical_id, or its qname when it has none> and depth=1 returns besides the hit itself, and what find_references with that qname and limit=<references_total> returns"), mcp.WithStringItems()),
		mcp.WithBoolean("resolve", mcp.Description("list the calls and value uses on each matched line (default true)")),
		mcp.WithNumber("context", mcp.Description("lines of context around each matched line (0 to 3, default 0)")),
		mcp.WithArray("ids", mcp.Description(ruleFor(name, searchCodeIdsRule, "qname (or canonical_id) of declarations to return in full, after a result that came as an index (as in search_code: its ids parameter explains the index)")), mcp.WithStringItems()),
		mcp.WithNumber("page", mcp.Description(ruleFor(name, searchCodePageRule, "part to return when a result asked for with ids is still too large (default 1; as in search_code)"))),
	}
	if searchCodeReasons {
		params = append(params, mcp.WithArray("reasons", mcp.Description(guideText(searchCodeReasonsManual)), mcp.WithStringItems()))
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
			// Echoes are cut before the answer is built, so the size the
			// answer is measured at is the size it is sent at.
			return textResult(cutAnswerEchoes(ctx, map[string]any{"pattern": req.GetString("pattern", ""), "error": err.Error(), "hits": []any{}})), nil
		}
		out = cutAnswerEchoes(ctx, out)
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
		// A blank declaration (`var _ I = (*T)(nil)`) has text to search
		// but no name to match.
		if (a.in == "name" || a.in == "both") && n.Name != store.BlankName {
			// The name is the qualified name or the declaration's first
			// line, so "func .*Round\w*\(" works as it does in grep.
			m := rx.FindString(n.QualifiedName)
			if m == "" {
				m = rx.FindString(d.header)
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
		for k, own := range d.lines {
			// The pattern runs over the fullest text stored for the line,
			// so that its line anchors mean the line; in is where this
			// declaration's part starts in it.
			line, in := own, 0
			if w, ok := d.full[k]; ok {
				line, in = w, len(w)-len(own)
			}
			locs := matchLine(rx, line, d.guard[k])
			if len(locs) == 0 {
				continue
			}
			// A match belongs to the declaration that owns its first
			// character (markOwnership): text inside a nested declaration
			// is that one's, with the indentation in front of it; other
			// text in front of it is the enclosing declaration's.
			var ms []string
			for _, loc := range locs {
				from, to := loc[0]-in, max(loc[1]-in, 0)
				if from < 0 {
					if strings.TrimSpace(line[:in]) != "" {
						continue
					}
					from = 0
				}
				if idx.owns(i, k, from, to) {
					ms = append(ms, line[loc[0]:loc[1]])
				}
			}
			if len(ms) == 0 {
				nestedLines++
				continue
			}
			h := hit(i)
			h.lines = append(h.lines, n.StartLine+k)
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
			// A promoted method may be asked for under an embedding type's
			// name; it is this declaration.
			keys := []string{n.CanonicalID, n.QualifiedName, store.PrintedName(reader, n)}
			for _, to := range idx.decls[h.decl].promotedTo {
				keys = append(keys, to+"."+n.Name)
			}
			for _, k := range keys {
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
		lineUses, lineExt = usesByLineExt(reader, ids, idx.lateCallLines)
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
		row := map[string]any{"qname": store.PrintedName(reader, n), "type": string(n.Type), "file": n.FilePath,
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
		if len(d.promotedTo) > 0 {
			row["promoted_to"] = d.promotedTo
		}
		// The other names of a multi-name declaration share this one's
		// lines; a shared match is reported under one of them only.
		if with := idx.declaredWith(h.decl); len(with) > 0 {
			row["declared_with"] = with
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
					// The line as it is in the file, trailing blanks included.
					text = append(text, d.lineText(off))
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
					rows[i][kind+expandTotalSuffix] = total
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
	out, _ := usesByLineExt(reader, ids, nil)
	return out
}

// The verbs of the uses a Solidity function has besides calls and value uses.
const (
	useEmits    = "emits "
	useReverts  = "reverts with "
	useModifier = "applies modifier "
	useCalls    = "calls "
)

// usesByLineExt is usesByLine plus, per node and line, the packages of the
// External targets (schema 1.36) named on that line, so a response can list
// each package it mentions once. An External target is named by its package
// name ("time.AfterFunc"); when two packages of the targets share a name
// (math/rand, crypto/rand) both are named by import path instead.
//
// late, when not nil, returns the lines after a call's first line on which
// the caller names the callee (codeIndex.lateCallLines); the call is listed
// on those lines too.
func usesByLineExt(reader store.Reader, ids []string, late func(src, dst string) []int) (map[string]map[int][]string, map[string]map[int][]types.ExternalPackage) {
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
	// list records use u of edge e on a line, once per line.
	list := func(e types.Edge, line int, u string) {
		k := fmt.Sprint(e.Src, line, u)
		if seen[k] {
			return
		}
		seen[k] = true
		if out[e.Src] == nil {
			out[e.Src] = map[int][]string{}
		}
		out[e.Src][line] = append(out[e.Src][line], u)
		if ep, ok := ext[e.Dst]; ok {
			if outExt[e.Src] == nil {
				outExt[e.Src] = map[int][]types.ExternalPackage{}
			}
			outExt[e.Src][line] = append(outExt[e.Src][line], ep)
		}
	}
	// listCall is list for a call: also on the later lines that name the
	// callee when the call spans several.
	listCall := func(e types.Edge, u string) {
		list(e, e.Line, u)
		if late == nil {
			return
		}
		for _, line := range late(e.Src, e.Dst) {
			list(e, line, u)
		}
	}
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
		switch e.Type {
		case types.EdgeDispatchesTo:
			// shown with the call it belongs to
		case types.EdgeInvokes:
			if r := runs[e.Dst]; r != "" && e.DispatchKind == "interface_method" {
				listCall(e, useCalls+name[e.Dst]+" (may run "+r+")")
				continue
			}
			listCall(e, useCalls+name[e.Dst])
		case types.EdgeCalls:
			listCall(e, useCalls+name[e.Dst])
		case types.EdgeEmitsEvent:
			list(e, e.Line, useEmits+name[e.Dst])
		case types.EdgeRevertsWith:
			list(e, e.Line, useReverts+name[e.Dst])
		case types.EdgeHasModifier:
			list(e, e.Line, useModifier+name[e.Dst])
		default:
			// "reads X" / "writes X" for the value edges, Solidity
			// mappings included; other edges are not uses on a line.
			if access := valueAccess(e); access != "" {
				list(e, e.Line, access+"s "+name[e.Dst])
			}
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
	// The first searchCodeExpandCap neighbours by qualified name, then file
	// and line, are listed; totals counts them all (expansionOrder).
	brief := func(kind string, ns []types.Node, self string) []map[string]any {
		var out []map[string]any
		ns = append([]types.Node(nil), ns...)
		sort.SliceStable(ns, func(i, j int) bool { return expansionOrder(ns[i], ns[j]) })
		for _, x := range ns {
			if x.ID == self {
				continue
			}
			totals[kind]++
			if len(out) == searchCodeExpandCap {
				continue
			}
			out = append(out, map[string]any{"qname": store.PrintedName(reader, x), "file": x.FilePath, "line": x.StartLine})
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
			if ns, _, err := forwardCallees(reader, seed, 1); err == nil {
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

// expandTotalSuffix ends the key that gives, on a hit whose expansion was
// cut, how many there are ("callers_total").
const expandTotalSuffix = "_total"

// expansionOrder is the order of an expansion list (callers, callees):
// qualified name, then file, then line, then id.
func expansionOrder(a, b types.Node) bool {
	switch {
	case a.QualifiedName != b.QualifiedName:
		return a.QualifiedName < b.QualifiedName
	case a.FilePath != b.FilePath:
		return a.FilePath < b.FilePath
	case a.StartLine != b.StartLine:
		return a.StartLine < b.StartLine
	}
	return a.ID < b.ID
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
	parts := splitSearchCode(out, rows, rest, tool)
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
	return json.RawMessage(renderPart(rows, rest, parts, page, tool))
}

// pageFits is the one test of whether a response fits one search_code
// response: its content blocks as encoded on the wire (wireBytes) within
// searchCodePageBytes.
// The whole result, every part of a split one and every index are held to
// it.
func pageFits(raw []byte) bool { return wireBytes(raw) <= searchCodePageBytes }

// splitSearchCode returns the parts ([from, to) indexes into rows) a result
// is sent in: one when it fits whole, otherwise as many as it takes for
// every rendered part, page header included, to fit (pageFits). A part of a
// single hit is sent even when that hit alone is too large.
func splitSearchCode(out map[string]any, rows []map[string]any, rest map[string]any, tool string) [][2]int {
	if full, _ := json.Marshal(out); pageFits(full) || len(rows) <= 1 {
		return [][2]int{{0, len(rows)}}
	}
	sizes := make([]int, len(rows))
	for i, r := range rows {
		b, _ := json.Marshal(r)
		sizes[i] = rowBytes(b)
	}
	base, _ := json.Marshal(rest)
	// The header's size grows with its numbers; no number exceeds the row
	// count, so a header written with that count bounds every real one.
	reserve := contentBytes(partHeaderRaw(len(rows), len(rows), len(rows), len(rows), len(rows), tool))
	budget := searchCodePageBytes - reserve
	var parts [][2]int
	from, size := 0, contentBytes(base)
	for i := range rows {
		if i > from && size+sizes[i] > budget {
			parts = append(parts, [2]int{from, i})
			from, size = i, contentBytes(base)
		}
		size += sizes[i]
	}
	parts = append(parts, [2]int{from, len(rows)})
	// Check every rendered part with the one test; split any that does not
	// fit (the estimate leaves out key order and separators).
	for {
		split := false
		for i := 0; i < len(parts); i++ {
			p := parts[i]
			if p[1]-p[0] > 1 && !pageFits(renderPart(rows, rest, parts, i+1, tool)) {
				mid := (p[0] + p[1]) / 2
				parts = append(parts[:i], append([][2]int{{p[0], mid}, {mid, p[1]}}, parts[i+1:]...)...)
				split = true
				break
			}
		}
		if !split {
			return parts
		}
	}
}

// renderPart renders part page (1-based) of parts: the page header first,
// then the result's other keys with the part's hits.
func renderPart(rows []map[string]any, rest map[string]any, parts [][2]int, page int, tool string) []byte {
	p := parts[page-1]
	body := make(map[string]any, len(rest)+1)
	for k, v := range rest {
		body[k] = v
	}
	body["hits"] = rows[p[0]:p[1]]
	rb, _ := json.Marshal(body)
	raw := partHeaderRaw(page, len(parts), p[0]+1, p[1], len(rows), tool)
	raw = append(raw, ',')
	return append(raw, rb[1:]...)
}

// partHeaderRaw renders `{"page":<header>` for part page of n holding
// declarations from..to of total.
func partHeaderRaw(page, n, from, to, total int, tool string) []byte {
	header := map[string]any{"number": page, "of": n,
		"declarations": fmt.Sprintf("%d-%d of %d", from, to, total)}
	if page < n {
		header["next"] = page + 1
		header["get_next"] = fmt.Sprintf("This is part %d of %d of one result, and the result is not complete until all %d parts are read. "+
			"Call %s again with the same arguments and page=%d for part %d.", page, n, n, ToolName(tool), page+1, page+1)
	} else {
		header["get_next"] = fmt.Sprintf("This is the last part (%d of %d). Parts 1 to %d together hold all %d declarations.", page, n, n, total)
	}
	hb, _ := json.Marshal(header)
	return append([]byte(`{"page":`), hb...)
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
	if pageFits(full) {
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
		// A row printed in the site form has a qualified name other
		// declarations carry: it keeps its canonical_id, as it did when
		// such rows shared the plain qname.
		q, _ := r["qname"].(string)
		_, _, _, site := store.ParseSiteName(q)
		dup := seenQ[r["qname"]] > 1 || site
		withLines[i], nameOnly[i] = indexRow(r, true, dup), indexRow(r, false, dup)
	}
	sizes := func(rs []map[string]any) []int {
		out := make([]int, len(rs))
		for i, r := range rs {
			b, _ := json.Marshal(r)
			out[i] = rowBytes(b)
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
	budget := searchCodePageBytes - indexHeaderReserve*contentCopies()
	total, m := contentBytes(base), 0
	for m < len(rows) && total+sn[m] <= budget {
		total += sn[m]
		m++
	}
	k := 0
	for k < m && total-sn[k]+sw[k] <= budget {
		total += sw[k] - sn[k]
		k++
	}
	// Every list besides the rows (the per-file counts of the rows not
	// listed, packages, related, ids_not_matched) is cut by one bound,
	// keep, entries of each (boundList). The lists may first take at most
	// half the response, so that rows keep room; then rows give up lines,
	// then names; then keep shrinks until the response fits (pageFits).
	largestKeep := func(limit int) int {
		lo, hi := 0, len(rows)+maxListLen(rest)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if r, _ := renderIndex(rows, withLines, nameOnly, rest, 0, 0, mid, tool); wireBytes(r) <= limit {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		return lo
	}
	keep := largestKeep(searchCodePageBytes / listShareDivisor)
	raw, hdr := renderIndex(rows, withLines, nameOnly, rest, k, m, keep, tool)
	for !pageFits(raw) && m > 0 {
		if k > 0 {
			k--
		} else {
			m--
		}
		raw, hdr = renderIndex(rows, withLines, nameOnly, rest, k, m, keep, tool)
	}
	if m == 0 {
		keep = largestKeep(searchCodePageBytes)
		raw, hdr = renderIndex(rows, withLines, nameOnly, rest, 0, 0, keep, tool)
	}
	if !pageFits(raw) {
		t.add("search_code.index", map[string]any{"declarations": len(rows), "too_large": wireBytes(raw), "budget_bytes": searchCodePageBytes})
		return tooLargeAnswer(len(rows), wireBytes(raw)), true
	}
	t.add("search_code.index", map[string]any{"declarations": len(rows), "with_lines": hdr["with_lines"], "names_only": m - k,
		"not_listed": len(rows) - m, "budget_bytes": searchCodePageBytes, "full_bytes": len(full), "bytes": len(raw)})
	return json.RawMessage(raw), true
}

// tooLargeAnswer is the answer when even the smallest index does not fit
// the page size (a pattern longer than maxFloorPatternBytes): the counts
// and how to narrow, without the pattern.
func tooLargeAnswer(declarations, smallest int) map[string]any {
	return map[string]any{"hits": []any{}, "total_declarations": declarations,
		"error": fmt.Sprintf("this result of %d declarations is too large for one response even as an index without rows (%d bytes, page size %d); "+
			"narrow it with path or a shorter, more specific pattern", declarations, smallest, searchCodePageBytes)}
}

// listShareDivisor sets how much of a response the lists besides the rows
// may take before rows are cut: 1/listShareDivisor of it.
const listShareDivisor = 2

// boundedListKeys are the lists of a search_code result besides hits that
// an index cuts to keep entries (boundList).
var boundedListKeys = []string{"packages", "related", "ids_not_matched"}

// leftOutSuffix names the count of the entries a cut list left out.
const leftOutSuffix = "_left_out"

// listLen returns the length of a list value of a result, -1 for a value
// that is not a list.
func listLen(v any) int {
	switch l := v.(type) {
	case []map[string]any:
		return len(l)
	case []string:
		return len(l)
	case []any:
		return len(l)
	}
	return -1
}

// maxListLen is the longest bounded list of rest.
func maxListLen(rest map[string]any) int {
	n := 0
	for _, k := range boundedListKeys {
		n = max(n, listLen(rest[k]))
	}
	return n
}

// boundList cuts the list under key in rest to its first keep entries, in
// the order the result lists them, and records how many it left out under
// key+leftOutSuffix. It is the one rule every list of an index is cut by.
func boundList(rest map[string]any, key string, keep int) {
	n := listLen(rest[key])
	if n <= keep {
		return
	}
	switch l := rest[key].(type) {
	case []map[string]any:
		rest[key] = l[:keep]
	case []string:
		rest[key] = l[:keep]
	case []any:
		rest[key] = l[:keep]
	}
	rest[key+leftOutSuffix] = n - keep
}

// renderIndex renders an index response: rows 0..k-1 with their lines,
// k..m-1 by name, the rest counted per file for at most keep files (those
// with the most rows first), and every other list cut to keep entries
// (boundList); what is left out is counted. with_lines in the header is k,
// the rows given in full, so that with_lines and names_only add up to the
// rows listed. A row matched in its name only is one of them and has no
// lines to carry: counting only rows with lines left such rows in neither
// number (137 rows against with_lines 131 in one go-stablenet response).
func renderIndex(rows, withLines, nameOnly []map[string]any, base map[string]any, k, m, keep int, tool string) ([]byte, map[string]any) {
	rest := make(map[string]any, len(base)+6)
	for key, v := range base {
		rest[key] = v
	}
	var cut []string
	for _, key := range boundedListKeys {
		total := listLen(rest[key])
		boundList(rest, key, keep)
		if total > keep {
			cut = append(cut, fmt.Sprintf("%s lists %d of %d (%s%s counts the rest)", key, keep, total, key, leftOutSuffix))
		}
	}
	index := make([]map[string]any, 0, m)
	index = append(index, withLines[:k]...)
	index = append(index, nameOnly[k:m]...)
	rest["hits"] = index
	header := map[string]any{"declarations": len(rows), "with_lines": k}
	if m > k {
		header["names_only"] = m - k
	}
	shown, all := 0, 0
	if m < len(rows) {
		perFile := map[string]int{}
		for _, r := range rows[m:] {
			f, _ := r["file"].(string)
			perFile[f]++
		}
		files := slices.Collect(maps.Keys(perFile))
		sort.Slice(files, func(i, j int) bool {
			if perFile[files[i]] != perFile[files[j]] {
				return perFile[files[i]] > perFile[files[j]]
			}
			return files[i] < files[j]
		})
		all, shown = len(files), min(len(files), keep)
		if shown > 0 {
			listed := make(map[string]int, shown)
			for _, f := range files[:shown] {
				listed[f] = perFile[f]
			}
			rest["not_listed"] = listed
		}
		header["not_listed"] = len(rows) - m
		if shown < all {
			left := 0
			for _, f := range files[shown:] {
				left += perFile[f]
			}
			rest["not_listed_files_left_out"] = map[string]int{"files": all - shown, "declarations": left}
		}
	}
	header["read_me"] = indexReadMe(len(rows), k, m, shown, all, cut, tool)
	hb, _ := json.Marshal(header)
	rb, _ := json.Marshal(rest)
	raw := append([]byte(`{"index":`), hb...)
	raw = append(raw, ',')
	raw = append(raw, rb[1:]...)
	return raw, header
}

// indexReadMe describes what an index response holds: total matched
// declarations, k listed with lines, m listed in all, the rows not listed
// counted for shown of all files, and the lists cut.
func indexReadMe(total, k, m, shown, all int, cut []string, tool string) string {
	text := fmt.Sprintf("This result is too large for one response, so it is an index of the %d matched declarations in rank order", total)
	switch {
	case m == 0:
		text += ": not even their names fit, so none is listed."
	case k == m && m == total:
		text += ": all of them, each with its matched line numbers and the calls and uses on them, without the code."
	case k == m:
		text += fmt.Sprintf(": the first %d, each with its matched line numbers and the calls and uses on them, without the code.", m)
	default:
		text += fmt.Sprintf(": the first %d with their matched line numbers and the calls and uses on them, the next %d by name, file and lines only, without the code.", k, m-k)
	}
	if m > 0 {
		text += fmt.Sprintf(" To see the code of the declarations you choose, call %s with the same pattern and ids=[their qname, ...].", ToolName(tool))
	}
	if m < total {
		switch shown {
		case all:
			text += fmt.Sprintf(" %d declarations are not listed; not_listed counts them per file.", total-m)
		case 0:
			text += fmt.Sprintf(" %d declarations are not listed; not_listed_files_left_out counts them and their %d files.", total-m, all)
		default:
			text += fmt.Sprintf(" %d declarations are not listed; not_listed counts them per file for the %d files with the most of them, "+
				"and not_listed_files_left_out counts the other %d files.", total-m, shown, all-shown)
		}
	}
	if len(cut) > 0 {
		text += " " + strings.Join(cut, "; ") + "."
	}
	if m < total || len(cut) > 0 {
		text += " Narrow the search to see more: add path (a folder or file) or make the pattern more specific."
	}
	return text
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
	if pageFits(full) {
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
		"It has the keys of a full %s result, which its tool description explains. "+
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
// calls either way and value reads and writes either way, and for Solidity
// the modifiers applied, events emitted and errors reverted with.
var reasonEdges = map[types.EdgeType]bool{
	types.EdgeCalls: true, types.EdgeInvokes: true, types.EdgeReadsField: true,
	types.EdgeWritesField: true, types.EdgeReferences: true,
	types.EdgeReadsMapping: true, types.EdgeWritesMapping: true,
	types.EdgeHasModifier: true, types.EdgeEmitsEvent: true, types.EdgeRevertsWith: true,
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
		m := map[string]any{"qname": store.PrintedName(reader, n), "type": string(n.Type), "file": n.FilePath,
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
