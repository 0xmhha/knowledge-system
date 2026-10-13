package sourcepath

import "context"

// Reader resolves a logical repository or namespaced corpus path to verified
// source bytes. A non-nil reader is authoritative: callers must not fall back
// to a mutable checkout when it rejects a path.
type Reader func(context.Context, string) ([]byte, error)
