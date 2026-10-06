package setup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ErrVersionedSetupRequired is deliberately path-free: public maintenance
// errors must not disclose retained paths or malformed manifest contents.
var ErrVersionedSetupRequired = errors.New("versioned_setup_required: in-place or unpinned maintenance is unavailable for versioned datasets; use cks setup with the reviewed config, project ID and a NEW version (optionally --hold-for-review), then restart readers after promotion")

// CheckMutableTarget refuses retained/pinned data even if its identity record
// is missing or corrupt. It checks existing ancestors, including symlink
// aliases of otherwise nonexistent child paths. It does not authorize races
// with another writer; the dataset mutation lock provides that serialization.
func CheckMutableTarget(path string) error {
	return checkMaintenanceTarget(path, false)
}

// CheckLegacyMaintenanceTarget additionally refuses blue-green layouts. Only
// flat legacy targets may use the synchronous in-place MCP refresh.
func CheckLegacyMaintenanceTarget(path string) error {
	return checkMaintenanceTarget(path, true)
}

func checkMaintenanceTarget(path string, versioned bool) error {
	if path == "" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ErrVersionedSetupRequired
	}
	for dir := abs; ; dir = filepath.Dir(dir) {
		// Resolve each existing ancestor separately: the final DB or candidate
		// may not exist yet, while current is already a symlink to a pin.
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil && !os.IsNotExist(err) {
			return ErrVersionedSetupRequired
		}
		for _, p := range []string{dir, resolved} {
			if p == "" {
				continue
			}
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				continue
			} else if err != nil && !os.IsNotExist(err) {
				return ErrVersionedSetupRequired
			}
			for _, marker := range []string{"dataset-identity.json", "sources/manifest.json", "sources/blobs"} {
				if _, err := os.Lstat(filepath.Join(p, marker)); err == nil || !os.IsNotExist(err) {
					return ErrVersionedSetupRequired
				}
			}
			if versioned {
				if _, err := os.Lstat(filepath.Join(p, "current")); err == nil || !os.IsNotExist(err) {
					return ErrVersionedSetupRequired
				}
			}
			for _, manifest := range []string{filepath.Join(p, "manifest.json"), filepath.Join(p, "graph", "manifest.json"), filepath.Join(p, "vector", "manifest.json")} {
				data, err := os.ReadFile(manifest)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return ErrVersionedSetupRequired
				}
				var pins struct {
					ProjectID  string `json:"project_id"`
					SnapshotID string `json:"snapshot_id"`
					DatasetID  string `json:"dataset_id"`
				}
				if json.Unmarshal(data, &pins) != nil || pins.ProjectID != "" || pins.SnapshotID != "" || pins.DatasetID != "" {
					return ErrVersionedSetupRequired
				}
			}
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return nil
}

// Only Reindex owns this fresh capture. The shared public BuildPlan cannot
// turn arbitrary pin flags into permission to overwrite an existing version.
func checkFreshCapturedCandidate(o Options, c CapturedSource) error {
	if o.DatasetID == "" || o.ProjectID != c.Identity.ProjectID || o.SnapshotID != c.Identity.SnapshotID ||
		o.FileManifestDigest != c.Identity.FileManifestDigest || o.CapturePolicyDigest != c.Identity.CapturePolicyDigest || o.SourceMode != c.Identity.SourceMode {
		return ErrVersionedSetupRequired
	}
	if err := CheckMutableTarget(filepath.Dir(o.Out)); err != nil {
		return err
	}
	info, err := os.Lstat(o.Out)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrVersionedSetupRequired
	}
	for _, artifact := range []string{"dataset-identity.json", "graph", "vector"} {
		if _, err := os.Lstat(filepath.Join(o.Out, artifact)); err == nil || !os.IsNotExist(err) {
			return ErrVersionedSetupRequired
		}
	}
	return VerifyRetainedSource(o.Out, c.Identity)
}

// CheckLegacyReindex prevents an unpinned caller from replacing a v2 dataset
// with a legacy candidate, including datasets whose candidates are held for
// review before the first current pointer exists.
func CheckLegacyReindex(dataset string) error {
	if err := CheckMutableTarget(dataset); err != nil {
		return err
	}
	entries, err := os.ReadDir(dataset)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return ErrVersionedSetupRequired
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			if err := CheckMutableTarget(filepath.Join(dataset, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
