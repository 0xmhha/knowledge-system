package mcphandlers

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// investigate.go works through a problem the way an engineer does with a few
// words in hand. For each word it finds:
//
//   - what the word names: a function or method, a value (field, package-level
//     variable, constant) or a type; or nothing, when it is a local name;
//   - every declaration whose source holds the word, with the lines;
//   - for a function, every line that calls, reads or writes something, so a
//     call on a line without the word is not lost; for a value, who writes
//     it and who reads it, with the source of each line; for a type, its
//     fields and methods and the fields that hold it;
//   - everything reachable from those, hop by hop, over calls, value uses and
//     type uses, up to three hops; and what two or more words reach.
//
// All of it is computed and kept for the session (one server process per
// client session). The response holds as much as the budget allows: each
// layer (what the words name, where they appear, joins, first hops, the
// words inside longer names) gets a share, what a layer leaves unused goes to
// the next, and the rest is listed with the handle investigate_open takes.
// The first response shows compact cards; open card:<qname> gives the full
// one. Cards shown earlier in the session are not repeated.

const (
	invDefaultBudget = 24000
	invMaxBudget     = 400000
	invMinBudget     = 600
	invHops          = 3
	invMaxWords      = 12
	invMinWord       = 3
	invLineWidth     = 120
	invCallerCap     = 20
	invJoinCap       = 200
	invHopListCap    = 200

	// compact cards in the investigate response
	invCompactCallers = 8
	invCompactWriters = 12
	invCompactReaders = 8
	invCompactNames   = 15
)

// invShares is the part of the budget each layer starts with, in layer order.
var invShares = []float64{0.25, 0.40, 0.12, 0.15, 0.08}

// invEdgeTypes are the relations the expansion follows, both ways.
var invEdgeTypes = []types.EdgeType{
	types.EdgeCalls, types.EdgeInvokes, types.EdgeReadsField, types.EdgeWritesField, types.EdgeReferences,
	types.EdgeUsesType, types.EdgeInstantiates, types.EdgeImplements, types.EdgeSpawns,
	types.EdgeDispatchesTo, types.EdgeSendsTo, types.EdgeRecvsFrom, types.EdgeFlowsTo,
}

var invKinds = map[types.NodeType]bool{
	types.NodeFunction: true, types.NodeMethod: true, types.NodeStruct: true, types.NodeInterface: true,
	types.NodeTypeAlias: true, types.NodeConstant: true, types.NodeVariable: true, types.NodeField: true,
	types.NodeChannel: true,
}

type invEdge struct {
	other string // the node at the other end
	typ   types.EdgeType
	kind  string // dispatch_kind: read or write on references
	line  int    // the line of the use, in the source node's file
	out   bool   // this node is the edge's source
}

func (e invEdge) write() bool {
	return e.typ == types.EdgeWritesField || e.typ == types.EdgeReferences && e.kind == "write"
}

func (e invEdge) valueUse() bool {
	return e.typ == types.EdgeReadsField || e.typ == types.EdgeWritesField || e.typ == types.EdgeReferences
}

// verb is how the edge reads from its source.
func (e invEdge) verb() string {
	switch e.typ {
	case types.EdgeCalls, types.EdgeInvokes:
		return "calls"
	case types.EdgeReadsField, types.EdgeReferences, types.EdgeWritesField:
		if e.write() {
			return "writes"
		}
		return "reads"
	case types.EdgeUsesType:
		return "uses type"
	case types.EdgeInstantiates:
		return "creates"
	case types.EdgeImplements:
		return "implements"
	case types.EdgeSpawns:
		return "starts goroutine"
	case types.EdgeDispatchesTo:
		if e.kind == "type_hierarchy" {
			return "may run (by type)"
		}
		return "may run"
	case types.EdgeFlowsTo:
		return "is stored in"
	case types.EdgeSendsTo:
		return "sends to"
	case types.EdgeRecvsFrom:
		return "receives from"
	}
	return string(e.typ)
}

type invIndex struct {
	// reader is the graph the index was built from; card: resolves a name
	// that has no card (a local) through it.
	reader    store.Reader
	code      *codeIndex
	nodes     map[string]types.Node
	test      map[string]bool
	declOf    map[string]int // node id -> index into code.decls
	byLeaf    map[string][]string
	byFold    map[string][]string
	adj       map[string][]invEdge
	members   map[string][]string // struct or interface -> fields and methods
	owner     map[string]string   // field or method -> struct or interface
	fieldDecl map[string]string   // field id -> its declaration line

	// Statement links (schema 1.32): for a write of a value on a line, the
	// innermost statement whose body holds it, and each statement's parent.
	guard    map[guardKey]invGuard
	parent   map[string]invGuard // statement id -> the statement directly around it
	stmt     map[string]types.Node
	funcFile map[string][]string // file -> functions and methods in it
}

type guardKey struct {
	value, file string
	line        int
}

// invGuard is a statement and the part of it (then, else, body, case, go,
// cond) that holds what it guards.
type invGuard struct {
	id, kind string
}

func leafName(q string) string {
	if i := strings.LastIndexByte(q, '.'); i >= 0 {
		return q[i+1:]
	}
	return q
}

func isTestFile(f string) bool {
	return strings.HasSuffix(f, "_test.go") || strings.Contains(f, "/testdata/")
}

// testFileRule returns what investigate takes for test code: the files the
// build recorded as test code (store.TestFiles: the build scope's file role,
// or the build's path rule without a scope), or isTestFile for a graph that
// holds no such record.
func testFileRule(reader store.Reader) (func(string) bool, error) {
	files, recorded, err := store.TestFiles(reader)
	if err != nil {
		return nil, fmt.Errorf("test files: %w", err)
	}
	if !recorded {
		return isTestFile, nil
	}
	set := make(map[string]bool, len(files))
	for _, f := range files {
		set[f] = true
	}
	return func(f string) bool { return set[f] }, nil
}

