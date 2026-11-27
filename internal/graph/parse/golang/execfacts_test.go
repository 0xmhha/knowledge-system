package golang_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// execFactsLabel names a node of testdata/concurrency/execfacts.go in the
// edge strings of TestExecFacts: a statement by its type and line, a
// declaration by its qualified name without the package.
func execFactsLabel(n types.Node) string {
	switch n.Type {
	case types.NodeCallSite, types.NodeGoroutine, types.NodeSwitchStmt, types.NodeChannel:
		return fmt.Sprintf("%s:%d", n.Type, n.StartLine)
	}
	return strings.TrimPrefix(n.QualifiedName, "mutex_fixture.")
}

// TestExecFacts checks the facts an execution-order reading needs: who
// closes a channel and the receives a close wakes, select statements and
// their cases, the locks held at each site, and the operations on sync
// primitives (testdata/concurrency/execfacts.go).
func TestExecFacts(t *testing.T) {
	g, err := gop.LoadAndResolveWith("testdata/concurrency", gop.LoadOptions{LockPropagation: true})
	if err != nil {
		t.Fatalf("LoadAndResolveWith: %v", err)
	}
	byID := map[string]types.Node{}
	got := map[string]bool{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
		if strings.HasSuffix(n.FilePath, "execfacts.go") && n.SubKind != "" {
			got[fmt.Sprintf("%s %s", execFactsLabel(n), n.SubKind)] = true
		}
	}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		if !strings.HasSuffix(src.FilePath, "execfacts.go") {
			continue
		}
		got[fmt.Sprintf("%s %s %s %s", execFactsLabel(src), e.Type, execFactsLabel(dst), e.DispatchKind)] = true
	}
	want := []string{
		// 1. Closing a channel: the closer, its site, the receives it wakes.
		"CallSite:25 close",
		"Crew.Stop closes Crew.quit field",
		"Crew.Stop triggers CallSite:33 mutex_fixture.Crew.quit",
		"Crew.Stop triggers CallSite:44 mutex_fixture.Crew.quit",
		"Crew.Stop triggers CallSite:46 mutex_fixture.Crew.quit",
		// 2. Select statements and their cases.
		"SwitchStmt:32 select_default",
		"SwitchStmt:43 select",
		"SwitchStmt:32 encloses CallSite:33 cond",
		"SwitchStmt:32 encloses CallSite:35 cond",
		"SwitchStmt:43 encloses CallSite:44 cond",
		"SwitchStmt:32 body_writes Crew.n case",
		// 3. Locks held at a site: in the function, and in a callee.
		"CallSite:25 runs_under_lock Crew.mu#mutex ",
		"CallSite:26 runs_under_lock Crew.mu#mutex ",
		"CallSite:68 runs_under_lock Crew.mu#mutex propagated",
		// 4. Operations on sync primitives, by declaration.
		"CallSite:52 operates_on Crew.Run.wg wg_add",
		"CallSite:54 operates_on Crew.Run.wg wg_done",
		"CallSite:55 operates_on Crew.wg wg_add",
		"CallSite:57 operates_on Crew.Run.wg wg_wait",
		"CallSite:58 operates_on Crew.once once_do",
		"CallSite:58 runs_func Crew.init once_do",
		"CallSite:59 operates_on Crew.cond cond_signal",
		"CallSite:68 operates_on Crew.cond cond_broadcast",
		"CallSite:60 operates_on Crew.hits atomic_add",
		"CallSite:61 operates_on Crew.count atomic_add",
		"CallSite:73 operates_on Crew.Cancel.ctx ctx_make",
		"CallSite:73 operates_on Crew.Cancel.cancel cancel_make",
		"CallSite:74 operates_on Crew.Cancel.cancel cancel",
		"CallSite:75 operates_on Crew.Cancel.ctx ctx_done",
		"CallSite:75 operates_on Crew.Cancel.ctx done_recv",
		"CallSite:76 operates_on Crew.Cancel.t timer_make",
		"CallSite:76 runs_func Crew.init after_func",
		"CallSite:77 operates_on Crew.Cancel.t timer_stop",
		"CallSite:78 operates_on CallSite:78 timer_recv",
		"CallSite:79 operates_on Crew.Cancel.tk timer_make",
		"CallSite:80 operates_on Crew.Cancel.tk timer_recv",
		"CallSite:81 operates_on Crew.Cancel.tk timer_stop",
		// 5. A go statement in a literal passed to a call runs in the callee.
		"Goroutine:89 literal_passed_to run arg",
		// A deferred close closes when the function returns.
		"Crew.Finish closes Crew.quit field",
		"Crew.Finish triggers CallSite:33 mutex_fixture.Crew.quit",
		"CallSite:99 runs_under_lock Crew.mu#mutex ",
		// A variable made twice has both origins.
		"Crew.Reorg closes Channel:107 make",
		"Crew.Reorg closes Channel:112 make",
		// Deferred calls run at the return, with what is held then.
		"CallSite:130 runs_under_lock Crew.mu#mutex ",
		"CallSite:131 runs_under_lock Crew.mu#mutex ",
		"CallSite:140 runs_under_lock Crew.mu#mutex ",
		// A primitive passed as a parameter, by the declarations it comes from.
		"CallSite:143 operates_on Crew.Workers.wg wg_done",
		"CallSite:143 operates_on Crew.wg wg_done",
		"CallSite:158 runs_func Crew.init after_func",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %q", w)
		}
	}
	for _, bad := range []string{
		"CallSite:24 runs_under_lock Crew.mu#mutex ", // the Lock call itself is not under mu
		"Crew.Work encloses CallSite:33 ",            // a case site lies in its select
	} {
		if got[bad] {
			t.Errorf("unexpected %q", bad)
		}
	}
	if t.Failed() {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			t.Log(k)
		}
	}
}

