package parse

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// MakeID returns a stable 16-char hash for (qname, language, startByte).
// All language parsers must use this so node IDs collide deterministically
// across pass boundaries and re-runs.
func MakeID(qname, lang string, startByte int) string {
	return hashID(fmt.Sprintf("%s|%s|%d", qname, lang, startByte))
}

// NodeID returns the node id of a declaration-level node from its language,
// the file (or, for a package node, the package directory) it is declared in,
// its qualified name and its start byte.
//
// The file is part of the key because the qualified name uses the package
// name, not the import path: two packages that both declare `package x`, or
// two `package main` programs, produce the same qualified name, and when the
// start byte matches too MakeID gave both one id and the store kept one node.
// A file and a start byte identify one declaration, so ids no longer collide.
// Cross-snapshot identity is the canonical id, not the node id.
func NodeID(lang, file, qname string, startByte int) string {
	return hashID(fmt.Sprintf("%s|%s|%s|%d", lang, file, qname, startByte))
}

func hashID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:16]
}
