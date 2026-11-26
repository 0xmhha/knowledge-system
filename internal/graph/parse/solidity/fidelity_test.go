package solidity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	sol "github.com/0xmhha/knowledge-system/internal/graph/parse/solidity"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// The fidelity fixture has a v1 and a v2 folder that both declare
// `contract Minter` at the same byte offset, a base chain Base <- Mid <-
// Minter, an interface, a library, and a mock that inherits an interface the
// fixture does not contain. Each test below compares the graph with what the
// source says.
const (
	fidelityDir      = "testdata/fidelity"
	fileBase         = "abstracts/Base.sol"
	fileMid          = "abstracts/Mid.sol"
	fileIToken       = "interfaces/IAsset.sol"
	fileSetLib       = "libraries/SetLib.sol"
	fileMinterV1     = "v1/Minter.sol"
	fileMinterV2     = "v2/Minter.sol"
	fileMock         = "test/MockAsset.sol"
	fileOrder        = "linear/Order.sol"
	fileGap          = "linear/Gap.sol"
	fileSetOwner     = "linear/SetOwner.sol"
	importOZ         = "@openzeppelin/contracts/token/ERC20/IERC20.sol"
	qnameMinter      = "Minter"
	qnameMinterMint  = "Minter.mint"
	qnameMinterAllow = "Minter.allow"
)

// fidelityGraph is the resolved graph of the fixture plus the source bytes,
// so a test can compare a node's byte span with the file.
type fidelityGraph struct {
	nodes      []types.Node
	edges      []types.Edge
	byID       map[string]types.Node
	unresolved map[string]int
	src        map[string][]byte
}

// loadFidelity parses every .sol file under dir and resolves the results.
func loadFidelity(t *testing.T, dir string) *fidelityGraph {
	t.Helper()
	p := sol.New(dir)
	g := &fidelityGraph{byID: map[string]types.Node{}, src: map[string][]byte{}}
	var results []*parse.ParseResult
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".sol") {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		res, parseErr := p.ParseFile(path, src)
		if parseErr != nil {
			return parseErr
		}
		g.src[filepath.ToSlash(res.Path)] = src
		results = append(results, res)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", dir, walkErr)
	}
	resolved, err := p.Resolve(results)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// The graph builder keeps one node per id and one edge per (type,
	// source, destination, line, dispatch kind); the tests look at what the
	// stored graph keeps, not at what Resolve returned.
	built, err := graph.Build([]*parse.ResolvedGraph{resolved})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(built.Nodes) != len(resolved.Nodes) {
		t.Errorf("the graph keeps %d of %d nodes: some share an id", len(built.Nodes), len(resolved.Nodes))
	}
	g.nodes, g.edges, g.unresolved = built.Nodes, built.Edges, resolved.Unresolved
	for _, n := range g.nodes {
		g.byID[n.ID] = n
	}
	return g
}

// find returns the nodes of one type and qualified name declared in file.
func (g *fidelityGraph) find(file string, nt types.NodeType, qname string) []types.Node {
	var out []types.Node
	for _, n := range g.nodes {
		if n.FilePath == file && n.Type == nt && n.QualifiedName == qname {
			out = append(out, n)
		}
	}
	return out
}

// one returns the single node of that type and qualified name in file and
// fails the test when there is none or more than one.
func (g *fidelityGraph) one(t *testing.T, file string, nt types.NodeType, qname string) types.Node {
	t.Helper()
	got := g.find(file, nt, qname)
	if len(got) != 1 {
		t.Fatalf("%s %s %q: got %d nodes, want 1", file, nt, qname, len(got))
	}
	return got[0]
}

// endpoint names one end of an edge by file and qualified name.
type endpoint struct {
	file  string
	qname string
}

// edgesBetween returns the edges of type et whose source and destination
// have the given file and qualified name.
func (g *fidelityGraph) edgesBetween(et types.EdgeType, src, dst endpoint) []types.Edge {
	var out []types.Edge
	for _, e := range g.edges {
		if e.Type != et {
			continue
		}
		s, d := g.byID[e.Src], g.byID[e.Dst]
		if s.FilePath == src.file && s.QualifiedName == src.qname &&
			d.FilePath == dst.file && d.QualifiedName == dst.qname {
			out = append(out, e)
		}
	}
	return out
}

