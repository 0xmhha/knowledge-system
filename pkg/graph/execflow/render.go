package execflow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// render.go prints an answer: the summary, then the chosen steps.

// Layout of a step line.
const (
	indentUnit   = "  "
	textLimit    = 110
	textEllipsis = "..."
	altLetters   = "abcdefghijklmnopqrstuvwxyz"
)

// render prints roots after choosing the steps the budget keeps.
func (t *tracer) render(roots []*step, res store.NameResolution) string {
	if t.q.Direction == Backward {
		t.root.shown = true
		choose(t.root.alts, t.q.Budget, true)
	} else {
		t.root.children = roots
		choose(roots, t.q.Budget, false)
	}
	t.label(roots)
	var b strings.Builder
	t.summary(&b, roots, res)
	for _, s := range roots {
		t.print(&b, s, 0)
	}
	if t.q.Direction == Forward {
		if n := hiddenRoots(roots); n > 0 {
			fmt.Fprintf(&b, "... %d more steps of the seed not shown (budget): %s\n", n, t.moreBudget())
		}
		if len(roots) == 0 {
			b.WriteString("No steps: the graph records no call, goroutine, channel operation or lock in it" + t.focusWords())
			if len(t.cuts[cutDepth]) > 0 && t.q.Depth < MaxDepth {
				fmt.Fprintf(&b, " within depth %d; %s depth=%d opens more", t.q.Depth, t.call(*t.root.open), t.q.Depth+1)
			}
			b.WriteString(".\n")
		}
	}
	return b.String()
}

// focusWords names the focus in an empty answer.
func (t *tracer) focusWords() string {
	if t.q.Focus == "" {
		return ""
	}
	return " that touches " + t.q.Focus
}

// label numbers the shown steps: forward "2", "2.1", and "2a" for an
// alternative; backward the seed is "0" and each way in is numbered under
// the step it leads to ("1", "1.2").
func (t *tracer) label(roots []*step) {
	var visit func(ss []*step, prefix string, alt bool)
	visit = func(ss []*step, prefix string, alt bool) {
		n := 0
		for _, s := range ss {
			if !s.shown {
				continue
			}
			switch {
			case alt && t.q.Direction == Forward:
				s.label = prefix + string(altLetters[n%len(altLetters)])
			case prefix == "":
				s.label = strconv.Itoa(n + 1)
			default:
				s.label = prefix + "." + strconv.Itoa(n+1)
			}
			n++
			visit(s.alts, s.label, true)
			visit(s.children, s.label, false)
		}
	}
	if t.q.Direction == Backward {
		t.root.label = "0"
		visit(t.root.alts, "", true)
		return
	}
	visit(roots, "", false)
}

// summary prints the two to four lines before the steps.
func (t *tracer) summary(b *strings.Builder, roots []*step, res store.NameResolution) {
	seed := *t.root.open
	fmt.Fprintf(b, "%s %s from %s (%s, %s:%d-%d); follows %s", t.q.ToolName, t.q.Direction, t.g.name(seed), seed.Type,
		seed.FilePath, seed.StartLine, seed.EndLine, joinLinks(t.q.Follow))
	if t.q.Focus != "" && t.q.Direction == Forward {
		fmt.Fprintf(b, "; focus %s (its reads, writes and guarding locks, and the steps that lead to them)", t.q.Focus)
	}
	if t.q.Focus != "" && t.q.Direction == Backward {
		fmt.Fprintf(b, "; focus %s (the steps that lead here and read or write it)", t.q.Focus)
	}
	if t.q.Via != "" {
		fmt.Fprintf(b, "; only paths through %s", t.q.Via)
	}
	if t.q.From != "" {
		fmt.Fprintf(b, "; paths from %s, shortest first", t.q.From)
	}
	if len(t.q.capped) > 0 {
		b.WriteString("; " + strings.Join(t.q.capped, ", "))
	}
	if note := res.SeedNote(); note != "" {
		b.WriteString("; " + note)
	}
	b.WriteString(".\n")
	shown := 0
	walk(roots, func(s, _ *step) {
		if s.shown && s.kind != kindSeed {
			shown++
		}
	})
	fmt.Fprintf(b, "%d of %d steps shown, %d branch points; %s.\n", shown, t.built, t.branchPoints, t.cutWords())
	if t.q.Direction == Backward {
		b.WriteString("Read: each numbered step leads to the step it is under (1 to the seed 0, 1.2 to 1); steps at one level are alternatives, in the build, production, higher rank first.\n")
	} else {
		b.WriteString("Read: steps run in source-line order; an indented step runs inside the one above it; 2a, 2b are alternatives, in the build, production, higher rank first.\n")
	}
	b.WriteString("Marks: {for N} repeats with the loop at line N; {if N}, {else N}, {case N} runs on that branch; {after return N}, {after continue N}, {after break N}, {after goto N} runs unless the return or jump at N is taken; {defer} runs when the function returns; a function literal that is deferred, passed, stored or returned holds its code under it, with when it runs: under the locks held where it is written when it is deferred or the callee calls or defers it before it returns, else without them; " +
		"~aliasing, ~branches, ~interleaving = approximated (told apart by declaration; true on some paths; timing unknown); ~by type = allowed by the type alone; [holding L since N] = lock L held there (the graph's must-hold fact at the site; ~branches when read by line order).\n")
}

