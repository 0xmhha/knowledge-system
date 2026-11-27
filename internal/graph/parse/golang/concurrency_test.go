package golang_test

import (
	"fmt"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestConcurrency_Mutex_FieldAndLocal asserts that struct-field and local
// sync.Mutex declarations both produce NodeMutex nodes with the right
// sub_kind, and that user-defined types named "Mutex" do NOT trip the
// false-positive guard.
func TestConcurrency_Mutex_FieldAndLocal(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}

	mutexes := nodesByType(g.Nodes, types.NodeMutex)
	bySubKind := groupBySubKind(mutexes)

	// Counter.mu (struct field), Cache.mu (rwmutex field), Embedded.Mutex
	// (embedded), localMu (local var). FakeMutex must NOT appear.
	if got := bySubKind["mutex"]; got < 2 {
		t.Errorf("mutex sub_kind: got %d, want >=2 (Counter.mu + LocalLock.localMu + Embedded.Mutex)", got)
	}
	// Cache.mu (struct field) plus GoroutineRWLock.rwMu (function-local) — 2 in total.
	if got := bySubKind["rwmutex"]; got < 1 {
		t.Errorf("rwmutex sub_kind: got %d, want >=1 (Cache.mu, GoroutineRWLock.rwMu)", got)
	}
	for _, n := range mutexes {
		if strings.Contains(n.QualifiedName, "FakeMutex") {
			t.Errorf("FakeMutex should NOT appear as Mutex node: %s", n.QualifiedName)
		}
	}
}

// TestConcurrency_Mutex_Embedded ensures the embedded `sync.Mutex` field
// becomes a Mutex node attached to the Embedded struct's qname.
func TestConcurrency_Mutex_Embedded(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	found := false
	for _, n := range g.Nodes {
		if n.Type != types.NodeMutex {
			continue
		}
		if strings.HasSuffix(n.QualifiedName, "Embedded.Mutex#mutex") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Mutex node for embedded sync.Mutex on Embedded struct")
	}
}

// TestConcurrency_LockEdges_DeferAndExplicit verifies acquires_lock /
// releases_lock edges are emitted for both `mu.Lock(); mu.Unlock()` and
// `mu.Lock(); defer mu.Unlock()` patterns.
func TestConcurrency_LockEdges_DeferAndExplicit(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	acquires := edgesByType(g.Edges, types.EdgeAcquiresLock)
	releases := edgesByType(g.Edges, types.EdgeReleasesLock)
	// Inc, Get, Read, Write, Set, LocalLock = 6 acquires + 6 releases
	// (FakeMutex.UseFake must not contribute — types.Info distinguishes).
	if len(acquires) < 6 {
		t.Errorf("acquires_lock count: got %d, want >=6", len(acquires))
	}
	if len(releases) < 6 {
		t.Errorf("releases_lock count: got %d, want >=6", len(releases))
	}
	// FakeMutex.Lock() should NOT have produced any lock edge — sweep both.
	for _, e := range acquires {
		dst := findNodeByID(g.Nodes, e.Dst)
		if dst != nil && strings.Contains(dst.QualifiedName, "FakeMutex") {
			t.Errorf("acquires_lock to FakeMutex.* — false-positive guard failed: dst=%s", dst.QualifiedName)
		}
	}
}

// TestConcurrency_RWMutex_LockEdges asserts RLock/RUnlock produce the
// same edge types as Lock/Unlock (the lock variant lives in the Mutex
// node's sub_kind, not a separate edge type).
func TestConcurrency_RWMutex_LockEdges(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	// Find the Cache.mu Mutex node ID.
	var cacheMuID string
	for _, n := range g.Nodes {
		if n.Type == types.NodeMutex && strings.HasSuffix(n.QualifiedName, "Cache.mu#mutex") {
			cacheMuID = n.ID
			break
		}
	}
	if cacheMuID == "" {
		t.Fatal("Cache.mu Mutex node not found")
	}
	var rlockSeen, runlockSeen bool
	for _, e := range g.Edges {
		if e.Dst != cacheMuID {
			continue
		}
		switch e.Type {
		case types.EdgeAcquiresLock:
			rlockSeen = true
		case types.EdgeReleasesLock:
			runlockSeen = true
		}
	}
	if !rlockSeen {
		t.Error("expected acquires_lock edge into Cache.mu (Read uses RLock)")
	}
	if !runlockSeen {
		t.Error("expected releases_lock edge into Cache.mu (Read uses RUnlock)")
	}
}

