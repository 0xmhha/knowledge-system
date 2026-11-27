package golang

import (
	"go/constant"
	"go/token"
	gotypes "go/types"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// chanlock.go finds the channels used as locks (a mutex of capacity one
// that holds a token when unlocked) and the operations that take and return
// the token, for the lock analysis (LockFacts) to treat like Lock and Unlock.

// ChanLockSubKind is the sub kind of a Channel node used as a lock.
const ChanLockSubKind = types.ChannelLockSubKind

// ChanLockOpKind says what a channel operation does to the lock.
type ChanLockOpKind int

const (
	// ChanLockAcquire takes the token: a receive (<-ch, _, ok := <-ch, a
	// select case receiving it).
	ChanLockAcquire ChanLockOpKind = iota + 1
	// ChanLockRelease returns the token: a send.
	ChanLockRelease
	// ChanLockClose closes the lock: close(ch).
	ChanLockClose
)

// ChanLockOp is an operation on a channel lock: which lock (its Channel
// node, the make site), what the operation does, and the fields and
// variables that hold the lock (Holders, by node id; shared by the lock's
// operations).
type ChanLockOp struct {
	Lock    *types.Node
	Kind    ChanLockOpKind
	Holders map[string]bool
}

// ChanLocks are the channel lock operations by the position of the
// operation: the arrow of a receive or a send, the opening parenthesis of
// close.
type ChanLocks map[token.Pos]ChanLockOp

// holds reports whether field is a holder of the channel lock mu: a lock is
// never held over the field that holds it.
func (cl ChanLocks) holds(mu, field *types.Node) bool {
	for _, op := range cl {
		if op.Lock == mu {
			return op.Holders[field.ID]
		}
	}
	return false
}

// ChanLockStats counts what the channel lock rule saw.
type ChanLockStats struct {
	Locks      int // channels the rule accepts as locks
	Semaphores int // the same shape with a constant capacity above one, filled where made: left channels
	Counting   int // the same shape with a constant capacity above one, not filled where made: counting semaphores, left channels
	Inverted   int // locks of the inverted rule (a send takes the token)
	Sized      int // empty-struct channels whose capacity is not a constant (a semaphore of n): left channels
	NotFilled  int // capacity one, empty-struct, not filled where made: left channels
}

// isChanLockShape reports whether make m makes a channel of empty structs
// (it carries no data), and its capacity: -1 when it is not a constant.
func isChanLockShape(m *ssa.MakeChan) (capacity int64, ok bool) {
	ch, isChan := m.Type().Underlying().(*gotypes.Chan)
	if !isChan {
		return 0, false
	}
	st, isStruct := ch.Elem().Underlying().(*gotypes.Struct)
	if !isStruct || st.NumFields() != 0 {
		return 0, false
	}
	k, isConst := m.Size.(*ssa.Const)
	if !isConst || k.Value == nil || k.Value.Kind() != constant.Int {
		return -1, true // a capacity known only at run time
	}
	n, exact := constant.Int64Val(k.Value)
	return n, exact
}

// channelLocks applies the one rule for a channel lock: a channel whose
// element carries no data (struct{}), whose buffer is one, and that is
// filled right where it is made: the block of the make sends on it (by any
// of its names, the channel aliases) before anything there receives from
// it, selects on it or passes it on (filledAfter). The channel is then a mutex that holds its token
// when unlocked: a receive takes it, a send returns it, close closes it. A
// channel of the same shape with a larger buffer is a semaphore and stays a
// channel; so does one not filled first where it is made, unless the second
// rule (invertedLock) accepts it as a lock taken by a send.
func (c *chanLinker) channelLocks(funcs map[*ssa.Function]bool) (ChanLocks, ChanLockStats) {
	var st ChanLockStats
	channel := c.channelIdentity()
	fns := make([]*ssa.Function, 0, len(funcs))
	for fn := range funcs {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].String() < fns[j].String() })
	locks := map[*types.Node]bool{}
	inverted := map[*types.Node]bool{}
	for _, fn := range fns {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				m, ok := in.(*ssa.MakeChan)
				if !ok {
					continue
				}
				capacity, ok := isChanLockShape(m)
				if !ok || capacity == 0 {
					continue
				}
				if capacity < 0 {
					st.Sized++
					continue
				}
				site := c.makeSite(m)
				if site == nil {
					continue
				}
				id := channel(site)
				filled := c.filledAfter(m, id, channel)
				switch {
				case filled && capacity == 1:
					locks[id] = true
				case filled:
					st.Semaphores++
				case capacity == 1 && c.invertedLock(fn, site, id, channel):
					locks[id], inverted[id] = true, true
				case capacity == 1:
					st.NotFilled++
				default:
					st.Counting++
				}
			}
		}
	}
	st.Locks, st.Inverted = len(locks), len(inverted)
	// The fields and variables that hold a lock (its channel aliases).
	holders := map[*types.Node]map[string]bool{}
	for id := range locks {
		holders[id] = map[string]bool{}
	}
	for _, n := range c.aliasNodes {
		if id := channel(n); locks[id] && n.Type != types.NodeChannel {
			holders[id][n.ID] = true
		}
	}
	ops := ChanLocks{}
	for _, ev := range c.events {
		for _, o := range ev.origins {
			id := channel(o.node)
			if !locks[id] {
				continue
			}
			// A receive takes the token and a send returns it; an inverted
			// lock is taken by a send and returned by a receive.
			kind := ChanLockAcquire
			if (ev.typ == types.EdgeSendsTo) != inverted[id] {
				kind = ChanLockRelease
			}
			ops[ev.pos] = ChanLockOp{Lock: id, Kind: kind, Holders: holders[id]}
		}
	}
	for _, fn := range fns {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				call, ok := in.(*ssa.Call)
				if !ok {
					continue
				}
				if bi, ok := call.Call.Value.(*ssa.Builtin); !ok || bi.Name() != "close" || len(call.Call.Args) != 1 {
					continue
				}
				for _, o := range c.origins(call.Call.Args[0], 0, map[ssa.Value]bool{}) {
					if id := channel(o.node); locks[id] {
						ops[call.Pos()] = ChanLockOp{Lock: id, Kind: ChanLockClose, Holders: holders[id]}
					}
				}
			}
		}
	}
	for id := range locks {
		id.SubKind = ChanLockSubKind
	}
	return ops, st
}

