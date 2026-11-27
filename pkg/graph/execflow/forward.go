package execflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// forward.go reads a body's steps and opens them: a call into the callee's
// steps, a goroutine start into the goroutine's, a send into the receives
// it can wake and the code each runs.

// isDeferred reports whether the statement on line of body is deferred:
// it runs when the function returns (a DeferStmt holds it).
func (t *tracer) isDeferred(body types.Node, line int) bool {
	st, ok := t.selectFacts(body)
	return ok && st.deferredAt(line)
}

// isSelectCase reports whether the channel operation on line of body is a
// case of a select (the select encloses its site as cond).
func (t *tracer) isSelectCase(body types.Node, line int) bool {
	st, ok := t.selectFacts(body)
	if !ok {
		return false
	}
	_, is := st.selectCases[line]
	return is
}

// selectFacts is the statement tree of body's function.
func (t *tracer) selectFacts(body types.Node) (*structure, bool) {
	owner, err := t.siteOwner(body)
	if err != nil {
		return nil, false
	}
	st, err := t.structureOf(owner)
	return st, err == nil
}

// signalType marks a channel whose messages carry no value.
const signalType = "struct{}"

// Dispatch kinds of the edges a body's steps are read from.
const (
	kindInterfaceMethod = "interface_method"
	kindTypeHierarchy   = "type_hierarchy"
)

// chanOp is one send or receive on a line: the channel's origin (a make
// site, a field, a variable), the operation's CallSite and the certainty of
// the origin.
type chanOp struct {
	ch        types.Node
	site      *types.Node
	certainty concfacts.Certainty
	found     bool
}

// bodyFacts are a body's edges grouped by what they say.
type bodyFacts struct {
	steps    []*step
	calls    []types.Edge
	invokes  []types.Edge
	dispatch map[int][]types.Edge
	sends    map[int]*chanOp
	recvs    map[int]*chanOp
	closes   map[int]*chanOp
	triggers map[int][]types.Edge
	locked   map[int]bool
}

// bodySteps returns the steps of body in execution order, nested (handled
// code under its receive, select cases as alternatives), each at hop with
// the locks it runs under (inherited from the step that opened body, then
// taken in body). up is the step that opened body (nil for the seed).
func (t *tracer) bodySteps(body types.Node, inherited []heldLock, hop int, up *step) ([]*step, error) {
	es, err := t.g.outOf(body.ID)
	if err != nil {
		return nil, err
	}
	f, err := t.gather(body, es, hop, up)
	if err != nil {
		return nil, err
	}
	if err := t.callSteps(body, f, hop, up); err != nil {
		return nil, err
	}
	if t.q.follows(LinkMessages) {
		if err := t.messageSteps(body, f, hop, up); err != nil {
			return nil, err
		}
	}
	sortSteps(t.g, f.steps)
	st, err := t.placeSteps(body, f.steps)
	if err != nil {
		return nil, err
	}
	top := t.nestLiterals(body, st, f.steps)
	lockContext(top, inherited)
	literalLocks(top)
	if err := t.siteLocks(body, top, inherited); err != nil {
		return nil, err
	}
	altLocks(top)
	t.localCalls(st, top)
	nested, err := t.nestHandled(top)
	if err != nil {
		return nil, err
	}
	if err := t.nestHandledInLiterals(nested); err != nil {
		return nil, err
	}
	if _, facts := t.selectFacts(body); facts {
		return t.groupBranches(t.groupSelectsByFact(body, st, t.caseOperandsDeep(body, nested))), nil
	}
	return t.groupBranches(t.groupSelects(body, nested)), nil
}

// gather sorts the edges of body into facts, and makes the steps that need
// no other edge: goroutine starts, locks and, with a focus, reads and
// writes.
func (t *tracer) gather(body types.Node, es []types.Edge, hop int, up *step) (*bodyFacts, error) {
	f := &bodyFacts{dispatch: map[int][]types.Edge{}, sends: map[int]*chanOp{}, recvs: map[int]*chanOp{}, closes: map[int]*chanOp{},
		triggers: map[int][]types.Edge{}, locked: map[int]bool{}}
	owner, err := t.siteOwner(body)
	if err != nil {
		return nil, err
	}
	spans, err := t.directGoroutines(body, owner)
	if err != nil {
		return nil, err
	}
	direct := map[string]bool{}
	for _, g := range spans {
		direct[g.ID] = true
	}
	targets, err := t.goTargets(append(spans, body))
	if err != nil {
		return nil, err
	}
	lineOf := t.lineRule(body, spans, targets)
	es, err = t.withOwnerEdges(body, owner, es, direct)
	if err != nil {
		return nil, err
	}
	for _, e := range es {
		if e.Type == types.EdgeSpawns && !direct[e.Dst] {
			continue
		}
		line, ok := lineOf(e)
		if !ok {
			continue
		}
		dst, found, err := t.g.node(e.Dst)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		switch e.Type {
		case types.EdgeSpawns:
			if t.q.follows(LinkGoroutines) {
				g := dst
				f.steps = append(f.steps, &step{kind: kindGo, node: dst, body: body, file: dst.FilePath, line: dst.StartLine,
					certainty: concfacts.Approx(concfacts.ReasonInterleaving), open: &g, hop: hop, up: up})
			}
		case types.EdgeAcquiresLock, types.EdgeReleasesLock:
			f.locked[line] = true
			k := kindLock
			if e.Type == types.EdgeReleasesLock {
				k = kindUnlock
			}
			s := &step{kind: k, node: dst, body: body, file: body.FilePath, line: line, certainty: certaintyOf(e), hop: hop, up: up}
			s.deferred = k == kindUnlock && t.isDeferred(body, line)
			if k == kindLock {
				if err := t.releasedElsewhere(s, body, es); err != nil {
					return nil, err
				}
			}
			f.steps = append(f.steps, s)
		case types.EdgeCalls:
			f.calls = append(f.calls, withLine(e, line))
		case types.EdgeInvokes:
			f.invokes = append(f.invokes, withLine(e, line))
		case types.EdgeDispatchesTo:
			f.dispatch[line] = append(f.dispatch[line], withLine(e, line))
		case types.EdgeSendsTo, types.EdgeRecvsFrom, types.EdgeCloses:
			ops := f.sends
			switch e.Type {
			case types.EdgeRecvsFrom:
				ops = f.recvs
			case types.EdgeCloses:
				ops = f.closes
			}
			addChanOp(ops, line, e, dst)
		case types.EdgeTriggers:
			f.triggers[line] = append(f.triggers[line], e)
		case types.EdgeReadsField, types.EdgeWritesField, types.EdgeReferences:
			if s := t.accessStep(body, e, dst, line, hop, up); s != nil {
				f.steps = append(f.steps, s)
			}
		}
	}
	if err := t.fillSites(owner, f); err != nil {
		return nil, err
	}
	return f, nil
}

