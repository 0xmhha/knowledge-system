//go:build !darwin || !cgo || !tokenizers

package coreml

import (
	"context"
	"fmt"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

var errNotAvailable = fmt.Errorf("coreml: requires macOS, CGO, and a build with the tokenizers tag")

// Adapter is the stub embedder for builds without CoreML support; every
// method reports that CoreML is unavailable.
type Adapter struct{}

func Available() bool { return false }

// Open returns an error when the native build profile is unavailable.
func Open(opts Options) (*Adapter, error) {
	return nil, errNotAvailable
}

func (a *Adapter) Name() string        { return "" }
func (a *Adapter) Dimension() int      { return 0 }
func (a *Adapter) MaxInputTokens() int { return 0 }
func (a *Adapter) Identity() types.EmbeddingIdentity {
	return types.EmbeddingIdentity{Provider: "coreml"}
}
func (a *Adapter) Close() error { return nil }
func (a *Adapter) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, errNotAvailable
}
