package mutex_fixture

import (
	"context"
	"sync"
)

type xferPeer struct{ ok bool }

// XferGossip skips a failed peer: the send after the if does not run then.
func XferGossip(peers []xferPeer, send func(xferPeer)) {
	for _, p := range peers {
		if !p.ok {
			continue
		}
		send(p)
	}
}

// XferPick leaves a labeled loop, falls through, returns in a clause.
func XferPick(xs []int) int {
outer:
	for _, x := range xs {
		switch {
		case x < 0:
			break outer
		case x == 0:
			fallthrough
		case x == 1:
			return x
		default:
			continue
		}
	}
	return -1
}

func xferRun(f func())  { f() }
func xferGo(f func())   { go f() }
func xferKeep(f func()) { xferKept = f }
func xferMaybe(f func(), ok bool) {
	if ok {
		f()
	}
}

var xferKept func()

// XferSchedule passes literals to callees that treat them differently.
func XferSchedule(mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()
	xferRun(func() { xferNote() })
	xferGo(func() { xferNote() })
	xferKeep(func() { xferNote() })
	xferMaybe(func() { xferNote() }, true)
	seven := func() int { return 7 }
	_ = seven()
}

func xferNote() {}

// xferSrv keeps a cancel in a field and calls it through the field.
type xferSrv struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newXferSrv() *xferSrv {
	ctx, cancel := context.WithCancel(context.Background())
	s := &xferSrv{ctx: ctx}
	s.cancel = cancel
	return s
}

func (s *xferSrv) Close() { s.cancel() }

func xferChans() (chan int, chan int) { return make(chan int), make(chan int) }

// xferSpawn runs the literal it is passed on a goroutine of its own.
func xferSpawn(f func()) { go func() { f() }() }

// xferQueue hands the literal it is passed to another goroutine, if one
// is ready.
func xferQueue(ops chan func(), f func()) {
	select {
	case ops <- f:
	default:
	}
}

// XferUses passes literals on a goroutine, over a channel and through a
// variable a nested literal captures.
func XferUses(ops chan func()) {
	xferSpawn(func() { xferNote() })
	xferQueue(ops, func() { xferNote() })
	run := func() { xferNote() }
	nested := func() { xferRun(run) }
	nested()
}

type xferBox[T any] struct{ v T }

func (b *xferBox[T]) each(f func(T)) { f(b.v) }

// XferGeneric passes a literal to a method of a generic type.
func XferGeneric() {
	b := &xferBox[int]{}
	b.each(func(int) { xferNote() })
}

// xferHolder keeps a context; trigger_names.go stores into it.
type xferHolder struct{ c context.Context }