// targets lists "file:qname" of every destination reached from src by et.
func (g *fidelityGraph) targets(et types.EdgeType, src endpoint) []string {
	var out []string
	for _, e := range g.edges {
		if e.Type != et {
			continue
		}
		s := g.byID[e.Src]
		if s.FilePath == src.file && s.QualifiedName == src.qname {
			d := g.byID[e.Dst]
			out = append(out, d.FilePath+":"+d.QualifiedName)
		}
	}
	return out
}

// TestFidelityNodeIDsAreFileKeyed covers the id collision: two files that
// declare the same contract at the same byte offset must keep two nodes, and
// each file must define its own.
func TestFidelityNodeIDsAreFileKeyed(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	seen := map[string]types.Node{}
	for _, n := range g.nodes {
		if prev, dup := seen[n.ID]; dup {
			t.Errorf("id %s shared by %s %q (%s) and %s %q (%s)", n.ID,
				prev.Type, prev.QualifiedName, prev.FilePath, n.Type, n.QualifiedName, n.FilePath)
		}
		seen[n.ID] = n
	}
	for _, file := range []string{fileMinterV1, fileMinterV2} {
		contract := g.one(t, file, types.NodeContract, qnameMinter)
		if got := g.edgesBetween(types.EdgeDefines, endpoint{file, "file:" + file}, endpoint{file, qnameMinter}); len(got) != 1 {
			t.Errorf("%s: file defines its own Minter %d times, want 1", file, len(got))
		}
		if contract.FilePath != file {
			t.Errorf("Minter node of %s is on %s", file, contract.FilePath)
		}
	}
}

// TestFidelityDeclarationSpans covers spans: a node spans its whole
// declaration, so its lines and its bytes are the declaration.
func TestFidelityDeclarationSpans(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name      string
		file      string
		nt        types.NodeType
		qname     string
		startLine int
		endLine   int
		prefix    string
		suffix    string
	}{
		{"contract", fileBase, types.NodeContract, "Base", 6, 60, "abstract contract Base {", "}"},
		{"function with body", fileBase, types.NodeFunction, "Base.pause", 45, 48, "function pause() external", "}"},
		{"function without body", fileBase, types.NodeFunction, "Base._execute", 54, 54, "function _execute(uint256 id)", ";"},
		{"constructor", fileBase, types.NodeFunction, "Base.constructor", 41, 43, "constructor() {", "}"},
		{"modifier", fileBase, types.NodeModifier, "Base.whenNotPaused", 31, 34, "modifier whenNotPaused() {", "}"},
		{"event", fileBase, types.NodeEvent, "Base.Paused", 10, 10, "event Paused(", ";"},
		{"error", fileBase, types.NodeError, "Base.NotMember", 7, 7, "error NotMember(", ";"},
		{"enum", fileBase, types.NodeEnum, "Base.Status", 12, 16, "enum Status {", "}"},
		{"enum value", fileBase, types.NodeConstant, "Base.Status.Active", 14, 14, "Active", "Active"},
		{"struct", fileBase, types.NodeStruct, "Base.Proposal", 18, 22, "struct Proposal {", "}"},
		{"struct member", fileBase, types.NodeField, "Base.Proposal.proposer", 20, 20, "address proposer", ";"},
		{"constant", fileBase, types.NodeConstant, "Base.MAX_MEMBERS", 24, 24, "uint256 public constant MAX_MEMBERS", ";"},
		{"state variable", fileBase, types.NodeField, "Base.paused", 26, 26, "bool public paused", ";"},
		{"mapping", fileBase, types.NodeMapping, "Base.members", 28, 28, "mapping(address => bool) public members", ";"},
		{"interface", fileIToken, types.NodeInterface, "IToken", 4, 12, "interface IToken {", "}"},
		{"library", fileSetLib, types.NodeContract, "SetLib", 4, 24, "library SetLib {", "}"},
		{"import", fileMinterV1, types.NodeImport, "import:../abstracts/Mid.sol", 4, 4, "import { Mid }", ";"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := g.one(t, tc.file, tc.nt, tc.qname)
			if n.StartLine != tc.startLine || n.EndLine != tc.endLine {
				t.Errorf("lines %d-%d, want %d-%d", n.StartLine, n.EndLine, tc.startLine, tc.endLine)
			}
			text := string(g.src[tc.file][n.StartByte:n.EndByte])
			if !strings.HasPrefix(text, tc.prefix) || !strings.HasSuffix(text, tc.suffix) {
				t.Errorf("text %q, want prefix %q and suffix %q", text, tc.prefix, tc.suffix)
			}
		})
	}
}