// TestConcurrency_Channel_Attributes asserts make(chan T, n) → Channel
// node with direction sub_kind and signature carrying the elem + buffer.
func TestConcurrency_Channel_Attributes(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	channels := nodesByType(g.Nodes, types.NodeChannel)
	if len(channels) < 4 {
		t.Errorf("Channel node count: got %d, want >=4 (one per make(chan ...))", len(channels))
	}
	directions := groupBySubKind(channels)
	if directions["bidi"] < 2 {
		t.Errorf("bidi channel count: got %d, want >=2", directions["bidi"])
	}
	if directions["send"] < 1 {
		t.Errorf("send-only channel count: got %d, want >=1", directions["send"])
	}
	if directions["recv"] < 1 {
		t.Errorf("recv-only channel count: got %d, want >=1", directions["recv"])
	}
	// At least one channel signature should carry buf= info.
	var hasBuffered bool
	for _, n := range channels {
		if strings.Contains(n.Signature, "buf=10") {
			hasBuffered = true
			break
		}
	}
	if !hasBuffered {
		t.Error("expected at least one Channel signature containing buf=10 (MakeBuffered)")
	}
}

// TestConcurrency_NoRegression_Goroutine confirms the concurrency pass didn't break the
// pre-existing Goroutine/spawns/sends_to/recvs_from extraction. The
// fixture's GoroutineFanout uses all four.
func TestConcurrency_NoRegression_Goroutine(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	if len(nodesByType(g.Nodes, types.NodeGoroutine)) < 1 {
		t.Error("expected >=1 Goroutine node")
	}
	if len(edgesByType(g.Edges, types.EdgeSpawns)) < 1 {
		t.Error("expected >=1 spawns edge")
	}
	if len(edgesByType(g.Edges, types.EdgeSendsTo)) < 1 {
		t.Error("expected >=1 sends_to edge")
	}
	if len(edgesByType(g.Edges, types.EdgeRecvsFrom)) < 1 {
		t.Error("expected >=1 recvs_from edge")
	}
}

// TestChannelFlow_MakeAssign_SendsToChannelNode verifies that a sends_to edge
// emitted for a channel created via `ch := make(chan T, n)` points to the
// Channel node rather than an anonymous CallSite node.
func TestChannelFlow_MakeAssign_SendsToChannelNode(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	channels := nodesByType(g.Nodes, types.NodeChannel)
	if len(channels) == 0 {
		t.Fatal("no Channel nodes found")
	}
	chanIDs := make(map[string]bool, len(channels))
	for _, n := range channels {
		chanIDs[n.ID] = true
	}
	sendsTo := edgesByType(g.Edges, types.EdgeSendsTo)
	var foundChannelDst bool
	for _, e := range sendsTo {
		if chanIDs[e.Dst] {
			foundChannelDst = true
			break
		}
	}
	if !foundChannelDst {
		t.Error("expected at least one sends_to edge with Channel node as Dst; all point to CallSite")
	}
}

// TestChannelFlow_MakeAssign_RecvsFromChannelNode verifies that a recvs_from
// edge for a make()-created channel points to the Channel node, not a CallSite.
func TestChannelFlow_MakeAssign_RecvsFromChannelNode(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	channels := nodesByType(g.Nodes, types.NodeChannel)
	chanIDs := make(map[string]bool, len(channels))
	for _, n := range channels {
		chanIDs[n.ID] = true
	}
	recvsFrom := edgesByType(g.Edges, types.EdgeRecvsFrom)
	var foundChannelDst bool
	for _, e := range recvsFrom {
		if chanIDs[e.Dst] {
			foundChannelDst = true
			break
		}
	}
	if !foundChannelDst {
		t.Error("expected at least one recvs_from edge with Channel node as Dst; all point to CallSite")
	}
}

