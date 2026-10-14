package store

import (
	"errors"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// BoundedEdgeReader is optional for legacy/custom Readers. Callers must return
// unavailable when absent; an unbounded read plus slicing is not equivalent.
type BoundedEdgeReader = persist.BoundedEdgeReader
type EdgePageInput = persist.EdgePageInput

var ErrBoundedEdgesUnsupported = errors.New("bounded edge queries unavailable on this graph reader")
