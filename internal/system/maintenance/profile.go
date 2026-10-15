package maintenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
	"github.com/0xmhha/knowledge-system/pkg/sourcepath"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// Profile is operator-owned configuration, never accepted in tool arguments.
type Profile struct {
	ID                string   `yaml:"id" json:"id"`
	ProjectID         string   `yaml:"project_id" json:"project_id"`
	SetupConfig       string   `yaml:"setup_config" json:"setup_config"`
	AllowedActivation []string `yaml:"allowed_activation" json:"allowed_activation"`
	TimeoutSeconds    int      `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type registered struct {
	spec     Profile
	src, out string
}
type prepared struct {
	options  setup.Options
	source   setup.SourceIdentity
	identity setup.DatasetIdentity
	view     ProfileContext
}

func (p Profile) Validate() error {
	if !identifier.MatchString(p.ID) || !identifier.MatchString(p.ProjectID) || !filepath.IsAbs(p.SetupConfig) ||
		filepath.Clean(p.SetupConfig) != p.SetupConfig || p.TimeoutSeconds < 10 || p.TimeoutSeconds > 86400 ||
		len(p.AllowedActivation) == 0 || len(p.AllowedActivation) > 2 {
		return reject("profile_unknown")
	}
	seen := map[string]bool{}
	for _, mode := range p.AllowedActivation {
		if seen[mode] || (mode != "hold_for_review" && mode != "activate_after_gate") {
			return reject("unsupported_capability")
		}
		seen[mode] = true
	}
	return nil
}

func safeDirectory(path string, mayNotExist bool) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return reject("profile_stale")
	}
	for cursor := path; ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err != nil && (!mayNotExist || !os.IsNotExist(err)) {
			return reject("profile_stale")
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return reject("profile_stale")
		}
		if filepath.Dir(cursor) == cursor {
			break
		}
	}
	return nil
}

func nested(root, child string) bool {
	rel, err := filepath.Rel(root, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func resolveBinary(name, fallback string) (string, error) {
	if name == "" {
		name = fallback
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", reject("unsupported_capability")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", reject("unsupported_capability")
	}
	return filepath.EvalSymlinks(path)
}

func loadProfile(p Profile, self string) (setup.Options, string, error) {
	o, err := setup.LoadConfigStrict(p.SetupConfig)
	if err != nil {
		return o, "", reject("profile_stale")
	}
	if o.ProjectID != p.ProjectID {
		return o, "", reject("project_mismatch")
	}
	if o.SkipVector {
		return o, "", reject("unsupported_capability")
	}
	if o.SourceMode == "" {
		o.SourceMode = "committed"
	}
	if o.SourceMode != "committed" && o.SourceMode != "working-tree" && o.SourceMode != "snapshot-only" {
		return o, "", reject("unsupported_capability")
	}
	if safeDirectory(o.Src, false) != nil || safeDirectory(o.Out, true) != nil || nested(o.Src, o.Out) || nested(o.Out, o.Src) {
		return o, "", reject("profile_stale")
	}
	if o.GraphBin, err = resolveBinary(o.GraphBin, filepath.Join(filepath.Dir(self), "ckg")); err != nil {
		return o, "", err
	}
	if o.VectorBin, err = resolveBinary(o.VectorBin, filepath.Join(filepath.Dir(self), "ckv")); err != nil {
		return o, "", err
	}
	if o.CksBin, err = resolveBinary(o.CksBin, self); err != nil {
		return o, "", err
	}
	if o.CodeRoot != "" && safeDirectory(o.CodeRoot, false) != nil {
		return o, "", reject("profile_stale")
	}
	// Bind config bytes, normalized paths/policies and actual engine binaries.
	raw, err := os.ReadFile(p.SetupConfig)
	if err != nil {
		return o, "", reject("profile_stale")
	}
	fileHash := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	binaries := map[string]string{}
	for role, path := range map[string]string{"graph": o.GraphBin, "vector": o.VectorBin, "cks": o.CksBin} {
		data, err := os.ReadFile(path)
		if err != nil {
			return o, "", reject("unsupported_capability")
		}
		binaries[role] = fileHash(data)
	}
	digest, err := Digest(map[string]any{"profile": p, "config_bytes": fileHash(raw), "resolved_options": o, "binaries": binaries}, "cks.dataset-profile.v1")
	return o, digest, err
}

// Attach only existing reviewed/rendered inputs. inspect_profile never renders
// domain files or mutates the source. A missing/stale rendering fails explicitly.
func attachInputs(o *setup.Options) error {
	manifestPath := filepath.Join(o.Src, ".cks", "knowledge", "manifest.yaml")
	if _, err := os.Lstat(manifestPath); err == nil {
		lock, err := knowledgepack.VerifyLock(o.Src)
		if err != nil || lock.ProjectID != o.ProjectID {
			return reject("input_stale")
		}
		manifest, err := knowledgepack.ReadManifest(o.Src)
		if err != nil {
			return reject("input_stale")
		}
		o.KnowledgeInputs = append(o.KnowledgeInputs, setup.KnowledgeInput{Role: "knowledge-overlay", Path: filepath.Dir(manifestPath)})
		for _, selected := range manifest.SelectedPacks {
			o.ExternalOrigins = append(o.ExternalOrigins, setup.CaptureOrigin{ID: "knowledge:" + selected.PackID, Root: filepath.Join(o.Src, selected.Source)})
			o.KnowledgeInputs = append(o.KnowledgeInputs, setup.KnowledgeInput{Role: "knowledge-pack:" + selected.PackID, Path: filepath.Join(o.Src, selected.Source)})
		}
	} else if !os.IsNotExist(err) {
		return reject("input_stale")
	}
	roots := []string{}
	if o.DomainKnowledge != "" {
		roots = append(roots, o.DomainCorpusDir())
	}
	if o.SemanticCorpus != "" {
		roots = append(roots, o.SemanticCorpus)
	}
	if o.FlowCorpus != "" {
		root := filepath.Dir(o.FlowCorpus)
		found := false
		for _, p := range roots {
			found = found || p == root
		}
		if !found {
			roots = append(roots, root)
		}
	}
	for i, root := range roots {
		if safeDirectory(root, false) != nil {
			return reject("input_stale")
		}
		o.ExternalOrigins = append(o.ExternalOrigins, setup.CaptureOrigin{ID: sourcepath.Origin(i), Root: root})
	}
	o.RetainedCorporaPrepared = true
	return nil
}

func sourceIdentity(ctx context.Context, o setup.Options) (setup.SourceIdentity, error) {
	commit := ""
	if o.SourceMode != "snapshot-only" {
		out, err := exec.CommandContext(ctx, "git", "-C", o.Src, "rev-parse", "HEAD").Output()
		if err != nil {
			return setup.SourceIdentity{}, reject("source_stale")
		}
		commit = strings.TrimSpace(string(out))
		if o.SourceMode == "committed" {
			status, err := exec.CommandContext(ctx, "git", "-C", o.Src, "status", "--porcelain").Output()
			if err != nil || len(bytesTrim(status)) != 0 {
				return setup.SourceIdentity{}, reject("source_stale")
			}
			ignored, err := setup.CountIgnoredIndexableContext(ctx, o.Src)
			if err != nil || ignored != 0 {
				return setup.SourceIdentity{}, reject("source_stale")
			}
		}
	}
	identity, err := setup.SnapshotSourceIdentityWithLimits(ctx, o.Src, o.ProjectID, o.SourceMode, commit, o.CaptureLimits(), o.ExternalOrigins...)
	if err != nil {
		return identity, reject("source_stale")
	}
	return identity, nil
}

func bytesTrim(raw []byte) string { return strings.TrimSpace(string(raw)) }

func coordinates(id setup.DatasetIdentity) contract.V2Coordinates {
	return contract.V2Coordinates{ProjectID: id.Source.ProjectID, DatasetID: id.DatasetID, SnapshotID: id.Source.SnapshotID, SourceMode: id.Source.SourceMode, BaseCommit: id.Source.SourceCommit}
}

func currentBase(ctx context.Context, o setup.Options) (Base, error) {
	path := filepath.Join(o.Out, "current")
	label, err := os.Readlink(path)
	if os.IsNotExist(err) {
		return Base{State: "absent"}, nil
	}
	if err != nil || !validVersion(label) {
		return Base{}, reject("base_stale")
	}
	id, err := setup.InspectVersionIdentityContext(ctx, filepath.Join(o.Out, label))
	if err != nil || id == nil {
		return Base{}, reject("base_stale")
	}
	if id.Source.ProjectID != o.ProjectID {
		return Base{}, reject("project_mismatch")
	}
	c := coordinates(*id)
	return Base{State: "pinned", Version: label, Coordinates: &c}, nil
}

func prepare(ctx context.Context, p registered, self string) (prepared, error) {
	o, pd, err := loadProfile(p.spec, self)
	if err != nil {
		return prepared{}, err
	}
	if o.Src != p.src || o.Out != p.out {
		return prepared{}, reject("profile_stale")
	}
	if err := attachInputs(&o); err != nil {
		return prepared{}, err
	}
	source, err := sourceIdentity(ctx, o)
	if err != nil {
		return prepared{}, err
	}
	input, err := setup.ConfiguredInputDigest(o)
	if err != nil {
		return prepared{}, reject("input_stale")
	}
	embedding, err := setup.ResolveEmbeddingIdentity(ctx, o)
	if err != nil {
		return prepared{}, reject("embedding_stale")
	}
	var embeddingValue any
	if err := json.Unmarshal(embedding, &embeddingValue); err != nil {
		return prepared{}, reject("embedding_stale")
	}
	embeddingDigest, err := Digest(embeddingValue, "cks.dataset-embedding.v1")
	if err != nil {
		return prepared{}, err
	}
	id, err := setup.NewDatasetIdentity(source, embedding, input)
	if err != nil {
		return prepared{}, reject("input_stale")
	}
	base, err := currentBase(ctx, o)
	if err != nil {
		return prepared{}, err
	}
	versions := []string{}
	rootOwned, err := ownsRoot(o.Out, o.ProjectID)
	if err != nil {
		return prepared{}, err
	}
	entries, err := os.ReadDir(o.Out)
	if err != nil && !os.IsNotExist(err) {
		return prepared{}, reject("base_stale")
	}
	for _, entry := range entries {
		if validVersion(entry.Name()) && (entry.IsDir() || entry.Type()&os.ModeSymlink != 0) {
			if entry.Type()&os.ModeSymlink != 0 {
				return prepared{}, reject("project_mismatch")
			}
			if err := checkVersionOwner(ctx, o.Out, entry.Name(), o.ProjectID, rootOwned); err != nil {
				return prepared{}, err
			}
			versions = append(versions, entry.Name())
		}
	}
	sort.Strings(versions)
	gates := []string{"source", "identity", "graph", "vector"}
	if len(o.KnowledgeInputs) > 0 {
		gates = append(gates, "knowledge")
	}
	if o.SemanticCorpus != "" || o.DomainKnowledge != "" || o.FlowCorpus != "" {
		gates = append(gates, "semantic")
	}
	o.SnapshotID, o.DatasetID, o.FileManifestDigest, o.CapturePolicyDigest = source.SnapshotID, id.DatasetID, source.FileManifestDigest, source.CapturePolicyDigest
	return prepared{options: o, source: source, identity: id, view: ProfileContext{ProjectID: o.ProjectID, ProfileID: p.spec.ID, ProfileDigest: pd,
		SourcePin: SourcePin{source.SourceMode, source.SnapshotID, source.SourceCommit}, InputDigest: input, EmbeddingDigest: embeddingDigest,
		ObservedBase: base, ExistingVersions: versions, AllowedActivation: append([]string{}, p.spec.AllowedActivation...), RequiredGates: gates, TimeoutMaxSeconds: p.spec.TimeoutSeconds}}, nil
}

func checkRequest(r BuildRequest, view ProfileContext, checkVersion bool) error {
	if r.ProjectID != view.ProjectID {
		return reject("project_mismatch")
	}
	if r.ProfileID != view.ProfileID {
		return reject("profile_unknown")
	}
	if r.ProfileDigest != view.ProfileDigest {
		return reject("profile_stale")
	}
	for _, required := range []string{"source", "identity", "graph", "vector"} {
		found := false
		for _, name := range view.RequiredGates {
			found = found || name == required
		}
		if !found {
			return reject("unsupported_capability")
		}
	}
	if r.SourcePin != view.SourcePin {
		return reject("source_stale")
	}
	if r.InputDigest != view.InputDigest {
		return reject("input_stale")
	}
	if r.EmbeddingDigest != view.EmbeddingDigest {
		return reject("embedding_stale")
	}
	if r.Base.Coordinates != nil && r.Base.Coordinates.ProjectID != r.ProjectID {
		return reject("project_mismatch")
	}
	a, _ := json.Marshal(r.Base)
	b, _ := json.Marshal(view.ObservedBase)
	if string(a) != string(b) {
		return reject("base_stale")
	}
	if checkVersion {
		for _, v := range view.ExistingVersions {
			if strings.EqualFold(v, r.NewVersion) {
				return reject("version_exists")
			}
		}
	}
	allowed := false
	for _, mode := range view.AllowedActivation {
		allowed = allowed || mode == r.Activation
	}
	if !allowed || r.DeadlineSeconds > view.TimeoutMaxSeconds {
		return reject("unsupported_capability")
	}
	return nil
}
