package persist_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// newNameStore holds the kinds of node one bare name can match: declarations,
// a name of another case, function-local names, the copy of a promoted method
// and an External stub.
func newNameStore(t *testing.T) persist.Store {
	t.Helper()
	s, err := persist.Open(filepath.Join(t.TempDir(), "names.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	node := func(id string, kind types.NodeType, qname, canonical string, start, end int) types.Node {
		name := qname
		for i := len(qname) - 1; i >= 0; i-- {
			if qname[i] == '.' {
				name = qname[i+1:]
				break
			}
		}
		return types.Node{ID: id, Type: kind, Name: name, QualifiedName: qname, CanonicalID: canonical,
			FilePath: "p/f.go", StartLine: start, EndLine: end, Language: "go", Confidence: types.ConfExtracted}
	}
	promoted := node("n000000000000008", types.NodeMethod, "logger.dummy.Prepare", "", 40, 42)
	promoted.DocComment = "promoted from state.StateDB.Prepare"
	external := node("n000000000000012", types.NodeExternal, "errors.New", "", 1, 1)
	external.FilePath = types.ExternalFilePath
	nodes := []types.Node{
		node("n000000000000001", types.NodeMethod, "abi.ABI.Pack", "ex/abi.(ABI).Pack", 1, 2),
		node("n000000000000002", types.NodeMethod, "abi.Arguments.pack", "ex/abi.(Arguments).pack", 3, 4),
		node("n000000000000003", types.NodeField, "abi.args.resultMap", "ex/abi.args.resultMap", 5, 5),
		node("n000000000000017", types.NodeFunction, "abi.TestParse", "ex/abi.TestParse", 10, 14),
		node("n000000000000004", types.NodeLocalVariable, "abi.TestParse.resultMap", "", 11, 11),
		// a.F declares a variable, a type with a field, and a := name.
		node("n000000000000018", types.NodeFunction, "a.F", "ex/a.F", 20, 30),
		node("n000000000000005", types.NodeLocalVariable, "a.F.tmp", "", 21, 21),
		node("n000000000000013", types.NodeVariable, "a.F.count", "", 22, 22),
		node("n000000000000015", types.NodeStruct, "a.F.row", "", 23, 25),
		node("n000000000000020", types.NodeField, "a.F.row.Size", "", 24, 24),
		node("n000000000000019", types.NodeFunction, "a.G", "ex/a.G", 31, 35),
		node("n000000000000006", types.NodeLocalVariable, "a.G.tmp", "", 32, 32),
		node("n000000000000007", types.NodeMethod, "state.StateDB.Prepare", "ex/state.(*StateDB).Prepare", 40, 42),
		promoted,
		node("n000000000000009", types.NodeMethod, "coll1.Set.Size", "ex/coll1.(*Set).Size", 50, 50),
		node("n000000000000010", types.NodeMethod, "coll2.Other.Size", "ex/coll2.(*Other).Size", 51, 51),
		node("n000000000000011", types.NodeFunction, "a.New", "ex/a.New", 52, 52),
		external,
		node("n000000000000014", types.NodeVariable, "b.count", "ex/b.count", 60, 60),
		node("n000000000000016", types.NodeStruct, "b.row", "ex/b.row", 61, 62),
		node("n000000000000021", types.NodeVariable, "b._", "", 63, 63), // var _ = ... at package level
		// A package-level variable whose initialiser calls a function
		// literal that declares a variable, and one of an anonymous struct
		// type, whose field code outside reaches.
		node("n000000000000022", types.NodeVariable, "enode.parseNodeTests", "ex/enode.parseNodeTests", 70, 80),
		node("n000000000000023", types.NodeVariable, "enode.parseNodeTests.r", "", 75, 75),
		node("n000000000000024", types.NodeVariable, "enode.cfg", "ex/enode.cfg", 82, 84),
		node("n000000000000025", types.NodeField, "enode.cfg.struct@900.Port", "", 83, 83),
		// One function declares the mutex errMu twice, in two blocks: each
		// declaration is a LocalVariable and a Mutex node at its line.
		node("n000000000000026", types.NodeFunction, "catalyst.TestBlock", "ex/catalyst.TestBlock", 90, 110),
		node("n000000000000027", types.NodeLocalVariable, "catalyst.TestBlock.errMu", "", 92, 92),
		node("n000000000000028", types.NodeMutex, "catalyst.TestBlock.errMu", "", 92, 92),
		node("n000000000000029", types.NodeLocalVariable, "catalyst.TestBlock.errMu", "", 101, 101),
		node("n000000000000030", types.NodeMutex, "catalyst.TestBlock.errMu", "", 101, 101),
		// A package-level mutex: its Variable and its Mutex node share the
		// qualified name and the place.
		node("n000000000000031", types.NodeVariable, "downloader.chainsLock", "ex/downloader.chainsLock", 120, 120),
		node("n000000000000032", types.NodeMutex, "downloader.chainsLock", "", 120, 120),
	}
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatalf("InsertNodes: %v", err)
	}
	return s
}

