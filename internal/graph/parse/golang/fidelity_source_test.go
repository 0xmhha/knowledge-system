package golang_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// The checks below state what the source of the fixture holds for the
// differences an independent comparison of a go-stablenet graph with its
// source listed (2026-11-26): each names the construct and fails while the
// graph says otherwise.
const (
	fileAssign  = "assign/assign.go"
	fileChans   = "chans/chans.go"
	fileDocs    = "docs/docs.go"
	fileIfcalls = "ifcalls/ifcalls.go"
	fileVars    = "vars/vars.go"

	dirOutsideScope = "scoped/out/"

	kindRead            = "read"
	kindWrite           = "write"
	kindClosure         = "closure"
	kindInterfaceMethod = "interface_method"
	kindGo              = "go"

	subKindAlias   = "alias"
	subKindDefined = "defined"
	subKindRWMutex = "rwmutex"
)

// fixtureLine returns the line of the fixture file that contains marker.
func fixtureLine(t *testing.T, file, marker string) int {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(fidelityRoot, file))
	if err != nil {
		t.Fatalf("read fixture %s: %v", file, err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, marker) {
			return i + 1
		}
	}
	t.Fatalf("fixture %s holds no line with %q", file, marker)
	return 0
}

// between returns the edges of type et from src to dst.
func (g *fidelityGraph) between(et types.EdgeType, src, dst string) []types.Edge {
	var out []types.Edge
	for _, e := range g.edges {
		if e.Type == et && e.Src == src && e.Dst == dst {
			out = append(out, e)
		}
	}
	return out
}

// kindsAt returns the dispatch kinds of the edges of type et from src to dst
// on line.
func (g *fidelityGraph) kindsAt(et types.EdgeType, src, dst string, line int) map[string]bool {
	out := map[string]bool{}
	for _, e := range g.between(et, src, dst) {
		if e.Line == line {
			out[e.DispatchKind] = true
		}
	}
	return out
}