// TestChannelFlow_Goroutine_SendsToChannelNode verifies the Goroutine node →
// Channel node sends_to edge is emitted for `go func() { ch <- 1 }()` where
// ch was created with make() in the enclosing function.
func TestChannelFlow_Goroutine_SendsToChannelNode(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	goroutines := nodesByType(g.Nodes, types.NodeGoroutine)
	if len(goroutines) == 0 {
		t.Fatal("no Goroutine nodes found")
	}
	goroutineIDs := make(map[string]bool, len(goroutines))
	for _, n := range goroutines {
		goroutineIDs[n.ID] = true
	}
	channels := nodesByType(g.Nodes, types.NodeChannel)
	chanIDs := make(map[string]bool, len(channels))
	for _, n := range channels {
		chanIDs[n.ID] = true
	}
	sendsTo := edgesByType(g.Edges, types.EdgeSendsTo)
	var found bool
	for _, e := range sendsTo {
		if goroutineIDs[e.Src] && chanIDs[e.Dst] {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Goroutine -sends_to-> Channel edge; none found")
	}
}

// TestConcurrency_GoroutineLockEdges — lock CallExprs inside a
// `go func() { ... }()` literal must produce acquires_lock / releases_lock
// edges. Pre-fix, statements.go returned false on *ast.GoStmt to avoid
// double-walking, and emitGoroutineChannelEdges only handled channel
// statements — so the entire goroutine body's lock calls were silently
// skipped. This test pins the post-fix behaviour: each goroutine fixture
// in goroutine_lock.go contributes 1 acquires_lock + 1 releases_lock.
func TestConcurrency_GoroutineLockEdges(t *testing.T) {
	root := "testdata/concurrency"
	g, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	// Find the function-local Mutex IDs for GoroutineLock + GoroutineRWLock.
	var goroutineLockMu, goroutineRWLockMu string
	for _, n := range g.Nodes {
		if n.Type != types.NodeMutex {
			continue
		}
		switch {
		case strings.HasSuffix(n.QualifiedName, ".GoroutineLock.errMu"):
			goroutineLockMu = n.ID
		case strings.HasSuffix(n.QualifiedName, ".GoroutineRWLock.rwMu"):
			goroutineRWLockMu = n.ID
		}
	}
	if goroutineLockMu == "" {
		t.Fatal("expected Mutex node for GoroutineLock.errMu (function-local var)")
	}
	if goroutineRWLockMu == "" {
		t.Fatal("expected Mutex node for GoroutineRWLock.rwMu (function-local var)")
	}
	acquires := edgesByType(g.Edges, types.EdgeAcquiresLock)
	releases := edgesByType(g.Edges, types.EdgeReleasesLock)
	var lockAcq, lockRel, rwlockAcq, rwlockRel int
	for _, e := range acquires {
		switch e.Dst {
		case goroutineLockMu:
			lockAcq++
		case goroutineRWLockMu:
			rwlockAcq++
		}
	}
	for _, e := range releases {
		switch e.Dst {
		case goroutineLockMu:
			lockRel++
		case goroutineRWLockMu:
			rwlockRel++
		}
	}
	if lockAcq < 1 {
		t.Errorf("acquires_lock(GoroutineLock -> errMu): got %d, want >=1", lockAcq)
	}
	if lockRel < 1 {
		t.Errorf("releases_lock(GoroutineLock -> errMu): got %d, want >=1", lockRel)
	}
	if rwlockAcq < 1 {
		t.Errorf("acquires_lock(GoroutineRWLock -> rwMu): got %d, want >=1 (RLock)", rwlockAcq)
	}
	if rwlockRel < 1 {
		t.Errorf("releases_lock(GoroutineRWLock -> rwMu): got %d, want >=1 (RUnlock)", rwlockRel)
	}
}

// helpers

func nodesByType(nodes []types.Node, t types.NodeType) []types.Node {
	out := nodes[:0:0]
	for _, n := range nodes {
		if n.Type == t {
			out = append(out, n)
		}
	}
	return out
}

func edgesByType(edges []types.Edge, t types.EdgeType) []types.Edge {
	out := edges[:0:0]
	for _, e := range edges {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func groupBySubKind(nodes []types.Node) map[string]int {
	out := map[string]int{}
	for _, n := range nodes {
		out[n.SubKind]++
	}
	return out
}

func findNodeByID(nodes []types.Node, id string) *types.Node {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

// TestChannelFlow_GoroutineFindsEveryOrigin: the goroutine literal of
// ChannelOrigins uses five local channels; each gets its Goroutine edge, as
// each gets its function edge (one rule, bindLocalChans). Before, a channel
// declared with var, ranged over, made by a multi-name := or made inside
// the literal had no Goroutine edge (57 operations of go-stablenet).
func TestChannelFlow_GoroutineFindsEveryOrigin(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/concurrency")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	type link struct {
		et   types.EdgeType
		line int
	}
	fromGoroutine, fromFunction := map[link]bool{}, map[link]bool{}
	for _, e := range g.Edges {
		if e.Type != types.EdgeSendsTo && e.Type != types.EdgeRecvsFrom {
			continue
		}
		src, dst := byID[e.Src], byID[e.Dst]
		if dst.Type != types.NodeChannel || !strings.HasSuffix(dst.FilePath, "channel_origins.go") {
			continue
		}
		switch src.Type {
		case types.NodeGoroutine:
			fromGoroutine[link{e.Type, dst.StartLine}] = true
		case types.NodeFunction:
			fromFunction[link{e.Type, dst.StartLine}] = true
		}
	}
	// Lines of the make calls: done 6, items 7, errs and outs 8, inner 16.
	for _, w := range []link{
		{types.EdgeRecvsFrom, 7}, {types.EdgeSendsTo, 6}, {types.EdgeSendsTo, 8}, {types.EdgeSendsTo, 16},
	} {
		if !fromGoroutine[w] {
			t.Errorf("no Goroutine %s edge to the channel made on line %d; have %v", w.et, w.line, fromGoroutine)
		}
		if !fromFunction[w] {
			t.Errorf("no function %s edge to the channel made on line %d; have %v", w.et, w.line, fromFunction)
		}
	}
}

// TestChannelFlow_GoroutineEdgesFromTheLinker: the Goroutine's channel edges
// come from the channel linker, the one rule for a channel's origin, so a
// go statement in a loop or a branch, a grouped var and a channel passed to
// a call before the goroutine ranges over it all get one. The function's
// own (extracted) operation on a grouped var points at the Channel too.
func TestChannelFlow_GoroutineEdgesFromTheLinker(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/concurrency")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	goroutineEdge, extracted := map[string]bool{}, map[string]bool{}
	for _, e := range g.Edges {
		if e.Type != types.EdgeSendsTo && e.Type != types.EdgeRecvsFrom {
			continue
		}
		src, dst := byID[e.Src], byID[e.Dst]
		if dst.Type != types.NodeChannel {
			continue
		}
		fn := src.QualifiedName
		if i := strings.Index(fn, "#"); i >= 0 {
			fn = fn[:i]
		}
		if src.Type == types.NodeGoroutine {
			goroutineEdge[fn] = true
		}
		if src.Type == types.NodeFunction && e.Confidence == types.ConfExtracted {
			extracted[fn] = true
		}
	}
	for _, fn := range []string{"Works", "InsideFor", "InsideIf", "GroupedVar", "CallArg"} {
		if !goroutineEdge["mutex_fixture."+fn] {
			t.Errorf("%s: no Goroutine -> Channel edge", fn)
		}
	}
	if !extracted["mutex_fixture.GroupedVar"] {
		t.Error("GroupedVar: the function's operation on the grouped var does not point at its Channel")
	}
}

// TestGoroutineReachesItsCode: a Goroutine node leads to the code it runs
// (dispatch kind goroutine): go f(x) to f, and a literal to the declared
// functions it calls and the fields it uses.
func TestGoroutineReachesItsCode(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/concurrency")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		if e.DispatchKind != gop.GoroutineKind {
			continue
		}
		src, dst := byID[e.Src], byID[e.Dst]
		if src.Type != types.NodeGoroutine {
			t.Errorf("goroutine edge from a %s: %s", src.Type, src.QualifiedName)
		}
		fn, _, _ := strings.Cut(src.QualifiedName, "#")
		got[fn+" "+string(e.Type)+" "+dst.QualifiedName] = true
	}
	for _, w := range []string{
		"mutex_fixture.StartsNamed calls mutex_fixture.worker",
		"mutex_fixture.StartsLiteral calls mutex_fixture.worker",
		"mutex_fixture.StartsLiteral writes_field mutex_fixture.Job.done",
		"mutex_fixture.StartsLiteral reads_field mutex_fixture.Job.id",
	} {
		if !got[w] {
			t.Errorf("missing %s; have %v", w, got)
		}
	}
}

// TestMessageLinks: a receive handles the code in its range (a range
// body, a select case body, the rest of a plain receive's block), its value
// flows to the uses it reaches (not for a signal channel), and each sender
// on the same channel triggers it: through a parameter, on an unbuffered
// channel, on a field channel; a send in a goroutine's literal is the
// goroutine's, not the function's that starts it.
func TestMessageLinks(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/concurrency")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	// owner names a statement node by its function and the line it is on.
	owner := func(n types.Node) string {
		fn, _, _ := strings.Cut(strings.TrimPrefix(n.QualifiedName, "mutex_fixture."), "#")
		if n.Type == types.NodeField || n.Type == types.NodeFunction || n.Type == types.NodeMethod {
			return fn
		}
		return fmt.Sprintf("%s:%d", fn, n.StartLine)
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		switch {
		case e.Type == types.EdgeHandles, e.Type == types.EdgeTriggers,
			e.Type == types.EdgeFlowsTo && e.DispatchKind == types.DispatchReceived:
		default:
			continue
		}
		kind := e.DispatchKind
		if e.Type == types.EdgeTriggers {
			kind = ""
		}
		got[fmt.Sprintf("%s %s %s %s", owner(byID[e.Src]), e.Type, kind, owner(byID[e.Dst]))] = true
	}
	for _, w := range []string{
		"WorkerLoop:18 handles call WorkerLoop:20", "WorkerLoop:18 handles write Store.total", "WorkerLoop:18 handles read Store.total",
		"Serve:41 handles call Serve:42", "Serve:43 handles write Store.last", "Serve:43 handles call Serve:45",
		"Serve:46 handles call Serve:47",
		"Store.Drain:68 handles call Store.Drain:69", "Store.Drain:68 handles write Store.total",
		"Serve:43 flows_to received Store.last", "Store.Drain:68 flows_to received Store.Drain:69",
		"Produce triggers  WorkerLoop:18", "RunServe triggers  Serve:41", "RunServe triggers  Serve:43",
		"RunServe triggers  Serve:46", "Store.Feed triggers  Store.Drain:68",
		"ChannelOrigins:9 triggers  ChannelOrigins:19",
	} {
		if !got[w] {
			t.Errorf("missing %q", w)
		}
	}
	for _, bad := range []string{"Serve:46 flows_to received Serve:47", "ChannelOrigins triggers  ChannelOrigins:19"} {
		if got[bad] {
			t.Errorf("unexpected %q", bad)
		}
	}
	if t.Failed() {
		t.Logf("have %v", got)
	}
}

// TestGoroutineRules: a goroutine on a declared function links to that
// function only (the literal loop calls in place is loop's own code); a
// goroutine on a literal links to its interface calls (invokes) and the
// fields it names, not to an embedded field a promoted call passes
// through; a send on a channel's make site and a receive through the field
// it is stored in are joined by the alias; the edge of an operation names
// the operation's file.
func TestGoroutineRules(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/concurrency")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		s := strings.TrimPrefix(src.QualifiedName, "mutex_fixture.")
		if src.Type == types.NodeGoroutine {
			s = "goroutine of " + strings.SplitN(s, "#", 2)[0]
		}
		d := strings.TrimPrefix(dst.QualifiedName, "mutex_fixture.")
		if i := strings.Index(d, "#CallSite@"); i >= 0 {
			d = d[:i+len("#CallSite")]
		}
		got[fmt.Sprintf("%s %s %s", s, e.Type, d)] = true
		if src.Type == types.NodeGoroutine && e.Line > 0 && dst.Type != types.NodeChannel && e.FilePath != "" {
			if !strings.HasSuffix(e.FilePath, "goroutine_rules.go") && strings.Contains(src.QualifiedName, "Worker.Start") {
				t.Errorf("edge %s -> %s at line %d names file %s", src.QualifiedName, dst.QualifiedName, e.Line, e.FilePath)
			}
		}
	}
	for _, w := range []string{
		"goroutine of Worker.Start calls Worker.loop", "goroutine of Worker.Start invokes Doer.Do",
		"goroutine of Worker.Start reads_field Worker.d", "NewWorker triggers Worker.loop#CallSite",
		"Worker.reply triggers Worker.loop#CallSite", "goroutine of Worker.Start references hits",
	} {
		if !got[w] {
			t.Errorf("missing %q", w)
		}
	}
	for _, bad := range []string{"goroutine of Worker.Start reads_field Worker.Base", "goroutine of Worker.Start writes_field Worker.n"} {
		if got[bad] {
			t.Errorf("unexpected %q", bad)
		}
	}
}

