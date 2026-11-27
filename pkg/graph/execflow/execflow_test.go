package execflow

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// fixtureDir is the flow fixture: a worker pool fed by a channel, a select
// with a done signal, workers started with a named method, a call chain
// under a mutex, an interface call with two implementations, a recursive
// function and a test file that sends to the pool.
const fixtureDir = "testdata/flowfix"

var (
	fixtureOnce sync.Once
	fixtureDB   string
	fixtureErr  error
)

// fixture opens the graph of the fixture, built once per test binary.
func fixture(t *testing.T) persist.Store {
	t.Helper()
	fixtureOnce.Do(func() {
		out, err := tempDir()
		if err != nil {
			fixtureErr = err
			return
		}
		_, fixtureErr = buildpipe.Run(buildpipe.Options{SrcRoot: fixtureDir, OutDir: out,
			Languages: []string{"go"}, CKGVersion: "test", NoCache: true})
		fixtureDB = filepath.Join(out, "graph.db")
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	s, err := persist.OpenReadOnly(fixtureDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// trace runs q on the fixture and fails the test on an error.
func trace(t *testing.T, q Query) string {
	t.Helper()
	out, err := Trace(fixture(t), q)
	if err != nil {
		t.Fatalf("Trace(%+v): %v", q, err)
	}
	return out
}

// stepLine returns the first step line of out that contains every part,
// or fails.
func stepLine(t *testing.T, out string, parts ...string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		ok := true
		for _, p := range parts {
			if !strings.Contains(l, p) {
				ok = false
				break
			}
		}
		if ok {
			return l
		}
	}
	t.Fatalf("no line with %q in:\n%s", parts, out)
	return ""
}

// labelOf is the step label a step line starts with.
func labelOf(line string) string {
	return strings.Fields(line)[0]
}

// indexOf is the position of the first line holding part, or -1.
func indexOf(out, part string) int { return strings.Index(out, part) }

// TestForwardWorkerPool: from Run the flow goes in source order through
// the start of the workers (a goroutine running a named method), the send
// of a job (received in the worker's select, whose message runs handle
// under the pool's mutex) and the done signal.
func TestForwardWorkerPool(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Run", Budget: 60})
	// The workers' own loop comes first, under the goroutine start; the
	// message path is read from the send on.
	order := []string{"calls flowfix.NewPool", "calls flowfix.Pool.Start", "starts goroutine flowfix.Pool.Start#Goroutine",
		"calls flowfix.Pool.Submit", "sends on flowfix.Pool.jobs", "received at flowfix.Pool.work#CallSite",
		"calls flowfix.Pool.Stop", "sends on flowfix.Pool.done"}
	last := -1
	for _, w := range order {
		i := indexOf(out, w)
		if i < 0 || i < last {
			t.Fatalf("%q missing or out of order in:\n%s", w, out)
		}
		last = i
	}
	msg := out[indexOf(out, "received at flowfix.Pool.work#CallSite"):]
	for _, w := range []string{"calls flowfix.Pool.handle", "takes lock flowfix.Pool.mu#mutex", "calls flowfix.Pool.add"} {
		if indexOf(msg, w) < 0 {
			t.Fatalf("the message does not run %q:\n%s", w, out)
		}
	}
	add := stepLine(t, msg, "calls flowfix.Pool.add")
	lock := stepLine(t, msg, "takes lock flowfix.Pool.mu#mutex")
	if !strings.Contains(add, "[holding flowfix.Pool.mu#mutex since "+labelOf(lock)+"]") {
		t.Errorf("add does not run under the lock taken at %s: %s", labelOf(lock), add)
	}
	if got := stepLine(t, out, "received at flowfix.Pool.work#CallSite"); !strings.Contains(got, "~interleaving") {
		t.Errorf("a message crossing is approximated (interleaving): %s", got)
	}
	if !strings.Contains(msg, "runs in goroutines started by flowfix.Pool.Start (pool.go:30)") {
		t.Errorf("the receive does not say which goroutines run it:\n%s", out)
	}
	if !strings.Contains(out, "releases lock flowfix.Pool.mu#mutex when the function returns (defer)") {
		t.Errorf("the deferred release is not said:\n%s", out)
	}
}

// TestForwardSelectCases: a worker loop's select is one step with its cases
// as alternatives; a case names who wakes it, a done signal carries no
// value, and the handled code is under its case.
func TestForwardSelectCases(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Pool.work"})
	sel := stepLine(t, out, "select, waits for one of 2 cases:")
	jobs := stepLine(t, out, "receives from flowfix.Pool.jobs")
	done := stepLine(t, out, "receives from flowfix.Pool.done")
	if !strings.HasPrefix(labelOf(jobs), labelOf(sel)) || !strings.HasPrefix(labelOf(done), labelOf(sel)) {
		t.Errorf("the cases are not alternatives of the select %s: %q %q", labelOf(sel), jobs, done)
	}
	for _, want := range []string{"woken by flowfix.Pool.Submit", "a signal: the message carries no value", "woken by flowfix.Pool.Stop"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	handle := stepLine(t, out, "calls flowfix.Pool.handle")
	if !strings.HasPrefix(labelOf(handle), labelOf(jobs)+".") {
		t.Errorf("handle (%s) is not under the jobs case (%s)", labelOf(handle), labelOf(jobs))
	}
}

// TestForwardInterfaceBranches: an interface call lists the
// implementations as alternatives, cut to Branches with the count and the
// call that lists them all.
func TestForwardInterfaceBranches(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Sum"})
	call := stepLine(t, out, "calls flowfix.Shape.Area (interface), may run:")
	for _, impl := range []string{"runs flowfix.Rect.Area", "runs flowfix.Square.Area"} {
		if l := stepLine(t, out, impl); !strings.HasPrefix(labelOf(l), labelOf(call)) {
			t.Errorf("%s is not an alternative of %s", l, call)
		}
	}
	// Which implementation ranks first depends on the graph's ranks, so the
	// one cut is the other of the two.
	cut := trace(t, Query{Seed: "flowfix.Sum", Branches: 1})
	next := "flowfix.Square.Area"
	if strings.Contains(cut, "runs flowfix.Square.Area") {
		next = "flowfix.Rect.Area"
	}
	if !strings.Contains(cut, "+1 more alternatives (2 distinct in all); next: "+next) ||
		!strings.Contains(cut, "all: execution_flow qname=flowfix.Sum depth=1 branches=2 budget=200") {
		t.Errorf("the cut branch does not say how to list the rest:\n%s", cut)
	}
}

// TestForwardStops: recursion, depth and budget each stop the flow, and the
// answer says so with the call that continues.
func TestForwardStops(t *testing.T) {
	rec := trace(t, Query{Seed: "flowfix.Depth"})
	if !strings.Contains(rec, "recursion: flowfix.Depth is already running at the seed") || !strings.Contains(rec, "recursion 1") {
		t.Errorf("recursion not stated:\n%s", rec)
	}
	leaf := trace(t, Query{Seed: "flowfix.Pool.handle", Depth: 1})
	if strings.Contains(leaf, "not opened (depth): execution_flow qname=flowfix.Pool.add") {
		t.Errorf("a continuation is offered into a function with no steps:\n%s", leaf)
	}
	deep := trace(t, Query{Seed: "flowfix.Run", Depth: 1})
	if !strings.Contains(deep, "not opened (depth): execution_flow qname=") || !strings.Contains(deep, "depth ") {
		t.Errorf("depth stop not stated:\n%s", deep)
	}
	small := trace(t, Query{Seed: "flowfix.Run", Budget: 3})
	if n := len(regexp.MustCompile(`(?m)^\s*\d[\w.]* \S+:\d+ `).FindAllString(small, -1)); n != 3 {
		t.Errorf("budget 3 shows %d steps:\n%s", n, small)
	}
	if !strings.Contains(small, "more steps of the seed not shown (budget)") {
		t.Errorf("the budget cut is not stated:\n%s", small)
	}
}

// TestForwardFollowAndFocus: follow drops the links not asked for; focus
// keeps the steps that touch the field and the steps that lead to them.
func TestForwardFollowAndFocus(t *testing.T) {
	calls := trace(t, Query{Seed: "flowfix.Run", Follow: []Link{LinkCalls}})
	if strings.Contains(calls, "sends on") || strings.Contains(calls, "starts goroutine") {
		t.Errorf("follow=calls shows messages or goroutines:\n%s", calls)
	}
	focus := trace(t, Query{Seed: "flowfix.Run", Focus: "total", Depth: 5, Budget: 60})
	for _, want := range []string{"calls flowfix.Pool.Submit", "received at", "calls flowfix.Pool.handle", "writes flowfix.Pool.total"} {
		if !strings.Contains(focus, want) {
			t.Errorf("focus=total lost %q:\n%s", want, focus)
		}
	}
	if strings.Contains(focus, "calls flowfix.NewPool") || strings.Contains(focus, "calls flowfix.Pool.Stop") {
		t.Errorf("focus=total keeps steps that do not lead to total:\n%s", focus)
	}
	shallow := trace(t, Query{Seed: "flowfix.Run", Focus: "total", Depth: 2})
	if !strings.Contains(shallow, "within depth 2; execution_flow qname=flowfix.Run focus=total depth=3 opens more") {
		t.Errorf("an empty focus answer cut by depth does not say how to go deeper:\n%s", shallow)
	}
}

// TestBackwardWaysIn: backward from add, the caller holds the lock at the
// call, the caller's call runs because a message arrives, and the sender
// and its caller lead there; test code is left out unless asked for.
func TestBackwardWaysIn(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, Depth: 4})
	handle := stepLine(t, out, "flowfix.Pool.handle calls flowfix.Pool.add")
	if !strings.Contains(handle, "[holding flowfix.Pool.mu#mutex since ") {
		t.Errorf("the caller's lock at the call is not marked: %s", handle)
	}
	work := stepLine(t, out, "flowfix.Pool.work calls flowfix.Pool.handle")
	if labelOf(work) != labelOf(handle)+".1" {
		t.Errorf("work (%s) is not the way into handle (%s)", labelOf(work), labelOf(handle))
	}
	if !strings.Contains(out, "runs when a message arrives at flowfix.Pool.work#CallSite") {
		t.Errorf("the receive whose message runs the call is not noted:\n%s", out)
	}
	stepLine(t, out, "flowfix.Pool.Submit sends on flowfix.Pool.jobs, received at flowfix.Pool.work#CallSite")
	if strings.Contains(out, "flowfix.Pool.Start calls flowfix.Pool.work") {
		t.Errorf("the go line is listed as a plain call besides the goroutine:\n%s", out)
	}
	stepLine(t, out, "flowfix.Run calls flowfix.Pool.Submit")
	stepLine(t, out, "flowfix.Pool.Start starts goroutine flowfix.Pool.Start#Goroutine")
	if strings.Contains(out, "submitFromTest") || !strings.Contains(out, "test code 1.") {
		t.Errorf("test code is not left out and counted:\n%s", out)
	}
	with := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, Depth: 4, Tests: TestsInclude, Branches: 5})
	stepLine(t, with, "flowfix.submitFromTest calls flowfix.Pool.Submit")
}

