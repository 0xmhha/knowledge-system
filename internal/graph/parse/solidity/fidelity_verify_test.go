package solidity_test

import (
	"fmt"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	sol "github.com/0xmhha/knowledge-system/internal/graph/parse/solidity"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Fixtures of the round that compared the graph with the compiler's syntax
// tree, under testdata/fidelity/verify. Each test names what the source holds
// at a line of one of them.
const (
	fileRules  = "verify/Rules.sol"
	fileLedger = "verify/Ledger.sol"
	fileDialer = "verify/Dialer.sol"
	fileLegacy = "verify/Legacy.sol"
)

// Dispatch kinds the tests read back from edges.
const (
	kindImportSymbol    = "import_symbol"
	kindInterfaceMethod = "interface_method"
)

// TestVerifyPointerReads covers a read through a local declared `storage`:
// it is a read of the state variable or mapping the pointer was bound to,
// like a write through it is a write.
func TestVerifyPointerReads(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name string
		et   types.EdgeType
		src  endpoint
		want []string
	}{
		{"pointer to a mapping entry", types.EdgeReadsMapping, endpoint{fileLedger, "Ledger.read"},
			[]string{"40 Ledger.entries@31", "41 Ledger.entries@31", "43 Ledger.entries@31"}},
		{"pointer to an array element", types.EdgeReadsField, endpoint{fileLedger, "Ledger.read"}, []string{
			"41 Ledger.Entry.amount@20",
			"42 Ledger.history@32",
			"43 Ledger.history@32", "43 Ledger.Entry.stage@21", "43 Ledger.Entry.owner@19", "43 Ledger.Stage.Live@15",
		}},
		{"pointer bound to another pointer's member", types.EdgeReadsMapping, endpoint{filePointers, "Pointers.vote"},
			[]string{"16 Pointers.proposals@11", "19 Pointers.proposals@11"}},
		{"pointer returned by a function", types.EdgeReadsMapping, endpoint{filePointers, "Pointers.grow"},
			[]string{"40 Pointers.lists@12"}},
		{"a storage parameter cannot be followed", types.EdgeReadsMapping, endpoint{filePointers, "Pointers.through"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.siteEdges(tc.et, tc.src), tc.want)
		})
	}
	t.Run("two reads through one pointer on a line count twice", func(t *testing.T) {
		if got := g.usesOn(types.EdgeReadsMapping, endpoint{fileLedger, "Ledger.read"}, endpoint{fileLedger, "Ledger.entries"}, 43); got != 2 {
			t.Errorf("entries is read %d times on line 43, want 2 (entry.stage and entry.owner)", got)
		}
	})
}

// usesOn returns how many times src uses dst by et on one line: the sum of
// the counts of the edges there.
func (g *fidelityGraph) usesOn(et types.EdgeType, src, dst endpoint, line int) int {
	uses := 0
	for _, e := range g.edgesBetween(et, src, dst) {
		if e.Line == line {
			uses += e.Count
		}
	}
	return uses
}

// TestVerifyStructLiterals covers a struct constructor: `S({a: 1, b: 2})`
// and `S(1, 2)` both initialise every member of the struct, which is a write
// of each member at the line of the constructor. Before 0.7 a struct with a
// mapping member can be constructed, and the mapping is left out.
func TestVerifyStructLiterals(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name string
		src  endpoint
		want []string
	}{
		{"named and positional", endpoint{fileLedger, "Ledger.add"}, []string{
			"48 Ledger.Entry.owner@19", "48 Ledger.Entry.amount@20", "48 Ledger.Entry.stage@21",
			"53 Ledger.history@32",
			"53 Ledger.Entry.owner@19", "53 Ledger.Entry.amount@20", "53 Ledger.Entry.stage@21",
		}},
		{"nested and qualified", endpoint{fileUses, "Uses.open"}, []string{
			"41 Uses.phase@34",
			"43 Uses.Item.owner@29",
			"44 Uses.Item.size@30",
			"45 Uses.Item.box@31", "45 Shapes.Box.width@15",
			"47 Uses.registry@35",
			"49 Uses.Item.owner@29", "49 Uses.Item.size@30", "49 Uses.Item.box@31",
			"49 Shapes.Box.width@15", "49 Shapes.Box.kind@16",
			"50 Uses.list@37",
		}},
		{"a mapping member is not initialised", endpoint{fileLegacy, "Legacy.reset"}, []string{
			"16 Legacy.shelf@13", "16 Legacy.Shelf.count@8", "16 Legacy.Shelf.keeper@10",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.siteEdges(types.EdgeWritesField, tc.src), tc.want)
		})
	}
}

