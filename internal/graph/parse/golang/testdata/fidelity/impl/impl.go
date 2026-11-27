// Package impl holds interface satisfaction that involves a test file, types
// declared inside a function, and an interface from outside the build.
package impl

import "io"

// Decoder is implemented by a type that a test file declares.
type Decoder interface{ Decode(s *Stream) error }

// Stream is named in Decoder's method, so a package loaded again for its
// tests has a second Stream.
type Stream struct{}

// ByteSource embeds an interface from outside the build.
type ByteSource interface {
	io.Reader
	Len() int
}

// Fixed implements the interface sized, which Hold declares in its body.
type Fixed struct{}

// Len is the length.
func (Fixed) Len() int { return 0 }

// Hold declares a struct and an interface in its body.
func Hold(d Decoder) Decoder {
	type local struct{ Decoder }
	type sized interface{ Len() int }
	var s sized = Fixed{}
	_ = s
	return local{d}
}
