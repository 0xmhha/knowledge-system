package setup

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"
)

// SourceIdentity is calculated from source bytes, not the checkout name or
// absolute path. The format is versioned so a future selection policy cannot
// silently reuse an old snapshot ID.
type SourceIdentity struct {
	ProjectID           string `json:"project_id"`
	SourceMode          string `json:"source_mode"`
	SourceCommit        string `json:"source_commit"`
	FileManifestDigest  string `json:"file_manifest_digest"`
	CapturePolicyDigest string `json:"capture_policy_digest"`
	SnapshotID          string `json:"snapshot_id"`
}

type sourceFile struct {
	OriginID string `json:"origin_id"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

// identityHashFields uses domain separation plus an 8-byte big-endian length
// for each UTF-8 field. Delimiters and JSON key ordering cannot introduce
// tuple collisions. File records are fed in sorted (origin_id,path) order.
func identityHashFields(domain string, fields ...string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(domain + "\x00"))
	var size [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(field))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func fileManifestDigest(files []sourceFile) string {
	fields := make([]string, 0, len(files)*5)
	for _, f := range files {
		fields = append(fields, f.OriginID, f.Path, f.Kind, fmt.Sprintf("%d", f.Size), f.SHA256)
	}
	return identityHashFields("cks.file-manifest.v2", fields...)
}

func capturePolicyDigest(mode string) string {
	// Selection rules are deliberately part of source identity. A change to
	// this value requires a new capture-policy version and reindex.
	selection := "walk-skip-generated-directories"
	if mode == "committed" {
		selection = "git-tracked-all"
	}
	return identityHashFields("cks.capture-policy.v2", "capture-policy-2026-10-01.1", mode,
		"regular-only", "reject-sensitive", "reject-symlinks", selection)
}

func sourceSnapshotID(s SourceIdentity) string {
	return identityHashFields("cks.snapshot.v2", s.ProjectID, s.SourceMode,
		s.SourceCommit, s.FileManifestDigest, s.CapturePolicyDigest)
}

func identityHash(domain string, value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(domain+"\x00"), data...))
	return hex.EncodeToString(sum[:]), nil
}

// CommittedSourceIdentity takes a deterministic, conservative inventory of
// all tracked regular files. It fails on symlinks rather than following one
// outside the source. A later engine-specific file selection can narrow the
// inventory only by introducing a new identity format version.
func CommittedSourceIdentity(root, projectID, commit string) (SourceIdentity, error) {
	return SnapshotSourceIdentity(root, projectID, "committed", commit)
}

// SnapshotSourceIdentity inventories exactly the same paths and bytes as
// CaptureSource without writing blobs. This pre-build value is compared to
// the retained candidate and, for mutable modes, to the source at the gate.
func SnapshotSourceIdentity(root, projectID, mode, commit string) (SourceIdentity, error) {
	if projectID == "" || strings.ContainsRune(projectID, 0) ||
		(mode != "committed" && mode != "working-tree" && mode != "snapshot-only") ||
		(mode == "snapshot-only" && commit != "") ||
		(mode != "snapshot-only" && len(commit) != 40) {
		return SourceIdentity{}, fmt.Errorf("source identity requires a project ID and a commit matching its source mode")
	}
	if mode != "snapshot-only" {
		if head, err := captureHead(root); err != nil || head != commit {
			return SourceIdentity{}, fmt.Errorf("source base commit changed: %v", err)
		}
	}
	paths, err := captureModePaths(root, mode)
	if err != nil {
		return SourceIdentity{}, err
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		return SourceIdentity{}, err
	}
	defer opened.Close()
	files := make([]sourceFile, 0, len(paths))
	for _, path := range paths {
		if path == "" || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") || !utf8.ValidString(path) || strings.ContainsRune(path, '\\') {
			return SourceIdentity{}, fmt.Errorf("unsafe tracked source path %q", path)
		}
		buf, err := readCapturedRegular(opened, path, 1<<62)
		if err != nil {
			return SourceIdentity{}, fmt.Errorf("read tracked source %q: %w", path, err)
		}
		sum := sha256.Sum256(buf)
		files = append(files, sourceFile{OriginID: "repo", Path: filepath.ToSlash(path), Kind: "regular",
			Size: int64(len(buf)), SHA256: hex.EncodeToString(sum[:])})
	}
	if mode != "snapshot-only" {
		if head, err := captureHead(root); err != nil || head != commit {
			return SourceIdentity{}, fmt.Errorf("source base commit changed during inventory: %v", err)
		}
	}
	result := SourceIdentity{ProjectID: projectID, SourceMode: mode, SourceCommit: commit,
		FileManifestDigest: fileManifestDigest(files), CapturePolicyDigest: capturePolicyDigest(mode)}
	result.SnapshotID = sourceSnapshotID(result)
	return result, nil
}

// DatasetIdentity holds build inputs only. Engine output hashes belong in the
// completed manifest and must never feed their own dataset ID.
type DatasetIdentity struct {
	Source            SourceIdentity  `json:"source"`
	EmbeddingIdentity json.RawMessage `json:"embedding_identity"`
	InputDigest       string          `json:"input_digest"`
	DatasetID         string          `json:"dataset_id"`
}

// NewDatasetIdentity preserves the exact model identity emitted by CKV and
// the caller's separately hashed policy/pack inputs. These values can be
// resolved before a build; computing the ID after build is safe only when the
// gate verifies the pre-build values did not drift.
func NewDatasetIdentity(source SourceIdentity, embedding json.RawMessage, inputDigest string) (DatasetIdentity, error) {
	if source.SnapshotID == "" || len(embedding) == 0 || inputDigest == "" || !json.Valid(embedding) {
		return DatasetIdentity{}, fmt.Errorf("dataset identity requires source, embedding and input digest")
	}
	var normalized any
	if err := json.Unmarshal(embedding, &normalized); err != nil {
		return DatasetIdentity{}, err
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return DatasetIdentity{}, err
	}
	result := DatasetIdentity{Source: source, EmbeddingIdentity: canonical, InputDigest: inputDigest}
	recipe := identityHashFields("cks.build-recipe.v2", "schema-contract-2026-10-01.1", string(canonical), inputDigest)
	result.DatasetID = identityHashFields("cks.dataset.v2", source.SnapshotID, recipe)
	return result, nil
}

type inputFile struct {
	Role   string `json:"role"`
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256"`
}

