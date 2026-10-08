package setup

import (
	"fmt"
	"os"
	"path/filepath"
)

// MaterializeRetainedReadTree makes a private, disposable plain source tree
// for retrieval. It has no Git metadata: history remains in the graph index,
// while citation checks and body fetches use exactly the archived bytes.
// The returned cleanup must be called when the server exits.
func MaterializeRetainedReadTree(versionDir string) (string, func(), error) {
	identity, err := InspectVersionIdentity(versionDir)
	if err != nil {
		return "", nil, err
	}
	if identity == nil {
		return "", nil, fmt.Errorf("reindex_required: no pinned source archive")
	}
	sources := filepath.Join(versionDir, "sources")
	if info, err := os.Lstat(filepath.Join(sources, "manifest.json")); err != nil || !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("source_missing: retained source manifest is unavailable")
	}
	var captured CapturedSource
	if err := readJSON(filepath.Join(sources, "manifest.json"), &captured); err != nil || captured.Identity != identity.Source {
		return "", nil, fmt.Errorf("snapshot_mismatch: retained source manifest differs from dataset")
	}
	captured.BlobDir = filepath.Join(sources, "blobs")
	root, err := os.MkdirTemp("", "cks-read-source-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	for _, file := range captured.Files {
		if file.OriginID != "repo" {
			continue
		}
		buf, err := captured.ReadBlob(file.SHA256)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		if int64(len(buf)) != file.Size {
			cleanup()
			return "", nil, fmt.Errorf("snapshot_mismatch: retained source size changed")
		}
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			cleanup()
			return "", nil, err
		}
		if err := os.WriteFile(path, buf, 0o600); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	return root, cleanup, nil
}
