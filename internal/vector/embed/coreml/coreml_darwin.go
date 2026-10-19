//go:build darwin && cgo && tokenizers

package coreml

/*
#cgo LDFLAGS: -framework CoreML -framework Foundation
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

// Adapter owns one CoreML model and tokenizer. This profile consumes Int32
// input_ids/attention_mask and [batch,sequence,hidden] output with CLS pooling.
// It does not infer compatibility from a GGUF model name or vector dimension.
type Adapter struct {
	mu        sync.Mutex
	handle    C.ckv_coreml_handle
	closed    bool
	modelPath string
	modelName string
	dim       int
	maxSeqLen int
	tokenizer adapterTokenizer
	identity  types.EmbeddingIdentity
}

type adapterTokenizer interface {
	Encode(text string, maxLen int) (ids, mask []int64, err error)
	Close() error
}

func Available() bool { return true }

func Open(opts Options) (*Adapter, error) {
	if opts.ModelPath == "" {
		return nil, fmt.Errorf("coreml: model path is required")
	}
	if opts.Dim <= 0 {
		return nil, fmt.Errorf("coreml: dimension must be > 0")
	}
	if opts.ModelName == "" || opts.TokenizerPath == "" {
		return nil, fmt.Errorf("coreml: model name and tokenizer path are required")
	}
	if opts.MaxSeqLen < 0 {
		return nil, fmt.Errorf("coreml: max sequence length must be positive")
	}
	if opts.MaxSeqLen == 0 {
		opts.MaxSeqLen = 512
	}
	digest, err := modelContentDigest(opts.ModelPath, opts.TokenizerPath)
	if err != nil {
		return nil, err
	}
	cPath := C.CString(opts.ModelPath)
	defer C.free(unsafe.Pointer(cPath))
	var handle C.ckv_coreml_handle
	if rc := C.ckv_coreml_load(cPath, &handle); rc != 0 {
		return nil, fmt.Errorf("coreml: load failed (code %d)", rc)
	}
	a := &Adapter{handle: handle, modelPath: opts.ModelPath, modelName: opts.ModelName, dim: opts.Dim, maxSeqLen: opts.MaxSeqLen}
	tk, err := newTokenizer(opts.TokenizerPath)
	if err != nil {
		return nil, errors.Join(err, a.Close())
	}
	a.tokenizer = tk
	after, err := modelContentDigest(opts.ModelPath, opts.TokenizerPath)
	if err != nil || after != digest {
		return nil, errors.Join(fmt.Errorf("coreml: model or tokenizer changed during open"), err, a.Close())
	}
	a.identity = types.EmbeddingIdentity{Provider: "coreml", Model: opts.ModelName, Dim: opts.Dim, Pooling: "cls", Normalize: "l2", Version: 2, ModelDigest: digest, NativeDim: opts.Dim, DimensionMethod: "native", PassageTransform: "raw", QueryTransform: "raw", TruncatePolicy: "tokenizer_head_max_len_v1", RuntimeContextTokens: opts.MaxSeqLen}
	return a, nil
}

func (a *Adapter) Name() string                      { return a.modelName }
func (a *Adapter) Dimension() int                    { return a.dim }
func (a *Adapter) MaxInputTokens() int               { return a.maxSeqLen }
func (a *Adapter) Identity() types.EmbeddingIdentity { return a.identity }

func (a *Adapter) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	var err error
	if a.tokenizer != nil {
		err = a.tokenizer.Close()
		a.tokenizer = nil
	}
	handle := a.handle
	a.handle = nil
	if rc := C.ckv_coreml_unload(handle); rc != 0 {
		err = errors.Join(err, fmt.Errorf("coreml: native cleanup failed (code %d)", rc))
	}
	return err
}

func checkedTensorSize(batch, sequence, hidden int) (int, error) {
	maxInt := int(^uint(0) >> 1)
	if batch <= 0 || sequence <= 0 || hidden <= 0 || batch > maxInt/sequence || batch*sequence > maxInt/8 || batch*sequence > maxInt/hidden || batch*sequence*hidden > maxInt/4 {
		return 0, fmt.Errorf("coreml: invalid tensor size")
	}
	return batch * sequence * hidden, nil
}

// Embed cooperatively checks cancellation around tokenization and synchronous
// native inference; it cannot interrupt a prediction already inside CoreML.
func (a *Adapter) Embed(ctx context.Context, batch []string) ([][]float32, error) {
	if ctx == nil {
		return nil, fmt.Errorf("coreml: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(batch) == 0 {
		return nil, nil
	}
	if a == nil {
		return nil, fmt.Errorf("coreml: adapter is unavailable")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a.closed || a.handle == nil {
		return nil, fmt.Errorf("coreml: model is not loaded")
	}
	if a.tokenizer == nil {
		return nil, fmt.Errorf("coreml: tokenizer is not configured")
	}
	seqLen := a.maxSeqLen
	outputSize, err := checkedTensorSize(1, seqLen, a.dim)
	if err != nil {
		return nil, err
	}
	if seqLen > math.MaxInt32 || a.dim > math.MaxInt32 {
		return nil, fmt.Errorf("coreml: native shape exceeds Int32")
	}
	// A batch-one export can serve arbitrary caller batches without assuming
	// that the model has a flexible batch axis or allocating all hidden states.
	result := make([][]float32, len(batch))
	output := make([]float32, outputSize)
	for i, text := range batch {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ids, mask, err := a.tokenizer.Encode(text, seqLen)
		if err != nil {
			return nil, fmt.Errorf("coreml: tokenize[%d]: %w", i, err)
		}
		if len(ids) != seqLen || len(mask) != seqLen {
			return nil, fmt.Errorf("coreml: tokenizer returned inconsistent dimensions")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rc := C.ckv_coreml_predict(a.handle, (*C.int64_t)(unsafe.Pointer(&ids[0])), (*C.int64_t)(unsafe.Pointer(&mask[0])), 1, C.int(seqLen), C.int(a.dim), (*C.float)(unsafe.Pointer(&output[0])), C.size_t(len(output)))
		if rc != 0 {
			return nil, fmt.Errorf("coreml: predict failed (code %d)", rc)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		vec := make([]float32, a.dim)
		copy(vec, output[:a.dim])
		l2Normalize(vec)
		result[i] = vec
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func l2Normalize(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	norm := math.Sqrt(sum)
	if norm == 0 {
		return
	}
	for i, v := range vec {
		vec[i] = float32(float64(v) / norm)
	}
}