// invertedLock applies the second, inverted rule to the empty channel of
// make site site (identity id) made in fn: it is a lock taken by a send when
// it never escapes (no other name, not passed to a call), every operation on
// it is in fn, every send is a case of a select with a default (a
// try-acquire: txpool's resetBusy) and every receive a plain receive (the
// release), with at least one of each.
func (c *chanLinker) invertedLock(fn *ssa.Function, site, id *types.Node, channel func(*types.Node) *types.Node) bool {
	for k := range c.aliases {
		if k[0] == site.ID || k[1] == site.ID {
			return false
		}
	}
	is := func(v ssa.Value) bool {
		for _, o := range c.origins(v, 0, map[ssa.Value]bool{}) {
			if channel(o.node) == id {
				return true
			}
		}
		return false
	}
	// The same source function built twice (a package and its test
	// variant) is one function here.
	where := c.prog.Fset.Position(fn.Pos())
	for _, ev := range c.events {
		if ev.fn == fn || c.prog.Fset.Position(ev.fn.Pos()) == where {
			continue
		}
		for _, o := range ev.origins {
			if channel(o.node) == id {
				return false
			}
		}
	}
	tries, releases := 0, 0
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			switch x := in.(type) {
			case *ssa.Send:
				if is(x.Chan) {
					return false
				}
			case *ssa.UnOp:
				if x.Op == token.ARROW && is(x.X) {
					if x.CommaOk {
						return false
					}
					releases++
				}
			case *ssa.Select:
				for _, st := range x.States {
					if !is(st.Chan) {
						continue
					}
					if st.Dir != gotypes.SendOnly || x.Blocking {
						return false
					}
					tries++
				}
			case ssa.CallInstruction:
				for _, a := range x.Common().Args {
					if is(a) {
						return false
					}
				}
			}
		}
	}
	return tries > 0 && releases > 0
}

// filledAfter reports whether the block of make m sends on the channel id
// after m, before anything else in that block receives from it, selects on
// it or passes it to a call: the token is put in right where the lock is
// made. A send later on (in a loop, a select) is not a fill.
func (c *chanLinker) filledAfter(m *ssa.MakeChan, id *types.Node, channel func(*types.Node) *types.Node) bool {
	is := func(v ssa.Value) bool {
		for _, o := range c.origins(v, 0, map[ssa.Value]bool{}) {
			if channel(o.node) == id {
				return true
			}
		}
		return false
	}
	after := false
	for _, in := range m.Block().Instrs {
		if in == m {
			after = true
			continue
		}
		if !after {
			continue
		}
		switch x := in.(type) {
		case *ssa.Send:
			if is(x.Chan) {
				return true
			}
		case *ssa.UnOp:
			if x.Op == token.ARROW && is(x.X) {
				return false
			}
		case *ssa.Select:
			for _, st := range x.States {
				if is(st.Chan) {
					return false
				}
			}
		case ssa.CallInstruction:
			for _, a := range x.Common().Args {
				if is(a) {
					return false
				}
			}
		}
	}
	return false
}
