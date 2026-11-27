package types

// Sub kinds of statement nodes that say how code runs (schema 1.44).
const (
	// SubKindSelect is the sub kind of a SwitchStmt node of a select
	// statement without a default clause: it blocks until one case can
	// run. Its cases are what it encloses with kind cond: the send or
	// receive site of each case (and the calls evaluated there).
	SubKindSelect = "select"
	// SubKindSelectDefault is the sub kind of a SwitchStmt node of a
	// select statement with a default clause: it never blocks.
	SubKindSelectDefault = "select_default"
	// SubKindClose is the sub kind of the CallSite of close(ch).
	SubKindClose = "close"
)

// SyncOp is the dispatch kind of an operates_on edge: what the call (or the
// receive) at a CallSite does to the sync primitive the edge ends at.
type SyncOp string

// The operations, by primitive. A primitive is named by its declaration:
// the field, package-level variable or local variable that holds it.
const (
	// sync.WaitGroup: Add, Done, Wait, Go.
	SyncWaitGroupAdd  SyncOp = "wg_add"
	SyncWaitGroupDone SyncOp = "wg_done"
	SyncWaitGroupWait SyncOp = "wg_wait"
	SyncWaitGroupGo   SyncOp = "wg_go"
	// sync.Once: Do runs its function once (runs_func names it).
	SyncOnceDo SyncOp = "once_do"
	// sync.Cond: Wait, Signal, Broadcast.
	SyncCondWait      SyncOp = "cond_wait"
	SyncCondSignal    SyncOp = "cond_signal"
	SyncCondBroadcast SyncOp = "cond_broadcast"
	// sync/atomic: a function on the address of a field or variable
	// (atomic.AddInt64(&s.n, 1)) or a method of an atomic type (s.n.Add(1)).
	// The access is atomic.
	SyncAtomicLoad  SyncOp = "atomic_load"
	SyncAtomicStore SyncOp = "atomic_store"
	SyncAtomicAdd   SyncOp = "atomic_add"
	SyncAtomicSwap  SyncOp = "atomic_swap"
	SyncAtomicCAS   SyncOp = "atomic_cas"
	SyncAtomicAnd   SyncOp = "atomic_and"
	SyncAtomicOr    SyncOp = "atomic_or"
	// context: the call of WithCancel, WithTimeout, WithDeadline (and their
	// Cause forms) makes a context (ctx_make, to the variable it is
	// assigned to) and its cancel function (cancel_make); a call of a
	// cancel function cancels (cancel); ctx.Done() returns the channel the
	// cancel closes (ctx_done, at the call; done_recv at a receive written
	// on that call, <-ctx.Done()).
	SyncContextMake SyncOp = "ctx_make"
	SyncCancelMake  SyncOp = "cancel_make"
	SyncCancel      SyncOp = "cancel"
	SyncContextDone SyncOp = "ctx_done"
	SyncDoneRecv    SyncOp = "done_recv"
	// time: AfterFunc, NewTimer and NewTicker make a timer (timer_make;
	// AfterFunc's function runs later, runs_func); Stop and Reset; a
	// receive on a timer's channel (t.C, time.After, time.Tick) is woken by
	// the runtime, not by a send in the build (timer_recv); for
	// <-time.After(d) the holder is the site of the time.After call, which
	// makes the timer.
	SyncTimerMake  SyncOp = "timer_make"
	SyncTimerStop  SyncOp = "timer_stop"
	SyncTimerReset SyncOp = "timer_reset"
	SyncTimerRecv  SyncOp = "timer_recv"
)

// AllSyncOps returns the operations of operates_on edges.
func AllSyncOps() []SyncOp {
	return []SyncOp{SyncWaitGroupAdd, SyncWaitGroupDone, SyncWaitGroupWait, SyncWaitGroupGo,
		SyncOnceDo, SyncCondWait, SyncCondSignal, SyncCondBroadcast,
		SyncAtomicLoad, SyncAtomicStore, SyncAtomicAdd, SyncAtomicSwap, SyncAtomicCAS, SyncAtomicAnd, SyncAtomicOr,
		SyncContextMake, SyncCancelMake, SyncCancel, SyncContextDone, SyncDoneRecv,
		SyncTimerMake, SyncTimerStop, SyncTimerReset, SyncTimerRecv}
}

