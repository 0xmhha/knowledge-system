package setup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type buildScratchKey struct{}

const buildStagingFile = "build-staging.json"

type buildStagingRecord struct {
	SchemaVersion int    `json:"schema_version"`
	ProjectID     string `json:"project_id"`
	DatasetID     string `json:"dataset_id"`
	SnapshotID    string `json:"snapshot_id"`
	Version       string `json:"version"`
	OperationID   string `json:"operation_id"`
	Directory     string `json:"directory"`
	Token         string `json:"token"`
}

func validBuildStaging(record buildStagingRecord) bool {
	token, err := hex.DecodeString(record.Token)
	return record.SchemaVersion == 1 && managedID.MatchString(record.ProjectID) && managedHash.MatchString(record.DatasetID) && managedHash.MatchString(record.SnapshotID) &&
		managedVersion(record.Version) && (record.OperationID == "" || managedID.MatchString(record.OperationID)) && err == nil && len(token) == 16 &&
		record.Directory == ".build-source-"+record.Token
}

// The staging path is always a relative child of this immutable candidate;
// no absolute path from persisted metadata is ever recursively removed.
func newBuildStaging(ctx context.Context, candidate string, source SourceIdentity, datasetID, operationID string) (string, func() error, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", nil, err
	}
	record := buildStagingRecord{SchemaVersion: 1, ProjectID: source.ProjectID, DatasetID: datasetID, SnapshotID: source.SnapshotID, Version: filepath.Base(candidate), OperationID: operationID, Token: hex.EncodeToString(random[:])}
	record.Directory = ".build-source-" + record.Token
	if !validBuildStaging(record) {
		return "", nil, fmt.Errorf("invalid staging identity")
	}
	recordPath := filepath.Join(candidate, buildStagingFile)
	if _, err := os.Lstat(recordPath); !os.IsNotExist(err) {
		return "", nil, fmt.Errorf("staging record already exists")
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	// Record the reserved location before allocating/populating it.
	if err := writeJSONAtomic(recordPath, record); err != nil {
		return "", nil, err
	}
	if err := durabilityStep("staging-intent", recordPath); err != nil {
		return "", nil, err
	}
	parent := filepath.Join(candidate, record.Directory)
	if err := os.Mkdir(parent, 0o700); err != nil {
		return "", nil, err
	}
	if err := durabilityStep("staging-directory", parent); err != nil {
		return "", nil, err
	}
	if err := writeJSONAtomic(filepath.Join(parent, ".owner.json"), record); err != nil {
		return "", nil, err
	}
	if err := durabilityStep("staging-owner", parent); err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		return cleanupBuildStaging(ctx, candidate, record.ProjectID, record.SnapshotID, record.DatasetID, record.OperationID)
	}
	return parent, cleanup, nil
}

// CleanupAbandonedBuildStaging must run under the dataset mutation lock after
// writer quiescence. A mismatched owner remains for inspection, not deletion.
func CleanupAbandonedBuildStaging(ctx context.Context, dataset, version, projectID, snapshotID, datasetID, operationID string) error {
	if !managedVersion(version) {
		return fmt.Errorf("invalid staging version")
	}
	return cleanupBuildStaging(ctx, filepath.Join(dataset, version), projectID, snapshotID, datasetID, operationID)
}
func cleanupBuildStaging(ctx context.Context, candidate, projectID, snapshotID, datasetID, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(candidate)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("staging candidate is linked or not a directory")
	}
	recordPath := filepath.Join(candidate, buildStagingFile)
	var record buildStagingRecord
	if err := readManagedMetadata(recordPath, &record); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if !validBuildStaging(record) || record.ProjectID != projectID || record.SnapshotID != snapshotID || record.DatasetID != datasetID || record.OperationID != operationID || record.Version != filepath.Base(candidate) {
		return fmt.Errorf("staging owner differs")
	}
	parent := filepath.Join(candidate, record.Directory)
	info, err = os.Lstat(parent)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("staging directory is linked or not a directory")
		}
		var owner buildStagingRecord
		ownerErr := readManagedMetadata(filepath.Join(parent, ".owner.json"), &owner)
		if os.IsNotExist(ownerErr) {
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				return fmt.Errorf("unconfirmed staging allocation")
			}
			// Exact reserved child, no data populated: only remove the empty directory.
			if err := os.Remove(parent); err != nil {
				return err
			}
		} else {
			if ownerErr != nil || owner != record {
				return fmt.Errorf("staging marker differs")
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := os.RemoveAll(parent); err != nil {
				return err
			}
		}
		if err := syncDirectory(candidate); err != nil {
			return err
		}
	}
	if err := os.Remove(recordPath); err != nil {
		return err
	}
	return syncDirectory(candidate)
}
