// Package concfacts is the language-independent contract for the
// concurrency facts the graph holds. A language frontend's adapter fills
// Facts from its own analyses (the Go adapter: the SSA channel linker and
// the must-hold lock analysis in internal/graph/parse/golang); the graph
// writer turns them into edges (Edges, LockEdges) and ckg verify names the
// relations it checks with the same vocabulary. The facts are data, not
// algorithms: how a frontend finds them is its own business.
//
// The contract holds what Go produces today:
//
//   - ExecContext: a goroutine (a Goroutine node) and the code it runs.
//   - ChanOp: a send or receive and the origin of its channel (a make site,
//     a field, a package-level variable); ChanAlias: a channel stored under
//     another name.
//   - Handles: the code a received message makes run; Received: where the
//     received value goes; Trigger: a sender and a receive it can wake.
//   - LockOp: an acquire or release of a mutex; UnderLock: a field used
//     where a mutex is held, in the function itself or in a callee.
//
// Every fact states its certainty (Certainty): proved from the code (no
// runtime value is needed to know it) or approximated, with the reason.
//
// What another language's adapter would have to supply, and what may not
// fit:
//
//   - Rust: threads (std::thread, tasks of an async runtime) are execution
//     contexts and channels (mpsc, crossbeam) have the same send, receive
//     and origin shape. Locks are values that own their data (Mutex<T>),
//     so UnderLock would name the protected value, not a field next to the
//     mutex, and the compiler proves the absence of data races: an
//     UnderLock fact would be proved there, not approximated, and no
//     "unguarded access" question arises for safe code.
//   - Solidity: there are no threads, channels or locks. The interleaving
//     concern is re-entrancy: an external call lets the callee run code of
//     the same contract before the caller finishes. That would need facts
//     this contract does not have (an external call as the point where
//     other code may run, and the state read before and written after it);
//     ExecContext, ChanOp and LockOp would stay empty, and a re-entrancy
//     guard modifier is not a LockOp of a mutex value.
//
// Neither adapter exists; the contract is not designed for them.
package concfacts

import "github.com/0xmhha/knowledge-system/pkg/graph/types"

// CertaintyKind says whether a fact follows from the code alone.
type CertaintyKind string

const (
	// Proved: the fact holds for every run; no runtime value is needed.
	Proved CertaintyKind = "proved"
	// Approximated: the fact depends on what the analysis cannot follow
	// exactly (see Reason).
	Approximated CertaintyKind = "approximated"
)

// Reason says why an approximated fact is not proved.
type Reason string

const (
	// ReasonAliasing: values are told apart by where they are declared or
	// made, not by instance; two values of one declaration are one.
	ReasonAliasing Reason = "aliasing"
	// ReasonBranches: the fact holds on some paths, or is read from the
	// value flow over all paths, not one run.
	ReasonBranches Reason = "branches"
	// ReasonInterleaving: when the code runs relative to other code is not
	// known (a goroutine, a message that may or may not arrive).
	ReasonInterleaving Reason = "interleaving"
)

// Certainty is the certainty of a fact and, when approximated, why.
type Certainty struct {
	Kind   CertaintyKind
	Reason Reason
}

// Proof is the certainty of a proved fact.
var Proof = Certainty{Kind: Proved}

// Approx is the certainty of a fact approximated for reason r.
func Approx(r Reason) Certainty { return Certainty{Kind: Approximated, Reason: r} }

// Confidence is the stored confidence of a fact of certainty c: EXTRACTED
// for proved, INFERRED for approximated.
func (c Certainty) Confidence() types.Confidence {
	if c.Kind == Proved {
		return types.ConfExtracted
	}
	return types.ConfInferred
}

// Kinds the graph stores in an edge's dispatch kind.
const (
	// PropagatedKind marks an UnderLock fact found in a callee.
	PropagatedKind = "propagated"
	// AliasKind marks a ChanAlias (flows_to from a channel's origin).
	AliasKind = "chan"
	// ReceivedKind marks a Received fact (flows_to from a receive).
	ReceivedKind = types.DispatchReceived
	// CloseKind marks the LockOp that closes a channel used as a lock.
	CloseKind = "close"
	// GoroutineKind marks the code edges of an ExecContext.
	GoroutineKind = types.DispatchGoroutine
)

// Nodes are named by their graph node ids. File is the file the edge is
// recorded in; Line its line (0: none).

// CodeUse is one thing a goroutine's code does: calls a function, calls an
// interface method (invokes), calls through a function value (dispatches_to),
// reads a field or writes one.
type CodeUse struct {
	Edge   types.EdgeType // calls, invokes, dispatches_to, reads_field, writes_field or references
	Target string
	Line   int
	// Write marks a references use that assigns the variable.
	Write bool
}

// ExecContext is a goroutine and the code it runs.
type ExecContext struct {
	Goroutine string
	File      string
	Runs      []CodeUse
	Certainty Certainty
}

// Direction is a channel operation's direction.
type Direction string

const (
	Send Direction = "send"
	Recv Direction = "recv"
)

// ChanOp is a send or receive by Actor (a function or a goroutine) on the
// channel whose origin is Origin; OriginKind is make, field or global.
type ChanOp struct {
	Actor, Origin string
	OriginKind    string
	Dir           Direction
	Line          int
	File          string
	Certainty     Certainty
}

// ChanAlias is a channel of origin Origin stored in Holder (a field or a
// package-level variable) at Line.
type ChanAlias struct {
	Origin, Holder string
	Line           int
	File           string
	Certainty      Certainty
}

