package locks

import "sync"

// Bump locks a mutex field that store.go declares.
func (s *Store) Bump() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
}

// Scan read-locks the package-level mutex that store.go declares.
func Scan() {
	registry.RLock()
	registry.RUnlock()
}

// guard returns the store's mutex.
func (s *Store) guard() *sync.Mutex { return &s.mu }

// Reset locks the mutex through the method that returns it.
func (s *Store) Reset() {
	s.guard().Lock()
	defer s.guard().Unlock()
	s.n = 0
}

// Write locks the second mutex of a field that declares two.
func (p *Pipe) Write() {
	p.wmu.Lock()
	p.wmu.Unlock()
}

// Guard locks a mutex made with new.
func Guard() {
	made := new(sync.Mutex)
	made.Lock()
	made.Unlock()
}
