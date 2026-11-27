package mcphandlers

import (
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
func sendersByReceive(reader store.Reader, trig []types.Edge, senders map[string]types.Node, spawner map[string]types.Node) map[string][]map[string]any {
	type sendKey struct {
		recv, channel string
		line          int
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
		k := sendKey{e.Dst, e.DispatchKind, e.Line}
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
	for _, k := range order {
		g := groups[k]
		if len(g.fn) == 0 {
			// The goroutine's own literal sends: the function that starts
			// it is the sender.
			for i, name := range g.gors {
				row := map[string]any{"sender": name, "line": k.line, "channel": k.channel}
				if f, ok := spawner[g.gorIDs[i]]; ok {
					row["sender"], row["goroutines"] = store.PrintedName(reader, f), []string{name}
				}
				out[k.recv] = append(out[k.recv], row)
			}
			continue
		}
		for _, name := range g.fn {
			row := map[string]any{"sender": name, "line": k.line, "channel": k.channel}
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
	sendersOf := sendersByReceive(reader, trig, senders, spawner)
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
	rows := []map[string]any{}
	for _, e := range trig {
		r, ok := recvs[e.Dst]
		if !ok {
			continue
		}
		row := map[string]any{"receive": store.PrintedName(reader, r), "file": r.FilePath, "line": r.StartLine,
			"channel": e.DispatchKind, "send_line": e.Line}
		if h := handledOf[r.ID]; len(h) > 0 {
			row["handles"] = h
		}
		rows = append(rows, row)
	}
	putRows(out, triggersKey, rows)
	return nil
}

// putRows sets out[key] to rows in file and line order, at most
// messageLimit, with key_total when there are more. Nothing is set for none.
func putRows(out map[string]any, key string, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		fi, fj := rows[i]["file"].(string), rows[j]["file"].(string)
		if fi != fj {
			return fi < fj
		}
		return rows[i]["line"].(int) < rows[j]["line"].(int)
	})
	if len(rows) > messageLimit {
		out[key+"_total"] = len(rows)
		rows = rows[:messageLimit]
	}
	out[key] = rows
}

// idsOf returns the ids of nodes.
func idsOf(nodes []types.Node) []string {
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	return ids
}