// lineRule returns the rule for which edges of body are its steps, and at
// what line. A goroutine's are those on the lines of its node (a literal's
// body, or the go line of a named function, which its line-0 code edges
// name); a function leaves the lines of the goroutine literals it starts to
// them, and keeps its spawns.
func (t *tracer) lineRule(body types.Node, spans []types.Node, targets map[string]string) func(types.Edge) (int, bool) {
	if body.Type == types.NodeGoroutine {
		own := targets[body.ID]
		return func(e types.Edge) (int, bool) {
			if e.Type == types.EdgeSpawns {
				return e.Line, true
			}
			if e.Line == 0 {
				return body.StartLine, true
			}
			if own != "" && e.Dst != own && isCallEdge(e) {
				// go f(x): only f runs in the goroutine; its receiver
				// and arguments are the starter's.
				return 0, false
			}
			if inner, ok := innerGo(e, spans, targets); ok {
				return e.Line, inner
			}
			return e.Line, e.Line >= body.StartLine && e.Line <= body.EndLine
		}
	}
	return func(e types.Edge) (int, bool) {
		if e.Type == types.EdgeSpawns {
			return e.Line, true
		}
		if inner, ok := innerGo(e, spans, targets); ok {
			return e.Line, inner
		}
		return e.Line, true
	}
}

// innerGo decides an edge on the lines of a goroutine the body starts:
// ok is true when it is on them, and keep says whether the body keeps it
// (the receiver and arguments of go f(x) are evaluated by the starter,
// under its locks; the rest is the goroutine's).
func innerGo(e types.Edge, spans []types.Node, targets map[string]string) (keep, ok bool) {
	for _, g := range spans {
		if e.Line < g.StartLine || e.Line > g.EndLine {
			continue
		}
		tg := targets[g.ID]
		return tg != "" && e.Dst != tg && (isCallEdge(e) || e.Type == types.EdgeAcquiresLock || e.Type == types.EdgeReleasesLock), true
	}
	return false, false
}

// isCallEdge reports whether e is a call: calls, invokes or dispatches_to.
func isCallEdge(e types.Edge) bool {
	return e.Type == types.EdgeCalls || e.Type == types.EdgeInvokes || e.Type == types.EdgeDispatchesTo
}

