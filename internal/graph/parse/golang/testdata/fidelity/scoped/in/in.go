// Package in is inside the build scope and uses package out, which is not.
package in

import "ckgfidelity.test/scoped/out"

// Use calls a function and a method of out and names its type.
func Use(t out.T) int { return out.Helper() + t.Twice() }
