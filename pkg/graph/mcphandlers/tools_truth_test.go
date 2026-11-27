package mcphandlers

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// rowNames returns the "qname" of every row under key, in response order.
func rowNames(out map[string]any, key string) []string {
	names := []string{}
	for _, r := range asSlice(out[key]) {
		if q, ok := r.(map[string]any)["qname"].(string); ok {
			names = append(names, q)
		}
	}
	return names
}

// sorted returns a sorted copy of names.
func sorted(names []string) []string {
	out := append([]string{}, names...)
	sort.Strings(out)
	return out
}

// hasAll reports the names of want that got lacks.
func missingNames(got []string, want ...string) []string {
	have := map[string]bool{}
	for _, g := range got {
		have[g] = true
	}
	var miss []string
	for _, w := range want {
		if !have[w] {
			miss = append(miss, w)
		}
	}
	return miss
}

// lineOf returns the row of a hit's matched line.
func lineOf(hit map[string]any, line int) map[string]any {
	for _, l := range asSlice(hit["lines"]) {
		if m := l.(map[string]any); int(m["line"].(float64)) == line {
			return m
		}
	}
	return nil
}

func stringsOf(v any) []string {
	out := []string{}
	for _, x := range asSlice(v) {
		out = append(out, x.(string))
	}
	return out
}

// TestSearchCode_PromotedMethodIsOneHit: a method that two types get through
// an embedded field is one declaration in the source, so it is one hit, under
// its own name, with its uses and callers; the embedding types are an
// attribute. On go-stablenet 546 of 1,505 such hits came only under an
// embedding type's name (state.StateDB.Prepare as logger.dummyStatedb.Prepare)
// with no uses, and 617 came twice with in=name or both.
func TestSearchCode_PromotedMethodIsOneHit(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"body", map[string]any{"pattern": "rules", "expand": []any{"callers"}}},
		{"name", map[string]any{"pattern": "Prepare", "in": "name", "expand": []any{"callers"}}},
		{"both", map[string]any{"pattern": "Prepare", "in": "both", "expand": []any{"callers"}}},
		{"embedding type name", map[string]any{"pattern": "dummyStatedb", "in": "both"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := callTool(t, s, "search_code", tc.args)
			names := hitNames(out)
			for _, n := range names {
				if strings.HasSuffix(n, "dummyStatedb.Prepare") {
					t.Errorf("hit under an embedding type's name: %v", names)
				}
			}
			if tc.name == "embedding type name" {
				return
			}
			if !reflect.DeepEqual(names, []string{"state.StateDB.Prepare"}) || out["total_declarations"].(float64) != 1 {
				t.Fatalf("hits = %v (total %v), want state.StateDB.Prepare once", names, out["total_declarations"])
			}
			h := hitByName(out, "state.StateDB.Prepare")
			if got := stringsOf(h["promoted_to"]); !reflect.DeepEqual(got, []string{"js.dummyStatedb", "logger.dummyStatedb"}) {
				t.Errorf("promoted_to = %v", got)
			}
			if got := rowNames(h, "callers"); !reflect.DeepEqual(got, []string{"logger.TestPrepare"}) {
				t.Errorf("callers = %v, want logger.TestPrepare", got)
			}
			if tc.name != "body" {
				return
			}
			if l := lineOf(h, 11); l == nil || !reflect.DeepEqual(stringsOf(l["uses"]), []string{"writes state.StateDB.logs"}) {
				t.Errorf("line 11 = %v, want the write of state.StateDB.logs", l)
			}
		})
	}
	// Asked for by an embedding type's name, the method is still that one hit.
	byCopy := callTool(t, s, "search_code", map[string]any{"pattern": "rules", "ids": []any{"logger.dummyStatedb.Prepare"}})
	if !reflect.DeepEqual(hitNames(byCopy), []string{"state.StateDB.Prepare"}) || byCopy["ids_not_matched"] != nil {
		t.Errorf("ids with a promoted name: hits %v, not matched %v", hitNames(byCopy), byCopy["ids_not_matched"])
	}
}

// TestSearchCode_LineText: a matched line is the line of the file. Text that
// follows a nested declaration on its line (the comment after an interface
// method or a Solidity enum value) is searched and reported under that
// declaration (16 such lines were reported nowhere), trailing whitespace is
// kept (6 lines differed), the indentation in front of a nested declaration
// is part of its line, and a call is listed on the line that names its
// callee when the call started on an earlier line (8 calls were missing).
func TestSearchCode_LineText(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		pattern, hit string
		line         int
		text         string
		uses         []string
	}{
		{"sending", "event.Subscription.Unsubscribe", 3, "\tUnsubscribe()      // cancels sending of events", nil},
		{"error channel", "event.Subscription.Err", 2, "\tErr() <-chan error // returns the error channel", nil},
		{"still open", "Gov.Status.Voting", 10, "        Voting, // still open for votes", nil},
		{`Reset\(rules`, "state.StateDB.Prepare", 13, "\t\tReset(rules)  ", []string{"calls state.StateDB.Reset"}},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			out := callTool(t, s, "search_code", map[string]any{"pattern": tc.pattern})
			h := hitByName(out, tc.hit)
			if h == nil {
				t.Fatalf("no hit %s in %v", tc.hit, hitNames(out))
			}
			l := lineOf(h, tc.line)
			if l == nil || l["text"] != tc.text {
				t.Fatalf("line %d = %q, want %q", tc.line, l, tc.text)
			}
			if tc.uses != nil && !reflect.DeepEqual(stringsOf(l["uses"]), tc.uses) {
				t.Errorf("uses on line %d = %v, want %v", tc.line, l["uses"], tc.uses)
			}
		})
	}
	// in=name matches the declaration's own first line, not what follows it.
	if out := callTool(t, s, "search_code", map[string]any{"pattern": "sending", "in": "name"}); len(hitNames(out)) != 0 {
		t.Errorf("in=name matched the comment after a declaration: %v", hitNames(out))
	}
}

