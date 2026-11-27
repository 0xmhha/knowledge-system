// Package twice has an in-package test, so the go command loads its files a
// second time in the test variant.
package twice

func helper() int { return 1 }

// Use calls helper once in a condition and once in a body.
func Use() int {
	if helper() > 0 {
		return helper()
	}
	return 0
}
