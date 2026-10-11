package ckgclient

import (
	"context"
	"errors"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func TestReviewFunctionResolvesTheSeedAndWrapsIndexErrors(t *testing.T) {
	m := &mockStoreReader{
		symbolOut: []types.Node{node("aaaaaaaaaaaaaaaa", "pkg.Engine.Finalize", "e.go", 1, 9, types.NodeMethod, "go")},
		reviewOut: store.Review{Seed: store.ReviewSymbol{QualifiedName: "pkg.Engine.Finalize"}, CommitsTotal: 2},
	}
	r := newRealWithStore(m)
	rv, err := r.ReviewFunction(context.Background(), "Engine.Finalize", ReviewOpts{CallerDepth: 3})
	if err != nil || rv.CommitsTotal != 2 {
		t.Fatalf("review = %+v, %v", rv, err)
	}
	if len(m.reviewCh) != 1 || m.reviewCh[0] != "pkg.Engine.Finalize" {
		t.Fatalf("store asked for %v, want the resolved qualified name", m.reviewCh)
	}
	m.reviewErr = store.ErrNoReviewIndex
	if _, err := r.ReviewFunction(context.Background(), "pkg.Engine.Finalize", ReviewOpts{}); !errors.Is(err, store.ErrNoReviewIndex) {
		t.Fatalf("old graph error = %v", err)
	}
	if _, err := r.ReviewFunction(context.Background(), "", ReviewOpts{}); err == nil {
		t.Fatal("empty seed accepted")
	}
}