// TestSearchCode_NameIsNoComment: in=name (and name_match of in=both)
// matches the qualified name and the declaration's first line without its
// comments; the body search still finds the comments. Since a declaration's
// text ran through the comment after it, `timeout` returned
// ethapi.Backend.RPCEVMTimeout from its comment on go-stablenet.
func TestSearchCode_NameIsNoComment(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		pattern, in string
		want        []string
	}{
		{"header fetches", "name", nil},
		{"holds", "name", nil},
		{"two slashes|c \\*/", "name", nil},
		{"maxSkeleton = 128$", "name", []string{"types.maxSkeleton"}},
		{`^type Holder struct \{$`, "name", []string{"types.Holder"}},
		{`"a // b"$`, "name", []string{"types.slashes"}},
		{"header fetches", "body", []string{"types.maxSkeleton"}},
		{"holds the fetch", "body", []string{"types.Holder"}},
		{"two slashes", "body", []string{"types.slashes"}},
	} {
		t.Run(tc.in+" "+tc.pattern, func(t *testing.T) {
			out := callTool(t, s, "search_code", map[string]any{"pattern": tc.pattern, "in": tc.in})
			if got := hitNames(out); !reflect.DeepEqual(sorted(got), sorted(tc.want)) && (len(got) != 0 || len(tc.want) != 0) {
				t.Errorf("hits = %v, want %v", got, tc.want)
			}
		})
	}
	both := callTool(t, s, "search_code", map[string]any{"pattern": "header fetches", "in": "both"})
	if h := hitByName(both, "types.maxSkeleton"); h == nil || h["name_match"] != nil {
		t.Errorf("in=both: hit %v, want types.maxSkeleton without name_match", h)
	}
}

// TestSearchCode_NestedIndentation: the indentation in front of a nested
// declaration belongs to it, in the line's text as in what the pattern
// matched. On go-stablenet `.{500,}` matched "\t..." under a nested
// declaration whose line text began after the tab.
func TestSearchCode_NestedIndentation(t *testing.T) {
	s := toolStore(t)
	out := callTool(t, s, "search_code", map[string]any{"pattern": `^\tconst limit`})
	h := hitByName(out, "abi.TestPack.limit")
	if h == nil {
		t.Fatalf("no hit abi.TestPack.limit in %v", hitNames(out))
	}
	if got := stringsOf(h["matched"]); !reflect.DeepEqual(got, []string{"\tconst limit"}) {
		t.Errorf("matched = %q, want the tab and the keyword", got)
	}
	if l := lineOf(h, 26); l == nil || l["text"] != "\tconst limit = 2" {
		t.Errorf("line 26 = %q, want %q", l, "\tconst limit = 2")
	}
	// Text in front of a nested declaration that is not indentation stays
	// with the enclosing one: the method's line is its own part of the line.
	slot := callTool(t, s, "search_code", map[string]any{"pattern": `Bytes\(\)`})
	if l := lineOf(hitByName(slot, "systemcontracts.Slot.interface@30.Bytes"), 1); l == nil || l["text"] != "Bytes() []byte }) int {" {
		t.Errorf("line 1 of the inline interface method = %q", l)
	}
}

// TestSearchCode_GroupLines: the opening `var (` line and the closing `)`
// line of a group are text outside every declaration, and the guide says so
// of both (it named only the opening line).
func TestSearchCode_GroupLines(t *testing.T) {
	s := toolStore(t)
	if out := callTool(t, s, "search_code", map[string]any{"pattern": `^var \($|^\)$`}); len(hitNames(out)) != 0 {
		t.Errorf("group lines matched: %v", hitNames(out))
	}
	if out := callTool(t, s, "search_code", map[string]any{"pattern": `^grouped = 1$`}); !reflect.DeepEqual(hitNames(out), []string{"types.grouped"}) {
		t.Errorf("the spec in the group: hits %v, want types.grouped", hitNames(out))
	}
	for _, must := range []string{"the opening line of a `var (`, `const (` or `type (` group and its closing `)` line"} {
		if !strings.Contains(searchCodeTexts(t, s), must) {
			t.Errorf("search_code does not say %q", must)
		}
	}
}

// TestLocalOfInitialiser: a variable declared in a function literal of a
// package-level variable's initialiser is part of that variable, as a local
// variable is part of its function: search_code reports its line under the
// package-level variable, find_symbol and search_text count it as local
// (store.InFunctionBody). enode.parseNodeTests.r was a search hit of its own
// on go-stablenet.
func TestLocalOfInitialiser(t *testing.T) {
	s := toolStore(t)
	out := callTool(t, s, "search_test_code", map[string]any{"pattern": "var rawURL"})
	if got := hitNames(out); !reflect.DeepEqual(got, []string{"enode.parseNodeTests"}) {
		t.Fatalf("search_test_code hits = %v, want enode.parseNodeTests", got)
	}
	if l := lineOf(hitByName(out, "enode.parseNodeTests"), 32); l == nil || l["text"] != "\tvar rawURL = \"enode://x\"" {
		t.Errorf("line 32 = %v", l)
	}
	found := callTool(t, s, "find_symbol", map[string]any{"name": "rawURL", "exact": false})
	if got := rowNames(found, "nodes"); len(got) != 0 || !reflect.DeepEqual(found["left_out"], map[string]any{"local": 1.0}) {
		t.Errorf("find_symbol(rawURL): nodes %v left_out %v, want none and one local", got, found["left_out"])
	}
	text := callTool(t, s, "search_text", map[string]any{"query": "parseNodeTests", "top_k": 50})
	if names := rowNames(text, "nodes"); len(missingNames(names, "enode.parseNodeTests.rawURL")) == 0 {
		t.Errorf("search_text lists the local of the initialiser: %v", names)
	}
}