// cutWords says what was not opened, by stop condition.
func (t *tracer) cutWords() string {
	var parts []string
	for _, r := range cutOrder {
		if n := len(t.cuts[r]); n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", r, n))
		}
	}
	if len(parts) == 0 {
		return "nothing cut"
	}
	return "not opened: " + strings.Join(parts, ", ")
}

// print prints s and the shown steps under it.
func (t *tracer) print(b *strings.Builder, s *step, depth int) {
	if !s.shown {
		return
	}
	indent := strings.Repeat(indentUnit, depth)
	fmt.Fprintf(b, "%s%s %s:%d %s", indent, s.label, s.file, s.line, t.words(s))
	if m := t.marks(s); m != "" {
		b.WriteString(" " + m)
	}
	if text := clip(t.g.textAt(s.body, s.line)); text != "" {
		b.WriteString(" | " + text)
	}
	b.WriteString("\n")
	for _, n := range t.notes(s) {
		fmt.Fprintf(b, "%s%s- %s\n", indent, indentUnit, n)
	}
	for _, a := range s.alts {
		t.print(b, a, depth+1)
	}
	for _, c := range s.children {
		t.print(b, c, depth+1)
	}
}

// clip trims a source line to textLimit runes.
func clip(text string) string {
	r := []rune(text)
	if len(r) <= textLimit {
		return text
	}
	return string(r[:textLimit]) + textEllipsis
}

