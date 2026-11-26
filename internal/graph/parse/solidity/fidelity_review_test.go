package solidity_test

import (
	"fmt"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Fixtures of the second review round, under testdata/fidelity/review.
const (
	fileOverloads = "review/Overloads.sol"
	filePointers  = "review/Pointers.sol"
	fileUses      = "review/Uses.sol"
	fileThings    = "review/Things.sol"
	fileTopLevel  = "review/TopLevel.sol"
	fileLowLevel  = "review/LowLevel.sol"
)

// siteEdges lists every edge of type et that starts at src as
// "<line> <destination qualified name>@<destination start line>", so a test
// can tell overloads (same qualified name) apart.
func (g *fidelityGraph) siteEdges(et types.EdgeType, src endpoint) []string {
	var out []string
	for _, e := range g.edges {
		if e.Type != et {
			continue
		}
		s := g.byID[e.Src]
		if s.FilePath != src.file || s.QualifiedName != src.qname {
			continue
		}
		d := g.byID[e.Dst]
		out = append(out, fmt.Sprintf("%d %s@%d", e.Line, d.QualifiedName, d.StartLine))
	}
	return out
}

// TestReviewOverloads covers overload choice: a call picks the overload
// whose parameter count fits its arguments, whether the call is by name, on a
// library by name, or through a using-for receiver (which is the first
// parameter of the library function).
func TestReviewOverloads(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	assertSameSet(t, g.siteEdges(types.EdgeCalls, endpoint{fileOverloads, "UsesL.run"}), []string{
		"47 L.f@10",
		"48 L.f@14",
		"49 L.inc@18",
		"50 L.inc@22",
		"51 L.peek@26",
		"52 UsesL.g@38",
		"52 UsesL.g@42",
	})
}

// TestReviewUsingForReceiverWrites covers a library call that changes its
// storage receiver: `y.inc()` writes y because inc takes it as a storage
// parameter and is not view or pure; `y.peek()` only reads it.
func TestReviewUsingForReceiverWrites(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	run := endpoint{fileOverloads, "UsesL.run"}
	assertSameSet(t, g.siteEdges(types.EdgeWritesField, run), []string{"49 UsesL.y@36", "50 UsesL.y@36"})
	assertSameSet(t, g.siteEdges(types.EdgeReadsField, run), []string{"49 UsesL.y@36", "50 UsesL.y@36", "51 UsesL.y@36"})
}

// TestReviewInitialiser covers a state variable's initialiser: the variable
// is the reader of what its initialiser names.
func TestReviewInitialiser(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	assertSameSet(t, g.siteEdges(types.EdgeReadsField, endpoint{fileOverloads, "UsesL.total"}), []string{"35 UsesL.START@34"})
}

// TestReviewStoragePointers covers writes through a local declared
// `storage`: the write is a write of the state variable or mapping the
// pointer was bound to, directly, through another pointer, or as the storage
// return of a function of the same file. Binding the pointer again is not a
// write, and a pointer that is a parameter cannot be followed.
func TestReviewStoragePointers(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name   string
		fn     string
		writes []string
		reads  []string
	}{
		{"pointer bound to a mapping entry", "Pointers.vote",
			[]string{"17 Pointers.proposals@11", "18 Pointers.proposals@11", "20 Pointers.proposals@11", "21 Pointers.proposals@11"},
			[]string{"16 Pointers.proposals@11"}},
		{"pointer returned by a function, named and unnamed", "Pointers.grow",
			[]string{"37 Pointers.lists@12", "39 Pointers.lists@12"}, nil},
		{"binding again is not a write", "Pointers.rebind",
			[]string{"46 Pointers.proposals@11"},
			[]string{"44 Pointers.proposals@11", "45 Pointers.proposals@11"}},
		{"the function that returns the pointer only reads", "Pointers._next",
			nil, []string{"26 Pointers.lists@12", "27 Pointers.lists@12"}},
		{"a storage parameter cannot be followed", "Pointers.through", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := endpoint{filePointers, tc.fn}
			assertSameSet(t, g.siteEdges(types.EdgeWritesMapping, src), tc.writes)
			assertSameSet(t, g.siteEdges(types.EdgeReadsMapping, src), tc.reads)
		})
	}
}