// TestSeed_NameDeclaredTwice: a qualified name that one function declares
// twice is ambiguous in every seed tool, with one candidate per declaration,
// and a candidate is a seed of its own. Seeds used to merge both silently.
func TestSeed_NameDeclaredTwice(t *testing.T) {
	s := toolStore(t)
	want := []string{"abi.TestPack.flag@" + fxNamesFile + ":21", "abi.TestPack.flag@" + fxNamesFile + ":25"}
	for _, tc := range []struct{ tool, arg string }{
		{"find_references", "qname"}, {"find_callers", "qname"}, {"get_subgraph", "seed_qname"},
		{"impact_of_change", "seed_qname"}, {"concurrency_impact", "seed_qname"}, {"change_history", "qname"},
		{"find_callees", "qname"},
	} {
		out := callTool(t, s, tc.tool, map[string]any{tc.arg: "abi.TestPack.flag"})
		if out["ambiguous"] != true || !reflect.DeepEqual(stringsOf(out["candidates"]), want) {
			t.Errorf("%s: ambiguous %v candidates %v, want %v", tc.tool, out["ambiguous"], out["candidates"], want)
		}
		if one := callTool(t, s, tc.tool, map[string]any{tc.arg: want[1]}); one["ambiguous"] == true || one["not_found"] == true {
			t.Errorf("%s(%s) did not resolve: %v", tc.tool, want[1], one)
		}
	}
	// find_symbol answers a site name with the one declaration it names, with
	// either exact setting; it found none of 145 on go-stablenet.
	for _, exact := range []bool{true, false} {
		out := callTool(t, s, "find_symbol", map[string]any{"name": want[1], "exact": exact})
		nodes := asSlice(out["nodes"])
		if len(nodes) != 1 || nodes[0].(map[string]any)["id"] != "nm00000000000016" {
			t.Errorf("find_symbol(%s, exact=%v) = %v, want the declaration at line 25", want[1], exact, out["nodes"])
		}
	}
	// investigate_open opens the card of the declaration a site name names.
	srv := newInvServer(t, s)
	callTextOn(t, srv, "investigate", map[string]any{"words": "TestPack"})
	if card := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:abi.TestPack@" + fxNamesFile + ":20"}); !strings.Contains(card, "function abi.TestPack") {
		t.Errorf("investigate_open with a site name: %s", card)
	}
	// A local has no card: the site name of one is answered with the card of
	// the function that holds it, after a line that says so (30 of 30 were
	// "unknown declaration"). The name of both is ambiguous, a wrong place
	// unknown.
	local := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:" + want[1]})
	if !strings.Contains(local, "local: "+want[1]+" is declared at "+fxNamesFile+":25 in abi.TestPack\n") || !strings.Contains(local, "function abi.TestPack") {
		t.Errorf("investigate_open with a local's site name:\n%s", local)
	}
	if both := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:abi.TestPack.flag"}); !strings.Contains(both, "names several declarations") {
		t.Errorf("investigate_open with an ambiguous local: %s", both)
	}
	if wrong := callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:abi.TestPack.flag@" + fxNamesFile + ":24"}); !strings.Contains(wrong, "unknown declaration") {
		t.Errorf("investigate_open with a wrong site name: %s", wrong)
	}
}