// TestVerifyNamesInExpressions covers a type named inside an expression:
// the enum of `Enum.Value`, the library of `Lib.f(x)` and `Lib.Err()`, the
// interface of a cast and of `type(I).interfaceId`, and the type a using
// directive binds.
func TestVerifyNamesInExpressions(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name string
		src  endpoint
		want []string
	}{
		{"enum of a value, library of an error", endpoint{fileLedger, "Ledger.read"},
			[]string{"40 Ledger.Entry@18", "41 Rules@4", "42 Ledger.Entry@18", "43 Ledger.Stage@13"}},
		{"library of a call, enum of a value in a literal", endpoint{fileLedger, "Ledger.add"}, []string{
			"47 Rules@4", "48 Ledger.Entry@18", "50 Rules@4", "51 Ledger.Stage@13",
			"53 Ledger.Entry@18", "53 Ledger.Stage@13",
		}},
		{"interface of a cast, by its import alias", endpoint{fileLedger, "Ledger.constructor"},
			[]string{"34 Rules.Limits@5", "35 IOracle@27"}},
		{"interface of type(I)", endpoint{fileLedger, "Ledger.supports"}, []string{"57 IOracle@27"}},
		{"type bound by a using directive", endpoint{fileLedger, "Ledger"}, []string{"11 Rules.Limits@5"}},
		{"enum reached through its library", endpoint{fileUses, "Uses.open"}, []string{
			"40 Uses.Phase@23", "41 Uses.Phase@23", "42 Uses.Item@28", "44 Shapes@8",
			"46 Shapes@8", "46 Shapes.Kind@9", "47 IRegistry@4",
			"49 Uses.Item@28", "49 Shapes.Box@14", "49 Shapes@8", "49 Shapes.Kind@9",
		}},
		{"file-level enum", endpoint{fileTopLevel, "C.bump"}, []string{"34 TopE@13"}},
		{"library of a direct call", endpoint{fileOverloads, "UsesL.run"}, []string{"47 L@4", "48 L@4"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.siteEdges(types.EdgeUsesType, tc.src), tc.want)
		})
	}
	t.Run("the library named twice on a line counts twice", func(t *testing.T) {
		if got := g.usesOn(types.EdgeUsesType, endpoint{fileUses, "Uses.open"}, endpoint{fileUses, "Shapes"}, 49); got != 2 {
			t.Errorf("Shapes is named %d times on line 49, want 2 (Shapes.Box and Shapes.Kind)", got)
		}
	})
}

// TestVerifyImportedSymbols covers `import { A, B as C } from "./x.sol"`:
// the import is linked to the declaration of every symbol it names, the
// aliased one included, besides the file.
func TestVerifyImportedSymbols(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name string
		src  endpoint
		want []string
	}{
		{"three symbols over several lines, one aliased", endpoint{fileLedger, "import:./Rules.sol"},
			[]string{"4 file:" + fileRules + "@1 ", "4 Rules@4 " + kindImportSymbol, "4 Caps@21 " + kindImportSymbol, "4 IOracle@27 " + kindImportSymbol}},
		{"one symbol", endpoint{fileDialer, "import:./Rules.sol"},
			[]string{"4 file:" + fileRules + "@1 ", "4 IOracle@27 " + kindImportSymbol}},
		{"aliased symbol", endpoint{fileTopLevel, "import:./Things.sol"},
			[]string{"4 file:" + fileThings + "@1 ", "4 Thing@4 " + kindImportSymbol, "5 file:" + fileThings + "@1 "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, e := range g.edges {
				s := g.byID[e.Src]
				if e.Type != types.EdgeImports || s.FilePath != tc.src.file || s.QualifiedName != tc.src.qname {
					continue
				}
				d := g.byID[e.Dst]
				got = append(got, fmt.Sprintf("%d %s@%d %s", e.Line, d.QualifiedName, d.StartLine, e.DispatchKind))
			}
			assertSameSet(t, got, tc.want)
		})
	}
}

