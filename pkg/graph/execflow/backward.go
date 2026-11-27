package execflow

import (
	"fmt"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// backward.go reads what leads to a function: the functions that call it
// (directly, through an interface method, through a function value), the
// goroutines that run it with the function that starts each, and the
// senders whose message makes a call in it run.

// Dispatch kinds of a call through a function value.
const (
	kindFuncValueCall   = "func_value"
	kindMethodValueCall = "method_value"
	kindClosureCall     = "closure"
)

// predecessors returns the steps that lead to the body s opens, as
// alternatives: its callers and starters, and, when s is a call at a line
// that a receive's message runs, the senders that can wake that receive.
// Each is at the line of the predecessor that leads on, under the locks
// held there.
func (t *tracer) predecessors(s *step) ([]*step, error) {
	body := *s.open
	var out []*step
	seen := map[string]bool{}
	add := func(p *step) {
		key := fmt.Sprintf("%d|%s|%s|%s|%d", p.kind, p.node.ID, p.via.ID, p.file, p.line)
		if seen[key] {
			return
		}
		seen[key] = true
		if t.leftOut(p.node) {
			t.count(cutTest, p.node)
			return
		}
		p.hop, p.up, p.below = s.hop+1, s, body
		out = append(out, p)
	}
	if body.Type == types.NodeGoroutine {
		ps, err := t.startedBy(body)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			add(p)
		}
	} else if err := t.callersOf(body, add); err != nil {
		return nil, err
	}
	if err := t.messageSenders(s, body, add); err != nil {
		return nil, err
	}
	for _, p := range out {
		// A goroutine and the code a message runs hold nothing of the
		// starter's or the sender's locks.
		if p.kind == kindStartedBy || p.kind == kindMessageFrom {
			continue
		}
		if l, ok := t.literalAround(p.body, p.line); ok {
			// The call is written in a literal that runs later: the graph
			// records it on the function it is written in.
			p.notes = append(p.notes, fmt.Sprintf("the call is in a function literal at line %d that %s: it runs then, not when %s runs",
				l.start, litWhen[l.kind], t.g.name(p.node)))
			continue
		}
		if held, ok, err := t.heldAtCall(p, body); err != nil {
			return nil, err
		} else if ok {
			p.held = held
			continue
		}
		held, err := t.heldAt(p.body, p.line)
		if err != nil {
			return nil, err
		}
		p.held = held
	}
	if len(out) == 0 && s != t.root {
		s.notes = append(s.notes, entryPoint)
	}
	sortAlts(t.g, out)
	if len(out) > 1 {
		t.branchPoints++
	}
	s.alts = out
	s.altsCall = t.flowCall(body, len(out))
	t.cutAlts(s, t.branchCap)
	return s.alts, nil
}

// callersOf adds the steps that call body: a call or interface call
// recorded against it, a call through an interface method that may run it,
// a call through a function value that may hold it, and a goroutine whose
// code calls it (with the function that starts the goroutine).
func (t *tracer) callersOf(body types.Node, add func(*step)) error {
	es, err := t.g.inOf(body.ID, types.EdgeCalls, types.EdgeInvokes, types.EdgeDispatchesTo)
	if err != nil {
		return err
	}
	for _, e := range es {
		src, ok, err := t.g.node(e.Src)
		if err != nil {
			return err
		}
		if !ok || e.Src == e.Dst {
			// An invokes edge from a function to itself marks its call
			// through a function value, not a call of itself.
			continue
		}
		switch {
		case src.Type == types.NodeGoroutine:
			if !t.q.follows(LinkGoroutines) {
				continue
			}
			ps, err := t.startedBy(src)
			if err != nil {
				return err
			}
			for _, p := range ps {
				add(p)
			}
		case !t.q.follows(LinkCalls):
		case e.Type == types.EdgeDispatchesTo && (e.DispatchKind == kindInterfaceMethod || e.DispatchKind == kindTypeHierarchy):
			if err := t.callersVia(src, e, add); err != nil {
				return err
			}
		case e.Type == types.EdgeDispatchesTo && (e.DispatchKind == kindFuncValueCall || e.DispatchKind == kindClosureCall):
			if err := t.addCaller(kindCalledByValue, src, e, add); err != nil {
				return err
			}
		default:
			if err := t.addCaller(kindCalledBy, src, e, add); err != nil {
				return err
			}
		}
	}
	return nil
}