// TestSolidityRelations: the tools follow Solidity's relations as they follow
// Go's. On go-stablenet find_callers and find_callees returned none of 70
// modifier applications, 58 emits and 128 reverts, find_references nothing
// for 39 mapping state variables with 168 uses, and errors and state
// variables were never search hits of their own.
func TestSolidityRelations(t *testing.T) {
	s := toolStore(t)
	t.Run("search hits", func(t *testing.T) {
		for _, tc := range []struct {
			pattern string
			want    []string
			absent  string
		}{
			{"NotAMember", []string{"Gov.NotAMember", "Gov.onlyMember"}, ""},
			{"quorum", []string{"Gov.quorum", "Gov.vote"}, ""},
			// A struct member is searched as part of its struct, as a Go field is.
			{"isActive", []string{"Gov.Member"}, "Gov.Member.isActive"},
		} {
			out := callTool(t, s, "search_code", map[string]any{"pattern": tc.pattern})
			names := hitNames(out)
			if miss := missingNames(names, tc.want...); miss != nil {
				t.Errorf("search %q: missing %v in %v", tc.pattern, miss, names)
			}
			if tc.absent != "" && len(missingNames(names, tc.absent)) == 0 {
				t.Errorf("search %q: %s must not be a hit of its own: %v", tc.pattern, tc.absent, names)
			}
		}
	})
	t.Run("uses on a line", func(t *testing.T) {
		out := callTool(t, s, "search_code", map[string]any{"pattern": "msg.sender", "fixed": true})
		for _, tc := range []struct {
			hit  string
			line int
			uses []string
		}{
			{"Gov.onlyMember", 14, []string{"reads Gov.members", "reverts with Gov.NotAMember"}},
			{"Gov.vote", 19, []string{"writes Gov.members"}},
			{"Gov.vote", 20, []string{"emits Gov.Voted"}},
		} {
			h := hitByName(out, tc.hit)
			if h == nil {
				t.Fatalf("no hit %s in %v", tc.hit, hitNames(out))
			}
			if l := lineOf(h, tc.line); l == nil || !reflect.DeepEqual(stringsOf(l["uses"]), tc.uses) {
				t.Errorf("%s line %d = %v, want uses %v", tc.hit, tc.line, l, tc.uses)
			}
		}
		mod := callTool(t, s, "search_code", map[string]any{"pattern": "function vote"})
		if l := lineOf(hitByName(mod, "Gov.vote"), 17); l == nil || !reflect.DeepEqual(stringsOf(l["uses"]), []string{"applies modifier Gov.onlyMember"}) {
			t.Errorf("line 17 = %v, want the modifier application", l)
		}
	})
	t.Run("calls", func(t *testing.T) {
		for _, tc := range []struct {
			tool, qname string
			depth       int
			want        []string
		}{
			{"find_callees", "Gov.vote", 1, []string{"Gov.Voted", "Gov.onlyMember", "Gov.vote"}},
			{"find_callees", "Gov.vote", 2, []string{"Gov.NotAMember", "Gov.Voted", "Gov.onlyMember", "Gov.vote"}},
			{"find_callees", "Gov.onlyMember", 1, []string{"Gov.NotAMember", "Gov.onlyMember"}},
			{"find_callers", "Gov.NotAMember", 1, []string{"Gov.NotAMember", "Gov.onlyMember"}},
			{"find_callers", "Gov.NotAMember", 2, []string{"Gov.NotAMember", "Gov.onlyMember", "Gov.vote"}},
			{"find_callers", "Gov.Voted", 1, []string{"Gov.Voted", "Gov.vote"}},
			{"find_callers", "Gov.onlyMember", 1, []string{"Gov.onlyMember", "Gov.vote"}},
		} {
			out := callTool(t, s, tc.tool, map[string]any{"qname": tc.qname, "depth": tc.depth})
			if got := sorted(rowNames(out, "nodes")); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s(%s, depth %d) = %v, want %v", tc.tool, tc.qname, tc.depth, got, tc.want)
			}
		}
	})
	t.Run("references", func(t *testing.T) {
		for _, tc := range []struct {
			tool, qname string
			want        map[string]string
		}{
			{"find_references", "Gov.members", map[string]string{"Gov.vote write": fxSolFile, "Gov.onlyMember read": fxSolFile}},
			{"find_references", "Gov.quorum", map[string]string{"Gov.vote write": fxSolFile}},
			{"find_callers", "Gov.members", map[string]string{"Gov.vote write": fxSolFile, "Gov.onlyMember read": fxSolFile}},
		} {
			if got := users(callTool(t, s, tc.tool, map[string]any{"qname": tc.qname})); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s(%s) users = %v, want %v", tc.tool, tc.qname, got, tc.want)
			}
		}
	})
}

// TestFindSymbol_BareName: with exact=false a name matches in the case the
// caller wrote, the declarations of the build come first, function-local
// names and External stubs are left out unless asked for (and counted), a
// cut is reported with the total, and every node has its end line. On
// go-stablenet 2,649 returned nodes had a name of another case and "Hash"
// returned 100 nodes, 16 of them named Hash, of 63 in the source.
func TestFindSymbol_BareName(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		name      string
		args      map[string]any
		want      []string
		total     float64
		truncated bool
		leftOut   map[string]any
		otherCase []string
	}{
		{name: "declaration only", args: map[string]any{"name": "Pack", "exact": false},
			want: []string{"abi.ABI.Pack"}, total: 1, leftOut: map[string]any{"local": 1.0, "external": 1.0}},
		{name: "all", args: map[string]any{"name": "Pack", "exact": false, "all": true},
			want: []string{"abi.ABI.Pack", "abi.TestPack.Pack", "strings.Pack"}, total: 3},
		{name: "other case", args: map[string]any{"name": "pack", "exact": false},
			want: []string{"abi.Arguments.pack"}, total: 1},
		{name: "no such case", args: map[string]any{"name": "PACK", "exact": false},
			want: []string{}, otherCase: []string{"abi.ABI.Pack", "abi.Arguments.pack"}},
		{name: "cut", args: map[string]any{"name": "Size", "exact": false, "limit": 1},
			want: []string{"coll1.Set.Size"}, total: 2, truncated: true},
		{name: "local var statement", args: map[string]any{"name": "count", "exact": false},
			want: []string{"abi.count"}, total: 1, leftOut: map[string]any{"local": 1.0}},
		// A promoted method is one declaration: one row, not one per type
		// that embeds its receiver ("String": 169 rows for 155 declarations).
		{name: "promoted method is one row", args: map[string]any{"name": "Prepare", "exact": false},
			want: []string{"state.StateDB.Prepare"}, total: 1},
		// A member of a type declared inside a function is as local as the
		// type (38 of 472 queries returned such members).
		{name: "member of a function-local type", args: map[string]any{"name": "Size", "exact": false},
			want: []string{"coll1.Set.Size", "coll2.Other.Size"}, total: 2, leftOut: map[string]any{"local": 1.0}},
		{name: "function-local type", args: map[string]any{"name": "row", "exact": false},
			want: []string{}, leftOut: map[string]any{"local": 1.0}},
		{name: "function-local type asked for", args: map[string]any{"name": "row", "exact": false, "all": true},
			want: []string{"abi.TestPack.row"}, total: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := callTool(t, s, "find_symbol", tc.args)
			if got := rowNames(out, "nodes"); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("nodes = %v, want %v", got, tc.want)
			}
			if total, _ := out["total"].(float64); total != tc.total {
				t.Errorf("total = %v, want %v", out["total"], tc.total)
			}
			if got := out["truncated"] == true; got != tc.truncated {
				t.Errorf("truncated = %v, want %v", out["truncated"], tc.truncated)
			}
			if tc.leftOut != nil && !reflect.DeepEqual(out["left_out"], tc.leftOut) {
				t.Errorf("left_out = %v, want %v", out["left_out"], tc.leftOut)
			}
			if tc.otherCase != nil && !reflect.DeepEqual(stringsOf(out["other_case"]), tc.otherCase) {
				t.Errorf("other_case = %v, want %v", out["other_case"], tc.otherCase)
			}
			for _, n := range asSlice(out["nodes"]) {
				if m := n.(map[string]any); m["end_line"] == nil {
					t.Errorf("node %v has no end_line", m["qname"])
				}
			}
		})
	}
	exact := callTool(t, s, "find_symbol", map[string]any{"name": "state.StateDB.Prepare"})
	if n := asSlice(exact["nodes"]); len(n) != 1 || n[0].(map[string]any)["end_line"] != 14.0 {
		t.Errorf("exact find_symbol must carry end_line 14: %v", exact["nodes"])
	}
	// The types a method is promoted to are an attribute of its row, and
	// asking for it under one of them returns that row.
	wantTo := []string{"js.dummyStatedb", "logger.dummyStatedb"}
	for _, args := range []map[string]any{
		{"name": "Prepare", "exact": false},
		{"name": "state.StateDB.Prepare"},
		{"name": "logger.dummyStatedb.Prepare"},
	} {
		out := callTool(t, s, "find_symbol", args)
		n := asSlice(out["nodes"])
		if len(n) != 1 || n[0].(map[string]any)["qname"] != "state.StateDB.Prepare" || !reflect.DeepEqual(stringsOf(n[0].(map[string]any)["promoted_to"]), wantTo) {
			t.Errorf("find_symbol(%v) = %v, want state.StateDB.Prepare with promoted_to %v", args, out["nodes"], wantTo)
		}
	}
	// A TypeAlias node is every type that is not a struct or an interface;
	// sub_kind says which it is (11 of 11 defined types were shown as alias).
	hash := callTool(t, s, "find_symbol", map[string]any{"name": "common.Hash"})
	if n := asSlice(hash["nodes"]); len(n) != 1 || n[0].(map[string]any)["type"] != "TypeAlias" || n[0].(map[string]any)["sub_kind"] != "defined" {
		t.Errorf("a defined type must say so in sub_kind: %v", hash["nodes"])
	}
}

