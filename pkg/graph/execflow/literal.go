package execflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// literal.go finds the function literals of a body that do not run where
// they are written: deferred (they run when the function returns), passed
// to a call (they run when that call calls them; a timer's callback runs
// later, in the timer's goroutine), stored in a variable or returned (they
// run when they are called). Their code is put under one step at the
// literal's line and starts with nothing held (a deferred literal, or one
// the callee calls before it returns, with what is held where it is
// written). A literal called where it is written (func(){...}()) and a
// goroutine literal (a Goroutine node) are not here.
//
// Every fact is the graph's (schema 1.45): a literal's extent is its
// FuncLit node, how it is written is the innermost statement around it
// (a defer statement, a call site, a return, a local variable), and what a
// function in the build does with it is its literal_passed_to edge (call,
// go, defer, store, pass).

// litKind is how a literal is used.
type litKind int

const (
	litDeferred litKind = iota
	litPassed
	litStored
	litReturned
	// litInPlace is a literal called where it is written; it runs in
	// sequence and is only read to tell its returns from the function's.
	litInPlace
)

// litSpan is one literal of a body: its kind, its first and last line, and
// for a stored literal the name it is stored in.
type litSpan struct {
	kind       litKind
	node       types.Node // the FuncLit
	start, end int
	storedIn   string
	// storedVar is the variable of the function the literal is kept in.
	storedVar types.Node
	// passLine is the line of the call a passed literal is an argument of.
	passLine    int
	passedTo    *types.Node
	passedWords string
	// runsLater marks a literal passed to a function outside the build
	// that runs it in another goroutine (time.AfterFunc, WaitGroup.Go): it
	// holds none of the caller's locks.
	runsLater bool
	// uses is what the function in the build does with the literal (the
	// literal_passed_to kinds), usesApprox that each depends on a branch
	// there (INFERRED).
	uses       []string
	usesApprox bool
	// handedOn, for a literal the callee passes on, is where the graph sees
	// it called (a call of a function value that reaches the literal's
	// function), and whether that is in a goroutine the callee starts.
	handedOn   []string
	handedInGo bool
}

// holds reports whether line is in the literal's body (after its first
// line, or on it for a literal that ends there).
func (l litSpan) holds(line int) bool {
	return line > l.start && line <= l.end || l.start == l.end && line == l.start
}

// passesBy reports whether step s is the call the literal is passed to: on
// a literal of one line it is written on that line but not inside it.
func (l *litSpan) passesBy(s *step) bool {
	return l.passedTo != nil && (s.kind == kindCall || s.kind == kindStd) && s.node.ID == l.passedTo.ID
}

// has reports whether the callee uses the literal in the way use.
func (l *litSpan) has(use string) bool {
	for _, u := range l.uses {
		if u == use {
			return true
		}
	}
	return false
}

// inherits reports whether the literal's code runs under the locks held
// where it is written: deferred, or passed to a function that calls or
// defers it before it returns (or hands it on to such a call).
func (l *litSpan) inherits() bool {
	switch {
	case l.kind == litDeferred:
		return true
	case l.kind != litPassed || l.runsLater:
		return false
	case len(l.uses) == 0:
		// A function outside the build: taken to call it before it returns.
		return true
	}
	return l.has(types.LiteralUseCall) || l.has(types.LiteralUseDefer) ||
		l.has(types.LiteralUsePass) && len(l.handedOn) > 0 && !l.handedInGo
}

// approxRun reports whether the literal runs under the caller's locks on
// some paths only: the callee's use depends on a branch, or it also keeps
// the literal, starts it or hands it on.
func (l *litSpan) approxRun() bool {
	if l.kind != litPassed || len(l.uses) == 0 {
		return false
	}
	if l.usesApprox {
		return true
	}
	for _, u := range l.uses {
		if u != types.LiteralUseCall && u != types.LiteralUseDefer {
			return true
		}
	}
	return false
}

// literalSpans returns the outermost function literals of body that start
// on the lines lo..hi, from the graph's FuncLit nodes, each with how it is
// used. A goroutine's own literal is not one (the goroutine is its body).
func (t *tracer) literalSpans(body types.Node, lo, hi int) []litSpan {
	owner, err := t.siteOwner(body)
	if err != nil {
		return nil
	}
	st, err := t.structureOf(owner)
	if err != nil {
		return nil
	}
	return st.literalSpans(body, lo, hi)
}