// TestResolveName: a bare name resolves to the declaration of exactly that
// name. Names of another case, function-local names, promoted copies and
// External stubs do not make it ambiguous; they answer only when no
// declaration carries the name. On go-stablenet 5 of 30 names that are unique
// in the source came back ambiguous for these reasons.
func TestResolveName(t *testing.T) {
	s := newNameStore(t)
	for _, tc := range []struct {
		name       string
		wantQName  string
		wantIDs    []string
		candidates []string
		otherCase  []string
		byID       bool
		promotedAs string
	}{
		{name: "abi.ABI.Pack", wantQName: "abi.ABI.Pack", wantIDs: []string{"n000000000000001"}},
		{name: "Pack", wantQName: "abi.ABI.Pack", wantIDs: []string{"n000000000000001"}},
		{name: "pack", wantQName: "abi.Arguments.pack", wantIDs: []string{"n000000000000002"}},
		{name: "PACK", otherCase: []string{"abi.ABI.Pack", "abi.Arguments.pack"}},
		{name: "resultMap", wantQName: "abi.args.resultMap", wantIDs: []string{"n000000000000003"}},
		{name: "tmp", candidates: []string{"a.F.tmp", "a.G.tmp"}},
		{name: "Prepare", wantQName: "state.StateDB.Prepare", wantIDs: []string{"n000000000000007"}},
		// The copy of a promoted method is answered with the method it copies.
		{name: "dummy.Prepare", wantQName: "state.StateDB.Prepare", wantIDs: []string{"n000000000000007"}, byID: true, promotedAs: "logger.dummy.Prepare"},
		{name: "logger.dummy.Prepare", wantQName: "state.StateDB.Prepare", wantIDs: []string{"n000000000000007"}, byID: true, promotedAs: "logger.dummy.Prepare"},
		{name: "Size", candidates: []string{"coll1.Set.Size", "coll2.Other.Size"}},
		{name: "New", wantQName: "a.New", wantIDs: []string{"n000000000000011"}},
		{name: "errors.New", wantQName: "errors.New", wantIDs: []string{"n000000000000012"}},
		{name: "count", wantQName: "b.count", wantIDs: []string{"n000000000000014"}},
		{name: "row", wantQName: "b.row", wantIDs: []string{"n000000000000016"}},
		{name: "ex/coll1.(*Set).Size", wantQName: "coll1.Set.Size", wantIDs: []string{"n000000000000009"}, byID: true},
		// A name declared at two places of one function is ambiguous, with
		// each place a candidate that resolves to its own declaration.
		{name: "catalyst.TestBlock.errMu", candidates: []string{"catalyst.TestBlock.errMu@p/f.go:92", "catalyst.TestBlock.errMu@p/f.go:101"}},
		{name: "errMu", candidates: []string{"catalyst.TestBlock.errMu@p/f.go:92", "catalyst.TestBlock.errMu@p/f.go:101"}},
		{name: "catalyst.TestBlock.errMu@p/f.go:92", wantQName: "catalyst.TestBlock.errMu", wantIDs: []string{"n000000000000027"}, byID: true},
		{name: "catalyst.TestBlock.errMu@p/f.go:93"},
		// The name of a mutex variable is the variable, a value; with
		// #mutex it is the mutex, as "pkg.T.mu#mutex" is for a field.
		{name: "downloader.chainsLock", wantQName: "downloader.chainsLock", wantIDs: []string{"n000000000000031"}, byID: true},
		{name: "chainsLock", wantQName: "downloader.chainsLock", wantIDs: []string{"n000000000000031"}, byID: true},
		{name: "downloader.chainsLock#mutex", wantQName: "downloader.chainsLock", wantIDs: []string{"n000000000000032"}, byID: true},
		{name: "catalyst.TestBlock.errMu@p/f.go:101", wantQName: "catalyst.TestBlock.errMu", wantIDs: []string{"n000000000000029"}, byID: true},
		// The mutex of a local declared twice: "#mutex" lists the two
		// mutexes by site names that keep the suffix, and each resolves to
		// its Mutex node alone.
		{name: "catalyst.TestBlock.errMu#mutex", candidates: []string{"catalyst.TestBlock.errMu#mutex@p/f.go:92", "catalyst.TestBlock.errMu#mutex@p/f.go:101"}},
		{name: "catalyst.TestBlock.errMu#mutex@p/f.go:101", wantQName: "catalyst.TestBlock.errMu", wantIDs: []string{"n000000000000030"}, byID: true},
		{name: "catalyst.TestBlock.errMu#mutex@p/f.go:93"},
		{name: "NoSuchName"},
		{name: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := persist.ResolveName(s, tc.name, "")
			if err != nil {
				t.Fatalf("ResolveName: %v", err)
			}
			var ids []string
			for _, n := range got.Nodes {
				ids = append(ids, n.ID)
			}
			if got.QName != tc.wantQName || !reflect.DeepEqual(ids, tc.wantIDs) {
				t.Errorf("resolved to %q %v, want %q %v", got.QName, ids, tc.wantQName, tc.wantIDs)
			}
			if !reflect.DeepEqual(got.Candidates, tc.candidates) {
				t.Errorf("candidates = %v, want %v", got.Candidates, tc.candidates)
			}
			if !reflect.DeepEqual(got.OtherCase, tc.otherCase) {
				t.Errorf("other case = %v, want %v", got.OtherCase, tc.otherCase)
			}
			if got.ByID != tc.byID || got.Found() != (tc.wantQName != "") || got.Ambiguous() != (tc.candidates != nil) {
				t.Errorf("byID %v found %v ambiguous %v", got.ByID, got.Found(), got.Ambiguous())
			}
			if got.PromotedAs != tc.promotedAs {
				t.Errorf("promotedAs = %q, want %q", got.PromotedAs, tc.promotedAs)
			}
		})
	}
}

