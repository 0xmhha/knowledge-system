package execflow

import (
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// stdfacts.go turns a call into the standard library's sync, sync/atomic
// and time packages into what it does for concurrency (waits on a
// WaitGroup, atomically loads, sleeps, arms a timer). Other calls outside
// the build are left out of an answer and counted.

// Standard library packages whose calls are facts, by import path.
const (
	pkgSync   = "sync"
	pkgAtomic = "sync/atomic"
	pkgTime   = "time"
)

// syncWords and timeWords say what a call of "<Type>.<Func>" (or "<Func>")
// does; atomicWords by the prefix of the function name.
var (
	syncWords = map[string]string{
		"WaitGroup.Wait": "waits for the WaitGroup", "WaitGroup.Add": "adds to the WaitGroup",
		"WaitGroup.Done": "marks the WaitGroup done", "WaitGroup.Go": "starts a goroutine of the WaitGroup",
		"Once.Do": "calls its function at most once", "Cond.Wait": "waits on the condition", "Cond.Signal": "wakes one waiter",
		"Cond.Broadcast": "wakes every waiter",
		"Mutex.Lock":     "locks", "Mutex.Unlock": "unlocks", "Mutex.TryLock": "tries to lock",
		"RWMutex.Lock": "locks", "RWMutex.Unlock": "unlocks", "RWMutex.RLock": "read-locks",
		"RWMutex.RUnlock": "read-unlocks", "RWMutex.TryLock": "tries to lock",
	}
	timeWords = map[string]string{
		"Sleep": "sleeps", "After": "waits on a timer channel", "AfterFunc": "arms a timer that calls a function",
		"NewTimer": "arms a timer", "NewTicker": "starts a ticker", "Tick": "starts a ticker",
		"Timer.Reset": "re-arms a timer", "Timer.Stop": "stops a timer", "Ticker.Reset": "re-arms a ticker",
		"Ticker.Stop": "stops a ticker",
	}
	atomicWords = []struct{ prefix, words string }{
		{"CompareAndSwap", "atomically compares and swaps"}, {"Load", "atomically loads"},
		{"Store", "atomically stores"}, {"Add", "atomically adds"}, {"Swap", "atomically swaps"},
		{"And", "atomically clears bits"}, {"Or", "atomically sets bits"},
	}
)

// stdFact says what a call of the External stub n does, when n is one of
// the sync, sync/atomic or time functions above.
func stdFact(n types.Node) (string, bool) {
	pkg, member, ok := splitCanonical(n.CanonicalID)
	if !ok {
		return "", false
	}
	switch pkg {
	case pkgSync:
		w, found := syncWords[member]
		if !found && strings.HasPrefix(member, "Map.") {
			return "uses a sync.Map", true
		}
		return w, found
	case pkgTime:
		w, found := timeWords[member]
		return w, found
	case pkgAtomic:
		fn := member[strings.LastIndex(member, ".")+1:]
		for _, a := range atomicWords {
			if strings.HasPrefix(fn, a.prefix) {
				return a.words, true
			}
		}
	}
	return "", false
}

// splitCanonical splits a canonical id ("sync.(*WaitGroup).Wait",
// "sync/atomic.AddInt64", "time.(Time).Add") into its import path and the
// member "<Type>.<Func>" or "<Func>".
func splitCanonical(id string) (pkg, member string, ok bool) {
	head := id
	if i := strings.Index(id, "("); i >= 0 {
		head = id[:i]
	}
	slash := strings.LastIndex(head, "/")
	dot := strings.Index(id[slash+1:], ".")
	if dot < 0 {
		return "", "", false
	}
	dot += slash + 1
	member = strings.NewReplacer("(", "", ")", "", "*", "").Replace(id[dot+1:])
	return id[:dot], member, member != ""
}