// TestChannelLocks: a channel of empty structs with a buffer of one, filled
// right where it is made, is a lock: its Channel node has sub kind lock,
// receives acquire it, sends and close release it (close with kind close),
// and fields used while its token is held are under it, on the select case
// that takes it only; it gets no handles or triggers. A semaphore of
// capacity two and a channel filled elsewhere stay channels.
func TestChannelLocks(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/concurrency")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	locks := map[int]bool{} // lines of the make sites marked lock
	for _, n := range g.Nodes {
		byID[n.ID] = n
		if n.Type == types.NodeChannel && strings.HasSuffix(n.FilePath, "chanlock.go") && n.SubKind == gop.ChanLockSubKind {
			locks[n.StartLine] = true
		}
	}
	if len(locks) != 2 || !locks[11] || !locks[57] {
		t.Errorf("lock channels on lines %v, want 11 (ClosableMutex) and 57 (Device)", locks)
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		if dst.Type != types.NodeChannel || !strings.HasSuffix(dst.FilePath, "chanlock.go") {
			if e.Type != types.EdgeAccessedUnderLock || !strings.HasSuffix(dst.FilePath, "chanlock.go") {
				continue
			}
		}
		name := strings.TrimPrefix(src.QualifiedName, "mutex_fixture.")
		got[fmt.Sprintf("%s %s %d %s", name, e.Type, dst.StartLine, e.DispatchKind)] = true
	}
	for _, w := range []string{
		"ClosableMutex.TryLock acquires_lock 11 ", "ClosableMutex.MustLock acquires_lock 11 ",
		"ClosableMutex.Unlock releases_lock 11 ", "ClosableMutex.Close releases_lock 11 close",
		"Device.Bump acquires_lock 57 ", "Device.Bump releases_lock 57 ",
		"Device.count accessed_under_lock 57 ",
	} {
		if !got[w] {
			t.Errorf("missing %q", w)
		}
	}
	for k := range got {
		if strings.HasPrefix(k, "Device.last accessed_under_lock") || strings.Contains(k, "Pool.Use acquires_lock") || strings.Contains(k, "Gate.Pass acquires_lock") {
			t.Errorf("unexpected %q", k)
		}
	}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		takesLock := src.Type == types.NodeCallSite && strings.HasSuffix(src.FilePath, "chanlock.go") && src.StartLine == 66
		wakesLock := dst.Type == types.NodeCallSite && strings.HasSuffix(dst.FilePath, "chanlock.go") && dst.StartLine == 66
		if e.Type == types.EdgeHandles && takesLock || e.Type == types.EdgeTriggers && wakesLock {
			t.Errorf("message link on a lock: %s -> %s", src.QualifiedName, dst.QualifiedName)
		}
	}
}

