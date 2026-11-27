package mcphandlers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// wake.go says what wakes a receive in the message answers (handled_by,
// triggers): a send (a value arrives) or a close (the receive returns the
// zero value, ok false), told apart by the sender's closes edge on the line
// of its triggers edge; and for a receive no send in the build reaches, the
// runtime when it is a timer's channel (operates_on kind timer_recv) or the
// cancel of a context when it is ctx.Done() (kind done_recv).

// Values of a row's "by" key and its line key.
const (
	wakeBySend     = "send"
	wakeByClose    = "close"
	sendLineKey    = "send_line"
	closeLineKey   = "close_line"
	wokenByKey     = "woken_by"
	wokenByTimer   = "timer"
	wokenByCancel  = "cancel"
	timerMadeAtKey = "timer_made_at"
	ctxMadeAtKey   = "context_made_at"
	cancelledByKey = "cancelled_by"
)

// wakeRow sets on row what a triggers edge says woke the receive, in the
// one shape of every row that names a sender (find_callers' handled_by
// senders, find_callees' triggers): by (send or close), the line of the
// send or close under send_line or close_line, and the channel.
func wakeRow(row map[string]any, by string, line int, channelKind string) {
	row["by"] = by
	row[lineKeyOf(by)] = line
	channelRow(row, channelKind)
}

// channelRow sets the channel of a triggers edge on row: its dispatch kind
// is the channel's name, or "<name>~<reason>" when the send and the receive
// may be on that channel only as far as the analysis cannot tell values
// apart (reason aliasing): the row then says certain false and the reason,
// and the name is printed without the mark.
func channelRow(row map[string]any, kind string) {
	name, reason, uncertain := strings.Cut(kind, "~")
	row["channel"] = name
	if uncertain {
		row["certain"], row["reason"] = false, reason
	}
}

// lineAt is a node's line.
type lineAt struct {
	id   string
	line int
}

// closeLines returns the (closer, line) pairs of the closes edges of ids.
func closeLines(reader store.Reader, ids []string) (map[lineAt]bool, error) {
	edges, _, err := edgesAt(reader, ids, false, types.EdgeCloses)
	if err != nil {
		return nil, err
	}
	out := map[lineAt]bool{}
	for _, e := range edges {
		out[lineAt{e.Src, e.Line}] = true
	}
	return out, nil
}

// wakeKind returns what the triggers edge e does: by close when its sender
// closes the channel on that line.
func wakeKind(e types.Edge, closes map[lineAt]bool) string {
	if closes[lineAt{e.Src, e.Line}] {
		return wakeByClose
	}
	return wakeBySend
}

// lineKeyOf is the key a row gives the line of what wakes it.
func lineKeyOf(by string) string {
	if by == wakeByClose {
		return closeLineKey
	}
	return sendLineKey
}

// place prints a node's file and line.
func place(n types.Node) string { return fmt.Sprintf("%s:%d", n.FilePath, n.StartLine) }

// runtimeWakes returns, for the receive sites recvIDs that a timer or a
// context wakes, what the row says instead of "no sender in the build".
func runtimeWakes(reader store.Reader, recvIDs []string) (map[string]map[string]any, error) {
	ops, holders, err := edgesAt(reader, recvIDs, false, types.EdgeOperatesOn)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]any{}
	for _, e := range ops {
		h, ok := holders[e.Dst]
		if !ok {
			continue
		}
		switch types.SyncOp(e.DispatchKind) {
		case types.SyncTimerRecv:
			made := []string{}
			if h.Type == types.NodeCallSite {
				made = append(made, place(h)) // <-time.After(d): the call makes the timer
			} else {
				sites, err := opsInto(reader, h.ID, types.SyncTimerMake)
				if err != nil {
					return nil, err
				}
				for _, s := range sites {
					made = append(made, place(s))
				}
			}
			sort.Strings(made)
			out[e.Src] = map[string]any{wokenByKey: wokenByTimer, timerMadeAtKey: made}
		case types.SyncDoneRecv:
			row, err := cancelOf(reader, h)
			if err != nil {
				return nil, err
			}
			out[e.Src] = row
		}
	}
	return out, nil
}

