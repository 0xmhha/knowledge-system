package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var corpusDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// VerifyTextCorpus checks a separately rendered semantic Markdown corpus
// before CKV embeds it. It rejects stale commits, foreign repositories,
// symlinks, missing/extra Markdown, and changed bytes.
func VerifyTextCorpus(corpusDir, graphDir string) error {
	var graph struct {
		SrcRoot   string `json:"src_root"`
		SrcCommit string `json:"src_commit"`
	}
	if err := readJSON(filepath.Join(graphDir, "manifest.json"), &graph); err != nil {
		return err
	}
	var corpus struct {
		SchemaVersion int `json:"schema_version"`
		Snapshot      struct {
			Commit string `json:"commit"`
		} `json:"snapshot"`
		SourceRoot       string `json:"source_root"`
		ProjectionSHA256 string `json:"projection_sha256"`
		Files            []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := readJSON(filepath.Join(corpusDir, "manifest.json"), &corpus); err != nil {
		return err
	}
	if corpus.SchemaVersion != 1 || !corpusDigest.MatchString(corpus.ProjectionSHA256) {
		return fmt.Errorf("semantic corpus manifest version or projection digest is invalid")
	}
	if corpus.Snapshot.Commit == "" || corpus.Snapshot.Commit != graph.SrcCommit {
		return fmt.Errorf("semantic corpus commit does not match candidate graph")
	}
	corpusRoot, err := filepath.EvalSymlinks(corpus.SourceRoot)
	if err != nil {
		return err
	}
	graphRoot, err := filepath.EvalSymlinks(graph.SrcRoot)
	if err != nil {
		return err
	}
	if corpusRoot != graphRoot {
		return fmt.Errorf("semantic corpus belongs to a different source root")
	}
	if len(corpus.Files) == 0 {
		return fmt.Errorf("semantic corpus has no reviewed Markdown")
	}
	seen := map[string]bool{}
	for _, file := range corpus.Files {
		if file.Path == "" || filepath.Base(file.Path) != file.Path || !strings.HasSuffix(file.Path, ".md") ||
			seen[file.Path] || !corpusDigest.MatchString(file.SHA256) {
			return fmt.Errorf("semantic corpus has an unsafe or duplicate file entry")
		}
		seen[file.Path] = true
		path := filepath.Join(corpusDir, file.Path)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("semantic corpus entry is not a regular file: %s", file.Path)
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(bytes)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			return fmt.Errorf("semantic corpus content digest mismatch: %s", file.Path)
		}
	}
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") && !seen[entry.Name()] {
			return fmt.Errorf("unmanifested Markdown in semantic corpus: %s", entry.Name())
		}
	}
	return nil
}
