// Package values calls functions through values the callee of which is
// known: a local set once, a method value, a field set once, a parameter.
package values

func inc(n int) int { return n + 1 }

func dec(n int) int { return n - 1 }

type box struct{ n int }

func (b *box) get() int { return b.n }

// Local calls through a local function value, plain and deferred.
func Local() int {
	f := inc
	defer f(1)
	return f(2)
}

// Method calls through a method value.
func Method(b *box) int {
	g := b.get
	return g()
}

type holder struct{ fn func(int) int }

// Field calls through a field that one composite literal sets.
func Field() int {
	h := holder{fn: dec}
	return h.fn(3)
}

func apply(f func(int) int) int { return f(4) }

// Param passes a function that apply calls.
func Param() int { return apply(inc) }

// Own calls a literal of its own: the callee has no node but its caller.
func Own() int {
	twice := func(n int) int { return 2 * n }
	return twice(5)
}
