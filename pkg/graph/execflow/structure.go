package execflow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// structure.go says what a step sits in, from the graph's statement nodes
// (IfStmt, LoopStmt, SwitchStmt, ReturnStmt, contained by their function)
// and their encloses edges (kinds cond, then, else, body, case, go): a
// loop it repeats with, the branch of an if or the case of a switch it
// runs on, and an earlier return that may skip it.

// Parts of a statement, the encloses kinds.
const (
	partCond = "cond"
	partThen = "then"
	partElse = "else"
	partBody = "body"
	partCase = "case"
)

// place is one statement a line sits in, and in which part.
type place struct {
	stmt types.Node
	part string
}

// jump is a break, continue or goto statement and where it goes (its
// jumps_to edge): kind is the keyword.
type jump struct {
	node   types.Node
	kind   string
	target types.Node
}

// skips reports whether taking the jump skips code at line below it: a
// continue the rest of its loop's body, a break the rest of the statement
// it leaves, a goto the code up to its label (all of it after a goto back).
func (j jump) skips(line int) bool {
	if line <= j.node.StartLine {
		return false
	}
	switch j.kind {
	case types.JumpContinue, types.JumpBreak:
		return line <= j.target.EndLine
	case types.JumpGoto:
		return j.target.StartLine < j.node.StartLine || line < j.target.StartLine
	}
	return false
}

// exit is an earlier statement that may leave the code around a step
// before the step runs: a return, or a break, continue or goto (kind the
// keyword, the jump kinds of jumps_to).
type exit struct {
	line int
	kind string
}

// structure is the statement tree of one function: its statements, its
// function literals and the statements that run them, its defers and its
// jumps, from the graph's statement nodes.
type structure struct {
	owner types.Node
	// selectCases maps the line of each case of a select (the send or
	// receive site it encloses as cond) to the select statement.
	selectCases map[int]types.Node
	stmts       []types.Node            // if, loop and switch statements, outer first
	children    map[string][]enclosed   // statement id -> what it encloses
	clauses     map[string][]types.Node // switch or select id -> its case clauses
	returns     []types.Node            // return statements
	jumps       []jump                  // break, continue and goto statements
	parent      map[string]types.Node   // return or jump id -> the statement that encloses it
	returnTo    map[string]string       // return id -> the function or literal it returns from
	lits        []types.Node            // function literals (FuncLit), every depth
	goLit       map[string]types.Node   // goroutine id -> the literal it runs
	isGoLit     map[string]bool         // literal id -> it is a goroutine's code
	litUse      map[string]types.Edge   // literal id -> what the function it is passed to does with it
	defers      []types.Node            // defer statements
	holders     []types.Node            // call sites, local variables and returns: what a literal is written in
	// writes are the variables of the function written on each line
	// (references write), reads those read (references read).
	writes, reads map[int][]types.Node
}

// Dispatch kinds of a references edge to a variable.
const (
	refRead  = "read"
	refWrite = "write"
)

// isVar reports whether n is a variable a function can keep a literal in.
func isVar(n types.Node) bool {
	return n.Type == types.NodeLocalVariable || n.Type == types.NodeVariable
}

// enclosed is a node a statement encloses, and in which part.
type enclosed struct {
	node types.Node
	part string
}