// ConfiguredInputDigest covers optional build inputs by role and byte content.
// It deliberately excludes machine-specific absolute paths and generated
// outputs. A folder uses relative paths so addition/deletion changes the ID.
func ConfiguredInputDigest(o Options) (string, error) {
	inputs := []struct {
		role, path string
		folder     bool
	}{
		{"graph-policy", o.PolicyFile, false},
		{"security-patterns", o.SecurityPatternFile, false},
		{"vector-policy", o.VectorPolicy, false},
		{"filelist", o.FilelistConfig, false},
		{"glossary", o.GlossaryFile, false},
		{"flow", o.FlowCorpus, false},
		{"semantic-corpus", o.SemanticCorpus, true},
		{"domain-knowledge", o.DomainKnowledge, true},
	}
	for _, knowledge := range o.KnowledgeInputs {
		if knowledge.Role == "" || knowledge.Path == "" {
			return "", fmt.Errorf("invalid registered knowledge input")
		}
		inputs = append(inputs, struct {
			role, path string
			folder     bool
		}{role: knowledge.Role, path: knowledge.Path, folder: true})
	}
	var entries []inputFile
	add := func(role, rel, path string) error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("input %s %q is not a regular file", role, path)
		}
		buf, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(buf)
		entries = append(entries, inputFile{Role: role, Path: filepath.ToSlash(rel), SHA256: hex.EncodeToString(sum[:])})
		return nil
	}
	for _, in := range inputs {
		if in.path == "" {
			continue
		}
		if !in.folder {
			if err := add(in.role, "", in.path); err != nil {
				return "", fmt.Errorf("hash %s input: %w", in.role, err)
			}
			continue
		}
		err := filepath.WalkDir(in.path, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(in.path, path)
			if err != nil {
				return err
			}
			return add(in.role, rel, path)
		})
		if err != nil {
			return "", fmt.Errorf("hash %s input: %w", in.role, err)
		}
	}
	for _, bin := range []struct{ role, path string }{
		{"engine-ckg", o.GraphBin}, {"engine-ckv", o.VectorBin}, {"orchestrator-cks", o.CksBin},
	} {
		if bin.path == "" {
			continue
		}
		path := bin.path
		if !strings.ContainsRune(path, filepath.Separator) {
			resolved, err := exec.LookPath(path)
			if err != nil {
				return "", fmt.Errorf("resolve %s binary: %w", bin.role, err)
			}
			path = resolved
		}
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("hash %s binary: %w", bin.role, err)
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			file.Close()
			return "", fmt.Errorf("%s binary is not a regular file: %v", bin.role, err)
		}
		h := sha256.New()
		_, err = io.Copy(h, file)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return "", fmt.Errorf("hash %s binary: %v %v", bin.role, err, closeErr)
		}
		entries = append(entries, inputFile{Role: bin.role, SHA256: hex.EncodeToString(h.Sum(nil))})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Role == entries[j].Role {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Role < entries[j].Role
	})
	return identityHash("cks.build-inputs.v2", struct {
		Platform string      `json:"platform"`
		Contract int         `json:"contract"`
		Files    []inputFile `json:"files"`
	}{runtime.GOOS + "/" + runtime.GOARCH, 2, entries})
}