// literalSpans is the structure's half of tracer.literalSpans.
func (st *structure) literalSpans(body types.Node, lo, hi int) []litSpan {
	own, ownsLit := st.goLit[body.ID]
	var cands []types.Node
	for _, l := range st.lits {
		switch {
		case l.FilePath != body.FilePath || l.StartLine < lo || l.StartLine > hi:
			continue
		case body.Type == types.NodeGoroutine && (!ownsLit || !inside(l, own)):
			continue
		}
		cands = append(cands, l)
	}
	var out []litSpan
	for _, l := range cands {
		outer := !st.isGoLit[l.ID]
		for _, o := range cands {
			if outer && inside(l, o) {
				outer = false
			}
		}
		if outer {
			out = append(out, st.spanOf(l))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].node.StartByte < out[j].node.StartByte })
	return out
}

// spanOf says how literal l is written: in a defer statement, else by the
// innermost call site, return or local variable around it (a call site
// that starts with it calls it in place); a literal in none of them is
// stored (in a field, a map or a composite value).
func (st *structure) spanOf(l types.Node) litSpan {
	s := litSpan{node: l, start: l.StartLine, end: l.EndLine, kind: litStored}
	for _, d := range st.defers {
		if inside(l, d) {
			s.kind = litDeferred
			return s
		}
	}
	var in *types.Node
	for i := range st.holders {
		h := &st.holders[i]
		if inside(l, *h) && (in == nil || h.EndByte-h.StartByte < in.EndByte-in.StartByte) {
			in = h
		}
	}
	if in == nil {
		// x = func() {...}: the variable written on the literal's line.
		if ws := st.writes[l.StartLine]; len(ws) == 1 {
			s.storedIn, s.storedVar = ws[0].Name, ws[0]
		}
		return s
	}
	switch {
	case in.Type == types.NodeCallSite && in.SubKind == siteSend:
		s.storedIn = "a channel"
	case in.Type == types.NodeCallSite && in.StartByte == l.StartByte:
		s.kind = litInPlace
	case in.Type == types.NodeCallSite:
		s.kind, s.passLine = litPassed, in.StartLine
	case in.Type == types.NodeReturnStmt:
		s.kind = litReturned
	case isVar(*in):
		s.storedIn, s.storedVar = in.Name, *in
	}
	return s
}

// literalWords say what a literal step does and when its code runs.
func (t *tracer) literalWords(l *litSpan) string {
	switch l.kind {
	case litDeferred:
		return "defers a function literal; it runs when the function returns:"
	case litStored:
		if l.storedIn == "" {
			return "stores a function literal (in a field, a map or a composite value); it runs when it is called, which the graph does not follow (no locks assumed):"
		}
		return "stores a function literal in " + l.storedIn + "; it runs when that is called (no locks assumed):"
	case litReturned:
		return "returns a function literal; it runs when the caller calls it:"
	}
	if l.passedTo == nil {
		return "passes a function literal to a call; it runs when that call calls it:"
	}
	to := t.g.name(*l.passedTo)
	if len(l.uses) > 0 {
		ws := make([]string, 0, len(l.uses))
		for _, u := range l.uses {
			ws = append(ws, l.useWords(u, to))
		}
		w := "passes a function literal to " + to + "; " + strings.Join(ws, "; or ")
		if l.usesApprox {
			w += " ~branches"
		}
		return w + ":"
	}
	switch {
	case l.runsLater:
		return "passes a function literal to " + to + " (" + l.passedWords + "); it runs later, in another goroutine, without the caller's locks:"
	case l.passedWords != "":
		return "passes a function literal to " + to + " (" + l.passedWords + "); it runs when that calls it, under the caller's locks:"
	}
	return "passes a function literal to " + to + "; it runs when that calls it, under the caller's locks if that is before it returns:"
}

// useWords say what the function to does with a literal for one use kind.
func (l *litSpan) useWords(use, to string) string {
	switch use {
	case types.LiteralUseCall:
		return to + " calls it before it returns, under the caller's locks"
	case types.LiteralUseGo:
		return to + " starts it in a goroutine; it runs concurrently, without the caller's locks ~interleaving"
	case types.LiteralUseDefer:
		return to + " defers it; it runs when " + to + " returns, under the caller's locks"
	case types.LiteralUseSend:
		return to + " sends it on a channel; another goroutine may run it, without the caller's locks ~interleaving"
	case types.LiteralUseStore:
		return to + " keeps it; it runs when something calls it, without the caller's locks"
	case types.LiteralUsePass:
		switch {
		case len(l.handedOn) > 0 && l.handedInGo:
			return to + " hands it on; a goroutine it starts calls it at " + shortListN(l.handedOn, peerOpsShown) + ": it runs concurrently, without the caller's locks ~aliasing"
		case len(l.handedOn) > 0:
			return to + " hands it on; it is called at " + shortListN(l.handedOn, peerOpsShown) + " before " + to + " returns, under the caller's locks ~aliasing"
		}
		return to + " hands it on; the graph does not follow it further, so when it runs is not known (no locks assumed)"
	}
	return to + " " + use + "s it"
}

