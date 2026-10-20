package maintenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

// SourceCheck is trusted operator registration, never a public tool argument.
// A manager using an immutable filtered source can recheck the live originals
// before admission/candidate completion and under the publication pointer lock.
// A successful exit is a mechanical freshness check, not semantic approval.
type SourceCheck struct {
	Command        []string          `yaml:"command" json:"command"`
	PinnedFiles    map[string]string `yaml:"pinned_files" json:"pinned_files"`
	TimeoutSeconds int               `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type checkReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r checkReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

func (s SourceCheck) Validate() error {
	if len(s.Command) == 0 || len(s.Command) > 32 || len(s.PinnedFiles) == 0 || len(s.PinnedFiles) > 64 ||
		s.TimeoutSeconds < 1 || s.TimeoutSeconds > 60 || !filepath.IsAbs(s.Command[0]) || filepath.Clean(s.Command[0]) != s.Command[0] {
		return reject("profile_unknown")
	}
	if _, ok := s.PinnedFiles[s.Command[0]]; !ok {
		return reject("profile_unknown")
	}
	for _, arg := range s.Command {
		if len(arg) == 0 || len(arg) > 4096 || strings.ContainsAny(arg, "\x00\r\n") {
			return reject("profile_unknown")
		}
	}
	for name, value := range s.PinnedFiles {
		if !filepath.IsAbs(name) || filepath.Clean(name) != name || !hashPattern.MatchString(value) {
			return reject("profile_unknown")
		}
	}
	return nil
}

// Bound both memory and total reads. Paths must still name the same canonical,
// regular, nonlinked file before/after hashing; pin failures never execute it.
func (s SourceCheck) verifyFiles(ctx context.Context) error {
	var total int64
	for path, expected := range s.PinnedFiles {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 128<<20 {
			return reject("profile_stale")
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		resolved, err := filepath.EvalSymlinks(path)
		if !ok || stat.Nlink != 1 || err != nil || resolved != path {
			return reject("profile_stale")
		}
		total += info.Size()
		if total > 256<<20 {
			return reject("profile_stale")
		}
		file, err := os.Open(path)
		if err != nil {
			return reject("profile_stale")
		}
		h := sha256.New()
		count, readErr := io.Copy(h, checkReader{ctx: ctx, reader: io.LimitReader(file, (128<<20)+1)})
		after, statErr := file.Stat()
		closeErr := file.Close()
		if readErr != nil || statErr != nil || closeErr != nil || count != info.Size() || !os.SameFile(info, after) ||
			after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) || hex.EncodeToString(h.Sum(nil)) != expected {
			return reject("profile_stale")
		}
	}
	return ctx.Err()
}

func runSourceCheck(ctx context.Context, check *SourceCheck, guardian string) error {
	if check == nil {
		return nil
	}
	if check.Validate() != nil {
		return reject("profile_stale")
	}
	bounded, cancel := context.WithTimeout(ctx, time.Duration(check.TimeoutSeconds)*time.Second)
	defer cancel()
	if err := check.verifyFiles(bounded); err != nil {
		return err
	}
	err := setup.RunGuardedCommand(bounded, guardian, check.Command, []string{"PYTHONDONTWRITEBYTECODE=1"})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if bounded.Err() != nil {
		return reject("deadline_exceeded")
	}
	if err != nil {
		return reject("source_stale")
	}
	return check.verifyFiles(bounded)
}
