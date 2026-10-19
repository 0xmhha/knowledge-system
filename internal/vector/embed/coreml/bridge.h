#ifndef CKV_COREML_BRIDGE_H
#define CKV_COREML_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

typedef void* ckv_coreml_handle;

// Each successful load returns an independently owned model handle. A package
// is compiled into an owned temporary model; unload releases it after the model.
int ckv_coreml_load(const char* model_path, ckv_coreml_handle* handle);
int ckv_coreml_unload(ckv_coreml_handle handle);

// Inputs are int64 Go buffers, converted value-by-value to CoreML Int32.
// The supported output is [batch, sequence, hidden], converted by logical
// indices (including non-contiguous strides) into the supplied float buffer.
int ckv_coreml_predict(
    ckv_coreml_handle handle,
    const int64_t* input_ids,
    const int64_t* attention_mask,
    int batch_size,
    int seq_len,
    int hidden_dim,
    float* output,
    size_t output_capacity
);
#endif