// TestVerifyTypeText covers the type a declaration stores: the type as
// written, with its qualifier, and a member access through a variable of a
// qualified type resolves to the struct the qualifier names when two
// libraries declare a struct of the same name.
func TestVerifyTypeText(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	signatures := []struct {
		nt    types.NodeType
		qname string
		want  string
	}{
		{types.NodeField, "Ledger.bounds", "Rules.Limits"},
		{types.NodeField, "Ledger.ceiling", "Caps.Limits"},
		{types.NodeField, "Ledger.oracle", "Feed"},
		{types.NodeField, "Ledger.history", "Entry[]"},
		{types.NodeMapping, "Ledger.entries", "mapping(uint256 => Entry)"},
	}
	for _, tc := range signatures {
		t.Run(tc.qname, func(t *testing.T) {
			if got := g.one(t, fileLedger, tc.nt, tc.qname).Signature; got != tc.want {
				t.Errorf("signature %q, want %q", got, tc.want)
			}
		})
	}
	edges := []struct {
		name string
		src  endpoint
		want []string
	}{
		{"locals of two structs of one name", endpoint{fileLedger, "Ledger.spread"},
			[]string{"63 Rules.Limits.high@7", "63 Rules.Limits.low@6", "63 Caps.Limits.cap@23"}},
		{"state variable of a qualified struct type", endpoint{fileLedger, "Ledger.pages"},
			[]string{"69 Ledger.ceiling@29", "69 Caps.Limits.cap@23"}},
	}
	for _, tc := range edges {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.siteEdges(types.EdgeReadsField, tc.src), tc.want)
		})
	}
	t.Run("a using-for call through a qualified receiver type still binds", func(t *testing.T) {
		assertSameSet(t, g.siteEdges(types.EdgeCalls, endpoint{fileLedger, "Ledger.add"}),
			[]string{"47 Rules.within@12", "50 Rules.scale@16"})
	})
}

// TestVerifyNatSpec covers the comment that documents a declaration by the
// compiler's rule: the last `/** */` block or run of `///` lines before it,
// whether or not a blank line or a plain comment stands between them.
func TestVerifyNatSpec(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name  string
		nt    types.NodeType
		qname string
		want  string
	}{
		{"block, blank line, declaration", types.NodeField, "Ledger.bounds", "@dev Bounds every amount must stay in."},
		{"no comment of its own", types.NodeField, "Ledger.ceiling", ""},
		{"a plain comment after the previous declaration", types.NodeField, "Ledger.oracle", ""},
		{"doc line, plain comment, declaration", types.NodeFunction, "Ledger.pages", "@notice Sum of the pages."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.one(t, fileLedger, tc.nt, tc.qname).DocComment; got != tc.want {
				t.Errorf("doc %q, want %q", got, tc.want)
			}
		})
	}
}