func buildInvIndex(reader store.Reader) (*invIndex, error) {
	code, err := codeIndexFor(reader)
	if err != nil {
		return nil, err
	}
	all, err := reader.AllNodes()
	if err != nil {
		return nil, err
	}
	isTest, err := testFileRule(reader)
	if err != nil {
		return nil, err
	}
	x := &invIndex{reader: reader, code: code, nodes: map[string]types.Node{}, test: map[string]bool{}, declOf: map[string]int{},
		byLeaf: map[string][]string{}, byFold: map[string][]string{}, adj: map[string][]invEdge{},
		members: map[string][]string{}, owner: map[string]string{}, fieldDecl: map[string]string{},
		guard: map[guardKey]invGuard{}, parent: map[string]invGuard{}, stmt: map[string]types.Node{}, funcFile: map[string][]string{}}
	for _, n := range all {
		if !invKinds[n.Type] || n.Type == types.NodeVariable && n.CanonicalID == "" {
			continue
		}
		x.nodes[n.ID] = n
		x.test[n.ID] = isTest(n.FilePath)
		leaf := leafName(n.QualifiedName)
		x.byLeaf[leaf] = append(x.byLeaf[leaf], n.ID)
		x.byFold[strings.ToLower(leaf)] = append(x.byFold[strings.ToLower(leaf)], n.ID)
	}
	for i, d := range code.decls {
		x.declOf[d.node.ID] = i
	}
	for _, t := range invEdgeTypes {
		es, err := reader.QueryEdgesByType(string(t))
		if err != nil {
			return nil, err
		}
		for _, e := range es {
			// An invokes edge back to its own function stands for a call
			// through a function value, whose target is unknown; a calls
			// edge to itself is a recursive call, and the function is one
			// of its own callers, as find_callers lists it.
			if e.Src == e.Dst && e.Type == types.EdgeInvokes {
				continue
			}
			if _, ok := x.nodes[e.Src]; !ok {
				continue
			}
			if _, ok := x.nodes[e.Dst]; !ok {
				continue
			}
			x.adj[e.Src] = append(x.adj[e.Src], invEdge{other: e.Dst, typ: e.Type, kind: e.DispatchKind, line: e.Line, out: true})
			x.adj[e.Dst] = append(x.adj[e.Dst], invEdge{other: e.Src, typ: e.Type, kind: e.DispatchKind, line: e.Line})
		}
	}
	defs, err := reader.QueryEdgesByType(string(types.EdgeDefines))
	if err != nil {
		return nil, err
	}
	for _, e := range defs {
		p, ok := x.nodes[e.Src]
		if !ok || p.Type != types.NodeStruct && p.Type != types.NodeInterface {
			continue
		}
		if _, ok := x.nodes[e.Dst]; !ok {
			continue
		}
		x.members[e.Src] = append(x.members[e.Src], e.Dst)
		x.owner[e.Dst] = e.Src
	}
	for id, n := range x.nodes {
		if n.Type != types.NodeField {
			continue
		}
		if t := x.lineIn(x.owner[id], n.StartLine); t != "" {
			x.fieldDecl[id] = t
		} else if n.Signature != "" {
			x.fieldDecl[id] = n.Signature
		}
	}
	for id := range x.adj {
		es := x.adj[id]
		sort.Slice(es, func(i, j int) bool {
			if es[i].line != es[j].line {
				return es[i].line < es[j].line
			}
			// Same line: no evidence orders these. The node id keeps the
			// order the same on every call without favouring names that
			// start with "a".
			if es[i].other != es[j].other {
				return es[i].other < es[j].other
			}
			return es[i].typ < es[j].typ
		})
	}
	if err := x.loadGuards(reader); err != nil {
		return nil, err
	}
	for _, m := range [](map[string][]string){x.members, x.byLeaf, x.byFold} {
		for k := range m {
			ids := m[k]
			sort.Slice(ids, func(i, j int) bool { return x.less(ids[i], ids[j]) })
		}
	}
	return x, nil
}

// less is the last tie-break of every investigate list: production before
// tests, then the node id. Nodes equal on every key before it are ties; the
// id (a hash) keeps their order the same on every call and carries no
// meaning. It used to be the qualified name, which put names starting with
// "a" first (user decision 2026-11-19, as for search_code).
func (x *invIndex) less(a, b string) bool {
	if x.test[a] != x.test[b] {
		return !x.test[a]
	}
	return a < b
}

// lineIn returns source line `line` of declaration id, trimmed.
func (x *invIndex) lineIn(id string, line int) string {
	i, ok := x.declOf[id]
	if !ok {
		return ""
	}
	d := x.code.decls[i]
	if off := line - d.node.StartLine; off >= 0 && off < len(d.lines) {
		return clip(strings.TrimSpace(d.lines[off]), invLineWidth)
	}
	return ""
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

var invIndexes sync.Map // store.Reader -> *lazyInvIndex

type lazyInvIndex struct {
	once sync.Once
	idx  *invIndex
	err  error
}

func invIndexFor(reader store.Reader) (*invIndex, error) {
	key := reader
	if w, ok := reader.(*llmSafeReader); ok {
		key = w.Reader
	}
	v, _ := invIndexes.LoadOrStore(key, &lazyInvIndex{})
	l := v.(*lazyInvIndex)
	l.once.Do(func() { l.idx, l.err = buildInvIndex(reader) })
	return l.idx, l.err
}

// --- words ---

// invTerm is one word of the input: a literal identifier, or a pattern when
// the input joined two parts with a wildcard (Override\w+Time).
type invTerm struct {
	label string         // as shown: the word, or the pattern with * for each wildcard
	lit   string         // the literal word; empty for a pattern
	rx    *regexp.Regexp // pattern: a whole identifier in a line
	name  *regexp.Regexp // pattern: a whole declaration name
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true, "defer": true,
	"else": true, "fallthrough": true, "for": true, "func": true, "go": true, "goto": true, "if": true,
	"import": true, "interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true, "nil": true, "err": true,
	"true": true, "false": true,
}

// letterClass matches a character class of letters only ([Ss], [Vv]).
var letterClass = regexp.MustCompile(`\[([A-Za-z]{1,4})\]`)

// splitTerms takes the words out of what an agent typed. "a|b", "a, b" and
// "a b" give a and b. Regular-expression input is read as the agent meant
// it: a letter class gives each spelling ([Ss]ig: Sig and sig), a wildcard
// between two parts keeps them as one pattern (Override\w+Time), and escapes
// such as \. and \( separate words.
func splitTerms(in string) []invTerm {
	var out []invTerm
	seen := map[string]bool{}
	addLit := func(w string) {
		if len(w) < invMinWord || goKeywords[w] || seen[w] || w[0] >= '0' && w[0] <= '9' {
			return
		}
		seen[w] = true
		out = append(out, invTerm{label: w, lit: w})
	}
	addPattern := func(parts []string, lead, trail bool) {
		label := strings.Join(parts, "*")
		if lead {
			label = "*" + label
		}
		if trail {
			label += "*"
		}
		lit := 0
		for _, p := range parts {
			lit += len(p)
		}
		if lit < invMinWord || seen[label] {
			return
		}
		seen[label] = true
		q := make([]string, len(parts))
		for i, p := range parts {
			q[i] = regexp.QuoteMeta(p)
		}
		body := strings.Join(q, `\w*`)
		if lead {
			body = `\w*` + body
		}
		if trail {
			body += `\w*`
		}
		out = append(out, invTerm{label: label, rx: regexp.MustCompile(`\b` + body + `\b`), name: regexp.MustCompile(`^` + body + `$`)})
	}
	for _, piece := range strings.FieldsFunc(in, func(r rune) bool {
		return r == '|' || r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '(' || r == ')'
	}) {
		for _, sp := range spellings(piece) {
			splitPiece(sp, addLit, addPattern)
		}
	}
	if len(out) > invMaxWords {
		out = out[:invMaxWords]
	}
	return out
}

// spellings expands the letter classes of a piece, at most four spellings.
func spellings(piece string) []string {
	out := []string{piece}
	for {
		m := letterClass.FindStringSubmatchIndex(out[0])
		if m == nil {
			return out
		}
		letters := out[0][m[2]:m[3]]
		var next []string
		for _, s := range out {
			mm := letterClass.FindStringSubmatchIndex(s)
			for _, c := range letters {
				if len(next) == 4 {
					break
				}
				next = append(next, s[:mm[0]]+string(c)+s[mm[1]:])
			}
		}
		out = next
	}
}

// splitPiece reads one piece left to right: identifier runs, wildcards
// (.*, .+, \w*, \w+, \w, \S, a class such as [A-Z]) that join runs into a
// pattern, and anything else, which ends the current word.
func splitPiece(p string, addLit func(string), addPattern func([]string, bool, bool)) {
	var parts []string
	var cur strings.Builder
	lead, wild := false, false
	end := func() {
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
		switch {
		case len(parts) == 1 && !lead && !wild:
			addLit(parts[0])
		case len(parts) > 0:
			addPattern(parts, lead, wild)
		}
		parts, lead, wild = nil, false, false
	}
	wildcard := func() {
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
		if len(parts) == 0 {
			lead = true
		}
		wild = true
	}
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case isIdentByte(c):
			cur.WriteByte(c)
			wild = false
		case c == '\\' && i+1 < len(p) && (p[i+1] == 'w' || p[i+1] == 'S'):
			i++
			if i+1 < len(p) && (p[i+1] == '*' || p[i+1] == '+') {
				i++
			}
			wildcard()
		case c == '.' && i+1 < len(p) && (p[i+1] == '*' || p[i+1] == '+'):
			i++
			wildcard()
		case c == '[':
			j := strings.IndexByte(p[i:], ']')
			if j < 0 {
				end()
				continue
			}
			i += j
			if i+1 < len(p) && (p[i+1] == '*' || p[i+1] == '+') {
				i++
			}
			wildcard()
		case c == '\\' && i+1 < len(p):
			i++ // an escaped character (\. \( \b) ends the word
			end()
		default:
			end()
		}
	}
	end()
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

