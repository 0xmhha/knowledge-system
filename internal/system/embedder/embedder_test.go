package embedder

import (
	"context"
	"testing"
)

func TestOpenMockForStructuralEvaluation(t *testing.T) {
	e, cap, err := Open("mock", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cap.Provider != "mock" || cap.Model != e.Name() || cap.Dim != e.Dimension() || cap.Endpoint != "" {
		t.Fatalf("capability does not describe mock embedder: %+v", cap)
	}
	vecs, err := e.Embed(context.Background(), []string{"fixture"})
	if err != nil || len(vecs) != 1 || len(vecs[0]) != e.Dimension() {
		t.Fatalf("mock embedding failed: vectors=%d err=%v", len(vecs), err)
	}
	if _, _, err := Open("mock", "bge-m3", ""); err == nil {
		t.Fatal("mismatched model name accepted")
	}
}