// TestVerifyCanonicalIDs covers the canonical id of a callable: every
// Function node, a constructor included, ends with its parameter types as
// written.
func TestVerifyCanonicalIDs(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		file  string
		nt    types.NodeType
		qname string
		want  string
	}{
		{fileLedger, types.NodeFunction, "Ledger.constructor", fileLedger + ":Ledger.constructor(address,Rules.Limits)"},
		{fileLedger, types.NodeFunction, "Ledger.pages", fileLedger + ":Ledger.pages(mapping(uint256 => uint256))"},
		{fileLedger, types.NodeFunction, "Ledger.supports", fileLedger + ":Ledger.supports(bytes4)"},
		{fileBase, types.NodeFunction, "Base.constructor", fileBase + ":Base.constructor()"},
		{fileBase, types.NodeModifier, "Base.onlyMember", fileBase + ":Base.onlyMember"},
	}
	for _, tc := range cases {
		t.Run(tc.qname, func(t *testing.T) {
			if got := g.one(t, tc.file, tc.nt, tc.qname).CanonicalID; got != tc.want {
				t.Errorf("canonical id %q, want %q", got, tc.want)
			}
		})
	}
}

// TestVerifyExternalCallMarker covers has_external_call: the callable makes
// a low-level call (call, delegatecall, staticcall) to an address. A call
// through a variable typed as an interface is not one: it is an invokes edge
// to the interface's method.
func TestVerifyExternalCallMarker(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name          string
		file          string
		qname         string
		external      bool
		lowLevel      bool
		valueTransfer bool
	}{
		{"msg.sender with value", fileDialer, "Dialer.paySender", true, true, true},
		{"element of an address array", fileDialer, "Dialer.fanOut", true, true, false},
		{"address member of a struct, through a pointer", fileDialer, "Dialer.viaRoute", true, true, false},
		{"calls through interface-typed variables", fileDialer, "Dialer.typed", false, false, false},
		{"no call at all", fileDialer, "Dialer.quiet", false, false, false},
		{"address state variable", fileLowLevel, "LowLevel.send", true, true, false},
		{"msg.sender with value in the review fixture", fileLowLevel, "LowLevel.refund", true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := g.one(t, tc.file, types.NodeFunction, tc.qname)
			if n.HasExternalCall != tc.external || n.HasLowLevelCall != tc.lowLevel || n.HasValueTransfer != tc.valueTransfer {
				t.Errorf("external %v low-level %v value transfer %v, want %v %v %v",
					n.HasExternalCall, n.HasLowLevelCall, n.HasValueTransfer, tc.external, tc.lowLevel, tc.valueTransfer)
			}
		})
	}
}

// TestVerifyInvokesConfidence covers the confidence of a call through a
// variable typed as an interface: one method fits the call, so the edge is
// EXTRACTED inside the file and INFERRED across files; two overloads that
// take the same number of arguments cannot be told apart and are AMBIGUOUS.
func TestVerifyInvokesConfidence(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	var got []string
	for _, e := range g.edges {
		s := g.byID[e.Src]
		if e.Type != types.EdgeInvokes || s.FilePath != fileDialer || s.QualifiedName != "Dialer.typed" {
			continue
		}
		d := g.byID[e.Dst]
		got = append(got, fmt.Sprintf("%d %s@%d %s %s", e.Line, d.QualifiedName, d.StartLine, e.Confidence, e.DispatchKind))
	}
	assertSameSet(t, got, []string{
		"40 ISink.take@7 " + string(types.ConfExtracted) + " " + kindInterfaceMethod,
		"41 IOracle.set@30 " + string(types.ConfAmbiguous) + " " + kindInterfaceMethod,
		"41 IOracle.set@32 " + string(types.ConfAmbiguous) + " " + kindInterfaceMethod,
		"42 IOracle.price@28 " + string(types.ConfInferred) + " " + kindInterfaceMethod,
	})
}