// wholeIn reports whether w occurs in s, and whether one occurrence is a
// whole identifier.
func wholeIn(s, w string) (found, whole bool) {
	for from := 0; ; {
		i := strings.Index(s[from:], w)
		if i < 0 {
			return found, false
		}
		i += from
		found = true
		before := i == 0 || !isIdentByte(s[i-1])
		after := i+len(w) == len(s) || !isIdentByte(s[i+len(w)])
		if before && after {
			return true, true
		}
		from = i + 1
	}
}

// match reports whether line holds the term, and whether as a whole
// identifier. A pattern matches whole identifiers only.
func (t invTerm) match(line string) (found, whole bool) {
	if t.rx == nil {
		return wholeIn(line, t.lit)
	}
	if t.rx.MatchString(line) {
		return true, true
	}
	return false, false
}

// --- one word ---

type invOcc struct {
	id    string
	lines []int
}

type invVia struct {
	from string
	edge invEdge
}

type invWord struct {
	invTerm
	entities []string // declarations the word names
	folded   bool     // named only when case is ignored
	occ      []invOcc // declarations holding the word as a whole identifier
	partial  []invOcc // declarations holding it only inside a longer name
	hop      map[string]int
	via      map[string]invVia
	perHop   [invHops + 1]int
}

func (x *invIndex) resolveWord(t invTerm, withTests bool) *invWord {
	r := &invWord{invTerm: t, hop: map[string]int{}, via: map[string]invVia{}}
	keep := func(id string) bool { return withTests || !x.test[id] }
	if t.name != nil {
		for id, n := range x.nodes {
			if keep(id) && t.name.MatchString(leafName(n.QualifiedName)) {
				r.entities = append(r.entities, id)
			}
		}
		sort.Slice(r.entities, func(i, j int) bool { return x.less(r.entities[i], r.entities[j]) })
	} else {
		for _, id := range x.byLeaf[t.lit] {
			if keep(id) {
				r.entities = append(r.entities, id)
			}
		}
		if len(r.entities) == 0 {
			for _, id := range x.byFold[strings.ToLower(t.lit)] {
				if keep(id) {
					r.entities = append(r.entities, id)
					r.folded = true
				}
			}
		}
	}
	for _, d := range x.code.decls {
		if !withTests && d.test {
			continue
		}
		var whole, part []int
		for k, l := range d.lines {
			f, wh := t.match(l)
			switch {
			case wh:
				whole = append(whole, d.node.StartLine+k)
			case f:
				part = append(part, d.node.StartLine+k)
			}
		}
		if len(whole) > 0 {
			r.occ = append(r.occ, invOcc{d.node.ID, whole})
		} else if len(part) > 0 {
			r.partial = append(r.partial, invOcc{d.node.ID, part})
		}
	}
	byID := func(o []invOcc) { sort.SliceStable(o, func(i, j int) bool { return x.less(o[i].id, o[j].id) }) }
	byID(r.occ)
	byID(r.partial)

	// Breadth-first from everything the word names or appears in, both ways
	// over every followed relation.
	var frontier []string
	add := func(id string) {
		if _, ok := r.hop[id]; !ok && keep(id) {
			r.hop[id] = 0
			frontier = append(frontier, id)
		}
	}
	for _, id := range r.entities {
		add(id)
	}
	for _, o := range r.occ {
		add(o.id)
	}
	for _, o := range r.partial {
		add(o.id)
	}
	sort.Slice(frontier, func(i, j int) bool { return x.less(frontier[i], frontier[j]) })
	r.perHop[0] = len(frontier)
	for h := 1; h <= invHops; h++ {
		var next []string
		for _, id := range frontier {
			for _, e := range x.adj[id] {
				if _, ok := r.hop[e.other]; ok || !keep(e.other) {
					continue
				}
				r.hop[e.other] = h
				r.via[e.other] = invVia{from: id, edge: e}
				next = append(next, e.other)
			}
		}
		r.perHop[h] = len(next)
		frontier = next
	}
	return r
}

// --- cards ---

// capRows keeps the first n rows and says how many more there are and how
// to see them.
func (x *invIndex) capRows(rows []string, n int, handle string) []string {
	if n <= 0 || len(rows) <= n {
		return rows
	}
	return append(rows[:n:n], fmt.Sprintf("    … %d more: open=%q", len(rows)-n, handle))
}

func capNames(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:n], ", ") + fmt.Sprintf(" … %d more", len(names)-n)
}

func capJoin(v []string, n int, sep string) string {
	if len(v) <= n {
		return strings.Join(v, sep)
	}
	return strings.Join(v[:n], sep) + fmt.Sprintf("%s… %d more", sep, len(v)-n)
}

func (x *invIndex) header(id string) string {
	n := x.nodes[id]
	return fmt.Sprintf("%s %s  %s:%d-%d", strings.ToLower(string(n.Type)), n.QualifiedName, n.FilePath, n.StartLine, n.EndLine)
}

