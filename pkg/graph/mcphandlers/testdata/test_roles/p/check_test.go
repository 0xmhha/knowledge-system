package p

// checkUses is listed as production code by the scope although its path
// ends in _test.go.
func checkUses(t *T) int { return t.v + 2 }
