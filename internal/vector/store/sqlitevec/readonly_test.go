package sqlitevec

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

func TestReadOnlyIndexSearchRefusesMutationAndCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vector #?.db")
	writable, err := Open(path, testDim)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	chunk := mkChunk("a", "main.go", "Alpha", 3, 3, "go", types.KindFunction)
	if err := writable.Upsert(ctx, []types.Chunk{chunk}, [][]float32{{1, 0, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	if err := writable.Close(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	ro, err := OpenReadOnly(path, testDim)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ro.SearchDetailed(ctx, []float32{1, 0, 0, 0}, 1, types.Filter{Language: "go"}, SearchOptions{})
	if err != nil || result.Status != SearchComplete || len(result.Hits) != 1 {
		t.Fatalf("read failed: %+v %v", result, err)
	}
	if err := ro.Upsert(ctx, []types.Chunk{chunk}, [][]float32{{0, 1, 0, 0}}); err == nil {
		t.Fatal("read-only index accepted vector writes")
	}
	if err := ro.SetManifest(ctx, map[string]string{"test": "mutation"}); err == nil {
		t.Fatal("read-only index accepted manifest writes")
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("frozen DB bytes changed")
	}
	if _, err := OpenReadOnly(path, testDim+1); err == nil {
		t.Fatal("wrong dimension accepted")
	}
	missing := filepath.Join(filepath.Dir(path), "missing.db")
	if _, err := OpenReadOnly(missing, testDim); err == nil {
		t.Fatal("missing index accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("read-only open created a DB")
	}
}
