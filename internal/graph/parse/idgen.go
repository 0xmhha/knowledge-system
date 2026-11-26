package parse

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// MakeID returns a stable 16-char hash for (qname, language, startByte).
// It has no file in its key, so it is only safe for a node whose qualified
// name already tells files apart (a File node, `file:<path>`). The Go and
// Solidity parsers key declarations with NodeID; the TypeScript and
// proto parsers still key declarations with MakeID, and two of their files
// that declare the same name at the same byte offset share one id.
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
