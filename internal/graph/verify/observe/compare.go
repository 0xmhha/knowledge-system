package observe

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// GraphReader is the read surface Compare needs from a graph store.
type GraphReader interface {
	AllNodes() ([]types.Node, error)
	QueryEdgesByType(t string) ([]types.Edge, error)
}

// Report compares what a test run did with the runtime links of the graph.
// Observation shows what happened, not all that can happen: it measures how
// much of what ran the graph holds (recall), and how much of the graph ran
// (coverage), not precision.
type Report struct {
	Records int `json:"records"`
	// Interface calls that ran: (interface method, implementation) pairs
	// found from a recorded call on a line where the graph has an
	// interface-method invokes edge.
	Interface PairStats `json:"interface_calls"`
	// InterfaceByTypeOnly: of Interface.InGraph, those linked only by the
	// type hierarchy (dispatch_kind type_hierarchy), not by the value flow.
	InterfaceByTypeOnly int `json:"interface_calls_by_type_only"`
	// Function-value calls that ran: (caller, line, callee) on a line where
	// the graph has a function-value invokes edge.
	FuncValue PairStats `json:"func_value_calls"`
	// DispatchCoverage: dispatches_to edges that some recorded call ran.
	DispatchEdges    int `json:"dispatch_edges"`
	DispatchObserved int `json:"dispatch_edges_observed"`
	// Channels: sender and receiver functions seen on the same channel.
	ChannelOps   PairStats `json:"channel_ops"`   // (function, line, op) with a linked channel edge on that line
	ChannelPairs PairStats `json:"channel_pairs"` // (sender, receiver) joined by a channel node in the graph
	// Of the pairs not joined: both operations are linked, to different
	// channel nodes (the graph names one channel twice, for example its
	// make site and the field it is stored in), or one is not linked.
	ChannelPairsDisjoint int `json:"channel_pairs_disjoint"`
	ChannelPairsUnlinked int `json:"channel_pairs_unlinked"`
	// ChannelOpsRuntime: receives on a channel the runtime makes and sends
	// on (a timer's or ticker's C, a context's Done), which the graph marks
	// with operates_on kind timer_recv or done_recv. No send in the build
	// reaches them, so they are outside ChannelOps.
	ChannelOpsRuntime int `json:"channel_ops_runtime"`
}

// PairStats counts observed items and how many the graph holds.
type PairStats struct {
	Observed       int      `json:"observed"`
	InGraph        int      `json:"in_graph"`
	MissingSamples []string `json:"missing_samples,omitempty"`
}

// Recall is InGraph over Observed.
func (s PairStats) Recall() float64 {
	if s.Observed == 0 {
		return 0
	}
	return float64(s.InGraph) / float64(s.Observed)
}

