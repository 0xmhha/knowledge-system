//go:build !darwin || !cgo || !tokenizers

package coreml

import (
	"strings"
	"testing"
)

func TestUnavailableNativeProfileDoesNotPretendToOpen(t *testing.T) {
	if Available() {
		t.Fatal("native profile incorrectly advertised")
	}
	a, err := Open(Options{ModelPath: "unused.mlmodelc", TokenizerPath: "unused.json", ModelName: "bge-m3", Dim: 1024})
	if a != nil || err == nil || !strings.Contains(err.Error(), "CGO") {
		t.Fatal(a, err)
	}
}