// goTargets maps each goroutine of gs started with a named function or
// method (go f(x)) to the node it calls (its site_calls edge).
func (t *tracer) goTargets(gs []types.Node) (map[string]string, error) {
	out := map[string]string{}
	for _, g := range gs {
		if g.Type != types.NodeGoroutine {
			continue
		}
		es, err := t.g.outOf(g.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range es {
			if e.Type == types.EdgeSiteCalls {
				out[g.ID] = e.Dst
				break
			}
		}
	}
	return out, nil
}

// syncStep is the step of a call outside the build that the graph says
// operates on a sync primitive (operates_on): what it does to which
// primitive, the other sites of a WaitGroup it waits for, and what it
// makes run later (runs_func); nil when the graph says nothing of it.
func (t *tracer) syncStep(body, dst types.Node, e types.Edge, hop int, up *step) (*step, error) {
	owner, err := t.siteOwner(body)
	if err != nil {
		return nil, err
	}
	sf, err := t.siteFactsOf(owner)
	if err != nil {
		return nil, err
	}
	site, ok := sf.opSiteAt(e.Line, dst)
	if !ok {
		if words, unknown := t.unknownOp(sf, e.Line, dst); unknown {
			return &step{kind: kindStd, node: dst, body: body, file: body.FilePath, line: e.Line, std: words,
				certainty: approxAliasing, hop: hop, up: up}, nil
		}
		return nil, nil
	}
	ops := sf.syncOps(site)
	words := t.opWords(ops)
	if words == "" {
		return nil, nil
	}
	s := &step{kind: kindStd, node: dst, body: body, file: body.FilePath, line: e.Line, std: words, certainty: certaintyOf(e), hop: hop, up: up}
	if err := t.peerOps(s, ops); err != nil {
		return nil, err
	}
	t.laterSteps(sf, s, site)
	return s, nil
}

// directGoroutines are the goroutines body starts itself: those its
// function (owner) starts inside body's lines (a goroutine's own lines,
// past its go line), and not inside another of them.
func (t *tracer) directGoroutines(body, owner types.Node) ([]types.Node, error) {
	es, err := t.g.outOf(owner.ID)
	if err != nil {
		return nil, err
	}
	var all []types.Node
	for _, e := range es {
		if e.Type != types.EdgeSpawns || e.Dst == body.ID {
			continue
		}
		g, ok, err := t.g.node(e.Dst)
		if err != nil {
			return nil, err
		}
		if ok && (body.Type != types.NodeGoroutine || g.StartLine > body.StartLine && g.EndLine <= body.EndLine) {
			all = append(all, g)
		}
	}
	var out []types.Node
	for _, g := range all {
		inner := false
		for _, o := range all {
			if o.ID != g.ID && o.StartLine <= g.StartLine && o.EndLine >= g.EndLine && (o.StartLine < g.StartLine || o.EndLine > g.EndLine) {
				inner = true
				break
			}
		}
		if !inner {
			out = append(out, g)
		}
	}
	return out, nil
}

// withOwnerEdges adds to a goroutine's edges those its function records
// for it: the locks taken and released in its literal and the goroutines
// it starts (the graph records them on the function around the literal).
func (t *tracer) withOwnerEdges(body, owner types.Node, es []types.Edge, direct map[string]bool) ([]types.Edge, error) {
	if owner.ID == body.ID {
		return es, nil
	}
	oes, err := t.g.outOf(owner.ID)
	if err != nil {
		return nil, err
	}
	out := append([]types.Edge(nil), es...)
	have := map[string]bool{}
	for _, e := range es {
		have[edgeAt(e)] = true
	}
	inBody := e2InBody(body)
	for _, e := range oes {
		switch {
		case e.Type == types.EdgeSpawns && direct[e.Dst]:
		case !inBody(e.Line):
			continue
		case e.Type == types.EdgeAcquiresLock || e.Type == types.EdgeReleasesLock:
		case e.Type == types.EdgeSendsTo || e.Type == types.EdgeRecvsFrom || e.Type == types.EdgeCloses:
			// The operation's site, and a channel the value flow found
			// no origin for (ctx.Done(), a timer's C), are recorded on the
			// function around the literal.
		case e.Type == types.EdgeCalls && outside(t.g.nodes[e.Dst]):
			// Calls outside the build (wg.Done, timer.Stop) have no
			// goroutine code edge.
		default:
			continue
		}
		if have[edgeAt(e)] {
			continue
		}
		have[edgeAt(e)] = true
		out = append(out, e)
	}
	sortEdges(out)
	return out, nil
}

// edgeAt keys an edge by type, far end and line.
func edgeAt(e types.Edge) string { return fmt.Sprintf("%s|%s|%d", e.Type, e.Dst, e.Line) }

// e2InBody reports whether a line is a goroutine's own (past its go line).
func e2InBody(body types.Node) func(int) bool {
	return func(line int) bool { return line > body.StartLine && line <= body.EndLine }
}

// withLine is e at line.
func withLine(e types.Edge, line int) types.Edge {
	e.Line = line
	return e
}

// addChanOp records a send or receive edge: an edge to a CallSite names
// the operation's site, any other its channel; the channel of an edge with
// a dispatch kind (the origin the value flow found) wins.
func addChanOp(ops map[int]*chanOp, line int, e types.Edge, dst types.Node) {
	op := ops[line]
	if op == nil {
		op = &chanOp{}
		ops[line] = op
	}
	if dst.Type == types.NodeCallSite {
		d := dst
		op.site = &d
		return
	}
	if !op.found || e.DispatchKind != "" {
		op.ch, op.certainty, op.found = dst, certaintyOf(e), true
	}
}

// siteOwner is the function whose edges name the CallSites of body's
// channel operations: body itself, or for a goroutine the function that
// starts it (a literal's operations are recorded on that function).
func (t *tracer) siteOwner(body types.Node) (types.Node, error) {
	if body.Type != types.NodeGoroutine {
		return body, nil
	}
	es, err := t.g.inOf(body.ID, types.EdgeSpawns)
	if err != nil {
		return types.Node{}, err
	}
	for _, e := range es {
		if n, ok, err := t.g.node(e.Src); err != nil || ok {
			return n, err
		}
	}
	return body, nil
}

// CallSite sub kinds of channel operations.
const (
	siteSend = "send"
	siteRecv = "recv"
)

// fillSites names the CallSite of each send and receive that has none: the
// CallSite of the operation's kind on its line among those owner contains
// (a goroutine literal's sites are contained by the function around it).
func (t *tracer) fillSites(owner types.Node, f *bodyFacts) error {
	es, err := t.g.outOf(owner.ID)
	if err != nil {
		return err
	}
	byKind := map[string]map[int]*chanOp{siteSend: f.sends, siteRecv: f.recvs}
	for _, e := range es {
		if e.Type != types.EdgeContains {
			continue
		}
		n, ok := t.g.nodes[e.Dst]
		if !ok || n.Type != types.NodeCallSite {
			continue
		}
		if op := byKind[n.SubKind][n.StartLine]; op != nil && op.site == nil {
			site := n
			op.site = &site
		}
	}
	return nil
}

// accessStep is the step of a read or write of dst, when the query's focus
// names dst; nil otherwise.
func (t *tracer) accessStep(body types.Node, e types.Edge, dst types.Node, line, hop int, up *step) *step {
	if t.q.Focus == "" || !t.focused(dst) {
		return nil
	}
	k := kindRead
	if e.Type == types.EdgeWritesField || strings.HasSuffix(e.DispatchKind, "write") {
		k = kindWrite
	}
	return &step{kind: k, node: dst, body: body, file: body.FilePath, line: line, certainty: certaintyOf(e), hop: hop, up: up, match: true}
}

// focused reports whether n is what the focus names: its printed name, its
// qualified name, its name, or the end of its qualified name.
func (t *tracer) focused(n types.Node) bool {
	f := t.q.Focus
	return n.Name == f || n.QualifiedName == f || t.g.name(n) == f || strings.HasSuffix(n.QualifiedName, "."+f)
}

// callSteps makes the steps of calls: a static call, an interface call
// with the implementations it may run, a call through a function value
// with the functions it may hold. A call outside the build on a line that
// takes or releases a lock is the lock step itself (sync.Mutex.Lock).
func (t *tracer) callSteps(body types.Node, f *bodyFacts, hop int, up *step) error {
	if !t.q.follows(LinkCalls) {
		return nil
	}
	used := map[int]bool{}
	for _, e := range f.calls {
		dst := t.g.nodes[e.Dst]
		if !outside(dst) {
			f.steps = append(f.steps, t.callStep(body, dst, e, hop, up))
			continue
		}
		if f.locked[e.Line] {
			continue
		}
		if s, err := t.syncStep(body, dst, e, hop, up); err != nil {
			return err
		} else if s != nil {
			f.steps = append(f.steps, s)
			continue
		}
		if words, ok := stdFact(dst); ok {
			f.steps = append(f.steps, &step{kind: kindStd, node: dst, body: body, file: body.FilePath, line: e.Line,
				std: words, certainty: certaintyOf(e), hop: hop, up: up})
			continue
		}
		t.count(cutOutside, dst)
	}
	for _, e := range f.invokes {
		dst := t.g.nodes[e.Dst]
		if e.DispatchKind == kindClosureCall {
			// A literal called where it is written: its code is this
			// body's own lines.
			continue
		}
		if e.Dst == body.ID || e.DispatchKind == kindFuncValueCall || e.DispatchKind == kindMethodValueCall {
			// A call through a function value (a variable, a field of
			// function type, a method value): the graph marks it with an
			// invokes edge to the function itself and names what the value
			// may hold with dispatches_to.
			used[e.Line] = true
			s := &step{kind: kindFuncValue, node: body, body: body, file: body.FilePath, line: e.Line, certainty: certaintyOf(e), hop: hop, up: up}
			if e.Dst != body.ID {
				s.node = dst
			}
			t.valueAlts(s, f.dispatch[e.Line], hop, up)
			s.std = t.valueCallOps(body, e.Line)
			f.steps = append(f.steps, s)
			continue
		}
		targets, err := t.dispatchTargets(dst)
		if err != nil {
			return err
		}
		if len(targets) == 0 && len(f.dispatch[e.Line]) > 0 {
			used[e.Line] = true
			s := &step{kind: kindFuncValue, node: dst, body: body, file: body.FilePath, line: e.Line, certainty: certaintyOf(e), hop: hop, up: up}
			t.valueAlts(s, f.dispatch[e.Line], hop, up)
			s.std = t.valueCallOps(body, e.Line)
			f.steps = append(f.steps, s)
			continue
		}
		s := &step{kind: kindIface, node: dst, body: body, file: body.FilePath, line: e.Line, certainty: certaintyOf(e), hop: hop, up: up}
		t.implAlts(s, targets, hop, up)
		f.steps = append(f.steps, s)
	}
	for line, es := range f.dispatch {
		if used[line] {
			continue
		}
		s := &step{kind: kindFuncValue, node: body, body: body, file: body.FilePath, line: line, certainty: certaintyOf(es[0]), hop: hop, up: up}
		t.valueAlts(s, es, hop, up)
		s.std = t.valueCallOps(body, line)
		f.steps = append(f.steps, s)
	}
	return nil
}

// callStep is the step of a call of dst; it opens dst when dst is in the
// build and is not left-out test code.
func (t *tracer) callStep(body, dst types.Node, e types.Edge, hop int, up *step) *step {
	s := &step{kind: kindCall, node: dst, body: body, file: body.FilePath, line: e.Line, certainty: certaintyOf(e), hop: hop, up: up}
	t.setOpen(s, dst)
	return s
}

// setOpen makes s open n, or records why it cannot: n outside the build,
// test code left out, or not a body.
func (t *tracer) setOpen(s *step, n types.Node) {
	switch {
	case outside(n):
		s.cut = cutOutside
		t.count(cutOutside, n)
	case t.leftOut(n):
		s.cut = cutTest
		t.count(cutTest, n)
	case isBody(n):
		o := n
		s.open = &o
	}
}

// dispatchTarget is an implementation an interface method may run.
type dispatchTarget struct {
	node   types.Node
	edge   types.Edge
	byType bool
	// typeOnly, on the first target, counts the implementations the type
	// allows that the value flow does not reach.
	typeOnly int
}

// dispatchTargets returns the implementations an interface method may run:
// those the value flow reaches, or, when it reaches none, those its type
// allows (by type).
func (t *tracer) dispatchTargets(im types.Node) ([]dispatchTarget, error) {
	es, err := t.g.outOf(im.ID)
	if err != nil {
		return nil, err
	}
	var flow, typed []dispatchTarget
	for _, e := range es {
		if e.Type != types.EdgeDispatchesTo {
			continue
		}
		n, ok := t.g.nodes[e.Dst]
		if !ok {
			continue
		}
		switch e.DispatchKind {
		case kindInterfaceMethod:
			flow = append(flow, dispatchTarget{node: n, edge: e})
		case kindTypeHierarchy:
			typed = append(typed, dispatchTarget{node: n, edge: e, byType: true})
		}
	}
	if len(flow) > 0 {
		inFlow := map[string]bool{}
		for _, d := range flow {
			inFlow[d.node.ID] = true
		}
		for _, d := range typed {
			if !inFlow[d.node.ID] {
				flow[0].typeOnly++
			}
		}
		return flow, nil
	}
	return typed, nil
}

// implAlts sets the alternatives of an interface call: one per
// implementation, at its declaration.
func (t *tracer) implAlts(s *step, targets []dispatchTarget, hop int, up *step) {
	if len(targets) > 0 && targets[0].typeOnly > 0 {
		s.notes = append(s.notes, fmt.Sprintf("+%d more alternatives its type allows that the value flow does not reach: %s qname=%s depth=1",
			targets[0].typeOnly, t.q.CalleesTool, t.g.name(s.node)))
	}
	for _, d := range targets {
		if t.leftOut(d.node) {
			t.count(cutTest, d.node)
			continue
		}
		a := &step{kind: kindImpl, node: d.node, body: d.node, file: d.node.FilePath, line: d.node.StartLine,
			certainty: certaintyOf(d.edge), byType: d.byType, hop: hop, up: up}
		t.setOpen(a, d.node)
		s.alts = append(s.alts, a)
	}
	t.capAlts(s, t.flowCall(s.body, len(s.alts)))
}

// valueAlts sets the alternatives of a call through a function value: the
// functions the value may hold.
func (t *tracer) valueAlts(s *step, es []types.Edge, hop int, up *step) {
	seen := map[string]bool{}
	for _, e := range es {
		n, ok := t.g.nodes[e.Dst]
		if !ok || seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		if t.leftOut(n) {
			t.count(cutTest, n)
			continue
		}
		a := &step{kind: kindImpl, node: n, body: n, file: n.FilePath, line: n.StartLine, certainty: certaintyOf(e), hop: hop, up: up,
			closure: e.DispatchKind == kindClosureCall}
		if a.closure {
			// The target names the function the literal is written in;
			// the literal's code is shown where it is written.
			a.notes = append(a.notes, "the literal's code is under the step where it is written, not opened here")
		} else {
			t.setOpen(a, n)
		}
		s.alts = append(s.alts, a)
	}
	t.capAlts(s, t.flowCall(s.body, len(s.alts)))
}

// capAlts orders s's alternatives by the branch rule and keeps
// branchCap of them; call lists them all.
func (t *tracer) capAlts(s *step, call string) {
	sortAlts(t.g, s.alts)
	if len(s.alts) > 1 {
		t.branchPoints++
	}
	s.altsCall = call
	t.cutAlts(s, t.branchCap)
}

// cutAlts keeps the first n alternatives of s (already in the branch
// order) and records what it cut: how many, the next few by name, and the
// distinct nodes among all.
func (t *tracer) cutAlts(s *step, n int) {
	distinct := map[string]bool{}
	for _, a := range s.alts {
		distinct[a.node.ID] = true
	}
	if len(s.alts) <= n {
		return
	}
	s.altsDistinct = len(distinct)
	s.altsLeft += len(s.alts) - n
	s.altsTop = nil
	for _, a := range s.alts[n:min(len(s.alts), n+cutTopShown)] {
		s.altsTop = append(s.altsTop, t.g.name(a.node))
	}
	s.alts = s.alts[:n]
}

// releasesShown is how many releases in other functions a lock step names.
const releasesShown = 2

// releasedElsewhere notes on a lock step whose body never releases the
// lock where it is released: the releases_lock edges of other functions.
func (t *tracer) releasedElsewhere(s *step, body types.Node, es []types.Edge) error {
	for _, e := range es {
		if e.Type == types.EdgeReleasesLock && e.Dst == s.node.ID {
			return nil
		}
	}
	rel, err := t.g.inOf(s.node.ID, types.EdgeReleasesLock)
	if err != nil {
		return err
	}
	var at []string
	for _, e := range rel {
		if e.Src == body.ID {
			continue
		}
		if n, ok, err := t.g.node(e.Src); err != nil {
			return err
		} else if ok && !t.leftOut(n) {
			at = append(at, fmt.Sprintf("%s (%s:%d)", t.g.name(n), e.FilePath, e.Line))
		}
	}
	switch {
	case len(at) == 0:
		s.notes = append(s.notes, "not released in this function, and no release is recorded")
	case len(at) > releasesShown:
		s.notes = append(s.notes, fmt.Sprintf("released elsewhere: %s, +%d more", strings.Join(at[:releasesShown], ", "), len(at)-releasesShown))
	default:
		s.notes = append(s.notes, "released elsewhere: "+strings.Join(at, ", "))
	}
	return nil
}

// messageSteps makes the steps of sends and receives. A send lists the
// receives it can wake (triggers) as alternatives; a receive names who can
// wake it. An operation on a line that takes or releases a lock is that
// lock step (a channel used as a lock).
func (t *tracer) messageSteps(body types.Node, f *bodyFacts, hop int, up *step) error {
	for line, op := range f.sends {
		if !op.traced() || f.locked[line] {
			continue
		}
		s := &step{kind: kindSend, node: op.ch, via: siteOf(op), body: body, file: body.FilePath, line: line, certainty: op.certainty, hop: hop, up: up}
		t.nameChannel(s, op)
		if err := t.receiveAlts(s, body, f.triggers[line], hop, up); err != nil {
			return err
		}
		f.steps = append(f.steps, s)
	}
	for line, op := range f.closes {
		if !op.traced() || f.locked[line] {
			continue
		}
		s := &step{kind: kindClose, node: op.ch, via: siteOf(op), body: body, file: body.FilePath, line: line, certainty: op.certainty, hop: hop, up: up}
		t.nameChannel(s, op)
		if err := t.receiveAlts(s, body, f.triggers[line], hop, up); err != nil {
			return err
		}
		f.steps = append(f.steps, s)
	}
	for line, op := range f.recvs {
		if !op.traced() || f.locked[line] {
			continue
		}
		s := &step{kind: kindRecv, node: op.ch, via: siteOf(op), body: body, file: body.FilePath, line: line, certainty: op.certainty, hop: hop, up: up}
		t.nameChannel(s, op)
		if err := t.wokenBy(s); err != nil {
			return err
		}
		f.steps = append(f.steps, s)
	}
	return nil
}

// traced reports whether op is a step: the graph names its channel or at
// least its site (a channel the value flow found no origin for, such as a
// parameter that no caller in the build fills).
func (op *chanOp) traced() bool { return op.found || op.site != nil }

// untraced notes a channel operation whose channel has no origin in the
// graph; the step then names its site.
const untraced = "the graph finds no origin for this channel (it comes from outside the build, or through a value it does not follow)"

// nameChannel sets the channel a send, receive or close step names: its
// origin, or its site when the graph finds no origin, or when the only
// origin is in test code and tests are left out (a parameter that only a
// test fills).
func (t *tracer) nameChannel(s *step, op *chanOp) {
	switch {
	case op.found && t.leftOut(op.ch) && op.site != nil:
		s.notes = append(s.notes, "the only origin of this channel the graph finds is in test code ("+t.g.name(op.ch)+")")
		s.node = *op.site
	case !op.found:
		s.node = siteOf(op)
		if o, _ := t.receiveOp(s); o == "" && s.kind != kindRecv {
			s.notes = append(s.notes, untraced)
		}
	}
}

// siteOf is op's CallSite, or the zero node.
func siteOf(op *chanOp) types.Node {
	if op.site == nil {
		return types.Node{}
	}
	return *op.site
}

// receiveAlts sets the alternatives of a send: each receive it can wake,
// in the function that receives, opened into the code the message runs.
func (t *tracer) receiveAlts(s *step, sender types.Node, trig []types.Edge, hop int, up *step) error {
	for _, e := range trig {
		r, ok, err := t.g.node(e.Dst)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		h, err := t.receiveOwner(r)
		if err != nil {
			return err
		}
		if t.leftOut(h) {
			t.count(cutTest, h)
			continue
		}
		a := &step{kind: kindReceivedAt, node: h, via: r, body: h, file: r.FilePath, line: r.StartLine, certainty: certaintyOf(e), hop: hop, up: up}
		if isBody(h) {
			o := h
			a.open = &o
		}
		if err := t.runsIn(a, h); err != nil {
			return err
		}
		if err := t.receiverContext(a, h, r); err != nil {
			return err
		}
		s.alts = append(s.alts, a)
	}
	if len(s.alts) == 0 {
		s.notes = append(s.notes, "no receive in the build")
	}
	t.capAlts(s, t.flowCall(sender, len(s.alts)))
	return nil
}

// receiverContext sets on a receive step a, which is a step of the
// receiving function h at its receive site r, what any of h's steps gets:
// the statements around r (its loop, branch, clause and the earlier
// returns and jumps that may skip it) and the locks the graph says are
// held at r (runs_under_lock), not the sender's.
func (t *tracer) receiverContext(a *step, h, r types.Node) error {
	if !isBody(h) {
		return nil
	}
	owner, err := t.siteOwner(h)
	if err != nil {
		return err
	}
	st, err := t.structureOf(owner)
	if err != nil {
		return err
	}
	lo, hi := bodySpan(h)
	a.places = st.placesOf(r.StartLine, lo, hi)
	a.afterReturn = st.earlyExits(r.StartLine, lo, hi, st.ownerAt(r.StartLine))
	a.rangeHead = st.rangeHeadAt(r.StartLine)
	sf, err := t.siteFactsOf(owner)
	if err != nil || !sf.recorded {
		return err
	}
	a.held = t.heldAtSite(sf, r)
	return nil
}

// altLocks gives the implementations an interface or function-value call
// may run the locks held at the call as finally read (the graph's
// must-hold fact at its site replaces the line order): what is certainly
// held at the call is certainly held where it runs.
func altLocks(ss []*step) {
	for _, s := range ss {
		if s.kind == kindIface || s.kind == kindFuncValue {
			for _, a := range s.alts {
				a.held = s.held
			}
		}
		altLocks(s.children)
	}
}

// channelLockNote is the note of a receive that takes a channel used as a
// lock (an acquires_lock of a Channel on its line): it is woken when a
// holder releases the channel, at each releases_lock of it.
func (t *tracer) channelLockNote(s *step) (string, bool, error) {
	es, err := t.g.outOf(s.body.ID)
	if err != nil {
		return "", false, err
	}
	for _, e := range es {
		ch, ok := t.g.nodes[e.Dst]
		if e.Type != types.EdgeAcquiresLock || e.Line != s.line || !ok || ch.Type != types.NodeChannel {
			continue
		}
		rel, err := t.g.inOf(ch.ID, types.EdgeReleasesLock)
		if err != nil {
			return "", false, err
		}
		sortEdges(rel)
		sort.SliceStable(rel, func(i, j int) bool {
			if rel[i].FilePath != rel[j].FilePath {
				return rel[i].FilePath < rel[j].FilePath
			}
			return rel[i].Line < rel[j].Line
		})
		var at []string
		seen := map[string]bool{}
		for _, r := range rel {
			k := fmt.Sprintf("%s:%d", r.FilePath, r.Line)
			if src, ok := t.g.nodes[r.Src]; ok && !t.leftOut(src) && !seen[k] {
				seen[k] = true
				at = append(at, k)
			}
		}
		if len(at) == 0 {
			return "a channel used as a lock (" + t.g.name(ch) + "); the graph links no release of it", true, nil
		}
		return "a channel used as a lock (" + t.g.name(ch) + "): woken when a holder releases it, at " + shortListN(at, peerOpsShown), true, nil
	}
	return "", false, nil
}

// valueCallOps says what a call through a function value on line does
// when the graph knows the value is a context's cancel function
// (operates_on cancel at a site there), or "".
func (t *tracer) valueCallOps(body types.Node, line int) string {
	owner, err := t.siteOwner(body)
	if err != nil {
		return ""
	}
	sf, err := t.siteFactsOf(owner)
	if err != nil {
		return ""
	}
	for _, n := range sf.byLine[line] {
		var ops []types.Edge
		for _, e := range sf.ops[n.ID] {
			if types.SyncOp(e.DispatchKind) == types.SyncCancel {
				ops = append(ops, e)
			}
		}
		if len(ops) > 0 {
			return t.opWords(ops)
		}
	}
	return ""
}

// leftOut reports whether n is test code the query leaves out.
func (t *tracer) leftOut(n types.Node) bool { return t.q.Tests == TestsExclude && t.g.test(n) }

// runsInShown is how many goroutine starts a receive names.
const runsInShown = 2

// runsIn notes on a receive step which goroutines run the function h that
// receives: the go statements whose goroutine's code calls h, or that
// start h when it is a goroutine literal.
func (t *tracer) runsIn(s *step, h types.Node) error {
	var gors []types.Node
	if h.Type == types.NodeGoroutine {
		gors = append(gors, h)
	} else {
		es, err := t.g.inOf(h.ID, types.EdgeCalls, types.EdgeInvokes, types.EdgeDispatchesTo)
		if err != nil {
			return err
		}
		for _, e := range es {
			if g, ok := t.g.nodes[e.Src]; ok && g.Type == types.NodeGoroutine {
				gors = append(gors, g)
			}
		}
	}
	var starts []string
	for _, g := range gors {
		ps, err := t.startedBy(g)
		if err != nil {
			return err
		}
		for _, p := range ps {
			if p.node.ID != g.ID && !t.leftOut(p.node) {
				starts = append(starts, fmt.Sprintf("%s (%s:%d)", t.g.name(p.node), p.file, p.line))
			}
		}
	}
	switch {
	case len(starts) > runsInShown:
		s.notes = append(s.notes, fmt.Sprintf("runs in goroutines started by %s, +%d more", strings.Join(starts[:runsInShown], ", "), len(starts)-runsInShown))
	case len(starts) > 0:
		s.notes = append(s.notes, "runs in goroutines started by "+strings.Join(starts, ", "))
	}
	return nil
}

// receiveOwner is the function that holds the receive site r (the one
// that contains it, or the source of its recvs_from edge), or r itself.
func (t *tracer) receiveOwner(r types.Node) (types.Node, error) {
	es, err := t.g.inOf(r.ID, types.EdgeContains, types.EdgeRecvsFrom)
	if err != nil {
		return types.Node{}, err
	}
	for _, e := range es {
		if n, ok, err := t.g.node(e.Src); err != nil {
			return types.Node{}, err
		} else if ok && isBody(n) {
			return n, nil
		}
	}
	return r, nil
}

// noSender notes a receive that no send in the build reaches. A close of
// the channel also wakes it, and the graph does not record closes yet, so
// it is not a conclusion that nothing wakes it.
const noSender = "no send in the build reaches it: it may be woken by a close of the channel (closes are not recorded yet) or from outside the build"

// wokenByShown is how many senders a receive names.
const wokenByShown = 2

// wokenBy notes on a receive who can wake it (the senders of its triggers
// edges), that nothing in the build does, and that a signal carries no
// value.
func (t *tracer) wokenBy(s *step) error {
	if signal, err := t.isSignal(s.node); err != nil {
		return err
	} else if signal {
		s.notes = append(s.notes, "a signal: the message carries no value")
	}
	if s.via.ID == "" {
		return nil
	}
	es, err := t.g.inOf(s.via.ID, types.EdgeTriggers)
	if err != nil {
		return err
	}
	var senders, closers []string
	for _, e := range es {
		n, ok, err := t.g.node(e.Src)
		if err != nil {
			return err
		}
		if !ok || t.leftOut(n) {
			continue
		}
		at := fmt.Sprintf("%s (%s:%d)", t.g.name(n), e.FilePath, e.Line)
		if strings.HasSuffix(e.DispatchKind, triggerAliasMark) {
			at += " " + triggerAliasMark
		}
		if t.g.closesAt(n, e.Line) {
			closers = append(closers, at)
		} else {
			senders = append(senders, at)
		}
	}
	if len(senders)+len(closers) == 0 {
		if note, ok, err := t.channelLockNote(s); err != nil || ok {
			if ok {
				s.notes = append(s.notes, note)
			}
			return err
		}
		note, err := t.unsentNote(s)
		if err != nil {
			return err
		}
		if note == noSendOrClose && s.node.Type == types.NodeCallSite {
			// The channel's origin is not found: the senders and closers
			// are unknown, not absent.
			note = "the graph links no send or close to it (its channel's origin is not found)"
		}
		s.notes = append(s.notes, note)
		return nil
	}
	if len(senders) > 0 {
		s.notes = append(s.notes, "woken by "+shortList(senders))
	}
	if len(closers) > 0 {
		s.notes = append(s.notes, "closed by "+shortList(closers))
	}
	return nil
}

// unsentNote is the note of a receive no send or close in the build
// reaches: a timer's channel the runtime sends on, a context's done
// channel its cancel closes (naming where cancel is called), else
// noSender.
func (t *tracer) unsentNote(s *step) (string, error) {
	if s.via.ID != "" {
		if site, ok, err := t.g.node(s.via.ID); err == nil && ok {
			for _, u := range site.SyncUnknown {
				switch u.Op {
				case types.SyncDoneRecv:
					return "closed when a context is cancelled; the graph does not name the context (" + unknownReason(u) + ") ~aliasing", nil
				case types.SyncTimerRecv:
					return "sent by the runtime when a timer fires; the graph does not name the timer (" + unknownReason(u) + ") ~aliasing", nil
				}
			}
		}
	}
	op, prim := t.receiveOp(s)
	switch op {
	case types.SyncTimerRecv:
		if prim.Type == types.NodeCallSite {
			return fmt.Sprintf("sent by the runtime when the timer made at %s:%d fires", prim.FilePath, prim.StartLine), nil
		}
		return "sent by the runtime when the timer " + t.g.name(prim) + " fires", nil
	case types.SyncDoneRecv:
		at, err := t.cancellers(prim)
		if err != nil {
			return "", err
		}
		if len(at) == 0 {
			return "closed when the context " + t.g.name(prim) + " is cancelled or times out; the graph links no cancel call to it (a cancel kept in a field or passed on is not followed)", nil
		}
		return "closed when the context " + t.g.name(prim) + " is cancelled: cancel at " + shortListN(at, peerOpsShown), nil
	}
	return t.noSender(), nil
}

// shortList joins the first wokenByShown names and counts the rest.
func shortList(names []string) string {
	if len(names) > wokenByShown {
		return fmt.Sprintf("%s, +%d more", strings.Join(names[:wokenByShown], ", "), len(names)-wokenByShown)
	}
	return strings.Join(names, ", ")
}

// noSender is the note of a receive nothing in the build sends to: a
// conclusion when the graph records closes, else a close may wake it.
func (t *tracer) noSender() string {
	if t.g.closesRecorded() {
		return noSendOrClose
	}
	return noSender
}

// noSendOrClose notes a receive no send or close in the build reaches.
const noSendOrClose = "nothing in the build sends on this channel or closes it (it comes from outside the build, or nothing wakes it)"

// isSignal reports whether messages on n carry no value: n, or a channel
// stored in it (flows_to of kind chan), is a channel of empty structs.
func (t *tracer) isSignal(n types.Node) (bool, error) {
	if n.Type == types.NodeChannel {
		return strings.Contains(n.Signature, signalType), nil
	}
	es, err := t.g.inOf(n.ID, types.EdgeFlowsTo)
	if err != nil {
		return false, err
	}
	for _, e := range es {
		if e.DispatchKind != concfacts.AliasKind {
			continue
		}
		if c, ok := t.g.nodes[e.Src]; ok && c.Type == types.NodeChannel && strings.Contains(c.Signature, signalType) {
			return true, nil
		}
	}
	return false, nil
}

// inGoroutine reports whether line of body is in a goroutine body starts
// (its go line or its literal): a call there runs in the goroutine.
func (t *tracer) inGoroutine(body types.Node, line int) (bool, error) {
	es, err := t.g.outOf(body.ID)
	if err != nil {
		return false, err
	}
	for _, e := range es {
		if e.Type != types.EdgeSpawns {
			continue
		}
		if g, ok, err := t.g.node(e.Dst); err != nil {
			return false, err
		} else if ok && line >= g.StartLine && line <= g.EndLine {
			return true, nil
		}
	}
	return false, nil
}

// lockContext sets on each step of one body, in order, the locks it runs
// under: inherited, then those taken on earlier lines of the body and not
// released before it (a deferred release holds to the end).
func lockContext(ss []*step, inherited []heldLock) {
	held := append([]heldLock(nil), inherited...)
	suspended := map[string]int{} // lock id -> last line of a returning branch that released it
	for _, s := range ss {
		s.held = nil
		for _, h := range held {
			if end, ok := suspended[h.lock.ID]; ok && s.line <= end || h.until > 0 && s.line > h.until {
				continue
			}
			s.held = append(s.held, h)
		}
		if s.kind == kindIface || s.kind == kindFuncValue {
			// The implementation runs where the call is; a receive of a
			// send runs elsewhere and holds nothing of the sender's.
			for _, a := range s.alts {
				a.held = s.held
			}
		}
		switch {
		case s.kind == kindLock:
			held = append(held, heldLock{lock: s.node, taken: s, file: s.file, line: s.line, until: s.releaseUntil})
		case s.kind == kindUnlock && s.releaseUntil > 0:
			// Released on a branch that returns: held again after it.
			suspended[s.node.ID] = s.releaseUntil
		case s.kind == kindUnlock && !s.deferred:
			kept := held[:0:0]
			for _, h := range held {
				if h.lock.ID != s.node.ID {
					kept = append(kept, h)
				}
			}
			held = kept
		}
	}
}

// nestHandled puts each step a receive's message runs (its handles edges
// name the line) under that receive; a step two receives handle goes under
// the later one, the innermost.
func (t *tracer) nestHandled(ss []*step) ([]*step, error) {
	handled := map[*step]map[int]bool{}
	for _, s := range ss {
		if s.kind != kindRecv || s.via.ID == "" {
			continue
		}
		es, err := t.g.outOf(s.via.ID)
		if err != nil {
			return nil, err
		}
		lines := map[int]bool{}
		for _, e := range es {
			if e.Type == types.EdgeHandles && e.Line > s.line {
				lines[e.Line] = true
			}
		}
		handled[s] = lines
	}
	var top []*step
	var recvs []*step
	for _, s := range ss {
		var owner *step
		for _, r := range recvs {
			if handled[r][s.line] {
				owner = r
			}
		}
		if owner != nil {
			owner.children = append(owner.children, s)
		} else {
			top = append(top, s)
		}
		if s.kind == kindRecv {
			recvs = append(recvs, s)
		}
	}
	return top, nil
}

// nestHandledInLiterals nests handled code inside each literal's steps.
func (t *tracer) nestHandledInLiterals(ss []*step) error {
	for _, s := range ss {
		if s.kind != kindLiteral {
			continue
		}
		nested, err := t.nestHandled(s.children)
		if err != nil {
			return err
		}
		s.children = nested
		if err := t.nestHandledInLiterals(nested); err != nil {
			return err
		}
	}
	return nil
}

// groupSelects turns each run of two or more select cases (receives and
// sends on lines that begin with "case") into one select step whose
// alternatives they are, in every list of steps.
func (t *tracer) groupSelects(body types.Node, ss []*step) []*step {
	var out []*step
	var run []*step
	_, facts := t.selectFacts(body)
	minCases := 2
	if facts {
		// The graph names the select: one case and a default is one too.
		minCases = 1
	}
	flush := func() {
		if len(run) < minCases {
			out = append(out, run...)
		} else {
			first := run[0]
			sel := &step{kind: kindSelect, node: t.selectNode(body, first.line), body: body, file: first.file, line: t.selectLine(body, first.line),
				alts: run, hop: first.hop, up: first.up, held: first.held, places: first.places, afterReturn: returnsBefore(first.afterReturn, first.line)}
			t.branchPoints++
			out = append(out, sel)
		}
		run = nil
	}
	ss = t.caseOperands(body, ss)
	for _, s := range ss {
		s.children = t.groupSelects(body, s.children)
		if (s.kind == kindRecv || s.kind == kindSend) && t.isSelectCase(body, s.line) {
			if len(run) > 0 && t.selectNode(body, run[0].line).ID != t.selectNode(body, s.line).ID {
				flush()
			}
			run = append(run, s)
			continue
		}
		flush()
		out = append(out, s)
	}
	flush()
	return out
}

// caseOperands puts the other steps on the line of a select case (the
// calls that give its channel, c.events.Chan()) first under the case's
// channel operation, so the cases of one select stay one run.
func (t *tracer) caseOperands(body types.Node, ss []*step) []*step {
	ops := map[int]*step{}
	for _, s := range ss {
		if (s.kind == kindRecv || s.kind == kindSend) && t.isSelectCase(body, s.line) {
			ops[s.line] = s
		}
	}
	var out []*step
	var moved = map[*step][]*step{}
	for _, s := range ss {
		if op, ok := ops[s.line]; ok && op != s {
			moved[op] = append(moved[op], s)
			continue
		}
		out = append(out, s)
	}
	for op, first := range moved {
		op.children = append(first, op.children...)
	}
	return out
}

// caseOperandsDeep applies caseOperands to every list of steps.
func (t *tracer) caseOperandsDeep(body types.Node, ss []*step) []*step {
	ss = t.caseOperands(body, ss)
	for _, s := range ss {
		s.children = t.caseOperandsDeep(body, s.children)
	}
	return ss
}

// selectLine is the line of the select statement whose case is on line,
// or line itself.
func (t *tracer) selectLine(body types.Node, line int) int {
	if st, ok := t.selectFacts(body); ok {
		if sel, ok := st.selectCases[line]; ok {
			return sel.StartLine
		}
	}
	return line
}

// selectNode is the select statement of the case on line when the graph
// records it, else body.
func (t *tracer) selectNode(body types.Node, line int) types.Node {
	if st, ok := t.selectFacts(body); ok {
		if sel, ok := st.selectCases[line]; ok {
			return sel
		}
	}
	return body
}

// handledSteps returns the steps the message received at r runs, read from
// the body that holds r (h, or the goroutine literal of h around r).
func (t *tracer) handledSteps(r, h types.Node, hop int, up *step) ([]*step, error) {
	body, err := t.bodyAround(h, r.StartLine)
	if err != nil {
		return nil, err
	}
	ss, err := t.bodySteps(body, nil, hop, up)
	if err != nil {
		return nil, err
	}
	var found *step
	walk(ss, func(s, _ *step) {
		if found == nil && s.kind == kindRecv && s.via.ID == r.ID {
			found = s
		}
	})
	if found == nil {
		return nil, nil
	}
	return found.children, nil
}

// bodyAround is the goroutine literal h starts around line, or h.
func (t *tracer) bodyAround(h types.Node, line int) (types.Node, error) {
	es, err := t.g.outOf(h.ID)
	if err != nil {
		return types.Node{}, err
	}
	for _, e := range es {
		if e.Type != types.EdgeSpawns {
			continue
		}
		if g, ok, err := t.g.node(e.Dst); err != nil {
			return types.Node{}, err
		} else if ok && line >= g.StartLine && line <= g.EndLine && g.EndLine > g.StartLine {
			return g, nil
		}
	}
	return h, nil
}
