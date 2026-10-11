package mcp

import (
	"context"
	"testing"
)

// Size arguments come from callers; a huge value must not size an
// allocation. Handlers are called directly, without the recovery middleware.
func TestHugeKDoesNotPanic(t *testing.T) {
	s := NewServer(buildSample(t))
	for _, c := range []struct {
		name string
		call func() bool
	}{
		{"keyword_search", func() bool {
			r, err := s.handleKeywordSearch(context.Background(), callRequest("cks.context.keyword_search", map[string]any{"query": "server", "k": 1e15}))
			return err == nil && r != nil
		}},
		{"semantic_search", func() bool {
			r, err := s.handleSemanticSearch(context.Background(), callRequest("cks.context.semantic_search", map[string]any{"intent": "server", "k": 1e15}))
			return err == nil && r != nil
		}},
		{"vector_search", func() bool {
			r, err := s.handleVectorSearch(context.Background(), callRequest("cks.context.vector_search", map[string]any{"query": "server", "k": 1e15}))
			return err == nil && r != nil
		}},
		{"find_branches", func() bool {
			r, err := s.handleFindBranches(context.Background(), callRequest("cks.flow.find_branches", map[string]any{"symptom": "server error", "k": 1e15}))
			return err == nil && r != nil
		}},
	} {
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("%s panicked: %v", c.name, p)
				}
			}()
			if !c.call() {
				t.Errorf("%s returned no result", c.name)
			}
		}()
	}
}
