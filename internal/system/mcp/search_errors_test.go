package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func TestSemanticSearchPreflightPreservesSourceCodeBeforeBackend(t *testing.T) {
	for _, code := range []string{"source_missing", "snapshot_mismatch"} {
		t.Run(code, func(t *testing.T) {
			version := newPinnedV2ErrorFixture(t)
			dir := filepath.Join(version, "sources", "blobs")
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("blobs: %v %v", entries, err)
			}
			path := filepath.Join(dir, entries[0].Name())
			if code == "source_missing" {
				err = os.Remove(path)
			} else {
				err = os.Chmod(path, 0600)
				if err == nil {
					err = os.WriteFile(path, []byte("PRIVATE never return\n"), 0600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			f := newFixture(t, nil)
			f.deps.EvidenceVersionDir = version
			r, err := handleSemanticSearch(context.Background(), f.deps, callToolReq(map[string]any{"query": "query"}))
			if err != nil || r == nil || !r.IsError || r.StructuredContent.(map[string]string)["code"] != code {
				t.Fatalf("preflight code: %+v %v", r, err)
			}
			if strings.Contains(resultText(r), version) || strings.Contains(resultText(r), "PRIVATE") {
				t.Fatal("preflight leaked private data")
			}
			if len(f.ckv.Calls.SemanticSearch) != 0 {
				t.Fatal("invalid archive reached embedding backend")
			}
		})
	}
}