// TestGoroutineLiteralHoldsNothing: a call in a goroutine literal is
// reached backward through the goroutine and its starter, not as a call of
// the function around the literal, and neither direction says the
// goroutine holds the starter's lock.
func TestGoroutineLiteralHoldsNothing(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, Depth: 1, Branches: 5})
	if strings.Contains(out, "flowfix.Pool.Flush calls flowfix.Pool.add") {
		t.Errorf("the literal's call is listed as a call of Flush:\n%s", out)
	}
	started := stepLine(t, out, "flowfix.Pool.Flush starts goroutine flowfix.Pool.Flush#Goroutine", "which runs flowfix.Pool.add")
	if strings.Contains(started, "[holding") {
		t.Errorf("the goroutine is said to hold Flush's lock: %s", started)
	}
	fwd := trace(t, Query{Seed: "flowfix.Pool.Flush"})
	if got := stepLine(t, fwd, "calls flowfix.Pool.add"); strings.Contains(got, "[holding") {
		t.Errorf("forward, the goroutine's call is said to hold Flush's lock: %s", got)
	}
	if got := stepLine(t, fwd, "starts goroutine flowfix.Pool.Flush#Goroutine"); !strings.Contains(got, "[holding flowfix.Pool.mu#mutex") {
		t.Errorf("the go statement itself runs under the lock: %s", got)
	}
}

