package coll1

// --- value-use fixture (value_access.go) ---

const slot = 8

// Limit's initializer reads slot.
const Limit = 4 * slot

// Registry's initializer reads Limit.
var Registry = map[string]int{"a": Limit}

var current int

type Inner struct{ Depth int }

type Outer struct {
	Inner
	Name string
}

func setCurrent(n int) { current = n } // writes current

func incCurrent() { current = current + 1 } // reads and writes current on one line

func readCurrent() int { return current + Limit } // reads current and Limit

func register(k string) { Registry[k] = 1 } // writes into Registry

func newOuter() *Outer { return &Outer{Name: "x"} } // sets Outer.Name in a literal

func depth(o *Outer) int { return o.Depth } // reads the promoted Inner.Depth

func bump(o *Outer) {
	o.Depth++ // writes the promoted Inner.Depth
	o.Depth++
}
