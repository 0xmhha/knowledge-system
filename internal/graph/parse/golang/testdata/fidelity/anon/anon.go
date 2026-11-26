// Package anon holds struct types that have no name of their own, and the
// local names of a function.
package anon

// Config is a package-level variable of an anonymous struct type.
var Config struct {
	Name string
	Port int
}

// Rows is a named slice type over an anonymous struct.
type Rows []struct {
	Key string
}

// Table declares a struct type inside a composite literal, as table tests do,
// and uses names made by := and by range.
func Table() int {
	cases := []struct {
		in   int
		want int
	}{{1, 2}}
	sum := 0
	for _, tc := range cases {
		sum += tc.in + tc.want
	}
	Config.Port = sum
	return len(Config.Name)
}