// funcCard lists the function's lines, each with what it calls, reads or
// writes, and its callers. Full: every line that uses something or holds a
// word. Compact: the signature, the lines with a word, a call, a write, a
// new value or goroutine; reads on other lines are summed up in one line.
func (x *invIndex) funcCard(id string, words []*invWord, withTests, compact bool) string {
	n := x.nodes[id]
	var b strings.Builder
	b.WriteString(x.header(id) + "\n")
	uses := map[int][]string{}
	strong := map[int]bool{}
	var readNames []string
	readSeen := map[string]bool{}
	for _, e := range x.adj[id] {
		if !e.out || e.line < n.StartLine || e.line > n.EndLine {
			continue
		}
		q := x.nodes[e.other].QualifiedName
		u := e.verb() + " " + q
		if e.write() {
			u += x.guardNote(e.other, n.FilePath, e.line)
		}
		if l := uses[e.line]; len(l) == 0 || l[len(l)-1] != u {
			uses[e.line] = append(uses[e.line], u)
		}
		if !e.valueUse() || e.write() {
			strong[e.line] = true
		} else if !readSeen[q] {
			readSeen[q] = true
			readNames = append(readNames, q)
		}
	}
	wordLine := map[int]bool{}
	if i, ok := x.declOf[id]; ok {
		for k, l := range x.code.decls[i].lines {
			for _, w := range words {
				if _, wh := w.match(l); wh {
					wordLine[n.StartLine+k] = true
				}
			}
		}
	}
	lines := map[int]bool{n.StartLine: true} // the signature, with any use on it
	for l := range uses {
		if !compact || strong[l] || wordLine[l] {
			lines[l] = true
		}
	}
	for l := range wordLine {
		lines[l] = true
	}
	order := make([]int, 0, len(lines))
	for l := range lines {
		order = append(order, l)
	}
	sort.Ints(order)
	for _, l := range order {
		fmt.Fprintf(&b, "  %5d  %s", l, x.lineIn(id, l))
		if u := uses[l]; len(u) > 0 {
			fmt.Fprintf(&b, "   ← %s", strings.Join(u, "; "))
		}
		b.WriteByte('\n')
	}
	if compact {
		hidden := 0
		for l := range uses {
			if !lines[l] {
				hidden++
			}
		}
		if hidden > 0 {
			fmt.Fprintf(&b, "  reads on %d more lines: %s\n", hidden, capNames(readNames, invCompactNames))
		}
	}
	var callers []string
	seen := map[string]bool{}
	for _, e := range x.adj[id] {
		if e.out || e.typ != types.EdgeCalls && e.typ != types.EdgeInvokes || seen[e.other] || !withTests && x.test[e.other] {
			continue
		}
		seen[e.other] = true
		callers = append(callers, x.nodes[e.other].QualifiedName+":"+strconv.Itoa(e.line))
	}
	if len(callers) > 0 {
		c := invCallerCap
		if compact {
			c = invCompactCallers
		}
		fmt.Fprintf(&b, "  callers: %s", capNames(callers, c))
		if len(callers) > c {
			// The card lists the direct callers (calls and invokes edges
			// to the function itself), without test code unless tests are
			// included: find_callers with direct=true, and tests=exclude
			// when they are left out, returns exactly those.
			tests := " tests=" + testsExclude
			if withTests {
				tests = ""
			}
			fmt.Fprintf(&b, "; find_callers qname=%q depth=1 direct=true%s returns exactly them", seedName(n), tests)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// seedName is the name that names n alone in a seed: its canonical id, or
// its qualified name when it has none.
func seedName(n types.Node) string {
	if n.CanonicalID != "" {
		return n.CanonicalID
	}
	return n.QualifiedName
}

// valueCard lists who writes the value and who reads it, one row per line
// with the line's source. Compact: up to invCompactWriters writer lines and
// invCompactReaders reader lines, then the other users by name.
func (x *invIndex) valueCard(id string, withTests, compact bool) string {
	n := x.nodes[id]
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s  %s:%d", strings.ToLower(string(n.Type)), n.QualifiedName, n.FilePath, n.StartLine)
	if d := x.fieldDecl[id]; d != "" {
		fmt.Fprintf(&b, "  decl: %s", d)
	} else if d := x.lineIn(id, n.StartLine); d != "" {
		fmt.Fprintf(&b, "  decl: %s", d)
	}
	b.WriteByte('\n')
	if o := x.owner[id]; o != "" {
		fmt.Fprintf(&b, "  in %s\n", x.nodes[o].QualifiedName)
	}
	for _, write := range []bool{true, false} {
		var rows, names []string
		users := map[string]bool{}
		for _, e := range x.adj[id] {
			if e.out || !e.valueUse() || e.write() != write || !withTests && x.test[e.other] {
				continue
			}
			q := x.nodes[e.other].QualifiedName
			if !users[e.other] {
				names = append(names, q)
			}
			users[e.other] = true
			row := fmt.Sprintf("    %s:%d  %s", q, e.line, x.lineIn(e.other, e.line))
			if write {
				row += x.guardNote(id, x.nodes[e.other].FilePath, e.line)
			}
			rows = append(rows, row)
		}
		label, c := "readers", invCompactReaders
		if write {
			label, c = "writers", invCompactWriters
		}
		fmt.Fprintf(&b, "  %s (%d functions, %d lines)\n", label, len(users), len(rows))
		if compact && len(rows) > c {
			for _, r := range rows[:c] {
				b.WriteString(r + "\n")
			}
			fmt.Fprintf(&b, "    … %d more lines, in: %s; open=\"card:%s\"\n", len(rows)-c, capNames(names, invCompactNames), n.QualifiedName)
			continue
		}
		for _, r := range rows {
			b.WriteString(r + "\n")
		}
	}
	// The other names of the same channel (flows_to, schema 1.34).
	var same []string
	sameSeen := map[string]bool{}
	for _, e := range x.adj[id] {
		if e.typ == types.EdgeFlowsTo && !sameSeen[e.other] {
			sameSeen[e.other] = true
			same = append(same, x.nodes[e.other].QualifiedName)
		}
	}
	if len(same) > 0 {
		fmt.Fprintf(&b, "  same channel as: %s\n", capNames(same, invCompactNames))
	}
	// A channel (a make site, or a field or variable that holds one): who
	// sends and who receives on it.
	for _, op := range []types.EdgeType{types.EdgeSendsTo, types.EdgeRecvsFrom} {
		var rows []string
		for _, e := range x.adj[id] {
			if e.out || e.typ != op || e.line == 0 || !withTests && x.test[e.other] {
				continue
			}
			rows = append(rows, fmt.Sprintf("    %s:%d  %s", x.nodes[e.other].QualifiedName, e.line, x.lineIn(e.other, e.line)))
		}
		if len(rows) == 0 {
			continue
		}
		label := "senders"
		if op == types.EdgeRecvsFrom {
			label = "receivers"
		}
		fmt.Fprintf(&b, "  %s (%d lines)\n", label, len(rows))
		if compact && len(rows) > invCompactReaders {
			fmt.Fprintf(&b, "%s\n    … %d more lines; open=\"card:%s\"\n", strings.Join(rows[:invCompactReaders], "\n"), len(rows)-invCompactReaders, n.QualifiedName)
			continue
		}
		b.WriteString(strings.Join(rows, "\n") + "\n")
	}
	return b.String()
}

// loadGuards reads the statement links (schema 1.32): body_writes gives, for
// a write of a value on a line, the innermost statement around it; encloses
// gives each statement's parent. Older graphs have neither, and the notes
// are left out.
func (x *invIndex) loadGuards(reader store.Reader) error {
	bw, err := reader.QueryEdgesByType(string(types.EdgeBodyWrites))
	if err != nil {
		return err
	}
	if len(bw) == 0 {
		return nil
	}
	enc, err := reader.QueryEdgesByType(string(types.EdgeEncloses))
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, e := range bw {
		x.guard[guardKey{e.Dst, e.FilePath, e.Line}] = invGuard{e.Src, e.DispatchKind}
		ids[e.Src] = true
	}
	for _, e := range enc {
		x.parent[e.Dst] = invGuard{e.Src, e.DispatchKind}
		ids[e.Src] = true
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	ns, err := reader.NodesByIDs(list)
	if err != nil {
		return err
	}
	for _, n := range ns {
		x.stmt[n.ID] = n
	}
	// Only functions with stored source, in id order: a promoted copy has
	// the lines of the method it copies and no source of its own, and with
	// the copies listed in map order stmtLine took one of them on some runs
	// and wrote "[then of L167 ; ...]" without the statement's line.
	for id, n := range x.nodes {
		if _, src := x.declOf[id]; src && (n.Type == types.NodeFunction || n.Type == types.NodeMethod) {
			x.funcFile[n.FilePath] = append(x.funcFile[n.FilePath], id)
		}
	}
	for _, ids := range x.funcFile {
		sort.Strings(ids)
	}
	return nil
}

// invGuardDepth bounds the statements named around one write.
const invGuardDepth = 3

// guardNote names the statements that decide whether a write of value on
// line runs, innermost first: the innermost with its source line, the ones
// around it by line only, "[then of L56 if …; in then of L54, body of L30]";
// open the function's card for their source.
func (x *invIndex) guardNote(value, file string, line int) string {
	g, ok := x.guard[guardKey{value, file, line}]
	if !ok {
		return ""
	}
	var parts, outer []string
	for i := 0; i < invGuardDepth && g.id != ""; i++ {
		sn, ok := x.stmt[g.id]
		if !ok {
			break
		}
		if i == 0 {
			parts = append(parts, fmt.Sprintf("%s of L%d %s", g.kind, sn.StartLine, clip(x.stmtLine(sn), 70)))
		} else {
			outer = append(outer, fmt.Sprintf("%s of L%d", g.kind, sn.StartLine))
		}
		g = x.parent[g.id]
	}
	if len(outer) > 0 {
		parts = append(parts, "in "+strings.Join(outer, ", "))
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, "; ") + "]"
}

// stmtLine is the source line a statement starts on, from the innermost
// function that holds it.
func (x *invIndex) stmtLine(sn types.Node) string {
	best, span := "", 1<<30
	for _, f := range x.funcFile[sn.FilePath] {
		n := x.nodes[f]
		if sn.StartLine >= n.StartLine && sn.StartLine <= n.EndLine && n.EndLine-n.StartLine < span {
			best, span = f, n.EndLine-n.StartLine
		}
	}
	if best == "" {
		return ""
	}
	return x.lineIn(best, sn.StartLine)
}

// typeCard lists a type's fields and methods, the fields declared with it,
// and the functions that create or use it.
func (x *invIndex) typeCard(id string, withTests, compact bool) string {
	n := x.nodes[id]
	var b strings.Builder
	b.WriteString(x.header(id) + "\n")
	limit := 1 << 30
	if compact {
		limit = invCompactNames
	}
	var fields, methods []string
	for _, m := range x.members[id] {
		switch x.nodes[m].Type {
		case types.NodeField:
			fields = append(fields, x.fieldDecl[m])
		default:
			methods = append(methods, leafName(x.nodes[m].QualifiedName))
		}
	}
	if len(fields) > 0 {
		fmt.Fprintf(&b, "  fields: %s\n", capJoin(fields, limit, "; "))
	}
	if len(methods) > 0 {
		fmt.Fprintf(&b, "  methods: %s\n", capNames(methods, limit))
	}
	leaf := leafName(n.QualifiedName)
	var held []string
	for fid, d := range x.fieldDecl {
		if x.owner[fid] == id || !withTests && x.test[fid] {
			continue
		}
		// "name Type", "name *pkg.Type", "name []Type": the type follows the name.
		if sp := strings.IndexAny(d, " \t"); sp > 0 {
			if _, wh := wholeIn(d[sp:], leaf); wh {
				held = append(held, x.nodes[fid].QualifiedName)
			}
		}
	}
	sort.Strings(held)
	if len(held) > 0 {
		fmt.Fprintf(&b, "  held by fields: %s\n", capNames(held, limit))
	}
	for _, rel := range []types.EdgeType{types.EdgeInstantiates, types.EdgeUsesType} {
		var us []string
		seen := map[string]bool{}
		for _, e := range x.adj[id] {
			if e.out || e.typ != rel || seen[e.other] || !withTests && x.test[e.other] {
				continue
			}
			seen[e.other] = true
			us = append(us, x.nodes[e.other].QualifiedName)
		}
		if len(us) > 0 {
			label := "used by"
			if rel == types.EdgeInstantiates {
				label = "created in"
			}
			fmt.Fprintf(&b, "  %s: %s\n", label, capNames(us, limit))
		}
	}
	return b.String()
}

func (x *invIndex) card(id string, words []*invWord, withTests, compact bool) string {
	switch x.nodes[id].Type {
	case types.NodeFunction, types.NodeMethod:
		return x.funcCard(id, words, withTests, compact)
	case types.NodeField, types.NodeVariable, types.NodeConstant, types.NodeChannel:
		return x.valueCard(id, withTests, compact)
	default:
		return x.typeCard(id, withTests, compact)
	}
}

// --- session ---

type investigation struct {
	id        string
	words     []*invWord
	withTests bool
	cut       []invBlock // what the investigate response left out, in order
}

type invSession struct {
	mu    sync.Mutex
	seq   int
	invs  map[string]*investigation
	order []string
	shown map[string]string // node id -> investigation that showed its card
}

var invSessions sync.Map // *server.MCPServer -> *invSession

func sessionFor(s *server.MCPServer) *invSession {
	v, _ := invSessions.LoadOrStore(s, &invSession{invs: map[string]*investigation{}, shown: map[string]string{}})
	return v.(*invSession)
}

// --- response ---

// invBlock is one unit of the response. A card is whole or left out; a list
// (head and entries) may be cut after any entry.
type invBlock struct {
	layer   int
	handle  string // what investigate_open takes to return this block alone
	what    string // a few words on what the block holds
	text    string // a card
	head    string // a list: its first line
	entries []string
	cards   []string // node ids whose cards the block shows
}

func (b invBlock) size() int {
	if b.head == "" {
		return len(b.text)
	}
	n := len(b.head)
	for _, e := range b.entries {
		n += len(e)
	}
	return n
}

// render returns the block in at most room characters; a list that does not
// fit whole ends with a line saying how to see the rest. ok is false when
// not even a card, or a list's head and first entry, fit.
func (b invBlock) render(room int) (text string, whole, ok bool) {
	if b.head == "" {
		return b.text, true, len(b.text) <= room
	}
	if b.size() <= room {
		return b.head + strings.Join(b.entries, ""), true, true
	}
	var s strings.Builder
	s.WriteString(b.head)
	for i, e := range b.entries {
		more := fmt.Sprintf("  … %d more: open=%q\n", len(b.entries)-i, b.handle)
		if s.Len()+len(e)+len(more) > room {
			if i == 0 {
				return "", false, false
			}
			s.WriteString(more)
			return s.String(), false, true
		}
		s.WriteString(e)
	}
	return s.String(), true, true
}

// fill decides what goes into the response. Each layer gets its share of the
// room; what a layer does not use goes to the next; then whatever still fits
// is added in order. The text keeps the block order. A list cut short counts
// as included; its handle returns all of it.
func fill(blocks []invBlock, room int) (string, []invBlock, []invBlock) {
	picked := make([]*string, len(blocks))
	used, carry := 0, 0
	for layer := range invShares {
		allow := int(float64(room)*invShares[layer]) + carry
		spent := 0
		for i, bl := range blocks {
			if bl.layer != layer+1 || picked[i] != nil {
				continue
			}
			if t, _, ok := bl.render(allow - spent); ok {
				picked[i] = &t
				spent += len(t)
			}
		}
		carry = allow - spent
		used += spent
	}
	for i, bl := range blocks {
		if picked[i] != nil {
			continue
		}
		if t, _, ok := bl.render(room - used); ok {
			picked[i] = &t
			used += len(t)
		}
	}
	var b strings.Builder
	var in, cut []invBlock
	for i, bl := range blocks {
		if picked[i] == nil {
			cut = append(cut, bl)
			continue
		}
		b.WriteString(*picked[i])
		in = append(in, bl)
	}
	return b.String(), in, cut
}

func (x *invIndex) summary(inv *investigation) string {
	var b strings.Builder
	words := make([]string, len(inv.words))
	for i, w := range inv.words {
		words[i] = w.label
	}
	tests := "tests excluded"
	if inv.withTests {
		tests = "tests included"
	}
	fmt.Fprintf(&b, "investigation %s · words: %s · %s\n", inv.id, strings.Join(words, " | "), tests)
	for _, w := range inv.words {
		var names []string
		for _, id := range w.entities {
			n := x.nodes[id]
			names = append(names, strings.ToLower(string(n.Type))+" "+n.QualifiedName)
		}
		named := "names no declaration (a local or part of a name)"
		if len(names) > 0 {
			named = "names " + capNames(names, 6)
			if w.folded {
				named += " (case ignored)"
			}
		}
		fmt.Fprintf(&b, "- %s: %s; in %d declarations as a word, %d inside longer names; reach hop1 %d, hop2 %d, hop3 %d\n",
			w.label, named, len(w.occ), len(w.partial), w.perHop[1], w.perHop[2], w.perHop[3])
	}
	return b.String()
}

type invJoin struct {
	id    string
	hops  []string // "word:hop"
	n     int
	sum   int
	order int // degree, smaller first
}

// joins lists what two or more words reach, words held together first, then
// fewer hops in all, then the less connected (more specific) declaration.
// Words of earlier investigations in the session count too.
func (x *invIndex) joins(cur []*invWord, earlier []*invWord) []invJoin {
	all := append(append([]*invWord{}, cur...), earlier...)
	byNode := map[string]*invJoin{}
	curN := map[string]int{}
	for wi, w := range all {
		for id, h := range w.hop {
			j := byNode[id]
			if j == nil {
				j = &invJoin{id: id, order: len(x.adj[id])}
				byNode[id] = j
			}
			label := w.label
			if wi >= len(cur) {
				label += "(earlier)"
			} else {
				curN[id]++
			}
			j.hops = append(j.hops, label+":"+strconv.Itoa(h))
			j.n++
			j.sum += h
		}
	}
	var out []invJoin
	for id, j := range byNode {
		if j.n >= 2 && curN[id] >= 1 {
			sort.Strings(j.hops)
			out = append(out, *j)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].n != out[b].n {
			return out[a].n > out[b].n
		}
		if out[a].sum != out[b].sum {
			return out[a].sum < out[b].sum
		}
		if out[a].order != out[b].order {
			return out[a].order < out[b].order
		}
		return x.less(out[a].id, out[b].id)
	})
	return out
}

func (x *invIndex) joinEntries(js []invJoin) []string {
	out := make([]string, len(js))
	for i, j := range js {
		n := x.nodes[j.id]
		out[i] = fmt.Sprintf("  %s  %s:%d  [%s]\n", n.QualifiedName, n.FilePath, n.StartLine, strings.Join(j.hops, " "))
	}
	return out
}

// hopEntries lists the declarations a word reaches at hop h, with the edge
// that reached each one; those more words reach first.
func (x *invIndex) hopEntries(w *invWord, h int, reached map[string]int) []string {
	var ids []string
	for id, hh := range w.hop {
		if hh == h {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if reached[ids[i]] != reached[ids[j]] {
			return reached[ids[i]] > reached[ids[j]]
		}
		return x.less(ids[i], ids[j])
	})
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		n := x.nodes[id]
		line := fmt.Sprintf("  %s  %s:%d", n.QualifiedName, n.FilePath, n.StartLine)
		if v, ok := w.via[id]; ok {
			from := x.nodes[v.from].QualifiedName
			at := ""
			if v.edge.line > 0 {
				at = fmt.Sprintf(" (line %d)", v.edge.line)
			}
			if v.edge.out { // the earlier declaration uses this one
				line += fmt.Sprintf("  ← %s %s it%s", from, v.edge.verb(), at)
			} else { // this one uses the earlier declaration
				line += fmt.Sprintf("  → it %s %s%s", v.edge.verb(), from, at)
			}
		}
		out = append(out, line+"\n")
	}
	return out
}

// specific orders words for the layers that take one card per word in turn:
// the word in fewer declarations (the more specific) first.
func specific(ws []*invWord) []*invWord {
	out := append([]*invWord{}, ws...)
	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i].occ)+len(out[i].partial) < len(out[j].occ)+len(out[j].partial)
	})
	return out
}