// TestFidelityDeclarationNodes covers the declaration kinds that had no
// node (errors, struct members, enum values, imports) and the constant that
// was a storage field.
func TestFidelityDeclarationNodes(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name    string
		file    string
		nt      types.NodeType
		qname   string
		subKind string
	}{
		{"error in contract", fileBase, types.NodeError, "Base.AlreadyPaused", ""},
		{"error in v1", fileMinterV1, types.NodeError, "Minter.ZeroAmount", ""},
		{"error in v2", fileMinterV2, types.NodeError, "Minter.ZeroAmount", ""},
		{"struct member", fileBase, types.NodeField, "Base.Proposal.id", "struct_member"},
		{"struct member of library struct", fileSetLib, types.NodeField, "SetLib.Set.positions", "struct_member"},
		{"enum value", fileBase, types.NodeConstant, "Base.Status.None", "enum_value"},
		{"enum value last", fileBase, types.NodeConstant, "Base.Status.Closed", "enum_value"},
		{"constant", fileBase, types.NodeConstant, "Base.MAX_MEMBERS", "constant"},
		{"immutable stays a field", fileBase, types.NodeField, "Base.deployedAt", "immutable"},
		{"import", fileMid, types.NodeImport, "import:./Base.sol", ""},
		{"import of a file outside the fixture", fileMock, types.NodeImport, "import:" + importOZ, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := g.one(t, tc.file, tc.nt, tc.qname)
			if n.SubKind != tc.subKind {
				t.Errorf("sub kind %q, want %q", n.SubKind, tc.subKind)
			}
		})
	}

	t.Run("import links to the imported file", func(t *testing.T) {
		got := g.edgesBetween(types.EdgeImports,
			endpoint{fileMinterV1, "import:../abstracts/Mid.sol"}, endpoint{fileMid, "file:" + fileMid})
		if len(got) != 1 {
			t.Errorf("import of Mid.sol links to the file %d times, want 1", len(got))
		}
		if got := g.edgesBetween(types.EdgeImports,
			endpoint{fileMinterV1, "file:" + fileMinterV1}, endpoint{fileMinterV1, "import:../abstracts/Mid.sol"}); len(got) != 1 {
			t.Errorf("file imports edge to its import node: %d, want 1", len(got))
		}
	})
	t.Run("import of an absent file has a node and no link", func(t *testing.T) {
		if got := g.targets(types.EdgeImports, endpoint{fileMock, "import:" + importOZ}); len(got) != 0 {
			t.Errorf("import of an absent file links to %v", got)
		}
	})
	t.Run("constant and immutable take no storage slot", func(t *testing.T) {
		if n := g.one(t, fileBase, types.NodeConstant, "Base.MAX_MEMBERS"); n.SlotIndex != 0 {
			t.Errorf("constant slot %d, want 0", n.SlotIndex)
		}
		if n := g.one(t, fileBase, types.NodeField, "Base.paused"); n.SlotIndex != 0 {
			t.Errorf("paused slot %d, want 0: the constant and the immutable before it use no slot", n.SlotIndex)
		}
		if n := g.one(t, fileBase, types.NodeField, "Base.memberCount"); n.SlotIndex != 1 {
			t.Errorf("memberCount slot %d, want 1", n.SlotIndex)
		}
	})
}

// TestFidelityMembership covers member naming: a member is named under its
// contract and the contract, not the file, defines it.
func TestFidelityMembership(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name    string
		file    string
		nt      types.NodeType
		qname   string
		definer endpoint
	}{
		{"event", fileBase, types.NodeEvent, "Base.Paused", endpoint{fileBase, "Base"}},
		{"error", fileBase, types.NodeError, "Base.NotMember", endpoint{fileBase, "Base"}},
		{"struct", fileBase, types.NodeStruct, "Base.Proposal", endpoint{fileBase, "Base"}},
		{"enum", fileBase, types.NodeEnum, "Base.Status", endpoint{fileBase, "Base"}},
		{"mapping", fileBase, types.NodeMapping, "Base.members", endpoint{fileBase, "Base"}},
		{"state variable", fileBase, types.NodeField, "Base.paused", endpoint{fileBase, "Base"}},
		{"constant", fileBase, types.NodeConstant, "Base.MAX_MEMBERS", endpoint{fileBase, "Base"}},
		{"function", fileBase, types.NodeFunction, "Base.pause", endpoint{fileBase, "Base"}},
		{"modifier", fileBase, types.NodeModifier, "Base.onlyMember", endpoint{fileBase, "Base"}},
		{"struct member", fileBase, types.NodeField, "Base.Proposal.id", endpoint{fileBase, "Base.Proposal"}},
		{"enum value", fileBase, types.NodeConstant, "Base.Status.Active", endpoint{fileBase, "Base.Status"}},
		{"interface method", fileIToken, types.NodeFunction, "IToken.mint", endpoint{fileIToken, "IToken"}},
		{"contract", fileBase, types.NodeContract, "Base", endpoint{fileBase, "file:" + fileBase}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g.one(t, tc.file, tc.nt, tc.qname)
			if got := g.edgesBetween(types.EdgeDefines, tc.definer, endpoint{tc.file, tc.qname}); len(got) != 1 {
				t.Errorf("%s defines %s %d times, want 1", tc.definer.qname, tc.qname, len(got))
			}
			definers := 0
			for _, e := range g.edges {
				if e.Type == types.EdgeDefines && g.byID[e.Dst].FilePath == tc.file && g.byID[e.Dst].QualifiedName == tc.qname {
					definers++
				}
			}
			if definers != 1 {
				t.Errorf("%s has %d definers, want 1", tc.qname, definers)
			}
		})
	}
}

