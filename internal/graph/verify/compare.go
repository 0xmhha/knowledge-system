package verify

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// GraphReader is the read surface verify needs from a graph store.
type GraphReader interface {
	AllNodes() ([]types.Node, error)
	QueryEdgesByType(t string) ([]types.Edge, error)
}

// declNodeTypes are the node kinds a package-level declaration maps to.
var declNodeTypes = map[types.NodeType]bool{
	types.NodeFunction: true, types.NodeMethod: true, types.NodeStruct: true,
	types.NodeInterface: true, types.NodeTypeAlias: true, types.NodeConstant: true,
	types.NodeVariable: true,
}

// Relation edge kinds read from the graph.
var (
	callEdgeTypes = []types.EdgeType{types.EdgeCalls, types.EdgeInvokes}
	typeEdgeTypes = []types.EdgeType{types.EdgeUsesType, types.EdgeInstantiates}
)

// Options tunes a run.
type Options struct {
	// Samples caps the examples listed per category (0 means 20).
	Samples int
	// GoBuild is the build configuration the graph was built for; the truth
	// is type-checked with the same platform, cgo setting and tags. nil uses
	// the host defaults.
	GoBuild *detect.GoBuild
}

const defaultSamples = 20

// RunGo compares the graph against go/types for the Go files the graph covers.
func RunGo(srcRoot string, g GraphReader, opts Options) (Report, error) {
	if opts.Samples <= 0 {
		opts.Samples = defaultSamples
	}
	nodes, err := g.AllNodes()
	if err != nil {
		return Report{}, fmt.Errorf("verify: read nodes: %w", err)
	}
	scope := map[string]bool{}
	for _, n := range nodes {
		if n.Language == "go" && strings.HasSuffix(n.FilePath, ".go") {
			scope[filepath.ToSlash(n.FilePath)] = true
		}
	}
	truth, err := collectTruth(srcRoot, scope, opts.GoBuild)
	if err != nil {
		return Report{}, err
	}
	idx := newNodeIndex(nodes)
	r := Report{Files: len(scope), Load: LoadStats{LoadProblems: truth.Load,
		ModulesMatched: truth.Modules.Matched, ModulesOtherVersion: truth.Modules.OtherVersion,
		ModulesOtherDir: truth.Modules.OtherDir, ModulesNotLoaded: truth.Modules.NotLoaded}}
	declNode := r.matchDecls(truth, idx, opts.Samples)
	calls, err := readEdges(g, callEdgeTypes)
	if err != nil {
		return Report{}, err
	}
	typeEdges, err := readEdges(g, typeEdgeTypes)
	if err != nil {
		return Report{}, err
	}
	r.StaticCalls = compare(truth.StaticCalls, calls, declNode, idx, opts.Samples)
	r.InterfaceCalls = compare(truth.InterfaceCalls, calls, declNode, idx, opts.Samples)
	r.TypeRefs = compare(truth.TypeRefs, typeEdges, declNode, idx, opts.Samples)
	external := externalNodes(nodes)
	r.OutsideCalls = compareOutside(truth.OutsideCalls, calls, declNode, external, opts.Samples)
	r.OutsideTypeRefs = compareOutside(truth.OutsideTypeRefs, typeEdges, declNode, external, opts.Samples)
	r.DynamicCallSites = truth.DynamicCallSites
	r.CallsOutsideFuncs = truth.CallsOutsideFuncs
	r.Unexpected = unexpected(truth, calls, typeEdges, declNode, opts.Samples)
	r.LocalDeclNodes = localDeclNodes(nodes, declNode, opts.Samples)
	if r.Statements, err = compareStatements(truth.Stmts, nodes, g, opts.Samples); err != nil {
		return Report{}, err
	}
	if r.UnderLock, err = compareUnderLock(truth.UnderLock, idx, g, opts.Samples); err != nil {
		return Report{}, err
	}
	return r, nil
}

// compareUnderLock compares the accessed_under_lock edges of g with the
// truth's pairs. A node is named like a truth declaration: its file, its
// first line and its name (a Mutex node has its declaration's span and the
// name of the variable or field, or of the embedded mutex type).
func compareUnderLock(truth map[[2]string]bool, idx nodeIndex, g GraphReader, samples int) (UnderLockStats, error) {
	edges, err := g.QueryEdgesByType(string(types.EdgeAccessedUnderLock))
	if err != nil {
		return UnderLockStats{}, fmt.Errorf("verify: read accessed_under_lock: %w", err)
	}
	key := func(id string) string {
		n, ok := idx.byID[id]
		if !ok {
			return ""
		}
		return fmt.Sprintf("%s:%d:%s", n.FilePath, n.StartLine, n.Name)
	}
	st := UnderLockStats{Truth: len(truth)}
	graph := map[[2]string]bool{}
	var unsupported []string
	for _, e := range edges {
		k := [2]string{key(e.Src), key(e.Dst)}
		if graph[k] {
			continue
		}
		graph[k] = true
		st.Edges++
		if truth[k] {
			st.Matched++
			continue
		}
		st.Unsupported++
		unsupported = append(unsupported, k[0]+" -> "+k[1])
	}
	missing := make([]string, 0)
	for k := range truth {
		if !graph[k] {
			missing = append(missing, k[0]+" -> "+k[1])
		}
	}
	sort.Strings(missing)
	sort.Strings(unsupported)
	st.MissingSamples = firstN(missing, samples)
	st.UnsupportedSamples = firstN(unsupported, samples)
	return st, nil
}

