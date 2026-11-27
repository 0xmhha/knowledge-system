// Package execflow answers, in one call, what happens in execution order
// starting from a function (forward) and what leads to it (backward),
// across function calls, goroutine starts and channel messages, with the
// locks held, as a short numbered narrative of source lines. It backs the
// execution_flow tool and the ckg flow command, which print the same text.
//
// The reader is a model that reads text front to back and loses the middle
// of long inputs, so the answer is assembled for it and kept short: a
// summary of two to four lines, then numbered steps, then the exact calls
// that continue where the answer was cut.
//
// # The rules
//
// Steps. A function's steps are what its graph edges say it does, one step
// per edge and line: calls (calls, an interface call with the code it may
// run, a call through a function value with the functions it may hold),
// goroutine starts (spawns), channel sends and receives, lock acquires and
// releases, and, when focus names a field or variable, reads and writes of
// it. A goroutine's steps are its code edges on the lines of its literal
// (or the call of a named function on its go line); the enclosing function
// leaves those lines to it. A call into the build is opened into the
// callee's steps, a goroutine start into the goroutine's, a send into the
// receives it can wake (triggers) and the code each receive runs (handles).
// A receive's handled code is nested under it; the cases of a select (the
// graph's select statement and the sites it encloses as cases) are
// alternatives of one select step, which says when it has a default.
// Calls into sync, sync/atomic, time and context are said as what they do
// to which primitive (operates_on): a wait on a WaitGroup names its Add and
// Done sites, a receive on a timer is sent by the runtime, a receive on a
// context's done channel names where its cancel is called, and a function
// passed to sync.Once.Do, time.AfterFunc or WaitGroup.Go is a step that
// runs later (runs_func). A literal that is deferred, passed, stored or
// returned holds its code under one step, with what the function it is
// passed to does with it (literal_passed_to: call, go, defer, store,
// pass); its extent, the defer statements, the case clauses and where
// each return, break, continue and goto goes are the graph's statement
// nodes and jumps_to edges. The source text is read only to quote a
// step's line.
//
// Order. Inside a function steps are in source-line order, and on one line
// in the order receive, acquire, call, go, send, read, write, release; then
// by printed name and node id. A nested step runs inside the step above it.
//
// Branches. Alternatives (the implementations an interface call may run,
// the functions a function value may hold, the receives one send can wake,
// the ways that lead backward to a step) are ordered in-build before
// outside the build, production before test, then by the graph's rank
// (PageRank) and printed name, and cut to Query.Branches, with the count
// left out and the call that lists them all.
//
// Locks. A step with a site runs under the locks the graph's must-hold
// analysis records at that site (runs_under_lock; "held by a caller" when
// propagated), and under those held where the step that opened its
// function runs. A step without a site falls back to line order (acquired
// on an earlier line, not released in between, a deferred release holding
// to the end, a release on a branch that returns not ending the lock after
// it), marked approximated (branches). A goroutine, a literal that runs
// later and the code a message runs start with nothing held.
//
// Certainty. Every approximated fact says so, with the reason the
// concurrency contract gives it (internal/graph/concfacts): aliasing,
// branches or interleaving; a dispatch target found by type alone says "by
// type".
//
// Stops. A step is not opened past Query.Depth hops, past the budget, when
// its function is already on the path (recursion), when it leaves the
// build (an External stub), or when it is test code and tests are left out
// (the graph's is_test). The summary counts each.
//
// Budget. Steps are chosen breadth first: the seed's own steps before the
// steps inside them, so a cut drops the deepest steps first.
//
// Determinism. Every list is ordered by a total order (above), so the same
// query on the same graph gives the same bytes.
package execflow