// TestFidelityScopedTargets covers name resolution: a name resolves in the
// contract that uses it, then its bases, then the file; never in a contract
// that is not a base.
func TestFidelityScopedTargets(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name string
		et   types.EdgeType
		src  endpoint
		want []string
	}{
		{"v1 modifier is v1's own", types.EdgeHasModifier, endpoint{fileMinterV1, qnameMinterMint},
			[]string{fileMinterV1 + ":Minter.whenNotPaused", fileMinterV1 + ":Minter.notZero"}},
		{"v2 modifier is v2's own", types.EdgeHasModifier, endpoint{fileMinterV2, qnameMinterMint},
			[]string{fileMinterV2 + ":Minter.whenNotPaused"}},
		{"base modifier from the base", types.EdgeHasModifier, endpoint{fileBase, "Base.pause"},
			[]string{fileBase + ":Base.onlyMember", fileBase + ":Base.whenNotPaused"}},
		{"inherited modifier through two levels", types.EdgeHasModifier, endpoint{fileMinterV1, qnameMinterAllow},
			[]string{fileBase + ":Base.onlyMember"}},
		{"v1 event is v1's own", types.EdgeEmitsEvent, endpoint{fileMinterV1, qnameMinterMint},
			[]string{fileMinterV1 + ":Minter.Minted"}},
		{"v2 event is v2's own", types.EdgeEmitsEvent, endpoint{fileMinterV2, qnameMinterMint},
			[]string{fileMinterV2 + ":Minter.Minted"}},
		{"event inherited from the interface", types.EdgeEmitsEvent, endpoint{fileMock, "MockToken.mint"},
			[]string{fileIToken + ":IToken.Minted"}},
		{"v2 mapping write stays in v2", types.EdgeWritesMapping, endpoint{fileMinterV2, qnameMinterMint},
			[]string{fileMinterV2 + ":Minter.quota"}},
		{"v1 extends the Mid its file imports", types.EdgeExtends, endpoint{fileMinterV1, qnameMinter},
			[]string{fileMid + ":Mid"}},
		{"v2 extends the Mid its file imports", types.EdgeExtends, endpoint{fileMinterV2, qnameMinter},
			[]string{fileMid + ":Mid"}},
		{"base listed on a later line than the contract", types.EdgeExtends, endpoint{fileGap, "Commented"},
			[]string{fileGap + ":GapBase"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.targets(tc.et, tc.src), tc.want)
		})
	}
}

// assertSameSet fails when got and want differ as sets.
func assertSameSet(t *testing.T, got, want []string) {
	t.Helper()
	gotSet, wantSet := map[string]bool{}, map[string]bool{}
	for _, s := range got {
		gotSet[s] = true
	}
	for _, s := range want {
		wantSet[s] = true
	}
	for s := range wantSet {
		if !gotSet[s] {
			t.Errorf("missing %s (got %v)", s, got)
		}
	}
	for s := range gotSet {
		if !wantSet[s] {
			t.Errorf("unexpected %s (want %v)", s, want)
		}
	}
}

