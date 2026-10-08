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
	if err != nil || loaded.Src != result.SourceRoot || loaded.Out != result.Dataset || loaded.Embedder != "mock" || loaded.ProjectID != result.ProjectID || !strings.HasPrefix(result.ProjectID, "p-") {
		t.Fatalf("setup config not loadable: %+v, %v", loaded, err)
	}
	second, err := Init(root, filepath.Join(other, "datasets", "project-b"), filepath.Join(other, "configs", "second.yaml"), "mock", "")
	if err != nil || second.ProjectID == result.ProjectID {
		t.Fatalf("same source name reused project ID: %+v, %v", second, err)
	}
	explicit, err := InitWithProjectID(root, filepath.Join(other, "datasets", "project-c"), filepath.Join(other, "configs", "explicit.yaml"), "mock", "", "team.alpha")
	if err != nil || explicit.ProjectID != "team.alpha" {
		t.Fatalf("explicit project ID: %+v, %v", explicit, err)
	}
	if _, err := InitWithProjectID(root, dataset, filepath.Join(other, "invalid-id.yaml"), "mock", "", "../escape"); err == nil {
		t.Fatal("unsafe project ID accepted")
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

func TestInitNonGitDefaultsToSnapshotOnly(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Init(root, filepath.Join(other, "dataset"), filepath.Join(other, "setup.yaml"), "mock", "")
	if err != nil || result.SourceMode != "snapshot-only" {
		t.Fatalf("non-Git init: %+v %v", result, err)
	}
	loaded, err := setup.LoadConfig(result.ConfigPath)
	if err != nil || loaded.SourceMode != "snapshot-only" || loaded.ProjectID != result.ProjectID {
		t.Fatalf("non-Git config lost mode: %+v %v", loaded, err)
	}
}
