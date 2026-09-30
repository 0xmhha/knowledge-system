package setup

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// VerifyEngineInputs checks that every CKG-discovered file and every CKV
// in-tree code/document citation came from the retained source inventory.
// The candidate engines run exclusively on the staged copy; this independent
// comparison prevents a parser/discovery path from escaping that inventory.
// External reviewed corpora carry their own input digest and are not selected
// by the committed-source query below.
func VerifyEngineInputs(versionDir string, captured CapturedSource) error {
	if captured.Identity.SourceMode == "" {
		return fmt.Errorf("engine input reconciliation requires pinned source mode")
	}
	files := make(map[string]string, len(captured.Files))
	for _, f := range captured.Files {
		files[f.Path] = f.SHA256
	}
	var graph struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := readJSON(filepath.Join(versionDir, "graph", "manifest.json"), &graph); err != nil {
		return fmt.Errorf("reconcile graph inputs: %w", err)
	}
	for _, f := range graph.Files {
		if files[f.Path] != f.SHA256 || f.SHA256 == "" {
			return fmt.Errorf("reconcile graph input %q differs from retained source", f.Path)
		}
	}
	dbPath := filepath.Join(versionDir, "vector", "vector.db")
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("reconcile vector inputs: %w", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT DISTINCT file FROM chunks
		WHERE commit_hash = ? AND chunk_kind IN ('symbol','function_split','file_header','doc')`,
		captured.Identity.SourceCommit)
	if err != nil {
		return fmt.Errorf("reconcile vector source files: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return err
		}
		if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path ||
			path == ".." || strings.HasPrefix(path, "../") || files[path] == "" {
			return fmt.Errorf("reconcile vector input %q is absent from retained source", path)
		}
	}
	return rows.Err()
}