// storedRefs returns the references of one file as the graph store hands
// them back on an incremental build: one row per source, edge type, target
// and line (the first one written wins), without byte offset and order.
func storedRefs(refs []parse.PendingRef) []parse.PendingRef {
	type rowKey struct {
		src    string
		et     types.EdgeType
		target string
		line   int
	}
	seen := map[rowKey]bool{}
	var out []parse.PendingRef
	for _, pr := range refs {
		key := rowKey{pr.SrcID, pr.EdgeType, pr.TargetQName, pr.Line}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, parse.PendingRef{
			SrcID: pr.SrcID, EdgeType: pr.EdgeType, TargetQName: pr.TargetQName,
			TargetCanonicalID: pr.TargetCanonicalID, HintFile: pr.HintFile,
			Line: pr.Line, DispatchKind: pr.DispatchKind,
		})
	}
	return out
}

// TestVerifyResolveFromStoredReferences covers the other half of an
// incremental build: an unchanged file whose references point into a changed
// file is resolved again from the references the store kept for it. Resolved
// from those, every file must get the edges a full build gives it, with the
// same counts: two uses of one name on a line are still two, and a read and
// a write of one name on a line are still both there.
func TestVerifyResolveFromStoredReferences(t *testing.T) {
	coldParser := sol.New(fidelityDir)
	coldFiles := parseFidelityFiles(t, coldParser, fidelityDir)
	var all, stored []*parse.ParseResult
	pass1 := map[edgeFact]int{}
	for path, r := range coldFiles {
		all = append(all, r)
		for _, e := range r.Edges {
			pass1[edgeFact{e.Type, e.Src, e.Dst, e.Line, e.Count, e.DispatchKind}]++
		}
		stored = append(stored, &parse.ParseResult{Path: path, Pending: storedRefs(r.Pending)})
	}
	cold, err := coldParser.Resolve(all)
	if err != nil {
		t.Fatalf("cold Resolve: %v", err)
	}
	node := map[string]types.Node{}
	byFile := map[string][]types.Node{}
	for _, n := range cold.Nodes {
		node[n.ID] = n
		byFile[n.FilePath] = append(byFile[n.FilePath], n)
	}
	for _, r := range stored {
		r.Nodes = byFile[r.Path]
	}
	got, err := sol.New(fidelityDir).Resolve(stored)
	if err != nil {
		t.Fatalf("Resolve from stored references: %v", err)
	}

	// What Pass 2 makes in a full build: its edges without those of Pass 1.
	want := map[edgeFact]int{}
	for _, e := range cold.Edges {
		want[edgeFact{e.Type, e.Src, e.Dst, e.Line, e.Count, e.DispatchKind}]++
	}
	for fact, n := range pass1 {
		if want[fact] -= n; want[fact] <= 0 {
			delete(want, fact)
		}
	}
	have := map[edgeFact]int{}
	for _, e := range got.Edges {
		have[edgeFact{e.Type, e.Src, e.Dst, e.Line, e.Count, e.DispatchKind}]++
	}
	describe := func(f edgeFact) string {
		return fmt.Sprintf("%s %s:%s -> %s line %d count %d %s", f.et,
			node[f.src].FilePath, node[f.src].QualifiedName, node[f.dst].QualifiedName, f.line, f.count, f.kind)
	}
	for fact, n := range want {
		if have[fact] != n {
			t.Errorf("a full build has %d, the stored references give %d: %s", n, have[fact], describe(fact))
		}
	}
	for fact, n := range have {
		if want[fact] == 0 {
			t.Errorf("only the stored references give (%d): %s", n, describe(fact))
		}
	}
}

// TestVerifyEdgeFilePath covers the file of an edge: every edge, whatever
// its type, names the file its site is in, which is the file of the
// declaration it starts at.
func TestVerifyEdgeFilePath(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	missing := map[types.EdgeType]int{}
	for _, e := range g.edges {
		if want := g.byID[e.Src].FilePath; e.FilePath != want {
			if missing[e.Type] == 0 {
				t.Errorf("%s edge from %s on line %d has file %q, want %q",
					e.Type, g.byID[e.Src].QualifiedName, e.Line, e.FilePath, want)
			}
			missing[e.Type]++
		}
	}
	for et, count := range missing {
		t.Errorf("%d %s edges do not name the file of their site", count, et)
	}
}
