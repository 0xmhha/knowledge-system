package evalcli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/eval"
	"gopkg.in/yaml.v3"
)

func TestMatrixEvaluationBindingBlocksBeforeSessionOrOutput(t *testing.T) {
	for _, mode := range []string{"valid", "source", "dataset", "config", "binary", "model", "K", "duplicate", "missing", "unknown", "changed-binding"} {
		t.Run(mode, func(t *testing.T) {
			input, cfg, binary, out := matrixTestFiles(t)
			c, err := config.Load(cfg)
			if err != nil {
				t.Fatal(err)
			}
			c.Retrieval.RecallK = 10
			configRaw, err := yaml.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cfg, configRaw, 0600); err != nil {
				t.Fatal(err)
			}
			cfgSHA, _ := shaFile(cfg)
			binSHA, _ := shaFile(binary)
			b := matrixEvaluationBinding{SchemaVersion: 1, DatasetID: "test-only", SourceCommit: strings.Repeat("a", 40), ConfigSHA256: cfgSHA, BinarySHA256: binSHA, Provider: "ollama", Model: "bge-m3:latest", ModelDigest: strings.Repeat("a", 64), Dimension: 1024, Context: 8192, Batch: 8192, RetrievalK: 10}
			switch mode {
			case "source":
				b.SourceCommit = strings.Repeat("b", 40)
			case "dataset":
				b.DatasetID = "wrong"
			case "config":
				b.ConfigSHA256 = strings.Repeat("b", 64)
			case "binary":
				b.BinarySHA256 = strings.Repeat("b", 64)
			case "model":
				b.Dimension = 3
			case "K":
				b.RetrievalK = 99
			case "missing":
				b.ModelDigest = ""
			}
			path := filepath.Join(filepath.Dir(input), "binding.json")
			raw, _ := json.Marshal(b)
			if mode == "duplicate" {
				raw = []byte(strings.TrimSuffix(string(raw), "}") + `,"dimension":1024}`)
			}
			if mode == "unknown" {
				raw = []byte(strings.TrimSuffix(string(raw), "}") + `,"approved":1}`)
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			reader := func(c *config.Config, cp, ip, bp string, extra []string) (map[string]string, *setup.DatasetIdentity, error) {
				locked, id, err := matrixTestInputReader(c, cp, ip, bp, extra)
				id.Source.SourceCommit = strings.Repeat("a", 40)
				id.EmbeddingIdentity = json.RawMessage(`{"Provider":"ollama","Model":"bge-m3:latest","Dim":1024,"model_digest":"` + strings.Repeat("a", 64) + `","runtime_context_tokens":8192,"runtime_batch_tokens":8192}`)
				if mode == "changed-binding" {
					locked[path] = strings.Repeat("b", 64)
				}
				return locked, id, err
			}
			starts := 0
			// No session creation is necessary to prove the input boundary: failed
			// binding propagates before mkdir/output and before every factory call.
			f := func(context.Context, string, string, time.Duration) (matrixSession, error) {
				starts++
				return &matrixFake{capture: func(_ eval.CaptureRequest) error { return nil }}, nil
			}
			err = matrixRunWithInputs(context.Background(), input, cfg, binary, out, 0, 1, 0, 0, time.Second, []string{path}, f, bindMatrixInputs(path, reader))
			if mode == "valid" {
				if err != nil || starts != 8 {
					t.Fatalf("valid: starts=%d err=%v", starts, err)
				}
			} else {
				if err == nil || starts != 0 {
					t.Fatalf("invalid binding dispatched: starts=%d err=%v", starts, err)
				}
				if _, err := os.Stat(out); !os.IsNotExist(err) {
					t.Fatalf("output created for rejected binding: %v", err)
				}
			}
		})
	}
}

func TestMatrixBindingCLIRequiresEnvironmentBeforeRead(t *testing.T) {
	cmd := newMatrixCmd()
	cmd.SetArgs([]string{"--evaluation-binding", "missing.json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "requires --environment-ledger") {
		t.Fatal(err)
	}
}