// nodeIndex looks graph nodes up by id and by (file, canonical id base).
type nodeIndex struct {
	byID   map[string]types.Node
	byBase map[[2]string][]types.Node // (file, canonical id without "@line")
}

func newNodeIndex(nodes []types.Node) nodeIndex {
	idx := nodeIndex{byID: make(map[string]types.Node, len(nodes)), byBase: map[[2]string][]types.Node{}}
	for _, n := range nodes {
		idx.byID[n.ID] = n
		if n.CanonicalID == "" || !declNodeTypes[n.Type] {
			continue
		}
		k := [2]string{filepath.ToSlash(n.FilePath), baseID(n.CanonicalID)}
		idx.byBase[k] = append(idx.byBase[k], n)
	}
	return idx
}

// baseID strips the "@<line>" suffix the build adds to duplicate ids.
func baseID(cid string) string {
	if i := strings.LastIndex(cid, "@"); i > 0 && !strings.ContainsAny(cid[i:], "./()") {
		return cid[:i]
	}
	return cid
}

// matchDecls maps each truth declaration to its graph node id, recording the
// declarations the graph lacks.
func (r *Report) matchDecls(t *Truth, idx nodeIndex, samples int) map[string]string {
	keys := make([]string, 0, len(t.Decls))
	for k := range t.Decls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		d := t.Decls[key]
		cid := d.CanonicalID
		r.Decls.Truth++
		var hit *types.Node
		for _, n := range idx.byBase[[2]string{d.File, cid}] {
			if n.StartLine <= d.Line && d.Line <= n.EndLine {
				hit = &n
				break
			}
		}
		if hit == nil {
			r.Decls.Missing++
			if len(r.Decls.MissingSamples) < samples {
				r.Decls.MissingSamples = append(r.Decls.MissingSamples, fmt.Sprintf("%s (%s:%d)", cid, d.File, d.Line))
			}
			continue
		}
		r.Decls.Matched++
		out[key] = hit.ID
		if hit.StartByte != d.StartByte || hit.EndByte != d.EndByte {
			r.Decls.OtherSpan++
			if len(r.Decls.OtherSpanSamples) < samples {
				r.Decls.OtherSpanSamples = append(r.Decls.OtherSpanSamples,
					fmt.Sprintf("%s (%s:%d): node spans bytes %d-%d, declaration %d-%d", cid, d.File, d.Line,
						hit.StartByte, hit.EndByte, d.StartByte, d.EndByte))
			}
		}
	}
	return out
}

func readEdges(g GraphReader, kinds []types.EdgeType) (map[string]map[string]bool, error) {
	out := map[string]map[string]bool{}
	for _, k := range kinds {
		edges, err := g.QueryEdgesByType(string(k))
		if err != nil {
			return nil, fmt.Errorf("verify: read %s edges: %w", k, err)
		}
		for _, e := range edges {
			if out[e.Src] == nil {
				out[e.Src] = map[string]bool{}
			}
			out[e.Src][e.Dst] = true
		}
	}
	return out, nil
}

// compare classifies each truth pair against the graph edges.
func compare(pairs []Pair, edges map[string]map[string]bool, declNode map[string]string, idx nodeIndex, samples int) RelationStats {
	var s RelationStats
	// truthTargets holds, per source node, the target nodes go/types supports
	// in this family. A same-name edge to one of them is correct, so the
	// absent edge next to it is missing, not misresolved.
	truthTargets := map[string]map[string]bool{}
	for _, p := range pairs {
		from, okFrom := declNode[p.From]
		to, okTo := declNode[p.To]
		if okFrom && okTo {
			if truthTargets[from] == nil {
				truthTargets[from] = map[string]bool{}
			}
			truthTargets[from][to] = true
		}
	}
	for _, p := range pairs {
		s.Truth++
		from, okFrom := declNode[p.From]
		to, okTo := declNode[p.To]
		if !okFrom || !okTo {
			s.EndpointMissing++
			continue
		}
		if edges[from][to] {
			s.Matched++
			continue
		}
		if wrong := sameNameTarget(edges[from], truthTargets[from], to, idx); wrong != "" {
			s.Misresolved++
			if len(s.MisresolvedSamples) < samples {
				s.MisresolvedSamples = append(s.MisresolvedSamples,
					fmt.Sprintf("%s -> %s at %s:%d; graph points to %s", p.From, p.To, p.File, p.Line, wrong))
			}
			continue
		}
		s.Missing++
		if p.InClosure {
			s.MissingInClosure++
		}
		if len(s.MissingSamples) < samples {
			where := ""
			if p.InClosure {
				where = " (in a function literal)"
			}
			s.MissingSamples = append(s.MissingSamples, fmt.Sprintf("%s -> %s at %s:%d%s", p.From, p.To, p.File, p.Line, where))
		}
	}
	return s
}

