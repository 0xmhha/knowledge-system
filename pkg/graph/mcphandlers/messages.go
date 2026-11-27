package mcphandlers

import (
	"encoding/json"
	"sort"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// messages.go answers the two message questions (schema 1.42): what runs a
// function because a message arrives (find_callers, key handledByKey), and
// what a function's sends make run (find_callees, key triggersKey). Both
// are two hops: triggers (sender -> receive site), then handles (receive
// site -> the code in its range).

// Keys of the message answers.
const (
	handledByKey = "handled_by"
	triggersKey  = "triggers"
)

// noSenderKey marks a receive no send in the build reaches.
const noSenderKey = "no_sender_in_build"

// spawnersOf maps each Goroutine among senders to the function that starts
// it (spawns).
func spawnersOf(reader store.Reader, senders map[string]types.Node) (map[string]types.Node, error) {
	var ids []string
	for id, n := range senders {
		if n.Type == types.NodeGoroutine {
			ids = append(ids, id)
		}
	}
	out := map[string]types.Node{}
	if len(ids) == 0 {
		return out, nil
	}
	nodes, edges, err := reader.NeighborhoodByIDs(ids, 1, true, string(types.EdgeSpawns))
	if err != nil {
		return nil, err
	}
	byID := map[string]types.Node{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	for _, e := range edges {
		if f, ok := byID[e.Src]; ok {
			out[e.Dst] = f
		}
	}
	return out, nil
}

// sendersByReceive groups the triggers edges trig by the receive they end
// at, one entry per send (line and channel): always a function, with the
// goroutines that run the send (goroutines). A send in a function that
// goroutines run has an edge from each, listed once; a send in a goroutine's
// own literal names the function that starts the goroutine (spawner).
func sendersByReceive(reader store.Reader, trig []types.Edge, senders map[string]types.Node, spawner map[string]types.Node,
	closes map[lineAt]bool) map[string][]map[string]any {
	type sendKey struct {
		recv, channel string
		line          int
		by            string
	}
	type group struct {
		fn     []string
		gors   []string
		gorIDs []string
	}
	groups := map[sendKey]*group{}
	var order []sendKey
	for _, e := range trig {
		s, ok := senders[e.Src]
		if !ok {
			continue
		}
		k := sendKey{e.Dst, e.DispatchKind, e.Line, wakeKind(e, closes)}
		g := groups[k]
		if g == nil {
			g = &group{}
			groups[k] = g
			order = append(order, k)
		}
		if s.Type == types.NodeGoroutine {
			g.gors = append(g.gors, store.PrintedName(reader, s))
			g.gorIDs = append(g.gorIDs, s.ID)
		} else {
			g.fn = append(g.fn, store.PrintedName(reader, s))
		}
	}
	out := map[string][]map[string]any{}
	defer func() {
		for _, rows := range out {
			sortRows(rows) // by line, then sender name
		}
	}()
	for _, k := range order {
		g := groups[k]
		if len(g.fn) == 0 {
			// The goroutine's own literal sends: the function that starts
			// it is the sender.
			for i, name := range g.gors {
				row := map[string]any{"sender": name}
				wakeRow(row, k.by, k.line, k.channel)
				if f, ok := spawner[g.gorIDs[i]]; ok {
					row["sender"], row["goroutines"] = store.PrintedName(reader, f), []string{name}
				}
				out[k.recv] = append(out[k.recv], row)
			}
			continue
		}
		for _, name := range g.fn {
			row := map[string]any{"sender": name}
			wakeRow(row, k.by, k.line, k.channel)
			if len(g.gors) > 0 {
				row["goroutines"] = g.gors
			}
			out[k.recv] = append(out[k.recv], row)
		}
	}
	return out
}

// messageLimit bounds the receives listed in one answer.
const messageLimit = 50

// edgesAt returns the edges of type t that end at ids (reverse) or start at
// them, and the nodes at their other ends by id.
func edgesAt(reader store.Reader, ids []string, reverse bool, t types.EdgeType) ([]types.Edge, map[string]types.Node, error) {
	if len(ids) == 0 {
		return nil, nil, nil
	}
	nodes, edges, err := reader.NeighborhoodByIDs(ids, 1, reverse, string(t))
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[string]types.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	return edges, byID, nil
}

// addHandledBy adds to out, under handledByKey, the receives whose message
// runs a call of the seed: the receive's site, the call it handles and the
// senders that can wake it (their name and the line of the send).
func addHandledBy(reader store.Reader, seed seedRef, out map[string]any) error {
	own, err := seed.nodes(reader)
	if err != nil {
		return err
	}
	ids := idsOf(own)
	calls, sites, err := edgesAt(reader, ids, true, types.EdgeSiteCalls)
	if err != nil {
		return err
	}
	siteIDs := make([]string, 0, len(calls))
	for _, e := range calls {
		siteIDs = append(siteIDs, e.Src)
	}
	handles, recvs, err := edgesAt(reader, siteIDs, true, types.EdgeHandles)
	if err != nil || len(handles) == 0 {
		return err
	}
	recvIDs := make([]string, 0, len(handles))
	for _, e := range handles {
		recvIDs = append(recvIDs, e.Src)
	}
	trig, senders, err := edgesAt(reader, recvIDs, true, types.EdgeTriggers)
	if err != nil {
		return err
	}
	spawner, err := spawnersOf(reader, senders)
	if err != nil {
		return err
	}
	closes, err := closeLines(reader, idsOf(nodeList(senders)))
	if err != nil {
		return err
	}
	runtime, err := runtimeWakes(reader, recvIDs)
	if err != nil {
		return err
	}
	sendersOf := sendersByReceive(reader, trig, senders, spawner, closes)
	rows := []map[string]any{}
	for _, e := range handles {
		r, ok := recvs[e.Src]
		if !ok {
			continue
		}
		row := map[string]any{"receive": store.PrintedName(reader, r), "file": r.FilePath, "line": r.StartLine,
			"call": store.PrintedName(reader, sites[e.Dst]), "call_line": e.Line}
		if s := sendersOf[r.ID]; len(s) > 0 {
			row["senders"] = s
		} else if w, ok := runtime[r.ID]; ok {
			// The runtime sends when a timer fires; a context's Done is
			// closed when it is cancelled.
			for k, v := range w {
				row[k] = v
			}
		} else if w, held, err := heldChannelWake(reader, r); err != nil {
			return err
		} else if held {
			// A field holding ctx.Done(), or a channel handed to a
			// subscription.
			for k, v := range w {
				row[k] = v
			}
		} else {
			// The channel comes from outside the build (time.Tick, a
			// library), or nothing in the build sends on it.
			row[noSenderKey] = true
		}
		rows = append(rows, row)
	}
	putRows(out, handledByKey, rows)
	return nil
}

// addTriggers adds to out, under triggersKey, the receives the seed's sends
// can wake: the receive's site, the channel, the line of the send and the
// code the receive then runs (handles: kind, the node and its line; a call
// with the function it calls).
func addTriggers(reader store.Reader, seed seedRef, out map[string]any) error {
	own, err := seed.nodes(reader)
	if err != nil {
		return err
	}
	trig, recvs, err := edgesAt(reader, idsOf(own), false, types.EdgeTriggers)
	if err != nil || len(trig) == 0 {
		return err
	}
	recvIDs := make([]string, 0, len(trig))
	for _, e := range trig {
		recvIDs = append(recvIDs, e.Dst)
	}
	handles, handled, err := edgesAt(reader, recvIDs, false, types.EdgeHandles)
	if err != nil {
		return err
	}
	siteIDs := []string{}
	for _, e := range handles {
		if n := handled[e.Dst]; n.Type == types.NodeCallSite {
			siteIDs = append(siteIDs, n.ID)
		}
	}
	calls, callees, err := edgesAt(reader, siteIDs, false, types.EdgeSiteCalls)
	if err != nil {
		return err
	}
	calleeOf := map[string]string{}
	for _, e := range calls {
		if n, ok := callees[e.Dst]; ok {
			calleeOf[e.Src] = store.PrintedName(reader, n)
		}
	}
	handledOf := map[string][]map[string]any{}
	for _, e := range handles {
		n, ok := handled[e.Dst]
		if !ok {
			continue
		}
		h := map[string]any{"kind": e.DispatchKind, "node": store.PrintedName(reader, n), "line": e.Line}
		if c, ok := calleeOf[n.ID]; ok {
			h["calls"] = c
		}
		handledOf[e.Src] = append(handledOf[e.Src], h)
	}
	closes, err := closeLines(reader, idsOf(own))
	if err != nil {
		return err
	}
	rows := []map[string]any{}
	for _, e := range trig {
		r, ok := recvs[e.Dst]
		if !ok {
			continue
		}
		by := wakeKind(e, closes)
		row := map[string]any{"receive": store.PrintedName(reader, r), "file": r.FilePath, "line": r.StartLine}
		wakeRow(row, by, e.Line, e.DispatchKind)
		if h := handledOf[r.ID]; len(h) > 0 {
			row["handles"] = h
		}
		rows = append(rows, row)
	}
	putRows(out, triggersKey, rows)
	return nil
}

// sortRows puts rows in a total order: by file, then line, then the whole
// row (its JSON text: the receive, call, caller or sender names next), so
// rows that share a line come back in the same order on every call.
func sortRows(rows []map[string]any) {
	keyOf := func(r map[string]any) string {
		b, _ := json.Marshal(r)
		return string(b)
	}
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = keyOf(r)
	}
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ra, rb := rows[idx[a]], rows[idx[b]]
		fa, _ := ra["file"].(string)
		fb, _ := rb["file"].(string)
		if fa != fb {
			return fa < fb
		}
		la, _ := ra["line"].(int)
		lb, _ := rb["line"].(int)
		if la != lb {
			return la < lb
		}
		return keys[idx[a]] < keys[idx[b]]
	})
	sorted := make([]map[string]any, len(rows))
	for i, j := range idx {
		sorted[i] = rows[j]
	}
	copy(rows, sorted)
}