// TestFidelityBodyRelations covers the relations a function body makes:
// calls inside the contract and to a library, state variable and mapping
// reads and writes, and reverts with a custom error.
func TestFidelityBodyRelations(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	v1 := func(q string) endpoint { return endpoint{fileMinterV1, q} }
	base := func(q string) endpoint { return endpoint{fileBase, q} }
	cases := []struct {
		name string
		et   types.EdgeType
		src  endpoint
		dst  endpoint
		line int
	}{
		{"internal call", types.EdgeCalls, v1(qnameMinterMint), v1("Minter._record"), 41},
		{"direct library call", types.EdgeCalls, v1(qnameMinterMint), endpoint{fileSetLib, "SetLib.hash"}, 42},
		{"super call", types.EdgeCalls, v1("Minter._onMemberAdded"), base("Base._onMemberAdded"), 71},
		{"call from a modifier", types.EdgeCalls, base("Base.onlyMember"), base("Base._checkMember"), 37},
		{"call inside a library", types.EdgeCalls, endpoint{fileSetLib, "SetLib.add"}, endpoint{fileSetLib, "SetLib.contains"}, 15},
		{"read of an inherited field in a modifier", types.EdgeReadsField, v1("Minter.whenNotPaused"), base("Base.paused"), 23},
		{"read of a field", types.EdgeReadsField, v1("Minter._midHook"), v1("Minter.minted"), 76},
		{"read of an inherited constant", types.EdgeReadsField, v1(qnameMinterAllow), base("Base.MAX_MEMBERS"), 55},
		{"read of an inherited field", types.EdgeReadsField, endpoint{fileMid, "Mid._midHook"}, base("Base.memberCount"), 10},
		{"read of a field used as an index", types.EdgeReadsField, v1("Minter._record"), v1("Minter.minted"), 62},
		{"write of a field", types.EdgeWritesField, base("Base.pause"), base("Base.paused"), 46},
		{"compound write of a field", types.EdgeWritesField, v1("Minter._record"), v1("Minter.minted"), 61},
		{"write inside try", types.EdgeWritesField, v1(qnameMinterMint), v1("Minter.minted"), 45},
		{"write of an inherited field", types.EdgeWritesField, v1("Minter._midHook"), endpoint{fileMid, "Mid.midValue"}, 76},
		{"write of an immutable in the constructor", types.EdgeWritesField, base("Base.constructor"), base("Base.deployedAt"), 42},
		{"write in a constructor", types.EdgeWritesField, v1("Minter.constructor"), v1("Minter.token"), 33},
		{"mapping read", types.EdgeReadsMapping, base("Base._checkMember"), base("Base.members"), 51},
		{"mapping read on the right side", types.EdgeReadsMapping, v1(qnameMinterAllow), v1("Minter.quota"), 56},
		{"read of an inherited mapping", types.EdgeReadsMapping, v1("Minter._execute"), base("Base.proposals"), 67},
		{"mapping write with =", types.EdgeWritesMapping, v1(qnameMinterAllow), v1("Minter.quota"), 55},
		{"nested mapping write", types.EdgeWritesMapping, v1(qnameMinterAllow), v1("Minter.grants"), 56},
		{"mapping delete", types.EdgeWritesMapping, v1(qnameMinterAllow), v1("Minter.quota"), 57},
		{"struct member write through a mapping", types.EdgeWritesMapping, v1("Minter._record"), base("Base.proposals"), 62},
		{"mapping write in the base", types.EdgeWritesMapping, base("Base._onMemberAdded"), base("Base.members"), 57},
		{"revert with an error", types.EdgeRevertsWith, v1(qnameMinterMint), v1("Minter.ZeroAmount"), 39},
		{"revert inside catch", types.EdgeRevertsWith, v1(qnameMinterMint), v1("Minter.ZeroAmount"), 47},
		{"require with an inherited error", types.EdgeRevertsWith, v1("Minter.whenNotPaused"), base("Base.AlreadyPaused"), 23},
		{"require with an error in a modifier", types.EdgeRevertsWith, v1("Minter.notZero"), v1("Minter.NotAllowed"), 28},
		{"revert in the base", types.EdgeRevertsWith, base("Base._checkMember"), base("Base.NotMember"), 51},
		{"v2 revert stays in v2", types.EdgeRevertsWith, endpoint{fileMinterV2, qnameMinterMint}, endpoint{fileMinterV2, "Minter.ZeroAmount"}, 23},
		{"call through an interface-typed variable", types.EdgeInvokes, v1(qnameMinterMint), endpoint{fileIToken, "IToken.mint"}, 43},
		{"call through an interface-typed variable in try", types.EdgeInvokes, v1(qnameMinterMint), endpoint{fileIToken, "IToken.burn"}, 44},
		{"call through an interface-typed variable behind a comparison", types.EdgeInvokes, v1("Minter.withinSupply"), endpoint{fileIToken, "IToken.totalSupply"}, 80},
		{"using-for call behind a negation", types.EdgeCalls, v1(qnameMinterMint), endpoint{fileSetLib, "SetLib.contains"}, 40},
		{"using-for call", types.EdgeCalls, v1(qnameMinterAllow), endpoint{fileSetLib, "SetLib.add"}, 54},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found := false
			for _, e := range g.edgesBetween(tc.et, tc.src, tc.dst) {
				if e.Line == tc.line {
					found = true
				}
			}
			if !found {
				t.Errorf("no %s edge %s -> %s:%s on line %d (targets: %v)",
					tc.et, tc.src.qname, tc.dst.file, tc.dst.qname, tc.line, g.targets(tc.et, tc.src))
			}
		})
	}

	t.Run("a local or a parameter is not a state variable", func(t *testing.T) {
		// SetLib.add uses only its parameters: its field edges end at the
		// members of the struct its `set` parameter has as type, never at
		// a state variable.
		for _, et := range []types.EdgeType{types.EdgeReadsField, types.EdgeWritesField} {
			for _, e := range g.edges {
				if s := g.byID[e.Src]; e.Type != et || s.FilePath != fileSetLib || s.QualifiedName != "SetLib.add" {
					continue
				}
				if d := g.byID[e.Dst]; d.SubKind != "struct_member" {
					t.Errorf("SetLib.add has a %s edge to %s %q; it uses only parameters", et, d.Type, d.QualifiedName)
				}
			}
		}
	})
	t.Run("v2 code never points at v1", func(t *testing.T) {
		for _, e := range g.edges {
			s, d := g.byID[e.Src], g.byID[e.Dst]
			if s.FilePath == fileMinterV2 && d.FilePath == fileMinterV1 {
				t.Errorf("%s edge from %s (v2) to %s (v1)", e.Type, s.QualifiedName, d.QualifiedName)
			}
		}
	})
}

