package verify

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// compareStatements sorts each fact into matched, missing, no statement
// node, endpoint missing, unexpected or given to an outer statement.
func TestCompareStatements_Classes(t *testing.T) {
	n := func(id string, typ types.NodeType, line int, cid string) types.Node {
		return types.Node{ID: id, Type: typ, FilePath: "a.go", StartLine: line, CanonicalID: cid, Language: "go"}
	}
	nodes := []types.Node{
		n("ifL2", types.NodeIfStmt, 2, ""), n("csL3", types.NodeCallSite, 3, ""),
		n("goL8", types.NodeGoroutine, 8, ""), n("csL10", types.NodeCallSite, 10, ""),
		n("fx", types.NodeFunction, 20, "p.X"), n("vv", types.NodeField, 30, "p.T.V"),
	}
	e := func(typ types.EdgeType, src, dst, kind string, line int) types.Edge {
		return types.Edge{Type: typ, Src: src, Dst: dst, DispatchKind: kind, Line: line, Count: 1}
	}
	g := &fakeGraph{nodes: nodes, edges: []types.Edge{
		e(types.EdgeSiteCalls, "csL3", "fx", "static", 3), // matched
		e(types.EdgeBodyWrites, "ifL2", "vv", "else", 4),  // wrong part: unexpected, and the truth one missing
		e(types.EdgeEncloses, "goL8", "csL10", "go", 10),  // inner IfStmt@L9 has no node: outer statement
	}}
	f := func(rel types.EdgeType, src, dst, kind string, line int) StmtFact {
		return StmtFact{Rel: string(rel), File: "a.go", Src: src, Dst: dst, Kind: kind, Line: line}
	}
	truth := map[StmtFact]bool{
		f(types.EdgeSiteCalls, "CallSite@L3", "p.X", "static", 3):       true,
		f(types.EdgeBodyWrites, "IfStmt@L2", "p.T.V", "then", 4):        true,
		f(types.EdgeEncloses, "IfStmt@L9", "CallSite@L10", "then", 10):  true,
		f(types.EdgeSiteCalls, "CallSite@L3", "q.Outside", "static", 3): true, // no node for the callee
	}
	got, err := compareStatements(truth, nodes, g, 5)
	if err != nil {
		t.Fatal(err)
	}
	sc, bw, en := got["site_calls"], got["body_writes"], got["encloses"]
	if sc.Matched != 1 || sc.EndpointMissing != 1 || sc.Unexpected != 0 {
		t.Errorf("site_calls %+v", sc)
	}
	if bw.Missing != 1 || bw.Unexpected != 1 || bw.Matched != 0 {
		t.Errorf("body_writes %+v", bw)
	}
	if en.NoStatementNode != 1 || en.OuterStatement != 1 || en.Unexpected != 0 {
		t.Errorf("encloses %+v", en)
	}
}
