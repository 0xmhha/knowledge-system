package mcpcli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

func TestRunGenConfigPreservesExistingUserFile(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "cks.yaml")
	want := []byte("user: chosen-settings\n")
	if err := os.WriteFile(out, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runGenConfig([]string{"--out", out, "--dataset-dir", "/data/new"}, io.Discard); err == nil {
		t.Fatal("generated config replaced an existing user file")
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != string(want) {
		t.Fatalf("existing config changed: %q %v", got, err)
	}
	link := filepath.Join(root, "linked.yaml")
	if err := os.Symlink(out, link); err != nil {
		t.Fatal(err)
	}
	if err := runGenConfig([]string{"--out", link, "--dataset-dir", "/data/new"}, io.Discard); err == nil {
		t.Fatal("generated config followed a destination symlink")
	}
	got, err = os.ReadFile(out)
	if err != nil || string(got) != string(want) {
		t.Fatalf("symlink target changed: %q %v", got, err)
	}
}

func TestRunGenConfig_WritesLoadableConfig(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "cks.yaml")
	args := []string{
		"--out", out,
		"--name", "cks-example",
		"--dataset-dir", "/data/pr-77",
		"--source-root", "/src/example-project",
		"--sanitize-rules", "/policies/sanitization_rules.yaml",
		"--semantic-store", "/data/semantic.db",
	}
	if err := runGenConfig(args, io.Discard); err != nil {
		t.Fatalf("runGenConfig: %v", err)
	}

	cfg, err := config.Load(out)
	if err != nil {
		t.Fatalf("Load generated config: %v", err)
	}
	if cfg.Name != "cks-example" {
		t.Errorf("Name = %q, want cks-example", cfg.Name)
	}
	wantCKG := filepath.Join("/data/pr-77", "graph", "graph.db")
	if cfg.Backends.CKG.Path != wantCKG {
		t.Errorf("CKG.Path = %q, want %q", cfg.Backends.CKG.Path, wantCKG)
	}
	if cfg.Backends.CKV.Path != filepath.Join("/data/pr-77", "vector") {
		t.Errorf("CKV.Path = %q", cfg.Backends.CKV.Path)
	}
	if cfg.Listen.Transport != "http" {
		t.Errorf("Transport = %q, want http", cfg.Listen.Transport)
	}
	if cfg.Semantic.StorePath != "/data/semantic.db" {
		t.Errorf("Semantic.StorePath = %q", cfg.Semantic.StorePath)
	}
}

func TestRunGenConfig_RequiresOut(t *testing.T) {
	t.Parallel()
	if err := runGenConfig([]string{"--dataset-dir", "/d"}, io.Discard); err == nil {
		t.Fatal("expected error when -out is omitted")
	}
}

func TestRunGenConfig_PortBindsLoopback(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "cks.yaml")
	args := []string{
		"--out", out,
		"--dataset-dir", "/data/ds",
		"--port", "8930",
	}
	if err := runGenConfig(args, io.Discard); err != nil {
		t.Fatalf("runGenConfig: %v", err)
	}
	cfg, err := config.Load(out)
	if err != nil {
		t.Fatalf("Load generated config: %v", err)
	}
	if cfg.Listen.HTTPAddr != "127.0.0.1:8930" {
		t.Errorf("HTTPAddr = %q, want 127.0.0.1:8930", cfg.Listen.HTTPAddr)
	}
	if cfg.Listen.AllowRemote {
		t.Error("AllowRemote = true, want false for a loopback bind")
	}
}

func TestRunGenConfig_PortAndHTTPAddrConflict(t *testing.T) {
	t.Parallel()
	args := []string{
		"--out", filepath.Join(t.TempDir(), "cks.yaml"),
		"--dataset-dir", "/data/ds",
		"--port", "8930",
		"--http-addr", "127.0.0.1:9000",
	}
	if err := runGenConfig(args, io.Discard); err == nil {
		t.Fatal("expected error when both -port and -http-addr are given")
	}
}

