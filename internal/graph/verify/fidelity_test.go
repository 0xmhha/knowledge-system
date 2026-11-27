package verify

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// fidelityFixture is the parser's fixture of constructs a graph once got
// wrong. The truth of this package is computed without the parser's code, so
// a graph of the fixture built by the parser must agree with it: a
// disagreement is a defect of one of the two.
const fidelityFixture = "../parse/golang/testdata/fidelity"

// memGraph is a GraphReader over a resolved graph held in memory.
type memGraph struct {
	nodes []types.Node
	edges []types.Edge
}

func (g memGraph) AllNodes() ([]types.Node, error) { return g.nodes, nil }

func (g memGraph) QueryEdgesByType(t string) ([]types.Edge, error) {
	var out []types.Edge
	for _, e := range g.edges {
		if string(e.Type) == t {
			out = append(out, e)
		}
	}
	return out, nil
}

func verifyFixture(t *testing.T, opts gop.LoadOptions) Report {
	t.Helper()
	root, err := filepath.Abs(fidelityFixture)
	if err != nil {
		t.Fatalf("abs %s: %v", fidelityFixture, err)
	}
	opts.Tests = true // the truth loads test files, as a build does
	rg, err := gop.LoadAndResolveWith(root, opts)
	if err != nil {
		t.Fatalf("build the fixture graph: %v", err)
	}
	r, err := RunGo(root, memGraph{nodes: rg.Nodes, edges: rg.Edges}, Options{Samples: 50})
	if err != nil {
		t.Fatalf("RunGo: %v", err)
	}
	return r
}

// TestRunGo_FixtureGraphAgreesWithTruth: the graph the parser builds for the
// fixture has every declaration, call, type reference and statement link the
// truth finds, and no statement link the truth does not have.
func TestRunGo_FixtureGraphAgreesWithTruth(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{})
	if r.Decls.Missing != 0 {
		t.Errorf("declarations missing from the graph: %v", r.Decls.MissingSamples)
	}
	if r.Decls.OtherSpan != 0 {
		t.Errorf("declarations whose node does not span them, first token through trailing comment: %v", r.Decls.OtherSpanSamples)
	}
	// accessed_under_lock: no edge the source does not support, and of the
	// true pairs only the one in a function with a goto, which the
	// extractor leaves out by rule, is missing.
	if r.UnderLock.Unsupported != 0 {
		t.Errorf("accessed_under_lock edges the source does not support: %v", r.UnderLock.UnsupportedSamples)
	}
	if want := []string{"locks/ledger.go:21:jumped -> locks/ledger.go:7:mu"}; !reflect.DeepEqual(r.UnderLock.MissingSamples, want) {
		t.Errorf("true accessed_under_lock pairs missing: %v, want %v", r.UnderLock.MissingSamples, want)
	}
	for _, rel := range []struct {
		name string
		s    RelationStats
	}{
		{"static calls", r.StaticCalls}, {"interface calls", r.InterfaceCalls}, {"type refs", r.TypeRefs},
		{"outside calls", r.OutsideCalls}, {"outside type refs", r.OutsideTypeRefs},
	} {
		t.Run(rel.name, func(t *testing.T) {
			if rel.s.Truth == 0 {
				t.Fatalf("the truth holds no %s for the fixture", rel.name)
			}
			if rel.s.Missing != 0 || rel.s.Misresolved != 0 || rel.s.EndpointMissing != 0 {
				t.Errorf("%s: truth %d, missing %d %v, misresolved %d %v, endpoint missing %d", rel.name, rel.s.Truth,
					rel.s.Missing, rel.s.MissingSamples, rel.s.Misresolved, rel.s.MisresolvedSamples, rel.s.EndpointMissing)
			}
		})
	}
	if r.Unexpected.Calls != 0 || r.Unexpected.TypeRefs != 0 {
		t.Errorf("edges the truth does not have: calls %v, type refs %v", r.Unexpected.CallSamples, r.Unexpected.TypeRefSamples)
	}
	for rel, s := range r.Statements {
		t.Run(rel, func(t *testing.T) {
			if s.Missing != 0 || s.Unexpected != 0 {
				t.Errorf("%s: truth %d, matched %d, missing %v, unexpected %v", rel, s.Truth, s.Matched, s.MissingSamples, s.UnexpectedSamples)
			}
		})
	}
	// The constructs of the 2026-11-26 comparison are in the truth, not
	// only in the graph.
	if s := r.Statements[string(types.EdgeEncloses)]; s.Matched == 0 {
		t.Errorf("no encloses link was compared")
	}
}