// named returns the only node of type nt called name in the file ending in
// fileSuffix whose lines lie within [from, to].
func (g *fidelityGraph) named(t *testing.T, nt types.NodeType, name, fileSuffix string, from, to int) types.Node {
	t.Helper()
	var found []types.Node
	for _, n := range g.inside(nt, fileSuffix, from, to) {
		if n.Name == name {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s %q in %s lines %d-%d: got %d nodes, want 1", nt, name, fileSuffix, from, to, len(found))
	}
	return found[0]
}

// externalByCanonical returns the External node with canonical id cid.
func (g *fidelityGraph) externalByCanonical(t *testing.T, cid string) types.Node {
	t.Helper()
	for _, n := range g.nodes {
		if n.Type == types.NodeExternal && n.CanonicalID == cid {
			return n
		}
	}
	t.Fatalf("no External node with canonical id %q", cid)
	return types.Node{}
}

func checkRedeclareWrite(t *testing.T, g *fidelityGraph) {
	fn := g.one(t, types.NodeFunction, "assign.Redeclare")
	errVar := g.one(t, types.NodeLocalVariable, "assign.Redeclare.err")
	line := fixtureLine(t, fileAssign, "err is assigned here")
	kinds := g.kindsAt(types.EdgeReferences, fn.ID, errVar.ID, line)
	if !kinds[kindWrite] {
		t.Errorf("`b, err := step()` on line %d does not write err: kinds %v", line, kinds)
	}
	if kinds[kindRead] {
		t.Errorf("`b, err := step()` on line %d is recorded as a read of err", line)
	}
}

func checkCrossFileLock(t *testing.T, g *fidelityGraph) {
	mu := g.one(t, types.NodeMutex, "locks.Store.mu#mutex")
	bump := g.one(t, types.NodeMethod, "locks.Store.Bump")
	for _, et := range []types.EdgeType{types.EdgeAcquiresLock, types.EdgeReleasesLock} {
		if !g.has(et, bump.ID, mu.ID) {
			t.Errorf("no %s edge from %s to %s, which another file declares", et, bump.QualifiedName, mu.QualifiedName)
		}
	}
}

func checkMutexFromValue(t *testing.T, g *fidelityGraph) {
	registry := g.one(t, types.NodeMutex, "locks.registry")
	if registry.SubKind != subKindRWMutex {
		t.Errorf("registry is a %q, want %q", registry.SubKind, subKindRWMutex)
	}
	made := g.one(t, types.NodeMutex, "locks.Guard.made")
	for _, c := range []struct {
		fn    string
		mutex types.Node
	}{{"locks.Scan", registry}, {"locks.Guard", made}} {
		fn := g.one(t, types.NodeFunction, c.fn)
		for _, et := range []types.EdgeType{types.EdgeAcquiresLock, types.EdgeReleasesLock} {
			if !g.has(et, fn.ID, c.mutex.ID) {
				t.Errorf("no %s edge from %s to %s", et, c.fn, c.mutex.QualifiedName)
			}
		}
	}
}

func checkValueSpecTypeRefs(t *testing.T, g *fidelityGraph) {
	for _, c := range []struct {
		nt        types.NodeType
		src       string
		dstType   types.NodeType
		dst, what string
	}{
		{types.NodeVariable, "vars.Origin", types.NodeStruct, "vars.Point", "its initialiser is a Point literal"},
		{types.NodeConstant, "vars.Fast", types.NodeTypeAlias, "vars.Mode", "Mode is its declared type"},
		{types.NodeVariable, "vars.Shared", types.NodeStruct, "vars.Point", "*Point is its declared type"},
	} {
		src, dst := g.one(t, c.nt, c.src), g.one(t, c.dstType, c.dst)
		if !g.has(types.EdgeUsesType, src.ID, dst.ID) {
			t.Errorf("no uses_type edge from %s to %s: %s", c.src, c.dst, c.what)
		}
	}
}

func checkLocalTypeRef(t *testing.T, g *fidelityGraph) {
	inner := g.one(t, types.NodeFunction, "vars.Inner")
	pair := g.one(t, types.NodeStruct, "vars.Inner.pair")
	if !g.has(types.EdgeUsesType, inner.ID, pair.ID) {
		t.Errorf("no uses_type edge from %s to the type %s it declares and uses", inner.QualifiedName, pair.QualifiedName)
	}
}

func checkAliasInstantiates(t *testing.T, g *fidelityGraph) {
	mk := g.one(t, types.NodeFunction, "vars.Make")
	short := g.one(t, types.NodeTypeAlias, "vars.Short")
	if !g.has(types.EdgeInstantiates, mk.ID, short.ID) {
		t.Errorf("no instantiates edge from %s to %s: new(Short) names the alias", mk.QualifiedName, short.QualifiedName)
	}
}

func checkTypeRefLine(t *testing.T, g *fidelityGraph) {
	wide := g.one(t, types.NodeFunction, "vars.Wide")
	for _, c := range []struct {
		nt            types.NodeType
		qname, marker string
	}{
		{types.NodeTypeAlias, "vars.Mode", "Mode is named here"},
		{types.NodeStruct, "vars.Point", "Point is named here"},
	} {
		dst := g.one(t, c.nt, c.qname)
		want := fixtureLine(t, fileVars, c.marker)
		edges := g.between(types.EdgeUsesType, wide.ID, dst.ID)
		if len(edges) != 1 {
			t.Errorf("uses_type edges from %s to %s: got %d, want 1", wide.QualifiedName, c.qname, len(edges))
			continue
		}
		if edges[0].Line != want {
			t.Errorf("uses_type from %s to %s is on line %d, want %d (the `func` keyword is on line %d)",
				wide.QualifiedName, c.qname, edges[0].Line, want, wide.StartLine)
		}
	}
}

func checkErrorMethodCall(t *testing.T, g *fidelityGraph) {
	text := g.one(t, types.NodeFunction, "ifcalls.Text")
	errMethod := g.externalByCanonical(t, gop.BuiltinErrorMethodID)
	edges := g.between(types.EdgeInvokes, text.ID, errMethod.ID)
	if len(edges) != 1 || edges[0].DispatchKind != kindInterfaceMethod {
		t.Errorf("invokes edges from %s to error.Error: %+v, want one of kind %q", text.QualifiedName, edges, kindInterfaceMethod)
	}
}

func checkAliasReceiver(t *testing.T, g *fidelityGraph) {
	one := g.one(t, types.NodeMethod, "ifcalls.E.One")
	if one.CanonicalID == "" {
		t.Errorf("%s, a method on an alias receiver, has no canonical id", one.QualifiedName)
	}
	newE := g.one(t, types.NodeFunction, "ifcalls.NewE")
	if !g.has(types.EdgeCalls, newE.ID, one.ID) {
		t.Errorf("no calls edge from %s to %s", newE.QualifiedName, one.QualifiedName)
	}
}

func checkLocalInterfaceCalls(t *testing.T, g *fidelityGraph) {
	for _, c := range []struct{ fn, method string }{
		{"ifcalls.Threads", "SetThreads"}, // interface declared in the body
		{"ifcalls.Temporary", "Temporary"},
		{"ifcalls.Slot", "Bytes"}, // interface written in the signature
	} {
		fn := g.one(t, types.NodeFunction, c.fn)
		m := g.named(t, types.NodeMethod, c.method, fileIfcalls, fn.StartLine, fn.EndLine)
		if !strings.HasPrefix(m.QualifiedName, c.fn+".") {
			t.Errorf("method %s is not named under %s", m.QualifiedName, c.fn)
		}
		edges := g.between(types.EdgeInvokes, fn.ID, m.ID)
		if len(edges) != 1 || edges[0].DispatchKind != kindInterfaceMethod {
			t.Errorf("invokes edges from %s to %s: %+v, want one of kind %q", c.fn, m.QualifiedName, edges, kindInterfaceMethod)
		}
		linked := false
		for _, s := range g.inside(types.NodeCallSite, fileIfcalls, fn.StartLine, fn.EndLine) {
			linked = linked || g.has(types.EdgeSiteCalls, s.ID, m.ID)
		}
		if !linked {
			t.Errorf("no site_calls edge from a call site of %s to %s", c.fn, m.QualifiedName)
		}
	}
}

func checkConstraintMethod(t *testing.T, g *fidelityGraph) {
	set := g.one(t, types.NodeStruct, "ifcalls.Set")
	key := g.named(t, types.NodeMethod, "Key", fileIfcalls, set.StartLine, set.EndLine)
	// The method belongs to the declaration that holds the interface, as
	// the method of an interface written in a signature belongs to its
	// function.
	if !g.has(types.EdgeDefines, set.ID, key.ID) {
		t.Errorf("%s, a method of the constraint of Set's type parameter, is not defined by %s", key.QualifiedName, set.QualifiedName)
	}
	keys := g.one(t, types.NodeMethod, "ifcalls.Set.Keys")
	if edges := g.between(types.EdgeInvokes, keys.ID, key.ID); len(edges) != 1 || edges[0].DispatchKind != kindInterfaceMethod {
		t.Errorf("invokes edges from %s to %s: %+v, want one of kind %q", keys.QualifiedName, key.QualifiedName, edges, kindInterfaceMethod)
	}
}

func checkInitialiserClosure(t *testing.T, g *fidelityGraph) {
	version := g.one(t, types.NodeVariable, "ifcalls.Version")
	edges := g.between(types.EdgeInvokes, version.ID, version.ID)
	if len(edges) != 1 || edges[0].DispatchKind != kindClosure {
		t.Errorf("marker edges on %s for the closure it calls: %+v, want one of kind %q", version.QualifiedName, edges, kindClosure)
	}
}

func checkEmbeddedInterfaceCall(t *testing.T, g *fidelityGraph) {
	pull := g.one(t, types.NodeMethod, "ifcalls.Wrap.Pull")
	read := g.one(t, types.NodeMethod, "ifcalls.Reader.Read")
	edges := g.between(types.EdgeInvokes, pull.ID, read.ID)
	if len(edges) != 1 || edges[0].DispatchKind != kindInterfaceMethod {
		t.Errorf("invokes edges from %s to %s: %+v, want one of kind %q", pull.QualifiedName, read.QualifiedName, edges, kindInterfaceMethod)
	}
	if g.has(types.EdgeCalls, pull.ID, read.ID) {
		t.Errorf("%s reaches %s through an embedded interface, and the graph has a static calls edge", pull.QualifiedName, read.QualifiedName)
	}
}

func checkTypeSwitchVariable(t *testing.T, g *fidelityGraph) {
	kind := g.one(t, types.NodeFunction, "assign.Kind")
	v := g.one(t, types.NodeLocalVariable, "assign.Kind.t")
	if !g.has(types.EdgeDefines, kind.ID, v.ID) {
		t.Errorf("%s is not defined by %s", v.QualifiedName, kind.QualifiedName)
	}
	for _, marker := range []string{"t is an int here", "t is a string here"} {
		line := fixtureLine(t, fileAssign, marker)
		if !g.kindsAt(types.EdgeReferences, kind.ID, v.ID, line)[kindRead] {
			t.Errorf("no read of the type switch variable t on line %d", line)
		}
	}
}

func checkConstantReads(t *testing.T, g *fidelityGraph) {
	local := g.one(t, types.NodeFunction, "assign.Local")
	limit := g.one(t, types.NodeConstant, "assign.Local.limit")
	if line := fixtureLine(t, fileAssign, "limit is read here"); !g.kindsAt(types.EdgeReferences, local.ID, limit.ID, line)[kindRead] {
		t.Errorf("no read of the local constant limit on line %d", line)
	}
	size := g.one(t, types.NodeConstant, "assign.Size")
	for _, c := range []struct {
		nt         types.NodeType
		qname, why string
	}{
		{types.NodeFunction, "assign.Sum", "its result type is [Size]byte"},
		{types.NodeTypeAlias, "assign.Digest", "it is declared as [Size]byte"},
	} {
		src := g.one(t, c.nt, c.qname)
		edges := g.between(types.EdgeReferences, src.ID, size.ID)
		if len(edges) != 1 || edges[0].DispatchKind != kindRead || edges[0].Line != src.StartLine {
			t.Errorf("references edges from %s to Size: %+v, want one read on line %d: %s", c.qname, edges, src.StartLine, c.why)
		}
	}
}

func checkChannelNodes(t *testing.T, g *fidelityGraph) {
	work := g.one(t, types.NodeFunction, "chans.Work")
	got := map[string]bool{}
	for _, c := range g.inside(types.NodeChannel, fileChans, work.StartLine, work.EndLine) {
		got[c.Signature] = true
	}
	// make(signal) is written with the named channel type and keeps it.
	for _, want := range []string{"signal", "chan []byte (buf=1)", "chan *int"} {
		if !got[want] {
			t.Errorf("no Channel node with signature %q in Work; got %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("Channel nodes in Work: %v, want 3 (a named channel type, a slice element, one made in a go literal)", got)
	}
}

func checkRangeOverChannel(t *testing.T, g *fidelityGraph) {
	work := g.one(t, types.NodeFunction, "chans.Work")
	line := fixtureLine(t, fileChans, "ranges over a channel")
	var sites []types.Node
	for _, s := range g.inside(types.NodeCallSite, fileChans, line, line) {
		if s.SubKind == subKindRecv {
			sites = append(sites, s)
		}
	}
	if len(sites) != 1 {
		t.Fatalf("receive sites on line %d (for v := range in): got %d, want 1", line, len(sites))
	}
	if !g.has(types.EdgeRecvsFrom, work.ID, sites[0].ID) {
		t.Errorf("no recvs_from edge from %s to the range over the channel on line %d", work.QualifiedName, line)
	}
}

func checkEncloses(t *testing.T, g *fidelityGraph) {
	stmtAt := func(nt types.NodeType, marker string) types.Node {
		line := fixtureLine(t, fileChans, marker)
		found := g.inside(nt, fileChans, line, 1<<30)
		for _, n := range found {
			if n.StartLine == line {
				return n
			}
		}
		t.Fatalf("no %s node starting on line %d", nt, line)
		return types.Node{}
	}
	enclosers := func(child types.Node) map[string]string {
		out := map[string]string{}
		for _, e := range g.edges {
			if e.Type == types.EdgeEncloses && e.Dst == child.ID {
				out[e.Src] = e.DispatchKind
			}
		}
		return out
	}
	only := func(what string, child, parent types.Node, kind string) {
		got := enclosers(child)
		if len(got) != 1 || got[parent.ID] != kind {
			t.Errorf("%s: enclosed by %d statements, want only the %s on line %d with kind %q (got kinds %v)",
				what, len(got), parent.Type, parent.StartLine, kind, got)
		}
	}

	// A send site lies in the if statement around it.
	sendLine := fixtureLine(t, fileChans, "sends inside an if")
	var send types.Node
	for _, s := range g.inside(types.NodeCallSite, fileChans, sendLine, sendLine) {
		if s.SubKind == subKindSend {
			send = s
		}
	}
	if send.ID == "" {
		t.Fatalf("no send site on line %d", sendLine)
	}
	only("send site in an if", send, stmtAt(types.NodeIfStmt, "if v > 0 {"), "then")

	// A statement in a function literal lies in the statement around the literal.
	only("if inside a literal inside a loop", stmtAt(types.NodeIfStmt, "inside a literal inside a loop"),
		stmtAt(types.NodeLoopStmt, "for i := range items"), "body")

	// A call in the arguments of a go statement is made by that statement.
	goLine := fixtureLine(t, fileChans, "next is called by the go statement")
	routine := stmtAt(types.NodeGoroutine, "next is called by the go statement")
	sites := g.inside(types.NodeCallSite, fileChans, goLine, goLine)
	if len(sites) != 1 {
		t.Fatalf("call sites on line %d (go add(next(total))): got %d, want 1 (next; add is the goroutine)", goLine, len(sites))
	}
	only("call in the arguments of a go statement", sites[0], routine, kindGo)
}

func checkDocComments(t *testing.T, g *fidelityGraph) {
	for _, c := range []struct {
		nt         types.NodeType
		qname, doc string
	}{
		{types.NodeConstant, "docs.Limit", "Limit is documented on a single const declaration."},
		{types.NodeVariable, "docs.Name", "Name is documented on a single var declaration."},
		{types.NodeVariable, "docs.Grouped", "Grouped is documented on its own specification."},
		{types.NodeVariable, "docs.Use.count", "count is documented inside a body."},
		{types.NodeStruct, "docs.First", "First has its own comment."},
		{types.NodeStruct, "docs.Second", ""},
		{types.NodeStruct, "docs.Third", "Third has its own comment in a group without one."},
		{types.NodeStruct, "docs.Lone", "Lone is documented on a single type declaration."},
	} {
		if got := g.one(t, c.nt, c.qname).DocComment; got != c.doc {
			t.Errorf("doc comment of %s: got %q, want %q", c.qname, got, c.doc)
		}
	}
}

func checkTypeAliasSubKind(t *testing.T, g *fidelityGraph) {
	for qname, want := range map[string]string{"assign.Digest": subKindDefined, "assign.Alias": subKindAlias, "vars.Mode": subKindDefined, "vars.Short": subKindAlias} {
		if got := g.one(t, types.NodeTypeAlias, qname).SubKind; got != want {
			t.Errorf("sub kind of %s: got %q, want %q", qname, got, want)
		}
	}
}

func checkInitialiserInstantiates(t *testing.T, g *fidelityGraph) {
	origin := g.one(t, types.NodeVariable, "vars.Origin")
	point := g.one(t, types.NodeStruct, "vars.Point")
	if !g.has(types.EdgeInstantiates, origin.ID, point.ID) {
		t.Errorf("no instantiates edge from %s to %s: its initialiser is a Point literal", origin.QualifiedName, point.QualifiedName)
	}
}

func checkGoLiteralChannelOp(t *testing.T, g *fidelityGraph) {
	work := g.one(t, types.NodeFunction, "chans.Work")
	line := fixtureLine(t, fileChans, "sends on a local channel inside a go literal")
	var raw types.Node
	for _, c := range g.inside(types.NodeChannel, fileChans, work.StartLine, work.EndLine) {
		if c.Signature == "chan []byte (buf=1)" {
			raw = c
		}
	}
	found := false
	// The edge the source states, not the one the runtime-link pass infers.
	for _, e := range g.between(types.EdgeSendsTo, work.ID, raw.ID) {
		found = found || (e.Line == line && e.Confidence == types.ConfExtracted)
	}
	if !found {
		t.Errorf("no extracted sends_to edge from %s to the channel raw on line %d (a send inside a go literal): %+v",
			work.QualifiedName, line, g.between(types.EdgeSendsTo, work.ID, raw.ID))
	}
}

// A parameter called like a local channel is another variable: an operation
// on it is a site, not an operation on the channel the local was made with.
func checkShadowedChannel(t *testing.T, g *fidelityGraph) {
	work := g.one(t, types.NodeFunction, "chans.Work")
	line := fixtureLine(t, fileChans, "sends on the parameter that hides the local channel")
	sites := 0
	for _, s := range g.inside(types.NodeCallSite, fileChans, line, line) {
		if s.SubKind == subKindSend {
			sites++
		}
	}
	if sites != 1 {
		t.Errorf("send sites on line %d: got %d, want 1", line, sites)
	}
	for _, e := range g.edges {
		if e.Type == types.EdgeSendsTo && e.Src == work.ID && e.Line == line && e.Confidence == types.ConfExtracted && g.byID[e.Dst].Type == types.NodeChannel {
			t.Errorf("the send on the parameter on line %d is recorded as a send on the channel made on line %d", line, g.byID[e.Dst].StartLine)
		}
	}
}

func checkHeaderLiteralRead(t *testing.T, g *fidelityGraph) {
	ceiling := g.one(t, types.NodeVariable, "chans.ceiling")
	line := fixtureLine(t, fileChans, "ceiling is read in the header")
	var loop types.Node
	for _, n := range g.inside(types.NodeLoopStmt, fileChans, line, 1<<30) {
		if n.StartLine == line {
			loop = n
		}
	}
	if loop.ID == "" {
		t.Fatalf("no LoopStmt on line %d", line)
	}
	if edges := g.between(types.EdgeCondReads, loop.ID, ceiling.ID); len(edges) != 1 || edges[0].Line != line {
		t.Errorf("cond_reads edges from the loop on line %d to %s: %+v, want one on that line (the read is in a function literal in the header)",
			line, ceiling.QualifiedName, edges)
	}
}

func checkLockThroughCall(t *testing.T, g *fidelityGraph) {
	mu := g.one(t, types.NodeMutex, "locks.Store.mu#mutex")
	reset := g.one(t, types.NodeMethod, "locks.Store.Reset")
	for _, et := range []types.EdgeType{types.EdgeAcquiresLock, types.EdgeReleasesLock} {
		if !g.has(et, reset.ID, mu.ID) {
			t.Errorf("no %s edge from %s to %s: s.guard() returns &s.mu", et, reset.QualifiedName, mu.QualifiedName)
		}
	}
}

// spanText returns the source text a node spans.
func spanText(t *testing.T, n types.Node) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(fidelityRoot, n.FilePath))
	if err != nil {
		t.Fatalf("read %s: %v", n.FilePath, err)
	}
	if n.StartByte < 0 || n.EndByte > len(src) || n.StartByte > n.EndByte {
		t.Fatalf("%s spans bytes %d-%d of a %d byte file", n.QualifiedName, n.StartByte, n.EndByte, len(src))
	}
	return string(src[n.StartByte:n.EndByte])
}

func checkDeclarationSpans(t *testing.T, g *fidelityGraph) {
	for _, c := range []struct {
		nt          types.NodeType
		qname, text string
	}{
		// A declaration that stands alone starts at its keyword.
		{types.NodeConstant, "docs.Limit", "const Limit = 10"},
		{types.NodeVariable, "docs.Name", `var Name = "x"`},
		{types.NodeStruct, "docs.Lone", "type Lone struct{}"},
		// Inside a function body too.
		{types.NodeVariable, "docs.Use.count", "var count = Limit"},
		{types.NodeStruct, "docs.Use.pair", "type pair struct{ a, b int }"},
		{types.NodeConstant, "docs.Use.step", "const step = 1"},
		// In a group the specification is the declaration.
		{types.NodeVariable, "docs.Grouped", "Grouped = 1"},
		{types.NodeStruct, "docs.First", "First struct{}"},
		// Every name of a specification holds the whole specification.
		{types.NodeVariable, "docs.first", "var first, second = 1, 2"},
		{types.NodeVariable, "docs.second", "var first, second = 1, 2"},
		// A function starts at its keyword, as before.
		{types.NodeFunction, "assign.Local", "func Local() int {\n\tconst limit = 3\n\treturn limit + 1 // limit is read here\n}"},
		// A function or method runs through the comment after its closing
		// brace on that line, which the syntax tree attaches to nothing.
		{types.NodeFunction, "docs.Listening", "func Listening() bool { return true } // Always listening."},
		{types.NodeFunction, "docs.Closing", "func Closing() int {\n\treturn 1\n} // Closing ends here."},
		{types.NodeMethod, "docs.Pair.Total", "func (p Pair) Total() int { return p.Left + p.Right } // Total is the sum."},
		// The comment on the next line is the next declaration's.
		{types.NodeFunction, "docs.Bare", "func Bare() {}"},
	} {
		if got := spanText(t, g.one(t, c.nt, c.qname)); got != c.text {
			t.Errorf("text of %s: got %q, want %q", c.qname, got, c.text)
		}
	}
	// The two names of one specification stay two nodes.
	if a, b := g.one(t, types.NodeVariable, "docs.first"), g.one(t, types.NodeVariable, "docs.second"); a.ID == b.ID {
		t.Errorf("docs.first and docs.second share the node id %s", a.ID)
	}
	// Uses still find a declaration whose span starts before its name.
	use := g.one(t, types.NodeFunction, "docs.Use")
	for _, c := range []struct {
		nt    types.NodeType
		qname string
		et    types.EdgeType
	}{
		{types.NodeVariable, "docs.Use.count", types.EdgeReferences},
		{types.NodeConstant, "docs.Use.step", types.EdgeReferences},
		{types.NodeVariable, "docs.second", types.EdgeReferences},
		{types.NodeStruct, "docs.Use.pair", types.EdgeUsesType},
	} {
		if !g.has(c.et, use.ID, g.one(t, c.nt, c.qname).ID) {
			t.Errorf("no %s edge from %s to %s", c.et, use.QualifiedName, c.qname)
		}
	}
}

// all returns the nodes of type nt with qualified name qname.
func (g *fidelityGraph) all(nt types.NodeType, qname string) []types.Node {
	var out []types.Node
	for _, n := range g.byQname[qname] {
		if n.Type == nt {
			out = append(out, n)
		}
	}
	return out
}

func checkSpecNamePositions(t *testing.T, g *fidelityGraph) {
	// Two blank names of one specification share everything but their
	// place among the names.
	blanks := g.all(types.NodeVariable, "docs._")
	if len(blanks) != 2 {
		t.Fatalf("blank variables of `var _, _ = 1, 2`: got %d nodes, want 2", len(blanks))
	}
	got := map[string]bool{}
	for _, n := range blanks {
		got[n.Signature] = true
	}
	if !got["name 1 of 2"] || !got["name 2 of 2"] {
		t.Errorf("signatures of the two blank names: %v, want name 1 of 2 and name 2 of 2", got)
	}
	for qname, want := range map[string]string{"docs.first": "name 1 of 2", "docs.second": "name 2 of 2", "docs.Name": ""} {
		if sig := g.one(t, types.NodeVariable, qname).Signature; sig != want {
			t.Errorf("signature of %s: got %q, want %q", qname, sig, want)
		}
	}
	for qname, want := range map[string]string{"docs.Pair.Left": "name 1 of 2", "docs.Pair.Right": "name 2 of 2", "docs.Pair.Sum": ""} {
		if sig := g.one(t, types.NodeField, qname).Signature; sig != want {
			t.Errorf("signature of %s: got %q, want %q", qname, sig, want)
		}
	}
}

func checkMutexSpans(t *testing.T, g *fidelityGraph) {
	for _, c := range []struct {
		mutex string
		nt    types.NodeType
		decl  string
	}{
		{"locks.registry", types.NodeVariable, "locks.registry"},
		{"locks.Pipe.rmu#mutex", types.NodeField, "locks.Pipe.rmu"},
		{"locks.Pipe.wmu#mutex", types.NodeField, "locks.Pipe.wmu"},
		{"locks.Store.mu#mutex", types.NodeField, "locks.Store.mu"},
	} {
		mu, decl := g.one(t, types.NodeMutex, c.mutex), g.one(t, c.nt, c.decl)
		if mu.StartByte != decl.StartByte || mu.EndByte != decl.EndByte {
			t.Errorf("Mutex %s spans %q, its %s spans %q", c.mutex, spanText(t, mu), c.nt, spanText(t, decl))
		}
	}
	if got, want := spanText(t, g.one(t, types.NodeMutex, "locks.registry")), "var registry = sync.RWMutex{}"; got != want {
		t.Errorf("text of the Mutex locks.registry: got %q, want %q", got, want)
	}
	// The second mutex of the field is still the one its lock finds.
	write := g.one(t, types.NodeMethod, "locks.Pipe.Write")
	if !g.has(types.EdgeAcquiresLock, write.ID, g.one(t, types.NodeMutex, "locks.Pipe.wmu#mutex").ID) {
		t.Errorf("no acquires_lock edge from %s to wmu", write.QualifiedName)
	}
	if g.has(types.EdgeAcquiresLock, write.ID, g.one(t, types.NodeMutex, "locks.Pipe.rmu#mutex").ID) {
		t.Errorf("%s locks wmu, and the graph has an acquires_lock edge to rmu", write.QualifiedName)
	}
}

func checkTrailingComments(t *testing.T, g *fidelityGraph) {
	for _, c := range []struct {
		nt          types.NodeType
		qname, text string
	}{
		{types.NodeConstant, "docs.Depth", "const Depth = 3 // Depth has a comment after it."},
		{types.NodeField, "docs.Pair.Left", "Left, Right int // Left and Right share a field declaration."},
		{types.NodeField, "docs.Pair.Right", "Left, Right int // Left and Right share a field declaration."},
		{types.NodeField, "docs.Pair.Sum", "Sum         int"},
		{types.NodeMethod, "docs.Walker.Walk", "Walk() int // Walk has a comment after it."},
		{types.NodeMutex, "locks.Pipe.wmu#mutex", "rmu, wmu sync.Mutex // one for each direction"},
	} {
		if got := spanText(t, g.one(t, c.nt, c.qname)); got != c.text {
			t.Errorf("text of %s: got %q, want %q", c.qname, got, c.text)
		}
	}
	if got, want := spanText(t, g.one(t, types.NodeStruct, "docs.Pair")), "} // Pair ends here."; !strings.HasSuffix(got, want) {
		t.Errorf("text of docs.Pair ends with %q, want %q", got[max(0, len(got)-len(want)):], want)
	}
}

// loadFidelityGraphWith loads the fixture with opts.
func loadFidelityGraphWith(t *testing.T, opts gop.LoadOptions) *fidelityGraph {
	t.Helper()
	root, err := filepath.Abs(fidelityRoot)
	if err != nil {
		t.Fatalf("abs %s: %v", fidelityRoot, err)
	}
	rg, err := gop.LoadAndResolveWith(root, opts)
	if err != nil {
		t.Fatalf("LoadAndResolveWith(%s): %v", root, err)
	}
	return newFidelityGraph(rg.Nodes, rg.Edges)
}

// TestFidelityTypeHierarchy loads the fixture as a build does, with test
// files: a package with tests is then type-checked twice, and a type a test
// file declares belongs to the second copy.
func TestFidelityTypeHierarchy(t *testing.T) {
	g := loadFidelityGraphWith(t, gop.LoadOptions{Tests: true})
	decoder := g.one(t, types.NodeInterface, "impl.Decoder")
	sized := g.one(t, types.NodeInterface, "impl.Hold.sized")
	cases := []struct {
		name     string
		nt       types.NodeType
		concrete string
		iface    types.Node
	}{
		{"a type declared in a test file", types.NodeStruct, "impl.testDecoder", decoder},
		{"a type declared in a function", types.NodeStruct, "impl.Hold.local", decoder},
		{"an interface declared in a function", types.NodeStruct, "impl.Fixed", sized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := g.one(t, tc.nt, tc.concrete)
			if !g.has(types.EdgeImplements, c.ID, tc.iface.ID) {
				t.Errorf("no implements edge from %s to %s", tc.concrete, tc.iface.QualifiedName)
			}
		})
	}
	// A package is shown by one of its production files, the same one on
	// every build, and by a test file only when it has no other.
	t.Run("the file a package node shows", func(t *testing.T) {
		for name, want := range map[string]string{"impl": "impl/impl.go", "twice": "twice/twice.go", "onlytest": "onlytest/only_test.go"} {
			n := 0
			for _, pkg := range g.nodes {
				if pkg.Type != types.NodePackage || pkg.Name != name {
					continue
				}
				n++
				if got := filepath.ToSlash(pkg.FilePath); got != want {
					t.Errorf("package %s is shown by %s, want %s", name, got, want)
				}
			}
			if n == 0 {
				t.Errorf("no Package node %s", name)
			}
		}
	})
	t.Run("an embedded interface from outside the build", func(t *testing.T) {
		src := g.one(t, types.NodeInterface, "impl.ByteSource")
		reader := g.externalByCanonical(t, "io.Reader")
		if !g.has(types.EdgeExtends, src.ID, reader.ID) {
			t.Errorf("no extends edge from %s to the External node of io.Reader", src.QualifiedName)
		}
	})
}

