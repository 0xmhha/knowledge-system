package ckgclient

import (
	"errors"
	"testing"
)

// countingStore counts manifest reads.
type countingStore struct {
	*mockStoreReader
	reads int
}

func (c *countingStore) LoadManifestSnapshot() (ManifestSnapshot, error) {
	c.reads++
	return c.mockStoreReader.LoadManifestSnapshot()
}

// Every citation carries the indexed commit. Reading the whole manifest per
// request cost about 85 MB on a production-size graph; a successful read is
// reused for the store's lifetime, a failed one is retried.
func TestCommitIsReadOnceAndFailuresAreRetried(t *testing.T) {
	cs := &countingStore{mockStoreReader: &mockStoreReader{manifestErr: errors.New("db locked")}}
	r := newRealWithStore(cs)
	if c, err := r.commit(); err == nil || c != "" {
		t.Fatalf("failed read returned %q, %v", c, err)
	}
	cs.manifestErr = nil
	cs.manifest = ManifestSnapshot{SrcCommit: "abc123"}
	for i := 0; i < 3; i++ {
		if c, err := r.commit(); err != nil || c != "abc123" {
			t.Fatalf("commit = %q, %v", c, err)
		}
	}
	if cs.reads != 2 {
		t.Fatalf("manifest reads = %d, want 2 (one failure, one success)", cs.reads)
	}
}