// TestRunGo_OutsideScope: with a package of the module left out of the
// graph, the truth counts the calls and type references into it as leaving
// the covered files, and the graph has them as edges to External nodes.
func TestRunGo_OutsideScope(t *testing.T) {
	const outside = "scoped/out/"
	r := verifyFixture(t, gop.LoadOptions{Skip: func(rel string) bool { return strings.HasPrefix(rel, outside) }})
	for _, rel := range []struct {
		name string
		s    RelationStats
		want string // a pair that only exists when the package is outside
	}{
		{"outside calls", r.OutsideCalls, "Helper"},
		{"outside type refs", r.OutsideTypeRefs, "T"},
	} {
		if rel.s.Missing != 0 || rel.s.Truth == 0 {
			t.Errorf("%s: truth %d, missing %d %v", rel.name, rel.s.Truth, rel.s.Missing, rel.s.MissingSamples)
		}
	}
	found := map[string]bool{}
	root, _ := filepath.Abs(fidelityFixture)
	truth, err := collectTruth(root, map[string]bool{"scoped/in/in.go": true}, nil)
	if err != nil {
		t.Fatalf("collectTruth: %v", err)
	}
	for _, p := range append(truth.OutsideCalls, truth.OutsideTypeRefs...) {
		found[p.To] = true
	}
	for _, want := range []string{"ckgfidelity.test/scoped/out.Helper", "ckgfidelity.test/scoped/out.(T).Twice", "ckgfidelity.test/scoped/out.T"} {
		if !found[want] {
			t.Errorf("the truth does not count %s as outside the covered files; got %v", want, found)
		}
	}
}

// TestRunGo_PropagatedUnderLock: with lock propagation the graph's
// propagated accessed_under_lock edges are exactly the pairs the truth's own
// call walk finds: a field used in a function called while the mutex is
// held, where it is still held at the use (Counter.viaCall and Counter.deep
// under Ledger.mu, not Counter.afterCall).
func TestRunGo_PropagatedUnderLock(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{LockPropagation: true})
	p := r.UnderLockPropagated
	if p.Edges == 0 || p.Unsupported != 0 || p.Matched != p.Truth {
		t.Errorf("propagated: %d edges, %d unsupported, %d of %d true pairs; unsupported %v missing %v",
			p.Edges, p.Unsupported, p.Matched, p.Truth, p.UnsupportedSamples, p.MissingSamples)
	}
	if r.UnderLock.Unsupported != 0 {
		t.Errorf("direct edges unsupported with propagation on: %v", r.UnderLock.UnsupportedSamples)
	}
	// Counter.viaCall and Counter.deep (Counter.afterCall is used after the
	// Unlock). Hub.cond and Hub.seen, which Hub.flush uses when Hub.Stop
	// calls it holding mu, are direct pairs too (Hub.Deferring uses them
	// holding mu), so not propagated ones.
	if p.Truth != 2 {
		t.Errorf("propagated truth pairs = %d, want 2", p.Truth)
	}
}

// TestRunGo_MessageLinks: the handles and triggers edges are exactly what
// the syntax says on the messages fixture (a worker loop with range, a
// select over two channels and a done signal, a send through a parameter,
// an unbuffered channel, a field channel).
func TestRunGo_MessageLinks(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{})
	for name, st := range map[string]UnderLockStats{"handles": r.Handles, "triggers": r.Triggers} {
		if st.Truth == 0 || st.Unsupported != 0 || st.Matched != st.Truth {
			t.Errorf("%s: %d edges, %d unsupported, %d of %d true; unsupported %v missing %v",
				name, st.Edges, st.Unsupported, st.Matched, st.Truth, st.UnsupportedSamples, st.MissingSamples)
		}
	}
}

// TestRunGo_ChannelLocks: the graph's channel locks and the fields under
// them agree with the truth's own rules: the inverted lock (a select send
// takes it) and a lock that reaches a method through a parameter and a
// field; no field is under the lock it holds.
func TestRunGo_ChannelLocks(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{})
	if r.UnderLock.Unsupported != 0 {
		t.Errorf("unsupported: %v", r.UnderLock.UnsupportedSamples)
	}
	want := []string{"chanlock/chanlock2.go:5:runs -> chanlock/chanlock2.go:10:channel", "chanlock/chanlock2.go:28:w -> chanlock/chanlock2.go:33:channel"}
	for _, w := range want {
		for _, m := range r.UnderLock.MissingSamples {
			if m == w {
				t.Errorf("missing %s", w)
			}
		}
	}
}