// TestFidelityModifierRepeats covers repeated modifier invocations: three
// uses of one modifier on one line are three uses, not one.
func TestFidelityModifierRepeats(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	uses := 0
	for _, e := range g.edgesBetween(types.EdgeHasModifier,
		endpoint{fileMinterV1, qnameMinterMint}, endpoint{fileMinterV1, "Minter.notZero"}) {
		uses += e.Count
	}
	if uses != 3 {
		t.Errorf("notZero is used %d times by Minter.mint, want 3", uses)
	}
}

// TestFidelityDeclarationDetails covers the details that were empty:
// signature, visibility and doc comment.
func TestFidelityDeclarationDetails(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name       string
		file       string
		nt         types.NodeType
		qname      string
		signature  string
		visibility string
		doc        string
	}{
		{"external function with NatSpec", fileMinterV1, types.NodeFunction, qnameMinterMint,
			"function mint(address to, uint256 amount) external returns (bool)", "external",
			"@notice Mints `amount` to `to`.\n@param to receiver"},
		{"internal view function", fileBase, types.NodeFunction, "Base._checkMember",
			"function _checkMember(address who) internal view", "internal", ""},
		{"virtual function without body", fileBase, types.NodeFunction, "Base._execute",
			"function _execute(uint256 id) internal virtual returns (bool)", "internal", ""},
		{"interface method", fileIToken, types.NodeFunction, "IToken.totalSupply",
			"function totalSupply() external view returns (uint256)", "external", ""},
		{"constructor", fileMinterV1, types.NodeFunction, "Minter.constructor",
			"constructor(address tokenAddress)", "", ""},
		{"modifier", fileMinterV1, types.NodeModifier, "Minter.notZero",
			"modifier notZero(address who)", "", ""},
		{"event", fileMinterV1, types.NodeEvent, "Minter.Minted",
			"event Minted(address indexed to, uint256 amount)", "", ""},
		{"error", fileMinterV1, types.NodeError, "Minter.NotAllowed",
			"error NotAllowed(address who)", "", ""},
		{"contract with NatSpec", fileBase, types.NodeContract, "Base",
			"abstract contract Base", "", "@title Base\n@notice Shared governance state."},
		{"contract header over several lines with a comment", fileGap, types.NodeContract, "Commented",
			"contract Commented is GapBase", "", ""},
		{"public state variable", fileBase, types.NodeField, "Base.paused", "bool", "public", ""},
		{"internal mapping", fileBase, types.NodeMapping, "Base.proposals",
			"mapping(uint256 => Proposal)", "internal", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := g.one(t, tc.file, tc.nt, tc.qname)
			if n.Signature != tc.signature {
				t.Errorf("signature %q, want %q", n.Signature, tc.signature)
			}
			if n.Visibility != tc.visibility {
				t.Errorf("visibility %q, want %q", n.Visibility, tc.visibility)
			}
			if n.DocComment != tc.doc {
				t.Errorf("doc %q, want %q", n.DocComment, tc.doc)
			}
		})
	}
}

