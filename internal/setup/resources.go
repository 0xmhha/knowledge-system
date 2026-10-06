package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

var ErrResourceLimit = errors.New("resource_limit")
var ErrBuildTimeout = errors.New("build_timeout")

const DefaultBuildTimeout = 2 * time.Hour

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buf []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buf)
}

type CaptureLimits struct {
	MaxFiles      int   `json:"max_files"`
	MaxFileBytes  int64 `json:"max_file_bytes"`
	MaxTotalBytes int64 `json:"max_total_bytes"`
}

func (l CaptureLimits) Normalized() (CaptureLimits, error) {
	if l.MaxFiles < 0 || l.MaxFileBytes < 0 || l.MaxTotalBytes < 0 || l.MaxFileBytes == math.MaxInt64 || l.MaxTotalBytes > math.MaxInt64/4 {
		return l, fmt.Errorf("invalid capture resource limits")
	}
	if l.MaxFiles == 0 {
		l.MaxFiles = defaultCaptureMaxFiles
	}
	if l.MaxFileBytes == 0 {
		l.MaxFileBytes = defaultCaptureMaxFileBytes
	}
	if l.MaxTotalBytes == 0 {
		l.MaxTotalBytes = defaultCaptureMaxTotalBytes
	}
	return l, nil
}

func (o Options) CaptureLimits() CaptureLimits {
	return CaptureLimits{MaxFiles: o.MaxCaptureFiles, MaxFileBytes: o.MaxCaptureFileBytes, MaxTotalBytes: o.MaxCaptureTotalBytes}
}

func (o Options) WithBuildDeadline(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if _, err := o.CaptureLimits().Normalized(); err != nil {
		return nil, nil, err
	}
	if o.BuildTimeout < 0 || o.MinFreeBytes < 0 || o.MaxGitHistoryBytes < 0 || o.MaxGitHistoryBytes == math.MaxInt64 {
		return nil, nil, fmt.Errorf("invalid build deadline or free-space reserve")
	}
	deadline := o.BuildTimeout
	if deadline == 0 {
		deadline = DefaultBuildTimeout
	}
	bounded, cancel := context.WithTimeout(ctx, deadline)
	return bounded, cancel, nil
}

func BuildContextError(ctx context.Context, err error) error {
	if errors.Is(err, ErrDurabilityUncertain) {
		return err
	}
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrBuildTimeout, err)
	}
	return err
}

type ResourceStatus struct {
	CaptureLimits          CaptureLimits `json:"capture_limits"`
	SourceFiles            int           `json:"source_files"`
	SourceBytes            int64         `json:"source_bytes"`
	GitHistoryBytes        int64         `json:"git_history_bytes"`
	MinimumAdditionalBytes int64         `json:"minimum_additional_bytes"`
	FreeBytes              uint64        `json:"output_free_bytes"`
	StagingFreeBytes       uint64        `json:"staging_free_bytes"`
	MinimumOutputBytes     int64         `json:"minimum_output_bytes"`
	MinimumStagingBytes    int64         `json:"minimum_staging_bytes"`
	ReserveBytes           int64         `json:"reserve_bytes"`
	DeadlineUTC            *time.Time    `json:"deadline_utc,omitempty"`
	ActualCandidateBytes   int64         `json:"actual_candidate_bytes"`
	EngineOutputEstimate   *int64        `json:"engine_output_estimate_bytes"`
	EstimateScope          string        `json:"estimate_scope"`
}

// Source copy/staging is a minimum, not an estimate of engine output or filesystem
// block usage. Engine output stays null until measured; free space is a live probe.
func candidateResources(ctx context.Context, out string, captured CapturedSource, o Options) (ResourceStatus, error) {
	limits, err := o.CaptureLimits().Normalized()
	if err != nil {
		return ResourceStatus{}, err
	}
	if o.MinFreeBytes < 0 {
		return ResourceStatus{}, fmt.Errorf("invalid free-space reserve")
	}
	status := ResourceStatus{CaptureLimits: limits, SourceFiles: len(captured.Files), ReserveBytes: o.MinFreeBytes, EstimateScope: "remaining staging copy; engine output and filesystem overhead not predicted"}
	for _, file := range captured.Files {
		status.SourceBytes += file.Size
	}
	if captured.GitHistory != nil {
		status.GitHistoryBytes = captured.GitHistory.BundleBytes
	}
	if status.GitHistoryBytes > math.MaxInt64-status.SourceBytes || o.MinFreeBytes > math.MaxInt64-status.SourceBytes-status.GitHistoryBytes {
		return status, fmt.Errorf("invalid resource estimate overflow")
	}
	status.MinimumStagingBytes = status.SourceBytes + status.GitHistoryBytes
	status.MinimumOutputBytes = o.MinFreeBytes
	status.MinimumAdditionalBytes = status.MinimumStagingBytes + status.MinimumOutputBytes
	if deadline, ok := ctx.Deadline(); ok {
		utc := deadline.UTC()
		status.DeadlineUTC = &utc
	}
	status.FreeBytes, err = filesystemFreeBytes(out)
	if err != nil {
		return status, err
	}
	status.StagingFreeBytes, err = filesystemFreeBytes(os.TempDir())
	if err != nil {
		return status, err
	}
	if err := ctx.Err(); err != nil {
		return status, err
	}
	if same, err := sameResourceFilesystem(out, os.TempDir()); err != nil {
		return status, err
	} else if same && uint64(status.MinimumAdditionalBytes) > status.FreeBytes {
		return status, fmt.Errorf("%w: insufficient combined staging and reserve space", ErrResourceLimit)
	}
	if uint64(status.MinimumOutputBytes) > status.FreeBytes || uint64(status.MinimumStagingBytes) > status.StagingFreeBytes {
		return status, fmt.Errorf("%w: insufficient output reserve or staging space", ErrResourceLimit)
	}
	return status, nil
}

func filesystemFreeBytes(path string) (uint64, error) {
	for {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(path)
		if parent == path {
			return 0, fmt.Errorf("filesystem unavailable")
		}
		path = parent
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	if stat.Bsize <= 0 {
		return 0, fmt.Errorf("invalid filesystem block size")
	}
	if uint64(stat.Bavail) > math.MaxUint64/uint64(stat.Bsize) {
		return math.MaxUint64, nil
	} else {
		return uint64(stat.Bavail) * uint64(stat.Bsize), nil
	}
}

// checkOutputSpace reserves only a known upcoming write; engine output remains unknown.
func checkOutputSpace(ctx context.Context, out string, additional, reserve int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if additional < 0 || reserve < 0 || additional > math.MaxInt64-reserve {
		return fmt.Errorf("invalid space requirement")
	}
	free, err := filesystemFreeBytes(out)
	if err != nil {
		return err
	}
	if uint64(additional+reserve) > free {
		return fmt.Errorf("%w: insufficient output space", ErrResourceLimit)
	}
	return nil
}

func existingResourceDirectory(path string) string {
	for {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}
func sameResourceFilesystem(a, b string) (bool, error) {
	var first, second unix.Stat_t
	if err := unix.Stat(existingResourceDirectory(a), &first); err != nil {
		return false, err
	}
	if err := unix.Stat(existingResourceDirectory(b), &second); err != nil {
		return false, err
	}
	return first.Dev == second.Dev, nil
}
func candidateLogicalBytes(ctx context.Context, root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("candidate artifact must be regular")
		}
		if info.Size() > math.MaxInt64-total {
			return fmt.Errorf("invalid candidate size overflow")
		}
		total += info.Size()
		return nil
	})
	return total, err
}