// TestSearchCode_SharedLine: a line that several declarations have a part of
// is searched in each part, and every match is reported once, under the
// declaration that owns its first character. On go-stablenet the text in
// front of a nested declaration was unsearchable (4 of 4 signature lines with
// an inline interface, 175 of 175 `type X` / `const X` lines inside
// functions), 8 of 69 names of multi-name declarations had no hit of their
// own and 3 such lines no hit at all.
func TestSearchCode_SharedLine(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		name, pattern string
		// line is reported under hit, with text, and under no other hit.
		line int
		hit  string
		text string
	}{
		{"text in front of an inline interface method", "func Slot(base", 1, "systemcontracts.Slot",
			"func Slot(base int, key interface{ Bytes() []byte }) int {"},
		{"a match that runs into the nested declaration", "interface{ Bytes", 1, "systemcontracts.Slot",
			"func Slot(base int, key interface{ Bytes() []byte }) int {"},
		{"the nested declaration's own part", `Bytes\(\) \[\]byte`, 1, "systemcontracts.Slot.interface@30.Bytes",
			"Bytes() []byte }) int {"},
		{"keyword of a package-level type", `type \w+ struct`, 1, "state.StateDB", "type StateDB struct {"},
		{"keyword of a package-level var", `var \w+ = common`, 3, "types.EmptyRootHash", "var EmptyRootHash = common.Hash{}"},
		{"keyword of a package-level const", `const \w+ =`, 4, "types.zeroHash", "const zeroHash = common.Hash{}"},
		{"keyword in front of a function-local type", "type row struct", 23, "abi.TestPack", "\ttype row struct {"},
		{"a function-local type's own text", "row struct", 23, "abi.TestPack.row", "row struct {"},
		{"a function-local const stored from its keyword", "const limit", 26, "abi.TestPack.limit", "\tconst limit = 2"},
		{"first name of a multi-name declaration", "forkA", 8, "types.forkA", "forkA, forkB = split()"},
		{"second name of a multi-name declaration", "forkB", 8, "types.forkB", "forkB = split()"},
		{"first name, both stored from the keyword", "twinA", 9, "types.twinA", "var twinA, twinB = split()"},
		{"second name, both stored from the keyword", "twinB", 9, "types.twinB", "var twinA, twinB = split()"},
		{"shared text of twins is reported once", "var twinA, twinB = split", 9, "types.twinA", "var twinA, twinB = split()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixed := !strings.ContainsAny(tc.pattern, `\`)
			out := callTool(t, s, "search_code", map[string]any{"pattern": tc.pattern, "fixed": fixed})
			var reported []string
			for _, r := range asSlice(out["hits"]) {
				h := r.(map[string]any)
				if l := lineOf(h, tc.line); l != nil && fileOf(tc.hit) == h["file"] {
					reported = append(reported, h["qname"].(string))
					if l["text"] != tc.text {
						t.Errorf("text = %q, want %q", l["text"], tc.text)
					}
				}
			}
			if !reflect.DeepEqual(reported, []string{tc.hit}) {
				t.Errorf("line %d reported under %v, want only %s (hits %v)", tc.line, reported, tc.hit, hitNames(out))
			}
		})
	}
}

