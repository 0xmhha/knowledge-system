package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/vector/embed/coreml"
	"github.com/0xmhha/knowledge-system/internal/vector/embed/registry"
)

// Resolve only the explicitly supported two-input hidden-state/CLS profile.
// Known Ollama-only Qwen entries do not acquire CoreML compatibility by name.
func coreMLOptions(modelDir, modelName, tokenizer string, maxSeq, dimension int) (coreml.Options, error) {
	var opts coreml.Options
	if modelDir == "" || modelName == "" {
		return opts, fmt.Errorf("coreml: --model-dir and --model-name are required")
	}
	cfg, err := registry.Lookup(modelName)
	if err != nil {
		return opts, err
	}
	if cfg.OnnxFile == "" || cfg.Pooling != registry.PoolingCLS {
		return opts, fmt.Errorf("coreml: model %q is not supported by the hidden-state/CLS profile", modelName)
	}
	if dimension < 0 || dimension > 0 && dimension != cfg.Dim {
		return opts, fmt.Errorf("coreml: --embed-dim must equal the model native dimension %d", cfg.Dim)
	}
	if maxSeq < 0 {
		return opts, fmt.Errorf("coreml: --coreml-max-seq must be positive")
	}
	if maxSeq == 0 {
		maxSeq = 512
	}
	if maxSeq > cfg.MaxInput {
		return opts, fmt.Errorf("coreml: sequence limit exceeds registered context")
	}
	modelPath, err := filepath.Abs(modelDir)
	if err != nil {
		return opts, err
	}
	base := modelPath
	if ext := filepath.Ext(modelPath); ext == ".mlpackage" || ext == ".mlmodelc" {
		base = filepath.Dir(modelPath)
	} else {
		found := []string{}
		for _, name := range []string{"model.mlmodelc", "model.mlpackage"} {
			p := filepath.Join(base, name)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				found = append(found, p)
			}
		}
		if len(found) != 1 {
			return opts, fmt.Errorf("coreml: --model-dir must identify exactly one model.mlmodelc or model.mlpackage")
		}
		modelPath = found[0]
	}
	if info, err := os.Stat(modelPath); err != nil || !info.IsDir() {
		return opts, fmt.Errorf("coreml: model directory is unavailable")
	}
	if tokenizer == "" {
		tokenizer = filepath.Join(base, cfg.TokenizerFile)
	}
	tokenizer, err = filepath.Abs(tokenizer)
	if err != nil {
		return opts, err
	}
	if info, err := os.Stat(tokenizer); err != nil || !info.Mode().IsRegular() {
		return opts, fmt.Errorf("coreml: compatible tokenizer.json is required")
	}
	return coreml.Options{ModelPath: modelPath, TokenizerPath: tokenizer, ModelName: cfg.Name, Dim: cfg.Dim, MaxSeqLen: maxSeq}, nil
}
