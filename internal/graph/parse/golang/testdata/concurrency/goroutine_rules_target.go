package mutex_fixture

// reply sends in another file than the go statement that runs loop.
func (w *Worker) reply(v int) {
	w.inbox <- v
}
