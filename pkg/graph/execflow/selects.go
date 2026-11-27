package execflow

import (
	"sort"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// selects.go builds select steps from the graph's select statements
// (schema 1.44): a SwitchStmt of sub kind select or select_default, whose
// cases are the send and receive sites it encloses as cond. A select is one
// step with every case as an alternative, each case's code under it; a case
// whose receive has no step of its own (a timer, a context's done channel,
// a channel the value flow does not trace) gets one from its site.

// selectStmts are the select statements of st with their case lines in
// order.
func (st *structure) selectStmts() map[string][]int {
	out := map[string][]int{}
	for line, sel := range st.selectCases {
		out[sel.ID] = append(out[sel.ID], line)
	}
	for id := range out {
		sort.Ints(out[id])
	}
	return out
}

// groupSelectsByFact turns the steps on the lines of each select statement
// of body into one select step, in every list of steps.
func (t *tracer) groupSelectsByFact(body types.Node, st *structure, ss []*step) []*step {
	return t.groupSelectsIn(body, st, ss, map[string]bool{})
}

// groupSelectsIn is groupSelectsByFact inside the select statements done
// (a case's code is grouped by the selects nested in it).
func (t *tracer) groupSelectsIn(body types.Node, st *structure, ss []*step, done map[string]bool) []*step {
	cases := st.selectStmts()
	var out []*step
	for i := 0; i < len(ss); {
		s := ss[i]
		sel, ok := t.outerSelect(body, st, cases, s.line, done)
		if !ok {
			s.children = t.groupSelectsIn(body, st, s.children, done)
			out = append(out, s)
			i++
			continue
		}
		j := i
		for j < len(ss) && ss[j].line >= sel.StartLine && ss[j].line <= sel.EndLine {
			j++
		}
		step, before := t.selectStep(body, st, sel, cases[sel.ID], ss[i:j], done)
		out = append(out, before...)
		out = append(out, step)
		i = j
	}
	return out
}

// outerSelect is the outermost select statement of body whose lines hold
// line. A goroutine's body is its own code: a select around its go
// statement is the starter's, not one of its steps.
func (t *tracer) outerSelect(body types.Node, st *structure, cases map[string][]int, line int, done map[string]bool) (types.Node, bool) {
	var best types.Node
	found := false
	lo, hi := bodySpan(body)
	for _, s := range st.stmts {
		if _, ok := cases[s.ID]; !ok || done[s.ID] || line < s.StartLine || line > s.EndLine || s.StartLine < lo || s.EndLine > hi {
			continue
		}
		if !found || s.EndLine-s.StartLine > best.EndLine-best.StartLine {
			best, found = s, true
		}
	}
	return best, found
}

// selectStep is the step of select sel with the steps on its lines: each
// case is an alternative headed by its channel operation, with the steps
// of the case under it. Steps above the first case (none in Go source, but
// a call written on the select line) are returned to go before it.
func (t *tracer) selectStep(body types.Node, st *structure, sel types.Node, caseLines []int, ss []*step, done map[string]bool) (*step, []*step) {
	inner := map[string]bool{sel.ID: true}
	for id := range done {
		inner[id] = true
	}
	first := ss[0]
	heads := make([]*step, len(caseLines))
	var before []*step
	caseIndex := func(line int) int {
		k := -1
		for i, l := range caseLines {
			if l <= line {
				k = i
			}
		}
		return k
	}
	rest := make([][]*step, len(caseLines))
	for _, s := range ss {
		k := caseIndex(s.line)
		switch {
		case k < 0:
			before = append(before, s)
		case heads[k] == nil && s.line == caseLines[k] && (s.kind == kindRecv || s.kind == kindSend):
			heads[k] = s
		default:
			rest[k] = append(rest[k], s)
		}
	}
	step := &step{kind: kindSelect, node: sel, body: body, file: first.file, line: sel.StartLine,
		hop: first.hop, up: first.up, held: first.held, places: st.placesOf(sel.StartLine, bodyLo(body), bodyHi(body)),
		afterReturn: returnsBefore(first.afterReturn, sel.StartLine)}
	for k, l := range caseLines {
		h := heads[k]
		if h == nil {
			h = t.caseFromSite(body, st, sel, l, first)
		}
		h.children = t.groupSelectsIn(body, st, append(h.children, rest[k]...), inner)
		sortSteps(t.g, h.children)
		step.alts = append(step.alts, h)
	}
	t.branchPoints++
	return step, before
}

// caseFromSite makes the step of a case whose channel operation has no
// step: from the site the select encloses on that line.
func (t *tracer) caseFromSite(body types.Node, st *structure, sel types.Node, line int, like *step) *step {
	var site types.Node
	for _, c := range st.children[sel.ID] {
		if c.part == partCond && c.node.Type == types.NodeCallSite && c.node.StartLine == line && (c.node.SubKind == siteRecv || c.node.SubKind == siteSend) {
			site = c.node
			break
		}
	}
	k := kindRecv
	if site.SubKind == siteSend {
		k = kindSend
	}
	s := &step{kind: k, node: site, via: site, body: body, file: body.FilePath, line: line, hop: like.hop, up: like.up, held: like.held, places: like.places}
	if k == kindRecv {
		if err := t.wokenBy(s); err == nil && len(s.notes) == 0 {
			s.notes = append(s.notes, untraced)
		}
	}
	return s
}
