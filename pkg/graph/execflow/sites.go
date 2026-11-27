package execflow

import (
	"fmt"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// sites.go reads the facts the graph records per CallSite (schema 1.44):
// the locks held there (runs_under_lock, by the must-hold analysis), the
// sync primitive a call operates on (operates_on) and the function a call
// makes run later (runs_func). A step finds its site by line and callee.

// siteFacts are the facts of the CallSites one function contains.
type siteFacts struct {
	byLine   map[int][]types.Node
	callee   map[string]string       // site id -> the node it calls (site_calls)
	locks    map[string][]types.Edge // site id -> runs_under_lock edges
	ops      map[string][]types.Edge // site id -> operates_on edges
	later    map[string][]types.Edge // site id -> runs_func edges
	recorded bool                    // the graph records held locks per site
}

// siteFactsOf reads the site facts of owner, once.
func (t *tracer) siteFactsOf(owner types.Node) (*siteFacts, error) {
	if sf, ok := t.sites[owner.ID]; ok {
		return sf, nil
	}
	es, err := t.g.outOf(owner.ID)
	if err != nil {
		return nil, err
	}
	sf := &siteFacts{byLine: map[int][]types.Node{}, callee: map[string]string{}, locks: map[string][]types.Edge{},
		ops: map[string][]types.Edge{}, later: map[string][]types.Edge{}, recorded: t.g.recorded(types.EdgeRunsUnderLock)}
	var ids []string
	for _, e := range es {
		if e.Type != types.EdgeContains {
			continue
		}
		if n, ok := t.g.nodes[e.Dst]; ok && n.Type == types.NodeCallSite {
			sf.byLine[n.StartLine] = append(sf.byLine[n.StartLine], n)
			ids = append(ids, n.ID)
		}
	}
	for _, f := range []struct {
		t  types.EdgeType
		to map[string][]types.Edge
	}{{types.EdgeRunsUnderLock, sf.locks}, {types.EdgeOperatesOn, sf.ops}, {types.EdgeRunsFunc, sf.later}} {
		got, err := t.g.batchOut(ids, f.t)
		if err != nil {
			return nil, err
		}
		for _, e := range got {
			f.to[e.Src] = append(f.to[e.Src], e)
		}
	}
	calls, err := t.g.batchOut(ids, types.EdgeSiteCalls)
	if err != nil {
		return nil, err
	}
	for _, e := range calls {
		sf.callee[e.Src] = e.Dst
	}
	t.sites[owner.ID] = sf
	return sf, nil
}

// siteOfStep is the CallSite of s: its channel operation's site, else the
// site on its line that calls its node, else the only site on its line.
func (sf *siteFacts) siteOfStep(s *step) (types.Node, bool) {
	if s.via.ID != "" && s.via.Type == types.NodeCallSite && (s.kind == kindRecv || s.kind == kindSend || s.kind == kindClose) {
		return s.via, true
	}
	if n, ok := sf.siteAt(s.line, s.node.ID); ok {
		return n, true
	}
	// A sync operation's own site is the one whose operates_on the call
	// performs (h.override.Store(len(x) != 0) has the len site too).
	return sf.opSiteAt(s.line, s.node)
}

// siteAt is the site on line that calls callee, else the only site there.
func (sf *siteFacts) siteAt(line int, callee string) (types.Node, bool) {
	ss := sf.byLine[line]
	for _, n := range ss {
		if sf.callee[n.ID] == callee {
			return n, true
		}
	}
	if len(ss) == 1 {
		return ss[0], true
	}
	return types.Node{}, false
}

// opSiteAt is the site on line that calls callee, else the first site
// there whose sync operation is one the callee (by name) performs: a
// time.Until on the line of a time.AfterFunc does not arm the timer.
func (sf *siteFacts) opSiteAt(line int, callee types.Node) (types.Node, bool) {
	if n, ok := sf.siteAt(line, callee.ID); ok && len(sf.ops[n.ID]) > 0 && sf.fits(n, callee) {
		return n, true
	}
	for _, n := range sf.byLine[line] {
		if len(sf.ops[n.ID]) > 0 && sf.callee[n.ID] == "" && sf.fits(n, callee) {
			return n, true
		}
	}
	return types.Node{}, false
}

// fits reports whether every operation at site is one a call named like
// callee performs.
func (sf *siteFacts) fits(site, callee types.Node) bool {
	for _, e := range sf.ops[site.ID] {
		names, ok := opCallees[types.SyncOp(e.DispatchKind)]
		if !ok {
			continue
		}
		fit := false
		for _, n := range names {
			if strings.HasPrefix(callee.Name, n) {
				fit = true
			}
		}
		if !fit {
			return false
		}
	}
	return true
}

// opCallees are the names (prefixes) of the calls that perform each
// operation.
var opCallees = map[types.SyncOp][]string{
	types.SyncWaitGroupAdd: {"Add"}, types.SyncWaitGroupDone: {"Done"}, types.SyncWaitGroupWait: {"Wait"},
	types.SyncWaitGroupGo: {"Go"}, types.SyncOnceDo: {"Do"}, types.SyncCondWait: {"Wait"}, types.SyncCondSignal: {"Signal"},
	types.SyncCondBroadcast: {"Broadcast"}, types.SyncAtomicLoad: {"Load"}, types.SyncAtomicStore: {"Store"},
	types.SyncAtomicAdd: {"Add"}, types.SyncAtomicSwap: {"Swap"}, types.SyncAtomicCAS: {"CompareAndSwap"},
	types.SyncAtomicAnd: {"And"}, types.SyncAtomicOr: {"Or"}, types.SyncContextMake: {"With"}, types.SyncCancelMake: {"With"},
	types.SyncContextDone: {"Done"}, types.SyncTimerMake: {"AfterFunc", "NewTimer", "NewTicker", "After", "Tick"},
	types.SyncTimerStop: {"Stop"}, types.SyncTimerReset: {"Reset"},
}

// lineLocks is what is held at the sites of line that call nothing in the
// build, when s has no site of its own: each lock held at all of them. A
// site inside a function literal in skip (the literal a step writes) runs
// elsewhere and is not read.
func (t *tracer) lineLocks(sf *siteFacts, line int, skip ...types.Node) ([]heldLock, bool) {
	var ss []types.Node
	for _, n := range sf.byLine[line] {
		in := false
		for _, l := range skip {
			in = in || inside(n, l)
		}
		if !in {
			ss = append(ss, n)
		}
	}
	if len(ss) == 0 {
		return nil, false
	}
	count := map[string]int{}
	var order []heldLock
	for _, n := range ss {
		for _, h := range t.heldAtSite(sf, n) {
			if count[h.lock.ID] == 0 {
				order = append(order, h)
			}
			count[h.lock.ID]++
		}
	}
	var out []heldLock
	for _, h := range order {
		if count[h.lock.ID] == len(ss) {
			out = append(out, h)
		}
	}
	return out, true
}

// heldAtSite is what the graph says is held at site: each lock, and
// whether a caller holds it (propagated).
func (t *tracer) heldAtSite(sf *siteFacts, site types.Node) []heldLock {
	var out []heldLock
	for _, e := range sf.locks[site.ID] {
		if n, ok := t.g.nodes[e.Dst]; ok {
			out = append(out, heldLock{lock: n, fact: true, byCaller: e.DispatchKind == propagatedKind})
		}
	}
	return out
}

// propagatedKind marks a runs_under_lock edge whose mutex a caller holds.
const propagatedKind = "propagated"

// siteLocks replaces, for every step of body that has a site, the lock
// context read by line order with the graph's: inherited locks (held where
// the step that opened body runs) and the locks held at the site. Steps
// without a site keep the line-order context, marked as inferred.
func (t *tracer) siteLocks(body types.Node, ss []*step, inherited []heldLock) error {
	owner, err := t.siteOwner(body)
	if err != nil {
		return err
	}
	sf, err := t.siteFactsOf(owner)
	if err != nil || !sf.recorded {
		return err
	}
	named, err := t.goTargets([]types.Node{body})
	if err != nil {
		return err
	}
	if named[body.ID] != "" {
		// go f(x): the site on the go line is the starter's go statement;
		// the goroutine holds none of the starter's locks.
		return nil
	}
	var visit func(ss []*step, inherited []heldLock)
	visit = func(ss []*step, inherited []heldLock) {
		for _, s := range ss {
			if s.kind == kindRunsLater {
				continue
			}
			if s.kind == kindLiteral || s.kind == kindGo {
				// The statement that defers, passes or starts code: the
				// locks held at the sites on its line. A deferred
				// literal's calls carry the locks held when it runs (on
				// return); a literal that runs before the call returns
				// holds the caller's; others hold nothing.
				var skip []types.Node
				if s.kind == kindLiteral {
					skip = append(skip, s.lit.node)
				}
				if held, ok := t.lineLocks(sf, s.line, skip...); ok {
					s.held = mergeHeld(inherited, s.held, held)
				}
				inner := inherited
				switch {
				case s.kind == kindGo:
					inner = nil
				case s.lit.kind == litPassed:
					inner = inheritedBy(s)
				case s.lit.kind != litDeferred:
					inner = nil
				}
				visit(s.children, inner)
				continue
			}
			if site, ok := sf.siteOfStep(s); ok {
				s.held = mergeHeld(inherited, s.held, t.heldAtSite(sf, site))
			} else if held, ok := t.lineLocks(sf, s.line); ok && s.kind != kindLock && s.kind != kindUnlock {
				s.held = mergeHeld(inherited, s.held, held)
			}
			inner := inherited
			if s.kind == kindLiteral && s.lit.kind != litDeferred {
				inner = nil
			}
			visit(s.children, inner)
		}
	}
	visit(ss, inherited)
	return nil
}

// mergeHeld is inherited, then the site's locks, each lock once; a
// site lock keeps the step that took it from the line-order context.
func mergeHeld(inherited, lineOrder, site []heldLock) []heldLock {
	out := append([]heldLock(nil), inherited...)
	have := map[string]bool{}
	for _, h := range inherited {
		have[h.lock.ID] = true
	}
	for _, h := range site {
		if have[h.lock.ID] {
			continue
		}
		for _, l := range lineOrder {
			if l.lock.ID == h.lock.ID {
				h.taken, h.file, h.line = l.taken, l.file, l.line
			}
		}
		have[h.lock.ID] = true
		out = append(out, h)
	}
	return out
}

// syncWordsByOp say what an operates_on operation does; the primitive's
// name follows.
var syncWordsByOp = map[types.SyncOp]string{
	types.SyncWaitGroupAdd: "adds to the WaitGroup", types.SyncWaitGroupDone: "marks done the WaitGroup",
	types.SyncWaitGroupWait: "waits for the WaitGroup", types.SyncWaitGroupGo: "starts a goroutine of the WaitGroup",
	types.SyncOnceDo: "runs once, by", types.SyncCondWait: "waits on the condition", types.SyncCondSignal: "wakes one waiter of",
	types.SyncCondBroadcast: "wakes every waiter of", types.SyncAtomicLoad: "atomically loads", types.SyncAtomicStore: "atomically stores",
	types.SyncAtomicAdd: "atomically adds to", types.SyncAtomicSwap: "atomically swaps", types.SyncAtomicCAS: "atomically compares and swaps",
	types.SyncAtomicAnd: "atomically clears bits of", types.SyncAtomicOr: "atomically sets bits of", types.SyncContextMake: "makes the context",
	types.SyncCancelMake: "makes the cancel function", types.SyncCancel: "cancels, through", types.SyncContextDone: "gets the done channel of",
	types.SyncTimerMake: "arms the timer", types.SyncTimerStop: "stops the timer", types.SyncTimerReset: "re-arms the timer",
}

// syncOps returns the operations at site, in order.
func (sf *siteFacts) syncOps(site types.Node) []types.Edge { return sf.ops[site.ID] }

// opWords says what the operations at a site do, naming each primitive.
func (t *tracer) opWords(ops []types.Edge) string {
	var parts []string
	for _, e := range ops {
		w, ok := syncWordsByOp[types.SyncOp(e.DispatchKind)]
		n, found := t.g.nodes[e.Dst]
		if !ok || !found {
			continue
		}
		part := w + " " + t.g.name(n)
		if e.Confidence == types.ConfInferred {
			// A primitive received as a parameter, followed to the
			// callers' declarations.
			part += " " + approxPrefix + string(concfactsAliasing)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

// concfactsAliasing is the reason of an operation the graph follows to a
// caller's declaration: values are told apart by declaration.
const concfactsAliasing = concfacts.ReasonAliasing

// peerOpsShown is how many other sites of one primitive a note names.
const peerOpsShown = 3

// peerOps notes, for a wait on a WaitGroup, the Add and Done sites on the
// same WaitGroup.
func (t *tracer) peerOps(s *step, ops []types.Edge) error {
	for _, e := range ops {
		if types.SyncOp(e.DispatchKind) != types.SyncWaitGroupWait {
			continue
		}
		in, err := t.g.inOf(e.Dst, types.EdgeOperatesOn)
		if err != nil {
			return err
		}
		for _, op := range []types.SyncOp{types.SyncWaitGroupAdd, types.SyncWaitGroupDone} {
			var at []string
			for _, p := range in {
				if types.SyncOp(p.DispatchKind) != op {
					continue
				}
				if site, ok := t.g.nodes[p.Src]; ok && !t.leftOut(site) {
					at = append(at, fmt.Sprintf("%s:%d", site.FilePath, site.StartLine))
				}
			}
			if len(at) > 0 {
				s.notes = append(s.notes, fmt.Sprintf("%s at %s", opLabel[op], shortListN(at, peerOpsShown)))
			}
		}
	}
	return nil
}

// opLabel names an operation in a note.
var opLabel = map[types.SyncOp]string{types.SyncWaitGroupAdd: "Add", types.SyncWaitGroupDone: "Done"}

// shortListN joins the first n items and counts the rest.
func shortListN(items []string, n int) string {
	if len(items) > n {
		return fmt.Sprintf("%s, +%d more", strings.Join(items[:n], ", "), len(items)-n)
	}
	return strings.Join(items, ", ")
}

// receiveOp is the operation the graph records for a receive's site:
// timer_recv (the runtime sends when the timer fires) or done_recv (a
// cancel closes it), or "".
func (t *tracer) receiveOp(s *step) (types.SyncOp, types.Node) {
	if s.via.ID == "" {
		return "", types.Node{}
	}
	es, err := t.g.outOf(s.via.ID)
	if err != nil {
		return "", types.Node{}
	}
	for _, e := range es {
		if e.Type != types.EdgeOperatesOn {
			continue
		}
		op := types.SyncOp(e.DispatchKind)
		if op == types.SyncTimerRecv || op == types.SyncDoneRecv {
			return op, t.g.nodes[e.Dst]
		}
	}
	return "", types.Node{}
}

// cancellers names who can cancel the context ctx: the sites that call a
// cancel function made with it (ctx_make and cancel_make on one site, then
// cancel on the cancel function).
//
// The context and the cancel function each may be kept in a field or a
// variable (flows_to sync): every name of each is read.
func (t *tracer) cancellers(ctx types.Node) ([]string, error) {
	ctxNames, err := t.syncAliases(ctx.ID)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	for _, name := range ctxNames {
		in, err := t.g.inOf(name, types.EdgeOperatesOn)
		if err != nil {
			return nil, err
		}
		for _, mk := range in {
			if types.SyncOp(mk.DispatchKind) != types.SyncContextMake {
				continue
			}
			at, err := t.cancelCalls(mk.Src, seen)
			if err != nil {
				return nil, err
			}
			out = append(out, at...)
		}
	}
	return out, nil
}

// cancelCalls are the sites that call the cancel function made at the
// site maker, under any name it is kept in; seen holds the sites already
// named.
func (t *tracer) cancelCalls(maker string, seen map[string]bool) ([]string, error) {
	es, err := t.g.outOf(maker)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, cm := range es {
		if cm.Type != types.EdgeOperatesOn || types.SyncOp(cm.DispatchKind) != types.SyncCancelMake {
			continue
		}
		names, err := t.syncAliases(cm.Dst)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			calls, err := t.g.inOf(name, types.EdgeOperatesOn)
			if err != nil {
				return nil, err
			}
			for _, c := range calls {
				site, ok := t.g.nodes[c.Src]
				if types.SyncOp(c.DispatchKind) != types.SyncCancel || !ok || t.leftOut(site) || seen[site.ID] {
					continue
				}
				seen[site.ID] = true
				out = append(out, fmt.Sprintf("%s:%d", site.FilePath, site.StartLine))
			}
		}
	}
	return out, nil
}

// maxSyncAliases bounds the names of one sync value that are read.
const maxSyncAliases = 32

// syncAliases are id and every holder joined to it by flows_to sync edges
// either way (a local stored into a field, the field read back), in the
// order found.
func (t *tracer) syncAliases(id string) ([]string, error) {
	out := []string{id}
	seen := map[string]bool{id: true}
	for i := 0; i < len(out) && len(out) < maxSyncAliases; i++ {
		fwd, err := t.g.batchOut([]string{out[i]}, types.EdgeFlowsTo)
		if err != nil {
			return nil, err
		}
		back, err := t.g.inOf(out[i], types.EdgeFlowsTo)
		if err != nil {
			return nil, err
		}
		for _, e := range append(fwd, back...) {
			next := e.Dst
			if next == out[i] {
				next = e.Src
			}
			if e.DispatchKind != concfacts.SyncAliasKind || seen[next] {
				continue
			}
			seen[next] = true
			out = append(out, next)
		}
	}
	return out, nil
}

// approxAliasing is the certainty of an operation whose primitive the
// graph does not name.
var approxAliasing = concfacts.Approx(concfacts.ReasonAliasing)

// unknownReasonWords say why the graph does not name a primitive.
var unknownReasonWords = map[types.SyncUnknownReason]string{
	types.SyncNoOrigin:          "its value has no origin the graph finds",
	types.SyncIncompleteCallers: "only %d of %d callers pass one the graph names",
	types.SyncOpenCallers:       "it is a parameter of an exported or uncalled function",
	types.SyncNoHolder:          "no field or variable holds it",
	types.SyncNotFollowed:       "the graph does not follow this value",
}

// unknownOp is the words of a sync operation on line whose primitive the
// graph does not name (sync_unknown on a site there, of an operation the
// call performs).
func (t *tracer) unknownOp(sf *siteFacts, line int, callee types.Node) (string, bool) {
	for _, n := range sf.byLine[line] {
		for _, u := range n.SyncUnknown {
			names, ok := opCallees[u.Op]
			fit := !ok
			for _, nm := range names {
				fit = fit || strings.HasPrefix(callee.Name, nm)
			}
			w, known := syncWordsByOp[u.Op]
			if !fit || !known {
				continue
			}
			return w + ": which one the graph does not name (" + unknownReason(u) + ")", true
		}
	}
	return "", false
}

// unknownReason says in words why an operation's primitive is not named.
func unknownReason(u types.SyncUnknown) string {
	w, ok := unknownReasonWords[u.Reason]
	if !ok {
		return string(u.Reason)
	}
	if u.Reason == types.SyncIncompleteCallers {
		return fmt.Sprintf(w, u.CallersResolved, u.CallersTotal)
	}
	return w
}

// laterWords say when a runs_func callback runs.
var laterWords = map[types.SyncOp]string{
	types.SyncOnceDo:      "runs it once, later or now, without the caller's locks",
	types.SyncAfterFunc:   "runs it when the timer fires, in another goroutine, without the caller's locks",
	types.SyncWaitGroupGo: "runs it in a new goroutine, without the caller's locks",
}

// laterSteps are the callbacks a site makes run later (runs_func), each a
// step that opens the function and holds nothing.
func (t *tracer) laterSteps(sf *siteFacts, s *step, site types.Node) {
	for _, e := range sf.later[site.ID] {
		n, ok := t.g.nodes[e.Dst]
		if !ok {
			continue
		}
		c := &step{kind: kindRunsLater, node: n, body: s.body, file: s.file, line: s.line, std: laterWords[types.SyncOp(e.DispatchKind)],
			hop: s.hop, up: s.up}
		t.setOpen(c, n)
		s.children = append(s.children, c)
	}
}
