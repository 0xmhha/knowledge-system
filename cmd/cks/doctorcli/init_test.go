package doctorcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

func TestInitWritesLoadableProjectConfigWithoutOverwrite(t *testing.T) {
	root, _ := doctorRepo(t)
	other := t.TempDir()
	dataset := filepath.Join(other, "datasets", "project-a")
	configPath := filepath.Join(other, "configs", "setup.yaml")
	result, err := Init(root, dataset, configPath, "mock", "")
	if err != nil || !strings.HasSuffix(result.Dataset, filepath.Join("datasets", "project-a")) {
		t.Fatalf("init: %+v, %v", result, err)
	}
	loaded, err := setup.LoadConfig(configPath)
	if err != nil || loaded.Src != result.SourceRoot || loaded.Out != result.Dataset || loaded.Embedder != "mock" {
		t.Fatalf("setup config not loadable: %+v, %v", loaded, err)
	}
	if _, err := Init(root, dataset, configPath, "mock", ""); err == nil || !strings.Contains(err.Error(), "without replacing") {
		t.Fatalf("existing config overwritten: %v", err)
	}
	if _, err := Init(root, filepath.Join(root, "data"), filepath.Join(other, "inside.yaml"), "mock", ""); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("dataset inside source accepted: %v", err)
	}
	if _, err := Init(root, dataset, filepath.Join(other, "real.yaml"), "ollama", ""); err == nil || !strings.Contains(err.Error(), "model-name") {
		t.Fatalf("unspecified real model accepted: %v", err)
	}
	if _, err := Init(root, dataset, filepath.Join(other, "invalid.yaml"), "unknown", "x"); err == nil {
		t.Fatal("unknown embedder accepted")
	}
}

func TestInitRejectsDatasetSymlinkIntoSource(t *testing.T) {
	root, _ := doctorRepo(t)
	other := t.TempDir()
	alias := filepath.Join(other, "source-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(root, filepath.Join(alias, "data"), filepath.Join(other, "setup.yaml"), "mock", ""); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("symlink into source accepted: %v", err)
	}
}