// Handles is the code a message received at Receive (its CallSite) makes
// run: Target (a CallSite, a Goroutine or a Field) and what it does there
// (Kind: call, send, recv, go, read or write).
type Handles struct {
	Receive, Target string
	Kind            string
	Line            int
	File            string
	Certainty       Certainty
}

// Received is a use (Use: a Field, a CallSite) that the value received at
// Receive reaches.
type Received struct {
	Receive, Use string
	Line         int
	File         string
	Certainty    Certainty
}

// Trigger is a sender (a function or a goroutine) whose send at Line can
// wake the receive at Receive; Channel names the channel's origin.
type Trigger struct {
	Sender, Receive string
	Channel         string
	Line            int
	File            string
	Certainty       Certainty
}

// LockOp is an acquire (Acquire) or release of Mutex by Function at Line;
// Close marks the release that closes a channel used as a lock.
type LockOp struct {
	Function, Mutex string
	Acquire, Close  bool
	Line            int
	File            string
	Certainty       Certainty
}

// UnderLock is Field used where Mutex is held: in the function that holds
// it, or (Propagated) in a function called while it is held.
type UnderLock struct {
	Field, Mutex string
	Propagated   bool
	File         string
	Certainty    Certainty
}

// Fact is one of the fact types above.
type Fact interface {
	edges() []types.Edge
}

// Facts are a frontend's concurrency facts, in the order it found them.
type Facts []Fact

func (f ExecContext) edges() []types.Edge {
	out := make([]types.Edge, 0, len(f.Runs))
	for _, u := range f.Runs {
		kind := GoroutineKind
		if u.Write {
			kind = types.DispatchGoroutineWrite
		}
		out = append(out, types.Edge{Src: f.Goroutine, Dst: u.Target, Type: u.Edge, Line: u.Line, Count: 1,
			Confidence: f.Certainty.Confidence(), DispatchKind: kind, FilePath: f.File})
	}
	return out
}

func (f ChanOp) edges() []types.Edge {
	t := types.EdgeSendsTo
	if f.Dir == Recv {
		t = types.EdgeRecvsFrom
	}
	return []types.Edge{{Src: f.Actor, Dst: f.Origin, Type: t, Line: f.Line, Count: 1,
		Confidence: f.Certainty.Confidence(), DispatchKind: f.OriginKind, FilePath: f.File}}
}

func (f ChanAlias) edges() []types.Edge {
	return []types.Edge{{Src: f.Origin, Dst: f.Holder, Type: types.EdgeFlowsTo, Line: f.Line, Count: 1,
		Confidence: f.Certainty.Confidence(), DispatchKind: AliasKind, FilePath: f.File}}
}

func (f Handles) edges() []types.Edge {
	return []types.Edge{{Src: f.Receive, Dst: f.Target, Type: types.EdgeHandles, Line: f.Line, Count: 1,
		Confidence: f.Certainty.Confidence(), DispatchKind: f.Kind, FilePath: f.File}}
}

func (f Received) edges() []types.Edge {
	return []types.Edge{{Src: f.Receive, Dst: f.Use, Type: types.EdgeFlowsTo, Line: f.Line, Count: 1,
		Confidence: f.Certainty.Confidence(), DispatchKind: ReceivedKind, FilePath: f.File}}
}

func (f Trigger) edges() []types.Edge {
	return []types.Edge{{Src: f.Sender, Dst: f.Receive, Type: types.EdgeTriggers, Line: f.Line, Count: 1,
		Confidence: f.Certainty.Confidence(), DispatchKind: f.Channel, FilePath: f.File}}
}

func (f LockOp) edges() []types.Edge {
	t := types.EdgeReleasesLock
	if f.Acquire {
		t = types.EdgeAcquiresLock
	}
	kind := ""
	if f.Close {
		kind = CloseKind
	}
	return []types.Edge{{Src: f.Function, Dst: f.Mutex, Type: t, Line: f.Line, Count: 1,
		Confidence: f.Certainty.Confidence(), DispatchKind: kind, FilePath: f.File}}
}

func (f UnderLock) edges() []types.Edge {
	kind := ""
	if f.Propagated {
		kind = PropagatedKind
	}
	return []types.Edge{{Src: f.Field, Dst: f.Mutex, Type: types.EdgeAccessedUnderLock, Count: 1,
		Confidence: f.Certainty.Confidence(), DispatchKind: kind, FilePath: f.File}}
}

// Edges returns the graph edges of facts, one per fact use, in order.
func (fs Facts) Edges() []types.Edge {
	var out []types.Edge
	for _, f := range fs {
		out = append(out, f.edges()...)
	}
	return out
}

// edgeKey is what makes two edges of facts the same graph edge.
type edgeKey struct {
	src, dst string
	typ      types.EdgeType
	kind     string
	line     int
}

// Counted returns the graph edges of facts with the uses of one edge (the
// same ends, type, kind and line) folded into its Count, in the order of
// their first use: what the graph stores for facts found once per
// operation.
func (fs Facts) Counted() []types.Edge {
	var out []types.Edge
	at := map[edgeKey]int{}
	for _, e := range fs.Edges() {
		k := edgeKey{e.Src, e.Dst, e.Type, e.DispatchKind, e.Line}
		if i, ok := at[k]; ok {
			out[i].Count++
			continue
		}
		at[k] = len(out)
		out = append(out, e)
	}
	return out
}