// words says what s does, in the words an agent acts on.
func (t *tracer) words(s *step) string {
	name := t.g.name(s.node)
	switch s.kind {
	case kindCall:
		return "calls " + name
	case kindIface:
		if strings.HasPrefix(s.node.QualifiedName, builtinPrefix) {
			return "calls " + name + " (a builtin interface: any type that implements it may run; the graph does not list them)"
		}
		if len(s.alts) == 0 && s.altsLeft == 0 {
			return "calls " + name + " (interface; the graph has no implementation it may run)"
		}
		return "calls " + name + " (interface), may run:"
	case kindFuncValue:
		if l := s.localLit; l != nil {
			if s.recursive {
				return fmt.Sprintf("calls the function value %s (the function literal kept at line %d), which is already running here (recursion)", l.lit.storedIn, l.line)
			}
			return fmt.Sprintf("calls the function value %s (the function literal kept at line %d); its code runs here:", l.lit.storedIn, l.line)
		}
		w := "calls a function value"
		if s.node.ID != s.body.ID {
			w = "calls the function value " + name
		}
		switch {
		case len(s.alts) == 0 && s.altsLeft == 0 && s.std != "":
			return w + " (" + s.std + ")"
		case len(s.alts) == 0 && s.altsLeft == 0:
			return w + " (the graph finds no function it may hold)"
		}
		return w + ", may run:"
	case kindImpl:
		if s.closure {
			return "runs a function literal written in " + name
		}
		return "runs " + name
	case kindGo:
		w := "starts goroutine " + name
		if l, ok := innerLoop(s.places); ok {
			w += fmt.Sprintf(", once per iteration of the loop at %d (several may run at once)", l.StartLine)
		}
		return w
	case kindSend:
		w := "sends on " + t.channelName(s.node)
		if l, ok := innerLoop(s.places); ok {
			w += fmt.Sprintf(" once per iteration of the loop at %d", l.StartLine)
		}
		if len(s.alts) > 0 || s.altsLeft > 0 {
			w += ", received at:"
		}
		return w
	case kindClose:
		w := "closes " + t.channelName(s.node)
		if len(s.alts) > 0 || s.altsLeft > 0 {
			w += ", which wakes every receive on it:"
		}
		return w
	case kindReceivedAt:
		return "received at " + t.g.name(s.via) + " in " + name + ", the message runs:"
	case kindRecv:
		w := "receives from " + t.channelName(s.node)
		if s.rangeHead {
			w = "receives each message from " + t.channelName(s.node) + " until it is closed"
		}
		if len(s.children) > 0 {
			w += ", the message runs:"
		}
		return w
	case kindStd:
		return s.std + " (" + name + ")"
	case kindLiteral:
		return t.literalWords(s.lit)
	case kindBranch:
		return "if, one of:"
	case kindBranchPart:
		return s.partName + ":"
	case kindSelect:
		if s.node.SubKind == types.SubKindSelectDefault {
			return fmt.Sprintf("select with a default (does not block), one of %d cases or the default:", len(s.alts))
		}
		return fmt.Sprintf("select, waits for one of %d cases:", len(s.alts))
	case kindRunsLater:
		return s.std + ": " + name
	case kindLock:
		return "takes lock " + name + lockWords(s.node)
	case kindUnlock:
		if s.deferred {
			return "releases lock " + name + " when the function returns (defer)"
		}
		return "releases lock " + name
	case kindRead:
		return "reads " + name
	case kindWrite:
		return "writes " + name
	case kindSeed:
		return "seed " + name
	case kindCalledBy:
		return name + " calls " + t.g.name(s.below)
	case kindCalledVia:
		return name + " calls " + t.g.name(s.via) + ", which may run " + t.g.name(s.below)
	case kindCalledByValue:
		return name + " calls a function value that may hold " + t.g.name(s.below)
	case kindStartedBy:
		if s.node.ID == s.via.ID {
			return "goroutine " + name + " runs " + t.g.name(s.below)
		}
		return name + " starts goroutine " + t.g.name(s.via) + ", which runs " + t.g.name(s.below)
	case kindMessageFrom:
		if t.g.closesAt(s.node, s.line) {
			return name + " closes " + s.channel + ", which wakes the receive at " + t.g.name(s.via)
		}
		return name + " sends on " + s.channel + ", received at " + t.g.name(s.via)
	}
	return name
}

// builtinPrefix starts the qualified name of a builtin interface's method
// (builtin.error.Error).
const builtinPrefix = "builtin."

// channelName is the printed name of a channel, or of the field or
// variable it is kept in.
func (t *tracer) channelName(n types.Node) string { return t.g.name(n) }

// lockWords says that a lock is a channel used as one.
func lockWords(n types.Node) string {
	if n.Type == types.NodeChannel {
		return " (a channel used as a lock)"
	}
	return ""
}

// marks are the certainty and lock context of s.
func (t *tracer) marks(s *step) string {
	var parts []string
	if w := placeWords(s.places, s.afterReturn, s.inDefer); w != "" {
		parts = append(parts, w)
	}
	if m := certaintyMark(s.certainty); m != "" && s.kind != kindSelect {
		parts = append(parts, m)
	}
	if s.byType {
		parts = append(parts, approxPrefix+byTypeMark)
	}
	for _, h := range s.held {
		if (s.kind == kindLock || s.kind == kindUnlock) && h.lock.ID == s.node.ID {
			continue
		}
		parts = append(parts, t.heldMark(h))
	}
	return strings.Join(parts, " ")
}

// heldMark is the mark of one lock held where a step runs: from the
// graph's must-hold fact at the site, held by a caller, or (no site)
// inferred by line order and marked so.
func (t *tracer) heldMark(h heldLock) string {
	name := t.g.name(h.lock)
	since := ""
	switch {
	case h.taken != nil && h.taken.label != "":
		since = " since " + h.taken.label
	case h.line > 0:
		since = fmt.Sprintf(" since %s:%d", h.file, h.line)
	}
	switch {
	case h.byCaller:
		return "[holding " + name + ", held by a caller]"
	case h.fact:
		return "[holding " + name + since + "]"
	}
	return "[holding " + name + since + " " + certaintyMark(lockContextCertainty) + "]"
}

