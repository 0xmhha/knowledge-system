package golang_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// The fidelity fixture (testdata/fidelity) holds one small package per way a
// graph was found to differ from its source on go-stablenet: a construct with
// no node, a use tied to the wrong declaration, two constructs merged into one
// node, or a relation counted twice. Each case states what the source holds
// and fails while the graph says otherwise.
const (
	fidelityRoot = "testdata/fidelity"

	fileAnon  = "anon/anon.go"
	fileCalls = "calls/calls.go"
	fileConc  = "conc/conc.go"
	fileGen   = "gen/gen.go"
	fileTwice = "twice/twice.go"

	subKindSend = "send"
	subKindRecv = "recv"
)

// fidelityGraph indexes a resolved graph for the checks below.
type fidelityGraph struct {
	nodes   []types.Node
	edges   []types.Edge
	byID    map[string]types.Node
	byQname map[string][]types.Node
}

func newFidelityGraph(nodes []types.Node, edges []types.Edge) *fidelityGraph {
	g := &fidelityGraph{nodes: nodes, edges: edges, byID: map[string]types.Node{}, byQname: map[string][]types.Node{}}
	for _, n := range nodes {
		g.byID[n.ID] = n
		g.byQname[n.QualifiedName] = append(g.byQname[n.QualifiedName], n)
	}
	return g
}

