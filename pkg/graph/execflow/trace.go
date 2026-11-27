package execflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// buildFactor is how many times the budget the flow reads before it
// chooses what to show, and focusBuildCap how many it reads with a focus,
// whose steps are chosen after the rest is pruned.
const (
	buildFactor   = 8
	focusBuildCap = 2000
)

// tracer is one Trace call.
type tracer struct {
	g            *reader
	q            Query
	root         *step
	built        int
	buildCap     int
	cuts         map[cutReason]map[string]bool
	branchPoints int
	structures   map[string]*structure
	sites        map[string]*siteFacts
	// branchCap is how many alternatives a branch point keeps while the
	// flow is built: Query.Branches, or all of them when via or from steer
	// the answer (cut after pruning).
	branchCap int
	// steer are the nodes via or from name; stopAtSteer stops a path at
	// them (from).
	steer       map[string]bool
	stopAtSteer bool
}

// count records that n was not opened for reason r; the summary counts
// distinct nodes per reason.
func (t *tracer) count(r cutReason, n types.Node) {
	if t.cuts[r] == nil {
		t.cuts[r] = map[string]bool{}
	}
	t.cuts[r][n.ID] = true
}

// Trace answers q on the graph r as text: a summary, numbered steps and the
// calls that continue the answer. A seed that does not name one symbol
// gets an answer that says so (the candidates of an ambiguous name). The
// error wraps ErrBadQuery for a query that cannot run, or the reader's
// error.
func Trace(r store.Reader, q Query) (string, error) {
	q, err := q.normalized()
	if err != nil {
		return "", err
	}
	res, err := store.ResolveName(r, q.Seed, "")
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", q.Seed, err)
	}
	if miss := seedMiss(q, res); miss != "" {
		return miss, nil
	}
	seed, ok := seedBody(res.Nodes)
	if !ok {
		return notBody(q, res), nil
	}
	g, err := newReader(r)
	if err != nil {
		return "", err
	}
	t := &tracer{g: g, q: q, cuts: map[cutReason]map[string]bool{}, structures: map[string]*structure{}, sites: map[string]*siteFacts{}, branchCap: q.Branches, buildCap: q.Budget * buildFactor}
	if q.Focus != "" {
		t.buildCap = focusBuildCap
	}
	if miss, err := t.steerBy(r, q); err != nil || miss != "" {
		return miss, err
	}
	s := seed
	t.root = &step{kind: kindSeed, node: seed, body: seed, file: seed.FilePath, line: seed.StartLine, open: &s}
	var roots []*step
	if q.Direction == Backward {
		roots, err = t.backward()
	} else {
		roots, err = t.forward()
	}
	if err != nil {
		return "", err
	}
	return t.render(roots, res), nil
}

// seedMiss is the answer for a seed that names no symbol, or several.
func seedMiss(q Query, res store.NameResolution) string {
	switch {
	case res.Ambiguous():
		var b strings.Builder
		fmt.Fprintf(&b, "%s: %q names %d declarations; call again with one of:\n", q.ToolName, q.Seed, len(res.Candidates))
		for _, c := range res.Candidates {
			fmt.Fprintf(&b, "  %s qname=%s\n", q.ToolName, c)
		}
		return b.String()
	case !res.Found():
		out := fmt.Sprintf("%s: %q names no symbol in the graph.\n", q.ToolName, q.Seed)
		if len(res.OtherCase) > 0 {
			out += "Names that differ only in case: " + strings.Join(res.OtherCase, ", ") + "\n"
		}
		return out
	}
	return ""
}

// seedBody is the body among the resolved nodes: the first function,
// method or goroutine by file and line.
func seedBody(nodes []types.Node) (types.Node, bool) {
	var bodies []types.Node
	for _, n := range nodes {
		if isBody(n) {
			bodies = append(bodies, n)
		}
	}
	if len(bodies) == 0 {
		return types.Node{}, false
	}
	sort.SliceStable(bodies, func(i, j int) bool {
		a, b := bodies[i], bodies[j]
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		return a.ID < b.ID
	})
	return bodies[0], true
}