// addCaller adds the step of caller at the line of e, unless that line is
// on a go line or in a goroutine literal of caller: the call then runs in
// the goroutine, which its own code edge names (startedBy). A caller that
// is not a function (a package-level variable's initialiser) is a step
// that is not opened.
func (t *tracer) addCaller(k stepKind, caller types.Node, e types.Edge, add func(*step)) error {
	if in, err := t.inGoroutine(caller, e.Line); err != nil || in {
		return err
	}
	add(t.callerStep(k, caller, e))
	return nil
}

// callerStep is the step of caller at the line of e.
func (t *tracer) callerStep(k stepKind, caller types.Node, e types.Edge) *step {
	file := e.FilePath
	if file == "" {
		file = caller.FilePath
	}
	s := &step{kind: k, node: caller, body: caller, file: file, line: e.Line, certainty: certaintyOf(e)}
	t.setOpen(s, caller)
	return s
}

// callersVia adds the callers of interface method im, which may run the
// body dispatch edge d ends at (by type when only the type hierarchy
// allows it).
func (t *tracer) callersVia(im types.Node, d types.Edge, add func(*step)) error {
	es, err := t.g.inOf(im.ID, types.EdgeInvokes)
	if err != nil {
		return err
	}
	byType := d.DispatchKind == kindTypeHierarchy
	for _, e := range es {
		c, ok, err := t.g.node(e.Src)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if c.Type == types.NodeGoroutine {
			if !t.q.follows(LinkGoroutines) {
				continue
			}
			ps, err := t.startedBy(c)
			if err != nil {
				return err
			}
			for _, p := range ps {
				p.notes = append(p.notes, "its code calls "+t.g.name(im)+", which may run the step above")
				add(p)
			}
			continue
		}
		if in, err := t.inGoroutine(c, e.Line); err != nil {
			return err
		} else if in {
			continue
		}
		s := t.callerStep(kindCalledVia, c, e)
		s.via, s.certainty, s.byType = im, certaintyOf(d), byType
		add(s)
	}
	return nil
}

// startedBy returns the steps that start goroutine g: each function whose
// spawns edge ends at it, at the go line; g itself when none does.
func (t *tracer) startedBy(g types.Node) ([]*step, error) {
	es, err := t.g.inOf(g.ID, types.EdgeSpawns)
	if err != nil {
		return nil, err
	}
	var out []*step
	for _, e := range es {
		s, ok, err := t.g.node(e.Src)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		st := &step{kind: kindStartedBy, node: s, via: g, body: s, file: g.FilePath, line: g.StartLine, certainty: certaintyOf(e)}
		t.setOpen(st, s)
		out = append(out, st)
	}
	if len(out) == 0 {
		out = append(out, &step{kind: kindStartedBy, node: g, via: g, body: g, file: g.FilePath, line: g.StartLine,
			notes: []string{"no function in the build starts this goroutine"}})
	}
	return out, nil
}

// messageSenders adds, when s is a call of s.below at s.line in body that
// runs because a message arrives (a receive's handles edge names the
// CallSite of the call), each sender that can wake that receive, and notes
// the receive on s (or that no sender is in the build).
func (t *tracer) messageSenders(s *step, body types.Node, add func(*step)) error {
	if !t.q.follows(LinkMessages) || s.line == 0 || s.below.ID == "" || s.kind == kindStartedBy || s.kind == kindMessageFrom {
		return nil
	}
	sites, err := t.g.inOf(s.below.ID, types.EdgeSiteCalls)
	if err != nil {
		return err
	}
	for _, sc := range sites {
		x, ok := t.g.nodes[sc.Src]
		if !ok || x.Type != types.NodeCallSite || x.StartLine != s.line || x.FilePath != s.file {
			continue
		}
		handles, err := t.g.inOf(x.ID, types.EdgeHandles)
		if err != nil {
			return err
		}
		for _, h := range handles {
			r, ok, err := t.g.node(h.Src)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := t.sendersOf(s, body, r, add); err != nil {
				return err
			}
		}
	}
	return nil
}