// one returns the only node of type t with qualified name qname.
func (g *fidelityGraph) one(t *testing.T, nt types.NodeType, qname string) types.Node {
	t.Helper()
	var found []types.Node
	for _, n := range g.byQname[qname] {
		if n.Type == nt {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s %q: got %d nodes, want 1", nt, qname, len(found))
	}
	return found[0]
}

// has reports whether an edge of type et runs from src to dst.
func (g *fidelityGraph) has(et types.EdgeType, src, dst string) bool {
	for _, e := range g.edges {
		if e.Type == et && e.Src == src && e.Dst == dst {
			return true
		}
	}
	return false
}

// inside returns the nodes of type nt in the file ending in fileSuffix whose
// lines lie within [from, to].
func (g *fidelityGraph) inside(nt types.NodeType, fileSuffix string, from, to int) []types.Node {
	var out []types.Node
	for _, n := range g.nodes {
		if n.Type == nt && strings.HasSuffix(filepath.ToSlash(n.FilePath), fileSuffix) && n.StartLine >= from && n.EndLine <= to {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartByte < out[j].StartByte })
	return out
}

func loadFidelityGraph(t *testing.T) *fidelityGraph {
	t.Helper()
	// A build passes an absolute source root; with a relative one every pass
	// that makes paths relative to the root would be checked against the wrong
	// base.
	root, err := filepath.Abs(fidelityRoot)
	if err != nil {
		t.Fatalf("abs %s: %v", fidelityRoot, err)
	}
	rg, err := gop.LoadAndResolve(root)
	if err != nil {
		t.Fatalf("LoadAndResolve(%s): %v", root, err)
	}
	return newFidelityGraph(rg.Nodes, rg.Edges)
}

func TestFidelity(t *testing.T) {
	g := loadFidelityGraph(t)
	cases := []struct {
		name  string
		check func(t *testing.T, g *fidelityGraph)
	}{
		{"embedded field has a Field node", checkEmbeddedField},
		{"fields of an anonymous nested struct have Field nodes", checkNestedFields},
		{"generic receiver method keeps its type name", checkGenericReceiver},
		{"each call of a chain has its own call site", checkChainedCallSites},
		{"calls in a package-level initialiser have a calls edge", checkInitialiserCalls},
		{"statements in a package-level initialiser have nodes", checkInitialiserStatements},
		{"local variable is named under its function", checkLocalVariableName},
		{"write to a local struct field is not tied to the package-level field", checkLocalStructField},
		{"field use through a defined type reaches the struct's field", checkDefinedTypeField},
		{"channel operations inside a go statement have sites", checkGoroutineChannelSites},
		{"handler shape alone does not make a message handler", checkNoHandlerByShape},
		{"edge file paths are relative to the source root", checkRelativeEdgePaths},
		{"fields of anonymous struct types have Field nodes", checkAnonymousStructFields},
		{"uses of anonymous struct fields have edges", checkAnonymousStructFieldUses},
		{"names made by := and range are LocalVariable nodes", checkLocalNames},
		{"uses of a local name have references edges", checkLocalNameUses},
		{"receiver, interface, defined type and body type references have edges", checkTypeReferences},
		{"an existing name on the left of := is written", checkRedeclareWrite},
		{"a lock on a mutex declared in another file has an edge", checkCrossFileLock},
		{"a mutex declared by a composite literal or new has a node", checkMutexFromValue},
		{"type references in package-level var and const declarations have edges", checkValueSpecTypeRefs},
		{"a type declared in a function is the target of a type reference", checkLocalTypeRef},
		{"new of an alias instantiates the alias", checkAliasInstantiates},
		{"a uses_type edge carries the line of the reference", checkTypeRefLine},
		{"a call of the builtin error method has an edge", checkErrorMethodCall},
		{"a method on an alias receiver has a canonical id and is called", checkAliasReceiver},
		{"methods of local and unnamed interfaces have nodes and are called", checkLocalInterfaceCalls},
		{"the method of a type parameter's unnamed constraint has a node and is called", checkConstraintMethod},
		{"a closure called in a package-level initialiser is recorded", checkInitialiserClosure},
		{"a call through an embedded interface is an interface call", checkEmbeddedInterfaceCall},
		{"the variable of a type switch has a node and its uses have edges", checkTypeSwitchVariable},
		{"reads of local constants and of array-length constants have edges", checkConstantReads},
		{"every make(chan) has a Channel node with its element type", checkChannelNodes},
		{"a range over a channel is a receive", checkRangeOverChannel},
		{"statements and channel sites lie in the statement around them", checkEncloses},
		{"single and grouped declarations keep their own doc comment", checkDocComments},
		{"a named type that is not a struct or interface says alias or defined", checkTypeAliasSubKind},
		{"a composite literal in a package-level initialiser instantiates its type", checkInitialiserInstantiates},
		{"a send on a local channel inside a go literal has its edge and line", checkGoLiteralChannelOp},
		{"an operation on a parameter that hides a local channel is a site", checkShadowedChannel},
		{"a read in a function literal in a statement header is a read of the header", checkHeaderLiteralRead},
		{"a lock through a method that returns the mutex has an edge", checkLockThroughCall},
		{"a declaration's span starts at its first token", checkDeclarationSpans},
		{"the names of one specification say which one they are", checkSpecNamePositions},
		{"a Mutex node spans the declaration of its variable or field", checkMutexSpans},
		{"a declaration's span runs through the comment after it", checkTrailingComments},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.check(t, g) })
	}
}

func checkEmbeddedField(t *testing.T, g *fidelityGraph) {
	g.one(t, types.NodeField, "shapes.Unlocked.Key")
}

func checkNestedFields(t *testing.T, g *fidelityGraph) {
	for _, qname := range []string{"shapes.Wire.Sig.R", "shapes.Wire.Sig.S"} {
		g.one(t, types.NodeField, qname)
	}
}

func checkGenericReceiver(t *testing.T, g *fidelityGraph) {
	g.one(t, types.NodeMethod, "shapes.Box.Reset")
	for _, n := range g.nodes {
		if strings.Contains(n.QualifiedName, "..") {
			t.Errorf("%s %q has an empty name segment", n.Type, n.QualifiedName)
		}
	}
}