// TestBackwardThroughInterface: an implementation is reached through the
// interface method its caller calls.
func TestBackwardThroughInterface(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Square.Area", Direction: Backward})
	stepLine(t, out, "flowfix.Sum calls flowfix.Shape.Area, which may run flowfix.Square.Area")
	stepLine(t, out, "flowfix.Draw calls flowfix.Sum")
}

// TestStatementPlaces: each step says what it sits in (a branch, a loop,
// after an early return, a deferred call), and the then and the else of
// one if are alternatives of one branch step, not a sequence.
func TestStatementPlaces(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Counter.Pick", Depth: 1})
	stepLine(t, out, "calls flowfix.Depth {if 37}", "return Depth(0)")
	stepLine(t, out, "calls flowfix.Depth {after return 38}", "return Depth(v)")
	branch := labelOf(stepLine(t, out, "if, one of:"))
	then := labelOf(stepLine(t, out, "then:"))
	els := labelOf(stepLine(t, out, "else:"))
	if then != branch+"a" || els != branch+"b" {
		t.Errorf("then %s and else %s are not the alternatives of the branch %s:\n%s", then, els, branch, out)
	}
	if hold := labelOf(stepLine(t, out, "calls flowfix.Counter.hold")); !strings.HasPrefix(hold, then+".") {
		t.Errorf("hold (%s) is not under the then (%s)", hold, then)
	}
	if store := labelOf(stepLine(t, out, "calls flowfix.Counter.store")); !strings.HasPrefix(store, els+".") {
		t.Errorf("store (%s) is not under the else (%s)", store, els)
	}
	fan := trace(t, Query{Seed: "flowfix.Counter.Fan"})
	stepLine(t, fan, "marks done the WaitGroup flowfix.Counter.wg (sync.WaitGroup.Done) {defer}")
}

// TestMultiplicityAndStandardLibrary: a goroutine start and a send in a
// loop say they happen once per iteration, a range receive handles every
// message until the channel is closed, and calls into sync, sync/atomic
// and time are said as what they do.
func TestMultiplicityAndStandardLibrary(t *testing.T) {
	fan := trace(t, Query{Seed: "flowfix.Counter.Fan"})
	stepLine(t, fan, "starts goroutine flowfix.Counter.Fan#Goroutine", "once per iteration of the loop at 20 (several may run at once)", "{for 20}")
	stepLine(t, fan, "sends on", "once per iteration of the loop at 20", "out <- it")
	for _, want := range []string{"adds to the WaitGroup flowfix.Counter.wg (sync.WaitGroup.Add) {for 20}",
		"waits for the WaitGroup flowfix.Counter.wg (sync.WaitGroup.Wait)", "- Add at branches.go:21", "- Done at branches.go:31",
		"atomically adds to flowfix.Counter.n", "sleeps (time.Sleep)", "the graph finds no origin for this channel"} {
		if !strings.Contains(fan, want) {
			t.Errorf("missing %q in:\n%s", want, fan)
		}
	}
	drain := trace(t, Query{Seed: "flowfix.Counter.Drain"})
	stepLine(t, drain, "receives each message from", "until it is closed", "for v := range ch")
}