// TestFidelityOutsideScope builds the fixture without one of its packages, as
// a build scope leaves packages of the module out: what the build uses of that
// package is outside code and gets External nodes, as the standard library does.
func TestFidelityOutsideScope(t *testing.T) {
	g := loadFidelityGraphWith(t, gop.LoadOptions{Skip: func(rel string) bool { return strings.HasPrefix(rel, dirOutsideScope) }})
	for _, n := range g.nodes {
		if strings.HasPrefix(filepath.ToSlash(n.FilePath), dirOutsideScope) {
			t.Fatalf("%s %s is in the graph, but its file %s was left out", n.Type, n.QualifiedName, n.FilePath)
		}
	}
	use := g.one(t, types.NodeFunction, "in.Use")
	const pkg = "ckgfidelity.test/scoped/out"
	cases := []struct {
		name, cid string
		et        types.EdgeType
	}{
		{"a function", pkg + ".Helper", types.EdgeCalls},
		{"a method", pkg + ".(T).Twice", types.EdgeCalls},
		{"a type", pkg + ".T", types.EdgeUsesType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext := g.externalByCanonical(t, tc.cid)
			if !g.has(tc.et, use.ID, ext.ID) {
				t.Errorf("no %s edge from %s to the External node %s", tc.et, use.QualifiedName, tc.cid)
			}
			if ext.External == nil || ext.External.Path != pkg {
				t.Errorf("External node %s names package %+v, want path %s", tc.cid, ext.External, pkg)
			}
		})
	}
}

