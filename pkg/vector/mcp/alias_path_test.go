package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// alias_path may only name a glossary inside the index directory or the
// indexed source root, and its errors must not echo file contents.
func TestSemanticSearchAliasPathStaysInsideTheIndex(t *testing.T) {
	eng := buildSample(t)
	s := NewServer(eng)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.env")
	if err := os.WriteFile(secret, []byte("OPENAI_API_KEY=sk-live-0123456789\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(eng.OutDir(), "link.yaml")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(eng.OutDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("OPENAI_API_KEY=sk-live-0123456789\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{secret, link, "/etc/hosts", filepath.Join(outside, "missing.yaml"), bad} {
		res, err := s.handleSemanticSearch(context.Background(), callRequest("cks.context.semantic_search", map[string]any{"intent": "server", "k": 1.0, "alias_path": p}))
		if err != nil {
			t.Fatal(err)
		}
		text := textContent(t, res)
		if !res.IsError || strings.Contains(text, "OPENAI") || strings.Contains(text, "no such file") {
			t.Fatalf("alias_path %q: IsError=%v text=%q", p, res.IsError, text)
		}
	}
	good := filepath.Join(eng.OutDir(), "glossary.yaml")
	if err := os.WriteFile(good, []byte("aliases:\n  서버: [server]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, _ := s.handleSemanticSearch(context.Background(), callRequest("cks.context.semantic_search", map[string]any{"intent": "서버", "k": 1.0, "alias_path": good}))
	if res.IsError {
		t.Fatalf("glossary in the index directory refused: %s", textContent(t, res))
	}
}