// TestLockReleasedElsewhere: a lock a function takes and never releases
// says where it is released.
func TestLockReleasedElsewhere(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Counter.hold"})
	if !strings.Contains(out, "released elsewhere: flowfix.Counter.release (branches.go:55)") {
		t.Errorf("the release in another function is not named:\n%s", out)
	}
	back := trace(t, Query{Seed: "flowfix.Counter.hold", Direction: Backward})
	if !strings.Contains(back, "inside: takes lock flowfix.Counter.mu#mutex") || !strings.Contains(back, "released elsewhere: flowfix.Counter.release (branches.go:55)") {
		t.Errorf("backward, the seed's lock does not say where it is released:\n%s", back)
	}
	if strings.Contains(trace(t, Query{Seed: "flowfix.Pool.handle"}), "released elsewhere") {
		t.Error("a lock released in its own function is said to be released elsewhere")
	}
}

// TestPassedLiteral: the code of a literal passed to a call (a function
// of the build, a timer) sits under the step that passes it and does not hold the locks held where the
// literal is written; the passing call does.
func TestPassedLiteral(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Counter.Arm", Depth: 1})
	timer := stepLine(t, out, "calls flowfix.Counter.store", "c.store(1)")
	if strings.Contains(timer, "[holding") {
		t.Errorf("the timer callback is said to hold the lock: %s", timer)
	}
	arm := stepLine(t, out, "passes a function literal to time.AfterFunc (arms a timer that calls a function); it runs later, in another goroutine, without the caller's locks:")
	if !strings.Contains(arm, "[holding flowfix.Counter.mu#mutex since 1]") {
		t.Errorf("the passing call itself runs under the lock (the graph's fact, no ~branches): %s", arm)
	}
	// A literal passed to a function of the build is taken to run before
	// that returns, under the caller's locks (sort.Search, Once.Do).
	now := trace(t, Query{Seed: "flowfix.Counter.Locked", Depth: 1})
	lit := labelOf(stepLine(t, now, "passes a function literal to flowfix.withEach; flowfix.withEach calls it before it returns, under the caller's locks:"))
	store := stepLine(t, now, "calls flowfix.Counter.store", "c.store(v)")
	if !strings.HasPrefix(labelOf(store), lit+".") || !strings.Contains(store, "[holding flowfix.Counter.mu#mutex") {
		t.Errorf("a callback run inside the call does not hold the caller's lock: %s\n%s", store, now)
	}
	params := trace(t, Query{Seed: "flowfix.Counter.Apply", Depth: 1})
	if strings.Contains(params, "a function literal;") || strings.Contains(params, "passes a function literal") {
		t.Errorf("a function type in a parameter list is read as a literal:\n%s", params)
	}
}

// TestLiteralUses: a literal passed to a function of the build is printed
// by what that function does with it (literal_passed_to kind): called now
// under the caller's locks, started or kept without them, deferred, handed
// on to the goroutine that calls it, or called on one branch (~branches).
func TestLiteralUses(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Jobs.Run", Depth: 1})
	cases := []struct {
		words, touch string
		holds        bool
		lockMark     string
	}{
		{"flowfix.callNow calls it before it returns, under the caller's locks:", "j.touch(1)", true, "since 1]"},
		{"flowfix.startLater starts it in a goroutine; it runs concurrently, without the caller's locks ~interleaving:", "j.touch(2)", false, ""},
		{"flowfix.Jobs.keep keeps it; it runs when something calls it, without the caller's locks:", "j.touch(3)", false, ""},
		{"flowfix.deferIt defers it; it runs when flowfix.deferIt returns, under the caller's locks:", "j.touch(4)", true, "since 1]"},
		{"flowfix.handOn starts it in a goroutine; it runs concurrently, without the caller's locks ~interleaving:", "j.touch(5)", false, ""},
		{"flowfix.relay hands it on; the graph does not follow it further, so when it runs is not known (no locks assumed):", "j.touch(7)", false, ""},
		{"flowfix.maybe calls it before it returns, under the caller's locks ~branches:", "j.touch(6)", true, "since 1 ~branches]"},
	}
	for _, c := range cases {
		lit := labelOf(stepLine(t, out, "passes a function literal to", c.words))
		touch := stepLine(t, out, "calls flowfix.Jobs.touch", c.touch)
		switch {
		case !strings.HasPrefix(labelOf(touch), lit+"."):
			t.Errorf("%s is not under its literal %s", touch, lit)
		case c.holds && !strings.Contains(touch, "[holding flowfix.Jobs.mu#mutex "+c.lockMark):
			t.Errorf("%s should hold the caller's lock (%s)", touch, c.lockMark)
		case !c.holds && strings.Contains(touch, "[holding"):
			t.Errorf("%s should hold none of the caller's locks", touch)
		}
	}
}

// TestLiteralSent: a literal the callee sends on a channel runs in another
// goroutine, without the caller's locks.
func TestLiteralSent(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Poster.Post", Depth: 1})
	lit := labelOf(stepLine(t, out, "flowfix.Poster.queue sends it on a channel; another goroutine may run it, without the caller's locks ~interleaving:"))
	call := stepLine(t, out, "calls flowfix.sideA", "sideA(8)")
	if !strings.HasPrefix(labelOf(call), lit+".") || strings.Contains(call, "[holding") {
		t.Errorf("the sent literal's code is not under it without locks: %s", call)
	}
}

