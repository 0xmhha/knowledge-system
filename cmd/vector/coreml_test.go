package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func coreMLCLIFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "model.mlmodelc"), 0700); err != nil {
		t.Fatal(err)
	}
	tok := filepath.Join(root, "tokenizer.json")
	if err := os.WriteFile(tok, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	return root, tok
}
func TestCoreMLOptionsExplicitProfileAndPaths(t *testing.T) {
	root, tok := coreMLCLIFixture(t)
	for _, modelDir := range []string{root, filepath.Join(root, "model.mlmodelc")} {
		opts, err := coreMLOptions(modelDir, "bge-m3", "", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if opts.ModelPath != filepath.Join(root, "model.mlmodelc") || opts.TokenizerPath != tok || opts.ModelName != "bge-m3" || opts.Dim != 1024 || opts.MaxSeqLen != 512 {
			t.Fatal(opts)
		}
	}
	opts, err := coreMLOptions(root, "bge-m3", tok, 256, 1024)
	if err != nil || opts.MaxSeqLen != 256 {
		t.Fatal(opts, err)
	}
}
func TestCoreMLOptionsRejectUnsupportedModelAndAmbiguousInputs(t *testing.T) {
	root, _ := coreMLCLIFixture(t)
	for _, model := range []string{"", "qwen3-embedding:0.6b", "qwen3-embedding:4b", "embeddinggemma-300m", "not-registered"} {
		if _, err := coreMLOptions(root, model, "", 512, 0); err == nil {
			t.Fatal("unsupported profile accepted", model)
		}
	}
	for _, pair := range [][2]int{{-1, 0}, {9000, 0}, {512, 512}, {512, -1}} {
		if _, err := coreMLOptions(root, "bge-m3", "", pair[0], pair[1]); err == nil {
			t.Fatal("invalid dimension/context accepted", pair)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "model.mlpackage"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := coreMLOptions(root, "bge-m3", "", 512, 0); err == nil {
		t.Fatal("ambiguous model accepted")
	}
}
func TestCoreMLBackendDispatchIsRecognizedWithoutModel(t *testing.T) {
	saved := globalFlags
	t.Cleanup(func() { globalFlags = saved })
	globalFlags.embeddingCacheEntries = 0
	emb, closeFn, err := resolveEmbedder("coreml", "")
	if closeFn != nil {
		closeFn()
	}
	if err == nil || emb != nil || strings.Contains(err.Error(), "unknown --embedder") {
		t.Fatal("coreml dispatch missing", emb, err)
	}
}
func TestCoreMLCLIHelpContainsOptInControls(t *testing.T) {
	saved := globalFlags
	t.Cleanup(func() { globalFlags = saved })
	cmd := newRootCmd()
	if cmd.PersistentFlags().Lookup("coreml-tokenizer") == nil || cmd.PersistentFlags().Lookup("coreml-max-seq") == nil {
		t.Fatal("CoreML options not registered")
	}
}
