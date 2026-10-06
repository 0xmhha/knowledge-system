package setup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/githistory"
)

const defaultMaxGitHistoryBytes int64 = 1 << 30

// CapturedGitHistory describes a full Git bundle of the base commit and the
// exact recovery commits selected for the graph. The archive contains only
// refs/cks/base and refs/cks/recovery/<SHA> refs. Its bytes are retained with
// source blobs so a later build does not depend on the original object store.
type CapturedGitHistory struct {
	BundleSHA256    string   `json:"bundle_sha256"`
	BundleBytes     int64    `json:"bundle_bytes"`
	RecoveryCommits []string `json:"recovery_commits"`
}

func runArchiveGit(args ...string) error { return runArchiveGitContext(context.Background(), args...) }

func runArchiveGitContext(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return nil
}

func captureGitHistory(source, bundle, base string, selected []string, maxBytes int64) (*CapturedGitHistory, error) {
	return captureGitHistoryContext(context.Background(), source, bundle, base, selected, maxBytes)
}

func captureGitHistoryContext(ctx context.Context, source, bundle, base string, selected []string, maxBytes int64) (_ *CapturedGitHistory, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if maxBytes == 0 {
		maxBytes = defaultMaxGitHistoryBytes
	}
	if maxBytes < 0 || maxBytes == int64(^uint64(0)>>1) {
		return nil, fmt.Errorf("invalid Git history byte limit")
	}
	if len(base) != 40 || !slices.IsSorted(selected) || slices.Contains(selected, base) {
		return nil, fmt.Errorf("invalid Git history commit selection")
	}
	stage, err := os.MkdirTemp(filepath.Dir(bundle), ".git-history-stage-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	seed := filepath.Join(stage, "seed.git")
	if err := runArchiveGitContext(ctx, "init", "--quiet", "--bare", seed); err != nil {
		return nil, err
	}
	// The temporary bare repo reads source objects through an alternate; it
	// does not copy a potentially huge source pack before the byte limit can
	// apply. Only the resulting bounded bundle is retained, and the restored
	// build repository has no alternate.
	objectPath, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "--git-path", "objects").Output()
	if err != nil {
		return nil, fmt.Errorf("locate source Git objects: %w", err)
	}
	objects := strings.TrimSpace(string(objectPath))
	if strings.ContainsAny(objects, "\r\n") {
		return nil, fmt.Errorf("invalid source Git object path")
	}
	if !filepath.IsAbs(objects) {
		objects = filepath.Join(source, objects)
	}
	objects, err = filepath.Abs(objects)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(objects); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("source Git objects unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seed, "objects", "info", "alternates"), []byte(objects+"\n"), 0o600); err != nil {
		return nil, err
	}
	if err := runArchiveGitContext(ctx, "-C", seed, "update-ref", "refs/cks/base", base); err != nil {
		return nil, err
	}
	for _, sha := range selected {
		if err := runArchiveGitContext(ctx, "-C", seed, "update-ref", "refs/cks/recovery/"+sha, sha); err != nil {
			return nil, err
		}
	}
	file, err := os.OpenFile(bundle, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
		if err != nil {
			_ = os.Remove(bundle)
		}
	}()
	cmd := exec.CommandContext(ctx, "git", "-C", seed, "bundle", "create", "-", "--all")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	hash := sha256.New()
	written, copyErr := io.CopyN(io.MultiWriter(file, hash), contextReader{ctx: ctx, reader: stdout}, maxBytes+1)
	if written > maxBytes {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("Git history archive exceeds %d bytes", maxBytes)
	}
	if copyErr != nil && copyErr != io.EOF {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("write Git history archive: %w", copyErr)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("create Git history archive: %w: %s", err, stderr.String())
	}
	if written == 0 {
		return nil, fmt.Errorf("Git history archive is empty")
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if err := file.Chmod(0o400); err != nil {
		return nil, err
	}
	return &CapturedGitHistory{BundleSHA256: hex.EncodeToString(hash.Sum(nil)),
		BundleBytes: written, RecoveryCommits: slices.Clone(selected)}, nil
}

func (c CapturedSource) gitBundlePath() string {
	return filepath.Join(filepath.Dir(c.BlobDir), "history.bundle")
}

func (c CapturedSource) verifyGitHistory() error {
	return c.verifyGitHistoryContext(context.Background())
}

func (c CapturedSource) verifyGitHistoryContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.Identity.GitArchivePolicy == "" {
		return nil // v2/v3 or snapshot-only retained source
	}
	if c.Identity.GitArchivePolicy != gitArchivePolicyV1 || c.GitHistory == nil ||
		c.Identity.SourceMode == "snapshot-only" || c.Identity.GitRecoveryDigest == "" ||
		c.GitHistory.BundleBytes <= 0 || len(c.GitHistory.BundleSHA256) != 64 ||
		!slices.IsSorted(c.GitHistory.RecoveryCommits) ||
		identityHashFields("cks.git-recovery.v1", c.GitHistory.RecoveryCommits...) != c.Identity.GitRecoveryDigest {
		return fmt.Errorf("snapshot_mismatch: invalid retained Git history metadata")
	}
	for i, sha := range c.GitHistory.RecoveryCommits {
		if len(sha) != 40 || (i > 0 && sha == c.GitHistory.RecoveryCommits[i-1]) {
			return fmt.Errorf("snapshot_mismatch: invalid retained recovery commit")
		}
	}
	path := c.gitBundlePath()
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("source_missing: retained Git history archive: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != c.GitHistory.BundleBytes {
		return fmt.Errorf("snapshot_mismatch: retained Git history archive changed size or type")
	}
	file, err := openCapturedNoFollow(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return fmt.Errorf("snapshot_mismatch: open retained Git history archive: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return fmt.Errorf("snapshot_mismatch: retained Git history archive changed while opening")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, contextReader{ctx: ctx, reader: file})
	if err != nil {
		return err
	}
	post, err := os.Lstat(path)
	if err != nil || !os.SameFile(opened, post) || n != c.GitHistory.BundleBytes ||
		hex.EncodeToString(hash.Sum(nil)) != c.GitHistory.BundleSHA256 {
		return fmt.Errorf("snapshot_mismatch: retained Git history archive changed")
	}
	return nil
}

func (c CapturedSource) materializeGitHistory(path string) error {
	return c.materializeGitHistoryContext(context.Background(), path)
}

func (c CapturedSource) materializeGitHistoryContext(ctx context.Context, path string) error {
	if err := c.verifyGitHistoryContext(ctx); err != nil {
		return err
	}
	if err := runArchiveGitContext(ctx, "init", "--quiet", path); err != nil {
		return err
	}
	if err := runArchiveGitContext(ctx, "-C", path, "config", "core.logAllRefUpdates", "always"); err != nil {
		return err
	}
	if err := runArchiveGitContext(ctx, "-c", "protocol.file.allow=always", "-C", path,
		"fetch", "--no-tags", "--no-write-fetch-head", c.gitBundlePath(),
		"refs/cks/*:refs/cks/*"); err != nil {
		return fmt.Errorf("restore Git history archive: %w", err)
	}
	if err := runArchiveGitContext(ctx, "-C", path, "checkout", "--quiet", "--detach", c.Identity.SourceCommit); err != nil {
		return err
	}
	selected, err := githistory.RecoveryCommitIDsContext(ctx, path, 0)
	if err != nil || !slices.Equal(selected, c.GitHistory.RecoveryCommits) {
		return fmt.Errorf("restored Git recovery input differs from capture: %v", err)
	}
	return nil
}