// notBody is the answer for a seed that is not a function, method or
// goroutine.
func notBody(q Query, res store.NameResolution) string {
	kind := "symbol"
	if len(res.Nodes) > 0 {
		kind = string(res.Nodes[0].Type)
	}
	return fmt.Sprintf("%s: %s is a %s; the flow starts at a function, method or goroutine. "+
		"For who reads or writes it use %s qname=%s.\n", q.ToolName, res.QName, kind, q.ReferencesTool, res.QName)
}

// forward builds the seed's steps and opens them breadth first.
func (t *tracer) forward() ([]*step, error) {
	roots, err := t.bodySteps(*t.root.open, nil, 0, t.root)
	if err != nil {
		return nil, err
	}
	t.root.children = roots
	queue := flatten(roots)
	t.built = len(queue)
	for i := 0; i < len(queue); i++ {
		s := queue[i]
		if !t.mayOpen(s) {
			continue
		}
		children, err := t.openForward(s)
		if err != nil {
			return nil, err
		}
		s.children = append(s.children, children...)
		added := flatten(children)
		t.built += len(added)
		queue = append(queue, added...)
	}
	if t.q.Focus != "" {
		if err := t.markGuards(roots); err != nil {
			return nil, err
		}
		roots = prune(roots)
	}
	roots = t.steered(roots)
	t.root.children = roots
	return roots, nil
}

// mayOpen reports whether s is opened, and records why not: nothing to
// open, depth, recursion or the build budget.
func (t *tracer) mayOpen(s *step) bool {
	if s.open == nil || s.cut != "" {
		return false
	}
	if again := t.running(s); again != nil {
		s.cut, s.again = cutRecursion, again
		if s.kind == kindReceivedAt {
			s.again = t.receiveStep(s.via)
			if s.again == nil {
				s.again = again
			}
		}
		t.count(cutRecursion, *s.open)
		return false
	}
	if t.stopAtSteer && t.isSteered(s) {
		return false
	}
	if s.hop >= t.q.Depth && t.q.Direction == Backward {
		if n, err := t.countPredecessors(*s.open); err == nil && n == 0 {
			s.notes = append(s.notes, entryPoint)
			return false
		}
	}
	if t.literalGo(s) {
		// A goroutine literal's code is the function's own: opening it is
		// not a hop.
		return t.built < t.buildCap
	}
	if s.hop >= t.q.Depth && t.q.Direction == Forward && s.kind != kindReceivedAt {
		if ss, err := t.bodySteps(*s.open, nil, s.hop+1, s); err == nil && len(ss) == 0 {
			s.notes = append(s.notes, leafNote)
			return false
		}
	}
	if s.hop >= t.q.Depth {
		s.cut = cutDepth
		t.count(cutDepth, *s.open)
		return false
	}
	if t.built >= t.buildCap {
		s.cut = cutBudget
		t.count(cutBudget, *s.open)
		return false
	}
	return true
}

// literalGo reports whether s starts a goroutine literal written in its
// own function (go func() {...}()), whose code is that function's own.
func (t *tracer) literalGo(s *step) bool {
	if s.kind != kindGo || s.open == nil || t.q.Direction != Forward {
		return false
	}
	targets, err := t.goTargets([]types.Node{*s.open})
	return err == nil && targets[s.open.ID] == ""
}

// leafNote notes a forward step whose function the graph records no step
// in: opening it would show nothing.
const leafNote = "the graph records no call, goroutine, channel operation or lock in it"

// entryPoint notes a backward step nothing in the build leads to.
const entryPoint = "nothing in the build calls it or starts it: an entry point (called from outside the build, by reflection, or not at all)"

// countPredecessors counts what calls or starts body (callersOf, and the
// starters of a goroutine).
func (t *tracer) countPredecessors(body types.Node) (int, error) {
	n := 0
	add := func(p *step) {
		if !t.leftOut(p.node) {
			n++
		}
	}
	if body.Type == types.NodeGoroutine {
		ps, err := t.startedBy(body)
		for _, p := range ps {
			add(p)
		}
		return n, err
	}
	err := t.callersOf(body, add)
	return n, err
}

// running returns the step that already runs the body s would open, on the
// path from the seed to s: s would recurse.
func (t *tracer) running(s *step) *step {
	for p := s.up; p != nil; p = p.up {
		if p.open != nil && p.open.ID == s.open.ID {
			return p
		}
	}
	return nil
}

