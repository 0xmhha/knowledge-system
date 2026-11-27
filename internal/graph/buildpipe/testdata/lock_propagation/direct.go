package lockprop

import "sync"

// Direct uses n while it holds mu in the same function: the intra-function
// pass records accessed_under_lock(Direct.n -> Direct.mu) without lock
// propagation.
type Direct struct {
	mu sync.Mutex
	n  int
}

// Inc locks mu around the use of n.
func (d *Direct) Inc() {
	d.mu.Lock()
	d.n++
	d.mu.Unlock()
}