// PublishCandidateIdentity stamps a completed candidate only after both
// engines have built. The input digest is captured before build; a caller must
// compare it again before promotion. A partial write leaves the candidate
// unpromotable because VerifyCandidateIdentity checks every copy.
func PublishCandidateIdentity(versionDir string, source SourceIdentity, inputDigest string) (DatasetIdentity, error) {
	vectorPath := filepath.Join(versionDir, "vector", "manifest.json")
	var vector map[string]json.RawMessage
	if err := readJSON(vectorPath, &vector); err != nil {
		return DatasetIdentity{}, fmt.Errorf("read vector identity: %w", err)
	}
	embedding, err := vectorEmbeddingIdentity(vector)
	if err != nil {
		return DatasetIdentity{}, err
	}
	identity, err := NewDatasetIdentity(source, embedding, inputDigest)
	if err != nil {
		return DatasetIdentity{}, err
	}
	for _, engine := range []string{"graph", "vector"} {
		path := filepath.Join(versionDir, engine, "manifest.json")
		var manifest map[string]json.RawMessage
		if err := readJSON(path, &manifest); err != nil {
			return DatasetIdentity{}, err
		}
		var commit string
		_ = json.Unmarshal(manifest["src_commit"], &commit)
		if commit != source.SourceCommit {
			return DatasetIdentity{}, fmt.Errorf("%s commit differs from captured source", engine)
		}
		for key, value := range map[string]string{
			"project_id": source.ProjectID, "snapshot_id": source.SnapshotID,
			"dataset_id": identity.DatasetID, "source_mode": source.SourceMode,
			"file_manifest_digest":  source.FileManifestDigest,
			"capture_policy_digest": source.CapturePolicyDigest,
		} {
			if raw := manifest[key]; len(raw) > 0 {
				var native string
				if err := json.Unmarshal(raw, &native); err != nil || native != value {
					return DatasetIdentity{}, fmt.Errorf("%s native %s differs from pre-build identity", engine, key)
				}
			}
			manifest[key], _ = json.Marshal(value)
		}
		if err := writeJSONAtomic(path, manifest); err != nil {
			return DatasetIdentity{}, err
		}
	}
	if err := writeJSONAtomic(filepath.Join(versionDir, "dataset-identity.json"), identity); err != nil {
		return DatasetIdentity{}, err
	}
	return identity, nil
}