// receiveStep finds the receive step of site r among the steps built so
// far.
func (t *tracer) receiveStep(r types.Node) *step {
	var found *step
	walk(t.root.children, func(s, _ *step) {
		if found == nil && s.kind == kindRecv && s.via.ID == r.ID {
			found = s
		}
	})
	return found
}

// openForward returns the steps s opens: the callee's or the goroutine's
// steps, or the code a received message runs. A call runs under the locks
// held at it; a goroutine and a message's code start with none.
func (t *tracer) openForward(s *step) ([]*step, error) {
	switch s.kind {
	case kindReceivedAt:
		return t.handledSteps(s.via, *s.open, s.hop+1, s)
	case kindGo, kindRunsLater:
		hop := s.hop + 1
		if t.literalGo(s) {
			hop = s.hop
		}
		return t.bodySteps(*s.open, nil, hop, s)
	}
	return t.bodySteps(*s.open, s.held, s.hop+1, s)
}

// backward lists the seed's predecessors and opens them breadth first.
func (t *tracer) backward() ([]*step, error) {
	if err := t.insideLocks(); err != nil {
		return nil, err
	}
	roots, err := t.predecessors(t.root)
	if err != nil {
		return nil, err
	}
	t.root.alts = roots
	queue := append([]*step(nil), roots...)
	t.built = len(queue)
	for i := 0; i < len(queue); i++ {
		s := queue[i]
		if !t.mayOpen(s) {
			continue
		}
		ps, err := t.predecessors(s)
		if err != nil {
			return nil, err
		}
		s.alts = ps
		t.built += len(ps)
		queue = append(queue, ps...)
	}
	if t.q.Focus != "" {
		if err := t.markAccesses(queue); err != nil {
			return nil, err
		}
		t.root.alts = prune(t.root.alts)
		if len(t.root.alts) == 0 {
			t.root.notes = append(t.root.notes, "no step found that leads here reads or writes "+t.q.Focus)
		}
	}
	t.root.alts = t.steered(t.root.alts)
	return []*step{t.root}, nil
}

// steerBy resolves the nodes Query.Via and Query.From name; it returns the
// answer for a name that names none or several.
func (t *tracer) steerBy(r store.Reader, q Query) (string, error) {
	name := q.Via
	if q.From != "" {
		name = q.From
	}
	if name == "" {
		return "", nil
	}
	res, err := store.ResolveName(r, name, "")
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", name, err)
	}
	if miss := seedMiss(Query{Seed: name, ToolName: q.ToolName}, res); miss != "" {
		return miss, nil
	}
	t.steer = map[string]bool{}
	for _, n := range res.Nodes {
		t.steer[n.ID] = true
	}
	t.branchCap, t.buildCap, t.stopAtSteer = steerBranchCap, focusBuildCap, q.From != ""
	return "", nil
}

// steerBranchCap keeps every alternative while a steered flow is built.
const steerBranchCap = 1 << 20

// isSteered reports whether s names a node via or from names.
func (t *tracer) isSteered(s *step) bool {
	return t.steer[s.node.ID] || s.via.ID != "" && t.steer[s.via.ID]
}

// steered keeps, when via or from steer the answer, the paths through the
// node they name, the shortest first, and cuts each branch point to
// Query.Branches.
func (t *tracer) steered(roots []*step) []*step {
	if t.steer == nil {
		return roots
	}
	walk(roots, func(s, _ *step) { s.steered = t.isSteered(s) })
	roots = pruneSteered(roots)
	dist := distances(roots)
	var recut func(ss []*step)
	recut = func(ss []*step) {
		for _, s := range ss {
			sort.SliceStable(s.alts, func(i, j int) bool { return dist[s.alts[i]] < dist[s.alts[j]] })
			s.altsLeft, s.altsTop = 0, nil
			t.cutAlts(s, t.q.Branches)
			recut(s.alts)
			recut(s.children)
		}
	}
	recut(roots)
	if t.q.Direction == Backward {
		t.root.altsLeft, t.root.altsTop = 0, nil
		sort.SliceStable(roots, func(i, j int) bool { return dist[roots[i]] < dist[roots[j]] })
		t.root.alts = roots
		t.cutAlts(t.root, t.q.Branches)
		roots = t.root.alts
	}
	if len(roots) == 0 {
		t.root.notes = append(t.root.notes, "no path found through "+t.steerName()+" within depth "+fmt.Sprint(t.q.Depth))
	}
	return roots
}

