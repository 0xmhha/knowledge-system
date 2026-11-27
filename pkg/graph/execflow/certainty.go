package execflow

import (
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// certainty.go says how sure a step is. The graph stores a fact's
// confidence (EXTRACTED for proved, INFERRED for approximated) but not the
// reason; the reason is the one the concurrency contract gives each fact
// type, read here from the edge that holds the fact.

// factReasons is the reason of an approximated fact by the edge type that
// holds it (concfacts: ChanOp and ChanAlias are aliasing, Received is
// branches, Handles and Trigger are interleaving, a lock of a channel used
// as a lock and UnderLock are aliasing).
var factReasons = map[types.EdgeType]concfacts.Reason{
	types.EdgeSendsTo:           concfacts.ReasonAliasing,
	types.EdgeRecvsFrom:         concfacts.ReasonAliasing,
	types.EdgeAcquiresLock:      concfacts.ReasonAliasing,
	types.EdgeReleasesLock:      concfacts.ReasonAliasing,
	types.EdgeAccessedUnderLock: concfacts.ReasonAliasing,
	types.EdgeHandles:           concfacts.ReasonInterleaving,
	types.EdgeTriggers:          concfacts.ReasonInterleaving,
}

// certaintyOf is the certainty of the fact edge e holds: proved when the
// graph extracted it, else approximated for the reason of its fact type. A
// goroutine's code edge (an ExecContext) is interleaving; a value that a
// receive passes on (flows_to received) is branches.
func certaintyOf(e types.Edge) concfacts.Certainty {
	if e.Confidence != types.ConfInferred {
		return concfacts.Proof
	}
	switch {
	case e.Type == types.EdgeTriggers && strings.HasSuffix(e.DispatchKind, triggerAliasMark):
		// The send and the receive may be on several channels: the pair
		// is told apart by declaration.
		return concfacts.Approx(concfacts.ReasonAliasing)
	case types.IsGoroutineCode(e):
		return concfacts.Approx(concfacts.ReasonInterleaving)
	case e.Type == types.EdgeFlowsTo && e.DispatchKind == concfacts.ReceivedKind:
		return concfacts.Approx(concfacts.ReasonBranches)
	case e.Type == types.EdgeFlowsTo:
		return concfacts.Approx(concfacts.ReasonAliasing)
	}
	if r, ok := factReasons[e.Type]; ok {
		return concfacts.Approx(r)
	}
	// A dispatch target found by value flow: values are told apart by
	// where they are made.
	return concfacts.Approx(concfacts.ReasonAliasing)
}

// triggerAliasMark ends the dispatch kind of a triggers edge whose send and
// receive may be on several channels.
const triggerAliasMark = "~" + string(concfacts.ReasonAliasing)

// triggerChannel is the channel a triggers edge names, without its mark.
func triggerChannel(e types.Edge) string { return strings.TrimSuffix(e.DispatchKind, triggerAliasMark) }

// lockContextCertainty is the certainty of a lock context read by line
// order: it does not see branches.
var lockContextCertainty = concfacts.Approx(concfacts.ReasonBranches)

// byTypeMark is how a dispatch target the type hierarchy alone allows is
// marked, as the other tools mark it.
const byTypeMark = "by type"

// certaintyMark is the mark a step prints for c: nothing for proved, else
// "~" and the reason.
func certaintyMark(c concfacts.Certainty) string {
	if c.Kind != concfacts.Approximated {
		return ""
	}
	return approxPrefix + string(c.Reason)
}

// approxPrefix starts the mark of an approximated fact.
const approxPrefix = "~"