// TestSearchCode_MultiNameDeclaration: the names of one declaration that the
// graph stores with one span share its lines. A match is counted once and
// reported under the name it touches, or under the first name; each hit lists
// the other names.
func TestSearchCode_MultiNameDeclaration(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		pattern string
		hits    []string
		matches float64
	}{
		{"twinB", []string{"types.twinB"}, 1},
		{`split\(\)`, []string{"types.forkB", "types.total", "types.twinA"}, 3},
		{`twin\w`, []string{"types.twinA", "types.twinB"}, 2},
	} {
		out := callTool(t, s, "search_code", map[string]any{"pattern": tc.pattern, "path": fxTypesFile})
		if got := sorted(hitNames(out)); !reflect.DeepEqual(got, tc.hits) || out["total_matches"] != tc.matches {
			t.Errorf("%s: hits %v total_matches %v, want %v and %v", tc.pattern, got, out["total_matches"], tc.hits, tc.matches)
		}
	}
	out := callTool(t, s, "search_code", map[string]any{"pattern": "twinB"})
	if h := hitByName(out, "types.twinB"); h == nil || !reflect.DeepEqual(stringsOf(h["declared_with"]), []string{"types.twinA"}) {
		t.Errorf("a name of a multi-name declaration must list the others: %v", h)
	}
}

// TestSearchCode_BlankDeclaration: a blank package-level declaration has no
// name but its text is source text: body search finds it (0 of 64 such lines
// of go-stablenet were found), name search does not.
func TestSearchCode_BlankDeclaration(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		args map[string]any
		want []string
	}{
		// Printed by its site name: other blank declarations carry types._.
		{map[string]any{"pattern": "var _ Hasher = (*Hash)(nil)", "fixed": true}, []string{"types._@" + fxTypesFile + ":11"}},
		{map[string]any{"pattern": "Hasher", "in": "both"}, []string{"types._@" + fxTypesFile + ":11"}},
		{map[string]any{"pattern": `types\._`, "in": "name"}, []string{}},
		{map[string]any{"pattern": "Hasher", "in": "name"}, []string{}},
	} {
		out := callTool(t, s, "search_code", tc.args)
		got := []string{}
		for _, r := range asSlice(out["hits"]) {
			h := r.(map[string]any)
			if h["file"] == fxTypesFile && h["start_line"] == 11.0 {
				got = append(got, h["qname"].(string))
				if h["name_match"] != nil {
					t.Errorf("%v: a blank declaration has no name to match", tc.args)
				}
			}
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%v: line 11 under %v, want %v", tc.args, got, tc.want)
		}
	}
	// A blank next to a name in one declaration (`var total, _ = split()`)
	// is no name: the line is reported under the named one, which does not
	// list the blank in declared_with.
	for _, pattern := range []string{"total, _", "_ = split"} {
		out := callTool(t, s, "search_code", map[string]any{"pattern": pattern, "fixed": true, "path": fxTypesFile})
		h := hitByName(out, "types.total")
		if names := hitNames(out); !reflect.DeepEqual(names, []string{"types.total"}) || h["declared_with"] != nil {
			t.Errorf("%q: hits %v declared_with %v, want only types.total without declared_with", pattern, names, h["declared_with"])
		}
	}
}

// TestSearchCode_LineAnchors: the anchors for line start and line end mean
// the line as far as it is stored, not a declaration's part of it, and a
// match that starts in the indentation in front of a nested declaration is
// that declaration's. On go-stablenet `(a+)+$` matched a constant whose line
// goes on with a comment, and `.{500,}` on an indented constant inside a
// function went to the constant although the stated rule gave its first
// character to the function.
func TestSearchCode_LineAnchors(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		name, pattern, file string
		line                int
		want                []string
	}{
		{"start anchor, declaration at the line start", `^func Slot`, fxSlotFile, 1, []string{"systemcontracts.Slot"}},
		{"start anchor, nested declaration in mid-line", `^Bytes`, fxSlotFile, 1, nil},
		{"end anchor, line shared with a nested declaration", `int \{$`, fxSlotFile, 1, []string{"systemcontracts.Slot.interface@30.Bytes"}},
		{"start anchor, indented nested declaration", `^Unsubscribe`, fxEventFile, 3, nil},
		{"start anchor with indentation", `^\s*Unsubscribe`, fxEventFile, 3, []string{"event.Subscription.Unsubscribe"}},
		{"indentation in front of a nested declaration", `\s+const limit`, fxNamesFile, 26, []string{"abi.TestPack.limit"}},
		{"whole indented line of a nested declaration", `^.{5,}$`, fxNamesFile, 26, []string{"abi.TestPack.limit"}},
		{"start anchor, second name of a multi-name spec", `^forkB`, fxTypesFile, 8, nil},
		{"start anchor, first name of a multi-name spec", `^forkA`, fxTypesFile, 8, []string{"types.forkA"}},
		{"end anchor before a stored trailing comment", `128$`, fxTypesFile, 12, nil},
		{"end anchor after a stored trailing comment", `fetches$`, fxTypesFile, 12, []string{"types.maxSkeleton"}},
		{"end anchor, another declaration follows on the line", `pairA struct\{\}$`, fxTypesFile, 13, nil},
		{"end anchor, last declaration of the line", `struct\{\}$`, fxTypesFile, 13, []string{"types.pairB"}},
		{"start anchor, first declaration of the line", `^type pair\w`, fxTypesFile, 13, []string{"types.pairA"}},
		{"no anchor, each declaration of the line", `type pair\w`, fxTypesFile, 13, []string{"types.pairA", "types.pairB"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := callTool(t, s, "search_code", map[string]any{"pattern": tc.pattern, "path": tc.file})
			var got []string
			for _, r := range asSlice(out["hits"]) {
				if h := r.(map[string]any); lineOf(h, tc.line) != nil {
					got = append(got, h["qname"].(string))
				}
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s: line %d under %v, want %v", tc.pattern, tc.line, got, tc.want)
			}
		})
	}
}