// cutCard shortens a card to at most max characters, at a line boundary,
// with the handle that returns the whole card.
func cutCard(card string, max int, handle string) string {
	if len(card) <= max {
		return card
	}
	total := strings.Count(card, "\n")
	cut := strings.LastIndexByte(card[:max], '\n')
	if cut < 0 {
		cut = strings.IndexByte(card, '\n')
	}
	shown := strings.Count(card[:cut+1], "\n")
	return card[:cut+1] + fmt.Sprintf("  … card cut at %d of %d lines: open=%q\n", shown, total, handle)
}

// blocks lays the investigation out in the order the response is filled.
// With compact, cards are compact and none is longer than maxCard.
func (x *invIndex) blocks(inv *investigation, earlier []*invWord, marks map[string]string, compact bool, maxCard int) []invBlock {
	var out []invBlock
	carded := map[string]bool{}
	cardBlock := func(layer int, id, why string) {
		if carded[id] {
			return
		}
		carded[id] = true
		n := x.nodes[id]
		if prev, ok := marks[id]; ok {
			out = append(out, invBlock{layer: layer, handle: "card:" + n.QualifiedName, what: why,
				text: fmt.Sprintf("%s %s: card shown in %s\n", strings.ToLower(string(n.Type)), n.QualifiedName, prev)})
			return
		}
		text := x.card(id, inv.words, inv.withTests, compact)
		if compact {
			text = cutCard(text, maxCard, "card:"+n.QualifiedName)
		}
		out = append(out, invBlock{layer: layer, handle: "card:" + n.QualifiedName, what: why,
			text: text, cards: []string{id}})
	}
	ordered := specific(inv.words)
	// Layer 1: what each word names, one per word in turn.
	for k := 0; ; k++ {
		more := false
		for _, w := range ordered {
			if k < len(w.entities) {
				more = true
				cardBlock(1, w.entities[k], "what "+w.label+" names")
			}
		}
		if !more {
			break
		}
	}
	// Layer 2: the declarations that hold each word, and those whose name
	// holds it inside a longer name (SetX for X), one per word in turn; in
	// each word, those whose name holds a word first, then those the other
	// words come closer to, then those holding more of the words, then the
	// shorter name.
	holds := map[string]int{}
	for _, v := range inv.words {
		for _, oc := range v.occ {
			holds[oc.id]++
		}
	}
	named := func(id string) bool {
		leaf := leafName(x.nodes[id].QualifiedName)
		for _, w := range inv.words {
			if w.lit != "" && strings.Contains(strings.ToLower(leaf), strings.ToLower(w.lit)) || w.rx != nil && w.rx.MatchString(leaf) {
				return true
			}
		}
		return false
	}
	// near: how close the other words come to a declaration, summed over
	// the words other than the one whose list it is in.
	weight := [invHops + 1]float64{1, 0.5, 0.25, 0.1}
	near := func(id string, self *invWord) float64 {
		s := 0.0
		for _, v := range inv.words {
			if v == self {
				continue
			}
			if h, ok := v.hop[id]; ok {
				s += weight[h]
			}
		}
		return s
	}
	occs := make([][]invOcc, len(ordered))
	for i, w := range ordered {
		o := append([]invOcc{}, w.occ...)
		for _, p := range w.partial {
			if named(p.id) {
				o = append(o, p)
			}
		}
		sort.SliceStable(o, func(a, b int) bool {
			if na, nb := named(o[a].id), named(o[b].id); na != nb {
				return na
			}
			if ca, cb := near(o[a].id, w), near(o[b].id, w); ca != cb {
				return ca > cb
			}
			if holds[o[a].id] != holds[o[b].id] {
				return holds[o[a].id] > holds[o[b].id]
			}
			if la, lb := len(leafName(x.nodes[o[a].id].QualifiedName)), len(leafName(x.nodes[o[b].id].QualifiedName)); la != lb {
				return la < lb
			}
			return x.less(o[a].id, o[b].id)
		})
		occs[i] = o
	}
	for k := 0; ; k++ {
		more := false
		for i, w := range ordered {
			if k < len(occs[i]) {
				more = true
				cardBlock(2, occs[i][k].id, "holds "+w.label)
			}
		}
		if !more {
			break
		}
	}
	// Layer 3: what two or more words reach.
	if js := x.joins(inv.words, earlier); len(js) > 0 {
		if len(js) > invJoinCap {
			js = js[:invJoinCap]
		}
		out = append(out, invBlock{layer: 3, handle: "joins", what: "joins",
			head:    "joins: declarations reached by two or more words (word:hop)\n",
			entries: x.joinEntries(js)})
	}
	// Layer 4: the first hop of each word.
	reached := map[string]int{}
	for _, w := range inv.words {
		for id := range w.hop {
			reached[id]++
		}
	}
	for _, w := range ordered {
		l := x.hopEntries(w, 1, reached)
		if len(l) == 0 {
			continue
		}
		if len(l) > invHopListCap {
			l = l[:invHopListCap]
		}
		out = append(out, invBlock{layer: 4, handle: "hop:" + w.label + ":1", what: "hop 1 of " + w.label,
			head: fmt.Sprintf("hop 1 from %s: %d declarations\n", w.label, w.perHop[1]), entries: l})
	}
	// Layer 5: the words inside longer names.
	for _, w := range ordered {
		if len(w.partial) == 0 {
			continue
		}
		var es []string
		for _, o := range w.partial {
			n := x.nodes[o.id]
			es = append(es, fmt.Sprintf("  %s  %s lines %s\n", n.QualifiedName, n.FilePath, joinInts(o.lines)))
		}
		out = append(out, invBlock{layer: 5, handle: "partial:" + w.label, what: w.label + " inside longer names",
			head: fmt.Sprintf("%s inside longer names: %d declarations\n", w.label, len(w.partial)), entries: es})
	}
	return out
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ",")
}

