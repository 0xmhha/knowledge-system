package knowledgepack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

// LoadRetainedProjectInstances reconstructs registered knowledge inputs only
// from a verified candidate archive. The temporary tree is removed before
// returning; callers never read a mutable checkout to answer a v2 request.
func LoadRetainedProjectInstances(versionDir string) (Instances, Lock, error) {
	identity, err := setup.InspectVersionIdentity(versionDir)
	if err != nil {
		return Instances{}, Lock{}, err
	}
	if identity == nil {
		return Instances{}, Lock{}, fmt.Errorf("reindex_required: retained knowledge needs a pinned dataset")
	}
	root, cleanup, err := setup.MaterializeRetainedReadTree(versionDir)
	if err != nil {
		return Instances{}, Lock{}, err
	}
	defer cleanup()
	manifest, err := ReadManifest(root)
	if err != nil {
		return Instances{}, Lock{}, err
	}
	if manifest.ProjectID != identity.Source.ProjectID {
		return Instances{}, Lock{}, fmt.Errorf("snapshot_mismatch: knowledge project differs from candidate")
	}
	data, err := os.ReadFile(filepath.Join(versionDir, "sources", "manifest.json"))
	if err != nil {
		return Instances{}, Lock{}, err
	}
	var archive setup.CapturedSource
	if err := json.Unmarshal(data, &archive); err != nil || archive.Identity != identity.Source {
		return Instances{}, Lock{}, fmt.Errorf("snapshot_mismatch: retained knowledge inventory differs from candidate")
	}
	selected := make(map[string]string, len(manifest.SelectedPacks))
	for _, pack := range manifest.SelectedPacks {
		if !packIDPattern.MatchString(pack.PackID) || !safeRetainedPackPath(pack.Source) || selected[pack.PackID] != "" {
			return Instances{}, Lock{}, fmt.Errorf("pack_incompatible: unsafe or duplicate selected pack")
		}
		selected[pack.PackID] = strings.TrimPrefix(pack.Source, "./")
	}
	seen := map[string]bool{}
	for _, file := range archive.Files {
		if !strings.HasPrefix(file.OriginID, "knowledge:") {
			continue
		}
		packID := strings.TrimPrefix(file.OriginID, "knowledge:")
		source, ok := selected[packID]
		if !ok {
			return Instances{}, Lock{}, fmt.Errorf("pack_incompatible: unselected knowledge origin")
		}
		buf, _, _, err := setup.ReadRetainedFile(versionDir, file.OriginID, file.Path)
		if err != nil {
			return Instances{}, Lock{}, err
		}
		if err := writeRetainedPackFile(root, source, file.Path, buf); err != nil {
			return Instances{}, Lock{}, err
		}
		seen[packID] = true
	}
	for id := range selected {
		if !seen[id] {
			return Instances{}, Lock{}, fmt.Errorf("source_missing: selected pack %q has no archived files", id)
		}
	}
	lock, err := VerifyLock(root)
	if err != nil {
		return Instances{}, Lock{}, err
	}
	instances, err := LoadProjectInstances(root)
	if err != nil {
		return Instances{}, Lock{}, err
	}
	return instances, lock, nil
}

func safeRetainedPackPath(path string) bool {
	path = strings.TrimPrefix(path, "./")
	return path != "" && path != "." && !filepath.IsAbs(path) && !strings.Contains(path, "\\") &&
		!strings.ContainsRune(path, 0) && filepath.ToSlash(filepath.Clean(path)) == path &&
		path != ".." && !strings.HasPrefix(path, "../")
}

func writeRetainedPackFile(root, source, relative string, data []byte) error {
	if !safeRetainedPackPath(source) || !safeRetainedPackPath(relative) {
		return fmt.Errorf("pack_incompatible: unsafe archived pack path")
	}
	path := filepath.Join(root, filepath.FromSlash(source), filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if old, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(old, data) {
			return fmt.Errorf("snapshot_mismatch: repository and pack origin disagree on %q", relative)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
