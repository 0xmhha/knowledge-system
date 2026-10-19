//go:build darwin && cgo && tokenizers

package coreml

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestNativeTensorShapeArithmetic(t *testing.T) {
	if n, err := checkedTensorSize(2, 3, 4); err != nil || n != 24 {
		t.Fatal(n, err)
	}
	max := int(^uint(0) >> 1)
	for _, shape := range [][3]int{{0, 1, 1}, {1, -1, 1}, {1, 1, 0}, {max, 2, 2}, {1, max / 4, 1}, {1, 2, max}} {
		if _, err := checkedTensorSize(shape[0], shape[1], shape[2]); err == nil {
			t.Fatal("invalid shape accepted", shape)
		}
	}
}
func TestNativeL2NormalizationDoesNotOverflowFloat32Norm(t *testing.T) {
	for _, vec := range [][]float32{{3, 4}, {math.MaxFloat32, math.MaxFloat32}, {0, 0}} {
		l2Normalize(vec)
		var sum float64
		for _, v := range vec {
			if math.IsInf(float64(v), 0) || math.IsNaN(float64(v)) {
				t.Fatal(vec)
			}
			sum += float64(v) * float64(v)
		}
		if vec[0] != 0 && math.Abs(sum-1) > 1e-6 {
			t.Fatal(vec, sum)
		}
	}
}

type nativeCloseFixture struct{ closed int }

func (f *nativeCloseFixture) Encode(string, int) ([]int64, []int64, error) {
	return nil, nil, errors.New("should not be called")
}
func (f *nativeCloseFixture) Close() error {
	f.closed++
	return errors.New("owned tokenizer close fixture")
}
func TestNativeAdapterCloseOnceAndCanceledNoOutput(t *testing.T) {
	tok := &nativeCloseFixture{}
	a := &Adapter{tokenizer: tok}
	if err := a.Close(); err == nil {
		t.Fatal("close error lost")
	}
	if err := a.Close(); err != nil || tok.closed != 1 {
		t.Fatal(err, tok.closed)
	}
	if result, err := a.Embed(context.Background(), []string{"hello"}); err == nil || result != nil {
		t.Fatal(result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := a.Embed(ctx, []string{"hello"}); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal(result, err)
	}
}
func TestNativeTokenizerLocalWordLevelPaddingAndTruncation(t *testing.T) {
	tok, err := newTokenizer("testdata/wordlevel-tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tok.Close() }()
	ids, mask, err := tok.Encode("hello world", 4)
	if err != nil || !reflect.DeepEqual(ids, []int64{2, 3, 0, 0}) || !reflect.DeepEqual(mask, []int64{1, 1, 0, 0}) {
		t.Fatal(ids, mask, err)
	}
	ids, mask, err = tok.Encode("hello world", 1)
	if err != nil || !reflect.DeepEqual(ids, []int64{2}) || !reflect.DeepEqual(mask, []int64{1}) {
		t.Fatal(ids, mask, err)
	}
	if err := tok.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tok.Encode("hello", 1); err == nil {
		t.Fatal("closed tokenizer encoded")
	}
}