// TestReviewUseSites covers the uses that only `defines` reached: an enum
// value (`Phase.Open`), a struct member (`item.owner`, read or written,
// through a local, a mapping entry, an array element or another member), a
// constant of a library (`Shapes.LIMIT`), and a struct, enum, contract or
// interface named as a type.
func TestReviewUseSites(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	open := endpoint{fileUses, "Uses.open"}
	cases := []struct {
		name string
		et   types.EdgeType
		src  endpoint
		want []string
	}{
		{"reads in a function", types.EdgeReadsField, open, []string{
			"41 Uses.Phase.Open@25",
			"44 Shapes.LIMIT@19",
			"45 Uses.Item.size@30", "45 Shapes.Box.width@15", "45 Uses.list@37", "45 Uses.top@38",
			"46 Uses.Item.box@31", "46 Shapes.Box.kind@16", "46 Shapes.Kind.Circle@11",
			"49 Shapes.Kind.Square@10",
			"51 Uses.phase@34",
		}},
		{"writes in a function", types.EdgeWritesField, open, []string{
			"41 Uses.phase@34",
			"43 Uses.Item.owner@29",
			"44 Uses.Item.size@30",
			"45 Uses.Item.box@31", "45 Shapes.Box.width@15",
			"47 Uses.registry@35",
			"50 Uses.list@37",
		}},
		{"types named in a function", types.EdgeUsesType, open, []string{
			"40 Uses.Phase@23", "42 Uses.Item@28", "47 IRegistry@4", "49 Uses.Item@28", "49 Shapes.Box@14",
		}},
		{"type of an enum state variable", types.EdgeUsesType, endpoint{fileUses, "Uses.phase"}, []string{"34 Uses.Phase@23"}},
		{"type of an interface state variable", types.EdgeUsesType, endpoint{fileUses, "Uses.registry"}, []string{"35 IRegistry@4"}},
		{"value type of a mapping", types.EdgeUsesType, endpoint{fileUses, "Uses.items"}, []string{"36 Uses.Item@28"}},
		{"element type of an array", types.EdgeUsesType, endpoint{fileUses, "Uses.list"}, []string{"37 Uses.Item@28"}},
		{"qualified struct type", types.EdgeUsesType, endpoint{fileUses, "Uses.top"}, []string{"38 Shapes.Box@14"}},
		{"type of a struct member", types.EdgeUsesType, endpoint{fileUses, "Uses.Item.box"}, []string{"31 Shapes.Box@14"}},
		{"enum type of a struct member", types.EdgeUsesType, endpoint{fileUses, "Shapes.Box.kind"}, []string{"16 Shapes.Kind@9"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.siteEdges(tc.et, tc.src), tc.want)
		})
	}
	t.Run("the type named twice on a line counts twice", func(t *testing.T) {
		uses := 0
		for _, e := range g.edgesBetween(types.EdgeUsesType, open, endpoint{fileUses, "Uses.Item"}) {
			if e.Line == 49 {
				uses += e.Count
			}
		}
		if uses != 2 {
			t.Errorf("Item is named %d times on line 49, want 2 (the local's type and the constructor)", uses)
		}
	})
}

// TestReviewFileLevelDeclarations covers declarations made outside any
// contract: each has a node defined by the file and resolves from a function
// or a contract of the file.
func TestReviewFileLevelDeclarations(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	nodes := []struct {
		name    string
		nt      types.NodeType
		qname   string
		subKind string
		start   int
		end     int
	}{
		{"constant", types.NodeConstant, "TOP_C", "constant", 7, 7},
		{"struct", types.NodeStruct, "TopS", "", 9, 11},
		{"struct member", types.NodeField, "TopS.a", "struct_member", 10, 10},
		{"enum", types.NodeEnum, "TopE", "", 13, 16},
		{"enum value", types.NodeConstant, "TopE.B", "enum_value", 15, 15},
		{"error", types.NodeError, "TopErr", "", 18, 18},
		{"event", types.NodeEvent, "TopEv", "", 20, 20},
		{"function", types.NodeFunction, "topFn", "function", 22, 25},
	}
	file := endpoint{fileTopLevel, "file:" + fileTopLevel}
	for _, tc := range nodes {
		t.Run(tc.name, func(t *testing.T) {
			n := g.one(t, fileTopLevel, tc.nt, tc.qname)
			if n.SubKind != tc.subKind || n.StartLine != tc.start || n.EndLine != tc.end {
				t.Errorf("sub kind %q lines %d-%d, want %q %d-%d", n.SubKind, n.StartLine, n.EndLine, tc.subKind, tc.start, tc.end)
			}
			if tc.subKind == "struct_member" || tc.subKind == "enum_value" {
				return
			}
			if got := g.edgesBetween(types.EdgeDefines, file, endpoint{fileTopLevel, tc.qname}); len(got) != 1 {
				t.Errorf("the file defines %s %d times, want 1", tc.qname, len(got))
			}
		})
	}
	edges := []struct {
		name string
		et   types.EdgeType
		src  string
		want []string
	}{
		{"free function reads the constant and the struct member", types.EdgeReadsField, "topFn",
			[]string{"23 TopS.a@10", "23 TOP_C@7", "24 TopS.a@10", "24 TOP_C@7"}},
		{"free function reverts with the file-level error", types.EdgeRevertsWith, "topFn", []string{"23 TopErr@18"}},
		{"free function names the struct", types.EdgeUsesType, "topFn", []string{"22 TopS@9"}},
		{"contract calls the free function", types.EdgeCalls, "C.bump", []string{"33 topFn@22"}},
		{"contract reads the enum value", types.EdgeReadsField, "C.bump",
			[]string{"33 C.top@28", "34 C.kind@29", "34 TopE.B@15", "35 Thing.count@5", "36 Thing.count@5"}},
		{"contract emits the file-level event", types.EdgeEmitsEvent, "C.bump", []string{"35 Thing.Bumped@7", "36 TopEv@20"}},
		{"state variable names the file-level struct", types.EdgeUsesType, "C.top", []string{"28 TopS@9"}},
		{"state variable names the file-level enum", types.EdgeUsesType, "C.kind", []string{"29 TopE@13"}},
	}
	for _, tc := range edges {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.siteEdges(tc.et, endpoint{fileTopLevel, tc.src}), tc.want)
		})
	}
}