// TestTriggerAliasMark: a triggers edge whose send and receive may be on
// several channels is approximated (aliasing), and its channel is named
// without the mark.
func TestTriggerAliasMark(t *testing.T) {
	for _, c := range []struct {
		kind, channel string
		approx        bool
	}{
		{"p2p.Peer.closed", "p2p.Peer.closed", false},
		{"p2p.Peer.closed~aliasing", "p2p.Peer.closed", true},
	} {
		e := types.Edge{Type: types.EdgeTriggers, DispatchKind: c.kind, Confidence: types.ConfInferred}
		if got := triggerChannel(e); got != c.channel {
			t.Errorf("channel of %q = %q, want %q", c.kind, got, c.channel)
		}
		if got := certaintyOf(e) == approxAliasing; got != c.approx {
			t.Errorf("certainty of %q is aliasing = %v, want %v", c.kind, got, c.approx)
		}
	}
}

// TestGoroutineInSelectCase: a goroutine started in a select case runs its
// own code only, never the select around its go statement; a literal
// started or deferred there holds its own code.
func TestGoroutineInSelectCase(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Poster.Loop", Depth: 2})
	for _, start := range []string{"go p.drain()", "go func() {"} {
		g := labelOf(stepLine(t, out, "starts goroutine", start))
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), g+".") && strings.Contains(l, "select") {
				t.Errorf("the goroutine at %q runs the starter's select: %s", start, l)
			}
		}
	}
	for _, c := range []struct{ code, under string }{{"sideA(10)", "go func() {"}, {"sideA(11)", "defers a function literal"}} {
		parent := labelOf(stepLine(t, out, c.under))
		if l := stepLine(t, out, "calls flowfix.sideA", c.code); !strings.HasPrefix(labelOf(l), parent+".") {
			t.Errorf("%s is not under %s:\n%s", l, c.under, out)
		}
	}
}

// TestLocalLiteralCalls: a literal kept in a local variable is named as
// kept there, and a call through the variable shows the literal's steps
// under the locks held at the call; a call inside the literal itself is
// recursion.
func TestLocalLiteralCalls(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Poster.Local", Depth: 1})
	stepLine(t, out, "stores a function literal in visit")
	stepLine(t, out, "stores a function literal in again")
	call := labelOf(stepLine(t, out, "calls the function value visit (the function literal kept at line", "its code runs here:"))
	inner := stepLine(t, out, "calls flowfix.sideA", "sideA(x)")
	found := false
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), call+".") && strings.Contains(l, "sideA(x)") {
			found = strings.Contains(l, "[holding flowfix.Poster.mu#mutex")
		}
	}
	if !found {
		t.Errorf("the call through visit does not show the literal's code under the lock (first copy: %s):\n%s", inner, out)
	}
	stepLine(t, out, "calls the function value again (the function literal kept at line", "already running here (recursion)")
	stepLine(t, out, "calls the function value again (the function literal kept at line", "its code runs here:", "again(3)")
}

// TestReceiverAndSiteContext: a receive site seen from the sender is a step
// of the receiving function, with its loop and its own lock; a sync call
// takes the locks of its own site when a builtin shares the line; an
// implementation run by an interface call holds what is certainly held
// at the call; a channel lock's receive names its releases; a deferred
// cancel through a function value says it cancels.
func TestReceiverAndSiteContext(t *testing.T) {
	put := trace(t, Query{Seed: "flowfix.Box.Put"})
	recv := stepLine(t, put, "received at", "flowfix.Box.Take")
	if !strings.Contains(recv, "{for 31}") || !strings.Contains(recv, "[holding flowfix.Box.mu#mutex]") {
		t.Errorf("the receive site lacks the receiver's loop or lock: %s", recv)
	}
	flag := stepLine(t, trace(t, Query{Seed: "flowfix.Box.Flag"}), "atomically stores")
	if !strings.Contains(flag, "[holding flowfix.Box.mu#mutex since 1]") {
		t.Errorf("the atomic store lost its lock: %s", flag)
	}
	impl := stepLine(t, trace(t, Query{Seed: "flowfix.Box.Measure"}), "runs flowfix.")
	if !strings.Contains(impl, "[holding flowfix.Box.mu#mutex since 1]") {
		t.Errorf("an implementation does not hold the call's certain lock: %s", impl)
	}
	if enter := trace(t, Query{Seed: "flowfix.Box.Enter"}); !strings.Contains(enter, "a channel used as a lock") ||
		!strings.Contains(enter, "woken when a holder releases it, at contexts.go:20, contexts.go:49") {
		t.Errorf("the channel lock's releases are not named:\n%s", enter)
	}
	stepLine(t, trace(t, Query{Seed: "flowfix.Timed"}), "calls a function value (cancels, through", "defer cancel()")
}

// TestJumpsAndClauses: a step after a continue or a break on its path is
// marked like one after an early return; a return in one case clause
// skips the code after it in that clause and after the switch, not another
// clause.
func TestJumpsAndClauses(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Skips"})
	for _, c := range []struct{ code, want, not string }{
		{"total += sideA(x)", "{for 76, after continue 78, after break 81}", ""},
		{"total += sideA(1)", "after return 88}", ""},
		{"total += sideA(2)", "{case 85}", "after return"},
		{"return total + sideA(3)", "{after return 88}", "continue"},
	} {
		l := stepLine(t, out, "calls flowfix.sideA", c.code)
		if !strings.Contains(l, c.want) || c.not != "" && strings.Contains(l, c.not) {
			t.Errorf("want %q (not %q) in %s", c.want, c.not, l)
		}
	}
}

// TestCancelThroughField: a cancel kept in a field is followed (flows_to
// sync): the done channel's receive names where it is called.
func TestCancelThroughField(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Svc.Wait"})
	if !strings.Contains(out, "is cancelled: cancel at literals.go:116") {
		t.Errorf("the cancel through the field is not named:\n%s", out)
	}
}

