//go:build darwin && cgo && tokenizers

#import <Foundation/Foundation.h>
#import <CoreML/CoreML.h>
#include "bridge.h"
#include <limits.h>
#include <math.h>

// The Go CGO profile uses manual reference counting. The helpers also support
// ARC so the same owned-handle contract can be tested under both compilers.
#if __has_feature(objc_arc)
#define CKV_AUTORELEASE(...) (__VA_ARGS__)
#define CKV_RELEASE(value) ((void)0)
#else
#define CKV_AUTORELEASE(...) [(__VA_ARGS__) autorelease]
#define CKV_RELEASE(value) [(value) release]
#endif

@interface CKVCoreMLContext : NSObject
@property(nonatomic, retain) MLModel* model;
@property(nonatomic, retain) NSURL* compiledURL;
@end

@implementation CKVCoreMLContext
- (void)dealloc {
    // Normal unload clears both fields first. This also covers a failed open.
    if (_compiledURL != nil) {
        [[NSFileManager defaultManager] removeItemAtURL:_compiledURL error:NULL];
    }
#if !__has_feature(objc_arc)
    [_model release];
    [_compiledURL release];
    [super dealloc];
#endif
}
@end

static ckv_coreml_handle ckv_retain_context(CKVCoreMLContext* context) {
#if __has_feature(objc_arc)
    return (__bridge_retained void*)context;
#else
    return (void*)[context retain];
#endif
}

static CKVCoreMLContext* ckv_context(ckv_coreml_handle handle) {
#if __has_feature(objc_arc)
    return (__bridge CKVCoreMLContext*)handle;
#else
    return (CKVCoreMLContext*)handle;
#endif
}

static void ckv_release_context(ckv_coreml_handle handle) {
#if __has_feature(objc_arc)
    id owner = (__bridge_transfer id)handle;
    (void)owner;
#else
    [(id)handle release];
#endif
}

int ckv_coreml_load(const char* model_path, ckv_coreml_handle* handle) {
    if (handle == NULL) return -1;
    *handle = NULL;
    if (model_path == NULL) return -1;
    @autoreleasepool {
        NSString* path = [NSString stringWithUTF8String:model_path];
        if (path == nil) return -1;
        NSURL* url = [NSURL fileURLWithPath:path];
        NSURL* compiled = nil;
        NSError* error = nil;
        if ([[path pathExtension] isEqualToString:@"mlpackage"]) {
            compiled = [MLModel compileModelAtURL:url error:&error];
            if (compiled == nil || error != nil) return -2;
            url = compiled;
        } else if (![[path pathExtension] isEqualToString:@"mlmodelc"]) {
            return -2;
        }
        MLModelConfiguration* config = CKV_AUTORELEASE([[MLModelConfiguration alloc] init]);
        config.computeUnits = MLComputeUnitsAll;
        MLModel* model = [MLModel modelWithContentsOfURL:url configuration:config error:&error];
        if (error != nil || model == nil) {
            if (compiled != nil) [[NSFileManager defaultManager] removeItemAtURL:compiled error:NULL];
            return -3;
        }
        // This first native profile has exactly these two Int32 tensor inputs.
        // Other exports must provide a separately verified input contract.
        NSDictionary<NSString*, MLFeatureDescription*>* inputs = model.modelDescription.inputDescriptionsByName;
        for (NSString* name in @[@"input_ids", @"attention_mask"]) {
            MLFeatureDescription* input = inputs[name];
            if (input == nil || input.type != MLFeatureTypeMultiArray ||
                input.multiArrayConstraint.dataType != MLMultiArrayDataTypeInt32) {
                if (compiled != nil) [[NSFileManager defaultManager] removeItemAtURL:compiled error:NULL];
                return -4;
            }
        }
        for (NSString* name in inputs) {
            if (![name isEqualToString:@"input_ids"] && ![name isEqualToString:@"attention_mask"] && !inputs[name].isOptional) {
                if (compiled != nil) [[NSFileManager defaultManager] removeItemAtURL:compiled error:NULL];
                return -4;
            }
        }
        CKVCoreMLContext* context = [[CKVCoreMLContext alloc] init];
        context.model = model;
        context.compiledURL = compiled;
        *handle = ckv_retain_context(context);
        CKV_RELEASE(context);
        return 0;
    }
}

int ckv_coreml_unload(ckv_coreml_handle handle) {
    if (handle == NULL) return 0;
    @autoreleasepool {
        CKVCoreMLContext* context = ckv_context(handle);
        context.model = nil;
        int result = 0;
        if (context.compiledURL != nil) {
            NSError* error = nil;
            if (![[NSFileManager defaultManager] removeItemAtURL:context.compiledURL error:&error]) result = -1;
            context.compiledURL = nil;
        }
        ckv_release_context(handle);
        return result;
    }
}

