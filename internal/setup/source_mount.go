package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// MaterializeRetainedReadTree makes a private, disposable plain source tree
// for retrieval. It has no Git metadata: history remains in the graph index,
// while citation checks and body fetches use exactly the archived bytes.
// The returned cleanup must be called when the server exits.
func MaterializeRetainedReadTree(versionDir string) (string, func(), error) {
	return MaterializeRetainedReadTreeContext(context.Background(), versionDir)
}

// MaterializeRetainedReadTreeContext is MaterializeRetainedReadTree within a
// request; it reuses a RequestVerification carried by ctx for the same version.
func MaterializeRetainedReadTreeContext(ctx context.Context, versionDir string) (string, func(), error) {
	identity, err := InspectVersionIdentityContext(ctx, versionDir)
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
	// A server killed during cleanup leaves its plain source copy behind.
	// Name each copy after its owner and remove copies of exited owners.
	sweepStaleReadTrees(os.TempDir())
	root, err := os.MkdirTemp("", fmt.Sprintf("%s%d-", readTreePrefix, os.Getpid()))
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

const readTreePrefix = "cks-read-source-"

// sweepStaleReadTrees removes retained read trees under dir whose owning
// process has exited. Only names of the form cks-read-source-<pid>-<random>
// are considered; a live or unknown owner, a reused PID, an unparsable name
// or a non-directory is left alone. Failures are ignored: the sweep is a
// best-effort cleanup that must not block serving.
func sweepStaleReadTrees(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		rest, ok := strings.CutPrefix(entry.Name(), readTreePrefix)
		if !ok || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		owner, _, ok := strings.Cut(rest, "-")
		pid, err := strconv.Atoi(owner)
		if !ok || err != nil || pid <= 1 || pid == os.Getpid() {
			continue
		}
		if unix.Kill(pid, 0) != unix.ESRCH {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
	}
}