// externalNodes maps the canonical id of each External node to its node id.
func externalNodes(nodes []types.Node) map[string]string {
	out := map[string]string{}
	for _, n := range nodes {
		if n.Type == types.NodeExternal && n.CanonicalID != "" {
			out[n.CanonicalID] = n.ID
		}
	}
	return out
}

// compareOutside classifies the truth pairs whose target is outside the
// covered files: matched when the graph has an edge from the source's node to
// the External node of the target, missing when it has no such node or edge.
func compareOutside(pairs []Pair, edges map[string]map[string]bool, declNode, external map[string]string, samples int) RelationStats {
	var s RelationStats
	for _, p := range pairs {
		s.Truth++
		from, ok := declNode[p.From]
		if !ok {
			s.EndpointMissing++
			continue
		}
		if to, ok := external[p.To]; ok && edges[from][to] {
			s.Matched++
			continue
		}
		s.Missing++
		if p.InClosure {
			s.MissingInClosure++
		}
		if len(s.MissingSamples) < samples {
			s.MissingSamples = append(s.MissingSamples, fmt.Sprintf("%s -> %s at %s:%d", p.From, p.To, p.File, p.Line))
		}
	}
	return s
}

// sameNameTarget returns a description of a graph target of from that has
// the expected node's name but is a different node and not itself a
// supported target (supported), or "".
func sameNameTarget(targets, supported map[string]bool, want string, idx nodeIndex) string {
	w := idx.byID[want]
	ids := make([]string, 0, len(targets))
	for id := range targets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := idx.byID[id]
		if id != want && !supported[id] && n.Name == w.Name && n.Type == w.Type {
			return fmt.Sprintf("%s (%s:%d)", n.CanonicalID, n.FilePath, n.StartLine)
		}
	}
	return ""
}

// unexpected counts graph edges between matched declarations that have no
// truth pair of the same family.
func unexpected(t *Truth, calls, typeEdges map[string]map[string]bool, declNode map[string]string, samples int) UnexpectedStats {
	nodeDecl := make(map[string]string, len(declNode))
	for cid, id := range declNode {
		nodeDecl[id] = cid
	}
	callTruth := pairKeys(t.StaticCalls, t.InterfaceCalls)
	typeTruth := pairKeys(t.TypeRefs)
	var u UnexpectedStats
	count := func(edges map[string]map[string]bool, truth map[[2]string]bool, n *int, list *[]string) {
		srcs := make([]string, 0, len(edges))
		for src := range edges {
			srcs = append(srcs, src)
		}
		sort.Strings(srcs)
		for _, src := range srcs {
			from, ok := nodeDecl[src]
			if !ok {
				continue
			}
			dsts := make([]string, 0, len(edges[src]))
			for dst := range edges[src] {
				dsts = append(dsts, dst)
			}
			sort.Strings(dsts)
			for _, dst := range dsts {
				to, ok := nodeDecl[dst]
				if !ok || from == to || truth[[2]string{from, to}] {
					continue
				}
				*n++
				if len(*list) < samples {
					*list = append(*list, from+" -> "+to)
				}
			}
		}
	}
	count(calls, callTruth, &u.Calls, &u.CallSamples)
	count(typeEdges, typeTruth, &u.TypeRefs, &u.TypeRefSamples)
	return u
}

func pairKeys(lists ...[]Pair) map[[2]string]bool {
	out := map[[2]string]bool{}
	for _, l := range lists {
		for _, p := range l {
			out[[2]string{p.From, p.To}] = true
		}
	}
	return out
}

// localDeclNodes lists declaration-kind graph nodes with a canonical id that
// no package-level truth declaration maps to: function-local types and the
// like, exposed under a package-level id.
func localDeclNodes(nodes []types.Node, declNode map[string]string, samples int) LocalDeclStats {
	matched := make(map[string]bool, len(declNode))
	for _, id := range declNode {
		matched[id] = true
	}
	var all []string
	for _, n := range nodes {
		if n.Language != "go" || n.CanonicalID == "" || !declNodeTypes[n.Type] || matched[n.ID] {
			continue
		}
		all = append(all, fmt.Sprintf("%s %s (%s:%d)", n.Type, n.CanonicalID, n.FilePath, n.StartLine))
	}
	sort.Strings(all)
	s := LocalDeclStats{Count: len(all)}
	s.Samples = all[:min(len(all), samples)]
	return s
}