// TestReviewImportAlias covers `import { Thing as T2 }`: the alias is the
// imported contract, as a base and for everything inherited from it. A
// whole-file alias (`import "./Things.sol" as Space`) qualifies a type.
func TestReviewImportAlias(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	bump := endpoint{fileTopLevel, "C.bump"}
	cases := []struct {
		name string
		et   types.EdgeType
		src  endpoint
		want []string
	}{
		{"base named by its alias", types.EdgeExtends, endpoint{fileTopLevel, "C"}, []string{"27 Thing@4"}},
		{"inherited modifier", types.EdgeHasModifier, bump, []string{"32 Thing.limited@11"}},
		{"inherited state variable", types.EdgeWritesField, bump, []string{"33 Thing.count@5"}},
		{"inherited error", types.EdgeRevertsWith, bump, []string{"34 Thing.TooMany@9"}},
		{"override of the aliased base", types.EdgeOverrides, bump, []string{"32 Thing.bump@16"}},
		{"type qualified by a file alias", types.EdgeUsesType, endpoint{fileTopLevel, "C.other"}, []string{"30 Thing@4"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameSet(t, g.siteEdges(tc.et, tc.src), tc.want)
		})
	}
}

// TestReviewSlotZero covers slot 0: a state variable in slot 0 says it has a
// storage slot, and a node without one (a constant, an immutable, a struct
// member, a function) does not.
func TestReviewSlotZero(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		name    string
		nt      types.NodeType
		qname   string
		hasSlot bool
		slot    int
	}{
		{"state variable in slot 0", types.NodeField, "Base.paused", true, 0},
		{"state variable in slot 1", types.NodeField, "Base.memberCount", true, 1},
		{"mapping", types.NodeMapping, "Base.members", true, 2},
		{"constant", types.NodeConstant, "Base.MAX_MEMBERS", false, 0},
		{"immutable", types.NodeField, "Base.deployedAt", false, 0},
		{"struct member", types.NodeField, "Base.Proposal.id", false, 0},
		{"function", types.NodeFunction, "Base.pause", false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := g.one(t, fileBase, tc.nt, tc.qname)
			if n.HasStorageSlot != tc.hasSlot || n.SlotIndex != tc.slot {
				t.Errorf("has slot %v index %d, want %v %d", n.HasStorageSlot, n.SlotIndex, tc.hasSlot, tc.slot)
			}
		})
	}
}

// TestReviewLowLevelCalls covers what a low-level call names in the source:
// `abi.encodeCall(I.fn, ...)` and `I.fn.selector` are references to that
// function; a call with opaque data names nothing and only sets the marker.
// A call that sends value (`.call{value: v}("")`) is a value transfer.
func TestReviewLowLevelCalls(t *testing.T) {
	g := loadFidelity(t, fidelityDir)
	cases := []struct {
		fn            string
		references    []string
		lowLevel      bool
		valueTransfer bool
	}{
		{"LowLevel.send", []string{"14 ITarget.poke@5"}, true, false},
		{"LowLevel.look", []string{"19 ITarget.peek@7"}, true, false},
		{"LowLevel.refund", nil, true, true},
		{"LowLevel.opaque", nil, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			assertSameSet(t, g.siteEdges(types.EdgeReferences, endpoint{fileLowLevel, tc.fn}), tc.references)
			n := g.one(t, fileLowLevel, types.NodeFunction, tc.fn)
			if n.HasLowLevelCall != tc.lowLevel || n.HasValueTransfer != tc.valueTransfer {
				t.Errorf("low-level %v value transfer %v, want %v %v", n.HasLowLevelCall, n.HasValueTransfer, tc.lowLevel, tc.valueTransfer)
			}
		})
	}
}
