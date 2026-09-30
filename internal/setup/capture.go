package setup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// CapturedFile is one immutable source byte sequence. Paths are source-root
// relative POSIX paths. Blobs are stored by digest independently of a Git
// checkout so old citations do not need a live working tree.
type CapturedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type CaptureOptions struct {
	Root          string
	Out           string
	ProjectID     string
	SourceMode    string // working-tree or snapshot-only
	SourceCommit  string // full HEAD for working-tree, empty for non-Git
	MaxFileBytes  int64
	MaxTotalBytes int64
}

type CapturedSource struct {
	Identity SourceIdentity `json:"identity"`
	Files    []CapturedFile `json:"files"`
	BlobDir  string         `json:"-"`
	Root     string         `json:"-"`
}

// CaptureSource stores a deterministic byte snapshot without exposing it to
// an engine. A4 will provide staging and v2 citation consumers only after
// CKG/CKV input reconciliation and no-follow tests pass. The output root must
// be outside the source tree so it cannot recursively capture itself.
func CaptureSource(o CaptureOptions) (CapturedSource, error) {
	if o.ProjectID == "" || o.Root == "" || o.Out == "" {
		return CapturedSource{}, fmt.Errorf("capture requires project ID, source root and output root")
	}
	if o.SourceMode != "working-tree" && o.SourceMode != "snapshot-only" {
		return CapturedSource{}, fmt.Errorf("unsupported capture source mode %q", o.SourceMode)
	}
	if (o.SourceMode == "working-tree" && len(o.SourceCommit) != 40) ||
		(o.SourceMode == "snapshot-only" && o.SourceCommit != "") {
		return CapturedSource{}, fmt.Errorf("capture source commit does not match mode")
	}
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return CapturedSource{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return CapturedSource{}, err
	}
	if o.SourceMode == "working-tree" {
		if head, err := captureHead(root); err != nil || head != o.SourceCommit {
			return CapturedSource{}, fmt.Errorf("working-tree base commit changed before capture: %v", err)
		}
	}
	out, err := filepath.Abs(o.Out)
	if err != nil {
		return CapturedSource{}, err
	}
	out, err = resolvedCapturePath(out)
	if err != nil {
		return CapturedSource{}, err
	}
	if rel, err := filepath.Rel(root, out); err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return CapturedSource{}, fmt.Errorf("capture output must be outside source tree")
	}
	if o.MaxFileBytes <= 0 {
		o.MaxFileBytes = 32 << 20
	}
	if o.MaxTotalBytes <= 0 {
		o.MaxTotalBytes = 2 << 30
	}
	paths, err := capturePaths(root)
	if err != nil {
		return CapturedSource{}, err
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		return CapturedSource{}, err
	}
	defer opened.Close()
	blobDir := filepath.Join(out, "sources", "blobs")
	if err := os.MkdirAll(blobDir, 0o700); err != nil {
		return CapturedSource{}, err
	}
	result := CapturedSource{BlobDir: blobDir, Root: root}
	var total int64
	for _, rel := range paths {
		info, err := opened.Lstat(filepath.FromSlash(rel))
		if err != nil || !info.Mode().IsRegular() {
			return CapturedSource{}, fmt.Errorf("source changed before capture %q: %v", rel, err)
		}
		if info.Size() > o.MaxFileBytes || total+info.Size() > o.MaxTotalBytes {
			return CapturedSource{}, fmt.Errorf("capture byte limit exceeded at %q", rel)
		}
		file, err := opened.Open(filepath.FromSlash(rel))
		if err != nil {
			return CapturedSource{}, err
		}
		buf, readErr := io.ReadAll(io.LimitReader(file, o.MaxFileBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || int64(len(buf)) != info.Size() {
			return CapturedSource{}, fmt.Errorf("source changed while capturing %q", rel)
		}
		sum := sha256.Sum256(buf)
		digest := hex.EncodeToString(sum[:])
		blob := filepath.Join(blobDir, digest)
		if err := writeCapturedBlob(blob, buf); err != nil {
			return CapturedSource{}, err
		}
		result.Files = append(result.Files, CapturedFile{Path: rel, SHA256: digest, Size: int64(len(buf))})
		total += int64(len(buf))
	}
	if err := result.VerifyAgainst(root); err != nil {
		return CapturedSource{}, err
	}
	if o.SourceMode == "working-tree" {
		if head, err := captureHead(root); err != nil || head != o.SourceCommit {
			return CapturedSource{}, fmt.Errorf("working-tree base commit changed during capture: %v", err)
		}
	}
	manifest, err := identityHash("cks.file-manifest.v2", result.Files)
	if err != nil {
		return CapturedSource{}, err
	}
	result.Identity = SourceIdentity{ProjectID: o.ProjectID, SourceMode: o.SourceMode,
		SourceCommit: o.SourceCommit, FileManifestDigest: manifest}
	result.Identity.SnapshotID, err = identityHash("cks.snapshot.v2", result.Identity)
	if err != nil {
		return CapturedSource{}, err
	}
	if err := writeJSONAtomic(filepath.Join(out, "sources", "manifest.json"), result); err != nil {
		return CapturedSource{}, err
	}
	return result, nil
}

func captureHead(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	buf, err := cmd.Output()
	return strings.TrimSpace(string(buf)), err
}

// MaterializeBuildTree creates a disposable copy from retained blobs. A Git
// working-tree capture starts with a detached worktree so CKG can still read
// the base history; snapshot-only mode has no Git metadata. This adapter is
// not yet exposed through setup: logical roots and v2 citations must land
// before either mode can be advertised as supported.
func (c CapturedSource) MaterializeBuildTree(path string) (func() error, error) {
	if c.Identity.SnapshotID == "" || c.Root == "" {
		return nil, fmt.Errorf("materialize requires a captured source")
	}
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("build root already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	cleanup := func() error { return os.RemoveAll(path) }
	if c.Identity.SourceMode == "working-tree" {
		if head, err := captureHead(c.Root); err != nil || head != c.Identity.SourceCommit {
			return nil, fmt.Errorf("working-tree base commit changed before materialization: %v", err)
		}
		cmd := exec.Command("git", "-C", c.Root, "worktree", "add", "--detach", path, c.Identity.SourceCommit)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("create temporary Git worktree: %w: %s", err, out)
		}
		cleanup = func() error {
			cmd := exec.Command("git", "-C", c.Root, "worktree", "remove", "--force", path)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("remove temporary worktree: %w: %s", err, out)
			}
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return cleanup, err
		}
		for _, entry := range entries {
			if entry.Name() == ".git" {
				continue
			}
			if err := os.RemoveAll(filepath.Join(path, entry.Name())); err != nil {
				return cleanup, err
			}
		}
	} else if err := os.Mkdir(path, 0o700); err != nil {
		return nil, err
	}
	for _, f := range c.Files {
		buf, err := c.ReadBlob(f.SHA256)
		if err != nil {
			return cleanup, err
		}
		file := filepath.Join(path, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			return cleanup, err
		}
		if err := os.WriteFile(file, buf, 0o600); err != nil {
			return cleanup, err
		}
	}
	return cleanup, nil
}

func capturePaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !utf8.ValidString(rel) || strings.ContainsRune(rel, '\\') {
			return fmt.Errorf("capture path is not portable: %q", rel)
		}
		name := entry.Name()
		if entry.IsDir() {
			if skipCaptureDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".git" { // linked Git worktree metadata is not source
			return nil
		}
		if sensitiveCapturePath(rel) {
			return fmt.Errorf("capture refused sensitive path %q", rel)
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("capture refused non-regular path %q", rel)
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func (c CapturedSource) VerifyAgainst(root string) error {
	paths, err := capturePaths(root)
	if err != nil {
		return err
	}
	if len(paths) != len(c.Files) {
		return fmt.Errorf("source file set changed after capture")
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer opened.Close()
	for i, f := range c.Files {
		if paths[i] != f.Path {
			return fmt.Errorf("source file set changed after capture")
		}
		info, err := opened.Lstat(filepath.FromSlash(f.Path))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("source changed after capture %q: %v", f.Path, err)
		}
		buf, err := opened.ReadFile(filepath.FromSlash(f.Path))
		if err != nil {
			return fmt.Errorf("source changed after capture %q: %w", f.Path, err)
		}
		sum := sha256.Sum256(buf)
		if int64(len(buf)) != f.Size || hex.EncodeToString(sum[:]) != f.SHA256 {
			return fmt.Errorf("source changed after capture %q", f.Path)
		}
	}
	return nil
}

func (c CapturedSource) ReadBlob(digest string) ([]byte, error) {
	if len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" {
		return nil, fmt.Errorf("invalid blob digest")
	}
	buf, err := os.ReadFile(filepath.Join(c.BlobDir, digest))
	if err != nil {
		return nil, fmt.Errorf("source_missing: %w", err)
	}
	sum := sha256.Sum256(buf)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, fmt.Errorf("snapshot_mismatch: retained source blob changed")
	}
	return buf, nil
}

func writeCapturedBlob(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
	if err == nil {
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(path)
			return fmt.Errorf("write retained blob: %v %v", writeErr, closeErr)
		}
		return nil
	}
	if !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("snapshot_mismatch: blob path is not a regular file: %v", err)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(old, data) {
		return fmt.Errorf("snapshot_mismatch: blob path contains different bytes")
	}
	return nil
}

func skipCaptureDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", ".venv", "__pycache__", ".next", ".cache":
		return true
	}
	return false
}

func sensitiveCapturePath(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	return base == ".env" || strings.HasPrefix(base, ".env.") || base == "id_rsa" ||
		strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") ||
		strings.HasPrefix(rel, ".aws/") || strings.HasPrefix(rel, "secrets/")
}

func resolvedCapturePath(path string) (string, error) {
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}