// footer names what the response left out and how to open it, in at most
// room characters: the cut cards by name, then the cut lists by handle.
func footer(inv *investigation, cut []invBlock, room int) string {
	if len(cut) == 0 {
		return fmt.Sprintf("nothing left out. Hop 2 and 3 lists: investigate_open id=%s open=\"hop:<word>:2\".\n", inv.id)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "left out %d blocks (%d chars); investigate_open id=%s open=\"rest\" gives them in order", len(cut), sumLen(cut), inv.id)
	add := func(s string) bool {
		if b.Len()+len(s)+1 > room {
			return false
		}
		b.WriteString(s)
		return true
	}
	first := true
	for i, c := range cut {
		q, ok := strings.CutPrefix(c.handle, "card:")
		if !ok {
			continue
		}
		s := ", " + q
		if first {
			s = "; cards (open=\"card:<qname>\"): " + q
		}
		if !add(s) {
			add(fmt.Sprintf(" … %d more", len(cut)-i))
			break
		}
		first = false
	}
	for _, c := range cut {
		if !strings.HasPrefix(c.handle, "card:") && !add(fmt.Sprintf("; open=%q", c.handle)) {
			break
		}
	}
	b.WriteString("\n")
	if b.Len() > room {
		return clip(b.String(), max(0, room-2)) + "\n"
	}
	return b.String()
}

