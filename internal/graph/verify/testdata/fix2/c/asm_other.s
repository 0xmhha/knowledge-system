//go:build !arm64

#include "textflag.h"

// func add(a, b *Word)
TEXT ·add(SB), NOSPLIT, $0-16
	RET