// checkUnderLock: accessed_under_lock names a field used where the mutex is
// held, not every field of a function that locks it somewhere. On
// go-stablenet 545 of 1,761 edges named a field used before the lock, after
// the unlock or in another branch (TypeMuxSubscription.closeMu under postMu).
func checkUnderLock(t *testing.T, g *fidelityGraph) {
	mu := g.one(t, types.NodeMutex, "locks.Ledger.mu#mutex")
	other := g.one(t, types.NodeMutex, "locks.Ledger.other#mutex")
	rw := g.one(t, types.NodeMutex, "locks.Ledger.rw#mutex")
	want := map[string]string{
		"inside": mu.ID, "deferred": mu.ID, "inPlace": mu.ID, "looped": mu.ID,
		"relocked": mu.ID, "early": mu.ID, "rlocked": rw.ID,
	}
	for _, name := range []string{"before", "inside", "after", "branch", "deferred", "literal", "inPlace", "looped",
		"relocked", "early", "rlocked", "jumped", "switched", "litOuter", "afterCall"} {
		field := g.one(t, types.NodeField, "locks.Ledger."+name)
		got := map[string]bool{}
		for _, m := range []types.Node{mu, other, rw} {
			if g.has(types.EdgeAccessedUnderLock, field.ID, m.ID) {
				got[m.ID] = true
			}
		}
		wantSet := map[string]bool{}
		if id, ok := want[name]; ok {
			wantSet[id] = true
		}
		if len(got) != len(wantSet) || !got[want[name]] && len(wantSet) > 0 {
			t.Errorf("Ledger.%s is under %v, want %v (mu %s, other %s, rw %s)", name, got, wantSet, mu.ID, other.ID, rw.ID)
		}
	}
	// A struct literal sets its fields, by key and by place: a use like a
	// selector (180 such pairs of go-stablenet had no edge). The receiver path
	// of a deferred Unlock is read where the defer stands, under mu.
	for _, q := range []string{"locks.entry.key", "locks.entry.value", "locks.Outer.inner"} {
		if f := g.one(t, types.NodeField, q); !g.has(types.EdgeAccessedUnderLock, f.ID, mu.ID) {
			t.Errorf("%s is not under mu", q)
		}
	}
	// Locking other while mu is held uses the field other under mu, the order
	// the two are taken in; no mutex field is used under itself.
	otherField, muField := g.one(t, types.NodeField, "locks.Ledger.other"), g.one(t, types.NodeField, "locks.Ledger.mu")
	if !g.has(types.EdgeAccessedUnderLock, otherField.ID, mu.ID) {
		t.Errorf("Ledger.other is not under mu, though Order locks it while it holds mu")
	}
	for _, c := range [][2]string{{muField.ID, other.ID}, {muField.ID, mu.ID}, {otherField.ID, other.ID}} {
		if g.has(types.EdgeAccessedUnderLock, c[0], c[1]) {
			t.Errorf("unexpected accessed_under_lock %s -> %s", c[0], c[1])
		}
	}
}

