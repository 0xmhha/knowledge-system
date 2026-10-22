package server_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/graph/server"
)

// blockingStore holds the edge-count warm-up query until released, so the
// test controls when the background warm-up finishes.
type blockingStore struct {
	persist.StoreReader
	entered  chan struct{}
	enter    sync.Once
	release  chan struct{}
	finished atomic.Bool
}

func (b *blockingStore) EdgeCountsByType() (map[string]int, error) {
	b.enter.Do(func() { close(b.entered) })
	<-b.release
	counts, err := b.StoreReader.EdgeCountsByType()
	b.finished.Store(true)
	return counts, err
}

// TestCloseWaitsForWarmUp: Close must not return while a warm-up query is
// still reading the store, or the caller's store.Close and temp-dir removal
// race with it.
func TestCloseWaitsForWarmUp(t *testing.T) {
	store := &blockingStore{StoreReader: buildFixture(t), entered: make(chan struct{}), release: make(chan struct{})}
	srv := server.New(store, nil)
	<-store.entered

	closed := make(chan bool)
	go func() {
		srv.Close()
		closed <- store.finished.Load()
	}()
	close(store.release)
	if finished := <-closed; !finished {
		t.Fatal("Close returned before the warm-up query finished")
	}
	srv.Close() // a second Close returns immediately
}
