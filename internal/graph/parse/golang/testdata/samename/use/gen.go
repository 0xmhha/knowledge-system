package use

// Marshal declares a function-local type with the same name as the
// package-level Genesis, the way gencodec output does.
func (g Genesis) Marshal() int {
	type Genesis struct{ B int }
	return Genesis{B: g.A}.B
}