// TestRound2: an inverted lock (a select send takes it, a receive returns
// it) is a lock; a send in the receive's own statement is not in its range;
// a closure stored in a variable and called later is not the goroutine's
// code, a deferred interface call is, and go on an interface method links to
// it; a received struct's promoted method call is where its value flows.
func TestRound2(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/concurrency")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		if !strings.HasSuffix(src.FilePath, "round2.go") && !strings.HasSuffix(dst.FilePath, "round2.go") {
			continue
		}
		s := strings.TrimPrefix(src.QualifiedName, "mutex_fixture.")
		if src.Type == types.NodeGoroutine || src.Type == types.NodeCallSite {
			s = fmt.Sprintf("%s:%d", src.Type, src.StartLine)
		}
		d := strings.TrimPrefix(dst.QualifiedName, "mutex_fixture.")
		if dst.Type == types.NodeCallSite || dst.Type == types.NodeChannel {
			d = fmt.Sprintf("%s:%d", dst.Type, dst.StartLine)
		}
		got[fmt.Sprintf("%s %s %s", s, e.Type, d)] = true
	}
	for _, w := range []string{
		"Resetter.Loop acquires_lock Channel:10", "Resetter.Loop releases_lock Channel:10",
		"Resetter.runs accessed_under_lock Channel:10",
		"Goroutine:46 invokes Sub.Unsubscribe", "Goroutine:50 invokes Doer.Do",
		"CallSite:64 flows_to CallSite:65", "CallSite:33 handles Indexer.state",
	} {
		if !got[w] {
			t.Errorf("missing %q", w)
		}
	}
	for _, bad := range []string{"CallSite:33 handles CallSite:33", "Goroutine:46 calls record"} {
		if got[bad] {
			t.Errorf("unexpected %q", bad)
		}
	}
	if t.Failed() {
		for k := range got {
			t.Log(k)
		}
	}
}