// TestLockEdges_UndecidedCounted: a function the accessed_under_lock rule
// cannot follow (Ledger.Jump has a goto) gets no edge and is counted, so a
// build log says how many uses were left out rather than guessing.
func TestLockEdges_UndecidedCounted(t *testing.T) {
	root, err := filepath.Abs(fidelityRoot)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{Dir: root, Mode: packages.NeedName | packages.NeedFiles |
		packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles}, "./locks")
	if err != nil {
		t.Fatal(err)
	}
	_, st := gop.LockFacts(pkgs, rg.Nodes, gop.LockOptions{})
	if st.UndecidedFunctions != 1 {
		t.Errorf("undecided functions = %d, want 1 (Ledger.Jump)", st.UndecidedFunctions)
	}
}

// TestImplementsEdgesInFixedOrder: implements and extends edges come out
// sorted by type, source and target, whatever order packages.Load lists the packages in,
// so a build of one source stores them with the same row ids each time
// (304 edges of go-stablenet changed ids between two builds).
func TestImplementsEdgesInFixedOrder(t *testing.T) {
	root, err := filepath.Abs(fidelityRoot)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{Dir: root, Mode: packages.NeedName | packages.NeedFiles |
		packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	for _, order := range [][]*packages.Package{pkgs, reversed(pkgs)} {
		edges, _ := gop.EmitImplementsEdges(order, rg.Nodes)
		if len(edges) == 0 {
			t.Fatal("no implements edges in the fixture")
		}
		for i := 1; i < len(edges); i++ {
			a, b := edges[i-1], edges[i]
			if a.Type > b.Type || a.Type == b.Type && (a.Src > b.Src || a.Src == b.Src && a.Dst > b.Dst) {
				t.Fatalf("edges out of order at %d: %s %s->%s before %s %s->%s", i, a.Type, a.Src, a.Dst, b.Type, b.Src, b.Dst)
			}
		}
	}
}

func reversed(pkgs []*packages.Package) []*packages.Package {
	out := make([]*packages.Package, len(pkgs))
	for i, p := range pkgs {
		out[len(pkgs)-1-i] = p
	}
	return out
}
