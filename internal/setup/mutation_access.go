package setup

import (
	"context"
	"errors"
)

// ErrDatasetWriterBusy is a retriable ownership conflict, not a gate failure.
var ErrDatasetWriterBusy = errors.New("dataset writer is busy")

// WithDatasetMutationLock lets trusted dataset services initialize their
// journals with exactly the same exclusion used by reindex/promotion/GC.
// The action must not recursively acquire the dataset mutation lock.
func WithDatasetMutationLock(ctx context.Context, dataset string, action func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := CheckMutableTarget(dataset); err != nil {
		return err
	}
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		return err
	}
	defer lock.release()
	if err := ctx.Err(); err != nil {
		return err
	}
	return action(ctx)
}
