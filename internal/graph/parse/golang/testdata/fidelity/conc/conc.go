// Package conc holds channel operations inside a go statement.
package conc

// Hub owns two channels.
type Hub struct {
	events chan int
	done   chan struct{}
}

// Run sends and receives on fields inside a goroutine literal.
func (h *Hub) Run() {
	go func() {
		h.events <- 1
		<-h.done
	}()
}
