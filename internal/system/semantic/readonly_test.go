package semantic

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadOnlySemanticStorePreservesProjectionAndRejectsWrites(t *testing.T) {
	p, repo := fixture(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "semantic.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, p, repo); err != nil {
		t.Fatal(err)
	}
	if err := s.Activate(ctx, p.Snapshot.ProjectID, p.Snapshot.DatasetID); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	ro, err := OpenStoreReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ro.Load(ctx, p.Snapshot.ProjectID, p.Snapshot.DatasetID)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("read-only load: %+v %v", got, err)
	}
	p.Snapshot.DatasetID = "v2"
	p.Evidence[0].Snapshot = p.Snapshot
	if err := ro.Put(ctx, p, repo); err == nil {
		t.Fatal("read-only store wrote projection")
	}
	if err := ro.Activate(ctx, p.Snapshot.ProjectID, "v1"); err == nil {
		t.Fatal("read-only store wrote current pointer")
	}
	ro.Close()
	after, _ := os.ReadFile(path)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("query changed semantic DB")
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	if s, err := OpenStoreReadOnly(ctx, missing); err == nil {
		s.Close()
		t.Fatal("missing store opened")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("query created missing store")
	}
}
