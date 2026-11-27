package mcphandlers

import (
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
	if len(trig) != 3 {
		t.Fatalf("triggers of RunServe = %d, want 3 (nums, names, done): %v", len(trig), callees[triggersKey])
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
// entries on one line under each key.
func TestSeveralEntriesOnOneLine(t *testing.T) {
	s := messagesFixture(t)
	callers := callTool(t, s, "find_callers", map[string]any{"qname": "mutex_fixture.multi", "depth": 1})
	for _, k := range []string{handledByKey, goroutineCallersKey, closureCallsKey} {
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