// structureOf reads the statement tree of owner (the function whose
// contains edges hold the statements).
func (t *tracer) structureOf(owner types.Node) (*structure, error) {
	if st, ok := t.structures[owner.ID]; ok {
		return st, nil
	}
	es, err := t.g.outOf(owner.ID)
	if err != nil {
		return nil, err
	}
	st := &structure{owner: owner, children: map[string][]enclosed{}, parent: map[string]types.Node{}, selectCases: map[int]types.Node{},
		clauses: map[string][]types.Node{}, returnTo: map[string]string{}, goLit: map[string]types.Node{}, isGoLit: map[string]bool{},
		litUse: map[string]types.Edge{}}
	var ids, exits, lits []string
	var branches []types.Node
	for _, e := range es {
		if e.Type != types.EdgeContains {
			continue
		}
		n, ok := t.g.nodes[e.Dst]
		if !ok {
			continue
		}
		switch n.Type {
		case types.NodeCallSite, types.NodeLocalVariable:
			st.holders = append(st.holders, n)
		case types.NodeIfStmt, types.NodeLoopStmt, types.NodeSwitchStmt:
			st.stmts = append(st.stmts, n)
			ids = append(ids, n.ID)
		case types.NodeGoroutine:
			ids = append(ids, n.ID)
		case types.NodeReturnStmt:
			st.returns = append(st.returns, n)
			st.holders = append(st.holders, n)
			exits = append(exits, n.ID)
		case types.NodeBranchStmt:
			branches = append(branches, n)
			exits = append(exits, n.ID)
		case types.NodeFuncLit:
			st.lits = append(st.lits, n)
			lits = append(lits, n.ID)
		case types.NodeDeferStmt:
			st.defers = append(st.defers, n)
		}
	}
	encl, err := t.g.batchOut(ids, types.EdgeEncloses)
	if err != nil {
		return nil, err
	}
	if err := t.readJumps(st, exits, branches); err != nil {
		return nil, err
	}
	defs, err := t.g.batchOut([]string{owner.ID}, types.EdgeDefines)
	if err != nil {
		return nil, err
	}
	for _, e := range defs {
		if n, ok := t.g.nodes[e.Dst]; ok && isVar(n) {
			st.holders = append(st.holders, n)
		}
	}
	st.writes, st.reads = map[int][]types.Node{}, map[int][]types.Node{}
	for _, e := range es {
		n, ok := t.g.nodes[e.Dst]
		if e.Type != types.EdgeReferences || !ok || !isVar(n) {
			continue
		}
		switch e.DispatchKind {
		case refWrite:
			st.writes[e.Line] = append(st.writes[e.Line], n)
		case refRead:
			st.reads[e.Line] = append(st.reads[e.Line], n)
		}
	}
	uses, err := t.g.batchOut(lits, types.EdgeLiteralPassedTo)
	if err != nil {
		return nil, err
	}
	for _, e := range uses {
		if _, ok := st.litUse[e.Src]; !ok {
			st.litUse[e.Src] = e
		}
	}
	for _, e := range encl {
		n, ok := t.g.nodes[e.Dst]
		src := t.g.nodes[e.Src]
		switch {
		case !ok:
			continue
		case src.Type == types.NodeGoroutine:
			// The goroutine's own literal is the first one on its line.
			if had, has := st.goLit[src.ID]; n.Type == types.NodeFuncLit && n.StartLine == src.StartLine && (!has || n.StartByte < had.StartByte) {
				delete(st.isGoLit, had.ID)
				st.goLit[src.ID] = n
				st.isGoLit[n.ID] = true
			}
			continue
		case e.DispatchKind == partClause:
			st.clauses[e.Src] = append(st.clauses[e.Src], n)
			continue
		}
		st.children[e.Src] = append(st.children[e.Src], enclosed{node: n, part: e.DispatchKind})
		if isSelectStmt(src) && e.DispatchKind == partCond && n.Type == types.NodeCallSite &&
			(n.SubKind == siteSend || n.SubKind == siteRecv) {
			st.selectCases[n.StartLine] = src
		}
		if n.Type == types.NodeReturnStmt || n.Type == types.NodeBranchStmt {
			st.parent[n.ID] = src
		}
	}
	sort.SliceStable(st.stmts, func(i, j int) bool {
		a, b := st.stmts[i], st.stmts[j]
		if sa, sb := a.EndLine-a.StartLine, b.EndLine-b.StartLine; sa != sb {
			return sa > sb
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		return a.ID < b.ID
	})
	t.structures[owner.ID] = st
	return st, nil
}

// readJumps reads where each return and jump goes (jumps_to): the
// function or literal a return leaves, the statement a break, continue or
// goto goes to.
func (t *tracer) readJumps(st *structure, exits []string, branches []types.Node) error {
	es, err := t.g.batchOut(exits, types.EdgeJumpsTo)
	if err != nil {
		return err
	}
	to := map[string]types.Edge{}
	for _, e := range es {
		if e.DispatchKind == types.JumpReturn {
			st.returnTo[e.Src] = e.Dst
			continue
		}
		if _, ok := to[e.Src]; !ok {
			to[e.Src] = e
		}
	}
	for _, b := range branches {
		e, ok := to[b.ID]
		target, found := t.g.nodes[e.Dst]
		if !ok || !found {
			continue
		}
		st.jumps = append(st.jumps, jump{node: b, kind: e.DispatchKind, target: target})
	}
	return nil
}

// partClause is the encloses kind from a switch or select to each of its
// case clauses.
const partClause = "clause"

// clausePart is the part of a switch or select that case clause c is.
func clausePart(c types.Node) string { return partCase + " " + c.ID }

// ownerAt is the id of the code a line's statements return from: the
// innermost function literal around line, else the function.
func (st *structure) ownerAt(line int) string {
	best := st.owner.ID
	size := -1
	for _, l := range st.lits {
		if !litHolds(l, line) {
			continue
		}
		if s := l.EndByte - l.StartByte; size < 0 || s < size {
			best, size = l.ID, s
		}
	}
	return best
}

// ownerAround is the id of the code literal l is written in: the
// innermost other literal around it, else the function.
func (st *structure) ownerAround(l types.Node) string {
	best := st.owner.ID
	size := -1
	for _, o := range st.lits {
		if o.ID == l.ID || !inside(l, o) {
			continue
		}
		if s := o.EndByte - o.StartByte; size < 0 || s < size {
			best, size = o.ID, s
		}
	}
	return best
}

// litHolds reports whether line is in the body of literal l: after its
// first line, or on it for a literal that ends there.
func litHolds(l types.Node, line int) bool {
	return line > l.StartLine && line <= l.EndLine || l.StartLine == l.EndLine && line == l.StartLine
}

// inside reports whether node a lies within node b's extent (one file,
// by byte offsets), a different node.
func inside(a, b types.Node) bool {
	return a.ID != b.ID && a.FilePath == b.FilePath && b.StartByte <= a.StartByte && a.EndByte <= b.EndByte
}

// returnOwner is the id of the function or literal return r leaves.
func (st *structure) returnOwner(r types.Node) string {
	if id, ok := st.returnTo[r.ID]; ok {
		return id
	}
	return st.ownerAt(r.StartLine)
}

// deferredAt reports whether the statement on line is deferred: a defer
// statement holds it, in the same function or literal.
func (st *structure) deferredAt(line int) bool {
	for _, d := range st.defers {
		if line >= d.StartLine && line <= d.EndLine && st.ownerAt(line) == st.ownerAt(d.StartLine) {
			return true
		}
	}
	return false
}

// isSelectStmt reports whether n is a select statement (a SwitchStmt of
// sub kind select or select_default).
func isSelectStmt(n types.Node) bool {
	return n.Type == types.NodeSwitchStmt && (n.SubKind == types.SubKindSelect || n.SubKind == types.SubKindSelectDefault)
}

// placesOf returns the statements line sits in, outer first, within the
// lines lo..hi of the body (a goroutine literal's own lines, or the whole
// function), with the part of each. The condition of an if or a switch
// runs before it branches, so a line there is not in it; a loop's header
// runs on every iteration, so a line there is in the loop.
func (st *structure) placesOf(line, lo, hi int) []place {
	var out []place
	for _, s := range st.stmts {
		if line < s.StartLine || line > s.EndLine || s.StartLine < lo || s.EndLine > hi {
			continue
		}
		part := st.partOf(s, line)
		if part == partCond && (s.Type != types.NodeLoopStmt || s.SubKind == loopRange && line == s.StartLine) {
			// An if or switch condition runs before it branches, a range
			// expression once before the loop; a for loop's header runs on
			// every iteration.
			continue
		}
		out = append(out, place{stmt: s, part: part})
	}
	return out
}

// partOf is the part of s that line is in: the part of the node s encloses
// around line; for an if without one, else when an else part starts above
// line; otherwise cond on the statement's own line and body below it.
func (st *structure) partOf(s types.Node, line int) string {
	for _, c := range st.children[s.ID] {
		if c.part == partCond && line >= c.node.StartLine && line <= c.node.EndLine {
			return partCond
		}
	}
	for _, c := range st.clauses[s.ID] {
		switch {
		case line == c.StartLine && !isSelectStmt(s):
			// A switch's case expressions are evaluated in turn before a
			// clause is chosen.
			return partCond
		case line >= c.StartLine && line <= c.EndLine:
			// Each case clause is its own part: two clauses are two
			// branches.
			return clausePart(c)
		}
	}
	elseFrom := 0
	for _, c := range st.children[s.ID] {
		if line >= c.node.StartLine && line <= c.node.EndLine {
			return c.part
		}
		if c.part == partElse && (elseFrom == 0 || c.node.StartLine < elseFrom) {
			elseFrom = c.node.StartLine
		}
	}
	switch {
	case line == s.StartLine:
		return partCond
	case s.Type == types.NodeIfStmt && elseFrom > 0 && line >= elseFrom:
		return partElse
	case s.Type == types.NodeIfStmt:
		return partThen
	}
	return partBody
}

// earlyExits are the returns and jumps above line that may skip it, in
// line order: each in the code own (the function or literal line is in),
// on a branch that does not hold line and on the path to it (the innermost
// statement around both has them in the same part, so a return in the
// then of an if does not skip its else, nor one in a case clause another
// clause). A jump skips only what it leaves (jump.skips).
func (st *structure) earlyExits(line, lo, hi int, own string) []exit {
	var out []exit
	seen := map[exit]bool{}
	lp := st.placesOf(line, lo, hi)
	if sel, ok := st.selectCases[line]; ok {
		// A select case's operation is printed outside the select, but it
		// runs only in its clause: a return in another clause does not
		// skip it.
		for _, c := range st.clauses[sel.ID] {
			if line >= c.StartLine && line <= c.EndLine {
				lp = append(lp, place{stmt: sel, part: clausePart(c)})
			}
		}
	}
	onPath := func(n types.Node) bool {
		if n.StartLine >= line || n.StartLine < lo || n.EndLine > hi {
			return false
		}
		p, ok := st.parent[n.ID]
		// A statement right in the function's body, or in the part of the
		// statement that holds line, has nothing after it on its path:
		// line is then dead code or on another branch.
		if !ok || line >= p.StartLine && line <= p.EndLine {
			return false
		}
		return !st.otherBranch(st.placesOf(n.StartLine, lo, hi), lp)
	}
	add := func(x exit) {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	for _, r := range st.returns {
		if st.returnOwner(r) == own && onPath(r) {
			add(exit{line: r.StartLine, kind: types.JumpReturn})
		}
	}
	for _, j := range st.jumps {
		if j.skips(line) && st.ownerAt(j.node.StartLine) == own && onPath(j.node) {
			add(exit{line: j.node.StartLine, kind: j.kind})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].line != out[j].line {
			return out[i].line < out[j].line
		}
		return out[i].kind < out[j].kind
	})
	return out
}

// otherBranch reports whether the innermost statement around two lines
// (their places pa and pb) has them in different parts (the then and the
// else of an if, two case clauses of a switch or a select): code at one
// does not run before code at the other.
func (st *structure) otherBranch(pa, pb []place) bool {
	k := -1
	for i := 0; i < len(pa) && i < len(pb) && pa[i].stmt.ID == pb[i].stmt.ID; i++ {
		k = i
	}
	if k < 0 {
		return false
	}
	return pa[k].part != pb[k].part
}

// returningBranch is the last line of the innermost if or switch around
// line whose part holding line also returns (if !ok { mu.Unlock(); return
// }): what runs there ends before the code after the statement; 0 when
// there is none.
func (st *structure) returningBranch(line, lo, hi int) int {
	ps := st.placesOf(line, lo, hi)
	own := st.ownerAt(line)
	for i := len(ps) - 1; i >= 0; i-- {
		p := ps[i]
		if p.stmt.Type == types.NodeLoopStmt {
			return 0
		}
		for _, r := range st.returns {
			if st.returnOwner(r) != own {
				// A return in a function literal ends the literal.
				continue
			}
			if r.StartLine > line && r.StartLine <= p.stmt.EndLine && st.parent[r.ID].ID == p.stmt.ID && st.partOf(p.stmt, r.StartLine) == p.part {
				return p.stmt.EndLine
			}
		}
	}
	return 0
}

// innerLoop is the innermost loop among places, or false.
func innerLoop(ps []place) (types.Node, bool) {
	for i := len(ps) - 1; i >= 0; i-- {
		if ps[i].stmt.Type == types.NodeLoopStmt {
			return ps[i].stmt, true
		}
	}
	return types.Node{}, false
}

// rangeHeadAt reports whether line is the head of a range loop: a receive
// there handles every message until the channel is closed.
func (st *structure) rangeHeadAt(line int) bool {
	for _, s := range st.stmts {
		if s.Type == types.NodeLoopStmt && s.SubKind == loopRange && s.StartLine == line {
			return true
		}
	}
	return false
}

// loopRange is the sub kind of a range loop: its range expression is
// evaluated once, before the first iteration.
const loopRange = "range"

// returnsBefore keeps the exits above line.
func returnsBefore(rs []exit, line int) []exit {
	var out []exit
	for _, r := range rs {
		if r.line < line {
			out = append(out, r)
		}
	}
	return out
}

// bodyLo and bodyHi are the lines of bodySpan.
func bodyLo(body types.Node) int { lo, _ := bodySpan(body); return lo }

// bodyHi is the last line of bodySpan.
func bodyHi(body types.Node) int { _, hi := bodySpan(body); return hi }

// placeWords is the marker of where a step sits, outer first: "for 73",
// "if 92" (then), "else 83", "case 40", "after return 97", "after
// continue 88", "after break 90", "after goto 95", "defer".
func placeWords(ps []place, rets []exit, deferred bool) string {
	var parts []string
	for _, p := range ps {
		parts = append(parts, placeWord(p))
	}
	for _, r := range rets {
		// One marker per exit: each says the step runs only if that
		// return or jump is not taken.
		parts = append(parts, "after "+r.kind+" "+strconv.Itoa(r.line))
	}
	if deferred {
		parts = append(parts, "defer")
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// placeWord names one place.
func placeWord(p place) string {
	switch {
	case p.stmt.Type == types.NodeLoopStmt:
		return fmt.Sprintf("for %d", p.stmt.StartLine)
	case p.part == partElse:
		return fmt.Sprintf("else %d", p.stmt.StartLine)
	case isSelectStmt(p.stmt):
		return fmt.Sprintf("select %d", p.stmt.StartLine)
	case p.stmt.Type == types.NodeSwitchStmt:
		return fmt.Sprintf("case %d", p.stmt.StartLine)
	}
	return fmt.Sprintf("if %d", p.stmt.StartLine)
}

// bodySpan is the lines whose statements belong to body: a goroutine's own
// lines, or every line of a function.
func bodySpan(body types.Node) (int, int) {
	if body.Type == types.NodeGoroutine {
		return body.StartLine + 1, body.EndLine
	}
	return 0, int(^uint(0) >> 1)
}

// placeSteps sets on each step of body where it sits.
func (t *tracer) placeSteps(body types.Node, ss []*step) (*structure, error) {
	owner, err := t.siteOwner(body)
	if err != nil {
		return nil, err
	}
	st, err := t.structureOf(owner)
	if err != nil {
		return nil, err
	}
	lo, hi := bodySpan(body)
	for _, s := range ss {
		s.places = st.placesOf(s.line, lo, hi)
		s.afterReturn = st.earlyExits(s.line, lo, hi, st.ownerAt(s.line))
		s.rangeHead = st.rangeHeadAt(s.line)
		s.inDefer = s.kind != kindUnlock && st.deferredAt(s.line)
		if s.kind == kindUnlock || s.kind == kindLock {
			s.releaseUntil = st.returningBranch(s.line, lo, hi)
		}
	}
	return st, nil
}

// groupBranches shows the steps of the then and the else of one if as
// alternatives, not a sequence: a run of steps inside an if that has steps
// in both becomes one branch step whose alternatives are its then and its
// else, each with its steps, grouped again inside. The if leaves the
// places of those steps.
func (t *tracer) groupBranches(ss []*step) []*step {
	both := map[string]bool{}
	seen := map[string]map[string]bool{}
	for _, s := range ss {
		for _, p := range s.places {
			if p.stmt.Type != types.NodeIfStmt {
				continue
			}
			if seen[p.stmt.ID] == nil {
				seen[p.stmt.ID] = map[string]bool{}
			}
			seen[p.stmt.ID][p.part] = true
			if seen[p.stmt.ID][partThen] && seen[p.stmt.ID][partElse] {
				both[p.stmt.ID] = true
			}
		}
	}
	var out []*step
	for i := 0; i < len(ss); {
		s := ss[i]
		s.children = t.groupBranches(s.children)
		for _, a := range s.alts {
			a.children = t.groupBranches(a.children)
		}
		at := -1
		for k, p := range s.places {
			if both[p.stmt.ID] {
				at = k
				break
			}
		}
		if at < 0 {
			out = append(out, s)
			i++
			continue
		}
		stmt := s.places[at].stmt
		var then, els []*step
		for ; i < len(ss); i++ {
			k := placeIndex(ss[i].places, stmt.ID)
			if k < 0 {
				break
			}
			part := ss[i].places[k].part
			ss[i].places = append(append([]place(nil), ss[i].places[:k]...), ss[i].places[k+1:]...)
			if part == partElse {
				els = append(els, ss[i])
			} else {
				then = append(then, ss[i])
			}
		}
		out = append(out, t.branchStep(s, stmt, then, els))
	}
	return out
}

// placeIndex is the index of the place of statement id, or -1.
func placeIndex(ps []place, id string) int {
	for i, p := range ps {
		if p.stmt.ID == id {
			return i
		}
	}
	return -1
}

// branchStep is the step of an if whose then and else both have steps.
func (t *tracer) branchStep(first *step, stmt types.Node, then, els []*step) *step {
	b := &step{kind: kindBranch, node: stmt, body: first.body, file: first.file, line: stmt.StartLine,
		places: outerPlaces(first.places, stmt), afterReturn: returnsBefore(first.afterReturn, stmt.StartLine), hop: first.hop, up: first.up, held: first.held}
	for _, part := range []struct {
		name  string
		steps []*step
	}{{partThen, then}, {partElse, els}} {
		if len(part.steps) == 0 {
			continue
		}
		a := &step{kind: kindBranchPart, node: stmt, body: first.body, file: first.file, line: part.steps[0].line,
			partName: part.name, hop: first.hop, up: first.up, children: t.groupBranches(part.steps)}
		b.alts = append(b.alts, a)
	}
	t.branchPoints++
	return b
}

// outerPlaces are the places of ps outside stmt (ps no longer holds stmt).
func outerPlaces(ps []place, stmt types.Node) []place {
	var out []place
	for _, p := range ps {
		if p.stmt.StartLine <= stmt.StartLine && p.stmt.EndLine >= stmt.EndLine && p.stmt.ID != stmt.ID {
			out = append(out, p)
		}
	}
	return out
}
