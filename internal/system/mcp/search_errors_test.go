package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/vector/ckv"
)

func TestSemanticSearchErrorClassesAndBoundedPublicMessage(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		err        error
	}{
		{"source", "source_missing", errors.New("source_missing: /private/source")},
		{"integrity", "snapshot_mismatch", errors.New("snapshot_mismatch: private hash")},
		{"model", "embedding_failed", fmt.Errorf("provider source_missing private: %w", ckv.ErrEmbeddingFailed)},
		{"provider_timeout", "embedding_failed", fmt.Errorf("%w: %w", ckv.ErrEmbeddingFailed, context.DeadlineExceeded)},
		{"cancel", "request_canceled", context.Canceled},
		{"deadline", "request_timeout", context.DeadlineExceeded},
		{"unknown", "retrieval_failed", errors.New("other private error")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, func(f *fixture) { f.ckv.SearchErr = tc.err })
			r, err := handleSemanticSearch(context.Background(), f.deps, callToolReq(map[string]any{"query": "query"}))
			if err != nil || r == nil || !r.IsError {
				t.Fatalf("error not returned: %+v %v", r, err)
			}
			code := r.StructuredContent.(map[string]string)["code"]
			if code != tc.code || !strings.HasPrefix(resultText(r), "code="+code+": ") || strings.Contains(resultText(r), "private") {
				t.Fatalf("wrong/unbounded classification: %+v", r)
			}
			if got := v2ToolErrorFor(tc.err, "compose_failed").StructuredContent.(map[string]string)["code"]; tc.name != "unknown" && got != tc.code {
				t.Fatalf("v2 class=%s want=%s", got, tc.code)
			}
		})
	}
}