func checkChainedCallSites(t *testing.T, g *fidelityGraph) {
	chain := g.one(t, types.NodeFunction, "calls.Chain")
	want := []string{"calls.New", "calls.T.Inc", "calls.T.Val"}
	sites := g.inside(types.NodeCallSite, fileCalls, chain.StartLine, chain.EndLine)
	if len(sites) != len(want) {
		t.Fatalf("call sites in Chain: got %d, want %d (New().Inc().Val() is three calls)", len(sites), len(want))
	}
	// The graph keeps one node per ID, so sites that share an ID become one.
	ids := map[string]bool{}
	for _, s := range sites {
		ids[s.ID] = true
	}
	if len(ids) != len(sites) {
		t.Errorf("the %d call sites of the chain have %d distinct node IDs", len(sites), len(ids))
	}
	var got []string
	for _, s := range sites {
		var targets []string
		for _, e := range g.edges {
			if e.Type == types.EdgeSiteCalls && e.Src == s.ID {
				targets = append(targets, g.byID[e.Dst].QualifiedName)
			}
		}
		if len(targets) != 1 {
			t.Errorf("call site at bytes %d-%d calls %v, want exactly one callee", s.StartByte, s.EndByte, targets)
			continue
		}
		got = append(got, targets[0])
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("callees of the chain's sites: got %v, want %v", got, want)
	}
}

func checkInitialiserCalls(t *testing.T, g *fidelityGraph) {
	def := g.one(t, types.NodeVariable, "calls.Default")
	for _, callee := range []struct {
		nt    types.NodeType
		qname string
	}{{types.NodeFunction, "calls.New"}, {types.NodeMethod, "calls.T.Inc"}} {
		dst := g.one(t, callee.nt, callee.qname)
		if !g.has(types.EdgeCalls, def.ID, dst.ID) {
			t.Errorf("no calls edge from %s to %s", def.QualifiedName, callee.qname)
		}
	}
}

func checkInitialiserStatements(t *testing.T, g *fidelityGraph) {
	hook := g.one(t, types.NodeVariable, "calls.Hook")
	for _, nt := range []types.NodeType{types.NodeIfStmt, types.NodeReturnStmt, types.NodeCallSite} {
		if len(g.inside(nt, fileCalls, hook.StartLine, hook.EndLine)) == 0 {
			t.Errorf("no %s node inside the initialiser of %s (lines %d-%d)", nt, hook.QualifiedName, hook.StartLine, hook.EndLine)
		}
	}
	// The call site inside the initialiser is linked to its callee like a
	// site inside a function.
	val := g.one(t, types.NodeMethod, "calls.T.Val")
	linked := false
	for _, s := range g.inside(types.NodeCallSite, fileCalls, hook.StartLine, hook.EndLine) {
		linked = linked || g.has(types.EdgeSiteCalls, s.ID, val.ID)
	}
	if !linked {
		t.Errorf("no site_calls edge from a call site inside %s to %s", hook.QualifiedName, val.QualifiedName)
	}
	// A variable makes calls but is never called: no call edge from another
	// node may end at one. Its own marker for a closure it calls names no
	// callee (checkInitialiserClosure).
	for _, e := range g.edges {
		if (e.Type == types.EdgeCalls || e.Type == types.EdgeInvokes) && g.byID[e.Dst].Type == types.NodeVariable && e.Src != e.Dst {
			t.Errorf("%s edge from %s ends at the variable %s", e.Type, g.byID[e.Src].QualifiedName, g.byID[e.Dst].QualifiedName)
		}
	}
}

func checkLocalVariableName(t *testing.T, g *fidelityGraph) {
	g.one(t, types.NodeVariable, "calls.Scoped.buf")
	if n := g.byQname["calls.buf"]; len(n) != 0 {
		t.Errorf("local variable buf is named %q, as a package-level variable would be", n[0].QualifiedName)
	}
}

func checkLocalStructField(t *testing.T, g *fidelityGraph) {
	marshal := g.one(t, types.NodeMethod, "gen.Tuple.Marshal")
	var pkgLevel, local *types.Node
	for i := range g.nodes {
		n := &g.nodes[i]
		if n.Type != types.NodeField || n.Name != "Address" || !strings.HasSuffix(filepath.ToSlash(n.FilePath), fileGen) {
			continue
		}
		if n.StartLine < marshal.StartLine {
			pkgLevel = n
		} else {
			local = n
		}
	}
	if pkgLevel == nil || local == nil {
		t.Fatalf("Address field nodes: package-level %v, local %v; want both", pkgLevel != nil, local != nil)
	}
	// Marshal reads the package-level field (t.Address) and writes only the
	// local struct's field (enc.Address).
	if g.has(types.EdgeWritesField, marshal.ID, pkgLevel.ID) {
		t.Errorf("Marshal has writes_field to the package-level Tuple.Address (line %d); it only reads it", pkgLevel.StartLine)
	}
	if !g.has(types.EdgeWritesField, marshal.ID, local.ID) {
		t.Errorf("Marshal has no writes_field to the local struct's Address (line %d)", local.StartLine)
	}
	if !g.has(types.EdgeReadsField, marshal.ID, pkgLevel.ID) {
		t.Errorf("Marshal has no reads_field to the package-level Tuple.Address (line %d)", pkgLevel.StartLine)
	}
}

