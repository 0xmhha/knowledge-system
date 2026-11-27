package persist

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func gzipped(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestBlobText: a gzip blob (a Hunk patch) comes back as its text; any
// other blob comes back unchanged.
func TestBlobText(t *testing.T) {
	for _, c := range []struct {
		name string
		in   []byte
		want string
	}{
		{"gzip", gzipped(t, "@@ -1 +1 @@\n-a\n+b\n"), "@@ -1 +1 @@\n-a\n+b\n"},
		{"plain", []byte("func A() {}"), "func A() {}"},
		{"empty", nil, ""},
	} {
		got, err := BlobText(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if string(got) != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestExportChunked_HunkBlobsAreText: a Hunk node's patch, stored gzipped,
// is written to blobs/<id>.txt as text. It was written as gzip bytes under
// the .txt name.
func TestExportChunked_HunkBlobsAreText(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	const id = "hhhhhhhhhhhhhhhh"
	if err := s.InsertNodes([]types.Node{{ID: id, Type: types.NodeHunk, Name: "h", QualifiedName: "h", FilePath: "a.go",
		StartLine: 1, EndLine: 1, Language: "go", Confidence: types.ConfExtracted}}); err != nil {
		t.Fatal(err)
	}
	const patch = "@@ -1 +1 @@\n-old\n+new\n"
	if err := s.InsertBlobs(map[string][]byte{id: gzipped(t, patch)}); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := s.ExportChunked(out, 10, 10); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(out, "blobs", id+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != patch {
		t.Errorf("blob file = %q, want the patch text", got)
	}
}
