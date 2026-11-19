package coll1

// --- runtime links fixture (dispatches_to) ---

// Tipper has three implementations. Values of only two of them ever reach a
// call of Tip, so the link from Tipper.Tip goes to those two; a link by the
// type hierarchy alone would also go to unusedTip.
type Tipper interface{ Tip() int }

type govTip struct{ v int }

func (g *govTip) Tip() int { return g.v }

// Reset is not part of Tipper: no interface call can reach it.
func (g *govTip) Reset() { g.v = 0 }

type fixedTip struct{}

func (fixedTip) Tip() int { return 1 }

// unusedTip satisfies Tipper but is never stored in one.
type unusedTip struct{}

func (unusedTip) Tip() int { return 2 }

func useTip(t Tipper) int { return t.Tip() }

func runTips() int {
	u := unusedTip{}
	return useTip(&govTip{v: 3}) + useTip(fixedTip{}) + u.Tip()
}

// A function value: eachBlock calls whatever it is given.
func onBlock(n int) int { return n + 1 }

func eachBlock(f func(int) int) int { return f(1) }

func runBlocks() int { return eachBlock(onBlock) }
