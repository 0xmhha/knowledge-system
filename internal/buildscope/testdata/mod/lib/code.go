package lib

import _ "embed"

// tokenCode is the compiled contracts/Token.sol, as a chain embeds the
// bytecode of its system contracts.
//
//go:embed artifacts/Token.bin
var tokenCode []byte

// TokenCode returns the embedded bytecode.
func TokenCode() []byte { return tokenCode }
