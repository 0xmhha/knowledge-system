package mcphandlers

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// messagesFixture is a graph of the concurrency fixture: goroutines, a
// worker loop, a select, channels passed as parameters and kept in fields.
func messagesFixture(t *testing.T) persist.Store {
	t.Helper()
	out := t.TempDir()
	if _, err := buildpipe.Run(buildpipe.Options{
		SrcRoot: "../../../internal/graph/parse/golang/testdata/concurrency", OutDir: out,
		Languages: []string{"go"}, CKGVersion: "test", NoCache: true,
	}); err != nil {
		t.Fatal(err)
	}
	s, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestWhatRunsThisCode: find_callers keeps its callers function-level and
// lists the goroutines that run a function (goroutine_callers) and the
// receives whose message runs it with their senders (handled_by); a
// function's callees list what its sends wake (triggers).
func TestWhatRunsThisCode(t *testing.T) {
	s := messagesFixture(t)
	callers := callTool(t, s, "find_callers", map[string]any{"qname": "mutex_fixture.record", "depth": 1})
	for _, n := range asSlice(callers["nodes"]) {
		if n.(map[string]any)["type"] == "Goroutine" {
			t.Errorf("a Goroutine among the callers: %v", n)
		}
	}
	gors := asSlice(callers[goroutineCallersKey])
	if len(gors) == 0 {
		t.Errorf("no goroutine_callers for record: %v", callers)
	}
	handled := map[string]bool{}
	for _, r := range asSlice(callers[handledByKey]) {
		row := r.(map[string]any)
		handled[row["receive"].(string)] = len(asSlice(row["senders"])) > 0
	}
	if len(handled) < 4 {
		t.Errorf("handled_by: %v, want the receives of WorkerLoop, Serve, Drain and Local", callers[handledByKey])
	}
	callees := callTool(t, s, "find_callees", map[string]any{"qname": "mutex_fixture.RunServe", "depth": 1})
	trig := asSlice(callees[triggersKey])
	// The close of done wakes its receive as the send after it does.
	if len(trig) != 4 {
		t.Fatalf("triggers of RunServe = %d, want 4 (nums, names, the close and the send on done): %v", len(trig), callees[triggersKey])
	}
	for _, r := range trig {
		if len(asSlice(r.(map[string]any)["handles"])) == 0 {
			t.Errorf("trigger without handled code: %v", r)
		}
	}
}

// TestGoroutineUsersAndChannelOps: a package-level variable a goroutine's
// literal assigns lists that goroutine under goroutine_users (access write);
// concurrency_impact keeps goroutine channel operations apart under
// goroutines; a send in a function that goroutines run is one handled_by
// sender with those goroutines.
func TestGoroutineUsersAndChannelOps(t *testing.T) {
	s := messagesFixture(t)
	refs := callTool(t, s, "find_references", map[string]any{"qname": "mutex_fixture.hits"})
	access := map[any]bool{}
	for _, u := range asSlice(refs[goroutineUsersKey]) {
		access[u.(map[string]any)["access"]] = true
	}
	if !access["read"] || !access["write"] {
		t.Errorf("goroutine_users of hits (hits++ reads and writes): %v", refs[goroutineUsersKey])
	}
	conc := callTool(t, s, "concurrency_impact", map[string]any{"seed_qname": "mutex_fixture.Worker.inbox"})
	if len(asSlice(conc["goroutines"])) == 0 {
		t.Errorf("no goroutines key: %v", conc)
	}
	for _, e := range asSlice(conc["edges"]) {
		if row := e.([]any); row[2] == "sends_to" || row[2] == "recvs_from" {
			for _, m := range asSlice(conc["modules"]) {
				mm := m.(map[string]any)
				if mm["id"] == row[0] && mm["type"] == "Goroutine" {
					t.Errorf("a goroutine channel edge in edges: %v", row)
				}
			}
		}
	}
}

// TestChannelLockSeed: a field holding a channel used as a lock seeds
// concurrency_impact like a mutex: mutex is its Channel node and locked_by
// lists the function that takes the token.
func TestChannelLockSeed(t *testing.T) {
	s := messagesFixture(t)
	out := callTool(t, s, "concurrency_impact", map[string]any{"seed_qname": "mutex_fixture.Device.comms"})
	if m, _ := out["mutex"].(string); !strings.HasPrefix(m, "chan@") {
		t.Fatalf("mutex = %v", out["mutex"])
	}
	found := false
	for _, l := range asSlice(out["locked_by"]) {
		if l.(map[string]any)["qname"] == "mutex_fixture.Device.Bump" {
			found = true
		}
	}
	if !found {
		t.Errorf("locked_by = %v", out["locked_by"])
	}
}

// TestLockSeedAndLiteralSender: "<field>#mutex" names a channel lock like a
// sync.Mutex field; a send in a goroutine's own literal is listed in
// handled_by under the function that starts the goroutine, with it.
func TestLockSeedAndLiteralSender(t *testing.T) {
	s := messagesFixture(t)
	out := callTool(t, s, "concurrency_impact", map[string]any{"seed_qname": "mutex_fixture.Device.comms#mutex"})
	if m, _ := out["mutex"].(string); !strings.HasPrefix(m, "chan@") {
		t.Errorf("#mutex seed: mutex = %v (%v)", out["mutex"], out)
	}
	callers := callTool(t, s, "find_callers", map[string]any{"qname": "mutex_fixture.pong", "depth": 1})
	rows := asSlice(callers[handledByKey])
	if len(rows) != 1 {
		t.Fatalf("handled_by = %v", callers[handledByKey])
	}
	senders := asSlice(rows[0].(map[string]any)["senders"])
	if len(senders) != 1 || senders[0].(map[string]any)["sender"] != "mutex_fixture.Pinger" || len(asSlice(senders[0].(map[string]any)["goroutines"])) != 1 {
		t.Errorf("senders = %v", senders)
	}
}

// TestLockFieldCard: the card of a field that holds a channel used as a
// lock says it is a lock and labels its sends and receives as lock
// operations.
func TestLockFieldCard(t *testing.T) {
	s := messagesFixture(t)
	x, err := invIndexFor(s)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for nid, n := range x.nodes {
		if n.QualifiedName == "mutex_fixture.Device.comms" {
			id = nid
		}
	}
	card := x.valueCard(id, false, false)
	for _, want := range []string{"lock: a channel used as a mutex", "(lock operations)"} {
		if !strings.Contains(card, want) {
			t.Errorf("card of Device.comms misses %q:\n%s", want, card)
		}
	}
}

// TestClosureCallsAndLocalLock: a call inside a literal passed to another
// call names that call's function (closure_calls), whose own handled_by
// gives the receive that runs it; a local variable holding a channel lock
// seeds concurrency_impact in both forms.
func TestClosureCallsAndLocalLock(t *testing.T) {
	s := messagesFixture(t)
	callers := callTool(t, s, "find_callers", map[string]any{"qname": "mutex_fixture.snap", "depth": 1})
	rows := asSlice(callers[closureCallsKey])
	if len(rows) != 1 || rows[0].(map[string]any)["in_literal_passed_to"] != "mutex_fixture.Reg.Each" {
		t.Errorf("closure_calls = %v", callers[closureCallsKey])
	}
	each := callTool(t, s, "find_callers", map[string]any{"qname": "mutex_fixture.Reg.Each", "depth": 1})
	if len(asSlice(each[handledByKey])) == 0 {
		t.Errorf("handled_by of Reg.Each = %v", each[handledByKey])
	}
	for _, q := range []string{"mutex_fixture.Resetter.Loop.busy", "mutex_fixture.Resetter.Loop.busy#mutex"} {
		out := callTool(t, s, "concurrency_impact", map[string]any{"seed_qname": q})
		if m, _ := out["mutex"].(string); !strings.HasPrefix(m, "chan@") || len(asSlice(out["locked_by"])) == 0 {
			t.Errorf("%s: mutex %v locked_by %v", q, out["mutex"], out["locked_by"])
		}
	}
}

// TestSeveralEntriesOnOneLine: the determinism subjects do have several
// entries on one line under the keys whose fixture code puts them there
// (the fixture's two go statements now stand on lines of their own, as
// gofmt writes them, so goroutine_callers is not among them).
func TestSeveralEntriesOnOneLine(t *testing.T) {
	s := messagesFixture(t)
	callers := callTool(t, s, "find_callers", map[string]any{"qname": "mutex_fixture.multi", "depth": 1})
	for _, k := range []string{handledByKey, closureCallsKey} {
		lines := map[any]int{}
		for _, r := range asSlice(callers[k]) {
			lines[r.(map[string]any)["line"]]++
		}
		max := 0
		for _, n := range lines {
			if n > max {
				max = n
			}
		}
		if max < 2 {
			t.Errorf("%s has no two entries on one line: %v", k, callers[k])
		}
	}
	callees := callTool(t, s, "find_callees", map[string]any{"qname": "mutex_fixture.RunTwo", "depth": 1})
	if len(asSlice(callees[triggersKey])) < 2 {
		t.Errorf("triggers of RunTwo: %v", callees[triggersKey])
	}
}

// TestWakeRows: a row says what wakes a receive. A close is a close, not a
// send (close_line, by close); a receive on a timer's channel is woken by
// the runtime when the timer made at a site fires; a receive on ctx.Done()
// when the context is cancelled, with who calls its cancel function.
func TestWakeRows(t *testing.T) {
	s := messagesFixture(t)
	callees := callTool(t, s, "find_callees", map[string]any{"qname": "mutex_fixture.Crew.Stop", "depth": 1})
	closes := 0
	for _, r := range asSlice(callees[triggersKey]) {
		row := r.(map[string]any)
		if row["by"] != "close" || row["close_line"] != float64(25) || row["send_line"] != nil {
			t.Errorf("Stop's trigger row should be a close at line 25: %v", row)
		}
		closes++
	}
	if closes == 0 {
		t.Fatalf("no triggers for Stop: %v", callees)
	}
	wake := map[string]map[string]any{}
	for _, q := range []string{"mutex_fixture.Crew.init", "mutex_fixture.Crew.flush"} {
		callers := callTool(t, s, "find_callers", map[string]any{"qname": q, "depth": 1})
		for _, r := range asSlice(callers[handledByKey]) {
			row := r.(map[string]any)
			if row["line"] == float64(174) || row["line"] == float64(176) {
				wake[q] = row
			}
		}
	}
	timer := wake["mutex_fixture.Crew.init"]
	if timer["woken_by"] != "timer" || !strings.Contains(asString(timer["timer_made_at"]), "execfacts.go:172") || timer[noSenderKey] != nil {
		t.Errorf("timer receive row: %v", timer)
	}
	ctx := wake["mutex_fixture.Crew.flush"]
	if ctx["woken_by"] != "cancel" || !strings.Contains(asString(ctx["cancelled_by"]), `"caller":"mutex_fixture.Crew.Tick","file":"execfacts.go","line":171`) || ctx[noSenderKey] != nil {
		t.Errorf("ctx.Done receive row: %v", ctx)
	}
}

func asString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// TestFindSymbolBareName: a bare name with the default exact=true is
// resolved as every tool resolves a name: one declaration ends in it -> its
// node; several -> the candidates.
func TestFindSymbolBareName(t *testing.T) {
	s := messagesFixture(t)
	out := callTool(t, s, "find_symbol", map[string]any{"name": "Deferring"})
	nodes := asSlice(out["nodes"])
	if len(nodes) != 1 || nodes[0].(map[string]any)["qname"] != "mutex_fixture.Crew.Deferring" {
		t.Errorf("find_symbol Deferring = %v", out)
	}
	if out["resolved_by"] != "bare name" {
		t.Errorf("the answer should say it resolved a bare name: %v", out)
	}
	amb := callTool(t, s, "find_symbol", map[string]any{"name": "Stop"})
	if amb["ambiguous"] != true || len(asSlice(amb["candidates"])) < 2 {
		t.Errorf("find_symbol Stop should list candidates: %v", amb)
	}
}

// TestGoStartIsNotACall: go Serve(...) starts Serve: RunServe is listed
// under goroutine_callers with the go line, and not among the callers.
func TestGoStartIsNotACall(t *testing.T) {
	s := messagesFixture(t)
	out := callTool(t, s, "find_callers", map[string]any{"qname": "mutex_fixture.Serve", "depth": 1})
	for _, n := range asSlice(out["nodes"]) {
		if n.(map[string]any)["qname"] == "mutex_fixture.RunServe" {
			t.Errorf("RunServe, which starts Serve, is among its callers: %v", out["edges"])
		}
	}
	found := false
	for _, r := range asSlice(out[goroutineCallersKey]) {
		row := r.(map[string]any)
		if row["spawned_by"] == "mutex_fixture.RunServe" {
			found = true
			if row["line"] != float64(58) {
				t.Errorf("goroutine row line = %v, want the go line 58", row["line"])
			}
		}
	}
	if !found {
		t.Errorf("no goroutine row spawned by RunServe: %v", out[goroutineCallersKey])
	}
}

// TestSenderRowShape: a handled_by sender row has the shape of a triggers
// row (by with send_line or close_line), and both answers explain the keys
// in their legend.
func TestSenderRowShape(t *testing.T) {
	s := messagesFixture(t)
	callers := callTool(t, s, "find_callers", map[string]any{"qname": "mutex_fixture.record", "depth": 1})
	n := 0
	for _, r := range asSlice(callers[handledByKey]) {
		for _, sd := range asSlice(r.(map[string]any)["senders"]) {
			row := sd.(map[string]any)
			n++
			if row["line"] != nil || (row["send_line"] == nil) == (row["close_line"] == nil) || row["by"] == nil {
				t.Errorf("sender row %v", row)
			}
		}
	}
	if n == 0 {
		t.Fatalf("no sender rows: %v", callers[handledByKey])
	}
	callees := callTool(t, s, "find_callees", map[string]any{"qname": "mutex_fixture.RunServe", "depth": 1})
	for name, out := range map[string]map[string]any{"find_callers": callers, "find_callees": callees} {
		if l, _ := out[legendKey].(map[string]any); l["by"] == nil {
			t.Errorf("%s: legend does not explain by: %v", name, out[legendKey])
		}
	}
}

// TestContextHoldsDeclarations: get_context_for_task answers with
// declarations: no statement node, goroutine or name local to a function
// among its bodies, summaries or subgraph nodes.
func TestContextHoldsDeclarations(t *testing.T) {
	s := messagesFixture(t)
	out := callTool(t, s, "get_context_for_task", map[string]any{"task_description": "serve record worker loop crew stop tick goroutine", "budget_tokens": 4000})
	seen := 0
	for _, key := range []string{"bodies", "summaries"} {
		for _, r := range asSlice(out[key]) {
			seen++
			// A mutex is named "<field>#mutex"; any other # is a statement's.
			if q, _ := r.(map[string]any)["qname"].(string); strings.Contains(strings.TrimSuffix(q, "#mutex"), "#") {
				t.Errorf("%s holds %s", key, q)
			}
		}
	}
	sub, _ := out["subgraph"].(map[string]any)
	for _, n := range asSlice(sub["nodes"]) {
		switch tp := n.(map[string]any)["type"]; tp {
		case "Goroutine", "LocalVariable", "CallSite", "FuncLit", "DeferStmt", "CaseClause", "BranchStmt", "IfStmt", "LoopStmt", "ReturnStmt", "SwitchStmt":
			t.Errorf("subgraph holds a %v", tp)
		}
	}
	if seen == 0 {
		t.Fatalf("no answer: %v", out)
	}
}
