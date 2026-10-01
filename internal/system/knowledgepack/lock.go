package knowledgepack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"gopkg.in/yaml.v3"
)

type Selection struct {
	PackID  string `yaml:"pack_id" json:"pack_id"`
	Version string `yaml:"version" json:"version"`
	Source  string `yaml:"source" json:"source"`
	SHA256  string `yaml:"sha256" json:"sha256"`
}

type Manifest struct {
	SchemaVersion int           `yaml:"schema_version" json:"schema_version"`
	ProjectID     string        `yaml:"project_id" json:"project_id"`
	SelectedPacks []Selection   `yaml:"selected_packs" json:"selected_packs"`
	OverlayRoot   string        `yaml:"overlay_root" json:"overlay_root"`
	ReviewPolicy  *ReviewPolicy `yaml:"review_policy" json:"review_policy,omitempty"`
}

type ReviewPolicy struct {
	MinApprovals int `yaml:"min_approvals" json:"min_approvals"`
}

type LockedPack struct {
	PackID       string       `json:"pack_id"`
	Version      string       `json:"version"`
	Digest       string       `json:"digest"`
	Dependencies []Dependency `json:"dependencies"`
	OriginID     string       `json:"origin_id"`
}

type Lock struct {
	ProjectID         string       `json:"project_id"`
	PackSchemaVersion int          `json:"pack_schema_version"`
	Packs             []LockedPack `json:"packs"`
	OverlayDigest     string       `json:"overlay_digest"`
	LockDigest        string       `json:"lock_digest,omitempty"`
}

func ReadManifest(projectRoot string) (Manifest, error) {
	root, err := registeredDir(projectRoot, ".cks/knowledge")
	if err != nil {
		return Manifest{}, err
	}
	buf, err := setup.ReadSourceFileNoFollow(root, "manifest.yaml", 1<<20)
	if err != nil {
		return Manifest{}, fmt.Errorf("pack_incompatible: read knowledge manifest: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(buf))
	decoder.KnownFields(true)
	var m Manifest
	if err := decoder.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("pack_incompatible: manifest YAML: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Manifest{}, fmt.Errorf("pack_incompatible: multiple manifest YAML documents")
	}
	if m.SchemaVersion != 1 || strings.TrimSpace(m.ProjectID) == "" || m.OverlayRoot != ".cks/knowledge" {
		return Manifest{}, fmt.Errorf("pack_incompatible: invalid knowledge manifest header")
	}
	if m.ReviewPolicy == nil {
		m.ReviewPolicy = &ReviewPolicy{MinApprovals: 1}
	}
	if m.ReviewPolicy.MinApprovals < 1 {
		return Manifest{}, fmt.Errorf("pack_incompatible: review_policy.min_approvals must be positive")
	}
	return m, nil
}

// BuildLock validates the complete selected set and snapshots its raw file
// digests. The returned lock can be compared with a saved lock before a
// candidate build; a changed policy or pack file produces a different ID.
func BuildLock(projectRoot string) (Lock, error) {
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return Lock{}, err
	}
	root, err := registeredDir(projectRoot, m.OverlayRoot)
	if err != nil {
		return Lock{}, err
	}
	overlayDigest, err := digestOverlay(root)
	if err != nil {
		return Lock{}, err
	}
	loaded := make([]LoadedPack, 0, len(m.SelectedPacks))
	selected := map[string]bool{}
	for _, selection := range m.SelectedPacks {
		if !packIDPattern.MatchString(selection.PackID) || !versionPattern.MatchString(selection.Version) ||
			!digestPattern.MatchString(selection.SHA256) || selected[selection.PackID] {
			return Lock{}, fmt.Errorf("pack_incompatible: invalid or duplicate selected pack %q", selection.PackID)
		}
		selected[selection.PackID] = true
		packRoot, err := registeredDir(projectRoot, selection.Source)
		if err != nil {
			return Lock{}, err
		}
		pack, err := Load(packRoot)
		if err != nil {
			return Lock{}, err
		}
		if pack.Pack.PackID != selection.PackID || pack.Pack.Version != selection.Version || pack.Digest != selection.SHA256 {
			return Lock{}, fmt.Errorf("pack_lock_mismatch: selected pack %q differs from registered version or digest", selection.PackID)
		}
		loaded = append(loaded, pack)
	}
	ordered, err := Resolve(loaded)
	if err != nil {
		return Lock{}, err
	}
	instances, err := LoadInstances(root, ordered)
	if err != nil {
		return Lock{}, err
	}
	if err := enforceReviewPolicy(m, instances); err != nil {
		return Lock{}, err
	}
	lock := Lock{ProjectID: m.ProjectID, PackSchemaVersion: 1, Packs: []LockedPack{}, OverlayDigest: overlayDigest}
	for _, pack := range ordered {
		deps := append([]Dependency(nil), pack.Pack.Requires...)
		sort.Slice(deps, func(i, j int) bool { return deps[i].PackID < deps[j].PackID })
		lock.Packs = append(lock.Packs, LockedPack{PackID: pack.Pack.PackID, Version: pack.Pack.Version,
			Digest: pack.Digest, Dependencies: deps, OriginID: "knowledge:" + pack.Pack.PackID})
	}
	if err := lock.Stamp(); err != nil {
		return Lock{}, err
	}
	return lock, nil
}

func (l *Lock) Stamp() error {
	if l == nil || l.ProjectID == "" || l.PackSchemaVersion != 1 || !digestPattern.MatchString(l.OverlayDigest) {
		return fmt.Errorf("pack_incompatible: invalid knowledge lock")
	}
	l.LockDigest = ""
	data, err := json.Marshal(*l)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	l.LockDigest = hex.EncodeToString(sum[:])
	return nil
}

func VerifyLock(projectRoot string) (Lock, error) {
	want, err := BuildLock(projectRoot)
	if err != nil {
		return Lock{}, err
	}
	root, err := registeredDir(projectRoot, ".cks/knowledge")
	if err != nil {
		return Lock{}, err
	}
	buf, err := setup.ReadSourceFileNoFollow(root, "knowledge.lock.json", 1<<20)
	if err != nil {
		return Lock{}, fmt.Errorf("pack_lock_mismatch: lock file missing: %w", err)
	}
	var got Lock
	decoder := json.NewDecoder(bytes.NewReader(buf))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&got); err != nil {
		return Lock{}, fmt.Errorf("pack_lock_mismatch: invalid lock: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Lock{}, fmt.Errorf("pack_lock_mismatch: trailing lock data")
	}
	if got.LockDigest != want.LockDigest || !bytes.Equal(mustJSON(got), mustJSON(want)) {
		return Lock{}, fmt.Errorf("pack_lock_mismatch: saved lock differs from current registered bytes")
	}
	return want, nil
}

// LoadProjectInstances is read-only and returns validated project-authored
// policies and ADRs. It never upgrades proposed records automatically.
func LoadProjectInstances(projectRoot string) (Instances, error) {
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return Instances{}, err
	}
	overlay, err := registeredDir(projectRoot, m.OverlayRoot)
	if err != nil {
		return Instances{}, err
	}
	var packs []LoadedPack
	for _, selected := range m.SelectedPacks {
		root, err := registeredDir(projectRoot, selected.Source)
		if err != nil {
			return Instances{}, err
		}
		pack, err := Load(root)
		if err != nil {
			return Instances{}, err
		}
		if pack.Pack.PackID != selected.PackID || pack.Pack.Version != selected.Version || pack.Digest != selected.SHA256 {
			return Instances{}, fmt.Errorf("pack_lock_mismatch: selected pack bytes changed")
		}
		packs = append(packs, pack)
	}
	ordered, err := Resolve(packs)
	if err != nil {
		return Instances{}, err
	}
	instances, err := LoadInstances(overlay, ordered)
	if err != nil {
		return Instances{}, err
	}
	if err := enforceReviewPolicy(m, instances); err != nil {
		return Instances{}, err
	}
	return instances, nil
}