// TestRunGo_ExecutionOrder: the closes, runs_under_lock, operates_on and
// runs_func edges are exactly what the syntax and go/types say on the
// fixture (execorder: a close of a field, a package-level variable and a
// local channel; a lock held at the sites of the function and of a callee;
// the operations on a WaitGroup, a Once, a Cond, atomics, a context and a
// timer).
func TestRunGo_ExecutionOrder(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{LockPropagation: true})
	for name, st := range map[string]UnderLockStats{"closes": r.Closes, "runs_under_lock": r.SiteLocks,
		"runs_under_lock propagated": r.SiteLocksPropagated, "operates_on": r.SyncOps, "runs_func": r.RunsFunc,
		"sync_unknown": r.SyncUnknown} {
		// Of the sites under a lock, only the one in a function with a goto,
		// which the extractor leaves out by rule, is missing.
		missing := 0
		if name == "runs_under_lock" {
			missing = 1
			if want := []string{"locks/ledger.go@1881-1894 -> locks/ledger.go:7:mu"}; !reflect.DeepEqual(st.MissingSamples, want) {
				t.Errorf("runs_under_lock missing %v, want %v", st.MissingSamples, want)
			}
		}
		if st.Truth == 0 || st.Unsupported != 0 || st.Matched != st.Truth-missing {
			t.Errorf("%s: %d edges, %d unsupported, %d of %d true; unsupported %v missing %v",
				name, st.Edges, st.Unsupported, st.Matched, st.Truth, st.UnsupportedSamples, st.MissingSamples)
		}
	}
}

// TestRunGo_ValueCalls: a call through a local set once to a function or
// method is a static call in both the truth and the graph; a call through
// a field or a parameter has its value-flow target; a literal the caller
// calls is counted apart (it is the caller's own code).
func TestRunGo_ValueCalls(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{})
	if r.StaticCalls.Missing != 0 || r.Unexpected.Calls != 0 {
		t.Errorf("static calls missing %v, unexpected %v", r.StaticCalls.MissingSamples, r.Unexpected.CallSamples)
	}
	for _, cause := range []string{causeField, causeParam} {
		if st := r.DynamicTargets[cause]; st.Sites == 0 || st.WithTarget != st.Sites {
			t.Errorf("%s: %d of %d sites with a target; without: %v", cause, st.WithTarget, st.Sites, st.Samples)
		}
	}
	if st := r.DynamicTargets[causeOwnLiteral]; st.Sites == 0 {
		t.Errorf("no literal call counted: %v", r.DynamicTargets)
	}
}

// TestTruth_DeferredWorkAndParamSources: the truth itself holds the facts
// a graph once lacked or got wrong (execorder/deferred.go), so ckg verify
// reports a graph without them: deferred work registered after an Unlock
// runs under the lock (a literal's call and the calls in it, a close); of
// two calls from one byte, new(T).M(), only M's call is a site; the
// receive of an inline timer names the call that makes the timer; a
// parameter's sources are all named, or the site expects no edge.
func TestTruth_DeferredWorkAndParamSources(t *testing.T) {
	const file = "execorder/deferred.go"
	root, err := filepath.Abs(fidelityFixture)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	// span is the site key of the n-th (from 1) occurrence of text.
	span := func(text string, n int) string {
		at := -1
		for range n {
			next := bytes.Index(src[at+1:], []byte(text))
			if next < 0 {
				t.Fatalf("%q (occurrence %d) not in the fixture", text, n)
			}
			at += 1 + next
		}
		return fmt.Sprintf("%s@%d-%d", file, at, at+len(text))
	}
	truth, err := collectTruth(root, map[string]bool{file: true}, nil)
	if err != nil {
		t.Fatalf("collectTruth: %v", err)
	}
	const mu, inner = file + ":12:mu", file + ":13:inner"
	literal := "func(start time.Time) {\n\t\tp.note()\n\t}(time.Now())"
	locks := []struct {
		name string
		site string
		mu   string
		want bool
	}{
		{"the deferred literal's call", span(literal, 1), mu, true},
		{"a call in the deferred literal", span("p.note()", 1), mu, true},
		{"a deferred close", span("close(p.quit)", 1), mu, true},
		{"the method on new(T)", span("new(Pool).note()", 1), mu, true},
		{"the builtin new", span("new(Pool)", 1), mu, false},
		{"a lock taken in a deferred literal", span("p.inner.Lock()", 1), mu, true},
		{"its unlock under the lock it releases", span("p.inner.Unlock()", 1), inner, true},
		{"a literal that unlocks", span("func() { p.mu.Unlock() }()", 1), mu, true},
		{"a defer registered before that literal", span("p.note()", 2), mu, false},
	}
	for _, tc := range locks {
		if got := truth.SiteLocks[[2]string{tc.site, tc.mu}]; got != tc.want {
			t.Errorf("%s: (%s, %s) in the truth = %v, want %v", tc.name, tc.site, tc.mu, got, tc.want)
		}
	}
	timer := span("time.NewTimer(time.Second)", 1)
	ops := []struct {
		name string
		key  string
		decl string
		want bool
	}{
		{"an inline timer", span("<-time.NewTimer(time.Second).C", 1) + "|timer_recv", timer, true},
		{"a context through a caller's parameter", span("<-ctx.Done()", 2) + "|done_recv", file + ":84:ctx", true},
		{"a context made in place by the caller", span("<-ctx.Done()", 2) + "|done_recv", span("context.Background()", 4), true},
		{"a timer a caller assigns to a local", span("t.Reset(time.Second)", 1) + "|timer_reset", file + ":94:t", true},
		{"a field a caller passes", span("atomic.AddInt64(n, 1)", 1) + "|atomic_add", file + ":15:n", true},
	}
	for _, tc := range ops {
		if got := truth.Exec.sync[[2]string{tc.key, tc.decl}]; got != tc.want {
			t.Errorf("%s: (%s, %s) in the truth = %v, want %v", tc.name, tc.key, tc.decl, got, tc.want)
		}
	}
	// One caller's context comes from a function: the site expects no
	// edge at all, and is decided (not left out).
	// An exported function's parameter is open to callers outside the
	// program: no edge either.
	for _, incomplete := range []string{span("<-ctx.Done()", 1) + "|done_recv", span("<-ctx.Done()", 3) + "|done_recv"} {
		for k := range truth.Exec.sync {
			if k[0] == incomplete {
				t.Errorf("incomplete sources expected as an edge: %v", k)
			}
		}
		if truth.Exec.undecided[incomplete] {
			t.Errorf("%s left out; want decided with no edge", incomplete)
		}
		if !truth.Exec.unknown[incomplete] {
			t.Errorf("%s not expected as a marked site", incomplete)
		}
	}
	// A context made into a parameter is held by the field it is stored in.
	if k := [2]string{span("context.WithCancel(ctx)", 1) + "|ctx_make", file + ":117:ctx"}; !truth.Exec.sync[k] {
		t.Errorf("ctx_make into a parameter: %v not in the truth", k)
	}
}