// opsInto returns the sites of the operates_on edges of kind op that end at
// the declaration id.
func opsInto(reader store.Reader, id string, op types.SyncOp) ([]types.Node, error) {
	edges, sites, err := edgesAt(reader, []string{id}, true, types.EdgeOperatesOn)
	if err != nil {
		return nil, err
	}
	var out []types.Node
	for _, e := range edges {
		if types.SyncOp(e.DispatchKind) == op {
			if s, ok := sites[e.Src]; ok {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// cancelOf says who can cancel the context ctx: the sites that make it
// (ctx_make), the cancel functions made with it (cancel_make) and the calls
// of those (cancel), each with the function the call is written in.
func cancelOf(reader store.Reader, ctx types.Node) (map[string]any, error) {
	row := map[string]any{wokenByKey: wokenByCancel}
	makers, err := opsInto(reader, ctx.ID, types.SyncContextMake)
	if err != nil {
		return nil, err
	}
	made := []string{}
	var cancels []string
	for _, m := range makers {
		made = append(made, place(m))
		edges, holders, err := edgesAt(reader, []string{m.ID}, false, types.EdgeOperatesOn)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if types.SyncOp(e.DispatchKind) == types.SyncCancelMake {
				if h, ok := holders[e.Dst]; ok {
					cancels = append(cancels, h.ID)
				}
			}
		}
	}
	if len(made) == 0 {
		row["note"] = "the context is not made in the build's code this receive reaches (a parameter, a field set elsewhere): who cancels it is not known"
		return row, nil
	}
	sort.Strings(made)
	row[ctxMadeAtKey] = made
	by := []map[string]any{}
	cancels, err = withStores(reader, cancels)
	if err != nil {
		return nil, err
	}
	for _, c := range cancels {
		sites, err := opsInto(reader, c, types.SyncCancel)
		if err != nil {
			return nil, err
		}
		owners, err := ownersOf(reader, sites)
		if err != nil {
			return nil, err
		}
		for _, s := range sites {
			r := map[string]any{"file": s.FilePath, "line": s.StartLine}
			if o, ok := owners[s.ID]; ok {
				r["caller"] = store.PrintedName(reader, o)
			}
			by = append(by, r)
		}
	}
	sortRows(by)
	row[cancelledByKey] = by
	return row, nil
}

// ownersOf maps each site to the function or method that contains it.
func ownersOf(reader store.Reader, sites []types.Node) (map[string]types.Node, error) {
	edges, owners, err := edgesAt(reader, idsOf(sites), true, types.EdgeContains)
	if err != nil {
		return nil, err
	}
	out := map[string]types.Node{}
	for _, e := range edges {
		if o, ok := owners[e.Src]; ok && (o.Type == types.NodeFunction || o.Type == types.NodeMethod) {
			out[e.Dst] = o
		}
	}
	return out, nil
}

// Values of woken_by for a channel held in a declaration (heldChannelWake).
const (
	wokenBySubscription = "subscription"
	subscribedAtKey     = "subscribed_at"
	subscribeCallKey    = "subscribe_call"
	doneStoredAtKey     = "done_stored_at"
	subscribePrefix     = "Subscribe"
)

// heldChannelWake says what wakes a receive no send in the build reaches,
// from where its channel is kept: a field that holds a context's Done
// channel (written on the line of a ctx.Done() call) is closed when the
// context is cancelled; a channel handed to a call named Subscribe... (an
// event.Feed subscription) is delivered to by the feed, by reflection,
// which the graph does not follow. Both are read off lines: the write or
// read of the channel and the call on that line.
func heldChannelWake(reader store.Reader, recv types.Node) (map[string]any, bool, error) {
	owners, err := ownersOf(reader, []types.Node{recv})
	if err != nil {
		return nil, false, err
	}
	owner, ok := owners[recv.ID]
	if !ok {
		return nil, false, nil
	}
	ops, chans, err := edgesAt(reader, []string{owner.ID}, false, types.EdgeRecvsFrom)
	if err != nil {
		return nil, false, err
	}
	for _, e := range ops {
		ch, ok := chans[e.Dst]
		if !ok || e.Line != recv.StartLine || ch.Type == types.NodeCallSite {
			continue
		}
		if row, ok, err := doneStored(reader, ch); err != nil || ok {
			return row, ok, err
		}
		if row, ok, err := subscribed(reader, ch); err != nil || ok {
			return row, ok, err
		}
	}
	return nil, false, nil
}

// lineUses returns the (function, line) pairs where edges of type t end at
// id (reverse) or start there.
func lineUses(reader store.Reader, id string, t types.EdgeType) ([]lineAt, error) {
	edges, _, err := edgesAt(reader, []string{id}, true, t)
	if err != nil {
		return nil, err
	}
	out := make([]lineAt, 0, len(edges))
	for _, e := range edges {
		out = append(out, lineAt{e.Src, e.Line})
	}
	return out, nil
}

// sitesOn returns the sites of function fn on line.
func sitesOn(reader store.Reader, at lineAt) ([]types.Node, error) {
	_, sites, err := edgesAt(reader, []string{at.id}, false, types.EdgeContains)
	if err != nil {
		return nil, err
	}
	var out []types.Node
	for _, s := range sites {
		if s.Type == types.NodeCallSite && s.StartLine == at.line {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartByte < out[j].StartByte })
	return out, nil
}

// doneStored reports a field written with the channel of a ctx.Done() call.
func doneStored(reader store.Reader, ch types.Node) (map[string]any, bool, error) {
	if ch.Type != types.NodeField {
		return nil, false, nil
	}
	writes, err := lineUses(reader, ch.ID, types.EdgeWritesField)
	if err != nil {
		return nil, false, err
	}
	for _, w := range writes {
		sites, err := sitesOn(reader, w)
		if err != nil {
			return nil, false, err
		}
		for _, s := range sites {
			edges, holders, err := edgesAt(reader, []string{s.ID}, false, types.EdgeOperatesOn)
			if err != nil {
				return nil, false, err
			}
			var ctxs []types.Node
			for _, e := range edges {
				if h, ok := holders[e.Dst]; ok && types.SyncOp(e.DispatchKind) == types.SyncContextDone {
					ctxs = append(ctxs, h)
				}
			}
			if len(ctxs) == 0 {
				continue
			}
			row, err := cancelOfAll(reader, ctxs)
			if err != nil {
				return nil, false, err
			}
			row[doneStoredAtKey] = place(s)
			return row, true, nil
		}
	}
	return nil, false, nil
}

// subscribed reports a channel handed to a call whose callee's name starts
// with Subscribe, on a line that reads it: a field read there, or the local
// variable a make site is assigned to.
func subscribed(reader store.Reader, ch types.Node) (map[string]any, bool, error) {
	var uses []lineAt
	switch ch.Type {
	case types.NodeField, types.NodeVariable:
		u, err := lineUses(reader, ch.ID, types.EdgeReadsField)
		if err != nil {
			return nil, false, err
		}
		uses = u
	case types.NodeChannel:
		// The local variable declared on the make's line.
		owners, err := ownersOf(reader, []types.Node{ch})
		if err != nil {
			return nil, false, err
		}
		o, ok := owners[ch.ID]
		if !ok {
			return nil, false, nil
		}
		_, locals, err := edgesAt(reader, []string{o.ID}, false, types.EdgeDefines)
		if err != nil {
			return nil, false, err
		}
		for _, l := range locals {
			if l.Type == types.NodeLocalVariable && l.StartLine == ch.StartLine && l.FilePath == ch.FilePath {
				u, err := lineUses(reader, l.ID, types.EdgeReferences)
				if err != nil {
					return nil, false, err
				}
				uses = append(uses, u...)
			}
		}
	}
	for _, u := range uses {
		sites, err := sitesOn(reader, u)
		if err != nil {
			return nil, false, err
		}
		for _, s := range sites {
			calls, callees, err := edgesAt(reader, []string{s.ID}, false, types.EdgeSiteCalls)
			if err != nil {
				return nil, false, err
			}
			for _, c := range calls {
				if n, ok := callees[c.Dst]; ok && strings.HasPrefix(n.Name, subscribePrefix) {
					return map[string]any{wokenByKey: wokenBySubscription, subscribedAtKey: place(s),
						subscribeCallKey: store.PrintedName(reader, n),
						"note":           "the channel is handed to a subscription; what is delivered through it (an event.Feed sends by reflection) is not followed in the graph"}, true, nil
				}
			}
		}
	}
	return nil, false, nil
}

// cancelOfAll is cancelOf over the contexts a ctx.Done() call may be on (a
// parameter followed to its callers' declarations): their makers and
// cancels together.
func cancelOfAll(reader store.Reader, ctxs []types.Node) (map[string]any, error) {
	made, by := []string{}, []map[string]any{}
	for _, c := range ctxs {
		row, err := cancelOf(reader, c)
		if err != nil {
			return nil, err
		}
		if m, ok := row[ctxMadeAtKey].([]string); ok {
			made = append(made, m...)
		}
		if b, ok := row[cancelledByKey].([]map[string]any); ok {
			by = append(by, b...)
		}
	}
	if len(made) == 0 {
		return cancelOf(reader, ctxs[0])
	}
	sort.Strings(made)
	sortRows(by)
	return map[string]any{wokenByKey: wokenByCancel, ctxMadeAtKey: made, cancelledByKey: by}, nil
}

// withStores adds to the holders ids the fields and variables they are
// stored into (flows_to of kind sync, transitively): a cancel function made
// into a local and kept in a field is called through the field.
func withStores(reader store.Reader, ids []string) ([]string, error) {
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	out := append([]string(nil), ids...)
	for frontier := ids; len(frontier) > 0; {
		edges, _, err := edgesAt(reader, frontier, false, types.EdgeFlowsTo)
		if err != nil {
			return nil, err
		}
		var next []string
		for _, e := range edges {
			if e.DispatchKind == concfacts.SyncAliasKind && !seen[e.Dst] {
				seen[e.Dst] = true
				next = append(next, e.Dst)
				out = append(out, e.Dst)
			}
		}
		frontier = next
	}
	return out, nil
}