// SyncUnknownReason says why the primitive of an operation is not named.
type SyncUnknownReason string

const (
	// SyncNoOrigin: the value comes from something no declaration holds
	// (a value a function returns, a map or slice element), not through a
	// parameter.
	SyncNoOrigin SyncUnknownReason = "no_origin"
	// SyncIncompleteCallers: the primitive is a parameter and the value
	// some caller passes has no declaration (CallersResolved of
	// CallersTotal callers resolve).
	SyncIncompleteCallers SyncUnknownReason = "incomplete_callers"
	// SyncOpenCallers: the primitive is a parameter of a function code
	// outside the graph can call (an exported one), or of one no call in
	// the graph reaches; the callers the graph has are not all of them.
	SyncOpenCallers SyncUnknownReason = "open_callers"
	// SyncNoHolder: a context or timer made at the site is stored in no
	// declaration with a node (only in a parameter variable, or nowhere).
	SyncNoHolder SyncUnknownReason = "no_holder"
	// SyncNotFollowed: the operation is on a value the value flow does not
	// follow (a cancel function held in a parameter, code without an SSA
	// body).
	SyncNotFollowed SyncUnknownReason = "not_followed"
)

// SyncUnknown is one operation at a CallSite whose primitive the graph does
// not name (Node.SyncUnknown): what the call or receive does, and why its
// primitive is not known. CallersResolved and CallersTotal count the direct
// callers of the parameter for the caller reasons (open_callers may have 0
// of 0), and are 0 otherwise.
type SyncUnknown struct {
	Op              SyncOp            `json:"op"`
	Reason          SyncUnknownReason `json:"reason"`
	CallersResolved int               `json:"callers_resolved,omitempty"`
	CallersTotal    int               `json:"callers_total,omitempty"`
}

// SyncAfterFunc is the dispatch kind of the runs_func edge of
// time.AfterFunc: the function runs when the timer fires, in a goroutine of
// its own.
const SyncAfterFunc SyncOp = "after_func"

// RunsFuncKinds returns the dispatch kinds of a runs_func edge: the call
// that runs the function later.
func RunsFuncKinds() []SyncOp { return []SyncOp{SyncOnceDo, SyncAfterFunc, SyncWaitGroupGo} }

// DispatchReflect is the dispatch kind of a sends_to or recvs_from edge
// of an operation done through package reflect (schema 1.45): a
// reflect.Select, Value.Send/TrySend or Value.Recv/TryRecv on cases or
// values loaded from a field that holds reflect.Value or
// reflect.SelectCase, on every channel a reflect.ValueOf stored into that
// field can be (the publish/subscribe pattern of event.Feed). The holder is
// the field, so every channel subscribed to any value of the type is one a
// send on any value of it reaches; triggers from these edges follow the
// usual rule (one edge per make site both sides can be, "~aliasing" when
// several).
const DispatchReflect = "reflect"

// IsDispatchMarker reports whether e is the marker of a dynamic call: an
// invokes edge from a node to itself that carries the dispatch kind (schema
// 1.39). It names no callee and no dependency.
func IsDispatchMarker(e Edge) bool {
	return e.Type == EdgeInvokes && e.Src == e.Dst && e.DispatchKind != ""
}

// IsSiteFact reports whether t is an execution-order fact that starts at a
// statement site (runs_under_lock, operates_on, runs_func, and the control
// transfer jumps_to of schema 1.45): it says how a statement runs, not how
// declarations depend on each other.
func IsSiteFact(t EdgeType) bool {
	switch t {
	case EdgeRunsUnderLock, EdgeOperatesOn, EdgeRunsFunc, EdgeJumpsTo:
		return true
	}
	return false
}
