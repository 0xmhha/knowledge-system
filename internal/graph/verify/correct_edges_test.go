package verify

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestRunGo_CorrectEdgesAreNotDefects covers two cases the report used to
// get wrong, on testdata/fix2:
//
//   - Both refers to a/x.K and b/x.K. The graph has only the edge to b/x.K,
//     which is itself a truth pair, so the absent a/x.K edge is missing, not
//     misresolved to b/x.K.
//   - Pull calls Read through the embedded interface field of Conn. The
//     graph's invokes edge to Reader.Read is correct: it must be matched as an
//     interface call, not counted as a dynamic call site and an unexpected edge.
//   - add has no Go body (assembly). Its signature refers to Word, so the
//     graph's uses_type edge is supported, not unexpected.
func TestRunGo_CorrectEdgesAreNotDefects(t *testing.T) {
	const root = "testdata/fix2"
	node := func(id string, nt types.NodeType, name, cid, file string, start, end int) types.Node {
		return types.Node{ID: id, Type: nt, Name: name, CanonicalID: cid, FilePath: file,
			StartLine: start, EndLine: end, Language: "go"}
	}
	g := &fakeGraph{
		nodes: []types.Node{
			node("aK", types.NodeStruct, "K", "example.com/fix2/a/x.K", "a/x/x.go", 5, 5),
			node("bK", types.NodeStruct, "K", "example.com/fix2/b/x.K", "b/x/x.go", 5, 5),
			node("Both", types.NodeFunction, "Both", "example.com/fix2/c.Both", "c/c.go", 10, 10),
			node("Reader", types.NodeInterface, "Reader", "example.com/fix2/c.Reader", "c/c.go", 13, 15),
			node("Read", types.NodeMethod, "Read", "example.com/fix2/c.Reader.Read", "c/c.go", 14, 14),
			node("Conn", types.NodeStruct, "Conn", "example.com/fix2/c.Conn", "c/c.go", 18, 20),
			node("Pull", types.NodeFunction, "Pull", "example.com/fix2/c.Pull", "c/c.go", 23, 25),
			node("Word", types.NodeTypeAlias, "Word", "example.com/fix2/c.Word", "c/asm.go", 4, 4),
			node("add", types.NodeFunction, "add", "example.com/fix2/c.add", "c/asm.go", 7, 7),
		},
		edges: []types.Edge{
			{Src: "Both", Dst: "bK", Type: types.EdgeUsesType},
			{Src: "Pull", Dst: "Read", Type: types.EdgeInvokes},
			{Src: "Pull", Dst: "Conn", Type: types.EdgeUsesType},
			{Src: "Conn", Dst: "Reader", Type: types.EdgeUsesType},
			{Src: "add", Dst: "Word", Type: types.EdgeUsesType},
		},
	}
	r, err := RunGo(root, g, Options{})
	if err != nil {
		t.Fatalf("RunGo: %v", err)
	}
	cases := []struct {
		name      string
		got, want int
	}{
		{"declarations missing", r.Decls.Missing, 0},
		{"type refs truth", r.TypeRefs.Truth, 5}, // Both->a/x.K, Both->b/x.K, Pull->Conn, Conn->Reader, add->Word
		{"type refs matched", r.TypeRefs.Matched, 4},
		{"type refs missing", r.TypeRefs.Missing, 1},
		{"type refs misresolved", r.TypeRefs.Misresolved, 0},
		{"interface calls truth", r.InterfaceCalls.Truth, 1},
		{"interface calls matched", r.InterfaceCalls.Matched, 1},
		{"dynamic call sites", r.DynamicCallSites, 0},
		{"unexpected call edges", r.Unexpected.Calls, 0},
		{"unexpected type edges", r.Unexpected.TypeRefs, 0},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}
