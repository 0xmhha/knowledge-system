package mutex_fixture

import "context"

// xferErrs returns a channel filled with n errors.
func xferErrs(n int) chan error {
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		errs <- nil
	}
	return errs
}

// xferVerify returns xferErrs's channel or one a goroutine fills: the
// receive in XferWait can be on either, each send only on its own.
func xferVerify(n int) <-chan error {
	if n == 0 {
		return xferErrs(1)
	}
	results := make(chan error, n)
	go func() { results <- nil }()
	return results
}

// XferWait receives from what xferVerify returns.
func XferWait() { <-xferVerify(2) }

// xferArm stores a context into a field declared in another file.
func xferArm(h *xferHolder) {
	ctx, cancel := context.WithCancel(context.Background())
	h.c = ctx
	cancel()
}

// xferSync keeps a cancel channel; xferRunSync stores the channel it is
// passed into an xferReq. The test-only caller passes another channel:
// the two must not become one channel through the shared parameter.
type xferSync struct{ cancel chan struct{} }

type xferReq struct{ cancel chan struct{} }

func xferRunSync(cancel chan struct{}) {
	r := &xferReq{cancel: cancel}
	<-r.cancel
}

func newXferSync() *xferSync {
	s := &xferSync{cancel: make(chan struct{})}
	go xferRunSync(s.cancel)
	return s
}

func (s *xferSync) Cancel() { close(s.cancel) }

func xferOtherSync() {
	c := make(chan struct{})
	go xferRunSync(c)
	c <- struct{}{}
}

// xferDone closes the channel it is passed; XferLoop passes one of two.
func xferDone(done chan struct{}) { close(done) }

func XferLoop(n int) {
	d := make(chan struct{})
	if n > 0 {
		d = make(chan struct{})
	}
	go xferDone(d)
	<-d
}

// xferReqs carries channels: XferAsk sends one, xferServe closes it.
var xferReqs = make(chan chan struct{})

func xferServe() {
	ch := <-xferReqs
	close(ch)
}

func XferAsk() {
	ch := make(chan struct{})
	xferReqs <- ch
	<-ch
}
