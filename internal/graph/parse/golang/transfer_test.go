package golang_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestControlTransferAndExtents checks the schema 1.45 facts on
// testdata/concurrency/transfer.go: where break, continue, fallthrough and
// return go (jumps_to), the clauses of a switch, defer statements and
// function literals as nodes with their extent, what a callee does with a
// literal it is passed, a cancel function followed through a field, and
// channel names that name the line.
func TestControlTransferAndExtents(t *testing.T) {
	g, err := gop.LoadAndResolveWith("testdata/concurrency", gop.LoadOptions{LockPropagation: true})
	if err != nil {
		t.Fatalf("LoadAndResolveWith: %v", err)
	}
	label := func(n types.Node) string {
		switch n.Type {
		case types.NodeFunction, types.NodeMethod, types.NodeField, types.NodeLocalVariable, types.NodeVariable:
			return strings.TrimPrefix(n.QualifiedName, "mutex_fixture.")
		}
		return fmt.Sprintf("%s:%d", n.Type, n.StartLine)
	}
	byID := map[string]types.Node{}
	got := map[string]bool{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
		if strings.HasSuffix(n.FilePath, "transfer.go") {
			got[fmt.Sprintf("node %s %s %d-%d", label(n), n.SubKind, n.StartLine, n.EndLine)] = true
			if n.Type == types.NodeChannel {
				got["channel "+strings.TrimPrefix(n.QualifiedName, "chan@")] = true
			}
		}
	}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		if !strings.HasSuffix(src.FilePath, "transfer.go") {
			continue
		}
		got[fmt.Sprintf("%s %s %s %s %s", label(src), e.Type, label(dst), e.DispatchKind, e.Confidence)] = true
		if e.Type == types.EdgeFlowsTo && e.DispatchKind == "sync" {
			got[fmt.Sprintf("sync store %s in %s", label(dst), filepath.Base(e.FilePath))] = true
		}
	}
	for _, e := range g.Edges {
		if src, dst := byID[e.Src], byID[e.Dst]; e.Type == types.EdgeFlowsTo && e.DispatchKind == "sync" && strings.HasSuffix(src.FilePath, "trigger_names.go") {
			got[fmt.Sprintf("sync store %s in %s", label(dst), filepath.Base(e.FilePath))] = true
		}
	}
	const file = "transfer.go"
	tests := []struct {
		name string
		fact string
	}{
		{"continue goes on with its loop", "BranchStmt:14 jumps_to LoopStmt:12 continue EXTRACTED"},
		{"a labeled break leaves the labeled loop", "BranchStmt:26 jumps_to LoopStmt:23 break EXTRACTED"},
		{"fallthrough goes into the next clause", "BranchStmt:28 jumps_to CaseClause:29 fallthrough EXTRACTED"},
		{"continue in a default clause", "BranchStmt:32 jumps_to LoopStmt:23 continue EXTRACTED"},
		{"a return in a clause returns from its function", "ReturnStmt:30 jumps_to XferPick return EXTRACTED"},
		{"a return in a literal returns from the literal", "ReturnStmt:57 jumps_to FuncLit:57 return EXTRACTED"},
		{"a break is a node", "node BranchStmt:26 break 26-26"},
		{"a clause spans its statements", "node CaseClause:27 case 27-28"},
		{"the default clause", "node CaseClause:31 default 31-32"},
		{"the switch encloses its clause", "SwitchStmt:24 encloses CaseClause:31 clause EXTRACTED"},
		{"a defer statement is a node", "node DeferStmt:52  52-52"},
		{"a literal is a node", "node FuncLit:53  53-53"},
		{"a literal the callee calls", "FuncLit:53 literal_passed_to xferRun call EXTRACTED"},
		{"a literal the callee starts in a goroutine", "FuncLit:54 literal_passed_to xferGo go EXTRACTED"},
		{"a literal the callee stores", "FuncLit:55 literal_passed_to xferKeep store EXTRACTED"},
		{"a literal the callee calls on one branch", "FuncLit:56 literal_passed_to xferMaybe call INFERRED"},
		{"a cancel stored into a field", "newXferSrv.cancel flows_to xferSrv.cancel sync INFERRED"},
		{"a context stored by a composite literal", "newXferSrv.ctx flows_to xferSrv.ctx sync INFERRED"},
		{"channels on one line are named by line and column", "channel " + file},
		{"a literal the callee runs inside a goroutine of its own", "FuncLit:95 literal_passed_to xferSpawn go EXTRACTED"},
		{"a literal the callee sends to another goroutine", "FuncLit:96 literal_passed_to xferQueue send INFERRED"},
		{"a literal passed through a variable a nested literal captures", "FuncLit:97 literal_passed_to xferRun call EXTRACTED"},
		{"a store names the file it is written in, not the field's", "sync store xferHolder.c in trigger_names.go"},
		{"a literal passed to a method of a generic type", "FuncLit:109 literal_passed_to xferBox.each call EXTRACTED"},
	}
	for _, tc := range tests {
		found := got[tc.fact]
		if strings.HasPrefix(tc.fact, "channel ") {
			found = false
			for k := range got {
				if strings.HasPrefix(k, "channel ") && strings.HasSuffix(k, file+":78:48") {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: %q missing", tc.name, tc.fact)
		}
	}
	if t.Failed() {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			t.Log(k)
		}
	}
}
