// Package buildpipe — lock_propagation.go: cross-function lock propagation
// (Options.LockPropagation, CLI --lock-propagation) is part of the Go lock
// pass (gop.LockFacts with gop.LockOptions.Propagate): a field used in a
// function called while a mutex is held counts as under the mutex when the
// must-hold rule says the mutex is held at the call and, in the callee, still
// held at the use.
//
// The former propagator walked the call graph from every function that
// locks a mutex anywhere and recorded every field of every callee up to five
// calls deep, whether the call ran under the lock or not, through go
// statements too: 667 of 1,761 accessed_under_lock edges of go-stablenet
// were not supported by the source even without it.
package buildpipe

import gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"

// PropagatedLockKind is the dispatch kind of a propagated accessed_under_lock
// edge (one found in a callee); a direct edge has none.
const PropagatedLockKind = gop.PropagatedLockKind
