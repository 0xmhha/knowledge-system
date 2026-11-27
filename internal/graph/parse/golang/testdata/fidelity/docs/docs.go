// Package docs holds declarations whose doc comment is on the declaration,
// on the specification, or on the group around it.
package docs

// Limit is documented on a single const declaration.
const Limit = 10

// Name is documented on a single var declaration.
var Name = "x"

var (
	// Grouped is documented on its own specification.
	Grouped = 1
)

// Messages is the comment of the group, not of its members.
type (
	// First has its own comment.
	First struct{}

	Second struct{}
)

type (
	// Third has its own comment in a group without one.
	Third struct{}
)

// first and second are declared by one specification.
var first, second = 1, 2

// Two blank names of one specification.
var _, _ = 1, 2

const Depth = 3 // Depth has a comment after it.

// Pair has comments after a field and after the declaration.
type Pair struct {
	Left, Right int // Left and Right share a field declaration.
	Sum         int
} // Pair ends here.

// Walker has a comment after a method.
type Walker interface {
	Walk() int // Walk has a comment after it.
}

// Lone is documented on a single type declaration.
type Lone struct{}

// Use declares a documented local variable.
func Use() int {
	// count is documented inside a body.
	var count = Limit
	type pair struct{ a, b int }
	const step = 1
	return count + pair{first, second}.a + step
}

// Listening ends with a comment after its closing brace.
func Listening() bool { return true } // Always listening.

// Closing has a comment after the brace that closes it.
func Closing() int {
	return 1
} // Closing ends here.

// Total is a method that ends with a comment.
func (p Pair) Total() int { return p.Left + p.Right } // Total is the sum.

// Bare has no comment after it; the line under it documents After.
func Bare() {}

// After is documented on the line under Bare.
func After() {}
