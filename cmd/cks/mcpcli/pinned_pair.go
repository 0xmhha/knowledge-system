package mcpcli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

// A versioned pair must mount one archive. Falling back to legacy wiring when
// its paths differ bypasses the retained-source mode guard on direct tools.
// Legacy independent indexes keep their diagnostic alignment behavior.
func validatePinnedBackendPair(cfg *config.Config) error {
	if cfg.Backends.CKG.Path == "" || cfg.Backends.CKV.Path == "" {
		return nil
	}
	graphPath, graphVersion, err := pinnedIndexVersion(cfg.Backends.CKG.Path)
	if err != nil {
		return err
	}
	vectorPath, vectorVersion, err := pinnedIndexVersion(cfg.Backends.CKV.Path)
	if err != nil {
		return err
	}
	if graphVersion == "" && vectorVersion == "" {
		return nil
	}
	if graphVersion == "" || graphVersion != vectorVersion ||
		graphPath != filepath.Join(graphVersion, "graph", "graph.db") ||
		vectorPath != filepath.Join(graphVersion, "vector") {
		return fmt.Errorf("pinned graph/vector must use the canonical paths of one retained version")
	}
	return nil
}

func pinnedIndexVersion(indexPath string) (string, string, error) {
	absolute, err := filepath.Abs(indexPath)
	if err != nil {
		return "", "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	for dir := filepath.Dir(absolute); ; dir = filepath.Dir(dir) {
		info, statErr := os.Lstat(filepath.Join(dir, "dataset-identity.json"))
		if statErr == nil {
			if !info.Mode().IsRegular() {
				return "", "", fmt.Errorf("retained version identity must be a regular file")
			}
			return absolute, dir, nil
		}
		if !os.IsNotExist(statErr) {
			return "", "", fmt.Errorf("inspect retained version identity: %w", statErr)
		}
		if filepath.Dir(dir) == dir {
			return absolute, "", nil
		}
	}
}