// TestFunctionValueCalls: a call through a function-typed field is a call
// of a function value with the functions it may hold, never a call of the
// function it is written in (the graph's self-loop marker), forward or
// backward.
func TestFunctionValueCalls(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Hooks.Height"})
	if strings.Contains(out, "calls flowfix.Hooks.Height") {
		t.Errorf("the marker is printed as a call of the function itself:\n%s", out)
	}
	stepLine(t, out, "calls a function value", "return h.height()")
	back := trace(t, Query{Seed: "flowfix.Hooks.Height", Direction: Backward})
	if strings.Contains(back, "flowfix.Hooks.Height calls flowfix.Hooks.Height") {
		t.Errorf("backward, the marker is printed as a self call:\n%s", back)
	}
}

// TestDeferredLiteralAndGoroutineLocks: a deferred literal's code sits
// under one step that runs when the function returns, so its release does
// not end the lock for the steps after it; a lock taken inside a goroutine
// shows, with the steps it covers.
func TestDeferredLiteralAndGoroutineLocks(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Hooks.Defers"})
	lit := labelOf(stepLine(t, out, "defers a function literal; it runs when the function returns:"))
	if rel := labelOf(stepLine(t, out, "releases lock flowfix.Hooks.mu#mutex")); !strings.HasPrefix(rel, lit+".") {
		t.Errorf("the release (%s) is not under the deferred literal (%s):\n%s", rel, lit, out)
	}
	if d := stepLine(t, out, "calls flowfix.Depth", "Depth(1)"); !strings.Contains(d, "[holding flowfix.Hooks.mu#mutex") {
		t.Errorf("the deferred release ends the lock too early: %s", d)
	}
	spawn := trace(t, Query{Seed: "flowfix.Hooks.Spawn"})
	stepLine(t, spawn, "takes lock flowfix.Hooks.mu#mutex")
	if d := stepLine(t, spawn, "calls flowfix.Depth", "Depth(2)"); !strings.Contains(d, "[holding flowfix.Hooks.mu#mutex") {
		t.Errorf("the goroutine's own lock is not marked: %s", d)
	}
}

// TestReleaseOnReturningBranch: a release on a branch that returns does not
// end the lock for the code after the branch, forward or backward; a call
// in a literal passed to another call holds nothing of its caller's locks.
func TestReleaseOnReturningBranch(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Hooks.Unregister"})
	if d := stepLine(t, out, "calls flowfix.Depth", "Depth(3)"); !strings.Contains(d, "[holding flowfix.Hooks.mu#mutex") {
		t.Errorf("the lock is said to be released after the returning branch: %s", d)
	}
	back := trace(t, Query{Seed: "flowfix.Depth", Direction: Backward, Branches: 10, Depth: 1})
	if d := stepLine(t, back, "flowfix.Hooks.Unregister calls flowfix.Depth"); !strings.Contains(d, "[holding flowfix.Hooks.mu#mutex") {
		t.Errorf("backward, the lock is said to be released: %s", d)
	}
	snap := trace(t, Query{Seed: "flowfix.Hooks.Snapshot"})
	if d := stepLine(t, snap, "calls flowfix.Depth", "Depth(4)"); strings.Contains(d, "[holding") {
		t.Errorf("a lock taken and released on a returning branch is said to be held after it: %s", d)
	}
	store := trace(t, Query{Seed: "flowfix.Counter.store", Direction: Backward, Branches: 10, Depth: 1})
	if !strings.Contains(store, "the call is in a function literal at line 72 that is passed to a call (it runs when that calls it)") {
		t.Errorf("backward, a call in a passed literal does not say it runs later:\n%s", store)
	}
	for _, l := range strings.Split(store, "\n") {
		if strings.Contains(l, "flowfix.Counter.Arm calls flowfix.Counter.store") && strings.Contains(l, "[holding") {
			t.Errorf("a call in a passed literal is said to hold the caller's lock: %s", l)
		}
	}
}

// TestNoSendIsNotAConclusion: a receive woken by a close names the closer
// (the graph records closes), and a close wakes the receives on it.
func TestNoSendIsNotAConclusion(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Hooks.Wait"})
	if !strings.Contains(out, "closed by flowfix.Hooks.Close (hooks.go:") {
		t.Errorf("the closer that wakes the receive is not named:\n%s", out)
	}
	cl := trace(t, Query{Seed: "flowfix.Hooks.Close"})
	stepLine(t, cl, "closes flowfix.Hooks.done, which wakes every receive on it:")
}

// TestCutsAreCounted: implementations only the type allows are counted
// with the call that lists them, and the call a cut names lists exactly
// the alternatives it counts.
func TestCutsAreCounted(t *testing.T) {
	sum := trace(t, Query{Seed: "flowfix.Sum"})
	if !strings.Contains(sum, "+1 more alternatives its type allows that the value flow does not reach: find_callees qname=flowfix.Shape.Area depth=1") {
		t.Errorf("the implementation the type alone allows is not counted:\n%s", sum)
	}
	tight := trace(t, Query{Seed: "flowfix.Sum", Budget: 2})
	if !strings.Contains(tight, "+1 more alternatives not shown (budget): execution_flow qname=flowfix.Sum depth=1 branches=2 budget=200") {
		t.Errorf("alternatives the budget hides are not counted with a listing call:\n%s", tight)
	}
	cut := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, Branches: 1, Depth: 1})
	const call = "execution_flow qname=flowfix.Pool.add direction=backward depth=1 branches=2"
	if !strings.Contains(cut, "all: "+call) {
		t.Fatalf("the cut line does not name the listing call:\n%s", cut)
	}
	all := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, Depth: 1, Branches: 2})
	if n := len(regexp.MustCompile(`(?m)^  \d+ `).FindAllString(all, -1)); n != 2 {
		t.Errorf("the named call lists %d alternatives, the cut counts 2:\n%s", n, all)
	}
}