func sumLen(bs []invBlock) int {
	n := 0
	for _, b := range bs {
		n += b.size()
	}
	return n
}

// RegisterInvestigate wires investigate and investigate_open.
func RegisterInvestigate(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader)
	tool := nsTool("investigate",
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDescription("Work through a problem from a few words, as an engineer would: for each word, what it names (function, value, type, or a local name), "+
			"every declaration that holds it with the lines, and for each of those the lines that call, write or hold a word (not only the lines with the word); "+
			"for a value who writes it (with the conditions each write runs under) and who reads it, line by line, and for a channel who sends and who receives; for a type its fields, methods and the fields that hold it; "+
			"then what the words reach over calls and value uses up to three hops, and what two or more words reach (joins). "+
			"The whole result is kept for the session; the response holds what fits in budget characters and lists the rest with handles for investigate_open. "+
			"Cards shown earlier in the session are not repeated. Give the names you expect, joined with | (\"Flush|Writer|bufSize\"); "+
			"a wildcard between two parts (\"Read\\\\w*Buffer\") matches names of that shape."),
		mcp.WithString("words", mcp.Required(), mcp.Description("identifiers or words, separated by |, commas or spaces")),
		mcp.WithString("tests", mcp.Description("exclude (default) or include test code: the files the build recorded as test code (the build scope's file role; without a scope, the file's path), in every part of the result, callers included")),
		mcp.WithNumber("budget", mcp.Description("response size in characters (default 24000)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		x, err := invIndexFor(reader)
		if err != nil {
			return nil, err
		}
		terms := splitTerms(req.GetString("words", ""))
		if len(terms) == 0 {
			return mcp.NewToolResultText("no words: give identifiers of three or more characters, separated by |"), nil
		}
		sess := sessionFor(s)
		sess.mu.Lock()
		defer sess.mu.Unlock()
		sess.seq++
		inv := &investigation{id: "i" + strconv.Itoa(sess.seq), withTests: req.GetString("tests", testsExclude) == testsInclude}
		for _, t := range terms {
			inv.words = append(inv.words, x.resolveWord(t, inv.withTests))
		}
		var earlier []*invWord
		for _, id := range sess.order {
			earlier = append(earlier, sess.invs[id].words...)
		}
		sess.invs[inv.id] = inv
		sess.order = append(sess.order, inv.id)
		budget := clampBudget(int(req.GetFloat("budget", invDefaultBudget)))
		head := x.summary(inv)
		reserve := max(300, min(1000, budget/8))
		room := budget - len(head) - reserve
		// Full cards when everything fits; otherwise compact ones, none
		// longer than an eighth of the room, so one long card does not
		// take the place of many short ones.
		blocks := x.blocks(inv, earlier, sess.shown, false, 0)
		if sumLen(blocks) > room {
			blocks = x.blocks(inv, earlier, sess.shown, true, max(1500, room/8))
		}
		body, in, cut := fill(blocks, room)
		inv.cut = cut
		markShown(sess, inv.id, in)
		text := head + body
		text += footer(inv, cut, budget-len(text))
		traceFrom(ctx).add("investigate", invTrace(x, inv, in, cut, len(text)))
		return mcp.NewToolResultText(text), nil
	})

	open := nsTool("investigate_open",
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDescription("Return a part of an earlier investigate result: open=\"card:<name>\" (the full card of any declaration; the name is resolved as in every tool, site names \"<qname>@<file>:<line>\" included, and a local, which has no card, is answered with the card of the declaration that holds it after a line \"local: <name> is declared at <file>:<line> in <owner>\"), \"joins\", "+
			"\"hop:<word>:<1|2|3>\" (what a word reaches at that hop and through what), \"partial:<word>\", or \"rest\" (what the last response left out, in order). "+
			"Use from= to continue a long part."),
		mcp.WithString("id", mcp.Description("investigation id (default: the latest)")),
		mcp.WithString("open", mcp.Required()),
		mcp.WithNumber("from", mcp.Description("skip this many entries (from the previous response's next=)")),
		mcp.WithNumber("budget", mcp.Description("response size in characters (default 24000)")),
	)
	s.AddTool(open, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		x, err := invIndexFor(reader)
		if err != nil {
			return nil, err
		}
		sess := sessionFor(s)
		sess.mu.Lock()
		defer sess.mu.Unlock()
		id := req.GetString("id", "")
		if id == "" && len(sess.order) > 0 {
			id = sess.order[len(sess.order)-1]
		}
		inv := sess.invs[id]
		if inv == nil {
			return mcp.NewToolResultText(fmt.Sprintf("unknown investigation %q; run investigate first", id)), nil
		}
		budget := clampBudget(int(req.GetFloat("budget", invDefaultBudget)))
		text := x.open(inv, sess, req.GetString("open", ""), int(req.GetFloat("from", 0)), budget)
		traceFrom(ctx).add("investigate_open", map[string]any{"id": id, "open": req.GetString("open", ""), "chars": len(text)})
		return mcp.NewToolResultText(text), nil
	})
}

