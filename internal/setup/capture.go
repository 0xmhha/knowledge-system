package setup

import (
	"bufio"
	"bytes"
	"context"
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

	"github.com/0xmhha/knowledge-system/internal/githistory"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// CapturedFile is one immutable source byte sequence and executable bit.
// Paths are source-root relative POSIX paths. Blobs are stored by digest
// independently of a Git checkout so old citations do not need a live tree.
type CapturedFile struct {
	OriginID   string `json:"origin_id"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	Executable bool   `json:"executable,omitempty"`
}

type CaptureOptions struct {
	Root            string
	Out             string
	ProjectID       string
	SourceMode      string // committed, working-tree or snapshot-only
	SourceCommit    string // full HEAD for Git modes, empty for non-Git
	MaxFileBytes    int64
	MaxTotalBytes   int64
	MaxHistoryBytes int64
	MaxFiles        int
	ExternalOrigins []CaptureOrigin
	MinFreeBytes    int64
}

const (
	defaultCaptureMaxFiles      = 100_000
	defaultCaptureMaxFileBytes  = 32 << 20
	defaultCaptureMaxTotalBytes = 4 << 30
)

type CaptureOrigin struct {
	ID   string
	Root string
}

type CapturedSource struct {
	Identity    SourceIdentity      `json:"identity"`
	Files       []CapturedFile      `json:"files"`
	GitHistory  *CapturedGitHistory `json:"git_history,omitempty"`
	BlobDir     string              `json:"-"`
	Root        string              `json:"-"`
	OriginRoots map[string]string   `json:"-"`
}

// CaptureSource stores a deterministic byte and mode snapshot without exposing it to
// an engine. The output root must be outside the source tree so it cannot
// recursively capture itself.
func CaptureSource(o CaptureOptions) (CapturedSource, error) {
	return CaptureSourceContext(context.Background(), o)
}

func CaptureSourceContext(ctx context.Context, o CaptureOptions) (CapturedSource, error) {
	limits, err := (CaptureLimits{MaxFiles: o.MaxFiles, MaxFileBytes: o.MaxFileBytes, MaxTotalBytes: o.MaxTotalBytes}).Normalized()
	if err != nil {
		return CapturedSource{}, err
	}
	o.MaxFiles, o.MaxFileBytes, o.MaxTotalBytes = limits.MaxFiles, limits.MaxFileBytes, limits.MaxTotalBytes
	if err := ctx.Err(); err != nil {
		return CapturedSource{}, err
	}
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
		if head, err := captureHeadContext(ctx, root); err != nil || head != o.SourceCommit {
			return CapturedSource{}, fmt.Errorf("working-tree base commit changed before capture: %v", err)
		}
	}
	recoveryBefore, err := gitRecoveryDigestContext(ctx, root, o.SourceMode)
	if err != nil {
		return CapturedSource{}, err
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
	origins, err := prepareCaptureOrigins(o.ExternalOrigins, out)
	if err != nil {
		return CapturedSource{}, err
	}
	if o.MaxFileBytes <= 0 {
		o.MaxFileBytes = defaultCaptureMaxFileBytes
	}
	if o.MaxTotalBytes <= 0 {
		o.MaxTotalBytes = defaultCaptureMaxTotalBytes
	}
	if o.MaxFiles <= 0 {
		o.MaxFiles = defaultCaptureMaxFiles
	}
	paths, err := captureModePathsContext(ctx, root, o.SourceMode, o.MaxFiles)
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
	result := CapturedSource{BlobDir: blobDir, Root: root, OriginRoots: map[string]string{},
		Identity: SourceIdentity{SourceMode: o.SourceMode, FileModePolicy: fileModePolicyV1}}
	for _, origin := range origins {
		result.OriginRoots[origin.ID] = origin.Root
	}
	var total int64
	add := func(originID, rel string, buf []byte, executable bool) error {
		if len(result.Files) >= o.MaxFiles {
			return fmt.Errorf("%w: capture file count limit exceeded: limit=%d", ErrResourceLimit, o.MaxFiles)
		}
		if int64(len(buf)) > o.MaxTotalBytes-total {
			return fmt.Errorf("%w: capture total byte limit exceeded", ErrResourceLimit)
		}
		sum := sha256.Sum256(buf)
		digest := hex.EncodeToString(sum[:])
		blob := filepath.Join(blobDir, digest)
		if err := checkOutputSpace(ctx, out, int64(len(buf)), o.MinFreeBytes); err != nil {
			return err
		}
		if err := writeCapturedBlob(blob, buf); err != nil {
			return err
		}
		result.Files = append(result.Files, CapturedFile{OriginID: originID, Path: rel,
			Kind: "regular", SHA256: digest, Size: int64(len(buf)), Executable: executable})
		total += int64(len(buf))
		return nil
	}
	for _, rel := range paths {
		buf, executable, err := readCapturedRegularWithModeContext(ctx, opened, rel, o.MaxFileBytes, nil)
		if err != nil {
			return CapturedSource{}, err
		}
		if err := add("repo", rel, buf, executable); err != nil {
			return CapturedSource{}, err
		}
	}
	for _, origin := range origins {
		externalPaths, err := captureExternalPathsContext(ctx, origin.Root, o.MaxFiles)
		if err != nil {
			return CapturedSource{}, err
		}
		external, err := os.OpenRoot(origin.Root)
		if err != nil {
			return CapturedSource{}, err
		}
		for _, rel := range externalPaths {
			buf, executable, readErr := readCapturedRegularWithModeContext(ctx, external, rel, o.MaxFileBytes, nil)
			if readErr != nil {
				external.Close()
				return CapturedSource{}, readErr
			}
			if err := add(origin.ID, rel, buf, executable); err != nil {
				external.Close()
				return CapturedSource{}, err
			}
		}
		external.Close()
	}
	sort.Slice(result.Files, func(i, j int) bool {
		if result.Files[i].OriginID == result.Files[j].OriginID {
			return result.Files[i].Path < result.Files[j].Path
		}
		return result.Files[i].OriginID < result.Files[j].OriginID
	})
	if err := result.VerifyAgainstContext(ctx, root); err != nil {
		return CapturedSource{}, err
	}
	if o.SourceMode == "working-tree" || o.SourceMode == "committed" {
		if head, err := captureHeadContext(ctx, root); err != nil || head != o.SourceCommit {
			return CapturedSource{}, fmt.Errorf("working-tree base commit changed during capture: %v", err)
		}
	}
	recoveryAfter, err := gitRecoveryDigestContext(ctx, root, o.SourceMode)
	if err != nil {
		return CapturedSource{}, err
	}
	if recoveryBefore != recoveryAfter {
		return CapturedSource{}, fmt.Errorf("Git recovery input changed during capture")
	}
	files := make([]sourceFile, 0, len(result.Files))
	for _, f := range result.Files {
		files = append(files, sourceFile{OriginID: f.OriginID, Path: f.Path,
			Kind: f.Kind, Size: f.Size, SHA256: f.SHA256, Executable: f.Executable})
	}
	manifest := fileManifestDigestForPolicy(files, fileModePolicyV1)
	result.Identity = SourceIdentity{ProjectID: o.ProjectID, SourceMode: o.SourceMode,
		SourceCommit: o.SourceCommit, FileManifestDigest: manifest,
		CapturePolicyDigest: capturePolicyDigest(o.SourceMode), GitRecoveryDigest: recoveryAfter,
		FileModePolicy: fileModePolicyV1}
	if o.SourceMode != "snapshot-only" {
		result.Identity.GitArchivePolicy = gitArchivePolicyV1
		selected, err := githistory.RecoveryCommitIDsContext(ctx, root, 0)
		if err != nil {
			return CapturedSource{}, err
		}
		if identityHashFields("cks.git-recovery.v1", selected...) != recoveryAfter {
			return CapturedSource{}, fmt.Errorf("Git recovery input changed before archive")
		}
		result.GitHistory, err = captureGitHistoryContext(ctx, root, filepath.Join(out, "sources", "history.bundle"),
			o.SourceCommit, selected, o.MaxHistoryBytes)
		if err != nil {
			return CapturedSource{}, err
		}
		finalRecovery, err := gitRecoveryDigestContext(ctx, root, o.SourceMode)
		if err != nil || finalRecovery != recoveryAfter {
			return CapturedSource{}, fmt.Errorf("Git recovery input changed while archiving: %v", err)
		}
	}
	result.Identity.SnapshotID = sourceSnapshotID(result.Identity)
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
	return readCapturedRegularWithOpenHook(root, rel, maxBytes, nil)
}

// The hook is used by tests to force a replacement at the exact Lstat/Open
// boundary. Production capture always passes nil.
func readCapturedRegularWithOpenHook(root *os.Root, rel string, maxBytes int64, beforeOpen func()) ([]byte, error) {
	buf, _, err := readCapturedRegularWithMode(root, rel, maxBytes, beforeOpen)
	return buf, err
}

func readCapturedRegularWithMode(root *os.Root, rel string, maxBytes int64, beforeOpen func()) ([]byte, bool, error) {
	return readCapturedRegularWithModeContext(context.Background(), root, rel, maxBytes, beforeOpen)
}

func readCapturedRegularWithModeContext(ctx context.Context, root *os.Root, rel string, maxBytes int64, beforeOpen func()) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	parts := strings.Split(rel, "/")
	for i := range parts {
		prefix := filepath.FromSlash(strings.Join(parts[:i+1], "/"))
		info, err := root.Lstat(prefix)
		if err != nil {
			return nil, false, fmt.Errorf("source changed before capture %q: %w", rel, err)
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return nil, false, fmt.Errorf("capture refused non-directory or linked component in %q", rel)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, false, fmt.Errorf("capture refused non-regular source %q: not a regular file", rel)
		}
		if info.Size() > maxBytes {
			return nil, false, fmt.Errorf("%w: capture file byte limit exceeded", ErrResourceLimit)
		}
		if beforeOpen != nil {
			beforeOpen()
		}
		file, err := openCapturedNoFollow(root.Name(), rel)
		if err != nil {
			return nil, false, fmt.Errorf("open captured source %q: %w", rel, err)
		}
		openedInfo, statErr := file.Stat()
		if statErr != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
			file.Close()
			return nil, false, fmt.Errorf("source changed while opening %q", rel)
		}
		buf, readErr := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, reader: file}, maxBytes+1))
		closeErr := file.Close()
		postInfo, postErr := root.Lstat(filepath.FromSlash(rel))
		if readErr != nil {
			return nil, false, readErr
		}
		if readErr != nil || closeErr != nil || postErr != nil ||
			!postInfo.Mode().IsRegular() || !os.SameFile(openedInfo, postInfo) ||
			int64(len(buf)) != info.Size() ||
			info.Mode().Perm()&0o111 != openedInfo.Mode().Perm()&0o111 ||
			info.Mode().Perm()&0o111 != postInfo.Mode().Perm()&0o111 {
			return nil, false, fmt.Errorf("source changed while capturing %q", rel)
		}
		return buf, info.Mode().Perm()&0o111 != 0, nil
	}
	return nil, false, fmt.Errorf("empty capture path")
}

// ReadSourceFileNoFollow is the shared byte reader for registered local
// knowledge packs. It applies the same portable path and no-follow checks as
// source capture; callers still decide which paths their policy permits.
func ReadSourceFileNoFollow(rootPath, rel string, maxBytes int64) ([]byte, error) {
	if err := validateCapturedPaths([]string{rel}); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readCapturedRegular(root, rel, maxBytes)
}

func captureHead(root string) (string, error) { return captureHeadContext(context.Background(), root) }

func captureHeadContext(ctx context.Context, root string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	buf, err := cmd.Output()
	return strings.TrimSpace(string(buf)), err
}

func captureModePaths(root, mode string) ([]string, error) {
	return captureModePathsContext(context.Background(), root, mode, 0)
}

func captureModePathsContext(ctx context.Context, root, mode string, maxFiles int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if mode != "committed" {
		return capturePathsContext(ctx, root, maxFiles)
	}
	cmd := buildCommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = pipe.Close() })
	defer stop()
	scan := bufio.NewScanner(pipe)
	scan.Buffer(make([]byte, 4096), 64*1024)
	scan.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if n := bytes.IndexByte(data, 0); n >= 0 {
			return n + 1, data[:n], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	var paths []string
	var scanErr error
	for scan.Scan() {
		if maxFiles > 0 && len(paths) >= maxFiles {
			scanErr = fmt.Errorf("%w: source file count exceeded", ErrResourceLimit)
			break
		}
		paths = append(paths, scan.Text())
	}
	if scanErr == nil {
		scanErr = scan.Err()
	}
	if scanErr != nil {
		_ = cmd.Cancel()
	}
	waitErr := cmd.Wait()
	if scanErr != nil {
		return nil, scanErr
	}
	if waitErr != nil {
		return nil, fmt.Errorf("list committed paths: %w", waitErr)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, path := range paths {
		if path == "" || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") ||
			strings.ContainsRune(path, '\\') || !utf8.ValidString(path) || sensitiveCapturePath(path) {
			return nil, fmt.Errorf("unsafe committed source path %q", path)
		}
	}
	sort.Strings(paths)
	if err := validateCapturedPaths(paths); err != nil {
		return nil, err
	}
	return paths, nil
}

func validateCapturedPaths(paths []string) error {
	seen := make(map[string]string, len(paths))
	fold := cases.Fold()
	for _, path := range paths {
		if path == "" || !utf8.ValidString(path) || !norm.NFC.IsNormalString(path) ||
			filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path ||
			path == ".." || strings.HasPrefix(path, "../") ||
			strings.ContainsRune(path, '\\') || strings.ContainsRune(path, 0) {
			return fmt.Errorf("capture path is not portable: %q", path)
		}
		key := fold.String(path)
		if old, ok := seen[key]; ok {
			return fmt.Errorf("capture refused case-colliding paths %q and %q", old, path)
		}
		seen[key] = path
	}
	return nil
}

// MaterializeBuildTree creates a disposable copy from retained blobs. New
// Git captures restore their sealed bundle into an independent repository;
// older v2/v3 captures retain their historical shared-worktree behavior.
func (c CapturedSource) MaterializeBuildTree(path string) (func() error, error) {
	return c.MaterializeBuildTreeContext(context.Background(), path)
}

func (c CapturedSource) MaterializeBuildTreeContext(ctx context.Context, path string) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.Identity.SnapshotID == "" || (c.Identity.GitArchivePolicy == "" && c.Root == "") {
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
		if c.Identity.GitArchivePolicy != "" {
			if err := c.materializeGitHistoryContext(ctx, path); err != nil {
				return cleanup, err
			}
		} else {
			if head, err := captureHeadContext(ctx, c.Root); err != nil || head != c.Identity.SourceCommit {
				return nil, fmt.Errorf("working-tree base commit changed before materialization: %v", err)
			}
			cmd := exec.CommandContext(ctx, "git", "-C", c.Root, "worktree", "add", "--detach", path, c.Identity.SourceCommit)
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
		}
		if c.Identity.SourceMode == "committed" {
			// HEAD already pins both bytes and Git executable bits. The caller
			// verifies every retained byte against this independent checkout.
			if err := c.VerifyBlobsContext(ctx); err != nil {
				return cleanup, err
			}
			return cleanup, nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return cleanup, err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return cleanup, err
			}
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
		if f.OriginID != "repo" {
			continue
		}
		buf, err := c.ReadBlobContext(ctx, f.SHA256)
		if err != nil {
			return cleanup, err
		}
		file := filepath.Join(path, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			return cleanup, err
		}
		mode := os.FileMode(0o600)
		if c.Identity.FileModePolicy == fileModePolicyV1 && f.Executable {
			mode = 0o700
		}
		if err := os.WriteFile(file, buf, mode); err != nil {
			return cleanup, err
		}
	}
	return cleanup, nil
}

func capturePaths(root string) ([]string, error) {
	return capturePathsContext(context.Background(), root, 0)
}

func capturePathsContext(ctx context.Context, root string, maxFiles int) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
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
		if maxFiles > 0 && len(paths) >= maxFiles {
			return fmt.Errorf("%w: source file count exceeded", ErrResourceLimit)
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if err := validateCapturedPaths(paths); err != nil {
		return nil, err
	}
	return paths, nil
}

func validCaptureOriginID(id string) bool {
	if id == "ontology" || id == "spec" {
		return true
	}
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return false
	}
	switch parts[0] {
	case "knowledge", "docs", "policy", "flow":
	default:
		return false
	}
	for _, r := range parts[1] {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func prepareCaptureOrigins(inputs []CaptureOrigin, out string) ([]CaptureOrigin, error) {
	result := make([]CaptureOrigin, 0, len(inputs))
	seen := map[string]bool{}
	for _, input := range inputs {
		if !validCaptureOriginID(input.ID) || input.Root == "" || seen[input.ID] {
			return nil, fmt.Errorf("invalid or duplicate external source origin %q", input.ID)
		}
		seen[input.ID] = true
		root, err := filepath.Abs(input.Root)
		if err != nil {
			return nil, err
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("external source root %q is unavailable", input.ID)
		}
		if out != "" {
			if rel, err := filepath.Rel(root, out); err != nil || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("capture output must be outside external source %q", input.ID)
			}
		}
		result = append(result, CaptureOrigin{ID: input.ID, Root: root})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func captureExternalPaths(root string) ([]string, error) {
	return captureExternalPathsContext(context.Background(), root, 0)
}

func captureExternalPathsContext(ctx context.Context, root string, maxFiles int) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
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
		rel = filepath.ToSlash(rel)
		for _, part := range strings.Split(rel, "/") {
			if strings.HasPrefix(part, ".") {
				return fmt.Errorf("external source has hidden path %q", rel)
			}
		}
		if sensitiveCapturePath(rel) {
			return fmt.Errorf("external source has sensitive path %q", rel)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("external source has linked path %q", rel)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("external source has nonregular path %q", rel)
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("external source is empty")
	}
	sort.Strings(paths)
	if err := validateCapturedPaths(paths); err != nil {
		return nil, err
	}
	return paths, nil
}

func (c CapturedSource) VerifyAgainst(root string) error {
	return c.VerifyAgainstContext(context.Background(), root)
}

func (c CapturedSource) VerifyAgainstContext(ctx context.Context, root string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	groups := map[string][]CapturedFile{}
	for _, f := range c.Files {
		groups[f.OriginID] = append(groups[f.OriginID], f)
	}
	verify := func(originID, sourceRoot string, external bool) error {
		var paths []string
		var err error
		if external {
			paths, err = captureExternalPathsContext(ctx, sourceRoot, len(c.Files))
		} else {
			paths, err = captureModePathsContext(ctx, sourceRoot, c.Identity.SourceMode, len(c.Files))
		}
		if err != nil {
			return fmt.Errorf("source file set changed after capture: %w", err)
		}
		group := groups[originID]
		if len(paths) != len(group) {
			return fmt.Errorf("source file set changed after capture for %q", originID)
		}
		opened, err := os.OpenRoot(sourceRoot)
		if err != nil {
			return err
		}
		defer opened.Close()
		for i, f := range group {
			if paths[i] != f.Path {
				return fmt.Errorf("source file set changed after capture for %q", originID)
			}
			buf, executable, err := readCapturedRegularWithModeContext(ctx, opened, f.Path, f.Size, nil)
			if err != nil {
				return fmt.Errorf("source changed after capture %q:%q: %w", originID, f.Path, err)
			}
			sum := sha256.Sum256(buf)
			if int64(len(buf)) != f.Size || hex.EncodeToString(sum[:]) != f.SHA256 {
				return fmt.Errorf("source changed after capture %q:%q", originID, f.Path)
			}
			if c.Identity.FileModePolicy == fileModePolicyV1 && executable != f.Executable {
				return fmt.Errorf("source mode changed after capture %q:%q", originID, f.Path)
			}
		}
		return nil
	}
	if err := verify("repo", root, false); err != nil {
		return err
	}
	for id, sourceRoot := range c.OriginRoots {
		if err := verify(id, sourceRoot, true); err != nil {
			return err
		}
		delete(groups, id)
	}
	delete(groups, "repo")
	if len(groups) != 0 {
		return fmt.Errorf("retained source contains unregistered origin")
	}
	return nil
}

func (c CapturedSource) ReadBlob(digest string) ([]byte, error) {
	return c.ReadBlobContext(context.Background(), digest)
}

func (c CapturedSource) ReadBlobContext(ctx context.Context, digest string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	file, err := openCapturedNoFollow(c.BlobDir, digest)
	if err != nil {
		return nil, fmt.Errorf("snapshot_mismatch: retained source blob changed while opening: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, fmt.Errorf("snapshot_mismatch: retained source blob changed while opening")
	}
	buf, err := io.ReadAll(contextReader{ctx: ctx, reader: file})
	if err != nil {
		return nil, fmt.Errorf("source_missing: %w", err)
	}
	postInfo, err := os.Lstat(path)
	if err != nil || !postInfo.Mode().IsRegular() || !os.SameFile(openedInfo, postInfo) {
		return nil, fmt.Errorf("snapshot_mismatch: retained source blob changed while reading")
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
	return VerifyRetainedSourceContext(context.Background(), versionDir, expected)
}

func VerifyRetainedSourceContext(ctx context.Context, versionDir string, expected SourceIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sources := filepath.Join(versionDir, "sources")
	info, err := os.Lstat(sources)
	if os.IsNotExist(err) {
		if expected.GitArchivePolicy != "" || expected.FileModePolicy != "" {
			return fmt.Errorf("source_missing: pinned source archive is required")
		}
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
	pathsByOrigin := make(map[string][]string)
	prevOrigin, prevPath := "", ""
	for _, f := range captured.Files {
		if (f.OriginID != "repo" && !validCaptureOriginID(f.OriginID)) || f.Kind != "regular" || f.Path == "" || filepath.IsAbs(f.Path) || f.Path == ".." ||
			strings.HasPrefix(f.Path, "../") || strings.ContainsRune(f.Path, '\\') ||
			!utf8.ValidString(f.Path) || f.OriginID < prevOrigin ||
			(f.OriginID == prevOrigin && f.Path <= prevPath) || f.Size < 0 {
			return fmt.Errorf("snapshot_mismatch: invalid retained source path")
		}
		prevOrigin, prevPath = f.OriginID, f.Path
		pathsByOrigin[f.OriginID] = append(pathsByOrigin[f.OriginID], f.Path)
		entries = append(entries, sourceFile{OriginID: f.OriginID, Path: f.Path,
			Kind: f.Kind, Size: f.Size, SHA256: f.SHA256, Executable: f.Executable})
	}
	for _, paths := range pathsByOrigin {
		if err := validateCapturedPaths(paths); err != nil {
			return fmt.Errorf("snapshot_mismatch: %w", err)
		}
	}
	if expected.FileModePolicy != "" && expected.FileModePolicy != fileModePolicyV1 {
		return fmt.Errorf("snapshot_mismatch: unsupported retained file mode policy")
	}
	digest := fileManifestDigestForPolicy(entries, expected.FileModePolicy)
	if digest != expected.FileManifestDigest || sourceSnapshotID(expected) != expected.SnapshotID ||
		expected.CapturePolicyDigest != capturePolicyDigest(expected.SourceMode) {
		return fmt.Errorf("snapshot_mismatch: retained file inventory differs from candidate")
	}
	blobs := filepath.Join(sources, "blobs")
	info, err = os.Lstat(blobs)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source_missing: retained blobs directory: %v", err)
	}
	captured.BlobDir = blobs
	return captured.VerifyBlobsContext(ctx)
}

// VerifyBlobs rechecks retained bytes before a candidate is promoted. A
// manifest alone is insufficient when a blob was deleted or replaced during
// a long graph/vector build.
func (c CapturedSource) VerifyBlobs() error { return c.VerifyBlobsContext(context.Background()) }

func (c CapturedSource) VerifyBlobsContext(ctx context.Context) error {
	if err := c.verifyGitHistoryContext(ctx); err != nil {
		return err
	}
	for _, f := range c.Files {
		buf, err := c.ReadBlobContext(ctx, f.SHA256)
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

// SensitiveCapturePath is the shared admission rule for source capture and
// readiness reporting. It checks file names and secret-bearing directories,
// never contents, so callers can report a blocked path without reading it.
func SensitiveCapturePath(rel string) bool {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(rel)), "/")
	for _, part := range parts[:len(parts)-1] {
		if part == ".aws" || part == "secrets" {
			return true
		}
	}
	base := parts[len(parts)-1]
	if base == ".env" || strings.HasPrefix(base, ".env.") || base == "credentials.json" ||
		base == ".npmrc" || base == ".netrc" || strings.HasPrefix(base, "id_rsa") ||
		strings.HasPrefix(base, "id_ed25519") {
		return true
	}
	switch filepath.Ext(base) {
	case ".pem", ".key", ".p12", ".pfx", ".keystore":
		return true
	}
	return false
}

func sensitiveCapturePath(rel string) bool { return SensitiveCapturePath(rel) }

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
