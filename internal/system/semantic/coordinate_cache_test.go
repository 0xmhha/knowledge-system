package semantic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func countingCoordinates() (*coordinateDecodeCache, *atomic.Int64) {
	calls := new(atomic.Int64)
	return &coordinateDecodeCache{unmarshal: func(buf []byte, out any) error {
		calls.Add(1)
		return json.Unmarshal(buf, out)
	}}, calls
}

func TestCoordinateCacheReusesOnlyIdenticalBytesAndValueCopies(t *testing.T) {
	c, calls := countingCoordinates()
	raw := []byte(`{"project_id":"one","src_commit":"commit","ignored":[1,2,3]}`)
	var first graphCoordinates
	if err := c.decode(raw, &first); err != nil {
		t.Fatal(err)
	}
	first.ProjectID = "caller mutation"
	var second graphCoordinates
	if err := c.decode(raw, &second); err != nil || second.ProjectID != "one" || calls.Load() != 1 {
		t.Fatalf("shared value or repeated decode: %+v calls=%d err=%v", second, calls.Load(), err)
	}
	var changed graphCoordinates
	if err := c.decode([]byte(`{"project_id":"two","src_commit":"commit","ignored":[1,2,3]}`), &changed); err != nil || changed.ProjectID != "two" || calls.Load() != 2 {
		t.Fatalf("changed bytes reused: %+v calls=%d err=%v", changed, calls.Load(), err)
	}
}

func TestCoordinateReadDetectsChangedBytesWithRestoredFileMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`{"src_commit":"aaa"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	var first graphCoordinates
	if err := readCoordinates(path, &first); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"src_commit":"bbb"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	var changed graphCoordinates
	if err := readCoordinates(path, &changed); err != nil || changed.SrcCommit != "bbb" {
		t.Fatalf("metadata restored behind cache: %+v %v", changed, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	var missing graphCoordinates
	if err := readCoordinates(path, &missing); err == nil {
		t.Fatal("cached result hid removed manifest")
	}
}

func TestCoordinateCacheKeepsGraphAndVectorDecodingSeparate(t *testing.T) {
	c, calls := countingCoordinates()
	raw := []byte(`{"project_id":"one","symbol_count":17,"schema_version":"1.23"}`)
	var graph graphCoordinates
	var vector vectorCoordinates
	if err := c.decode(raw, &graph); err != nil {
		t.Fatal(err)
	}
	if err := c.decode(raw, &vector); err != nil || vector.SymbolCount != 17 || graph.SchemaVersion != "1.23" || calls.Load() != 2 {
		t.Fatalf("type cache collision: %+v %+v %v", graph, vector, err)
	}
}

func TestCoordinateCacheDoesNotRememberInvalidJSONOrMissingFields(t *testing.T) {
	c, calls := countingCoordinates()
	for i := 0; i < 2; i++ {
		var graph graphCoordinates
		if err := c.decode([]byte(`{"project_id":`), &graph); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
	if calls.Load() != 2 || len(c.values) != 0 {
		t.Fatal("invalid JSON cached")
	}
	var full graphCoordinates
	if err := c.decode([]byte(`{"project_id":"one","src_root":"root"}`), &full); err != nil {
		t.Fatal(err)
	}
	var missing graphCoordinates
	if err := c.decode([]byte(`{"project_id":"one"}`), &missing); err != nil || missing.SrcRoot != "" {
		t.Fatalf("deleted coordinate borrowed: %+v %v", missing, err)
	}
}

func TestCoordinateCacheIsBoundedAndReparsesEvictedBytes(t *testing.T) {
	c, calls := countingCoordinates()
	for i := 0; i <= coordinateCacheEntries; i++ {
		var graph graphCoordinates
		if err := c.decode([]byte(fmt.Sprintf(`{"project_id":"%d"}`, i)), &graph); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.values) != coordinateCacheEntries || len(c.order) != coordinateCacheEntries {
		t.Fatal("unbounded parsed manifest retention")
	}
	var first graphCoordinates
	if err := c.decode([]byte(`{"project_id":"0"}`), &first); err != nil || calls.Load() != coordinateCacheEntries+2 {
		t.Fatal("evicted coordinates not reparsed")
	}
}

func TestCoordinateCacheConcurrentReadsCannotPoisonStoredValues(t *testing.T) {
	c, calls := countingCoordinates()
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			var graph graphCoordinates
			if err := c.decode([]byte(`{"project_id":"one"}`), &graph); err != nil || graph.ProjectID != "one" {
				t.Errorf("concurrent decode: %+v %v", graph, err)
			}
			graph.ProjectID = "caller mutation"
		}()
	}
	workers.Wait()
	if calls.Load() != 1 {
		t.Fatal("identical concurrent decoding not shared")
	}
}

func TestCoordinateCachePreservesNonzeroAndOtherDestinationSemantics(t *testing.T) {
	c, _ := countingCoordinates()
	graph := graphCoordinates{SrcRoot: "already set"}
	if err := c.decode([]byte(`{"project_id":"one"}`), &graph); err != nil || graph.SrcRoot != "already set" {
		t.Fatal("Unmarshal merge behavior changed")
	}
	var generic map[string]any
	if err := c.decode([]byte(`{"project_id":"one"}`), &generic); err != nil || generic["project_id"] != "one" {
		t.Fatal("generic decoding changed")
	}
	if err := c.decode([]byte(`{}`), (*graphCoordinates)(nil)); err == nil {
		t.Fatal("nil destination accepted")
	}
}