// TestRunGo_ControlTransfer: the schema 1.45 facts of the fixture
// (execorder/transfer.go and the rest) agree with the truth, and the truth
// holds each kind: jumps_to (break, continue, fallthrough, goto, return),
// the clauses a switch encloses, literal_passed_to from a FuncLit with what
// the callee does with it, the extents of branch, clause, defer and literal
// nodes, and flows_to of a sync primitive stored into a field.
func TestRunGo_ControlTransfer(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{})
	for _, rel := range []string{string(types.EdgeJumpsTo), string(types.EdgeLiteralPassedTo), string(types.EdgeEncloses)} {
		s := r.Statements[rel]
		if s.Truth == 0 || s.Missing != 0 || s.Unexpected != 0 {
			t.Errorf("%s: truth %d matched %d, missing %v, unexpected %v", rel, s.Truth, s.Matched, s.MissingSamples, s.UnexpectedSamples)
		}
	}
	for name, st := range map[string]UnderLockStats{"extents": r.Extents, "flows_to sync": r.SyncAliases} {
		if st.Truth == 0 || st.Unsupported != 0 || st.Matched != st.Truth {
			t.Errorf("%s: %d edges, %d unsupported, %d of %d true; unsupported %v missing %v",
				name, st.Edges, st.Unsupported, st.Matched, st.Truth, st.UnsupportedSamples, st.MissingSamples)
		}
	}
	root, _ := filepath.Abs(fidelityFixture)
	truth, err := collectTruth(root, map[string]bool{"execorder/transfer.go": true}, nil)
	if err != nil {
		t.Fatalf("collectTruth: %v", err)
	}
	kinds := map[string]bool{}
	for f := range truth.Stmts {
		if f.Rel == string(types.EdgeJumpsTo) || f.Rel == string(types.EdgeLiteralPassedTo) && strings.HasPrefix(f.Src, string(types.NodeFuncLit)) {
			kinds[f.Rel+" "+f.Kind] = true
		}
	}
	for _, want := range []string{"jumps_to break", "jumps_to continue", "jumps_to fallthrough", "jumps_to goto", "jumps_to return",
		"literal_passed_to call/EXTRACTED", "literal_passed_to go/EXTRACTED", "literal_passed_to store/EXTRACTED", "literal_passed_to call/INFERRED", "literal_passed_to send/INFERRED"} {
		if !kinds[want] {
			t.Errorf("the truth holds no %s fact; has %v", want, kinds)
		}
	}
}
