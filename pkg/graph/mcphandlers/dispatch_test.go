package mcphandlers

import (
	"strings"
	"testing"
)

// The runtime-links fixture is testdata/resolve/coll1/dispatch.go:
// useTip(t Tipper) calls t.Tip(); govTip and fixedTip values reach it,
// unusedTip implements Tipper but never reaches one; eachBlock(f) calls f,
// and runBlocks passes onBlock.

func hasQname(ns []nodeQname, q string) bool {
	for _, n := range ns {
		if string(n) == q {
			return true
		}
	}
	return false
}

// find_callees: an interface call reaches the implementations its values can
// run at the same depth, and not an implementation no value reaches.
func TestFindCallees_DispatchTargets(t *testing.T) {
	store := newFixtureStore(t)
	seed, _, _, ok := resolveSeed(store, "coll1.useTip", "")
	if !ok {
		t.Fatal("fixture has no coll1.useTip")
	}
	ns, _, err := forwardCallees(store, seed, 1)
	if err != nil {
		t.Fatal(err)
	}
	got := qnames(ns)
	for _, want := range []string{"coll1.Tipper.Tip", "coll1.govTip.Tip", "coll1.fixedTip.Tip"} {
		if !hasQname(got, want) {
			t.Errorf("find_callees(useTip, depth 1) misses %s; got %v", want, got)
		}
	}
	if hasQname(got, "coll1.unusedTip.Tip") {
		t.Errorf("find_callees(useTip) names unusedTip.Tip, which no Tipper value holds")
	}
}

// find_callers: callers through an interface come from dispatches_to when it
// reaches the method, and from the type hierarchy when no analysed value does.
func TestFindCallers_DispatchSeeds(t *testing.T) {
	store := newFixtureStore(t)
	for _, c := range []struct {
		seed, source string
	}{
		{"coll1.govTip.Tip", "dispatches_to"},
		{"coll1.unusedTip.Tip", "type_hierarchy"},
	} {
		seed, _, _, ok := resolveSeed(store, c.seed, "")
		if !ok {
			t.Fatalf("fixture has no %s", c.seed)
		}
		seeds, source := dispatchSeeds(store, seed)
		if source != c.source || len(seeds) != 1 || seeds[0] != "coll1.Tipper.Tip" {
			t.Errorf("dispatchSeeds(%s) = %v from %q, want [coll1.Tipper.Tip] from %q", c.seed, seeds, source, c.source)
		}
		ns, _, err := reverseCallersUnion(store, seed, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !hasQname(qnames(ns), "coll1.useTip") {
			t.Errorf("find_callers(%s) misses coll1.useTip", c.seed)
		}
	}
}

// search_code line uses name what a dynamic call can run.
func TestUsesByLine_DynamicTargets(t *testing.T) {
	store := newFixtureStore(t)
	var ids []string
	for _, q := range []string{"coll1.useTip", "coll1.eachBlock", "coll1.UseHasher"} {
		seed, _, _, ok := resolveSeed(store, q, "")
		own, err := seed.nodes(store)
		if !ok || err != nil || len(own) == 0 {
			t.Fatalf("fixture has no %s", q)
		}
		for _, n := range own {
			ids = append(ids, n.ID)
		}
	}
	uses := usesByLine(store, ids)
	var all []string
	for _, m := range uses {
		for _, us := range m {
			all = append(all, us...)
		}
	}
	text := strings.Join(all, "\n")
	for _, want := range []string{
		// value-flow targets named; the type hierarchy's only counted
		"calls coll1.Tipper.Tip (may run coll1.fixedTip.Tip, coll1.govTip.Tip; 1 more by type)",
		// no value-flow target: the type hierarchy's named
		"calls coll1.Hasher.Hash (may run by type coll1.Thing.Hash)",
		"calls a function value that may run coll1.onBlock",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("line uses miss %q; got:\n%s", want, text)
		}
	}
}

// The type-hierarchy bridge seeds only interface methods that exist: Reset
// is a method of govTip, which implements Tipper, but Tipper has no Reset.
func TestImplementsSeeds_OnlyExistingMethods(t *testing.T) {
	store := newFixtureStore(t)
	if got := implementsSeeds(store, "coll1.govTip.Reset"); len(got) != 0 {
		t.Errorf("implementsSeeds(govTip.Reset) = %v, want none (Tipper has no Reset)", got)
	}
	if got := implementsSeeds(store, "coll1.govTip.Tip"); len(got) != 1 || got[0] != "coll1.Tipper.Tip" {
		t.Errorf("implementsSeeds(govTip.Tip) = %v, want [coll1.Tipper.Tip]", got)
	}
}
