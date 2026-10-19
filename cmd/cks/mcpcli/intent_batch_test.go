package mcpcli

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	ckvtypes "github.com/0xmhha/knowledge-system/pkg/vector/types"
)

type intentBatchProvider struct {
	raw         bool
	budget      int
	calls       [][]string
	fail        error
	short       bool
	wrongDim    bool
	reuse       bool
	mutateInput bool
	shared      []float32
}

func (p *intentBatchProvider) Identity() ckvtypes.EmbeddingIdentity {
	id := ckvtypes.EmbeddingIdentity{Model: "owned", Dim: 2, RuntimeBatchTokens: p.budget}
	if p.raw {
		id.PassageTransform = "raw:v1"
	}
	return id
}
func (p *intentBatchProvider) Name() string        { return "owned" }
func (p *intentBatchProvider) Dimension() int      { return 2 }
func (p *intentBatchProvider) MaxInputTokens() int { return 8192 }
func (p *intentBatchProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	p.calls = append(p.calls, append([]string(nil), texts...))
	if p.fail != nil {
		return nil, p.fail
	}
	if p.short {
		return nil, nil
	}
	var result [][]float32
	for _, text := range texts {
		v := []float32{float32(len(text)), 1}
		if p.wrongDim {
			v = v[:1]
		}
		if p.reuse {
			if p.shared == nil {
				p.shared = make([]float32, 2)
			}
			copy(p.shared, v)
			v = p.shared
		}
		result = append(result, v)
	}
	if p.mutateInput && len(texts) > 0 {
		texts[0] = "provider changed caller input"
	}
	return result, nil
}
func TestIntentBatchAdapterGroupsKnownRawBudget(t *testing.T) {
	p := &intentBatchProvider{raw: true, budget: 8192}
	a := intentEmbedderAdapter{e: p}
	texts := make([]string, 65)
	for i := range texts {
		texts[i] = "anchor"
	}
	result, err := a.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.calls) != 1 || len(p.calls[0]) != 65 || len(result) != 65 {
		t.Fatalf("startup wasn't grouped: calls=%d result=%d", len(p.calls), len(result))
	}
}
func TestIntentBatchAdapterUnknownCapabilitiesStaySingle(t *testing.T) {
	for _, p := range []*intentBatchProvider{{raw: true, budget: 0}, {raw: false, budget: 8192}} {
		_, err := (intentEmbedderAdapter{e: p}).EmbedBatch(context.Background(), []string{"a", "bb", "ccc"})
		if err != nil || len(p.calls) != 3 {
			t.Fatalf("unknown capability grouped inputs: calls=%d err=%v", len(p.calls), err)
		}
		for _, call := range p.calls {
			if len(call) != 1 {
				t.Fatalf("unknown provider batch=%v", call)
			}
		}
	}
}
func TestIntentBatchAdapterBudgetUsesUTF8AndForwardsLongInput(t *testing.T) {
	p := &intentBatchProvider{raw: true, budget: 12}
	_, err := (intentEmbedderAdapter{e: p}).EmbedBatch(context.Background(), []string{"가", "나"})
	if err != nil || len(p.calls) != 2 {
		t.Fatalf("UTF8 budget exceeded: %v %v", p.calls, err)
	}
	p = &intentBatchProvider{raw: true, budget: 20}
	long := strings.Repeat("x", 21)
	_, err = (intentEmbedderAdapter{e: p}).EmbedBatch(context.Background(), []string{"aa", "bb", long})
	if err != nil || !reflect.DeepEqual(p.calls, [][]string{{"aa", "bb"}, {long}}) {
		t.Fatalf("long input was changed or lost: %v %v", p.calls, err)
	}
}
func TestIntentBatchAdapterCopiesInputAndCrossCallVectors(t *testing.T) {
	p := &intentBatchProvider{reuse: true, mutateInput: true}
	texts := []string{"a", "bb", "ccc"}
	got, err := (intentEmbedderAdapter{e: p}).EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(texts, []string{"a", "bb", "ccc"}) || !reflect.DeepEqual(got, [][]float32{{1, 1}, {2, 1}, {3, 1}}) {
		t.Fatalf("provider aliases leaked: input=%v output=%v", texts, got)
	}
}
func TestIntentBatchAdapterProviderFailureDoesNotRetry(t *testing.T) {
	down := errors.New("owned model down")
	p := &intentBatchProvider{raw: true, budget: 8192, fail: down}
	_, err := (intentEmbedderAdapter{e: p}).EmbedBatch(context.Background(), []string{"a", "b"})
	if !errors.Is(err, down) || len(p.calls) != 1 {
		t.Fatalf("provider error retried: %v %v", p.calls, err)
	}
}
func TestIntentBatchAdapterMalformedProviderDoesNotRetry(t *testing.T) {
	for _, p := range []*intentBatchProvider{{raw: true, budget: 8192, short: true}, {raw: true, budget: 8192, wrongDim: true}} {
		_, err := (intentEmbedderAdapter{e: p}).EmbedBatch(context.Background(), []string{"a", "b"})
		if err == nil || len(p.calls) != 1 {
			t.Fatalf("malformed response accepted or retried: %v %v", p.calls, err)
		}
	}
}
func TestIntentBatchAdapterCancellationAvoidsProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &intentBatchProvider{raw: true, budget: 8192}
	_, err := (intentEmbedderAdapter{e: p}).EmbedBatch(ctx, []string{"a"})
	if !errors.Is(err, context.Canceled) || len(p.calls) != 0 {
		t.Fatalf("cancelled input reached provider: %v", err)
	}
}