// putRows sets out[key] to rows in file and line order (sortRows), at most
// messageLimit, with key_total when there are more. Nothing is set for none.
func putRows(out map[string]any, key string, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	sortRows(rows)
	if len(rows) > messageLimit {
		out[key+"_total"] = len(rows)
		rows = rows[:messageLimit]
	}
	out[key] = rows
}

// nodeList returns the nodes of a map by id, in id order.
func nodeList(m map[string]types.Node) []types.Node {
	out := make([]types.Node, 0, len(m))
	for _, n := range m {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// idsOf returns the ids of nodes.
func idsOf(nodes []types.Node) []string {
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

// closureCallsKey lists the calls of the seed written inside a function
// literal passed as an argument (find_callers).
const closureCallsKey = "closure_calls"

// addClosureCalls adds to out, under closureCallsKey, the calls of the seed
// that sit in a function literal passed as an argument of another call
// (r.Each(func(...) { m.Snapshot() })), read from the graph's
// literal_passed_to edges: the caller (the function the literal is written
// in, the container of the call's site), the line of the call, and the
// function the literal is passed to with the line of that call. The literal
// runs when that function calls it, so what runs the seed there is found
// with find_callers on that function (and its handled_by).
func addClosureCalls(reader store.Reader, seed seedRef, out map[string]any) error {
	own, err := seed.nodes(reader)
	if err != nil {
		return err
	}
	calls, sites, err := edgesAt(reader, idsOf(own), true, types.EdgeSiteCalls)
	if err != nil || len(calls) == 0 {
		return err
	}
	siteIDs := make([]string, 0, len(calls))
	for _, e := range calls {
		siteIDs = append(siteIDs, e.Src)
	}
	passed, receivers, err := edgesAt(reader, siteIDs, false, types.EdgeLiteralPassedTo)
	if err != nil || len(passed) == 0 {
		return err
	}
	contains, owners, err := edgesAt(reader, siteIDs, true, types.EdgeContains)
	if err != nil {
		return err
	}
	ownerOf := map[string]types.Node{}
	for _, e := range contains {
		if o, ok := owners[e.Src]; ok && (o.Type == types.NodeFunction || o.Type == types.NodeMethod || o.Type == types.NodeVariable) {
			ownerOf[e.Dst] = o
		}
	}
	var rows []map[string]any
	for _, e := range passed {
		x, ok := sites[e.Src]
		recv, ok2 := receivers[e.Dst]
		if !ok || !ok2 {
			continue
		}
		row := map[string]any{"file": x.FilePath, "line": x.StartLine,
			"in_literal_passed_to": store.PrintedName(reader, recv), "passed_at": e.Line}
		if o, ok := ownerOf[x.ID]; ok {
			row["caller"] = store.PrintedName(reader, o)
		}
		rows = append(rows, row)
	}
	putRows(out, closureCallsKey, rows)
	return nil
}
