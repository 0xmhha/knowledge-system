// Package vars holds type references written in package-level var and const
// declarations, through an alias, and to a type declared inside a function.
package vars

// Mode is a defined type used by a constant.
type Mode int

// Point is used by package-level variables.
type Point struct{ X, Y int }

// Origin is initialised by a composite literal of Point.
var Origin = Point{}

// Fast has a declared type.
const Fast Mode = 1

// Shared has a declared type and no value.
var Shared *Point

// Short is an alias of Point.
type Short = Point

// Make instantiates Point through its alias.
func Make() *Short { return new(Short) }

// Wide has a signature that spans several lines.
func Wide(
	a int,
	m Mode, // Mode is named here
) Point { // Point is named here
	return Point{X: a + int(m)}
}

// Inner uses a type declared in its body.
func Inner() int {
	type pair struct{ a, b int }
	p := pair{1, 2}
	return p.a + p.b
}