// TestExecFactsDeferredAndFollowed checks, on the end of
// testdata/concurrency/execfacts.go, that deferred work registered after
// an Unlock runs under the lock (its literal's call, the calls in it, a
// close), that two calls starting at one byte stay two sites, that an
// inline timer's receive names the call that makes the timer, and that a
// primitive reached through a parameter names all its sources or none.
// A CallSite is labelled by its line and the start of its source text.
func TestExecFactsDeferredAndFollowed(t *testing.T) {
	const fixture = "testdata/concurrency/execfacts.go"
	src, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	g, err := gop.LoadAndResolveWith("testdata/concurrency", gop.LoadOptions{LockPropagation: true})
	if err != nil {
		t.Fatalf("LoadAndResolveWith: %v", err)
	}
	const textWidth = 18
	label := func(n types.Node) string {
		if n.Type != types.NodeCallSite || n.EndByte > len(src) {
			return execFactsLabel(n)
		}
		text, _, _ := strings.Cut(string(src[n.StartByte:n.EndByte]), "\n")
		if len(text) > textWidth {
			text = text[:textWidth]
		}
		return fmt.Sprintf("%d[%s]", n.StartLine, text)
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		if !strings.HasSuffix(src.FilePath, "execfacts.go") {
			continue
		}
		got[fmt.Sprintf("%s %s %s %s", label(src), e.Type, label(dst), e.DispatchKind)] = true
	}
	for _, n := range g.Nodes {
		if !strings.HasSuffix(n.FilePath, "execfacts.go") {
			continue
		}
		for _, u := range n.SyncUnknown {
			got[fmt.Sprintf("%s unknown %s %s %d/%d", label(n), u.Op, u.Reason, u.CallersResolved, u.CallersTotal)] = true
		}
	}
	tests := []struct {
		name string
		edge string
		want bool
	}{
		{"deferred literal's call runs under mu", "187[func(start time.Ti] runs_under_lock Crew.mu#mutex ", true},
		{"a call in the deferred literal runs under mu", "188[p.flush()] runs_under_lock Crew.mu#mutex ", true},
		{"its argument is evaluated at the defer, under mu", "189[time.Now()] runs_under_lock Crew.mu#mutex ", true},
		{"a deferred close runs under mu", "190[close(p.quit)] runs_under_lock Crew.mu#mutex ", true},
		{"a method on new(T) runs under mu", "191[new(Crew).flush()] runs_under_lock Crew.mu#mutex ", true},
		{"the builtin new is no site", "191[new(Crew)] runs_under_lock Crew.mu#mutex ", false},
		{"a lock taken in a deferred literal, under mu", "201[crewMu.Lock()] runs_under_lock Crew.mu#mutex ", true},
		{"its unlock, under both", "203[crewMu.Unlock()] runs_under_lock crewMu ", true},
		{"its unlock, under mu", "203[crewMu.Unlock()] runs_under_lock Crew.mu#mutex ", true},
		{"a literal that unlocks runs under mu", "212[func() { p.mu.Unlo] runs_under_lock Crew.mu#mutex ", true},
		{"a defer registered before it runs after the unlock", "210[p.flush()] runs_under_lock Crew.mu#mutex ", false},
		{"an inline timer names the call that makes it", "217[<-time.NewTimer(ti] operates_on 217[time.NewTimer(time] timer_recv", true},
		{"a context from a caller that returns it is unresolved: no edge", "224[<-ctx.Done()] operates_on Crew.Rest.ctx done_recv", false},
		{"nor for its Done call", "224[ctx.Done()] operates_on Crew.Rest.ctx ctx_done", false},
		{"a context every caller holds in a variable", "155[<-ctx.Done()] operates_on Crew.Arm.ctx done_recv", true},
		{"a timer a caller assigns to a local", "240[t.Reset(time.Secon] operates_on Crew.Rearm.t timer_reset", true},
		{"an atomic counter a caller passes as a field", "248[atomic.AddInt64(n,] operates_on Crew.hits atomic_add", true},
		{"the unresolved receive is marked on its site", "224[<-ctx.Done()] unknown done_recv incomplete_callers 1/2", true},
		{"and its Done call", "224[ctx.Done()] unknown ctx_done incomplete_callers 1/2", true},
		{"a named site carries no mark", "155[<-ctx.Done()] unknown done_recv incomplete_callers 1/1", false},
		{"a context made into a parameter is held by the field it is stored in", "260[context.WithCancel] operates_on crewIter.ctx ctx_make", true},
		{"its cancel function, in a new local, keeps the local", "260[context.WithCancel] operates_on newCrewIter.cancel cancel_make", true},
	}
	for _, tc := range tests {
		if got[tc.edge] != tc.want {
			t.Errorf("%s: %q present=%v, want %v", tc.name, tc.edge, got[tc.edge], tc.want)
		}
	}
	if t.Failed() {
		keys := make([]string, 0, len(got))
		for k := range got {
			if strings.Contains(k, "runs_under_lock") || strings.Contains(k, "operates_on") {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			t.Log(k)
		}
	}
}