type record struct {
	K      string `json:"k"`
	Callee string `json:"callee"`
	Caller string `json:"caller"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Ch     string `json:"ch"`
	Op     int    `json:"op"`
	Fn     string `json:"fn"`
	// Defer are the lines of the caller's defer statements when Line is
	// where the caller ends: the call may be a deferred one.
	Defer []int `json:"defer"`
}

// closure and instantiation suffixes of runtime function names:
// pkg.F.func1, pkg.F.func1.2, pkg.F.gowrap1, pkg.F[...].
var (
	reClosure = regexp.MustCompile(`(\.(func|gowrap|deferwrap)\d+|\.\d+)+$`)
	reInst    = regexp.MustCompile(`\[[^\]]*\]`)
)

// Canonical turns a runtime function name into the graph's canonical id of
// the declaration that holds it: closures belong to their declaration, a
// value receiver is written (T), an instantiation is its generic origin.
func Canonical(fn string) string {
	fn = reInst.ReplaceAllString(fn, "")
	fn = reClosure.ReplaceAllString(fn, "")
	i := strings.LastIndex(fn, "/")
	rest := fn[i+1:]
	j := strings.Index(rest, ".")
	if j < 0 {
		return fn
	}
	pkg, sym := fn[:i+1+j], rest[j+1:]
	if k := strings.Index(sym, "."); k > 0 && !strings.HasPrefix(sym, "(") {
		sym = "(" + sym[:k] + ")" + sym[k:]
	}
	return pkg + "." + sym
}

func isClosure(fn string) bool { return reClosure.MatchString(reInst.ReplaceAllString(fn, "")) }

// Compare reads every obs-*.jsonl under dir and compares it with g.
func Compare(dir string, g GraphReader, samples int) (Report, error) {
	var r Report
	recs, err := readRecords(dir)
	if err != nil {
		return r, err
	}
	r.Records = len(recs)
	nodes, err := g.AllNodes()
	if err != nil {
		return r, err
	}
	byCanon := map[string]types.Node{}
	byID := map[string]types.Node{}
	for _, n := range nodes {
		byID[n.ID] = n
		if (n.Type == types.NodeFunction || n.Type == types.NodeMethod) && n.CanonicalID != "" {
			byCanon[baseID(n.CanonicalID)] = n
		}
	}
	edges := func(t types.EdgeType) []types.Edge {
		es, _ := g.QueryEdgesByType(string(t))
		return es
	}
	type lineKey struct {
		src  string
		line int
	}
	ifaceAt := map[lineKey][]string{} // interface methods invoked on a line
	fvAt := map[lineKey]bool{}        // function-value calls on a line
	type staticKey struct {
		src, dst string
		line     int
	}
	static := map[staticKey]bool{} // static calls: a recorded call matching one is not dynamic
	for _, e := range edges(types.EdgeCalls) {
		static[staticKey{e.Src, e.Dst, e.Line}] = true
	}
	for _, e := range edges(types.EdgeInvokes) {
		k := lineKey{e.Src, e.Line}
		switch {
		case e.DispatchKind == "interface_method":
			ifaceAt[k] = append(ifaceAt[k], e.Dst)
		case e.Src == e.Dst:
			fvAt[k] = true
		}
	}
	type dkey struct {
		src, dst string
		line     int
	}
	disp := map[dkey]bool{}
	byTypeOnly := map[dkey]bool{}
	dispEdges := edges(types.EdgeDispatchesTo)
	for _, e := range dispEdges {
		disp[dkey{e.Src, e.Dst, e.Line}] = true
		if e.DispatchKind == "type_hierarchy" {
			byTypeOnly[dkey{e.Src, e.Dst, e.Line}] = true
		}
	}
	r.DispatchEdges = len(dispEdges)
	ran := map[dkey]bool{}

	sample := func(s *PairStats, text string) {
		if len(s.MissingSamples) < samples {
			s.MissingSamples = append(s.MissingSamples, text)
		}
	}
	seenIface := map[[2]string]bool{}
	seenFV := map[dkey]bool{}
	for _, rc := range recs {
		if rc.K != "call" {
			continue
		}
		caller, ok1 := byCanon[Canonical(rc.Caller)]
		callee, ok2 := byCanon[Canonical(rc.Callee)]
		if !ok1 || !ok2 {
			continue
		}
		// A deferred call is recorded where its caller ends: it is matched
		// on the first defer statement whose call can be this one (a static
		// call of the callee, an interface method of its name, a call
		// through a function value). Two defers through function values
		// are not told apart.
		line := rc.Line
		leaf := leafOf(callee.QualifiedName)
		has := func(l int) bool {
			k := lineKey{caller.ID, l}
			if static[staticKey{caller.ID, callee.ID, l}] || fvAt[k] {
				return true
			}
			for _, m := range ifaceAt[k] {
				if leafOf(byID[m].QualifiedName) == leaf {
					return true
				}
			}
			return false
		}
		if !has(line) {
			for _, d := range rc.Defer {
				if has(d) {
					line = d
					break
				}
			}
		}
		rc.Line = line
		k := lineKey{caller.ID, rc.Line}
		if static[staticKey{caller.ID, callee.ID, rc.Line}] {
			continue // a static call on a line that also makes a dynamic one
		}
		viaIface := false
		for _, m := range ifaceAt[k] {
			if leafOf(byID[m].QualifiedName) != leaf {
				continue
			}
			viaIface = true
			p := [2]string{m, callee.ID}
			if seenIface[p] {
				continue
			}
			seenIface[p] = true
			r.Interface.Observed++
			if disp[dkey{m, callee.ID, 0}] {
				r.Interface.InGraph++
				ran[dkey{m, callee.ID, 0}] = true
				if byTypeOnly[dkey{m, callee.ID, 0}] {
					r.InterfaceByTypeOnly++
				}
			} else {
				sample(&r.Interface, fmt.Sprintf("%s -> %s (called at %s:%d)", byID[m].QualifiedName, callee.QualifiedName, caller.FilePath, rc.Line))
			}
		}
		if fvAt[k] && !viaIface && caller.ID != callee.ID {
			d := dkey{caller.ID, callee.ID, rc.Line}
			if !seenFV[d] {
				seenFV[d] = true
				r.FuncValue.Observed++
				if disp[d] {
					r.FuncValue.InGraph++
					ran[d] = true
				} else {
					kind := "function"
					if isClosure(rc.Callee) {
						kind = "closure in"
					}
					sample(&r.FuncValue, fmt.Sprintf("%s:%d calls %s %s", caller.QualifiedName, rc.Line, kind, callee.QualifiedName))
				}
			}
		}
	}
	r.DispatchObserved = len(ran)

	// Channels.
	type opKey struct {
		fn   string
		line int
		op   int
	}
	chanTo := map[opKey]map[string]bool{} // graph: channel nodes an operation reaches
	for t, op := range map[types.EdgeType]int{types.EdgeSendsTo: 1, types.EdgeRecvsFrom: 2} {
		for _, e := range edges(t) {
			if e.Line == 0 || e.Confidence != types.ConfInferred {
				continue
			}
			k := opKey{e.Src, e.Line, op}
			if chanTo[k] == nil {
				chanTo[k] = map[string]bool{}
			}
			chanTo[k][e.Dst] = true
		}
	}
	runtimeAt := runtimeReceives(edges, byID)
	type side struct {
		fn   string
		line int
	}
	byChan := map[string]map[int][]side{}
	seenOp := map[opKey]bool{}
	for _, rc := range recs {
		if rc.K != "chan" {
			continue
		}
		fn, ok := byCanon[Canonical(rc.Fn)]
		if !ok {
			continue
		}
		k := opKey{fn.ID, rc.Line, rc.Op}
		if rc.Op == recvOp && runtimeAt[siteLine{fn.ID, rc.Line}] {
			if !seenOp[k] {
				seenOp[k] = true
				r.ChannelOpsRuntime++
			}
			continue
		}
		if !seenOp[k] {
			seenOp[k] = true
			r.ChannelOps.Observed++
			if len(chanTo[k]) > 0 {
				r.ChannelOps.InGraph++
			} else {
				sample(&r.ChannelOps, fmt.Sprintf("%s:%d %s", fn.QualifiedName, rc.Line, map[int]string{1: "send", 2: "receive"}[rc.Op]))
			}
		}
		if byChan[rc.Ch] == nil {
			byChan[rc.Ch] = map[int][]side{}
		}
		byChan[rc.Ch][rc.Op] = append(byChan[rc.Ch][rc.Op], side{fn.ID, rc.Line})
	}
	// One channel under several names: union the nodes flows_to joins.
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		p, ok := parent[x]
		if !ok || p == x {
			return x
		}
		r := find(p)
		parent[x] = r
		return r
	}
	for _, e := range edges(types.EdgeFlowsTo) {
		a, b := find(e.Src), find(e.Dst)
		if a != b {
			parent[a] = b
		}
	}
	seenPair := map[[2]opKey]bool{}
	chans := make([]string, 0, len(byChan))
	for c := range byChan {
		chans = append(chans, c)
	}
	sort.Strings(chans)
	for _, c := range chans {
		for _, s := range byChan[c][1] {
			for _, v := range byChan[c][2] {
				sk, vk := opKey{s.fn, s.line, 1}, opKey{v.fn, v.line, 2}
				if seenPair[[2]opKey{sk, vk}] {
					continue
				}
				seenPair[[2]opKey{sk, vk}] = true
				r.ChannelPairs.Observed++
				joined := false
				comps := map[string]bool{}
				for n := range chanTo[vk] {
					comps[find(n)] = true
				}
				for n := range chanTo[sk] {
					if comps[find(n)] {
						joined = true
					}
				}
				if joined {
					r.ChannelPairs.InGraph++
				} else if len(chanTo[sk]) > 0 && len(chanTo[vk]) > 0 {
					r.ChannelPairsDisjoint++
					sample(&r.ChannelPairs, fmt.Sprintf("%s:%d sends, %s:%d receives (different channel nodes)", byID[s.fn].QualifiedName, s.line, byID[v.fn].QualifiedName, v.line))
				} else {
					r.ChannelPairsUnlinked++
					sample(&r.ChannelPairs, fmt.Sprintf("%s:%d sends, %s:%d receives", byID[s.fn].QualifiedName, s.line, byID[v.fn].QualifiedName, v.line))
				}
			}
		}
	}
	return r, nil
}

// recvOp is the op of a receive record.
const recvOp = 2

// siteLine is a line of a function.
type siteLine struct {
	fn   string
	line int
}

// runtimeReceives returns the lines of the receives on a channel the
// runtime sends on: the sites of operates_on edges of kind timer_recv or
// done_recv, by the function that contains them.
func runtimeReceives(edges func(types.EdgeType) []types.Edge, byID map[string]types.Node) map[siteLine]bool {
	sites := map[string]bool{}
	for _, e := range edges(types.EdgeOperatesOn) {
		if op := types.SyncOp(e.DispatchKind); op == types.SyncTimerRecv || op == types.SyncDoneRecv {
			sites[e.Src] = true
		}
	}
	out := map[siteLine]bool{}
	if len(sites) == 0 {
		return out
	}
	for _, e := range edges(types.EdgeContains) {
		if sites[e.Dst] {
			out[siteLine{e.Src, byID[e.Dst].StartLine}] = true
		}
	}
	return out
}

func readRecords(dir string) ([]record, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "obs-*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []record
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<16), 1<<22)
		for sc.Scan() {
			var rc record
			if json.Unmarshal(sc.Bytes(), &rc) == nil {
				out = append(out, rc)
			}
		}
		_ = f.Close()
	}
	return out, nil
}

func leafOf(q string) string {
	if i := strings.LastIndex(q, "."); i >= 0 {
		return q[i+1:]
	}
	return q
}

// baseID strips the "@<line>" suffix the build adds to duplicate ids.
func baseID(cid string) string {
	if i := strings.LastIndex(cid, "@"); i > 0 && !strings.ContainsAny(cid[i:], "./()") {
		return cid[:i]
	}
	return cid
}
