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
	SourceMode    string // committed, working-tree or snapshot-only
	SourceCommit  string // full HEAD for Git modes, empty for non-Git
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
// an engine. The output root must be outside the source tree so it cannot
// recursively capture itself.
func CaptureSource(o CaptureOptions) (CapturedSource, error) {
	if o.ProjectID == "" || o.Root == "" || o.Out == "" {
		return CapturedSource{}, fmt.Errorf("capture requires project ID, source root and output root")
	}
	if o.SourceMode != "committed" && o.SourceMode != "working-tree" && o.SourceMode != "snapshot-only" {
		return CapturedSource{}, fmt.Errorf("unsupported capture source mode %q", o.SourceMode)
	}
	if ((o.SourceMode == "committed" || o.SourceMode == "working-tree") && len(o.SourceCommit) != 40) ||
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
	if o.SourceMode == "working-tree" || o.SourceMode == "committed" {
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
	paths, err := captureModePaths(root, o.SourceMode)
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
		buf, err := readCapturedRegular(opened, rel, o.MaxFileBytes)
		if err != nil {
			return CapturedSource{}, err
		}
		if total+int64(len(buf)) > o.MaxTotalBytes {
			return CapturedSource{}, fmt.Errorf("capture total byte limit exceeded at %q", rel)
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
	if o.SourceMode == "working-tree" || o.SourceMode == "committed" {
		if head, err := captureHead(root); err != nil || head != o.SourceCommit {
			return CapturedSource{}, fmt.Errorf("working-tree base commit changed during capture: %v", err)
		}
	}
	files := make([]sourceFile, 0, len(result.Files))
	for _, f := range result.Files {
		files = append(files, sourceFile{Path: f.Path, SHA256: f.SHA256})
	}
	manifest, err := identityHash("cks.file-manifest.v2", files)
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

// readCapturedRegular refuses a symlink in every path component and checks
// that the opened file is the same inode inspected before reading. os.Root
// confines a concurrent path replacement to the source tree; post-read path
// checks and the final inventory pass catch ordinary swaps and byte drift.
func readCapturedRegular(root *os.Root, rel string, maxBytes int64) ([]byte, error) {
	parts := strings.Split(rel, "/")
	for i := range parts {
		prefix := filepath.FromSlash(strings.Join(parts[:i+1], "/"))
		info, err := root.Lstat(prefix)
		if err != nil {
			return nil, fmt.Errorf("source changed before capture %q: %w", rel, err)
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return nil, fmt.Errorf("capture refused non-directory or linked component in %q", rel)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("capture refused non-regular source %q", rel)
		}
		if info.Size() > maxBytes {
			return nil, fmt.Errorf("capture file byte limit exceeded at %q", rel)
		}
		file, err := root.Open(filepath.FromSlash(rel))
		if err != nil {
			return nil, fmt.Errorf("open captured source %q: %w", rel, err)
		}
		openedInfo, statErr := file.Stat()
		if statErr != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
			file.Close()
			return nil, fmt.Errorf("source changed while opening %q", rel)
		}
		buf, readErr := io.ReadAll(io.LimitReader(file, maxBytes+1))
		closeErr := file.Close()
		postInfo, postErr := root.Lstat(filepath.FromSlash(rel))
		if readErr != nil || closeErr != nil || postErr != nil ||
			!postInfo.Mode().IsRegular() || !os.SameFile(openedInfo, postInfo) ||
			int64(len(buf)) != info.Size() {
			return nil, fmt.Errorf("source changed while capturing %q", rel)
		}
		return buf, nil
	}
	return nil, fmt.Errorf("empty capture path")
}

func captureHead(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	buf, err := cmd.Output()
	return strings.TrimSpace(string(buf)), err
}

func captureModePaths(root, mode string) ([]string, error) {
	if mode != "committed" {
		return capturePaths(root)
	}
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--cached")
	listed, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list committed paths: %w", err)
	}
	if len(listed) == 0 {
		return nil, nil
	}
	paths := strings.Split(strings.TrimSuffix(string(listed), "\x00"), "\x00")
	for _, path := range paths {
		if path == "" || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") ||
			strings.ContainsRune(path, '\\') || !utf8.ValidString(path) || sensitiveCapturePath(path) {
			return nil, fmt.Errorf("unsafe committed source path %q", path)
		}
	}
	sort.Strings(paths)
	return paths, nil
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
	if c.Identity.SourceMode == "working-tree" || c.Identity.SourceMode == "committed" {
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
	paths, err := captureModePaths(root, c.Identity.SourceMode)
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
		buf, err := readCapturedRegular(opened, f.Path, f.Size)
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
	path := filepath.Join(c.BlobDir, digest)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("source_missing: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("snapshot_mismatch: retained source blob is not a regular file")
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("source_missing: %w", err)
	}
	sum := sha256.Sum256(buf)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, fmt.Errorf("snapshot_mismatch: retained source blob changed")
	}
	return buf, nil
}

// VerifyRetainedSource checks a candidate's optional archive. Older pinned
// versions created before source retention remain readable, while any archive
// that is present must match the immutable source identity and every blob.
func VerifyRetainedSource(versionDir string, expected SourceIdentity) error {
	sources := filepath.Join(versionDir, "sources")
	info, err := os.Lstat(sources)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.IsDir() {
		return fmt.Errorf("snapshot_mismatch: source archive is not a directory: %v", err)
	}
	var captured CapturedSource
	if err := readJSON(filepath.Join(sources, "manifest.json"), &captured); err != nil {
		return fmt.Errorf("source_missing: retained source manifest: %w", err)
	}
	if captured.Identity != expected || len(captured.Files) == 0 {
		return fmt.Errorf("snapshot_mismatch: retained source identity differs from candidate")
	}
	entries := make([]sourceFile, 0, len(captured.Files))
	prev := ""
	for _, f := range captured.Files {
		if f.Path == "" || filepath.IsAbs(f.Path) || f.Path == ".." ||
			strings.HasPrefix(f.Path, "../") || strings.ContainsRune(f.Path, '\\') ||
			!utf8.ValidString(f.Path) || f.Path <= prev || f.Size < 0 {
			return fmt.Errorf("snapshot_mismatch: invalid retained source path")
		}
		prev = f.Path
		entries = append(entries, sourceFile{Path: f.Path, SHA256: f.SHA256})
	}
	digest, err := identityHash("cks.file-manifest.v2", entries)
	if err != nil || digest != expected.FileManifestDigest {
		return fmt.Errorf("snapshot_mismatch: retained file inventory differs from candidate")
	}
	blobs := filepath.Join(sources, "blobs")
	info, err = os.Lstat(blobs)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source_missing: retained blobs directory: %v", err)
	}
	captured.BlobDir = blobs
	return captured.VerifyBlobs()
}

// VerifyBlobs rechecks retained bytes before a candidate is promoted. A
// manifest alone is insufficient when a blob was deleted or replaced during
// a long graph/vector build.
func (c CapturedSource) VerifyBlobs() error {
	for _, f := range c.Files {
		buf, err := c.ReadBlob(f.SHA256)
		if err != nil {
			return fmt.Errorf("retained source %q: %w", f.Path, err)
		}
		if int64(len(buf)) != f.Size {
			return fmt.Errorf("snapshot_mismatch: retained source %q changed size", f.Path)
		}
	}
	return nil
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