func vectorEmbeddingIdentity(vector map[string]json.RawMessage) (json.RawMessage, error) {
	embedding := vector["embedding_identity_v2"]
	if len(embedding) == 0 || string(embedding) == "null" {
		// Legacy/mock providers have no v2 object. This is still an explicit
		// space identity; Ollama must carry its v2 digest and is rejected below.
		var model, checksum string
		_ = json.Unmarshal(vector["embedding_model"], &model)
		_ = json.Unmarshal(vector["embedding_checksum"], &checksum)
		var dim int
		_ = json.Unmarshal(vector["embedding_dim"], &dim)
		if model == "" || checksum == "" || dim <= 0 {
			return nil, fmt.Errorf("vector embedding identity incomplete")
		}
		embedding, _ = json.Marshal(struct {
			Model    string `json:"model"`
			Dim      int    `json:"dim"`
			Checksum string `json:"checksum"`
		}{model, dim, checksum})
	}
	var identityFields struct {
		Provider    string `json:"Provider"`
		ModelDigest string `json:"model_digest"`
	}
	_ = json.Unmarshal(embedding, &identityFields)
	if identityFields.Provider == "ollama" && identityFields.ModelDigest == "" {
		return nil, fmt.Errorf("Ollama model digest missing from vector identity")
	}
	return embedding, nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".identity-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func VerifyCandidateIdentity(versionDir string, expected SourceIdentity, inputDigest string) error {
	path := filepath.Join(versionDir, "dataset-identity.json")
	var identity DatasetIdentity
	if err := readJSON(path, &identity); err != nil {
		return fmt.Errorf("candidate identity: %w", err)
	}
	if identity.Source != expected || identity.InputDigest != inputDigest {
		return fmt.Errorf("candidate project/snapshot/input identity differs from build start")
	}
	if expected.CapturePolicyDigest != "" &&
		(expected.CapturePolicyDigest != capturePolicyDigest(expected.SourceMode) ||
			expected.SnapshotID != sourceSnapshotID(expected)) {
		return fmt.Errorf("candidate source snapshot or capture policy digest invalid")
	}
	recomputed, err := NewDatasetIdentity(identity.Source, identity.EmbeddingIdentity, identity.InputDigest)
	if err != nil || recomputed.DatasetID != identity.DatasetID {
		return fmt.Errorf("candidate dataset identity digest invalid: %v", err)
	}
	for _, engine := range []string{"graph", "vector"} {
		var fields struct {
			ProjectID           string `json:"project_id"`
			SnapshotID          string `json:"snapshot_id"`
			DatasetID           string `json:"dataset_id"`
			FileManifestDigest  string `json:"file_manifest_digest"`
			CapturePolicyDigest string `json:"capture_policy_digest"`
		}
		if err := readJSON(filepath.Join(versionDir, engine, "manifest.json"), &fields); err != nil {
			return err
		}
		if fields.ProjectID != expected.ProjectID || fields.SnapshotID != expected.SnapshotID ||
			fields.DatasetID != identity.DatasetID || fields.FileManifestDigest != expected.FileManifestDigest ||
			fields.CapturePolicyDigest != expected.CapturePolicyDigest {
			return fmt.Errorf("%s candidate identity differs from dataset", engine)
		}
	}
	var vector map[string]json.RawMessage
	if err := readJSON(filepath.Join(versionDir, "vector", "manifest.json"), &vector); err != nil {
		return err
	}
	embedding, err := vectorEmbeddingIdentity(vector)
	if err != nil {
		return err
	}
	fromVector, err := NewDatasetIdentity(expected, embedding, inputDigest)
	if err != nil || fromVector.DatasetID != identity.DatasetID {
		return fmt.Errorf("vector embedding identity differs from candidate dataset: %v", err)
	}
	if err := VerifyRetainedSource(versionDir, expected); err != nil {
		return err
	}
	return nil
}

func verifyVersionIdentityIfPresent(versionDir string) (*DatasetIdentity, error) {
	path := filepath.Join(versionDir, "dataset-identity.json")
	var identity DatasetIdentity
	if err := readJSON(path, &identity); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		var graph struct {
			ProjectID string `json:"project_id"`
		}
		if err := readJSON(filepath.Join(versionDir, "graph", "manifest.json"), &graph); err == nil && graph.ProjectID != "" {
			return nil, fmt.Errorf("pinned graph has no dataset identity record")
		}
		return nil, nil // legacy version
	}
	if err := VerifyCandidateIdentity(versionDir, identity.Source, identity.InputDigest); err != nil {
		return nil, err
	}
	if err := VerifyAlignment(filepath.Join(versionDir, "graph"), filepath.Join(versionDir, "vector"), nil); err != nil {
		return nil, err
	}
	return &identity, nil
}

// InspectVersionIdentity classifies a version without modifying it. A nil
// identity is a readable legacy committed dataset; pinned candidates are
// checked against both engine manifests before details are returned.
func InspectVersionIdentity(versionDir string) (*DatasetIdentity, error) {
	return verifyVersionIdentityIfPresent(versionDir)
}