// notes are the lines under s: what the graph says about it, why it was
// not opened and how to continue.
func (t *tracer) notes(s *step) []string {
	out := append([]string(nil), s.notes...)
	switch s.cut {
	case cutDepth, cutBudget:
		if s.open != nil {
			out = append(out, fmt.Sprintf("not opened (%s): %s", s.cut, t.call(*s.open)))
		}
	case cutRecursion:
		out = append(out, t.again(s))
	case cutOutside:
		out = append(out, "outside the build: not opened")
	case cutTest:
		out = append(out, "test code: not opened (tests=include opens it)")
	}
	switch {
	case s.hidden > 0 && s.open != nil:
		call := t.call(*s.open)
		if s.hidden > DefaultBudget {
			// The default budget would cut the continuation short of them.
			call += fmt.Sprintf(" budget=%d", min(s.hidden, MaxBudget))
		}
		out = append(out, fmt.Sprintf("%d more steps here not shown (budget): %s", s.hidden, call))
	case s.hidden > 0:
		out = append(out, fmt.Sprintf("%d more steps here not shown (budget): %s", s.hidden, t.moreBudget()))
	}
	if s.hiddenAlts > 0 {
		call := s.altsCall
		if call == "" {
			call = t.flowCall(s.body, len(s.alts))
		}
		out = append(out, fmt.Sprintf("+%d more alternatives not shown (budget): %s", s.hiddenAlts, call))
	}
	if s.altsLeft > 0 {
		out = append(out, t.cutLine(s))
	}
	return out
}

// cutTopShown is how many cut alternatives the cut line names.
const cutTopShown = 3

// cutLine says what Branches cut at s: how many, how many distinct nodes
// the branch point has, the next few by the branch order, and how to
// steer to one of them or list them all.
func (t *tracer) cutLine(s *step) string {
	line := fmt.Sprintf("+%d more alternatives (%d distinct in all)", s.altsLeft, s.altsDistinct)
	if len(s.altsTop) > 0 {
		line += "; next: " + strings.Join(s.altsTop, ", ")
	}
	if t.q.Direction == Backward {
		line += "; keep one with via=<name> or from=<name>"
	} else {
		line += "; keep one with via=<name>"
	}
	return line + "; all: " + s.altsCall
}

// again says where the body of a recursive step already runs.
func (t *tracer) again(s *step) string {
	at := "the seed"
	if s.again != nil && s.again != t.root && s.again.label != "" {
		at = "step " + s.again.label
	}
	if s.kind == kindReceivedAt {
		return "recursion: " + t.g.name(s.node) + " is already on the path; its receive is at " + at
	}
	return "recursion: " + t.g.name(*s.open) + " is already running at " + at
}

// flowCall is the call that lists every one of the alts alternatives at
// one branch point of n: this tool on n, one hop deep, with that many
// branches and room for them.
func (t *tracer) flowCall(n types.Node, alts int) string {
	c := fmt.Sprintf("%s qname=%s", t.q.ToolName, t.g.name(n))
	if t.q.Direction == Backward {
		c += " direction=" + string(Backward)
	}
	c += fmt.Sprintf(" depth=1 branches=%d", min(alts, MaxBranches))
	if t.q.Direction == Forward || alts > DefaultBudget {
		c += fmt.Sprintf(" budget=%d", MaxBudget)
	}
	return c
}

// moreBudget is the call that shows more of this answer: a larger budget,
// or, at the cap, opening a step's name (the answer cannot grow).
func (t *tracer) moreBudget() string {
	if t.q.Budget >= MaxBudget {
		return fmt.Sprintf("the budget is at its cap (%d); call again with a step's name as qname", MaxBudget)
	}
	return fmt.Sprintf("%s budget=%d", t.call(*t.root.open), min(MaxBudget, t.q.Budget*2))
}

// call is the call that continues the flow from n.
func (t *tracer) call(n types.Node) string {
	c := fmt.Sprintf("%s qname=%s", t.q.ToolName, t.g.name(n))
	if t.q.Direction == Backward {
		c += " direction=" + string(Backward)
	}
	if t.q.Focus != "" && t.q.Direction == Forward {
		c += " focus=" + t.q.Focus
	}
	return c
}