func checkDefinedTypeField(t *testing.T, g *fidelityGraph) {
	set := g.one(t, types.NodeMethod, "gen.Criteria.Set")
	from := g.one(t, types.NodeField, "gen.Query.From")
	if !g.has(types.EdgeWritesField, set.ID, from.ID) {
		t.Errorf("no writes_field from %s to %s: c.From on a Criteria is Query.From", set.QualifiedName, from.QualifiedName)
	}
}

func checkGoroutineChannelSites(t *testing.T, g *fidelityGraph) {
	run := g.one(t, types.NodeMethod, "conc.Hub.Run")
	got := map[string]int{}
	for _, s := range g.inside(types.NodeCallSite, fileConc, run.StartLine, run.EndLine) {
		got[s.SubKind]++
	}
	for _, kind := range []string{subKindSend, subKindRecv} {
		if got[kind] != 1 {
			t.Errorf("%s sites inside the go statement of Run: got %d, want 1", kind, got[kind])
		}
	}
}

func checkNoHandlerByShape(t *testing.T, g *fidelityGraph) {
	for _, e := range g.edges {
		if e.Type == types.EdgeHandlesMessage {
			t.Errorf("handles_message from %s to %s, but no file of the fixture imports net/rpc",
				g.byID[e.Src].QualifiedName, g.byID[e.Dst].QualifiedName)
		}
	}
}

func checkRelativeEdgePaths(t *testing.T, g *fidelityGraph) {
	bad := map[types.EdgeType]int{}
	for _, e := range g.edges {
		if filepath.IsAbs(e.FilePath) {
			bad[e.Type]++
		}
	}
	for et, n := range bad {
		t.Errorf("%d %s edges hold an absolute file path", n, et)
	}
}

