package mcphandlers

import (
	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// legend.go explains, in the answer that holds them, the execution-order
// facts of schema 1.44 an answer's nodes and edges carry: one entry per kind
// present, under legendKey, so the meaning reaches the model with the value
// and costs nothing in answers without them.

// Dispatch kinds the legend recognises: flows_to between the holders of one
// sync primitive (the concurrency contract's), and encloses from a switch
// or select to a clause (the Go frontend's statement links write it).
const (
	sharedSyncKind = concfacts.SyncAliasKind
	clauseKind     = "clause"
)

// legendKey is the key of an answer's legend.
const legendKey = "legend"

// edgeLegend explains the edge types, keyed by type.
var edgeLegend = map[types.EdgeType]string{
	types.EdgeCloses: "closes: the function or goroutine closes that channel (its make site, field or variable); " +
		"every receive on it wakes and gets the zero value (ok false); triggers on the same line are the receives it wakes",
	types.EdgeRunsUnderLock: "runs_under_lock: the call, send, receive or close at this site runs while that mutex is held " +
		"(dispatch_kind propagated: a caller holds it); a deferred call runs at the return with what is held then",
	types.EdgeOperatesOn: "operates_on: the call or receive at this site operates on the sync primitive that declaration holds; " +
		"dispatch_kind is the operation: wg_add, wg_done, wg_wait, wg_go, once_do, cond_wait, cond_signal, cond_broadcast, " +
		"atomic_load, atomic_store, atomic_add, atomic_swap, atomic_cas, atomic_and, atomic_or (the access is atomic), " +
		"ctx_make and cancel_make (the context and the cancel function a With* call makes), cancel (a call of the cancel function), " +
		"ctx_done and done_recv (ctx.Done() and a receive on it: woken when the context is cancelled), timer_make, timer_stop, timer_reset, " +
		"timer_recv (a receive the runtime sends to when the timer fires; for <-time.After(d) the edge ends at the call that makes the timer); " +
		"INFERRED when the primitive is a parameter followed to the callers' declarations",
	types.EdgeRunsFunc: "runs_func: the call at this site runs that function later: once (once_do), when a timer fires (after_func) or in a goroutine (wg_go)",
	types.EdgeJumpsTo: "jumps_to: where a statement leaves the normal order to (dispatch_kind): break -> the loop or switch it leaves, " +
		"continue -> the loop it goes on with, goto -> the statement of its label, fallthrough -> the next case clause, return -> the function or literal it returns from",
	types.EdgeLiteralPassedTo: "literal_passed_to: a call in a function literal (or the FuncLit itself, or a go statement in it) -> the function the literal is passed to; " +
		"from a FuncLit, dispatch_kind says what that function does with it, joined by +: call (calls it before it returns), go (starts it in a goroutine), " +
		"defer (runs it as it returns), store (stores or returns it), send (sends it on a channel), pass (passes it on); INFERRED when it does so only on some paths",
	types.EdgeTriggers: "triggers: a sender (or closer) -> a receive it can wake, one edge per channel they may share; " +
		"certain false, reason aliasing: the analysis cannot tell that channel from others made at the same place",
}

// flowsToSyncLegend explains flows_to of kind sync.
const flowsToSyncLegend = "flows_to sync: a sync primitive's holder -> the field or variable it is stored into: operations on either are on one primitive"

// clauseLegend explains encloses of kind clause.
const clauseLegend = "encloses clause: a switch, type switch or select -> one of its case or default clauses (CaseClause), in order"

// syncUnknownLegend explains a site's sync_unknown.
const syncUnknownLegend = "sync_unknown: operations on a sync primitive at this site whose primitive the graph does not name, each with op and reason: " +
	"no_origin (a value no declaration holds: a call result, an element), incomplete_callers (a parameter some callers pass without a declaration: " +
	"callers_resolved of callers_total), open_callers (a parameter of a function code outside the graph can call), no_holder (a context or timer " +
	"stored in no declaration), not_followed (a value the value flow does not follow)"

// subKindLegend explains the statement sub kinds, keyed by sub kind.
var subKindLegend = map[string]string{
	types.SubKindSelect:        "SwitchStmt select: a select statement that waits until one case is ready; its cases are the send and receive sites it encloses with kind cond",
	types.SubKindSelectDefault: "SwitchStmt select_default: a select statement with a default case: it never waits",
	types.SubKindClose:         "CallSite close: a close(ch) call",
}

// nodeLegend explains the statement node types of schema 1.45.
var nodeLegend = map[types.NodeType]string{
	types.NodeBranchStmt: "BranchStmt: a break, continue, goto or fallthrough (sub_kind); jumps_to says where it goes",
	types.NodeCaseClause: "CaseClause: one case or default clause of a switch or select, enclosed by it with kind clause",
	types.NodeDeferStmt:  "DeferStmt: a defer statement; the call it defers runs when the function returns",
	types.NodeFuncLit:    "FuncLit: a function literal; its code is the enclosing function's, and literal_passed_to says what a call does with it",
}

// addLegend sets out[legendKey] to the explanations of the facts nodes and
// edges carry, when there are any.
func addLegend(out map[string]any, nodes []types.Node, edges []types.Edge) {
	l := map[string]string{}
	for _, e := range edges {
		if text, ok := edgeLegend[e.Type]; ok {
			l[string(e.Type)] = text
		}
		switch {
		case e.Type == types.EdgeFlowsTo && e.DispatchKind == sharedSyncKind:
			l["flows_to sync"] = flowsToSyncLegend
		case e.Type == types.EdgeEncloses && e.DispatchKind == clauseKind:
			l["encloses clause"] = clauseLegend
		}
	}
	for _, n := range nodes {
		if text, ok := subKindLegend[n.SubKind]; ok {
			l[string(n.Type)+" "+n.SubKind] = text
		}
		if text, ok := nodeLegend[n.Type]; ok {
			l[string(n.Type)] = text
		}
		if len(n.SyncUnknown) > 0 {
			l["sync_unknown"] = syncUnknownLegend
		}
	}
	if len(l) > 0 {
		out[legendKey] = l
	}
}

// rowLegend explains the keys of the message rows (handled_by, triggers).
var rowLegend = map[string]string{
	"by":        "by: what woke the receive: send (a value arrives; send_line is the send) or close (the channel was closed; close_line is the close: every receive wakes and gets the zero value, ok false)",
	"certain":   "certain false, reason aliasing: the send (or close) and the receive may be on this channel only as far as the analysis cannot tell it from other channels made at the same place",
	wokenByKey:  "woken_by, for a receive no send in the build reaches: timer (the runtime sends when the timer made at timer_made_at fires), cancel (the context's Done channel, closed when the context made at context_made_at is cancelled by the calls in cancelled_by; done_stored_at when the channel is kept in a field), subscription (the channel is handed to the call at subscribed_at; what is delivered through it, an event.Feed by reflection, is not followed)",
	noSenderKey: "no_sender_in_build true: nothing in the build sends on or closes the channel as far as the graph knows (a channel of code outside the build)",
}

// addRowLegend adds to out's legend the keys its handled_by and triggers
// rows use, when they use any.
func addRowLegend(out map[string]any) {
	l, _ := out[legendKey].(map[string]string)
	if l == nil {
		l = map[string]string{}
	}
	var visit func(v any)
	visit = func(v any) {
		switch x := v.(type) {
		case []map[string]any:
			for _, r := range x {
				visit(r)
			}
		case map[string]any:
			for k, val := range x {
				if text, ok := rowLegend[k]; ok {
					l[k] = text
				}
				visit(val)
			}
		}
	}
	visit(out[handledByKey])
	visit(out[triggersKey])
	if len(l) > 0 {
		out[legendKey] = l
	}
}
