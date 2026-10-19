package backendmeasure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/intent"
)

type measuredIntentBatch struct {
	intent.Embedder
	ctx    context.Context
	texts  []string
	calls  int
	result [][]float32
	err    error
}

func (b *measuredIntentBatch) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	b.ctx = ctx
	b.texts = texts
	b.calls++
	return b.result, b.err
}
func TestIntentMeasurementPreservesOnlySupportedBatchCapability(t *testing.T) {
	scalar := &intent.FakeEmbedder{}
	if _, ok := WrapIntent(scalar).(intent.BatchEmbedder); ok {
		t.Fatal("measurement invented a batch capability")
	}
	if _, ok := WrapIntent(&measuredIntentBatch{Embedder: scalar}).(intent.BatchEmbedder); !ok {
		t.Fatal("measurement erased batching")
	}
}
func TestIntentBatchMeasurementPreservesDelegationAndFingerprint(t *testing.T) {
	for _, backendErr := range []error{nil, errors.New("PRIVATE_BACKEND"), context.Canceled} {
		base := &measuredIntentBatch{Embedder: &intent.FakeEmbedder{}, result: [][]float32{{1, 0}, {0, 1}}, err: backendErr}
		texts := []string{"PRIVATE_ANCHOR_A", "PRIVATE_ANCHOR_B"}
		ctx, scope := (&Recorder{}).Begin(context.Background(), "batch-startup", "startup.intent_anchors")
		wrapped := WrapIntent(base)
		batch, ok := wrapped.(intent.BatchEmbedder)
		if !ok {
			t.Fatal("batch missing")
		}
		got, err := batch.EmbedBatch(ctx, texts)
		if err != backendErr || !reflect.DeepEqual(got, base.result) || base.ctx != ctx || !reflect.DeepEqual(base.texts, texts) || base.calls != 1 {
			t.Fatalf("batch delegation changed: %v", err)
		}
		report := scope.Finish("done")
		if len(report.Calls) != 1 {
			t.Fatalf("attempts=%d", len(report.Calls))
		}
		call := report.Calls[0]
		input, _ := json.Marshal(texts)
		hash := sha256.Sum256(input)
		if call.Backend != "intent" || call.Method != "embed_batch" || call.ResultCount != 2 || call.InputSHA256 != hex.EncodeToString(hash[:]) || call.InputBytes != len(input) {
			t.Fatalf("batch call unit or fingerprint changed: %+v", call)
		}
		data, _ := json.Marshal(report)
		if strings.Contains(string(data), "PRIVATE_") {
			t.Fatal("measurement released raw input or backend message")
		}
	}
}
func TestIntentBatchMeasurementWithoutScopeStillDelegates(t *testing.T) {
	base := &measuredIntentBatch{Embedder: &intent.FakeEmbedder{}, result: [][]float32{{1, 0}}}
	ctx := context.Background()
	got, err := WrapIntent(base).(intent.BatchEmbedder).EmbedBatch(ctx, []string{"text"})
	if err != nil || !reflect.DeepEqual(got, base.result) || base.calls != 1 || base.ctx != ctx {
		t.Fatal("scope-free batch changed")
	}
}
