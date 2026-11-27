package execflow

import (
	"sort"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// tree.go holds the steps of an answer and the rules that order, choose and
// cut them.

// stepKind is the link that led to a step.
type stepKind int

// The kinds of step, forward then backward. The order of the forward kinds
// is their order on one line (lineRank).
const (
	kindRecv stepKind = iota
	kindLock
	kindCall
	kindIface
	kindFuncValue
	kindGo
	kindSend
	kindClose
	kindRead
	kindWrite
	kindUnlock
	kindStd
	kindLiteral
	kindRunsLater
	kindSelect
	kindBranch
	kindBranchPart
	kindImpl
	kindReceivedAt
	kindSeed
	kindCalledBy
	kindCalledVia
	kindCalledByValue
	kindStartedBy
	kindMessageFrom
)

// cutReason says why a step was not opened.
type cutReason string

// The stop conditions an answer states.
const (
	cutDepth     cutReason = "depth"
	cutBudget    cutReason = "budget"
	cutRecursion cutReason = "recursion"
	cutOutside   cutReason = "outside the build (calls left out)"
	cutTest      cutReason = "test code"
)

// cutOrder is the order the summary counts cuts in.
var cutOrder = []cutReason{cutDepth, cutBudget, cutRecursion, cutOutside, cutTest}

// heldLock is a lock held where a step runs: the lock, the step that took
// it (nil when it was read from a caller's lock edges) and where.
type heldLock struct {
	lock  types.Node
	taken *step
	file  string
	line  int
	// until, for a lock taken on a branch that returns, is the last line
	// of that branch: it is not held after it.
	until int
	// fact marks a lock the graph says is held at the step's site
	// (runs_under_lock); byCaller one a caller holds (propagated).
	fact, byCaller bool
}

// step is one line of the narrative.
type step struct {
	kind stepKind
	// node is what the step names: the callee, the goroutine, the channel
	// or lock, the field, the caller; body is the function or goroutine
	// whose source holds the line.
	node, body types.Node
	file       string
	line       int
	// via names a second node: the interface method of kindCalledVia, the
	// goroutine of kindStartedBy, the receive site of kindRecv,
	// kindReceivedAt and kindMessageFrom; below is what a backward step
	// leads to; channel names the channel of kindMessageFrom.
	via, below types.Node
	channel    string
	certainty  concfacts.Certainty
	byType     bool
	deferred   bool
	held       []heldLock
	notes      []string
	// children run inside the step, in order; alts are alternatives.
	children, alts []*step
	// altsLeft counts alternatives cut by Branches; altsCall lists them all.
	altsLeft int
	altsCall string
	// open is the body the step opens (forward) or whose predecessors it
	// lists (backward); cut says why it was not opened.
	open  *types.Node
	cut   cutReason
	again *step // the step already running open (recursion)
	hop   int
	// up is the step that opened the body this step is in.
	up *step
	// places are the statements the step sits in, outer first;
	// afterReturn the lines of the earlier returns that may skip it,
	// rangeHead that it is the receive of a range over a channel; inDefer
	// marks a deferred call; partName names the part of a kindBranchPart
	// (then or else); std says what a standard-library call does.
	places      []place
	afterReturn []exit
	rangeHead   bool
	inDefer     bool
	partName    string
	lit         *litSpan
	// localLit, on a call through a variable of the function, is the step
	// of the literal kept in it; recursive that the call is in that
	// literal's own code.
	localLit  *step
	recursive bool
	closure   bool
	// releaseUntil, on a release in a branch that returns, is the last
	// line of that branch: the lock is still held after it.
	releaseUntil int
	std          string
	// altsTop names the first alternatives cut by Branches, altsKinds
	// counts the distinct nodes among all of them.
	altsTop      []string
	altsDistinct int
	// Set when the answer is chosen and printed.
	shown      bool
	hiddenAlts int
	steered    bool
	hidden     int
	label      string
	match      bool
}

// lineRank orders the steps of one line.
func lineRank(k stepKind) int { return int(k) }

// sortSteps orders the steps of one body: line, then lineRank, then the
// printed name and id of the node.
func sortSteps(g *reader, ss []*step) {
	sort.SliceStable(ss, func(i, j int) bool {
		a, b := ss[i], ss[j]
		switch {
		case a.line != b.line:
			return a.line < b.line
		case a.kind != b.kind:
			return lineRank(a.kind) < lineRank(b.kind)
		case g.name(a.node) != g.name(b.node):
			return g.name(a.node) < g.name(b.node)
		}
		return a.node.ID < b.node.ID
	})
}

// sortAlts orders alternatives by the branch rule: in the build before
// outside it, production before test, higher graph rank (PageRank), then
// printed name, file, line and id.
func sortAlts(g *reader, ss []*step) {
	sort.SliceStable(ss, func(i, j int) bool {
		a, b := ss[i], ss[j]
		if oa, ob := outside(a.node), outside(b.node); oa != ob {
			return !oa
		}
		if ta, tb := g.test(a.node), g.test(b.node); ta != tb {
			return !ta
		}
		if a.node.PageRank != b.node.PageRank {
			return a.node.PageRank > b.node.PageRank
		}
		if na, nb := g.name(a.node), g.name(b.node); na != nb {
			return na < nb
		}
		if a.file != b.file {
			return a.file < b.file
		}
		if a.line != b.line {
			return a.line < b.line
		}
		return a.node.ID < b.node.ID
	})
}

// walk calls f on every step under roots, in pre-order (alternatives
// before children).
func walk(roots []*step, f func(s, parent *step)) {
	var visit func(ss []*step, parent *step)
	visit = func(ss []*step, parent *step) {
		for _, s := range ss {
			f(s, parent)
			visit(s.alts, s)
			visit(s.children, s)
		}
	}
	visit(roots, nil)
}

// Classes of step for the budget, in the order it keeps them.
const (
	classCrossing = iota // goroutines, messages, locks, focused reads and writes
	classCall            // calls into the build
	classOutside         // calls outside the build
)

// stepClass is the class of s: what changes who runs or what is held comes
// first, then calls in the build, then calls outside it.
func stepClass(s *step) int {
	switch s.kind {
	case kindCall, kindIface, kindFuncValue, kindImpl, kindCalledBy, kindCalledVia, kindCalledByValue:
		if outside(s.node) {
			return classOutside
		}
		return classCall
	}
	return classCrossing
}

// choose marks the steps an answer shows, at most budget, best first: a
// step is a candidate once the step above it is shown, and candidates are
// taken by level (how many steps opened lie between it and the seed; an
// alternative is on its step's level forward, one deeper backward), then
// class, then the order they became candidates. A shown step counts its
// steps that were not shown (hidden). It returns how many were shown.
func choose(roots []*step, budget int, altsDeeper bool) int {
	type cand struct {
		s          *step
		level, seq int
	}
	var frontier []cand
	seq := 0
	push := func(s *step, level int) {
		frontier = append(frontier, cand{s, level, seq})
		seq++
	}
	less := func(a, b cand) bool {
		if a.level != b.level {
			return a.level < b.level
		}
		if ca, cb := stepClass(a.s), stepClass(b.s); ca != cb {
			return ca < cb
		}
		return a.seq < b.seq
	}
	for _, r := range roots {
		push(r, 0)
	}
	shown := 0
	for len(frontier) > 0 && shown < budget {
		best := 0
		for i := 1; i < len(frontier); i++ {
			if less(frontier[i], frontier[best]) {
				best = i
			}
		}
		c := frontier[best]
		frontier = append(frontier[:best], frontier[best+1:]...)
		c.s.shown = true
		shown++
		altLevel := c.level
		if altsDeeper {
			altLevel++
		}
		for _, a := range c.s.alts {
			push(a, altLevel)
		}
		for _, ch := range c.s.children {
			push(ch, c.level+1)
		}
	}
	walk(roots, func(s, _ *step) {
		if !s.shown {
			return
		}
		for _, c := range s.alts {
			if !c.shown {
				s.hiddenAlts++
			}
		}
		for _, c := range s.children {
			if !c.shown {
				s.hidden++
			}
		}
	})
	return shown
}

// hiddenRoots counts the roots that were not shown.
func hiddenRoots(roots []*step) int {
	n := 0
	for _, s := range roots {
		if !s.shown {
			n++
		}
	}
	return n
}

// pruneSteered keeps the steps on paths through a steered step (via,
// from): the steps that lead to one, and, under it, all its steps.
func pruneSteered(roots []*step) []*step {
	var keep func(ss []*step) []*step
	keep = func(ss []*step) []*step {
		var out []*step
		for _, s := range ss {
			if s.steered {
				out = append(out, s)
				continue
			}
			s.alts = keep(s.alts)
			s.children = keep(s.children)
			if len(s.alts) > 0 || len(s.children) > 0 {
				out = append(out, s)
			}
		}
		return out
	}
	return keep(roots)
}

// distances returns, for every step under roots, how many steps lie
// between it and the nearest steered step under it (0: it is steered;
// noDistance when none is).
func distances(roots []*step) map[*step]int {
	out := map[*step]int{}
	var visit func(s *step) int
	visit = func(s *step) int {
		best := noDistance
		if s.steered {
			best = 0
		}
		for _, c := range append(append([]*step(nil), s.alts...), s.children...) {
			best = min(best, visit(c)+1)
		}
		out[s] = best
		return best
	}
	for _, s := range roots {
		visit(s)
	}
	return out
}

// noDistance is the distance of a step no steered step is under.
const noDistance = 1 << 20

// prune keeps the steps that match (focus) and the steps that lead to them;
// it returns the kept roots.
func prune(roots []*step) []*step {
	var keep func(ss []*step) []*step
	keep = func(ss []*step) []*step {
		var out []*step
		for _, s := range ss {
			s.alts = keep(s.alts)
			s.children = keep(s.children)
			if s.match || len(s.alts) > 0 || len(s.children) > 0 {
				out = append(out, s)
			}
		}
		return out
	}
	return keep(roots)
}