// TestFidelityOverrides covers overriding without the keyword, overriding a
// function a grandparent declares, and a base that is not in the graph.
func TestFidelityOverrides(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name string
		src  endpoint
		want []string
	}{
		{"interface method implemented without override", endpoint{fileMock, "MockToken.mint"},
			[]string{fileIToken + ":IToken.mint"}},
		{"interface view method implemented without override", endpoint{fileMock, "MockToken.totalSupply"},
			[]string{fileIToken + ":IToken.totalSupply"}},
		{"override of a grandparent's function", endpoint{fileMinterV1, "Minter._execute"},
			[]string{fileBase + ":Base._execute"}},
		{"override of a parent's function", endpoint{fileMinterV1, "Minter._midHook"},
			[]string{fileMid + ":Mid._midHook"}},
		{"v2 override of a grandparent's function", endpoint{fileMinterV2, "Minter._execute"},
			[]string{fileBase + ":Base._execute"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.targets(types.EdgeOverrides, tc.src), tc.want)
		})
	}

	t.Run("a base outside the graph is counted, not invented", func(t *testing.T) {
		assertSameSet(t, g.targets(types.EdgeImplements, endpoint{fileMock, "MockToken"}),
			[]string{fileIToken + ":IToken"})
		for _, n := range g.nodes {
			if n.Name == "IERC20" {
				t.Errorf("node %s %q invented for a file that is not there", n.Type, n.QualifiedName)
			}
		}
		if g.unresolved[parse.UnresolvedNoMatch] == 0 {
			t.Errorf("the missing base IERC20 is not counted as unresolved: %v", g.unresolved)
		}
	})
}

// TestFidelityLinearisation covers the order of bases. Solidity lists bases
// from the most base-like to the most derived (`Leaf is Root, Left, Right`),
// so names are looked up in Leaf, Right, Left, Root. Read the other way the
// hierarchy has no linearisation at all, and a contract that compiles was
// flagged as one that cannot.
func TestFidelityLinearisation(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	leaf := endpoint{fileOrder, "Leaf.f"}
	t.Run("super is the last base listed", func(t *testing.T) {
		assertSameSet(t, g.targets(types.EdgeCalls, leaf), []string{fileOrder + ":Right.f"})
	})
	t.Run("explicit override reaches every named base", func(t *testing.T) {
		assertSameSet(t, g.targets(types.EdgeOverrides, leaf),
			[]string{fileOrder + ":Root.f", fileOrder + ":Left.f", fileOrder + ":Right.f"})
	})
	t.Run("a hierarchy that compiles has no fallback flag", func(t *testing.T) {
		if g.one(t, fileOrder, types.NodeContract, "Leaf").HasInheritanceMROFallback {
			t.Errorf("Leaf is flagged as having no linearisation")
		}
	})
	t.Run("slots follow the bases, mappings included", func(t *testing.T) {
		cases := []struct {
			file  string
			nt    types.NodeType
			qname string
			slot  int
		}{
			{fileOrder, types.NodeField, "Root.rootValue", 0},
			{fileOrder, types.NodeMapping, "Right.rightMap", 1},
			{fileOrder, types.NodeField, "Leaf.leafValue", 2},
			{fileGap, types.NodeField, "GapBase.__gap", 1},
			// The base ends with a four-slot array, so its child starts
			// at slot 5, not at the array's first slot plus one.
			{fileGap, types.NodeField, "GapChild.owner", 5},
			{fileGap, types.NodeMapping, "GapChild.flags", 6},
			// A struct with a dynamic array and a mapping takes two slots
			// (GovCouncil's AddressSet variables are documented at 0x32,
			// 0x34 and 0x36).
			{fileGap, types.NodeField, "SetUser.blocked", 0},
			{fileGap, types.NodeField, "SetUser.allowed", 2},
			{fileGap, types.NodeField, "SetUser.manager", 4},
			{fileGap, types.NodeField, "Commented.extra", 5},
			// The struct and the enum are declared in another file.
			{fileSetOwner, types.NodeField, "SetOwner.second", 2},
			{fileSetOwner, types.NodeField, "SetOwner.phase", 2},
			{fileSetOwner, types.NodeField, "SetOwner.done", 2},
			{fileSetOwner, types.NodeField, "SetOwner.third", 3},
		}
		for _, tc := range cases {
			if n := g.one(t, tc.file, tc.nt, tc.qname); n.SlotIndex != tc.slot {
				t.Errorf("%s slot %d, want %d", tc.qname, n.SlotIndex, tc.slot)
			}
		}
	})
}