func enforceReviewPolicy(m Manifest, instances Instances) error {
	if m.ReviewPolicy == nil || m.ReviewPolicy.MinApprovals <= 1 {
		return nil
	}
	for _, p := range instances.Policies {
		if p.Status == "verified" {
			return fmt.Errorf("evidence_unverified: policy %q has fewer than %d independent approvals", p.ID, m.ReviewPolicy.MinApprovals)
		}
	}
	for _, d := range instances.Decisions {
		if d.Status == "verified" {
			return fmt.Errorf("evidence_unverified: decision %q has fewer than %d independent approvals", d.ID, m.ReviewPolicy.MinApprovals)
		}
	}
	for _, relation := range instances.Relations {
		if relation.Status == "verified" {
			return fmt.Errorf("evidence_unverified: relation %q has fewer than %d independent approvals", relation.ID, m.ReviewPolicy.MinApprovals)
		}
	}
	return nil
}

// LoadRegisteredPack permits a CLI to calculate the exact local pack digest
// before adding it to manifest.yaml, using the same path checks as locking.
func LoadRegisteredPack(projectRoot, source string) (LoadedPack, error) {
	root, err := registeredDir(projectRoot, source)
	if err != nil {
		return LoadedPack{}, err
	}
	return Load(root)
}

func mustJSON(value any) []byte { data, _ := json.Marshal(value); return data }

func registeredDir(projectRoot, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "\\") || strings.ContainsRune(rel, 0) {
		return "", fmt.Errorf("pack_incompatible: invalid registered source path")
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	if clean == ".." || strings.HasPrefix(clean, "../") || clean != strings.TrimPrefix(rel, "./") {
		return "", fmt.Errorf("pack_incompatible: source path escapes project")
	}
	project, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return "", err
	}
	current := project
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("pack_incompatible: unsafe source path component")
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() {
			return "", fmt.Errorf("pack_incompatible: source root is missing, linked or not a directory")
		}
	}
	return current, nil
}
