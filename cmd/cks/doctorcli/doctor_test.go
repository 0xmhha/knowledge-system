package doctorcli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func doctorGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func doctorRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	doctorGit(t, root, "init", "-q")
	for name, body := range map[string]string{"main.go": "package sample\n", "schema.proto": "syntax = \"proto3\";\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	doctorGit(t, root, "add", ".")
	doctorGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	return root, doctorGit(t, root, "rev-parse", "HEAD")
}

func writeDoctorManifest(t *testing.T, path string, data any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	buf, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInspectSourceAndActiveDatasetIsolation(t *testing.T) {
	root, commit := doctorRepo(t)
	r, err := Inspect(root, "")
	if err != nil || r.Status != "ready" || r.Languages["go"] != 1 || r.GraphOnlyProto != 1 || r.Dirty {
		t.Fatalf("clean source report: %+v, %v", r, err)
	}
	dataset := t.TempDir()
	version := filepath.Join(dataset, "v1")
	digest := strings.Repeat("a", 64)
	graphManifest := map[string]any{"schema_version": "1.23", "src_root": root, "src_commit": commit, "graph_digest": digest}
	vectorManifest := map[string]any{"src_root": root, "src_commit": commit,
		"sources": map[string]any{"ckg": map[string]any{"src_commit": commit, "graph_digest": digest}}}
	writeDoctorManifest(t, filepath.Join(version, "graph", "manifest.json"), graphManifest)
	writeDoctorManifest(t, filepath.Join(version, "vector", "manifest.json"), vectorManifest)
	if err := os.Symlink("v1", filepath.Join(dataset, "current")); err != nil {
		t.Fatal(err)
	}
	r, err = Inspect(root, dataset)
	if err != nil || r.Status != "ready" || r.DatasetVersion != "v1" {
		t.Fatalf("aligned dataset: %+v, %v", r, err)
	}
	vectorManifest["src_root"] = t.TempDir()
	writeDoctorManifest(t, filepath.Join(version, "vector", "manifest.json"), vectorManifest)
	r, err = Inspect(root, dataset)
	if err != nil || r.Status != "degraded" || len(r.Issues) == 0 {
		t.Fatalf("foreign dataset accepted: %+v, %v", r, err)
	}
}

func TestDoctorReportsDirtySecretsAndSymlinksWithoutReadingTheirContents(t *testing.T) {
	root, _ := doctorRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=do-not-print\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("main.go", filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}
	r, err := Inspect(root, "")
	if err != nil || r.Status != "degraded" || !r.Dirty || r.SecretPathCount != 1 || r.SymlinksSkipped != 1 {
		t.Fatalf("dirty source report: %+v, %v", r, err)
	}
	cmd := NewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--src", root, "--strict"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("strict doctor accepted degraded source")
	}
	if strings.Contains(out.String(), "do-not-print") {
		t.Fatal("doctor exposed secret contents")
	}
}

func TestLanguageCapabilityPreview(t *testing.T) {
	for file, want := range map[string]string{"a.go": "go", "x.tsx": "typescript", "x.jsx": "javascript", "x.sol": "solidity", "x.md": "markdown", "x.proto": "proto", "x.py": ""} {
		if got := languageOf(file); got != want {
			t.Errorf("%s: got %q, want %q", file, got, want)
		}
	}
}

func TestDoctorLabelsDocumentOnlyRepositoryAsDegraded(t *testing.T) {
	root := t.TempDir()
	doctorGit(t, root, "init", "-q")
	for name, contents := range map[string]string{"README.md": "# Sample\n", "main.py": "print('sample')\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	doctorGit(t, root, "add", ".")
	doctorGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	report, err := Inspect(root, "")
	if err != nil || report.Status != "degraded" || report.SharedCodeFiles != 0 || report.Languages["markdown"] != 1 {
		t.Fatalf("unsupported code capacity overstated: %+v, %v", report, err)
	}
}

func TestDoctorFlagsGitIgnoredIndexableFiles(t *testing.T) {
	root, _ := doctorRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doctorGit(t, root, "add", ".gitignore")
	doctorGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "ignore")
	if err := os.WriteFile(filepath.Join(root, "ignored.md"), []byte("# Hidden from Git only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Inspect(root, "")
	if err != nil || r.Dirty || r.IgnoredIndexable != 1 || r.Status != "degraded" {
		t.Fatalf("ignored source risk missed: %+v, %v", r, err)
	}
}
