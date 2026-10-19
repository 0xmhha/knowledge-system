package intent

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type anchorBatchSpy struct {
	base        *FakeEmbedder
	batchCalls  int
	scalarCalls int
	input       []string
	returned    [][]float32
	override    [][]float32
	batchErr    error
	during      func()
}

func (s *anchorBatchSpy) Embed(ctx context.Context, text string) ([]float32, error) {
	s.scalarCalls++
	return s.base.Embed(ctx, text)
}
func (s *anchorBatchSpy) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	s.batchCalls++
	s.input = texts
	if s.during != nil {
		s.during()
	}
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	if s.override != nil {
		return s.override, nil
	}
	var result [][]float32
	for _, text := range texts {
		vec, err := s.base.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		result = append(result, vec)
	}
	s.returned = result
	return result, nil
}
func batchAnchors() map[contract.Intent][]string {
	return map[contract.Intent][]string{contract.IntentBugFix: {"bug-a", "bug-b"}, contract.IntentFeatureAdd: {"feature-a"}}
}
func batchBase() *FakeEmbedder {
	return &FakeEmbedder{Dim: 2, Vectors: map[string][]float32{"bug-a": {1, 0}, "bug-b": {1, 0}, "feature-a": {0, 1}, "bug-q": {1, 0}}}
}

func TestAnchorBatchMapsCopiesAndKeepsScalarQuery(t *testing.T) {
	spy := &anchorBatchSpy{base: batchBase()}
	c, err := New(context.Background(), spy, WithAnchors(batchAnchors()))
	if err != nil {
		t.Fatal(err)
	}
	if spy.batchCalls != 1 || spy.scalarCalls != 0 {
		t.Fatalf("startup batch=%d scalar=%d", spy.batchCalls, spy.scalarCalls)
	}
	if !reflect.DeepEqual(spy.input, []string{"bug-a", "bug-b", "feature-a"}) {
		t.Fatalf("unstable positional mapping: %v", spy.input)
	}
	spy.returned[0][0] = 0
	spy.input[0] = "provider overwrote input slice"
	got, err := c.Classify(context.Background(), "bug-q")
	if err != nil {
		t.Fatal(err)
	}
	if got.Intent != contract.IntentBugFix || got.BestAnchor != "bug-a" || got.Confidence != 1 {
		t.Fatalf("anchor changed through provider aliases: %+v", got)
	}
	if spy.scalarCalls != 1 || spy.batchCalls != 1 {
		t.Fatalf("query path changed: batch=%d scalar=%d", spy.batchCalls, spy.scalarCalls)
	}
}
func TestAnchorBatchAbsentPreservesScalarFallback(t *testing.T) {
	scalar := batchBase()
	c, err := New(context.Background(), scalar, WithAnchors(batchAnchors()))
	if err != nil {
		t.Fatal(err)
	}
	if len(scalar.Calls) != 3 {
		t.Fatalf("startup scalar calls=%d", len(scalar.Calls))
	}
	got, err := c.Classify(context.Background(), "bug-q")
	if err != nil || got.Intent != contract.IntentBugFix {
		t.Fatalf("fallback classification=%+v, %v", got, err)
	}
}
func TestAnchorBatchMalformedResultsAbortWithoutFallback(t *testing.T) {
	cases := map[string][][]float32{
		"short": {{1, 0}, {0, 1}}, "long": {{1, 0}, {1, 0}, {0, 1}, {0, 1}}, "empty": {{}, {}, {}}, "mixed_dimension": {{1, 0}, {1}, {0, 1}}, "nan": {{1, 0}, {float32(math.NaN()), 0}, {0, 1}}, "infinity": {{1, 0}, {float32(math.Inf(1)), 0}, {0, 1}},
	}
	for name, vectors := range cases {
		t.Run(name, func(t *testing.T) {
			spy := &anchorBatchSpy{base: batchBase(), override: vectors}
			c, err := New(context.Background(), spy, WithAnchors(batchAnchors()))
			if err == nil || c != nil || spy.batchCalls != 1 || spy.scalarCalls != 0 {
				t.Fatalf("malformed startup accepted/retried: c=%v err=%v batch=%d scalar=%d", c, err, spy.batchCalls, spy.scalarCalls)
			}
		})
	}
}
func TestAnchorBatchBackendErrorDoesNotBecomeScalarRetry(t *testing.T) {
	down := errors.New("owned provider unavailable")
	spy := &anchorBatchSpy{base: batchBase(), batchErr: down}
	_, err := New(context.Background(), spy, WithAnchors(batchAnchors()))
	if !errors.Is(err, down) || spy.scalarCalls != 0 || spy.batchCalls != 1 {
		t.Fatalf("provider failure hidden: %v, calls=%d/%d", err, spy.batchCalls, spy.scalarCalls)
	}
}
func TestAnchorBatchCancellationBoundaries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	spy := &anchorBatchSpy{base: batchBase()}
	_, err := New(ctx, spy, WithAnchors(batchAnchors()))
	if !errors.Is(err, context.Canceled) || spy.batchCalls != 0 || spy.scalarCalls != 0 {
		t.Fatalf("cancelled context reached provider: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	spy = &anchorBatchSpy{base: batchBase(), during: cancel}
	_, err = New(ctx, spy, WithAnchors(batchAnchors()))
	if !errors.Is(err, context.Canceled) || spy.scalarCalls != 0 {
		t.Fatalf("cancel during provider published anchors: %v", err)
	}
}
func TestAnchorBatchEmptyConfigAvoidsProvider(t *testing.T) {
	spy := &anchorBatchSpy{base: batchBase()}
	_, err := New(context.Background(), spy, WithAnchors(map[contract.Intent][]string{}))
	if err != nil || spy.batchCalls != 0 || spy.scalarCalls != 0 {
		t.Fatalf("empty anchors made provider call: %v", err)
	}
}