static MLMultiArray* ckv_input_tensor(const int64_t* input, int batch, int sequence) {
    if (input == NULL || batch <= 0 || sequence <= 0 || (size_t)batch > SIZE_MAX/(size_t)sequence) return nil;
    size_t count = (size_t)batch*(size_t)sequence;
    // Validate every input before creating or publishing the feature tensor.
    for (size_t i=0; i<count; i++) {
        if (input[i] < INT32_MIN || input[i] > INT32_MAX) return nil;
    }
    NSError* error = nil;
    MLMultiArray* array = CKV_AUTORELEASE([[MLMultiArray alloc] initWithShape:@[@(batch), @(sequence)] dataType:MLMultiArrayDataTypeInt32 error:&error]);
    if (array == nil || error != nil) return nil;
    for (int b=0; b<batch; b++) {
        for (int t=0; t<sequence; t++) {
            array[@[@(b), @(t)]] = @((int32_t)input[(size_t)b*(size_t)sequence+(size_t)t]);
        }
    }
    return array;
}

static int ckv_copy_output(MLMultiArray* array, int batch, int sequence, int hidden, float* output, size_t capacity) {
    if (array == nil || output == NULL || batch <= 0 || sequence <= 0 || hidden <= 0) return -7;
    if ((size_t)batch > SIZE_MAX/(size_t)sequence) return -7;
    size_t count = (size_t)batch*(size_t)sequence;
    if (count > SIZE_MAX/(size_t)hidden) return -7;
    count *= (size_t)hidden;
    if (capacity < count || array.shape.count != 3 ||
        [array.shape[0] longLongValue] != batch || [array.shape[1] longLongValue] != sequence ||
        [array.shape[2] longLongValue] != hidden) return -7;
    if (array.dataType != MLMultiArrayDataTypeFloat32 && array.dataType != MLMultiArrayDataTypeDouble) {
        if (@available(macOS 12.0, *)) {
            if (array.dataType != MLMultiArrayDataTypeFloat16) return -7;
        } else { return -7; }
    }
    size_t index = 0;
    for (int b=0; b<batch; b++) {
        for (int t=0; t<sequence; t++) {
            for (int h=0; h<hidden; h++) {
                float value = [array[@[@(b), @(t), @(h)]] floatValue];
                if (!isfinite(value)) return -8;
                output[index++] = value;
            }
        }
    }
    return 0;
}

int ckv_coreml_predict(ckv_coreml_handle handle, const int64_t* input_ids, const int64_t* attention_mask,
                      int batch_size, int seq_len, int hidden_dim, float* output, size_t output_capacity) {
    if (handle == NULL) return -1;
    if (output == NULL || batch_size <= 0 || seq_len <= 0 || hidden_dim <= 0 ||
        (size_t)batch_size > SIZE_MAX/(size_t)seq_len) return -7;
    size_t required = (size_t)batch_size*(size_t)seq_len;
    if (required > SIZE_MAX/(size_t)hidden_dim) return -7;
    required *= (size_t)hidden_dim;
    if (output_capacity < required) return -7;
    @autoreleasepool {
        CKVCoreMLContext* context = ckv_context(handle);
        if (context.model == nil) return -1;
        MLMultiArray* ids = ckv_input_tensor(input_ids, batch_size, seq_len);
        MLMultiArray* mask = ckv_input_tensor(attention_mask, batch_size, seq_len);
        if (ids == nil || mask == nil) return -2;
        NSError* error = nil;
        MLDictionaryFeatureProvider* provider = CKV_AUTORELEASE([[MLDictionaryFeatureProvider alloc]
            initWithDictionary:@{@"input_ids":ids, @"attention_mask":mask} error:&error]);
        if (error != nil || provider == nil) return -4;
        id<MLFeatureProvider> result = [context.model predictionFromFeatures:provider error:&error];
        if (error != nil || result == nil) return -5;
        MLMultiArray* array = nil;
        for (NSString* name in @[@"last_hidden_state", @"output", @"embeddings"]) {
            MLFeatureValue* value = [result featureValueForName:name];
            if (value != nil && value.type == MLFeatureTypeMultiArray) { array=value.multiArrayValue; break; }
        }
        if (array == nil) return -6;
        return ckv_copy_output(array, batch_size, seq_len, hidden_dim, output, output_capacity);
    }
}
