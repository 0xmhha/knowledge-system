package concurrency_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/concurrency"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// fakeReader implements store.Reader by embedding the interface (nil) and
// overriding only the methods Analyze calls. Any other method would panic,
// proving Analyze depends on exactly the name lookups (FindByCanonicalID,
// FindSymbol) and NeighborhoodByIDs.
type fakeReader struct {
	store.Reader
	nodes map[string]types.Node
	edges []types.Edge
}

func (f *fakeReader) FindSymbol(name string, _ bool, _ store.FindSymbolOptions) ([]types.Node, error) {
	var out []types.Node
	for _, n := range f.nodes {
		if n.QualifiedName == name {
			out = append(out, n)
		}
	}
	return out, nil
}

// AllNodes returns every fixture node (the printed-name index reads them).
func (f *fakeReader) NodesByFilePath(path string) ([]types.Node, error) {
	var out []types.Node
	for _, n := range f.nodes {
		if n.FilePath == path {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeReader) AllNodes() ([]types.Node, error) {
	out := make([]types.Node, 0, len(f.nodes))
	for _, n := range f.nodes {
		out = append(out, n)
	}
	return out, nil
}

// FindByCanonicalID finds nothing: the fixture nodes carry no canonical id.
func (f *fakeReader) FindByCanonicalID(string) (types.Node, bool, error) {
	return types.Node{}, false, nil
}

// NeighborhoodByIDs faithfully mirrors the sqlite reader: start from the
// nodes with the given ids (include them), BFS to depth in one direction,
// filter by edge type, return the union node set + every traversed edge.
func (f *fakeReader) NeighborhoodByIDs(ids []string, depth int, reverse bool, edgeTypes ...string) ([]types.Node, []types.Edge, error) {
	var roots []types.Node
	for _, id := range ids {
		if n, ok := f.nodes[id]; ok {
			roots = append(roots, n)
		}
	}
	if len(roots) == 0 {
		return nil, nil, nil
	}
	want := map[string]bool{}
	for _, t := range edgeTypes {
		want[t] = true
	}
	allow := func(t types.EdgeType) bool { return len(want) == 0 || want[string(t)] }

	seen := map[string]types.Node{}
	frontier := map[string]bool{}
	for _, r := range roots {
		seen[r.ID] = r
		frontier[r.ID] = true
	}
	var allEdges []types.Edge
	for d := 0; d < depth; d++ {
		if len(frontier) == 0 {
			break
		}
		next := map[string]bool{}
		for _, e := range f.edges {
			if !allow(e.Type) {
				continue
			}
			from, to := e.Src, e.Dst
			if reverse {
				from, to = e.Dst, e.Src
			}
			if !frontier[from] {
				continue
			}
			allEdges = append(allEdges, e)
			if _, ok := seen[to]; !ok {
				if n, ok := f.nodes[to]; ok {
					seen[n.ID] = n
					next[to] = true
				}
			}
		}
		frontier = next
	}
	out := make([]types.Node, 0, len(seen))
	for _, n := range seen {
		out = append(out, n)
	}
	return out, allEdges, nil
}

func node(id, name, qname, file string, line int, kind types.NodeType) types.Node {
	return types.Node{ID: id, Name: name, QualifiedName: qname, FilePath: file, StartLine: line, Type: kind}
}

func edge(src, dst string, t types.EdgeType, line int) types.Edge {
	return types.Edge{Src: src, Dst: dst, Type: t, Line: line, Count: 1, Confidence: types.ConfInferred}
}

// fixture mirrors the REAL go-stablenet concurrency topology (verified against
// a 255k-node build): goroutine/channel/lock edges are anchored on the
// ENCLOSING FUNCTION, and channel ops point to a per-site CallSite node
// (Function -> sends_to/recvs_from -> CallSite), NOT Goroutine -> Channel.
//
//	Worker --spawns--> g1
//	Worker --sends_to--> cs_send (CallSite)
//	Peer   --spawns--> g2
//	Peer   --recvs_from--> cs_recv (CallSite)
//	Touch  --acquires_lock--> mu
//	Touch  --accessed_under_lock--> balance
//	Touch  --releases_lock--> mu        (must NOT surface)
func fixture() *fakeReader {
	nodes := []types.Node{
		node("n_worker", "Worker", "pkg.Worker", "a.go", 10, types.NodeFunction),
		node("n_g1", "goroutine", "pkg.Worker#Goroutine@12", "a.go", 12, types.NodeGoroutine),
		node("n_cs_send", "ch<-", "pkg.Worker#CallSite@13", "a.go", 13, types.NodeCallSite),
		node("n_peer", "Peer", "pkg.Peer", "b.go", 20, types.NodeFunction),
		node("n_g2", "goroutine", "pkg.Peer#Goroutine@22", "b.go", 22, types.NodeGoroutine),
		node("n_cs_recv", "<-ch", "pkg.Peer#CallSite@23", "b.go", 23, types.NodeCallSite),
		node("n_mu", "mu", "pkg.State.mu", "c.go", 30, types.NodeMutex),
		node("n_bal", "balance", "pkg.State.balance", "c.go", 31, types.NodeField),
		node("n_touch", "Touch", "pkg.Touch", "c.go", 35, types.NodeFunction),
	}
	m := map[string]types.Node{}
	for _, n := range nodes {
		m[n.ID] = n
	}
	return &fakeReader{
		nodes: m,
		edges: []types.Edge{
			edge("n_worker", "n_g1", types.EdgeSpawns, 12),
			edge("n_worker", "n_cs_send", types.EdgeSendsTo, 13), // Function -> CallSite (real)
			edge("n_peer", "n_g2", types.EdgeSpawns, 22),
			edge("n_peer", "n_cs_recv", types.EdgeRecvsFrom, 23), // Function -> CallSite (real)
			edge("n_touch", "n_mu", types.EdgeAcquiresLock, 36),
			edge("n_touch", "n_bal", types.EdgeAccessedUnderLock, 37),
			edge("n_touch", "n_mu", types.EdgeReleasesLock, 38), // excluded by design
		},
	}
}

func qnames(ms []concurrency.Module) map[string]concurrency.Module {
	out := map[string]concurrency.Module{}
	for _, m := range ms {
		out[m.Qname] = m
	}
	return out
}

func TestAnalyze_LockSeed_ExcludesReleasesLock(t *testing.T) {
	res, err := concurrency.Analyze(fixture(), "pkg.Touch", concurrency.Options{Depth: 2})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res.NotFound {
		t.Fatal("seed pkg.Touch should resolve")
	}
	mods := qnames(res.Modules)
	// A Mutex node is printed by the name that opens it (store.PrintedQName).
	mu, ok := mods["pkg.State.mu#mutex"]
	if !ok {
		t.Fatalf("expected pkg.State.mu#mutex in modules, got %+v", res.Modules)
	}
	if mu.Direction != "affected_by" {
		t.Errorf("mu direction = %q, want affected_by", mu.Direction)
	}
	if mu.Type != types.NodeMutex {
		t.Errorf("mu type = %q, want Mutex", mu.Type)
	}
	if _, ok := mods["pkg.State.balance"]; !ok {
		t.Errorf("expected pkg.State.balance in modules, got %+v", res.Modules)
	}
	if mu.Citation != "c.go:30" {
		t.Errorf("mu citation = %q, want c.go:30", mu.Citation)
	}
	for _, e := range res.Edges {
		if e[2].(string) == string(types.EdgeReleasesLock) {
			t.Errorf("releases_lock edge must NOT be surfaced, got %+v", e)
		}
	}
	var sawAcquire bool
	for _, e := range res.Edges {
		if e[2].(string) == string(types.EdgeAcquiresLock) {
			sawAcquire = true
		}
	}
	if !sawAcquire {
		t.Errorf("expected an acquires_lock edge, got %+v", res.Edges)
	}
}

func TestAnalyze_FieldSeed_ReverseFindsTouchers(t *testing.T) {
	// Byzantine query: "what touches this field under the same lock?"
	res, err := concurrency.Analyze(fixture(), "pkg.State.balance", concurrency.Options{Depth: 1})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	mods := qnames(res.Modules)
	touch, ok := mods["pkg.Touch"]
	if !ok {
		t.Fatalf("expected pkg.Touch (reverse) in modules, got %+v", res.Modules)
	}
	if touch.Direction != "affects" {
		t.Errorf("touch direction = %q, want affects", touch.Direction)
	}
}

func TestAnalyze_FunctionSeed_ForwardCallSiteTopology(t *testing.T) {
	// Real go-stablenet topology: a function reaches its own goroutine (spawns)
	// and the per-site CallSite of its channel op (sends_to), but NOT the peer
	// function across the channel (single-direction BFS; channel edges are
	// Function -> CallSite, not Goroutine -> Channel).
	res, err := concurrency.Analyze(fixture(), "pkg.Worker", concurrency.Options{Depth: 3})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	mods := qnames(res.Modules)
	if g := mods["pkg.Worker#Goroutine@12"]; g.Type != types.NodeGoroutine {
		t.Errorf("expected own Goroutine via spawns, got %+v", res.Modules)
	}
	cs, ok := mods["pkg.Worker#CallSite@13"]
	if !ok || cs.Type != types.NodeCallSite {
		t.Errorf("expected the send CallSite via sends_to, got %+v", res.Modules)
	}
	if _, ok := mods["pkg.Peer"]; ok {
		t.Errorf("function seed must NOT reach the peer across a CallSite, got %+v", res.Modules)
	}
}

// mutexFixture is a struct with a mutex field as the graph stores it: the
// Field node and a Mutex node named <field>#mutex, the lockers' acquires_lock
// edges to the Mutex node and the guarded fields' accessed_under_lock edges
// to it. A second type has a method of the same name.
func mutexFixture() *fakeReader {
	nodes := []types.Node{
		node("n_field", "mu", "pkg.State.mu", "c.go", 30, types.NodeField),
		node("n_mutex", "mu", "pkg.State.mu#mutex", "c.go", 30, types.NodeMutex),
		node("n_bal", "balance", "pkg.State.balance", "c.go", 31, types.NodeField),
		node("n_add", "Add", "pkg.State.Add", "c.go", 35, types.NodeMethod),
		node("n_get", "Get", "pkg.State.Get", "d.go", 12, types.NodeMethod),
		node("n_other", "Add", "pkg.Other.Add", "e.go", 5, types.NodeMethod),
	}
	m := map[string]types.Node{}
	for _, n := range nodes {
		m[n.ID] = n
	}
	return &fakeReader{
		nodes: m,
		edges: []types.Edge{
			edge("n_add", "n_mutex", types.EdgeAcquiresLock, 36),
			edge("n_get", "n_mutex", types.EdgeAcquiresLock, 13),
			edge("n_field", "n_mutex", types.EdgeAccessedUnderLock, 0),
			edge("n_bal", "n_mutex", types.EdgeAccessedUnderLock, 0),
		},
	}
}

// TestAnalyze_MutexFieldSeed: seeding the field that is a mutex leads to its
// Mutex node and so to the functions that lock it and the fields it guards.
// On go-stablenet a mutex field seed returned none of its lockers (0 of 183
// at depth 1 and 2): the lock edges end at the Mutex node, not at the field.
func TestAnalyze_MutexFieldSeed(t *testing.T) {
	for _, tc := range []struct {
		seed  string
		depth int
	}{
		{"pkg.State.mu", 1},
		{"pkg.State.mu", 2},
		{"pkg.State.mu#mutex", 1},
	} {
		res, err := concurrency.Analyze(mutexFixture(), tc.seed, concurrency.Options{Depth: tc.depth})
		if err != nil {
			t.Fatalf("Analyze(%s): %v", tc.seed, err)
		}
		mods := qnames(res.Modules)
		for _, want := range []string{"pkg.State.Add", "pkg.State.Get", "pkg.State.balance"} {
			if m, ok := mods[want]; !ok || m.Direction != "affects" {
				t.Errorf("Analyze(%s, depth %d): %s missing or not \"affects\": %+v", tc.seed, tc.depth, want, res.Modules)
			}
		}
		if _, ok := mods["pkg.Other.Add"]; ok {
			t.Errorf("Analyze(%s): a method that does not lock the mutex is listed", tc.seed)
		}
	}
	res, err := concurrency.Analyze(mutexFixture(), "pkg.State.mu", concurrency.Options{Depth: 1})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res.Mutex != "pkg.State.mu#mutex" {
		t.Errorf("a mutex field seed must name its Mutex node: %q", res.Mutex)
	}
}

func TestAnalyze_NotFound(t *testing.T) {
	res, err := concurrency.Analyze(fixture(), "pkg.DoesNotExist", concurrency.Options{Depth: 2})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !res.NotFound {
		t.Errorf("expected NotFound for unknown seed")
	}
	if len(res.Modules) != 0 {
		t.Errorf("expected no modules for unknown seed, got %+v", res.Modules)
	}
}

func TestAnalyze_MaxTotalAndDeterministicSort(t *testing.T) {
	res, err := concurrency.Analyze(fixture(), "pkg.Touch", concurrency.Options{Depth: 2, MaxTotal: 1})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Modules) != 1 {
		t.Fatalf("MaxTotal=1 should cap to 1 module, got %d", len(res.Modules))
	}
	// Sorted by qname: "pkg.State.balance" < "pkg.State.mu".
	if got := res.Modules[0].Qname; got != "pkg.State.balance" {
		t.Errorf("first module qname = %q, want pkg.State.balance (deterministic sort)", got)
	}
}

// TestAnalyze_MutexSeedForms: a mutex is a Field, a package-level Variable or
// a LocalVariable with a Mutex node beside it. The Go extractor names the
// Mutex node of a field "<field>#mutex" and that of a variable after the
// variable itself (node names are kept: ids and stored graphs depend on
// them). Both seed forms, the declaring name and that name with "#mutex",
// give one answer: mutex and declared_by name the two nodes, locked_by the
// functions that lock the mutex with their lines, guarded_by the other
// mutexes held where the declaration is accessed, and modules the walk from
// the Mutex node. On go-stablenet the two forms of 150 of 150 field mutexes
// differed (the field among the modules in one, the other mutexes guarding
// the field in the other), and "pkg.registry#mutex" was not found.
func TestAnalyze_MutexSeedForms(t *testing.T) {
	nodes := []types.Node{
		node("n_field", "mu", "pkg.State.mu", "c.go", 30, types.NodeField),
		node("n_fmutex", "mu", "pkg.State.mu#mutex", "c.go", 30, types.NodeMutex),
		node("n_outer", "outerMu", "pkg.State.outerMu#mutex", "c.go", 29, types.NodeMutex),
		node("n_bal", "balance", "pkg.State.balance", "c.go", 31, types.NodeField),
		node("n_var", "registry", "pkg.registry", "c.go", 5, types.NodeVariable),
		node("n_vmutex", "registry", "pkg.registry", "c.go", 5, types.NodeMutex),
		node("n_guard", "Guard", "pkg.Guard", "c.go", 40, types.NodeFunction),
		node("n_local", "made", "pkg.Guard.made", "c.go", 41, types.NodeLocalVariable),
		node("n_lmutex", "made", "pkg.Guard.made", "c.go", 41, types.NodeMutex),
		node("n_add", "Add", "pkg.State.Add", "c.go", 35, types.NodeMethod),
		node("n_scan", "Scan", "pkg.Scan", "c.go", 10, types.NodeFunction),
	}
	m := map[string]types.Node{}
	for _, n := range nodes {
		m[n.ID] = n
	}
	r := &fakeReader{nodes: m, edges: []types.Edge{
		edge("n_add", "n_fmutex", types.EdgeAcquiresLock, 36),
		edge("n_field", "n_fmutex", types.EdgeAccessedUnderLock, 0),
		edge("n_field", "n_outer", types.EdgeAccessedUnderLock, 0),
		edge("n_bal", "n_fmutex", types.EdgeAccessedUnderLock, 0),
		edge("n_scan", "n_vmutex", types.EdgeAcquiresLock, 11),
		edge("n_guard", "n_lmutex", types.EdgeAcquiresLock, 42),
	}}
	for _, tc := range []struct {
		forms             []string
		mutex, declaredBy string
		lockedBy          []string
		guardedBy         []string
		modules           []string
	}{
		{[]string{"pkg.State.mu", "pkg.State.mu#mutex"}, "pkg.State.mu#mutex", "pkg.State.mu",
			[]string{"pkg.State.Add:36"}, []string{"pkg.State.outerMu#mutex"}, []string{"pkg.State.Add", "pkg.State.balance"}},
		{[]string{"pkg.registry", "pkg.registry#mutex"}, "pkg.registry#mutex", "pkg.registry",
			[]string{"pkg.Scan:11"}, nil, []string{"pkg.Scan"}},
		{[]string{"pkg.Guard.made", "pkg.Guard.made#mutex"}, "pkg.Guard.made#mutex", "pkg.Guard.made",
			[]string{"pkg.Guard:42"}, nil, []string{"pkg.Guard"}},
	} {
		var first []byte
		for _, seed := range tc.forms {
			res, err := concurrency.Analyze(r, seed, concurrency.Options{Depth: 1})
			if err != nil {
				t.Fatalf("Analyze(%s): %v", seed, err)
			}
			if res.NotFound || res.Mutex != tc.mutex || res.DeclaredBy != tc.declaredBy {
				t.Errorf("Analyze(%s): not_found %v, mutex %q, declared_by %q", seed, res.NotFound, res.Mutex, res.DeclaredBy)
			}
			var locked, guarded, mods []string
			for _, l := range res.LockedBy {
				locked = append(locked, fmt.Sprintf("%s:%d", l.Qname, l.Line))
			}
			for _, g := range res.GuardedBy {
				guarded = append(guarded, g.Qname)
			}
			for _, md := range res.Modules {
				mods = append(mods, md.Qname)
			}
			if !reflect.DeepEqual(locked, tc.lockedBy) || !reflect.DeepEqual(guarded, tc.guardedBy) || !reflect.DeepEqual(mods, tc.modules) {
				t.Errorf("Analyze(%s): locked_by %v guarded_by %v modules %v, want %v %v %v", seed, locked, guarded, mods, tc.lockedBy, tc.guardedBy, tc.modules)
			}
			res.Seed = ""
			b, _ := json.Marshal(res)
			if first == nil {
				first = b
			} else if string(b) != string(first) {
				t.Errorf("the forms %v answer differently:\n%s\n%s", tc.forms, first, b)
			}
		}
	}
	// A name with the suffix that names no mutex stays not found.
	if res, err := concurrency.Analyze(r, "pkg.Scan#mutex", concurrency.Options{Depth: 1}); err != nil || !res.NotFound {
		t.Errorf("Analyze(pkg.Scan#mutex) = %+v, %v; want not found", res, err)
	}
}

// TestAnalyze_LocalDeclaredTwice: a name that one function declares twice
// names two mutexes; the seed is ambiguous with one candidate per
// declaration, in both forms, and each candidate resolves to its own mutex.
// catalyst.TestSimultaneousNewBlock.errMu merged the two silently.
func TestAnalyze_LocalDeclaredTwice(t *testing.T) {
	nodes := []types.Node{
		node("n_fn", "F", "pkg.F", "c.go", 1, types.NodeFunction),
		node("n_a", "mu", "pkg.F.mu", "c.go", 3, types.NodeLocalVariable),
		node("n_am", "mu", "pkg.F.mu", "c.go", 3, types.NodeMutex),
		node("n_b", "mu", "pkg.F.mu", "c.go", 9, types.NodeLocalVariable),
		node("n_bm", "mu", "pkg.F.mu", "c.go", 9, types.NodeMutex),
	}
	m := map[string]types.Node{}
	for _, n := range nodes {
		m[n.ID] = n
	}
	r := &fakeReader{nodes: m, edges: []types.Edge{
		edge("n_fn", "n_am", types.EdgeAcquiresLock, 4),
		edge("n_fn", "n_bm", types.EdgeAcquiresLock, 10),
	}}
	want := []string{"pkg.F.mu@c.go:3", "pkg.F.mu@c.go:9"}
	for seed, cands := range map[string][]string{"pkg.F.mu": want, "pkg.F.mu#mutex": {"pkg.F.mu#mutex@c.go:3", "pkg.F.mu#mutex@c.go:9"}} {
		res, err := concurrency.Analyze(r, seed, concurrency.Options{Depth: 1})
		if err != nil {
			t.Fatalf("Analyze(%s): %v", seed, err)
		}
		if !res.Ambiguous || !reflect.DeepEqual(res.Candidates, cands) {
			t.Errorf("Analyze(%s): ambiguous %v candidates %v, want %v", seed, res.Ambiguous, res.Candidates, cands)
		}
	}
	if res, err := concurrency.Analyze(r, "pkg.F.mu#mutex@c.go:9", concurrency.Options{Depth: 1}); err != nil || res.Mutex != "pkg.F.mu#mutex@c.go:9" || res.DeclaredBy != "pkg.F.mu@c.go:9" || len(res.LockedBy) != 1 || res.LockedBy[0].Line != 10 {
		t.Errorf("Analyze(pkg.F.mu#mutex@c.go:9) = %+v, %v", res, err)
	}
	res, err := concurrency.Analyze(r, want[1], concurrency.Options{Depth: 1})
	if err != nil || len(res.LockedBy) != 1 || res.LockedBy[0].Line != 10 {
		t.Errorf("Analyze(%s) = %+v, %v; want the mutex locked at line 10 only", want[1], res.LockedBy, err)
	}
}
