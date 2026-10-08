package query

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/vector/build"
	"github.com/0xmhha/knowledge-system/internal/vector/embed/mock"
)

// Exercise the query response, not just the separate freshness endpoint:
// callers still receive indexed snippets after HEAD changes, but must not
// mistake them for current-source evidence.
func TestSearchMetadataFreshTracksSourceHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git required for source-HEAD regression")
	}
	src, out := t.TempDir(), t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		prefix := []string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgSign=false", "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-C", src}
		cmd := exec.Command("git", append(prefix, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	write := func(value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(src, "main.go"), []byte("package fixture\n\nfunc Alpha() string { return \""+value+"\" }\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "--quiet")
	write("old-token")
	git("add", "main.go")
	git("commit", "--quiet", "-m", "old")
	oldHead := git("rev-parse", "HEAD")
	if _, err := build.Run(context.Background(), build.Options{SrcRoot: src, OutDir: out, Embedder: mock.Default()}); err != nil {
		t.Fatal(err)
	}
	eng, err := Open(out, mock.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	search := func(opts Options) *Response {
		t.Helper()
		r, err := eng.Search(context.Background(), "Alpha", opts)
		if err != nil {
			t.Fatal(err)
		}
		if r.Metadata.IndexedHeadCKV != oldHead {
			t.Fatalf("indexed HEAD changed: %+v", r.Metadata)
		}
		return r
	}
	options := Options{K: 10, Threshold: -1}
	initial := search(options)
	if !initial.Metadata.Fresh || len(initial.Hits) == 0 {
		t.Fatalf("matching source HEAD should be fresh with hits: %+v", initial)
	}
	write("new-token")
	git("add", "main.go")
	git("commit", "--quiet", "-m", "new")
	changed := search(options)
	if changed.Metadata.Fresh {
		t.Error("changed source HEAD incorrectly reported fresh=true")
	}
	if len(changed.Hits) != len(initial.Hits) {
		t.Fatalf("freshness hint changed hit count: %d vs %d", len(changed.Hits), len(initial.Hits))
	}
	for _, h := range changed.Hits {
		if !h.StaleCitation || h.Citation.CommitHash != oldHead || strings.Contains(h.Snippet, "new-token") {
			t.Fatalf("indexed evidence was replaced or stale flag lost: %+v", h)
		}
	}
	options.Threshold = 2 // No hits must not conceal the known HEAD mismatch.
	empty := search(options)
	if len(empty.Hits) != 0 || empty.Metadata.Fresh {
		t.Errorf("empty stale result should still be fresh=false: %+v", empty)
	}
	// A source archive has no Git HEAD. Preserve the best-effort hint when
	// the comparison is unavailable rather than making archives unusable.
	archive := t.TempDir()
	if err := os.WriteFile(filepath.Join(archive, "main.go"), []byte("package fixture\n\nfunc Alpha() string { return \"old-token\" }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	options = Options{K: 10, Threshold: -1, SrcRoot: archive}
	retained := search(options)
	if !retained.Metadata.Fresh || len(retained.Hits) == 0 {
		t.Fatalf("unknown source HEAD should retain the best-effort hint: %+v", retained)
	}
}