// nestLiterals puts the steps written in a literal of body under one
// literal step at the literal's line, which runs where it is written; the
// literal's steps run later (deferred: when the function returns). It names
// the function a passed literal goes to from the literal_passed_to edges
// (st) or the call outside the build on its line.
func (t *tracer) nestLiterals(body types.Node, st *structure, ss []*step) []*step {
	lo, hi := bodySpan(body)
	if body.Type != types.NodeGoroutine {
		lo, hi = body.StartLine, body.EndLine
	}
	spans := st.literalSpans(body, lo, hi)
	kept := spans[:0]
	for _, l := range spans {
		if l.kind != litInPlace {
			kept = append(kept, l)
		}
	}
	spans = kept
	if len(spans) == 0 {
		return ss
	}
	var out []*step
	lits := make([]*step, len(spans))
	for i := range spans {
		l := &spans[i]
		if l.kind == litPassed {
			t.namePassed(l, st, ss)
		}
	}
	skip := map[*step]bool{}
	for _, l := range spans {
		if l.kind != litPassed || l.passedTo == nil || l.passedWords == "" {
			continue
		}
		// The passing call is the literal step itself.
		for _, s := range ss {
			if s.kind == kindStd && s.line == l.passLine && s.node.ID == l.passedTo.ID {
				skip[s] = true
			}
		}
	}
	for _, s := range ss {
		if skip[s] {
			continue
		}
		k := -1
		for i, l := range spans {
			if l.holds(s.line) && (s.line != l.start || l.start == l.end && !l.passesBy(s)) {
				k = i
				break
			}
		}
		if k < 0 {
			out = append(out, s)
			continue
		}
		if lits[k] == nil {
			l := spans[k]
			lits[k] = &step{kind: kindLiteral, node: body, body: body, file: body.FilePath, line: l.start, lit: &spans[k],
				hop: s.hop, up: s.up}
			lits[k].places = st.placesOf(l.start, bodyLo(body), bodyHi(body))
			lits[k].afterReturn = st.earlyExits(l.start, bodyLo(body), bodyHi(body), st.ownerAround(l.node))
			out = append(out, lits[k])
		}
		lits[k].children = append(lits[k].children, s)
	}
	for i, lit := range lits {
		if lit == nil {
			continue
		}
		for _, c := range lit.children {
			c.places = innerPlaces(c.places, spans[i])
			c.afterReturn = returnsAfter(c.afterReturn, spans[i].start)
		}
	}
	sortSteps(t.g, out)
	return out
}

// literalLocks sets the locks a literal's steps run under: none, or for a
// literal that runs before the function or the callee returns those held
// where it is written (approximated when that is on some paths only); then
// the locks they take themselves.
func literalLocks(ss []*step) {
	for _, s := range ss {
		if s.kind != kindLiteral {
			continue
		}
		lockContext(s.children, inheritedBy(s))
		literalLocks(s.children)
	}
}

// inheritedBy is what a literal step's code holds of the locks held where
// it is written: all of them for a literal that runs before the function
// or the callee returns, none otherwise; read by line order (not the
// graph's must-hold fact) when it runs so on some paths only.
func inheritedBy(s *step) []heldLock {
	if !s.lit.inherits() {
		return nil
	}
	if !s.lit.approxRun() {
		return s.held
	}
	out := make([]heldLock, len(s.held))
	for i, h := range s.held {
		h.fact, h.byCaller = false, false
		out[i] = h
	}
	return out
}

// runLaterIDs are the functions outside the build that run a function they
// are passed in another goroutine, later, by canonical id.
var runLaterIDs = map[string]bool{"time.AfterFunc": true, "sync.(*WaitGroup).Go": true}

// runsInGoroutine reports whether n runs the function passed to it later,
// in another goroutine.
func runsInGoroutine(n types.Node) bool { return runLaterIDs[n.CanonicalID] }

// returnsAfter keeps the exits below line: a literal's code is not
// skipped by a return of the function it is written in.
func returnsAfter(rs []exit, line int) []exit {
	var out []exit
	for _, r := range rs {
		if r.line > line {
			out = append(out, r)
		}
	}
	return out
}

// innerPlaces are the places of a literal's step inside the literal.
func innerPlaces(ps []place, l litSpan) []place {
	var out []place
	for _, p := range ps {
		if p.stmt.StartLine > l.start {
			out = append(out, p)
		}
	}
	return out
}

