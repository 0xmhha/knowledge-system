package mutex_fixture

import (
	"context"
	"reflect"
)

// xferFeed delivers to subscribed channels by reflection, as event.Feed.
type xferFeed struct {
	inbox []reflect.SelectCase
	cases []reflect.SelectCase
}

func (f *xferFeed) Subscribe(ch interface{}) {
	cas := reflect.SelectCase{Dir: reflect.SelectSend, Chan: reflect.ValueOf(ch)}
	f.inbox = append(f.inbox, cas)
}

func (f *xferFeed) Send(v interface{}) {
	f.cases = append(f.cases, f.inbox...)
	rv := reflect.ValueOf(v)
	for i := range f.cases {
		f.cases[i].Send = rv
	}
	reflect.Select(f.cases)
}

// XferListen subscribes a channel and receives from it.
func XferListen(f *xferFeed) {
	events := make(chan int, 1)
	f.Subscribe(events)
	<-events
}

// xferWatch keeps a context's Done channel and its cancel function in
// fields and uses both through them.
type xferWatch struct {
	done   <-chan struct{}
	cancel func()
}

func newXferWatch() *xferWatch {
	ctx, cancel := context.WithCancel(context.Background())
	return &xferWatch{done: ctx.Done(), cancel: cancel}
}

func (w *xferWatch) Wait() { <-w.done }

func (w *xferWatch) Stop() { w.cancel() }

// xferPick receives a channel from a carrier inside a select.
var xferPicks = make(chan chan int)

func xferPick() {
	select {
	case ch := <-xferPicks:
		ch <- 1
	default:
	}
}

func XferOffer() {
	ch := make(chan int)
	xferPicks <- ch
	<-ch
}