// TestClassifyNames: one rule says what a node matched by name is, and one
// rule says that it is declared inside a function body: by position, so the
// member of a function-local type is local too (find_symbol returned such
// members in 38 of 472 bare-name queries on go-stablenet), while a blank
// package-level variable, which has no canonical id either, is not local; it
// has no name and is no answer to a name query.
func TestClassifyNames(t *testing.T) {
	s := newNameStore(t)
	for _, tc := range []struct {
		qname string
		want  persist.NameClass
	}{
		{"abi.ABI.Pack", persist.NameDeclared},
		{"b.count", persist.NameDeclared},
		{"b._", persist.NameOther}, // not inside a function, but no name either
		{"a.F", persist.NameDeclared},
		{"logger.dummy.Prepare", persist.NamePromoted},
		{"a.F.tmp", persist.NameLocal},
		{"a.F.count", persist.NameLocal},
		{"a.F.row", persist.NameLocal},
		{"a.F.row.Size", persist.NameLocal},
		{"b.row", persist.NameDeclared},
		{"errors.New", persist.NameExternal},
		// A variable of a function literal in a package-level initialiser is
		// part of the package-level variable, as a local is of its function.
		{"enode.parseNodeTests.r", persist.NameLocal},
		{"enode.parseNodeTests", persist.NameDeclared},
		{"enode.cfg.struct@900.Port", persist.NameDeclared},
	} {
		nodes, err := s.FindSymbol(tc.qname, true, persist.FindSymbolOptions{})
		if err != nil || len(nodes) != 1 {
			t.Fatalf("FindSymbol(%q) = %v, %v", tc.qname, nodes, err)
		}
		classes, err := persist.ClassifyNames(s, nodes)
		if err != nil {
			t.Fatalf("ClassifyNames(%s): %v", tc.qname, err)
		}
		if got := classes[nodes[0].ID]; got != tc.want {
			t.Errorf("ClassifyNames(%s) = %v, want %v", tc.qname, got, tc.want)
		}
	}
	if from, ok := persist.PromotedFrom(types.Node{Type: types.NodeMethod, DocComment: "promoted from state.StateDB.Prepare"}); !ok || from != "state.StateDB.Prepare" {
		t.Errorf("PromotedFrom = %q, %v", from, ok)
	}
}

// TestPromotedTo: the types that get a method through an embedded field are
// an attribute of the declared method.
func TestPromotedTo(t *testing.T) {
	s := newNameStore(t)
	methods, err := s.FindSymbol("Prepare", false, persist.FindSymbolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := persist.PromotedTo(s, methods)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"n000000000000007": {"logger.dummy"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PromotedTo = %v, want %v", got, want)
	}
}

// TestSeedNote: a name that resolves through a promoted copy carries the note
// every seed tool reports.
func TestSeedNote(t *testing.T) {
	s := newNameStore(t)
	for _, tc := range []struct {
		name string
		note bool
	}{
		{"logger.dummy.Prepare", true},
		{"state.StateDB.Prepare", false},
	} {
		res, err := persist.ResolveName(s, tc.name, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := res.SeedNote() != ""; got != tc.note || res.QName != "state.StateDB.Prepare" {
			t.Errorf("ResolveName(%s): note %q, qname %s", tc.name, res.SeedNote(), res.QName)
		}
	}
}

// TestNameOfRoundTrip: the name printed for a node (NameOf) resolves back to
// that node, for every node of the fixture: the Mutex node of a variable is
// printed "<name>#mutex", a declaration whose qualified name others carry
// by its site name. The copy of a promoted method is the one exception by
// design: its name answers with the method it copies.
func TestNameOfRoundTrip(t *testing.T) {
	s := newNameStore(t)
	all, err := s.AllNodes()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range all {
		name, err := persist.NameOf(s, n)
		if err != nil {
			t.Fatalf("NameOf(%s): %v", n.QualifiedName, err)
		}
		res, err := persist.ResolveName(s, name, "")
		if err != nil {
			t.Fatalf("ResolveName(%s): %v", name, err)
		}
		if _, isCopy := persist.PromotedFrom(n); isCopy {
			continue
		}
		found := false
		for _, m := range res.Nodes {
			found = found || m.ID == n.ID
		}
		if !found || len(res.Nodes) != 1 && n.Type != types.NodeExternal {
			t.Errorf("%s %s printed as %q resolves to %v", n.Type, n.QualifiedName, name, res.Nodes)
		}
	}
}