func clampBudget(b int) int {
	return max(invMinBudget, min(invMaxBudget, b))
}

func markShown(sess *invSession, invID string, in []invBlock) {
	for _, b := range in {
		for _, id := range b.cards {
			if _, ok := sess.shown[id]; !ok {
				sess.shown[id] = invID
			}
		}
	}
}

// open renders one part of an investigation, entry by entry from `from`.
func (x *invIndex) open(inv *investigation, sess *invSession, what string, from, budget int) string {
	word := func(name string) *invWord {
		for _, w := range inv.words {
			if w.label == name {
				return w
			}
		}
		return nil
	}
	reached := map[string]int{}
	for _, w := range inv.words {
		for id := range w.hop {
			reached[id]++
		}
	}
	var entries []string
	var cards []string
	switch {
	case strings.HasPrefix(what, "card:"):
		q := strings.TrimPrefix(what, "card:")
		ids, local, msg := x.cardIDs(q)
		if len(ids) == 0 {
			return msg
		}
		if local != "" {
			entries = append(entries, local)
		}
		for _, id := range ids {
			entries = append(entries, x.card(id, inv.words, inv.withTests, false))
			cards = append(cards, id)
		}
	case what == "joins":
		earlier := []*invWord{}
		for _, id := range sess.order {
			if id != inv.id {
				earlier = append(earlier, sess.invs[id].words...)
			}
		}
		entries = x.joinEntries(x.joins(inv.words, earlier))
	case strings.HasPrefix(what, "hop:"):
		parts := strings.Split(what, ":")
		h, _ := strconv.Atoi(parts[len(parts)-1])
		w := word(strings.Join(parts[1:len(parts)-1], ":"))
		if w == nil || h < 1 || h > invHops {
			return fmt.Sprintf("unknown part %q: hop:<word>:<1..%d> with a word of %s", what, invHops, inv.id)
		}
		entries = x.hopEntries(w, h, reached)
	case strings.HasPrefix(what, "partial:"):
		w := word(strings.TrimPrefix(what, "partial:"))
		if w == nil {
			return fmt.Sprintf("unknown word in %q", what)
		}
		for _, o := range w.partial {
			n := x.nodes[o.id]
			entries = append(entries, fmt.Sprintf("  %s  %s lines %s\n", n.QualifiedName, n.FilePath, joinInts(o.lines)))
		}
	case what == "rest":
		for _, bl := range inv.cut {
			if bl.head == "" {
				entries = append(entries, bl.text)
			} else {
				entries = append(entries, bl.head)
				entries = append(entries, bl.entries...)
			}
			cards = append(cards, bl.cards...)
		}
	default:
		return fmt.Sprintf("unknown part %q", what)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s: %d entries\n", inv.id, what, len(entries))
	i := max(0, from)
	for ; i < len(entries); i++ {
		if b.Len()+len(entries[i]) > budget-80 && i > from {
			break
		}
		b.WriteString(entries[i])
	}
	if i < len(entries) {
		fmt.Fprintf(&b, "next=%d (%d entries left)\n", i, len(entries)-i)
	}
	for _, id := range cards {
		if _, ok := sess.shown[id]; !ok {
			sess.shown[id] = inv.id
		}
	}
	return b.String()
}

// localKey starts the line a card: answer has when the name is a local.
const localKey = "local: "

// cardIDs resolves the name of a card: request. A node id of the index is
// that node; any other name is resolved as in every tool (store.ResolveName:
// a canonical id, a qualified name, a site name "<qname>@<file>:<line>" or a
// bare name), with the same answers when it names several declarations or
// none. The cards are those of the nodes it resolves to that have one. A
// local has none: it is answered with the card of the function or variable
// that holds it, and local is the line that says so ("local: <name> is
// declared at <file>:<line> in <owner>"). msg is the answer when there is no
// card.
func (x *invIndex) cardIDs(q string) (ids []string, local, msg string) {
	if _, ok := x.nodes[q]; ok {
		return []string{q}, "", ""
	}
	res, err := store.ResolveName(x.reader, q, "")
	switch {
	case err != nil:
		return nil, "", fmt.Sprintf("unknown declaration %q: %v", q, err)
	case res.Ambiguous():
		return nil, "", fmt.Sprintf("%q names several declarations; open one: %s", q, strings.Join(res.Candidates, ", "))
	case !res.Found():
		msg = fmt.Sprintf("unknown declaration %q", q)
		if len(res.OtherCase) > 0 {
			msg += "; other case: " + strings.Join(res.OtherCase, ", ")
		}
		return nil, "", msg
	}
	for _, n := range res.Nodes {
		if _, ok := x.nodes[n.ID]; ok {
			ids = append(ids, n.ID)
		}
	}
	if len(ids) > 0 {
		return ids, "", ""
	}
	n := res.Nodes[0]
	owner := x.innermost(n)
	if owner == "" {
		return nil, "", fmt.Sprintf("unknown declaration %q: no card holds it", q)
	}
	return []string{owner}, fmt.Sprintf("%s%s is declared at %s:%d in %s\n", localKey, q, n.FilePath, n.StartLine, x.nodes[owner].QualifiedName), ""
}

// innermost returns the function, method or variable (store.IsEncloser,
// what a local is part of) with a card and stored source whose lines hold n,
// the narrowest one; a tie goes to the smaller id. "" when none does.
func (x *invIndex) innermost(n types.Node) string {
	best, span := "", 1<<30
	for id, o := range x.nodes {
		if id == n.ID || o.FilePath != n.FilePath || n.StartLine < o.StartLine || n.EndLine > o.EndLine {
			continue
		}
		if _, src := x.declOf[id]; !src || !store.IsEncloser(o) {
			continue
		}
		s := o.EndLine - o.StartLine
		if s < span || s == span && id < best {
			best, span = id, s
		}
	}
	return best
}

func invTrace(x *invIndex, inv *investigation, in, cut []invBlock, chars int) map[string]any {
	var ws []map[string]any
	for _, w := range inv.words {
		var ents []string
		for _, id := range w.entities {
			ents = append(ents, x.nodes[id].QualifiedName)
		}
		ws = append(ws, map[string]any{"word": w.label, "entities": ents, "folded": w.folded,
			"occurrences": len(w.occ), "partial": len(w.partial), "per_hop": w.perHop[:]})
	}
	handles := func(bs []invBlock) []string {
		out := make([]string, len(bs))
		for i, b := range bs {
			out[i] = b.handle
		}
		return out
	}
	return map[string]any{"id": inv.id, "words": ws, "included": handles(in), "cut": handles(cut), "chars": chars}
}