// fileOf is the fixture file of the declaration a shared-line case names.
func fileOf(qname string) string {
	switch {
	case strings.HasPrefix(qname, "systemcontracts."):
		return fxSlotFile
	case strings.HasPrefix(qname, "state."):
		return fxStateFile
	case strings.HasPrefix(qname, "abi."):
		return fxNamesFile
	}
	return fxTypesFile
}

// TestIndexHeaderCounts: with_lines and names_only add up to the rows listed,
// also when a row matched in its name only and so has no lines (137 rows
// against with_lines 131 in one go-stablenet response).
func TestIndexHeaderCounts(t *testing.T) {
	rows := []map[string]any{
		{"qname": "p.A", "file": "p/a.go", "lines": []map[string]any{{"line": 3}}},
		{"qname": "p.B", "file": "p/a.go", "name_match": true},
		{"qname": "p.C", "file": "p/a.go", "lines": []map[string]any{{"line": 9}}},
	}
	with, names := make([]map[string]any, len(rows)), make([]map[string]any, len(rows))
	for i, r := range rows {
		with[i], names[i] = indexRow(r, true, false), indexRow(r, false, false)
	}
	for _, tc := range []struct{ k, m, wantWith, wantNames int }{
		{3, 3, 3, 0},
		{2, 3, 2, 1},
		{0, 2, 0, 2},
	} {
		_, hdr := renderIndex(rows, with, names, map[string]any{}, tc.k, tc.m, len(rows), "search_code")
		gotNames, _ := hdr["names_only"].(int)
		if hdr["with_lines"] != tc.wantWith || gotNames != tc.wantNames {
			t.Errorf("k=%d m=%d: with_lines %v names_only %v, want %d and %d", tc.k, tc.m, hdr["with_lines"], hdr["names_only"], tc.wantWith, tc.wantNames)
		}
	}
}

// TestBareNameSeed: every tool that takes a symbol name resolves a bare name
// by the same rule. A name of another case or a function-local name does not
// make a unique declaration ambiguous (5 of 30 unique names did), an
// ambiguous name returns its candidates from impact_of_change and
// concurrency_impact too (impact_of_change answered not_found for 25 of 25),
// and a promoted method is answered as the method it copies.
func TestBareNameSeed(t *testing.T) {
	s := toolStore(t)
	t.Run("unique", func(t *testing.T) {
		for _, tc := range []struct {
			tool, arg, name, want string
		}{
			{"find_callers", "qname", "Pack", "abi.ABI.Pack"},
			{"find_callees", "qname", "Pack", "abi.ABI.Pack"},
			{"get_subgraph", "seed_qname", "Pack", "abi.ABI.Pack"},
			{"find_references", "qname", "count", "abi.count"},
			{"find_callers", "qname", "logger.dummyStatedb.Prepare", "state.StateDB.Prepare"},
		} {
			out := callTool(t, s, tc.tool, map[string]any{tc.arg: tc.name})
			if out["seed_qname"] != tc.want || out["ambiguous"] != nil || out["not_found"] != nil {
				t.Errorf("%s(%s): seed %v ambiguous %v not_found %v, want %s", tc.tool, tc.name, out["seed_qname"], out["ambiguous"], out["not_found"], tc.want)
			}
		}
		imp := callTool(t, s, "impact_of_change", map[string]any{"seed_qname": "use"})
		if seed, _ := imp["seed"].(map[string]any); seed["qname"] != "coll1.use" {
			t.Errorf("impact_of_change(use): seed %v, want coll1.use", imp["seed"])
		}
	})
	t.Run("promoted method", func(t *testing.T) {
		out := callTool(t, s, "find_callers", map[string]any{"qname": "logger.dummyStatedb.Prepare", "depth": 1})
		if out["promoted_from"] != "state.StateDB.Prepare" || len(missingNames(rowNames(out, "nodes"), "logger.TestPrepare")) != 0 {
			t.Errorf("callers of a promoted method: promoted_from %v nodes %v", out["promoted_from"], rowNames(out, "nodes"))
		}
		// Every seed tool says the same: the method answered for and a
		// note. impact_of_change and concurrency_impact gave neither key
		// (60 of 60 each on go-stablenet).
		for _, tc := range []struct{ tool, arg string }{
			{"find_callers", "qname"}, {"find_callees", "qname"}, {"find_references", "qname"}, {"get_subgraph", "seed_qname"},
			{"impact_of_change", "seed_qname"}, {"concurrency_impact", "seed_qname"}, {"change_history", "qname"},
		} {
			out := callTool(t, s, tc.tool, map[string]any{tc.arg: "logger.dummyStatedb.Prepare"})
			note, _ := out["seed_note"].(string)
			if out["promoted_from"] != "state.StateDB.Prepare" || !strings.Contains(note, "logger.dummyStatedb.Prepare") {
				t.Errorf("%s: promoted_from %v, seed_note %q", tc.tool, out["promoted_from"], note)
			}
		}
		conc := callTool(t, s, "concurrency_impact", map[string]any{"seed_qname": "logger.dummyStatedb.Prepare"})
		if conc["seed"] != "state.StateDB.Prepare" {
			t.Errorf("concurrency_impact seed = %v, want the declared method", conc["seed"])
		}
	})
	t.Run("ambiguous", func(t *testing.T) {
		want := []string{"coll1.Set.Size", "coll2.Other.Size"}
		for _, tc := range []struct{ tool, arg string }{
			{"find_callers", "qname"}, {"find_callees", "qname"}, {"find_references", "qname"},
			{"get_subgraph", "seed_qname"}, {"impact_of_change", "seed_qname"}, {"concurrency_impact", "seed_qname"},
			{"change_history", "qname"},
		} {
			out := callTool(t, s, tc.tool, map[string]any{tc.arg: "Size"})
			if out["ambiguous"] != true || !reflect.DeepEqual(stringsOf(out["candidates"]), want) {
				t.Errorf("%s(Size): ambiguous %v candidates %v not_found %v", tc.tool, out["ambiguous"], out["candidates"], out["not_found"])
			}
		}
	})
	t.Run("other case", func(t *testing.T) {
		out := callTool(t, s, "find_callers", map[string]any{"qname": "PACK"})
		if out["not_found"] != true || !reflect.DeepEqual(stringsOf(out["other_case"]), []string{"abi.ABI.Pack", "abi.Arguments.pack"}) {
			t.Errorf("find_callers(PACK) = %v", out)
		}
	})
}