// sendersOf adds the senders that can wake receive r in body, and notes r
// on s, whose call the message runs.
func (t *tracer) sendersOf(s *step, body, r types.Node, add func(*step)) error {
	trig, err := t.g.inOf(r.ID, types.EdgeTriggers)
	if err != nil {
		return err
	}
	note := fmt.Sprintf("runs when a message arrives at %s (%s:%d)", t.g.name(r), r.FilePath, r.StartLine)
	if len(trig) == 0 {
		note += "; " + t.noSender()
	}
	s.notes = append(s.notes, note)
	for _, e := range trig {
		sender, ok, err := t.g.node(e.Src)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		ch, err := t.sentOn(sender, e)
		if err != nil {
			return err
		}
		p := &step{kind: kindMessageFrom, node: sender, via: r, body: sender, file: e.FilePath, line: e.Line,
			channel: ch, certainty: certaintyOf(e)}
		t.setOpen(p, sender)
		add(p)
	}
	return nil
}

// sentOn names the channel of trigger e by the name its send uses (the
// field or variable of the sends_to edge on its line), else by the origin
// the trigger names.
func (t *tracer) sentOn(sender types.Node, e types.Edge) (string, error) {
	es, err := t.g.outOf(sender.ID)
	if err != nil {
		return "", err
	}
	for _, s := range es {
		if s.Type != types.EdgeSendsTo || s.Line != e.Line || s.DispatchKind == "" {
			continue
		}
		if n, ok := t.g.nodes[s.Dst]; ok && n.Type != types.NodeCallSite {
			return t.g.name(n), nil
		}
	}
	return triggerChannel(e), nil
}

// litWhen says when a literal of each kind runs.
var litWhen = map[litKind]string{
	litDeferred: "is deferred (it runs when the function returns)",
	litPassed:   "is passed to a call (it runs when that calls it)",
	litStored:   "is stored (it runs when it is called)",
	litReturned: "is returned (it runs when the caller calls it)",
}

// literalAround is the literal of body, other than one called in place,
// that holds line.
func (t *tracer) literalAround(body types.Node, line int) (litSpan, bool) {
	if !isBody(body) || body.Type == types.NodeGoroutine {
		return litSpan{}, false
	}
	for _, l := range t.literalSpans(body, body.StartLine, body.EndLine) {
		if l.kind != litInPlace && l.holds(line) {
			return l, true
		}
	}
	return litSpan{}, false
}

// heldAtCall is what the graph says is held at the call of below in p's
// function (runs_under_lock at that site); ok is false without the fact.
func (t *tracer) heldAtCall(p *step, below types.Node) ([]heldLock, bool, error) {
	if !isBody(p.body) {
		return nil, false, nil
	}
	sf, err := t.siteFactsOf(p.body)
	if err != nil || !sf.recorded {
		return nil, false, err
	}
	callee := below.ID
	if p.kind == kindCalledVia {
		callee = p.via.ID
	}
	site, ok := sf.siteAt(p.line, callee)
	if !ok {
		return nil, false, nil
	}
	// The line order gives where each lock the fact names was taken.
	lineOrder, err := t.heldAt(p.body, p.line)
	if err != nil {
		return nil, false, err
	}
	return mergeHeld(nil, lineOrder, t.heldAtSite(sf, site)), true, nil
}

// heldAt returns the locks held at line of body by line order: acquired on
// an earlier line and not released on a line in between (a deferred
// release holds to the end).
func (t *tracer) heldAt(body types.Node, line int) ([]heldLock, error) {
	es, err := t.g.outOf(body.ID)
	if err != nil {
		return nil, err
	}
	st, err := t.structureOf(body)
	if err != nil {
		return nil, err
	}
	for _, l := range st.literalSpans(body, body.StartLine, body.EndLine) {
		if l.kind == litPassed {
			t.namePassed(&l, st, nil)
		}
		if l.kind != litInPlace && !l.inherits() && l.holds(line) {
			// A literal that runs later (stored, returned, started or kept
			// by the callee) holds nothing the function holds where it is
			// written.
			return nil, nil
		}
	}
	var held []heldLock
	for _, e := range es {
		if e.Line >= line || e.Line == 0 {
			continue
		}
		lock, ok := t.g.nodes[e.Dst]
		if !ok {
			continue
		}
		end := st.returningBranch(e.Line, 0, body.EndLine)
		switch e.Type {
		case types.EdgeAcquiresLock:
			if end > 0 && line > end {
				continue
			}
			held = append(held, heldLock{lock: lock, file: body.FilePath, line: e.Line})
		case types.EdgeReleasesLock:
			if st.deferredAt(e.Line) || end > 0 && line > end {
				continue
			}
			kept := held[:0:0]
			for _, h := range held {
				if h.lock.ID != lock.ID {
					kept = append(kept, h)
				}
			}
			held = kept
		}
	}
	return held, nil
}
