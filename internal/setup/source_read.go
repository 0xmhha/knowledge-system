package setup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// RetainedEvidence is one v2 citation resolved solely from a version's
// content-addressed archive. Text is the exact, unsanitized source line bytes;
// callers must sanitize it before exposing it in a coding context.
type RetainedEvidence struct {
	ProjectID     string `json:"project_id"`
	DatasetID     string `json:"dataset_id"`
	SnapshotID    string `json:"snapshot_id"`
	SourceMode    string `json:"source_mode"`
	OriginID      string `json:"origin_id"`
	File          string `json:"file"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
	CommitHash    string `json:"commit_hash"`
	BaseCommit    string `json:"base_commit"`
	FileSHA256    string `json:"file_sha256"`
	ContentSHA256 string `json:"content_sha256"`
	Text          string `json:"text"`
}

// ReadRetainedLines never falls back to the live checkout or Git HEAD. A
// missing or changed archive is an explicit error. Each call verifies the
// candidate tuple and all archived file hashes before returning one span;
// long-lived servers can hold a verified archive index for batched lookups.
func ReadRetainedLines(versionDir, originID, path string, first, last int) (RetainedEvidence, error) {
	if (originID != "repo" && !validCaptureOriginID(originID)) || first <= 0 || last < first || path == "" {
		return RetainedEvidence{}, fmt.Errorf("invalid retained citation coordinates")
	}
	buf, identity, fileSHA, err := ReadRetainedFile(versionDir, originID, path)
	if err != nil {
		return RetainedEvidence{}, err
	}
	span, err := sourceLineBytes(buf, first, last)
	if err != nil {
		return RetainedEvidence{}, err
	}
	sum := sha256.Sum256(span)
	commit := ""
	if identity.Source.SourceMode == "committed" {
		commit = identity.Source.SourceCommit
	}
	return RetainedEvidence{ProjectID: identity.Source.ProjectID, DatasetID: identity.DatasetID,
		SnapshotID: identity.Source.SnapshotID, SourceMode: identity.Source.SourceMode,
		OriginID: originID, File: path, StartLine: first, EndLine: last,
		CommitHash: commit, BaseCommit: identity.Source.SourceCommit,
		FileSHA256: fileSHA, ContentSHA256: hex.EncodeToString(sum[:]), Text: string(span)}, nil
}

// ReadRetainedFile returns verified original bytes for semantic extraction.
// This is an internal raw-byte API: public responses must sanitize them.
func ReadRetainedFile(versionDir, originID, path string) ([]byte, *DatasetIdentity, string, error) {
	if (originID != "repo" && !validCaptureOriginID(originID)) || path == "" {
		return nil, nil, "", fmt.Errorf("invalid retained source coordinates")
	}
	if err := validateCapturedPaths([]string{path}); err != nil {
		return nil, nil, "", err
	}
	identity, err := InspectVersionIdentity(versionDir)
	if err != nil {
		return nil, nil, "", err
	}
	if identity == nil {
		return nil, nil, "", fmt.Errorf("requires_v2: legacy dataset has no retained source identity")
	}
	sources := filepath.Join(versionDir, "sources")
	info, err := os.Lstat(sources)
	if err != nil || !info.IsDir() {
		return nil, nil, "", fmt.Errorf("source_missing: source archive is unavailable")
	}
	manifestPath := filepath.Join(sources, "manifest.json")
	info, err = os.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, nil, "", fmt.Errorf("source_missing: source archive manifest is unavailable")
	}
	var captured CapturedSource
	if err := readJSON(manifestPath, &captured); err != nil || captured.Identity != identity.Source {
		return nil, nil, "", fmt.Errorf("snapshot_mismatch: retained source identity changed: %v", err)
	}
	var record *CapturedFile
	for i := range captured.Files {
		if captured.Files[i].OriginID == originID && captured.Files[i].Path == path {
			record = &captured.Files[i]
			break
		}
	}
	if record == nil {
		return nil, nil, "", fmt.Errorf("source_missing: citation file is not in the retained inventory")
	}
	captured.BlobDir = filepath.Join(sources, "blobs")
	buf, err := captured.ReadBlob(record.SHA256)
	if err != nil {
		return nil, nil, "", err
	}
	if int64(len(buf)) != record.Size || !utf8.Valid(buf) {
		return nil, nil, "", fmt.Errorf("snapshot_mismatch: retained citation file changed or is not UTF-8")
	}
	return buf, identity, record.SHA256, nil
}

func sourceLineBytes(data []byte, first, last int) ([]byte, error) {
	line := 1
	start := 0
	spanStart := -1
	for start < len(data) {
		if line == first {
			spanStart = start
		}
		end := len(data)
		if offset := bytes.IndexByte(data[start:], '\n'); offset >= 0 {
			end = start + offset + 1
		}
		if line == last {
			if spanStart < 0 {
				return nil, fmt.Errorf("snapshot_mismatch: invalid citation span")
			}
			return data[spanStart:end], nil
		}
		start = end
		line++
	}
	return nil, fmt.Errorf("source_missing: citation line range is outside the retained file")
}