// TestGetContextForTask_AnyText: a task description is words, whatever
// characters it holds. 14 of 26 inputs holding ( : - * " or a full-text
// operator came back as a raw full-text syntax error.
func TestGetContextForTask_AnyText(t *testing.T) {
	s := toolStore(t)
	for _, in := range []string{"%", "(", "*", "-", "AND*", "NOT x", "OR*", `\`, "a'b*", `a"b`, "foo(bar", "x AND", "x:y", "{}",
		`where does (Prepare) "reset" the logs: state-db*`} {
		// callTool fails the test when the handler returns an error.
		out := callTool(t, s, "get_context_for_task", map[string]any{"task_description": in})
		if out == nil {
			t.Errorf("no result for %q", in)
		}
	}
	out := callTool(t, s, "get_context_for_task", map[string]any{"task_description": `where does (Prepare) "reset" the logs: state-db*`})
	if out["not_found"] == true {
		t.Errorf("words of the description must still be searched: %v", out)
	}
}

// TestSearchText_NamesFirst: a declaration whose name is the query comes
// before every other match (exact-name recall was 14 of 20 at the default
// top_k), and names known only inside a function are left out by default
// (about 9% of hits were locals matched through the enclosing function's
// name).
func TestSearchText_NamesFirst(t *testing.T) {
	s := toolStore(t)
	for _, tc := range []struct {
		name   string
		args   map[string]any
		first  string
		absent []string
	}{
		{"exact name first", map[string]any{"query": "Pack", "top_k": 1}, "abi.ABI.Pack", nil},
		{"package-level variable, not the local", map[string]any{"query": "count"}, "abi.count", []string{"abi.TestPack.count"}},
		// Whatever is declared inside the function body is left out: of
		// 7,247 rows on go-stablenet 362 were fields of function-local
		// types, 8 local variables or constants and 66 goroutines or channels.
		{"nothing from inside the function through its name", map[string]any{"query": "TestPack", "top_k": 50}, "abi.TestPack",
			[]string{"abi.TestPack.count", "abi.TestPack.Pack", "abi.TestPack.row", "abi.TestPack.row.Size", "abi.TestPack.limit", "abi.TestPack#Goroutine@77"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := callTool(t, s, "search_text", tc.args)
			names := rowNames(out, "nodes")
			if len(names) == 0 || names[0] != tc.first {
				t.Errorf("nodes = %v, want %s first", names, tc.first)
			}
			if got := missingNames(names, tc.absent...); len(got) != len(tc.absent) {
				t.Errorf("nodes = %v, must not hold %v", names, tc.absent)
			}
		})
	}
	cut := callTool(t, s, "search_text", map[string]any{"query": "Size", "top_k": 1})
	if len(asSlice(cut["nodes"])) != 1 || cut["exact_names"] != 2.0 || cut["truncated"] != true {
		t.Errorf("a cut list of exact names must say so: exact_names %v truncated %v", cut["exact_names"], cut["truncated"])
	}
	asked := callTool(t, s, "search_text", map[string]any{"query": "count", "node_kinds": []any{"Variable"}})
	if len(missingNames(rowNames(asked, "nodes"), "abi.TestPack.count")) != 0 {
		t.Errorf("node_kinds given explicitly returns locals of that kind: %v", rowNames(asked, "nodes"))
	}
}

// TestTypeUsers_PackageLevelValues: a package-level variable or constant that
// uses a type is a user of that type. get_subgraph and impact_of_change
// missed 13 of 491 sampled users because the graph had no uses_type edge from
// Variable and Constant nodes; with the edge present both tools return them.
func TestTypeUsers_PackageLevelValues(t *testing.T) {
	s := toolStore(t)
	want := []string{"types.EmptyRootHash", "types.rootOf", "types.zeroHash"}
	sub := callTool(t, s, "get_subgraph", map[string]any{"seed_qname": "common.Hash", "depth": 1})
	if miss := missingNames(rowNames(sub, "nodes"), want...); miss != nil {
		t.Errorf("get_subgraph(common.Hash) misses %v", miss)
	}
	imp := callTool(t, s, "impact_of_change", map[string]any{"seed_qname": "common.Hash", "depth": 1})
	users, _ := imp["impact"].(map[string]any)
	if got := sorted(rowNames(users, "type_users")); !reflect.DeepEqual(got, want) {
		t.Errorf("impact_of_change(common.Hash).type_users = %v, want %v", got, want)
	}
}
