package maintenance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

const ownerFile = ".maintenance-project.json"

type owner struct {
	SchemaVersion int    `json:"schema_version"`
	ProjectID     string `json:"project_id"`
}

func ownsRoot(path, project string) (bool, error) {
	info, err := os.Lstat(filepath.Join(path, ownerFile))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return false, reject("project_mismatch")
	}
	raw, err := os.ReadFile(filepath.Join(path, ownerFile))
	if err != nil {
		return false, reject("project_mismatch")
	}
	var value owner
	if Decode(raw, &value) != nil || value.SchemaVersion != 1 || value.ProjectID != project {
		return false, reject("project_mismatch")
	}
	return true, nil
}

// claimRoot is invoked only by BeforeCandidate with the setup mutation lock.
func claimRoot(path, project string) error {
	if found, err := ownsRoot(path, project); found || err != nil {
		return err
	}
	raw, err := json.Marshal(owner{SchemaVersion: 1, ProjectID: project})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(path, ownerFile), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return reject("project_mismatch")
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

func checkVersionOwner(ctx context.Context, root, version, project string, rootOwned bool) error {
	path := filepath.Join(root, version)
	id, err := setup.InspectVersionIdentityContext(ctx, path)
	if err == nil && id != nil {
		if id.Source.ProjectID != project {
			return reject("project_mismatch")
		}
		return nil
	}
	// Incomplete owned candidates are diagnostic data, not alternate active DBs.
	// Check their capture identity too before allowing another version to build.
	raw, readErr := os.ReadFile(filepath.Join(path, "sources", "manifest.json"))
	if readErr == nil {
		var capture struct {
			Identity setup.SourceIdentity `json:"identity"`
		}
		if json.Unmarshal(raw, &capture) != nil || capture.Identity.ProjectID != project {
			return reject("project_mismatch")
		}
		if rootOwned {
			return nil
		}
	}
	if rootOwned && os.IsNotExist(readErr) {
		return nil
	}
	return reject("unsupported_capability")
}
