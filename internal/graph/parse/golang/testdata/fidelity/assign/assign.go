// Package assign holds assignments, constants and a type switch whose uses
// the fidelity tests check.
package assign

import "errors"

// Size is an array length used in a signature and in a type declaration.
const Size = 4

// Digest is a defined type whose length is a constant.
type Digest [Size]byte

// Alias is another name for Digest.
type Alias = Digest

// Sum has the constant in its result type only.
func Sum(data []byte) (out [Size]byte) {
	copy(out[:], data)
	return out
}

func step() (int, error) { return 1, errors.New("step") }

// Redeclare assigns err a second time with a short variable declaration.
func Redeclare() error {
	a, err := step()
	b, err := step() // err is assigned here
	_ = a + b
	return err
}

// Local reads a constant declared in its body.
func Local() int {
	const limit = 3
	return limit + 1 // limit is read here
}

// Kind switches on the type of v.
func Kind(v any) int {
	switch t := v.(type) {
	case int:
		return t // t is an int here
	case string:
		return len(t) // t is a string here
	}
	return 0
}
