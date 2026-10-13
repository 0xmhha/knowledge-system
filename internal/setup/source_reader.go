package setup

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/sourcepath"
)

// OpenRetainedSourceReader verifies a pinned archive without copying it into
// a temporary source tree. Each selected blob is re-read and hashed; v2 also
// keeps its existing whole-candidate before/after request verification.
// The reader owns no open handles or temporary files, so shutdown has no
// tree deletion to wait for.
func OpenRetainedSourceReader(ctx context.Context, versionDir string) (sourcepath.Reader, error) {
	identity, err := InspectVersionIdentityContext(ctx, versionDir)
	if err != nil || identity == nil {
		return nil, fmt.Errorf("retained read source requires verified identity: %v", err)
	}
	var captured CapturedSource
	if err := readJSON(filepath.Join(versionDir, "sources", "manifest.json"), &captured); err != nil {
		return nil, err
	}
	if captured.Identity != identity.Source {
		return nil, fmt.Errorf("snapshot_mismatch: retained source identity changed")
	}
	captured.BlobDir = filepath.Join(versionDir, "sources", "blobs")
	files := make(map[string]CapturedFile, len(captured.Files))
	for _, file := range captured.Files {
		path := file.Path
		if err := validateCapturedPaths([]string{path}); err != nil {
			return nil, err
		}
		if file.OriginID != "repo" {
			if !strings.HasPrefix(file.OriginID, "docs:corpus.") {
				continue
			}
			number := strings.TrimPrefix(file.OriginID, "docs:corpus.")
			index, err := strconv.Atoi(number)
			if err != nil || index < 0 || strconv.Itoa(index) != number {
				return nil, fmt.Errorf("invalid retained corpus origin")
			}
			path = sourcepath.Docs(index, path)
		}
		if _, exists := files[path]; exists {
			return nil, fmt.Errorf("duplicate retained source path")
		}
		files[path] = file
	}
	// Close the startup manifest-read window before publishing the index.
	after, err := InspectVersionIdentityContext(ctx, versionDir)
	if err != nil {
		return nil, err
	}
	if after == nil || after.DatasetID != identity.DatasetID || after.Source != identity.Source {
		return nil, fmt.Errorf("snapshot_mismatch: retained source changed during open")
	}
	return func(ctx context.Context, path string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, relative, reserved, valid := sourcepath.Parse(path)
		if reserved && !valid {
			return nil, fmt.Errorf("invalid retained source namespace")
		}
		if err := validateCapturedPaths([]string{relative}); err != nil {
			return nil, err
		}
		file, found := files[path]
		if !found {
			return nil, fmt.Errorf("source_missing: source is not in retained inventory")
		}
		data, err := captured.ReadBlobContext(ctx, file.SHA256)
		if err != nil {
			return nil, err
		}
		if int64(len(data)) != file.Size {
			return nil, fmt.Errorf("snapshot_mismatch: retained source size changed")
		}
		return data, nil
	}, nil
}
