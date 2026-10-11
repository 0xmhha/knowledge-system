package setup

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
)

// RequestVerification shares one full archive verification among the reads
// of a single request. It never outlives that request: Finish verifies the
// whole archive again before the caller may publish anything, and after
// Finish or Close every read verifies the archive on its own. Each retained
// blob is still re-read and hashed when it is used.
type RequestVerification struct {
	versionDir string
	identity   DatasetIdentity
	done       atomic.Bool
}

type requestVerificationKey struct{}

// BeginRequestVerification verifies versionDir once and returns a context
// whose retained reads of the same version reuse that result. A legacy
// version without an identity returns a nil verification and the unchanged
// context.
func BeginRequestVerification(ctx context.Context, versionDir string) (context.Context, *RequestVerification, error) {
	identity, err := verifyVersionIdentityContext(ctx, versionDir)
	if err != nil || identity == nil {
		return ctx, nil, err
	}
	dir, err := filepath.Abs(versionDir)
	if err != nil {
		return ctx, nil, err
	}
	v := &RequestVerification{versionDir: dir, identity: *identity}
	return context.WithValue(ctx, requestVerificationKey{}, v), v, nil
}

// Identity returns a copy of the identity verified at the start of the request.
func (v *RequestVerification) Identity() *DatasetIdentity {
	identity := v.identity
	return &identity
}

// Finish ends reuse and verifies the whole archive again. Results built during
// the request must be discarded when it fails.
func (v *RequestVerification) Finish(ctx context.Context) error {
	v.Close()
	after, err := verifyVersionIdentityContext(ctx, v.versionDir)
	if err != nil {
		return err
	}
	if after == nil || after.DatasetID != v.identity.DatasetID {
		return fmt.Errorf("snapshot_mismatch: retained dataset changed during request")
	}
	return nil
}

// Close ends reuse without verifying again. It is safe on a nil verification.
func (v *RequestVerification) Close() {
	if v != nil {
		v.done.Store(true)
	}
}

func requestVerifiedIdentity(ctx context.Context, versionDir string) *DatasetIdentity {
	v, ok := ctx.Value(requestVerificationKey{}).(*RequestVerification)
	if !ok || v.done.Load() {
		return nil
	}
	dir, err := filepath.Abs(versionDir)
	if err != nil || dir != v.versionDir {
		return nil
	}
	return v.Identity()
}
