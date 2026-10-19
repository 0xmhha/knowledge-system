// Native feature-provider fixtures exercise the production bridge without
// loading weights or asserting inference/model quality. Compile under MRC/ARC.
#import "../bridge_darwin.m"
#include <stdio.h>
#include <stdlib.h>

static int releasedModels=0;
static int fixturePredictions=0;
static void require(BOOL ok) { if (!ok) abort(); }

@interface CKVFixtureModel : NSObject
@property(nonatomic,retain) MLMultiArray* result;
@end
@implementation CKVFixtureModel
- (id<MLFeatureProvider>)predictionFromFeatures:(id<MLFeatureProvider>)features error:(NSError**)error {
    fixturePredictions++;
    MLMultiArray* ids=[features featureValueForName:@"input_ids"].multiArrayValue;
    MLMultiArray* mask=[features featureValueForName:@"attention_mask"].multiArrayValue;
    for (int i=0; i<3; i++) {
        require([ids[@[@0,@(i)]] longLongValue]==101+i);
        require([mask[@[@0,@(i)]] longLongValue]==1);
    }
    return CKV_AUTORELEASE([[MLDictionaryFeatureProvider alloc] initWithDictionary:@{@"last_hidden_state":self.result} error:error]);
}
- (void)dealloc {
    releasedModels++;
#if !__has_feature(objc_arc)
    [_result release];
    [super dealloc];
#endif
}
@end

static ckv_coreml_handle fixtureHandle(MLMultiArray* result) {
    CKVFixtureModel* model=[[CKVFixtureModel alloc] init];
    model.result=result;
    CKVCoreMLContext* context=[[CKVCoreMLContext alloc] init];
    context.model=(MLModel*)model;
    ckv_coreml_handle handle=ckv_retain_context(context);
    CKV_RELEASE(model);
    CKV_RELEASE(context);
    return handle;
}
static MLMultiArray* array(MLMultiArrayDataType type, NSArray<NSNumber*>* shape) {
    NSError* error=nil;
    MLMultiArray* result=CKV_AUTORELEASE([[MLMultiArray alloc] initWithShape:shape dataType:type error:&error]);
    require(result!=nil && error==nil);
    return result;
}
static void checkOutput(MLMultiArray* value) {
    float output[6]={0};
    require(ckv_copy_output(value,1,3,2,output,6)==0);
    for(int i=0;i<6;i++) require(output[i]==i+1);
}
int main(void) {
    @autoreleasepool {
        int64_t ids[3]={101,102,103}, mask[3]={1,1,1};
        MLMultiArray* input=ckv_input_tensor(ids,1,3);
        for(int i=0;i<3;i++) require([input[@[@0,@(i)]] longLongValue]==101+i);
        ids[1]=999;
        require([input[@[@0,@1]] longLongValue]==102);
        ids[1]=102;
        int64_t outside[1]={INT64_MAX};
        require(ckv_input_tensor(outside,1,1)==nil);
        require(ckv_input_tensor(NULL,1,3)==nil);
        require(ckv_input_tensor(ids,0,3)==nil);

        MLMultiArray* normal=array(MLMultiArrayDataTypeFloat32,@[@1,@3,@2]);
        MLMultiArray* wide=array(MLMultiArrayDataTypeDouble,@[@1,@3,@2]);
        for(int t=0;t<3;t++)for(int h=0;h<2;h++) {
            normal[@[@0,@(t),@(h)]]=@(t*2+h+1);
            wide[@[@0,@(t),@(h)]]=@(t*2+h+1);
        }
        checkOutput(normal);checkOutput(wide);
        if (@available(macOS 12.0,*)) {
            MLMultiArray* half=array(MLMultiArrayDataTypeFloat16,@[@1,@3,@2]);
            for(int t=0;t<3;t++)for(int h=0;h<2;h++)half[@[@0,@(t),@(h)]]=@(t*2+h+1);
            checkOutput(half);
        }
        float storage[6]={1,3,5,2,4,6};NSError* error=nil;
        MLMultiArray* strided=CKV_AUTORELEASE([[MLMultiArray alloc] initWithDataPointer:storage shape:@[@1,@3,@2] dataType:MLMultiArrayDataTypeFloat32 strides:@[@6,@1,@3] deallocator:nil error:&error]);
        require(error==nil);checkOutput(strided);
        float sentinel[6]={99,99,99,99,99,99};
        require(ckv_copy_output(normal,1,3,2,sentinel,5)==-7);
        require(ckv_copy_output(array(MLMultiArrayDataTypeFloat32,@[@1,@6]),1,3,2,sentinel,6)==-7);
        require(ckv_copy_output(array(MLMultiArrayDataTypeInt32,@[@1,@3,@2]),1,3,2,sentinel,6)==-7);
        for(int i=0;i<6;i++)require(sentinel[i]==99);
        normal[@[@0,@0,@0]]=@(NAN);
        require(ckv_copy_output(normal,1,3,2,sentinel,6)==-8);
        normal[@[@0,@0,@0]]=@1;

        ckv_coreml_handle first=fixtureHandle(normal), second=fixtureHandle(wide);
        float output[6]={0};
        require(ckv_coreml_predict(first,ids,mask,1,3,2,output,6)==0);
        require(ckv_coreml_predict(first,ids,mask,1,3,2,output,5)==-7);
        require(fixturePredictions==1);
        require(ckv_coreml_unload(first)==0);
        require(releasedModels==1);
        require(ckv_coreml_predict(second,ids,mask,1,3,2,output,6)==0);
        for(int i=0;i<6;i++)require(output[i]==i+1);
        require(ckv_coreml_unload(second)==0);
        require(releasedModels==2);
        require(ckv_coreml_unload(NULL)==0);
        require(ckv_coreml_predict(NULL,ids,mask,1,3,2,output,6)==-1);
        ckv_coreml_handle missing=(ckv_coreml_handle)(uintptr_t)1;
        require(ckv_coreml_load("fixture.gguf",&missing)!=0 && missing==NULL);
        printf("{\"tokens\":[101,102,103],\"mask\":[1,1,1],\"float32_double_float16_and_strided\":true,\"invalid_output_refused\":true,\"models_released\":%d,\"fixture_predictions\":%d,\"actual_model_inference_calls\":0}\n",releasedModels,fixturePredictions);
    }
    return 0;
}