// fieldIn returns the only Field node called name in the file ending in
// fileSuffix whose lines lie within [from, to].
func (g *fidelityGraph) fieldIn(t *testing.T, name, fileSuffix string, from, to int) types.Node {
	t.Helper()
	var found []types.Node
	for _, n := range g.inside(types.NodeField, fileSuffix, from, to) {
		if n.Name == name {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		t.Fatalf("Field %q in %s lines %d-%d: got %d nodes, want 1", name, fileSuffix, from, to, len(found))
	}
	return found[0]
}

func checkAnonymousStructFields(t *testing.T, g *fidelityGraph) {
	// Under a variable and under a defined type the fields are named after it.
	for _, qname := range []string{"anon.Config.Name", "anon.Config.Port", "anon.Rows.Key"} {
		g.one(t, types.NodeField, qname)
	}
	// A struct type written inside a function body belongs to the function.
	table := g.one(t, types.NodeFunction, "anon.Table")
	for _, name := range []string{"in", "want"} {
		g.fieldIn(t, name, fileAnon, table.StartLine, table.EndLine)
	}
}

func checkAnonymousStructFieldUses(t *testing.T, g *fidelityGraph) {
	table := g.one(t, types.NodeFunction, "anon.Table")
	port := g.one(t, types.NodeField, "anon.Config.Port")
	name := g.one(t, types.NodeField, "anon.Config.Name")
	in := g.fieldIn(t, "in", fileAnon, table.StartLine, table.EndLine)
	for _, want := range []struct {
		et  types.EdgeType
		dst types.Node
	}{{types.EdgeWritesField, port}, {types.EdgeReadsField, name}, {types.EdgeReadsField, in}} {
		if !g.has(want.et, table.ID, want.dst.ID) {
			t.Errorf("no %s edge from %s to the field %s (line %d)", want.et, table.QualifiedName, want.dst.Name, want.dst.StartLine)
		}
	}
}

func checkLocalNames(t *testing.T, g *fidelityGraph) {
	table := g.one(t, types.NodeFunction, "anon.Table")
	for _, qname := range []string{"anon.Table.cases", "anon.Table.sum", "anon.Table.tc"} {
		n := g.one(t, types.NodeLocalVariable, qname)
		if !g.has(types.EdgeDefines, table.ID, n.ID) {
			t.Errorf("%s is not defined by %s", qname, table.QualifiedName)
		}
	}
	// The blank identifier names nothing.
	for _, n := range g.nodes {
		if n.Type == types.NodeLocalVariable && n.Name == "_" {
			t.Errorf("LocalVariable node for the blank identifier at %s:%d", n.FilePath, n.StartLine)
		}
	}
}

func checkLocalNameUses(t *testing.T, g *fidelityGraph) {
	table := g.one(t, types.NodeFunction, "anon.Table")
	sum := g.one(t, types.NodeLocalVariable, "anon.Table.sum")
	kinds := map[string]bool{}
	for _, e := range g.edges {
		if e.Type == types.EdgeReferences && e.Src == table.ID && e.Dst == sum.ID {
			kinds[e.DispatchKind] = true
		}
	}
	// sum += ... writes it, Config.Port = sum reads it.
	for _, kind := range []string{"read", "write"} {
		if !kinds[kind] {
			t.Errorf("no references edge of kind %q from %s to %s", kind, table.QualifiedName, sum.QualifiedName)
		}
	}
}

func checkTypeReferences(t *testing.T, g *fidelityGraph) {
	item := g.one(t, types.NodeStruct, "refs.Item")
	for _, src := range []struct {
		nt    types.NodeType
		qname string
		why   string
	}{
		{types.NodeInterface, "refs.Store", "its method signatures name Item"},
		{types.NodeTypeAlias, "refs.List", "it is a slice of Item"},
		{types.NodeMethod, "refs.Item.Double", "Item is its receiver"},
		{types.NodeFunction, "refs.Convert", "its body asserts to Item"},
	} {
		n := g.one(t, src.nt, src.qname)
		if !g.has(types.EdgeUsesType, n.ID, item.ID) {
			t.Errorf("no uses_type edge from %s to %s: %s", src.qname, item.QualifiedName, src.why)
		}
	}
}

// TestFidelityCountsWithTestVariant loads the fixture as a build does, with
// test variants. A package with an in-package test is then loaded twice, and a
// pass that walks both copies counts every statement link twice.
func TestFidelityCountsWithTestVariant(t *testing.T) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps |
			packages.NeedImports | packages.NeedModule | packages.NeedCompiledGoFiles,
		Dir:   fidelityRoot,
		Tests: true,
	}, "./twice")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	p := gop.New(fidelityRoot)
	p.SetPackages(pkgs)
	var results []*parse.ParseResult
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		for _, path := range pkg.GoFiles {
			if seen[path] {
				continue
			}
			seen[path] = true
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			r, err := p.ParseFile(path, src)
			if err != nil {
				t.Fatalf("ParseFile(%s): %v", path, err)
			}
			results = append(results, r)
		}
	}
	rg, err := p.Resolve(results)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	edges, _ := gop.EmitStatementEdges(p.Pkgs(), rg.Nodes)
	g := newFidelityGraph(rg.Nodes, edges)
	checked := 0
	for _, e := range edges {
		src := g.byID[e.Src]
		if !strings.HasSuffix(filepath.ToSlash(src.FilePath), fileTwice) {
			continue
		}
		checked++
		if e.Count != 1 {
			t.Errorf("%s edge from %s at line %d has count %d, want 1: the source holds it once", e.Type, src.QualifiedName, e.Line, e.Count)
		}
	}
	if checked == 0 {
		t.Fatalf("no statement edge from %s was checked", fileTwice)
	}
}