// TestEntryPoint: a backward step with nothing before it says it is an
// entry point instead of offering a continuation that returns nothing.
func TestEntryPoint(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Counter.hold", Direction: Backward, Depth: 1})
	pick := stepLine(t, out, "flowfix.Counter.Pick calls flowfix.Counter.hold")
	_ = pick
	if strings.Contains(out, "not opened (depth): execution_flow qname=flowfix.Counter.Pick") || !strings.Contains(out, "an entry point") {
		t.Errorf("a caller nothing calls is offered as a continuation:\n%s", out)
	}
	tests := trace(t, Query{Seed: "flowfix.testOnly", Direction: Backward, Depth: 1})
	stepLine(t, tests, "flowfix.viaTestsOnly calls flowfix.testOnly")
	if strings.Contains(tests, "not opened (depth)") || !strings.Contains(tests, "an entry point") {
		t.Errorf("a caller only test code calls is offered as a continuation:\n%s", tests)
	}
}

// TestGraphFacts: select statements, timers, contexts, sync.Once and the
// evaluation of a go statement are read from the graph's facts.
func TestGraphFacts(t *testing.T) {
	tick := trace(t, Query{Seed: "flowfix.Ticker", Depth: 1})
	stepLine(t, tick, "select, waits for one of 2 cases:")
	stepLine(t, tick, "select with a default (does not block)")
	if !strings.Contains(tick, "sent by the runtime when the timer") {
		t.Errorf("a timer receive is not said to be sent by the runtime:\n%s", tick)
	}
	cancels := trace(t, Query{Seed: "flowfix.Cancels", Depth: 1})
	if !strings.Contains(cancels, "is cancelled: cancel at facts.go:") {
		t.Errorf("the done receive does not name where cancel is called:\n%s", cancels)
	}
	initOnce := trace(t, Query{Seed: "flowfix.Init"})
	stepLine(t, initOnce, "runs once, by flowfix.once")
	stepLine(t, initOnce, "runs it once", "flowfix.setup")
	stepLine(t, trace(t, Query{Seed: "flowfix.ErrText"}), "a builtin interface")
	spawn := trace(t, Query{Seed: "flowfix.Hooks.Spawner", Depth: 1})
	pick := stepLine(t, spawn, "calls flowfix.Hooks.pick")
	if !strings.Contains(pick, "[holding flowfix.Hooks.mu#mutex") || strings.Contains(labelOf(pick), ".") {
		t.Errorf("the go statement's receiver is not evaluated by the starter under its lock: %s\n%s", pick, spawn)
	}
}

// TestTesterClasses: a select is one step with every case (a case whose
// body receives, a timer case, a context case); a goroutine's waits and
// sync calls show; a return on another branch does not mark a step; a
// range expression runs once; a literal step carries its own place; caps
// and depth 0 are said.
func TestTesterClasses(t *testing.T) {
	polls := trace(t, Query{Seed: "flowfix.Polls", Depth: 1})
	if n := strings.Count(polls, "select, waits for one of"); n != 1 || !strings.Contains(polls, "select, waits for one of 2 cases:") {
		t.Errorf("the select is split or loses cases (%d select steps):\n%s", n, polls)
	}
	watch := trace(t, Query{Seed: "flowfix.Watch", Depth: 1})
	for _, want := range []string{"marks done the WaitGroup: which one the graph does not name (it is a parameter of an exported or uncalled function)", "select, waits for one of 2 cases:", "sent by the runtime when the timer"} {
		if !strings.Contains(watch, want) {
			t.Errorf("missing %q in:\n%s", want, watch)
		}
	}
	if strings.Contains(watch, "nothing in the build sends") {
		t.Errorf("a receive whose channel has no origin is said to have no sender:\n%s", watch)
	}
	br := trace(t, Query{Seed: "flowfix.Branches", Depth: 1})
	if s := stepLine(t, br, "calls flowfix.Depth", "Depth(9)"); strings.Contains(s, "after return") {
		t.Errorf("a return on the other branch marks the step: %s", s)
	}
	stepLine(t, br, "calls flowfix.Depth", "Depth(8)", "after return")
	if s := stepLine(t, br, "calls flowfix.rangeOf"); strings.Contains(s, "{for") {
		t.Errorf("the range expression is marked as in the loop: %s", s)
	}
	if s := stepLine(t, trace(t, Query{Seed: "flowfix.Pick2", Depth: 1}), "calls flowfix.Depth", "Depth(11)"); strings.Contains(s, "after return") {
		t.Errorf("a return of the function marks a step of a literal written in it: %s", s)
	}
	if s := stepLine(t, trace(t, Query{Seed: "flowfix.LitIf", Depth: 1}), "passes a function literal"); strings.Contains(s, "{if") {
		t.Errorf("a place inside the literal leaks onto the literal step: %s", s)
	}
	wide := trace(t, Query{Seed: "flowfix.Run", Budget: 300})
	if !strings.Contains(wide, "budget 300 capped at 200") {
		t.Errorf("the budget cap is not said:\n%s", wide)
	}
	if lit := trace(t, Query{Seed: "flowfix.Pool.Flush", Depth: OwnStepsOnly}); !strings.Contains(lit, "calls flowfix.Pool.add") {
		t.Errorf("depth 0 hides the code of a goroutine literal written in the seed:\n%s", lit)
	}
	own := trace(t, Query{Seed: "flowfix.Run", Depth: OwnStepsOnly})
	if regexp.MustCompile(`(?m)^\s+\d+\.\d+ `).MatchString(own) {
		t.Errorf("depth 0 opens steps:\n%s", own)
	}
}

