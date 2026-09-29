package setup

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CountIgnoredIndexable is a conservative guard for commit-named datasets.
// CKV has its own .ckvignore rules rather than adopting .gitignore, so a Git
// ignored source can still be indexed while the Git status looks clean. This
// preview excludes only the directory classes both engines skip by default.
func CountIgnoredIndexable(root string) (int, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "--others", "--ignored", "--exclude-standard", "-z")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("inspect Git-ignored sources: %w", err)
	}
	count := 0
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		path := filepath.ToSlash(string(raw))
		if defaultIgnoredDir(path) {
			continue
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".sol", ".md", ".markdown", ".proto":
			count++
		}
	}
	return count, nil
}

func defaultIgnoredDir(path string) bool {
	part := strings.SplitN(path, "/", 2)[0]
	switch part {
	case ".git", "node_modules", "vendor", ".next", "out", "dist", "build", "target", ".venv", "__pycache__":
		return true
	}
	return false
}
