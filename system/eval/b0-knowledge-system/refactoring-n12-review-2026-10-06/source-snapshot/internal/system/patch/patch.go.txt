package patch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type ChangedFile struct {
	OriginID     string `json:"origin_id"`
	Path         string `json:"path"`
	BeforeSHA256 string `json:"before_sha256,omitempty"`
	AfterSHA256  string `json:"after_sha256,omitempty"`
}

// Attempt is immutable. A test passing does not change its unconfirmed state;
// review decisions are separate append-only records.
type Attempt struct {
	PatchID            string        `json:"patch_id"`
	ProjectID          string        `json:"project_id"`
	BaseVersion        string        `json:"base_version"`
	BaseSnapshotID     string        `json:"base_snapshot_id"`
	ResultVersion      string        `json:"result_version"`
	ResultSnapshotID   string        `json:"result_snapshot_id"`
	ResultDatasetID    string        `json:"result_dataset_id"`
	ChangedFiles       []ChangedFile `json:"changed_files"`
	ChangedFilesDigest string        `json:"changed_files_digest"`
	State              string        `json:"state"`
}

type Decision struct {
	DecisionID     string    `json:"decision_id"`
	PatchID        string    `json:"patch_id"`
	CriterionID    string    `json:"criterion_id"`
	SpecVersion    int       `json:"spec_version"`
	ProjectID      string    `json:"project_id"`
	DatasetID      string    `json:"dataset_id"`
	SnapshotID     string    `json:"snapshot_id"`
	EvidenceSHA256 string    `json:"evidence_sha256"`
	ReviewedBy     string    `json:"reviewed_by"`
	Outcome        string    `json:"outcome"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

type hold struct {
	ProjectID   string `json:"project_id"`
	DatasetID   string `json:"dataset_id"`
	SnapshotID  string `json:"snapshot_id"`
	BaseVersion string `json:"base_version"`
}

func versionPath(dataset, version string) (string, error) {
	if !safeID.MatchString(version) || version == "current" {
		return "", fmt.Errorf("invalid dataset version")
	}
	return filepath.Join(dataset, version), nil
}

func registryPath(dataset, patchID string) (string, error) {
	if !safeID.MatchString(patchID) {
		return "", fmt.Errorf("invalid patch ID")
	}
	return filepath.Join(dataset, ".patches", patchID+".json"), nil
}

func readJSON(path string, out any) error {
	return decodeJSON(path, out, true)
}

func readJSONLoose(path string, out any) error { return decodeJSON(path, out, false) }

func decodeJSON(path string, out any, strict bool) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("patch record unavailable: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing patch JSON")
	}
	return nil
}

func writeImmutable(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		old, readErr := os.ReadFile(path)
		if readErr == nil && bytes.Equal(old, data) {
			return nil
		}
		return fmt.Errorf("patch ID already exists with different bytes")
	}
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err = f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

func readHold(dataset, version string) (hold, *setup.DatasetIdentity, error) {
	vdir, err := versionPath(dataset, version)
	if err != nil {
		return hold{}, nil, err
	}
	identity, err := setup.InspectVersionIdentity(vdir)
	if err != nil {
		return hold{}, nil, err
	}
	if identity == nil {
		return hold{}, nil, fmt.Errorf("patch workflow requires a pinned dataset")
	}
	var h hold
	if err := readJSON(filepath.Join(vdir, "review-hold.json"), &h); err != nil {
		return hold{}, nil, err
	}
	if h.ProjectID != identity.Source.ProjectID || h.DatasetID != identity.DatasetID || h.SnapshotID != identity.Source.SnapshotID {
		return hold{}, nil, fmt.Errorf("patch review hold differs from candidate")
	}
	return h, identity, nil
}

// Register derives the exact changed-file set from two retained archives.
// It refuses a reused patch ID with different bytes even across versions.
func Register(dataset, version, patchID string) (Attempt, error) {
	if !safeID.MatchString(patchID) {
		return Attempt{}, fmt.Errorf("invalid patch ID")
	}
	h, result, err := readHold(dataset, version)
	if err != nil {
		return Attempt{}, err
	}
	if h.BaseVersion == "" {
		return Attempt{}, fmt.Errorf("patch attempt requires an active base version")
	}
	baseDir, err := versionPath(dataset, h.BaseVersion)
	if err != nil {
		return Attempt{}, err
	}
	base, err := setup.InspectVersionIdentity(baseDir)
	if err != nil {
		return Attempt{}, err
	}
	if base == nil || base.Source.ProjectID != result.Source.ProjectID || base.Source.SnapshotID == result.Source.SnapshotID {
		return Attempt{}, fmt.Errorf("patch base is missing, foreign, or identical to result")
	}
	changed, changedDigest, err := diffArchives(baseDir, filepath.Join(dataset, version))
	if err != nil {
		return Attempt{}, err
	}
	if len(changed) == 0 {
		return Attempt{}, fmt.Errorf("patch attempt has no changed source files")
	}
	attempt := Attempt{PatchID: patchID, ProjectID: result.Source.ProjectID, BaseVersion: h.BaseVersion,
		BaseSnapshotID: base.Source.SnapshotID, ResultVersion: version, ResultSnapshotID: result.Source.SnapshotID,
		ResultDatasetID: result.DatasetID, ChangedFiles: changed, ChangedFilesDigest: changedDigest, State: "unconfirmed"}
	path, err := registryPath(dataset, patchID)
	if err != nil {
		return Attempt{}, err
	}
	if err := writeImmutable(path, attempt); err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}

func diffArchives(baseDir, resultDir string) ([]ChangedFile, string, error) {
	before, err := archiveFiles(baseDir)
	if err != nil {
		return nil, "", err
	}
	after, err := archiveFiles(resultDir)
	if err != nil {
		return nil, "", err
	}
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	changed := []ChangedFile{}
	for _, key := range ordered {
		old, new := before[key], after[key]
		if old == new {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		changed = append(changed, ChangedFile{OriginID: parts[0], Path: parts[1], BeforeSHA256: old, AfterSHA256: new})
	}
	bytes, err := json.Marshal(changed)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(bytes)
	return changed, hex.EncodeToString(sum[:]), nil
}

func archiveFiles(versionDir string) (map[string]string, error) {
	var manifest setup.CapturedSource
	if err := readJSON(filepath.Join(versionDir, "sources", "manifest.json"), &manifest); err != nil {
		return nil, err
	}
	files := make(map[string]string, len(manifest.Files))
	for _, f := range manifest.Files {
		files[f.OriginID+"\x00"+f.Path] = f.SHA256
	}
	return files, nil
}

func Load(dataset, patchID string) (Attempt, error) {
	path, err := registryPath(dataset, patchID)
	if err != nil {
		return Attempt{}, err
	}
	var attempt Attempt
	if err := readJSON(path, &attempt); err != nil {
		return Attempt{}, err
	}
	if attempt.PatchID != patchID || attempt.State != "unconfirmed" {
		return Attempt{}, fmt.Errorf("patch registry ID or state mismatch")
	}
	h, identity, err := readHold(dataset, attempt.ResultVersion)
	if err != nil {
		return Attempt{}, err
	}
	if h.BaseVersion != attempt.BaseVersion || identity.DatasetID != attempt.ResultDatasetID || identity.Source.SnapshotID != attempt.ResultSnapshotID || identity.Source.ProjectID != attempt.ProjectID {
		return Attempt{}, fmt.Errorf("patch candidate identity changed")
	}
	baseDir, err := versionPath(dataset, attempt.BaseVersion)
	if err != nil {
		return Attempt{}, err
	}
	base, err := setup.InspectVersionIdentity(baseDir)
	if err != nil {
		return Attempt{}, err
	}
	if base == nil || base.Source.ProjectID != attempt.ProjectID || base.Source.SnapshotID != attempt.BaseSnapshotID {
		return Attempt{}, fmt.Errorf("patch base identity changed")
	}
	changed, digest, err := diffArchives(baseDir, filepath.Join(dataset, attempt.ResultVersion))
	if err != nil {
		return Attempt{}, err
	}
	if !reflect.DeepEqual(changed, attempt.ChangedFiles) || digest != attempt.ChangedFilesDigest {
		return Attempt{}, fmt.Errorf("patch changed-file record differs from retained archives")
	}
	return attempt, nil
}

// CandidateProjection validates the stored semantic model against retained
// source and both engines before any criterion decision can be recorded.
func CandidateProjection(ctx context.Context, dataset string, a Attempt, storePath string) (semantic.Projection, error) {
	store, err := semantic.OpenStore(storePath)
	if err != nil {
		return semantic.Projection{}, err
	}
	defer store.Close()
	p, err := store.Load(ctx, a.ProjectID, a.ResultDatasetID)
	if err != nil {
		return semantic.Projection{}, err
	}
	vdir := filepath.Join(dataset, a.ResultVersion)
	var graph struct {
		SrcRoot string `json:"src_root"`
	}
	if err := readJSONLoose(filepath.Join(vdir, "graph", "manifest.json"), &graph); err != nil {
		return semantic.Projection{}, err
	}
	if err := semantic.ValidateDatasetAlignment(p, graph.SrcRoot, filepath.Join(vdir, "graph"), filepath.Join(vdir, "vector")); err != nil {
		return semantic.Projection{}, err
	}
	if err := semantic.ValidateCodeAnchors(p, filepath.Join(vdir, "graph")); err != nil {
		return semantic.Projection{}, err
	}
	if err := semantic.ValidateCKVChunkLinks(ctx, p, filepath.Join(vdir, "vector")); err != nil {
		return semantic.Projection{}, err
	}
	if err := p.ValidateRetainedSources(vdir); err != nil {
		return semantic.Projection{}, err
	}
	if p.Snapshot.SnapshotID != a.ResultSnapshotID {
		return semantic.Projection{}, fmt.Errorf("semantic projection crosses patch snapshot")
	}
	return p, nil
}