// parseFidelityFiles parses every .sol file under dir with p, keyed by the
// path relative to dir.
func parseFidelityFiles(t *testing.T, p *sol.Parser, dir string) map[string]*parse.ParseResult {
	t.Helper()
	out := map[string]*parse.ParseResult{}
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".sol") {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		res, parseErr := p.ParseFile(path, src)
		if parseErr != nil {
			return parseErr
		}
		res.Path = filepath.ToSlash(res.Path)
		out[res.Path] = res
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", dir, walkErr)
	}
	return out
}

// edgeFact is what an edge says, without its row id.
type edgeFact struct {
	et       types.EdgeType
	src, dst string
	line     int
	count    int
	kind     string
}

// TestFidelityIncrementalResolve covers an incremental build. Only the
// changed file is parsed; every other file reaches Resolve as its stored
// nodes, without edges and without pending references. The changed file must
// still get every edge a full build gives it, and the same slot indexes:
// membership, imports, bases and parameter counts are read from nodes.
func TestFidelityIncrementalResolve(t *testing.T) {
	coldParser := sol.New(fidelityDir)
	coldFiles := parseFidelityFiles(t, coldParser, fidelityDir)
	var all []*parse.ParseResult
	for _, r := range coldFiles {
		all = append(all, r)
	}
	cold, err := coldParser.Resolve(all)
	if err != nil {
		t.Fatalf("cold Resolve: %v", err)
	}
	coldNode := map[string]types.Node{}
	storedByFile := map[string][]types.Node{}
	for _, n := range cold.Nodes {
		coldNode[n.ID] = n
		storedByFile[n.FilePath] = append(storedByFile[n.FilePath], n)
	}

	for dirty := range coldFiles {
		t.Run(dirty, func(t *testing.T) {
			// A new parser sees the changed file only, as in an incremental
			// build: what it knows of the other files is their nodes.
			p := sol.New(fidelityDir)
			full := filepath.Join(fidelityDir, filepath.FromSlash(dirty))
			src, readErr := os.ReadFile(full)
			if readErr != nil {
				t.Fatalf("read %s: %v", full, readErr)
			}
			fresh, parseErr := p.ParseFile(full, src)
			if parseErr != nil {
				t.Fatalf("parse %s: %v", full, parseErr)
			}
			fresh.Path = dirty
			results := []*parse.ParseResult{fresh}
			for path := range coldFiles {
				if path != dirty {
					results = append(results, &parse.ParseResult{Path: path, Nodes: storedByFile[path]})
				}
			}
			got, err := p.Resolve(results)
			if err != nil {
				t.Fatalf("incremental Resolve: %v", err)
			}
			have := map[edgeFact]bool{}
			for _, e := range got.Edges {
				have[edgeFact{e.Type, e.Src, e.Dst, e.Line, e.Count, e.DispatchKind}] = true
			}
			for _, e := range cold.Edges {
				if coldNode[e.Src].FilePath != dirty {
					continue
				}
				if !have[edgeFact{e.Type, e.Src, e.Dst, e.Line, e.Count, e.DispatchKind}] {
					t.Errorf("lost %s edge %s -> %s (%s) line %d", e.Type,
						coldNode[e.Src].QualifiedName, coldNode[e.Dst].QualifiedName, coldNode[e.Dst].FilePath, e.Line)
				}
			}
			for _, n := range got.Nodes {
				want := coldNode[n.ID]
				if n.SlotIndex != want.SlotIndex {
					t.Errorf("%s %s slot %d, want %d as in a full build", n.FilePath, n.QualifiedName, n.SlotIndex, want.SlotIndex)
				}
				// Pass 2 decides this marker for a receiver whose type another
				// file declares (`route.hop.call(...)`).
				if n.HasExternalCall != want.HasExternalCall {
					t.Errorf("%s %s external call marker %v, want %v as in a full build",
						n.FilePath, n.QualifiedName, n.HasExternalCall, want.HasExternalCall)
				}
			}
		})
	}
}
