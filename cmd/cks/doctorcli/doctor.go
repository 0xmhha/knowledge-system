// Package doctorcli reports source and dataset readiness without modifying
// a project. It uses manifests and filesystem metadata, not engine internals.
package doctorcli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

type Report struct {
	SourceRoot       string         `json:"source_root"`
	Commit           string         `json:"commit"`
	Dirty            bool           `json:"dirty"`
	Languages        map[string]int `json:"languages"`
	SharedCodeFiles  int            `json:"shared_code_files"`
	GraphOnlyProto   int            `json:"graph_only_proto"`
	SymlinksSkipped  int            `json:"symlinks_skipped"`
	UnreadableFiles  int            `json:"unreadable_files"`
	SecretPathCount  int            `json:"secret_path_count"`
	IgnoredIndexable int            `json:"ignored_indexable"`
	DatasetVersion   string         `json:"dataset_version,omitempty"`
	Issues           []string       `json:"issues"`
	Status           string         `json:"status"`
}

func NewCmd() *cobra.Command {
	var src, dataset string
	var strict bool
	cmd := &cobra.Command{Use: "doctor", Short: "Inspect a project and its active graph/vector dataset",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := Inspect(src, dataset)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(report); err != nil {
				return err
			}
			if strict && report.Status != "ready" {
				return fmt.Errorf("doctor: %s (%d issues)", report.Status, len(report.Issues))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&src, "src", "", "Git source repository to inspect")
	cmd.Flags().StringVar(&dataset, "dataset", "", "dataset root with a current pointer (optional)")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit nonzero unless status is ready")
	_ = cmd.MarkFlagRequired("src")
	return cmd
}

// Inspect is read-only. Counts describe filenames the current engines can
// plausibly ingest; the actual build may apply additional project filters.
func Inspect(src, dataset string) (Report, error) {
	root, err := filepath.Abs(src)
	if err != nil {
		return Report{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Report{}, fmt.Errorf("doctor: source root: %w", err)
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return Report{}, fmt.Errorf("doctor: source root is not a directory: %s", root)
	}
	report := Report{SourceRoot: root, Languages: map[string]int{}, Issues: []string{}, Status: "ready"}
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	gitRoot, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return Report{}, fmt.Errorf("doctor: source is not a Git repository: %w", err)
	}
	gitRoot, err = filepath.EvalSymlinks(gitRoot)
	if err != nil {
		return Report{}, err
	}
	if gitRoot != root {
		return Report{}, fmt.Errorf("doctor: --src must name the Git root (%s)", gitRoot)
	}
	report.Commit, err = git("rev-parse", "HEAD")
	if err != nil {
		return Report{}, fmt.Errorf("doctor: read HEAD: %w", err)
	}
	status, err := git("status", "--porcelain")
	if err != nil {
		return Report{}, fmt.Errorf("doctor: read working-tree status: %w", err)
	}
	report.Dirty = status != ""
	if report.Dirty {
		report.Issues = append(report.Issues, "working tree has tracked or untracked changes; commit-based snapshot naming is unsafe")
	}
	report.IgnoredIndexable, err = setup.CountIgnoredIndexable(root)
	if err != nil {
		return Report{}, err
	}
	if report.IgnoredIndexable > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("%d Git-ignored source files may still be indexed; review .ckvignore", report.IgnoredIndexable))
	}
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			report.UnreadableFiles++
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			report.SymlinksSkipped++
			return nil
		}
		if entry.IsDir() {
			if rel == entry.Name() {
				switch entry.Name() {
				case ".git", "node_modules", "vendor", ".next", "dist", "build", "out", ".venv", "target", "__pycache__":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if secretPath(rel) {
			report.SecretPathCount++
			return nil
		}
		language := languageOf(rel)
		if language == "" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			report.UnreadableFiles++
			return nil
		}
		var head [8192]byte
		n, readErr := file.Read(head[:])
		file.Close()
		if readErr != nil && readErr != io.EOF {
			report.UnreadableFiles++
			return nil
		}
		for _, b := range head[:n] {
			if b == 0 {
				return nil
			}
		}
		if language == "proto" {
			report.GraphOnlyProto++
		} else {
			report.Languages[language]++
		}
		return nil
	})
	if walkErr != nil {
		return Report{}, walkErr
	}
	if report.UnreadableFiles > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("%d source paths were unreadable", report.UnreadableFiles))
	}
	if report.SecretPathCount > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("%d sensitive-looking paths found; verify both engine filters before indexing", report.SecretPathCount))
	}
	if len(report.Languages) == 0 {
		report.Issues = append(report.Issues, "no CKV-supported source or Markdown files detected; graph-only proto files may still be available")
	}
	for language, count := range report.Languages {
		if language != "markdown" {
			report.SharedCodeFiles += count
		}
	}
	if report.SharedCodeFiles == 0 {
		report.Issues = append(report.Issues, "no code files supported by both graph and vector engines; indexing is document-only or graph-only")
	}
	if dataset != "" {
		inspectDataset(&report, dataset)
	}
	if len(report.Issues) > 0 {
		report.Status = "degraded"
	}
	return report, nil
}

func inspectDataset(report *Report, dataset string) {
	root, err := filepath.Abs(dataset)
	if err != nil {
		report.Issues = append(report.Issues, "dataset path is invalid")
		return
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		report.Issues = append(report.Issues, "dataset root is unavailable")
		return
	}
	current := filepath.Join(root, "current")
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		report.Issues = append(report.Issues, "no active dataset; run cks setup")
		return
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		report.Issues = append(report.Issues, "active dataset pointer escapes the dataset root")
		return
	}
	report.DatasetVersion = rel
	graph, vector := filepath.Join(resolved, "graph"), filepath.Join(resolved, "vector")
	if err := setup.VerifyAlignment(graph, vector, nil); err != nil {
		report.Issues = append(report.Issues, "active graph/vector alignment failed: "+err.Error())
		return
	}
	for _, path := range []string{filepath.Join(graph, "manifest.json"), filepath.Join(vector, "manifest.json")} {
		var m struct {
			SrcRoot   string `json:"src_root"`
			SrcCommit string `json:"src_commit"`
		}
		buf, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(buf, &m) != nil {
			report.Issues = append(report.Issues, "active manifest unreadable: "+path)
			continue
		}
		indexedRoot, err := filepath.EvalSymlinks(m.SrcRoot)
		if err != nil || indexedRoot != report.SourceRoot {
			report.Issues = append(report.Issues, "active index belongs to a different source root")
		}
		if m.SrcCommit != report.Commit {
			report.Issues = append(report.Issues, "active index was built from a different source commit")
		}
	}
}

func languageOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".sol":
		return "solidity"
	case ".md", ".markdown":
		return "markdown"
	case ".proto":
		return "proto"
	default:
		return ""
	}
}

func secretPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if base == ".env" || strings.HasPrefix(base, ".env.") || base == "credentials.json" ||
		base == ".npmrc" || base == ".netrc" || strings.HasPrefix(base, "id_rsa") ||
		strings.HasPrefix(base, "id_ed25519") {
		return true
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".pem", ".key", ".p12", ".pfx", ".keystore":
		return true
	}
	return false
}
