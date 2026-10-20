package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTrustedJournalAccessSharesExistingDatasetExclusion(t *testing.T) {
	root := t.TempDir()
	called := false
	err := WithDatasetMutationLock(context.Background(), root, func(ctx context.Context) error {
		called = true
		innerCalled := false
		err := WithDatasetMutationLock(ctx, root, func(context.Context) error {
			innerCalled = true
			return nil
		})
		if !errors.Is(err, ErrDatasetWriterBusy) || innerCalled {
			t.Fatal("journal initialization bypassed the existing writer lock")
		}
		return nil
	})
	if err != nil || !called {
		t.Fatal("trusted initialization failed")
	}
	lock, err := acquireReindexLock(root)
	if err != nil {
		t.Fatal("callback completion did not release the existing lock")
	}
	lock.release()
}

func TestCanceledJournalAdmissionDoesNotCreateDataset(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := WithDatasetMutationLock(ctx, root, func(context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, context.Canceled) || called {
		t.Fatal("canceled admission entered its mutation callback")
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("canceled admission created a dataset")
	}
}