// TestSteering: from shows the paths from a node to the seed, via keeps
// the paths through one, and a cut branch point says how many distinct
// nodes it has and names the next ones.
func TestSteering(t *testing.T) {
	from := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, From: "flowfix.Run"})
	stepLine(t, from, "flowfix.Run calls flowfix.Pool.Submit")
	stepLine(t, from, "flowfix.Run calls flowfix.Pool.Start")
	if strings.Contains(from, "flowfix.Pool.Flush") {
		t.Errorf("from=Run keeps a path that does not start at Run:\n%s", from)
	}
	via := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, Via: "flowfix.Pool.Submit", Depth: 4})
	stepLine(t, via, "flowfix.Pool.Submit sends on flowfix.Pool.jobs")
	if strings.Contains(via, "starts goroutine flowfix.Pool.Start#Goroutine") || strings.Contains(via, "flowfix.Pool.Flush") {
		t.Errorf("via=Submit keeps paths that do not go through Submit:\n%s", via)
	}
	cut := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, Branches: 1, Depth: 1})
	if !strings.Contains(cut, "+1 more alternatives (2 distinct in all); next: ") || !strings.Contains(cut, "keep one with via=<name> or from=<name>") {
		t.Errorf("the cut line does not count and name what it cut:\n%s", cut)
	}
	if _, err := Trace(fixture(t), Query{Seed: "flowfix.Run", From: "flowfix.Pool.add"}); !errors.Is(err, ErrBadQuery) {
		t.Errorf("from forward: %v, want ErrBadQuery", err)
	}
}

// TestFocusBackward: backward, focus notes where the steps that lead to the
// seed read or write the field, and keeps the paths to them.
func TestFocusBackward(t *testing.T) {
	out := trace(t, Query{Seed: "flowfix.Pool.add", Direction: Backward, Focus: "jobs"})
	if !strings.Contains(out, "reads flowfix.Pool.jobs at line 60") || !strings.Contains(out, "reads flowfix.Pool.jobs at line 38") {
		t.Errorf("the reads of jobs on the way in are not noted:\n%s", out)
	}
	if strings.Contains(out, "flowfix.Pool.Flush") {
		t.Errorf("a path that never touches jobs is kept:\n%s", out)
	}
}

// TestSeedMisses: a name that names several declarations lists them, one
// that names none says so, and a field is not a place to start.
func TestSeedMisses(t *testing.T) {
	if out := trace(t, Query{Seed: "Area"}); !strings.Contains(out, "names 4 declarations") ||
		!strings.Contains(out, "execution_flow qname=flowfix.Rect.Area") {
		t.Errorf("ambiguous seed:\n%s", out)
	}
	if out := trace(t, Query{Seed: "flowfix.Nothing"}); !strings.Contains(out, "names no symbol") {
		t.Errorf("unknown seed:\n%s", out)
	}
	if out := trace(t, Query{Seed: "flowfix.Pool.total"}); !strings.Contains(out, "find_references qname=flowfix.Pool.total") {
		t.Errorf("field seed:\n%s", out)
	}
}

// TestDeterministic: the same query gives the same bytes, on one store and
// on another opened from the same graph.
func TestDeterministic(t *testing.T) {
	queries := []Query{{Seed: "flowfix.Run", Budget: 60}, {Seed: "flowfix.Pool.add", Direction: Backward, Depth: 4}, {Seed: "flowfix.Sum"}}
	for _, q := range queries {
		first := trace(t, q)
		for i := 0; i < 3; i++ {
			if again := trace(t, q); again != first {
				t.Fatalf("%+v run %d differs:\n%s\n---\n%s", q, i, first, again)
			}
		}
	}
}

// TestBadQueries: a query the tool cannot run is an error wrapping
// ErrBadQuery.
func TestBadQueries(t *testing.T) {
	for _, q := range []Query{{}, {Seed: "x", Direction: "up"}, {Seed: "x", Follow: []Link{"calls", "jumps"}}, {Seed: "x", Tests: "only"}} {
		if _, err := Trace(fixture(t), q); !errors.Is(err, ErrBadQuery) {
			t.Errorf("%+v: err %v, want ErrBadQuery", q, err)
		}
	}
	if ls, err := ParseLinks("calls, messages,calls"); err != nil || len(ls) != 2 {
		t.Errorf("ParseLinks = %v, %v", ls, err)
	}
	if _, err := ParseLinks("calls,jumps"); !errors.Is(err, ErrBadQuery) {
		t.Errorf("ParseLinks of an unknown link: %v", err)
	}
}
