package coreml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModelContentDigestBindsBytesNotLocation(t *testing.T) {
	makeBundle := func() (string, string) {
		t.Helper()
		root := t.TempDir()
		model := filepath.Join(root, "model.mlmodelc")
		if err := os.Mkdir(model, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(model, "weights.bin"), []byte("owned test bytes, not a model"), 0600); err != nil {
			t.Fatal(err)
		}
		tok := filepath.Join(root, "tokenizer.json")
		if err := os.WriteFile(tok, []byte(`{"vocab":"fixture"}`), 0600); err != nil {
			t.Fatal(err)
		}
		return model, tok
	}
	model, tok := makeBundle()
	other, otherTok := makeBundle()
	first, err := modelContentDigest(model, tok)
	if err != nil {
		t.Fatal(err)
	}
	same, err := modelContentDigest(other, otherTok)
	if err != nil || first != same || len(first) != 64 {
		t.Fatal(first, same, err)
	}
	if err := os.WriteFile(filepath.Join(model, "weights.bin"), []byte("different bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := modelContentDigest(model, tok)
	if err != nil || changed == first {
		t.Fatal(changed, err)
	}
	if err := os.WriteFile(otherTok, []byte(`{"vocab":"changed"}`), 0600); err != nil {
		t.Fatal(err)
	}
	tokenChanged, err := modelContentDigest(other, otherTok)
	if err != nil || tokenChanged == same {
		t.Fatal(tokenChanged, err)
	}
}

func TestModelContentDigestRejectsGGUFAndUnavailableAssets(t *testing.T) {
	root := t.TempDir()
	tok := filepath.Join(root, "tokenizer.json")
	if err := os.WriteFile(tok, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{filepath.Join(root, "model.gguf"), filepath.Join(root, "missing.mlpackage")} {
		if _, err := modelContentDigest(model, tok); err == nil {
			t.Fatal("unavailable model identity accepted")
		}
	}
	empty := filepath.Join(root, "empty.mlmodelc")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := modelContentDigest(empty, tok); err == nil {
		t.Fatal("empty model accepted")
	}
}
