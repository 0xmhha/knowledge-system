package mcphandlers

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// The callers of fanin.hub in cardStore: direct production callers, direct
// test callers, and one caller through the interface method fanin.Hubber.hub
// that dispatches to hub.
const (
	cardProdCallers = 25
	cardTestCallers = 5
	cardFile        = "fanin/fanin.go"
	cardTestFile    = "fanin/fanin_test.go"
)

func cardStore(t *testing.T) persist.Store {
	t.Helper()
	s, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	blobs := map[string][]byte{}
	node := func(id string, nt types.NodeType, qname, file string, line int, src string) types.Node {
		blobs[id] = []byte(src)
		return types.Node{ID: id, Type: nt, Name: qname[strings.LastIndex(qname, ".")+1:], QualifiedName: qname,
			CanonicalID: "ex/" + qname, FilePath: file, StartLine: line, EndLine: line, EndByte: len(src),
			Language: "go", Confidence: types.ConfExtracted}
	}
	call := func(src, dst string, et types.EdgeType, line int, kind string) types.Edge {
		return types.Edge{Src: src, Dst: dst, Type: et, Line: line, DispatchKind: kind, Count: 1, Confidence: types.ConfExtracted}
	}
	nodes := []types.Node{
		node("hb00000000000001", types.NodeFunction, "fanin.hub", cardFile, 1, "func hub() {}"),
		node("hb00000000000002", types.NodeInterface, "fanin.Hubber", cardFile, 2, "type Hubber interface{ hub() }"),
		node("hb00000000000003", types.NodeMethod, "fanin.Hubber.hub", cardFile, 2, "hub()"),
		node("hb00000000000004", types.NodeFunction, "fanin.viaIface", cardFile, 3, "func viaIface(h Hubber) { h.hub() }"),
	}
	edges := []types.Edge{
		call("hb00000000000003", "hb00000000000001", types.EdgeDispatchesTo, 0, "interface_method"),
		call("hb00000000000004", "hb00000000000003", types.EdgeInvokes, 3, "interface_method"),
	}
	for i := 0; i < cardProdCallers+cardTestCallers; i++ {
		file := cardFile
		if i >= cardProdCallers {
			file = cardTestFile
		}
		id := fmt.Sprintf("cc%014d", i)
		name := fmt.Sprintf("fanin.caller%02d", i)
		nodes = append(nodes, node(id, types.NodeFunction, name, file, 10+i, "func "+name[6:]+"() { hub() }"))
		edges = append(edges, call(id, "hb00000000000001", types.EdgeCalls, 10+i, ""))
	}
	for i, f := range []string{cardFile, cardTestFile} {
		nodes = append(nodes, types.Node{ID: fmt.Sprintf("fi0000000000000%d", i), Type: types.NodeFile, Name: f, QualifiedName: "file:" + f,
			FilePath: f, StartLine: 1, EndLine: 1, EndByte: 1, Language: "go", Confidence: types.ConfExtracted})
	}
	for _, step := range []func() error{
		func() error { return s.InsertNodes(nodes) }, func() error { return s.InsertBlobs(blobs) },
		func() error { return s.InsertEdges(edges) }, s.RebuildFTS,
		func() error { return s.RebuildReviewIndex([]string{cardTestFile}) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// cardCallerRe reads the callers line of a card: the names shown and how many
// more there are.
var cardCallerRe = regexp.MustCompile(`(?m)^  callers: (.*?)(?: … (\d+) more; (find_callers .*))?$`)

// TestCard_CutCallersPointToExactCall: a card whose callers are cut names a
// find_callers call that returns exactly the callers the card counts (direct
// calls, test code left out). It named find_callers at depth 1, which on
// go-stablenet returned more in 36 of 40 cut cards: the test callers, and the
// callers through an interface method the function implements.
func TestCard_CutCallersPointToExactCall(t *testing.T) {
	s := cardStore(t)
	srv := newInvServer(t, s)
	callTextOn(t, srv, "investigate", map[string]any{"words": "hub"})
	card := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:fanin.hub"})
	m := cardCallerRe.FindStringSubmatch(card)
	if m == nil || m[3] == "" {
		t.Fatalf("no cut callers line in:\n%s", card)
	}
	more, err := strconv.Atoi(m[2])
	if err != nil {
		t.Fatal(err)
	}
	shown := len(strings.Split(m[1], ", "))
	if shown+more != cardProdCallers {
		t.Errorf("card counts %d callers, want %d", shown+more, cardProdCallers)
	}
	want := `find_callers qname="ex/fanin.hub" depth=1 direct=true tests=exclude returns exactly them`
	if m[3] != want {
		t.Fatalf("pointer %q, want %q", m[3], want)
	}
	out := callTool(t, s, "find_callers", map[string]any{"qname": "ex/fanin.hub", "depth": 1, "direct": true, "tests": "exclude"})
	var got []string
	for _, n := range asSlice(out["nodes"]) {
		if q := n.(map[string]any)["qname"].(string); q != "fanin.hub" {
			got = append(got, q)
		}
	}
	sort.Strings(got)
	if len(got) != cardProdCallers || got[0] != "fanin.caller00" || got[len(got)-1] != fmt.Sprintf("fanin.caller%02d", cardProdCallers-1) {
		t.Errorf("find_callers direct, tests excluded = %v, want the %d production callers", got, cardProdCallers)
	}
	all := callTool(t, s, "find_callers", map[string]any{"qname": "ex/fanin.hub", "depth": 1})
	if n := len(asSlice(all["nodes"])) - 1; n != cardProdCallers+cardTestCallers+2 {
		t.Errorf("find_callers depth=1 = %d callers, want the direct, test and interface ones (%d)", n, cardProdCallers+cardTestCallers+2)
	}
}

// TestCard_EveryNameForm: card: resolves a name as every tool does. A
// canonical id was refused ("no card holds it", 51 of 51).
func TestCard_EveryNameForm(t *testing.T) {
	s := cardStore(t)
	srv := newInvServer(t, s)
	callTextOn(t, srv, "investigate", map[string]any{"words": "hub"})
	for _, name := range []string{"ex/fanin.hub", "fanin.hub", "fanin.hub@" + cardFile + ":1", "hb00000000000001"} {
		card := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:" + name})
		if !strings.Contains(card, "function fanin.hub  "+cardFile+":1-1") {
			t.Errorf("card:%s:\n%s", name, card)
		}
	}
	if got := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:hub"}); !strings.Contains(got, `"hub" names several declarations; open one: fanin.Hubber.hub, fanin.hub`) {
		t.Errorf("card:hub: %s", got)
	}
	if got := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:viaiface"}); !strings.Contains(got, `unknown declaration "viaiface"; other case: fanin.viaIface`) {
		t.Errorf("card:viaiface: %s", got)
	}
}

// TestFindCallers_ValueWithFilters: find_callers on a value answers with its
// readers and writers whatever the arguments, as the plain call does;
// tests=exclude drops the test readers and writers, and direct changes
// nothing for a value (they are its direct uses). With either argument the
// value got an empty call graph instead.
func TestFindCallers_ValueWithFilters(t *testing.T) {
	s := toolStore(t)
	if err := s.RebuildReviewIndex([]string{fxLoggerFile}); err != nil {
		t.Fatal(err)
	}
	users := func(args map[string]any) []string {
		out := callTool(t, s, "find_callers", args)
		var got []string
		for _, u := range asSlice(out["users"]) {
			m := u.(map[string]any)
			got = append(got, m["qname"].(string)+" "+m["access"].(string))
		}
		sort.Strings(got)
		return got
	}
	plain := users(map[string]any{"qname": "state.StateDB.logs"})
	if len(plain) == 0 {
		t.Fatal("no users of state.StateDB.logs in the fixture")
	}
	if got := users(map[string]any{"qname": "state.StateDB.logs", "direct": true}); strings.Join(got, ",") != strings.Join(plain, ",") {
		t.Errorf("direct=true: %v, want the plain users %v", got, plain)
	}
	if got := users(map[string]any{"qname": "state.StateDB.logs", "tests": "exclude"}); strings.Join(got, ",") != strings.Join(plain, ",") {
		t.Errorf("tests=exclude: %v, want %v (no test user)", got, plain)
	}
	// A value with a test user: Gov.quorum is written by Gov.vote; add a test
	// reader in the logger test file.
	if err := s.InsertEdges([]types.Edge{{Src: "lg00000000000002", Dst: "st00000000000002", Type: types.EdgeReadsField,
		FilePath: fxLoggerFile, Line: 11, Count: 1, Confidence: types.ConfExtracted}}); err != nil {
		t.Fatal(err)
	}
	all := users(map[string]any{"qname": "state.StateDB.logs"})
	noTests := users(map[string]any{"qname": "state.StateDB.logs", "tests": "exclude", "direct": true})
	if len(all) != len(plain)+1 || strings.Join(noTests, ",") != strings.Join(plain, ",") {
		t.Errorf("with a test reader: all %v, tests excluded %v, want %v", all, noTests, plain)
	}
}

// TestMutexVariableIsAValue: the name of a mutex variable names the
// variable, so find_callers answers with its readers and writers, as for any
// variable and as for a mutex field, and the #mutex form names the Mutex
// node. downloader.testBlockchainsLock was answered with its two nodes.
func TestMutexVariableIsAValue(t *testing.T) {
	s, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	const file = "dl/dl.go"
	nodes := []types.Node{
		{ID: "mv00000000000001", Type: types.NodeVariable, Name: "chainsLock", QualifiedName: "dl.chainsLock", CanonicalID: "ex/dl.chainsLock", FilePath: file, StartLine: 5, EndLine: 5},
		{ID: "mv00000000000002", Type: types.NodeMutex, Name: "chainsLock", QualifiedName: "dl.chainsLock", FilePath: file, StartLine: 5, EndLine: 5},
		{ID: "mv00000000000003", Type: types.NodeFunction, Name: "newChain", QualifiedName: "dl.newChain", CanonicalID: "ex/dl.newChain", FilePath: file, StartLine: 10, EndLine: 14},
	}
	for i := range nodes {
		nodes[i].Language, nodes[i].Confidence = "go", types.ConfExtracted
	}
	edges := []types.Edge{
		{Src: "mv00000000000003", Dst: "mv00000000000001", Type: types.EdgeReferences, DispatchKind: "read", Line: 11, Count: 1, Confidence: types.ConfExtracted},
		{Src: "mv00000000000003", Dst: "mv00000000000002", Type: types.EdgeAcquiresLock, Line: 11, Count: 1, Confidence: types.ConfExtracted},
	}
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEdges(edges); err != nil {
		t.Fatal(err)
	}
	for _, args := range []map[string]any{
		{"qname": "dl.chainsLock"}, {"qname": "chainsLock"}, {"qname": "ex/dl.chainsLock"},
		{"qname": "dl.chainsLock", "direct": true}, {"qname": "dl.chainsLock", "tests": "exclude"},
	} {
		out := callTool(t, s, "find_callers", args)
		if got := rowNames(out, "users"); len(got) != 1 || got[0] != "dl.newChain" {
			t.Errorf("find_callers %v: users %v (nodes %v), want dl.newChain", args, got, rowNames(out, "nodes"))
		}
	}
	// find_symbol follows the same rule: the name is the variable, #mutex
	// its Mutex node (it returned both nodes, and nothing for #mutex).
	for name, want := range map[string]string{"dl.chainsLock": "Variable", "dl.chainsLock#mutex": "Mutex"} {
		out := callTool(t, s, "find_symbol", map[string]any{"name": name})
		ns := asSlice(out["nodes"])
		if len(ns) != 1 || ns[0].(map[string]any)["type"] != want {
			t.Errorf("find_symbol(%s) = %v, want one %s", name, out["nodes"], want)
		}
	}
	conc := callTool(t, s, "concurrency_impact", map[string]any{"seed_qname": "dl.chainsLock#mutex"})
	// The mutex is printed by the name that opens it.
	if conc["mutex"] != "dl.chainsLock#mutex" || conc["declared_by"] != "dl.chainsLock" {
		t.Errorf("concurrency_impact #mutex: %v", conc)
	}
	sub := callTool(t, s, "get_subgraph", map[string]any{"seed_qname": "dl.chainsLock#mutex"})
	if sub["not_found"] == true || sub["ambiguous"] == true {
		t.Errorf("get_subgraph #mutex: %v", sub)
	}
}

// TestCard_NameOfNoCard: a name that resolves to a declaration without a
// card (a package: "filters" is the package filters, and also the leaf of
// two fields and a local) lists the declarations of that name and what
// each has: a card, or the declaration that holds it. card:filters said
// "no card holds it".
func TestCard_NameOfNoCard(t *testing.T) {
	s := toolStore(t)
	pkg := types.Node{ID: "pk00000000000001", Type: types.NodePackage, Name: "abi", QualifiedName: "abi", FilePath: fxNamesFile,
		StartLine: 1, EndLine: 1, Language: "go", Confidence: types.ConfExtracted}
	field := types.Node{ID: "pk00000000000002", Type: types.NodeField, Name: "abi", QualifiedName: "abi.Config.abi", CanonicalID: "ex/abi.Config.abi",
		FilePath: fxNamesFile, StartLine: 40, EndLine: 40, Language: "go", Confidence: types.ConfExtracted}
	local := types.Node{ID: "pk00000000000003", Type: types.NodeLocalVariable, Name: "abi", QualifiedName: "abi.TestPack.abi",
		FilePath: fxNamesFile, StartLine: 22, EndLine: 22, Language: "go", Confidence: types.ConfExtracted}
	again := local
	again.ID, again.StartLine, again.EndLine = "pk00000000000004", 26, 26
	if err := s.InsertNodes([]types.Node{pkg, field, local, again}); err != nil {
		t.Fatal(err)
	}
	srv := newInvServer(t, s)
	callTextOn(t, srv, "investigate", map[string]any{"words": "Pack"})
	got := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:abi"})
	want := []string{`"abi" names package abi, which has no card; declarations with that name: ` +
		"abi.Config.abi (field, card), abi.TestPack.abi@" + fxNamesFile + ":22 (localvariable, local of abi.TestPack), abi.TestPack.abi@" + fxNamesFile + ":26 (localvariable, local of abi.TestPack)"}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("card:abi lacks %q:\n%s", w, got)
		}
	}
}
