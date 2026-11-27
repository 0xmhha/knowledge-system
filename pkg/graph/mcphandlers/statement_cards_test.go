package mcphandlers

import (
	"strings"
	"testing"
)

// Cards name the conditions that decide a write (statement links, schema
// 1.32) and who sends and receives on a channel. Fixtures:
// testdata/resolve/coll1/stmts.go and chans.go.
func TestCards_GuardsAndChannels(t *testing.T) {
	store := newFixtureStore(t)
	x, err := invIndexFor(store)
	if err != nil {
		t.Fatal(err)
	}
	id := func(q string) string {
		for k, n := range x.nodes {
			if n.QualifiedName == q {
				return k
			}
		}
		t.Fatalf("no node %s", q)
		return ""
	}
	state := x.valueCard(id("coll1.env.state"), false, false)
	for _, want := range []string{
		"coll1.env.refresh:21  e.state = load(root) [then of L19 if e.root != root {]",
		"[go of L38 go func() {]",
	} {
		if !strings.Contains(state, want) {
			t.Errorf("value card of env.state misses %q:\n%s", want, state)
		}
	}
	count := x.valueCard(id("coll1.env.count"), false, false)
	if !strings.Contains(count, "coll1.env.drain:33  e.count = 0 [case of L31 switch e.state {; in body of L30]") {
		t.Errorf("value card of env.count misses the switch and loop around the write:\n%s", count)
	}
	if strings.Contains(count, "coll1.env.refresh:25  e.count++ [") {
		t.Errorf("e.count++ in refresh is outside every condition:\n%s", count)
	}
	fn := x.funcCard(id("coll1.env.refresh"), nil, true, false)
	if !strings.Contains(fn, "writes coll1.env.ready [else of L19 if e.root != root {]") {
		t.Errorf("function card of refresh misses the else guard:\n%s", fn)
	}
	ch := x.valueCard(id("coll1.pump.out"), false, false)
	for _, want := range []string{"same channel as: chan@coll1/chans.go:", "senders (1 lines)", "coll1.pump.push:10", "receivers (1 lines)", "coll1.pump.pull:12"} {
		if !strings.Contains(ch, want) {
			t.Errorf("value card of pump.out misses %q:\n%s", want, ch)
		}
	}
}
