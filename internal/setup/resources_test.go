package setup

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func resourceSource(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, data := range map[string]string{"README.md": "alpha\n", "second.md": "beta\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestResourceLimitsIdentityCaptureAndExternalOrigins(t *testing.T) {
	root := resourceSource(t)
	external := t.TempDir()
	os.WriteFile(filepath.Join(external, "third.md"), []byte("third\n"), 0600)
	origin := CaptureOrigin{ID: "knowledge:external", Root: external}
	for _, tc := range []struct {
		name    string
		l       CaptureLimits
		origins []CaptureOrigin
	}{
		{"count", CaptureLimits{MaxFiles: 1}, nil},
		{"file", CaptureLimits{MaxFileBytes: 3}, nil},
		{"total", CaptureLimits{MaxTotalBytes: 7}, nil},
		{"external-count", CaptureLimits{MaxFiles: 2}, []CaptureOrigin{origin}},
		{"external-total", CaptureLimits{MaxTotalBytes: 12}, []CaptureOrigin{origin}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SnapshotSourceIdentityWithLimits(context.Background(), root, "resource", "snapshot-only", "", tc.l, tc.origins...)
			if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("identity accepted over-limit source: %v", err)
			}
			_, err = CaptureSourceContext(context.Background(), CaptureOptions{Root: root, Out: filepath.Join(t.TempDir(), "candidate"), ProjectID: "resource", SourceMode: "snapshot-only", MaxFiles: tc.l.MaxFiles, MaxFileBytes: tc.l.MaxFileBytes, MaxTotalBytes: tc.l.MaxTotalBytes, ExternalOrigins: tc.origins})
			if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("capture accepted over-limit source: %v", err)
			}
		})
	}
	old, err := SnapshotSourceIdentity(root, "resource", "snapshot-only", "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := SnapshotSourceIdentityWithLimits(context.Background(), root, "resource", "snapshot-only", "", CaptureLimits{MaxFiles: 2, MaxFileBytes: 6, MaxTotalBytes: 11})
	if err != nil || got != old {
		t.Fatalf("limits changed source identity: %v", err)
	}
}

func TestResourceInputsDeadlineAndCapacityDTO(t *testing.T) {
	for _, l := range []CaptureLimits{{MaxFiles: -1}, {MaxFileBytes: -1}, {MaxTotalBytes: -1}, {MaxFileBytes: math.MaxInt64}, {MaxTotalBytes: math.MaxInt64}} {
		if _, err := l.Normalized(); err == nil {
			t.Fatalf("accepted invalid limits: %+v", l)
		}
	}
	ctx, cancel, err := (Options{}).WithBuildDeadline(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	deadline, _ := ctx.Deadline()
	if remaining := time.Until(deadline); remaining < DefaultBuildTimeout-time.Minute || remaining > DefaultBuildTimeout {
		t.Fatal(remaining)
	}
	parent, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	bounded, stopBound, err := (Options{BuildTimeout: time.Hour}).WithBuildDeadline(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer stopBound()
	pd, _ := parent.Deadline()
	bd, _ := bounded.Deadline()
	if !pd.Equal(bd) {
		t.Fatal("reset parent deadline")
	}
	captured := CapturedSource{Files: []CapturedFile{{Size: 6}, {Size: 5}}}
	status, err := candidateResources(ctx, t.TempDir(), captured, Options{})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(status)
	if status.SourceFiles != 2 || status.SourceBytes != 11 || status.MinimumStagingBytes != 11 || status.FreeBytes == 0 || status.StagingFreeBytes == 0 || !strings.Contains(string(body), `"engine_output_estimate_bytes":null`) || strings.Contains(string(body), "/Users/") {
		t.Fatalf("invalid public resources: %s", body)
	}
	_, err = candidateResources(ctx, t.TempDir(), captured, Options{MinFreeBytes: math.MaxInt64 - 100})
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("capacity accepted: %v", err)
	}
}

func TestResourceCancellationPreservesCurrentAndFinishesAfterRename(t *testing.T) {
	for _, stage := range []string{"candidate-file", "before-rename", "after-rename"} {
		t.Run(stage, func(t *testing.T) {
			dataset := t.TempDir()
			old := durableTestVersion(t, dataset, "old")
			durableTestVersion(t, dataset, "new")
			if _, err := Promote(dataset, "old"); err != nil {
				t.Fatal(err)
			}
			before := durabilityTree(t, old)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			durabilityFault = func(at, path string) error {
				if at == stage {
					cancel()
				}
				return nil
			}
			defer func() { durabilityFault = nil }()
			_, err := promoteLockedContext(ctx, dataset, "new", false)
			if stage == "after-rename" {
				if err != nil {
					t.Fatal(err)
				}
				assertDurabilityCurrent(t, dataset, "new")
			} else {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel not honored: %v", err)
				}
				assertDurabilityCurrent(t, dataset, "old")
			}
			if !reflect.DeepEqual(before, durabilityTree(t, old)) {
				t.Fatal("old candidate changed")
			}
		})
	}
}

func TestResourceSharedReindexDeadlineAndLimitsBeforeEngine(t *testing.T) {
	root := resourceSource(t)
	dataset := t.TempDir()
	old := durableTestVersion(t, dataset, "old")
	if _, err := Promote(dataset, "old"); err != nil {
		t.Fatal(err)
	}
	before := durabilityTree(t, old)
	identity, err := SnapshotSourceIdentity(root, "resource", "snapshot-only", "")
	if err != nil {
		t.Fatal(err)
	}
	err = Reindex(context.Background(), Options{Src: root, Out: dataset, ProjectID: "resource", MaxCaptureFiles: 1}, "over-limit", GateOptions{ExpectedSourceSnapshot: identity}, gateRunner{}, nil)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("shared API ignored limit: %v", err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	err = Reindex(ctx, Options{Src: root, Out: dataset, ProjectID: "resource"}, "timeout", GateOptions{ExpectedSourceSnapshot: identity}, gateRunner{}, nil)
	if !errors.Is(err, ErrBuildTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("untyped timeout: %v", err)
	}
	assertDurabilityCurrent(t, dataset, "old")
	if !reflect.DeepEqual(before, durabilityTree(t, old)) {
		t.Fatal("current evidence changed")
	}
	if _, err := os.Lstat(filepath.Join(dataset, "timeout")); !os.IsNotExist(err) {
		t.Fatal("timed-out API created candidate")
	}
}

func TestResourceSubprocessDeadlineWithDescendantOutputPipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := (SubprocessRunner{}).Run(ctx, Step{ID: "deadline", Cmd: []string{"/bin/sh", "-c", "sleep 30 & wait"}}, func(Event) {})
	if err == nil || ctx.Err() != context.DeadlineExceeded || time.Since(start) > 3*time.Second {
		t.Fatalf("command exceeded deadline: %v, %s", err, time.Since(start))
	}
}

func TestResourceDeadlineCannotHideDurabilityUncertain(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if got := BuildContextError(ctx, ErrDurabilityUncertain); !errors.Is(got, ErrDurabilityUncertain) || errors.Is(got, ErrBuildTimeout) {
		t.Fatalf("lost post-rename uncertainty: %v", got)
	}
	for _, o := range []Options{{BuildTimeout: -1}, {MinFreeBytes: -1}, {MaxGitHistoryBytes: math.MaxInt64}} {
		if _, _, err := o.WithBuildDeadline(context.Background()); err == nil {
			t.Fatalf("invalid resource option accepted: %+v", o)
		}
	}
}
