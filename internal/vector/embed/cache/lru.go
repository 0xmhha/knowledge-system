// Package cache preserves the internal import path for the shared embedding cache.
package cache

import shared "github.com/0xmhha/knowledge-system/pkg/vector/embed/cache"
import "github.com/0xmhha/knowledge-system/pkg/vector/types"

const DefaultSize = shared.DefaultSize
const MaxSize = shared.MaxSize

type Cached = shared.Cached

func New(emb types.Embedder, size int) *Cached { return shared.New(emb, size) }
func ValidateSize(size int) error              { return shared.ValidateSize(size) }