// namePassed names the function a passed literal goes to: its
// literal_passed_to edge with what that function does with it, else a call
// outside the build on the passing call's line (with what it does,
// time.AfterFunc).
func (t *tracer) namePassed(l *litSpan, st *structure, ss []*step) {
	if e, ok := st.litUse[l.node.ID]; ok {
		if n, ok := t.g.nodes[e.Dst]; ok {
			l.passedTo = &n
			l.uses = strings.Split(e.DispatchKind, types.LiteralUseSep)
			l.usesApprox = e.Confidence == types.ConfInferred
			if l.has(types.LiteralUsePass) {
				l.handedOn, l.handedInGo = t.handedOn(n, st.owner)
			}
			return
		}
	}
	for _, s := range ss {
		if s.line == l.passLine && s.kind == kindStd {
			n := s.node
			l.passedTo, l.passedWords, l.runsLater = &n, s.std, runsInGoroutine(n)
			return
		}
	}
	for _, s := range ss {
		if s.line == l.passLine && s.kind == kindCall {
			n := s.node
			l.passedTo = &n
			return
		}
	}
}

// Dispatch kinds of a call of a function value that reaches a literal: the
// graph names the function the literal is written in.
var valueCallKinds = map[string]bool{kindClosureCall: true, kindFuncValueCall: true, types.DispatchGoroutine: true}

// handedOn is where callee, or a goroutine it starts, calls a function
// value that reaches code of fn (dispatches_to to fn: the graph names the
// function a literal is written in, so another literal of fn matches too),
// and whether every such call is in a goroutine.
func (t *tracer) handedOn(callee, fn types.Node) ([]string, bool) {
	es, err := t.g.outOf(callee.ID)
	if err != nil {
		return nil, false
	}
	inGo := map[int]bool{}
	direct := map[int]bool{}
	for _, e := range es {
		switch {
		case e.Type == types.EdgeDispatchesTo && e.Dst == fn.ID && valueCallKinds[e.DispatchKind]:
			direct[e.Line] = true
		case e.Type == types.EdgeSpawns:
			ges, err := t.g.outOf(e.Dst)
			if err != nil {
				continue
			}
			for _, g := range ges {
				if g.Type == types.EdgeDispatchesTo && g.Dst == fn.ID && valueCallKinds[g.DispatchKind] {
					inGo[g.Line] = true
				}
			}
		}
	}
	var lines []int
	allGo := len(direct)+len(inGo) > 0
	for l := range direct {
		lines = append(lines, l)
		allGo = allGo && inGo[l]
	}
	for l := range inGo {
		if !direct[l] {
			lines = append(lines, l)
		}
	}
	sort.Ints(lines)
	at := make([]string, 0, len(lines))
	for _, l := range lines {
		at = append(at, fmt.Sprintf("%s:%d", callee.FilePath, l))
	}
	return at, allGo
}

// localCalls puts, under each call through a variable of the function that
// keeps one of its literals (references read on the call's line), the
// literal's steps with the locks held at the call; a call in the literal's
// own code is recursion and is not opened again.
func (t *tracer) localCalls(st *structure, top []*step) {
	kept := map[string]*step{}
	var find func(ss []*step)
	find = func(ss []*step) {
		for _, s := range ss {
			if s.kind == kindLiteral && s.lit.storedVar.ID != "" {
				kept[s.lit.storedVar.ID] = s
			}
			find(s.children)
		}
	}
	find(top)
	if len(kept) == 0 {
		return
	}
	var visit func(ss []*step, in []*step)
	visit = func(ss []*step, in []*step) {
		for _, s := range ss {
			if s.kind == kindFuncValue {
				callLocal(st, s, kept, in)
				if s.localLit != nil {
					// Its steps are a copy of the literal's, already read.
					continue
				}
			}
			next := in
			if s.kind == kindLiteral {
				next = append(append([]*step(nil), in...), s)
			}
			visit(s.children, next)
		}
	}
	visit(top, nil)
}

// callLocal opens call s through a variable that keeps a literal step of
// kept: the literal's steps, copied, under the call.
func callLocal(st *structure, s *step, kept map[string]*step, in []*step) {
	for _, v := range st.reads[s.line] {
		l, ok := kept[v.ID]
		if !ok {
			continue
		}
		s.localLit, s.alts, s.altsLeft = l, nil, 0
		for _, a := range in {
			if a == l {
				s.recursive = true
				return
			}
		}
		s.children = append(s.children, cloneSteps(l.children, s)...)
		lockContext(s.children, s.held)
		return
	}
}

// cloneSteps copies ss and everything under them, each copy under up.
func cloneSteps(ss []*step, up *step) []*step {
	out := make([]*step, 0, len(ss))
	for _, s := range ss {
		c := *s
		c.up = up
		c.children = cloneSteps(s.children, &c)
		c.alts = cloneSteps(s.alts, &c)
		out = append(out, &c)
	}
	return out
}