func TestRunGenConfigRefusesAnExistingOutputWithAPublicCode(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cks.yaml")
	args := []string{"--out", out, "--dataset-dir", "/data", "--sanitize-rules", "/policies/rules.yaml"}
	if err := runGenConfig(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runGenConfig(args, io.Discard); err == nil || !strings.Contains(err.Error(), "output_exists") {
		t.Fatalf("second gen-config = %v, want output_exists", err)
	}
}

// The default ruleset must not depend on the working directory: a config
// generated elsewhere used to name <cwd>/system/policies/..., which does not
// exist, and the server then refused to start.
func TestRunGenConfigLocatesTheBaselineBesideTheBinary(t *testing.T) {
	for _, layout := range []struct{ name, binary, rules string }{
		{"checkout", "bin/cks", "system/policies/sanitization_rules.yaml"},
		{"package", "cks", "policies/sanitization_rules.yaml"},
	} {
		t.Run(layout.name, func(t *testing.T) {
			root := t.TempDir()
			rules := filepath.Join(root, layout.rules)
			if err := os.MkdirAll(filepath.Dir(rules), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(rules, []byte("version: 1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			restore := executablePath
			executablePath = func() (string, error) { return filepath.Join(root, layout.binary), nil }
			t.Cleanup(func() { executablePath = restore })
			t.Chdir(t.TempDir()) // an unrelated working directory

			out := filepath.Join(t.TempDir(), "cks.yaml")
			if err := runGenConfig([]string{"--out", out, "--dataset-dir", "/data"}, io.Discard); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(out)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := filepath.EvalSymlinks(rules)
			got, _ := filepath.EvalSymlinks(cfg.Sanitize.RulesPath)
			if got != want {
				t.Fatalf("rules_path = %q, want %q", cfg.Sanitize.RulesPath, rules)
			}
		})
	}
}

func TestRunGenConfigFailsWhenNoBaselineIsFound(t *testing.T) {
	restore := executablePath
	executablePath = func() (string, error) { return filepath.Join(t.TempDir(), "cks"), nil }
	t.Cleanup(func() { executablePath = restore })
	t.Chdir(t.TempDir())
	out := filepath.Join(t.TempDir(), "cks.yaml")
	err := runGenConfig([]string{"--out", out, "--dataset-dir", "/data"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "config_invalid") {
		t.Fatalf("gen-config without a baseline = %v, want config_invalid", err)
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		t.Fatalf("a config naming a missing ruleset was written: %v", err)
	}
}

func TestGenConfigOntologyModesRoundTrip(t *testing.T) {
	out := filepath.Join(t.TempDir(), "ontology.yaml")
	args := []string{"--out", out, "--dataset-dir", "/fixture/version", "--source-root", "/fixture/source", "--sanitize-rules", "/fixture/rules.yaml", "--semantic-store", "/fixture/meaning.sqlite", "--ontology-mode", "combined", "--ontology-budget-ms", "5000"}
	if err := runGenConfig(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Semantic.OntologyMode != "combined" || cfg.Semantic.OntologyBudgetMS != 5000 || cfg.Semantic.StorePath != "/fixture/meaning.sqlite" {
		t.Fatalf("settings lost: %+v", cfg.Semantic)
	}
}

func TestGenConfigRejectsIncompleteOntologyWithoutWriting(t *testing.T) {
	for _, flags := range [][]string{{"--ontology-mode", "combined"}, {"--ontology-mode", "typo"}, {"--ontology-budget-ms", "0"}, {"--ontology-budget-ms", "5001"}} {
		out := filepath.Join(t.TempDir(), "new.yaml")
		args := append([]string{"--out", out, "--dataset-dir", "/fixture", "--sanitize-rules", "/fixture/rules.yaml"}, flags...)
		if err := runGenConfig(args, io.Discard); err == nil {
			t.Fatalf("invalid options accepted: %v", flags)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("invalid options wrote a config")
		}
	}
	out := filepath.Join(t.TempDir(), "off.yaml")
	if err := runGenConfig([]string{"--out", out, "--dataset-dir", "/fixture", "--sanitize-rules", "/fixture/rules.yaml"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Semantic.OntologyMode != "" {
		t.Fatal("default off changed the legacy raw-query-first setting")
	}
}
