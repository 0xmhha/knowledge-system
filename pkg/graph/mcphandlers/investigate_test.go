package mcphandlers

import (
	"context"
	"regexp"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// callText calls a tool on a fresh server over store and returns its text.
// One server is one session, so session memory starts empty.
func newInvServer(t *testing.T, store persist.StoreReader) *server.MCPServer {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, store)
	return s
}

func callTextOn(t *testing.T, s *server.MCPServer, name string, args map[string]any) string {
	t.Helper()
	tool := s.GetTool(name)
	if tool == nil || tool.Handler == nil {
		t.Fatalf("%s not registered", name)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

var invIDRe = regexp.MustCompile(`investigation (i\d+)`)

// TestInvestigate_FunctionCardShowsEveryUseLine: the card of a function the
// word names lists every line that calls or uses something, not only the
// lines that contain the word. bump never mentions "bump" on lines 35 and
// 36, where it writes Inner.Depth.
func TestInvestigate_FunctionCardShowsEveryUseLine(t *testing.T) {
	s := newInvServer(t, newFixtureStore(t))
	out := callTextOn(t, s, "investigate", map[string]any{"words": "bump"})
	for _, want := range []string{"coll1.bump", "35", "36", "writes coll1.Inner.Depth"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// A use on the declaration's first line (a one-line function) is listed too.
	one := callTextOn(t, s, "investigate", map[string]any{"words": "incCurrent"})
	if !strings.Contains(one, "writes coll1.current") || !strings.Contains(one, "reads coll1.current") {
		t.Errorf("incCurrent reads and writes current on its only line:\n%s", one)
	}
}

// TestInvestigate_ValueCard: a word naming a value gets who writes it and
// who reads it, each with the lines and their source text.
func TestInvestigate_ValueCard(t *testing.T) {
	s := newInvServer(t, newFixtureStore(t))
	out := callTextOn(t, s, "investigate", map[string]any{"words": "Depth"})
	for _, want := range []string{"coll1.Inner.Depth", "writers", "coll1.bump", "o.Depth++", "readers", "coll1.depth", "return o.Depth"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if w, r := strings.Index(out, "writers"), strings.Index(out, "readers"); w < 0 || r < 0 || w > r {
		t.Errorf("writers must come before readers:\n%s", out)
	}
}

// TestInvestigate_JoinAcrossWords: a declaration that two words reach is
// listed as a join, the ones holding both words first.
func TestInvestigate_JoinAcrossWords(t *testing.T) {
	s := newInvServer(t, newFixtureStore(t))
	out := callTextOn(t, s, "investigate", map[string]any{"words": "current|Limit"})
	j := strings.Index(out, "joins")
	if j < 0 {
		t.Fatalf("no joins section:\n%s", out)
	}
	if !strings.Contains(out[j:], "coll1.readCurrent") {
		t.Errorf("readCurrent uses both current and Limit; not in joins:\n%s", out[j:])
	}
}

// TestInvestigate_BudgetCutAndOpen: what does not fit is named with a handle,
// and investigate_open returns it.
func TestInvestigate_BudgetCutAndOpen(t *testing.T) {
	s := newInvServer(t, newFixtureStore(t))
	out := callTextOn(t, s, "investigate", map[string]any{"words": "current|Depth|Limit|Box", "budget": 1300})
	if len(out) > 1300 {
		t.Errorf("response %d chars over budget 1300", len(out))
	}
	m := regexp.MustCompile(`cards \(open="card:<qname>"\): ([\w.]+)`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no cut card in:\n%s", out)
	}
	m[1] = "card:" + m[1]
	id := invIDRe.FindStringSubmatch(out)
	if id == nil {
		t.Fatalf("no investigation id in:\n%s", out)
	}
	opened := callTextOn(t, s, "investigate_open", map[string]any{"id": id[1], "open": m[1]})
	name := strings.TrimPrefix(m[1], "card:")
	if !strings.Contains(opened, name) || strings.Contains(opened, "unknown") {
		t.Errorf("open %s returned:\n%s", m[1], opened)
	}
}

// TestInvestigate_Deterministic: the same words give the same response,
// apart from the investigation id and the session marks.
func TestInvestigate_Deterministic(t *testing.T) {
	st := newFixtureStore(t)
	a := callTextOn(t, newInvServer(t, st), "investigate", map[string]any{"words": "current|Depth|Box"})
	b := callTextOn(t, newInvServer(t, st), "investigate", map[string]any{"words": "current|Depth|Box"})
	if a != b {
		t.Errorf("two fresh sessions differ:\n%s\n---\n%s", a, b)
	}
}

// TestInvestigate_SessionMemory: a card shown in an earlier response is not
// repeated in full, and the later response says where it was.
func TestInvestigate_SessionMemory(t *testing.T) {
	s := newInvServer(t, newFixtureStore(t))
	first := callTextOn(t, s, "investigate", map[string]any{"words": "Depth"})
	if !strings.Contains(first, "o.Depth++") {
		t.Fatalf("first response has no Depth card:\n%s", first)
	}
	second := callTextOn(t, s, "investigate", map[string]any{"words": "Depth|current"})
	if strings.Contains(second, "o.Depth++") {
		t.Errorf("the Depth card is repeated in full:\n%s", second)
	}
	if !strings.Contains(second, "shown in i1") {
		t.Errorf("the second response does not point to the earlier card:\n%s", second)
	}
}

// TestSplitTerms: regular-expression input from an agent becomes the words
// it means: a letter class gives each spelling, a wildcard between two parts
// keeps them as one pattern, and escapes are not words.
func TestSplitTerms(t *testing.T) {
	cases := map[string]string{
		"a|bc, def ghi":                     "def ghi",
		`[Ss]ignature|VerifySig`:            "Signature signature VerifySig",
		`Override\w+Time|ForkBlock`:         `Override*Time ForkBlock`,
		`func \(.*\) (StartRound|setRound)`: "StartRound setRound",
		`overrides\.Override`:               "overrides Override",
		`Verify.*Sig`:                       "Verify*Sig",
	}
	for in, want := range cases {
		var got []string
		for _, w := range splitTerms(in) {
			got = append(got, w.label)
		}
		if strings.Join(got, " ") != want {
			t.Errorf("splitTerms(%q) = %v, want %s", in, got, want)
		}
	}
}

// TestInvestigate_PatternWord: a pattern word names the declarations whose
// name it matches whole.
func TestInvestigate_PatternWord(t *testing.T) {
	s := newInvServer(t, newFixtureStore(t))
	out := callTextOn(t, s, "investigate", map[string]any{"words": `set\w*Current`})
	if !strings.Contains(out, "function coll1.setCurrent") {
		t.Errorf("set*Current must name coll1.setCurrent:\n%s", out)
	}
}

// TestInvestigate_CompactCardOpensFull: the first response shows a value's
// readers up to a cap; open card returns all of them.
func TestInvestigate_CompactCardOpensFull(t *testing.T) {
	x := &invIndex{}
	if got := x.capRows([]string{"a", "b", "c"}, 2, "card:q"); len(got) != 3 || !strings.Contains(got[2], `open="card:q"`) {
		t.Errorf("capRows = %v", got)
	}
}

// TestCutCard: a card longer than the cap ends at a line boundary with the
// handle of the full card.
func TestCutCard(t *testing.T) {
	card := "head\n" + strings.Repeat("  12345  line\n", 50)
	got := cutCard(card, 200, "card:q")
	if len(got) > 200+80 || !strings.Contains(got, `open="card:q"`) || !strings.HasPrefix(got, "head\n") {
		t.Errorf("cutCard = %q", got)
	}
	if short := "head\n  1 x\n"; cutCard(short, 200, "card:q") != short {
		t.Errorf("a short card must stay whole")
	}
}

// TestInvestigate_FullCardsWhenRoom: when everything fits, the cards are
// full; a value's readers are all listed.
func TestInvestigate_FullCardsWhenRoom(t *testing.T) {
	s := newInvServer(t, newFixtureStore(t))
	out := callTextOn(t, s, "investigate", map[string]any{"words": "current", "budget": 100000})
	if strings.Contains(out, "more lines, in:") || strings.Contains(out, "reads on ") {
		t.Errorf("compact card with room to spare:\n%s", out)
	}
}

// TestInvestigate_NameHoldsWord: a declaration whose name holds the word
// gets a card even when its body has the word only inside a longer name.
// setCurrent's name holds "Current"; its body has only "current".
func TestInvestigate_NameHoldsWord(t *testing.T) {
	s := newInvServer(t, newFixtureStore(t))
	out := callTextOn(t, s, "investigate", map[string]any{"words": "Current"})
	if !strings.Contains(out, "function coll1.setCurrent  ") {
		t.Errorf("setCurrent's name holds Current; no card:\n%s", out)
	}
}

// TestFooterCompact: the footer lists cut cards by name on one line.
func TestFooterCompact(t *testing.T) {
	cut := []invBlock{{handle: "card:a.B", text: "x"}, {handle: "card:c.D", text: "y"}, {handle: "joins", head: "h\n", entries: []string{"e\n"}}}
	f := footer(&investigation{id: "i1"}, cut, 1000)
	if !strings.Contains(f, "a.B, c.D") || !strings.Contains(f, `open="joins"`) || strings.Count(f, "\n") > 3 {
		t.Errorf("footer = %q", f)
	}
}

// TestInvestigate_TestsExcludedFromCallers: with tests excluded, as the
// header says, a card lists no caller from test code. Test code is what the
// build recorded as such (nodes.is_test: the build scope's file role, the
// path rule without a scope). The card of core.TxIndexer.run listed
// core.TestTxIndexer under a header that said "tests excluded".
func TestInvestigate_TestsExcludedFromCallers(t *testing.T) {
	const caller = "logger.TestPrepare"
	card := func(t *testing.T, s persist.StoreReader, tests string) string {
		t.Helper()
		srv := newInvServer(t, s)
		head := callTextOn(t, srv, "investigate", map[string]any{"words": "Prepare", "tests": tests})
		if want := "tests " + tests + "d"; !strings.Contains(head, want) {
			t.Fatalf("header does not say %q:\n%s", want, head)
		}
		return callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:state.StateDB.Prepare"})
	}
	recorded := toolStore(t)
	if err := recorded.RebuildReviewIndex([]string{fxLoggerFile}); err != nil {
		t.Fatal(err)
	}
	if got := card(t, recorded, "exclude"); strings.Contains(got, caller) {
		t.Errorf("tests excluded, yet the card lists the test caller:\n%s", got)
	}
	if got := card(t, recorded, "include"); !strings.Contains(got, caller) {
		t.Errorf("tests included, yet the card lacks the test caller:\n%s", got)
	}
	// The build's record decides, not the file name: a build that
	// classified no file as test code keeps the caller.
	none := toolStore(t)
	if err := none.RebuildReviewIndex(nil); err != nil {
		t.Fatal(err)
	}
	if got := card(t, none, "exclude"); !strings.Contains(got, caller) {
		t.Errorf("no file is test code in this build, yet the card drops %s:\n%s", caller, got)
	}
}

// TestInvestigate_SelfCallIsACaller: a function that calls itself is one of
// its own callers, in the card as in find_callers. The card of
// core.ChainIndexer.newHead left out its call at core/chain_indexer.go:272.
// Only the invokes edge a function has to itself, which stands for a call
// through a function value, is not a call to itself.
func TestInvestigate_SelfCallIsACaller(t *testing.T) {
	s := toolStore(t)
	srv := newInvServer(t, s)
	callTextOn(t, srv, "investigate", map[string]any{"words": "wrap"})
	card := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:state.wrap"})
	if !strings.Contains(card, "state.wrap:20") {
		t.Errorf("the card of a recursive function lacks its own call:\n%s", card)
	}
	callers := callTool(t, s, "find_callers", map[string]any{"qname": "state.wrap", "depth": 1})
	self := false
	for _, e := range asSlice(callers["edges"]) {
		m := e.(map[string]any)
		self = self || m["src"] == "st00000000000005" && m["dst"] == "st00000000000005"
	}
	if !self {
		t.Errorf("find_callers lacks the self-call: %v", callers["edges"])
	}
}

// TestInvestigate_StatementLineIsStable: the line a guard names is read from
// the function that holds the statement, whatever order the index lists the
// functions in. A promoted copy has the lines of the method it copies and no
// source; taken from map order, it made a guard read "[then of L167 ; ...]"
// on some server starts and "[then of L167 if !conf.SkipCode {; ...]" on
// others (5 of 30 investigate queries differed across fresh servers).
func TestInvestigate_StatementLineIsStable(t *testing.T) {
	s := toolStore(t)
	// A statement of Prepare that writes a field: the index reads guards
	// only from a graph that has them.
	stmt := types.Node{ID: "if00000000000001", Type: types.NodeIfStmt, Name: "if", QualifiedName: "state.StateDB.Prepare#IfStmt@40",
		FilePath: fxStateFile, StartLine: 11, EndLine: 11, Language: "go", Confidence: types.ConfExtracted}
	if err := s.InsertNodes([]types.Node{stmt}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEdges([]types.Edge{{Src: stmt.ID, Dst: "st00000000000002", Type: types.EdgeBodyWrites, FilePath: fxStateFile,
		Line: 11, Count: 1, Confidence: types.ConfExtracted}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		x, err := buildInvIndex(safeReader(s))
		if err != nil {
			t.Fatal(err)
		}
		if got := x.stmtLine(stmt); got != "s.logs = rules" {
			t.Fatalf("build %d: statement line = %q, want the line of state.StateDB.Prepare", i, got)
		}
	}
}