// steerName is the name via or from gives.
func (t *tracer) steerName() string {
	if t.q.From != "" {
		return t.q.From
	}
	return t.q.Via
}

// accessLinesShown is how many lines a focused access note names.
const accessLinesShown = 3

// markAccesses notes on each backward step whose function reads or writes
// the focused field or variable where it does, and marks it.
func (t *tracer) markAccesses(steps []*step) error {
	for _, s := range append([]*step{t.root}, steps...) {
		if !isBody(s.node) {
			continue
		}
		es, err := t.g.outOf(s.node.ID)
		if err != nil {
			return err
		}
		lines := map[string][]string{}
		var order []string
		for _, e := range es {
			n, ok := t.g.nodes[e.Dst]
			if !ok || !t.focused(n) {
				continue
			}
			verb := ""
			switch {
			case e.Type == types.EdgeWritesField || e.Type == types.EdgeReferences && strings.HasSuffix(e.DispatchKind, "write"):
				verb = "writes"
			case e.Type == types.EdgeReadsField || e.Type == types.EdgeReferences:
				verb = "reads"
			default:
				continue
			}
			key := verb + " " + t.g.name(n)
			if _, ok := lines[key]; !ok {
				order = append(order, key)
			}
			if len(lines[key]) < accessLinesShown {
				lines[key] = append(lines[key], fmt.Sprint(e.Line))
			}
		}
		for _, k := range order {
			s.notes = append(s.notes, k+" at line "+strings.Join(lines[k], ", "))
			s.match = true
		}
	}
	return nil
}

// flatten returns ss and every step under them, in pre-order.
func flatten(ss []*step) []*step {
	var out []*step
	walk(ss, func(s, _ *step) { out = append(out, s) })
	return out
}

// markGuards marks the lock steps of the mutexes that guard a focused
// field (accessed_under_lock) as steps the focus keeps.
func (t *tracer) markGuards(roots []*step) error {
	guards := map[string]bool{}
	seen := map[string]bool{}
	var err error
	walk(roots, func(s, _ *step) {
		if err != nil || !s.match || seen[s.node.ID] {
			return
		}
		seen[s.node.ID] = true
		var es []types.Edge
		if es, err = t.g.outOf(s.node.ID); err != nil {
			return
		}
		for _, e := range es {
			if e.Type == types.EdgeAccessedUnderLock {
				guards[e.Dst] = true
			}
		}
	})
	if err != nil {
		return err
	}
	walk(roots, func(s, _ *step) {
		if (s.kind == kindLock || s.kind == kindUnlock) && guards[s.node.ID] {
			s.match = true
		}
	})
	return nil
}

// insideLocks notes on the seed the locks it takes and releases itself, so
// a backward answer says what the seed runs under.
func (t *tracer) insideLocks() error {
	seed := *t.root.open
	es, err := t.g.outOf(seed.ID)
	if err != nil {
		return err
	}
	for _, e := range es {
		lock, ok := t.g.nodes[e.Dst]
		if !ok {
			continue
		}
		verb := "takes"
		switch e.Type {
		case types.EdgeAcquiresLock:
		case types.EdgeReleasesLock:
			verb = "releases"
		default:
			continue
		}
		note := fmt.Sprintf("inside: %s lock %s%s at %s:%d", verb, t.g.name(lock), lockWords(lock), seed.FilePath, e.Line)
		if m := certaintyMark(certaintyOf(e)); m != "" {
			note += " " + m
		}
		t.root.notes = append(t.root.notes, note)
		if e.Type == types.EdgeAcquiresLock {
			held := &step{node: lock}
			if err := t.releasedElsewhere(held, seed, es); err != nil {
				return err
			}
			for _, n := range held.notes {
				t.root.notes = append(t.root.notes, "  "+n)
			}
		}
	}
	return nil
}
