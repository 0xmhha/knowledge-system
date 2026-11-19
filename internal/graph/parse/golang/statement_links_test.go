package golang_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestStatementLinks checks the statement edges of testdata/resolve/coll1/stmts.go.
// Statements are named "<Type>@L<line>" in the function that holds them.
func TestStatementLinks(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/resolve")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	name := map[string]string{}
	for _, n := range g.Nodes {
		switch n.Type {
		case types.NodeIfStmt, types.NodeLoopStmt, types.NodeSwitchStmt, types.NodeReturnStmt, types.NodeCallSite, types.NodeGoroutine:
			if strings.HasSuffix(n.FilePath, "coll1/stmts.go") {
				name[n.ID] = fmt.Sprintf("%s@L%d", n.Type, n.StartLine)
			}
		default:
			name[n.ID] = n.QualifiedName
		}
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		switch e.Type {
		case types.EdgeSiteCalls, types.EdgeCondReads, types.EdgeCondCalls, types.EdgeBodyWrites, types.EdgeEncloses:
		default:
			continue
		}
		src, dst := name[e.Src], name[e.Dst]
		if src == "" || dst == "" || len(src) < 3 {
			continue
		}
		got[fmt.Sprintf("%s %s %s %s L%d", src, e.Type, dst, e.DispatchKind, e.Line)] = true
	}
	want := []string{
		// if e.root != root { e.root = ...; e.state = load(root) } else { e.ready = true }
		"IfStmt@L19 cond_reads coll1.env.root  L19",
		"IfStmt@L19 body_writes coll1.env.root then L20",
		"IfStmt@L19 body_writes coll1.env.state then L21",
		"IfStmt@L19 body_writes coll1.env.ready else L23",
		"IfStmt@L19 encloses CallSite@L21 then L21",
		"CallSite@L21 site_calls coll1.load static L21",
		// for e.fresh() { switch e.state { case 1: e.count = 0; default: return } }
		"LoopStmt@L30 cond_calls coll1.env.fresh  L30",
		"LoopStmt@L30 encloses CallSite@L30 cond L30",
		"LoopStmt@L30 encloses SwitchStmt@L31 body L31",
		"SwitchStmt@L31 cond_reads coll1.env.state  L31",
		"SwitchStmt@L31 body_writes coll1.env.count case L33",
		"SwitchStmt@L31 encloses ReturnStmt@L35 case L35",
		// go func() { e.state = load(e.count) }()
		"Goroutine@L38 body_writes coll1.env.state go L39",
		"Goroutine@L38 encloses CallSite@L39 go L39",
		"CallSite@L39 site_calls coll1.load static L39",
		// go e.refresh(e.root)
		"Goroutine@L41 site_calls coll1.env.refresh static L41",
		// if e.state, e.count = load(1), 2; e.ready { e.root = 0 }
		"IfStmt@L46 cond_reads coll1.env.ready  L46",
		"IfStmt@L46 body_writes coll1.env.root then L47",
		// x.Tip() on a struct that embeds Tipper
		"CallSite@L54 site_calls coll1.Tipper.Tip interface_method L54",
		// go func() { if e.ready { e.count = load(2) } }()
		"Goroutine@L59 encloses IfStmt@L60 go L60",
		"IfStmt@L60 cond_reads coll1.env.ready  L60",
		"IfStmt@L60 body_writes coll1.env.count then L61",
		"IfStmt@L60 encloses CallSite@L61 then L61",
		// go e.refresh(load(3)): the argument is a call site of spin
		"CallSite@L64 site_calls coll1.load static L64",
		"Goroutine@L64 site_calls coll1.env.refresh static L64",
	}
	for _, w := range want {
		if !got[w] {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("missing %q; have:\n%v", w, have)
			break
		}
	}
	for _, bad := range []string{"IfStmt@L46 cond_reads coll1.env.state  L46", "IfStmt@L46 cond_reads coll1.env.count  L46"} {
		if got[bad] {
			t.Errorf("a write in a condition recorded as a read: %q", bad)
		}
	}
	// e.count++ in refresh is outside every condition: no statement writes it there.
	for k := range got {
		if k == "IfStmt@L19 body_writes coll1.env.count then L25" || k == "IfStmt@L19 body_writes coll1.env.count else L25" {
			t.Errorf("unexpected %q", k)
		}
	}
}

// TestStatementLinks_InitPerFile: statements and value uses of an init
// function belong to the init of their own file (init_a.go, init_b.go).
func TestStatementLinks_InitPerFile(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/resolve")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	file := map[string]string{}
	stmtLine := map[string]string{}
	for _, n := range g.Nodes {
		file[n.ID] = n.FilePath
		stmtLine[n.ID] = fmt.Sprintf("%s %s@L%d", filepathBase(n.FilePath), n.Type, n.StartLine)
		if n.Type == types.NodeVariable || n.Type == types.NodeField {
			stmtLine[n.ID] = n.QualifiedName
		}
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		switch e.Type {
		case types.EdgeCondReads, types.EdgeBodyWrites:
			got[fmt.Sprintf("%s %s %s", stmtLine[e.Src], e.Type, stmtLine[e.Dst])] = true
		case types.EdgeReferences:
			if strings.HasSuffix(stmtLine[e.Dst], ".initA") || strings.HasSuffix(stmtLine[e.Dst], ".initB") {
				got[fmt.Sprintf("%s references %s %s", filepathBase(file[e.Src]), stmtLine[e.Dst], e.DispatchKind)] = true
			}
		}
	}
	for _, w := range []string{
		"init_a.go IfStmt@L9 cond_reads coll1.initA",
		"init_a.go IfStmt@L9 body_writes coll1.initA",
		"init_b.go LoopStmt@L6 cond_reads coll1.initB",
		"init_b.go LoopStmt@L6 body_writes coll1.initB",
		"init_a.go references coll1.initA write",
		"init_b.go references coll1.initB write",
	} {
		if !got[w] {
			t.Errorf("missing %q; have %v", w, got)
		}
	}
	for k := range got {
		if strings.HasPrefix(k, "init_a.go references coll1.initB") || strings.HasPrefix(k, "init_b.go references coll1.initA") {
			t.Errorf("value use attributed to the other file's init: %q", k)
		}
	}
}

func filepathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
