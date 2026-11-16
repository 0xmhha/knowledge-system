package buildpipe_test

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func fileSum(t *testing.T, f *os.File) [32]byte {
	t.Helper()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

// TestUpdateBuilds_ReplaceGraphByRename: an incremental build and a
// short-circuit build leave the graph.db a reader already holds untouched and
// put a new file in its place. Readers open graph.db with immutable=1, which
// is only safe when the file under them never changes; before the fix both
// builds wrote it in place.
func TestUpdateBuilds_ReplaceGraphByRename(t *testing.T) {
	src := makeMiniGoModule(t)
	out := t.TempDir()
	db := filepath.Join(out, "graph.db")
	runBuild(t, src, out)

	for _, step := range []struct {
		name   string
		change func()
	}{
		{"incremental", func() {
			mustWrite(t, filepath.Join(src, "a.go"), "package cachetest\n\nfunc Add(a, b int) int { return a + b + 0 }\n")
		}},
		{"short circuit", func() {}},
	} {
		held, err := os.Open(db) // what a reader has open
		if err != nil {
			t.Fatal(err)
		}
		before, _ := held.Stat()
		sum := fileSum(t, held)

		step.change()
		runBuild(t, src, out)

		after, err := os.Stat(db)
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(before, after) {
			t.Errorf("%s: graph.db was updated in place", step.name)
		}
		if fileSum(t, held) != sum {
			t.Errorf("%s: the file a reader holds changed", step.name)
		}
		_ = held.Close()
		if _, err := os.Stat(db + ".building"); !os.IsNotExist(err) {
			t.Errorf("%s: graph.db.building left behind (%v)", step.name, err)
		}
	}
}
