package persist

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// printed_name.go decides the name a tool prints for a node: the one rule
// behind every qname a response shows. The printed name resolves back, with
// ResolveName, to exactly that node (a promoted copy to the method it
// copies). The rule:
//
//	A node is printed by its qualified name (a variable's Mutex node with
//	"#mutex"). When other declarations carry that name it is printed by
//	its site name "<name>@<file>:<line>", with "#<n>" after it when
//	several names of one declaration share the place (`var _, _ = f()`),
//	and an External stub, which has no place, by its canonical id.
//
// Tools printed the plain qualified name of a node whose name several
// declarations carry, and 1,262 of 71,115 printed names on go-stablenet
// resolved to several nodes (locals declared twice, the v1 and v2 copies of
// a Solidity contract, init functions, blank variables, External stubs of
// one name).

// ordinalMark separates a site name from the place of the name among the
// names of one declaration ("<name>@<file>:<line>#2").
const ordinalMark = "#"

// ordinalOf returns i of a Signature "name i of n" (the names of one
// declaration), or 0.
func ordinalOf(n types.Node) int {
	var i, of int
	if _, err := fmt.Sscanf(n.Signature, "name %d of %d", &i, &of); err != nil || i < 1 {
		return 0
	}
	return i
}

// declKey tells declarations apart among the nodes of one qualified name:
// one declaration is one place and, at that place, one name of the
// declaration; an External stub is its canonical id.
func declKey(n types.Node) string {
	if n.Type == types.NodeExternal || n.FilePath == types.ExternalFilePath {
		return "external|" + n.CanonicalID
	}
	return fmt.Sprintf("%s|%s|%d|%d", n.Language, n.FilePath, n.StartLine, ordinalOf(n))
}

// placeLess orders nodes by file, line and place among the names there.
func placeLess(a, b types.Node) bool {
	if a.FilePath != b.FilePath {
		return a.FilePath < b.FilePath
	}
	if a.StartLine != b.StartLine {
		return a.StartLine < b.StartLine
	}
	if oa, ob := ordinalOf(a), ordinalOf(b); oa != ob {
		return oa < ob
	}
	return a.ID < b.ID
}

// printedNames returns the printed name of each of nodes, which all carry
// one qualified name (what FindSymbol returns for it), by node id.
func printedNames(nodes []types.Node) map[string]string {
	out := make(map[string]string, len(nodes))
	byBase := map[string][]types.Node{}
	for _, n := range nodes {
		byBase[PrintedQName(n)] = append(byBase[PrintedQName(n)], n)
	}
	for base, group := range byBase {
		keys := map[string]bool{}
		atPlace := map[string]int{}
		cids := map[string]int{}
		for _, n := range group {
			if _, isCopy := PromotedFrom(n); isCopy {
				continue
			}
			if k := declKey(n); !keys[k] {
				keys[k] = true
				atPlace[n.FilePath+lineMark+strconv.Itoa(n.StartLine)]++
			}
			cids[n.CanonicalID]++
		}
		for _, n := range group {
			_, isCopy := PromotedFrom(n)
			switch {
			case isCopy || len(keys) < 2:
				out[n.ID] = base
			case n.Type == types.NodeExternal || n.FilePath == types.ExternalFilePath:
				out[n.ID] = base
				if n.CanonicalID != "" && cids[n.CanonicalID] == 1 {
					out[n.ID] = n.CanonicalID
				}
			default:
				name := siteName(base, n)
				if atPlace[n.FilePath+lineMark+strconv.Itoa(n.StartLine)] > 1 {
					name += ordinalMark + strconv.Itoa(ordinalOf(n))
				}
				out[n.ID] = name
			}
		}
	}
	return out
}

// NameOf returns the name a tool prints for n (see printedNames); it is
// PrintedName with an error result for callers that report one.
func NameOf(r StoreReader, n types.Node) (string, error) {
	return PrintedName(r, n), nil
}

// ReaderUnwrapper is implemented by a reader that wraps another one, so a
// cache keyed by reader (PrintedName) is shared by the wrappers of one store.
type ReaderUnwrapper interface {
	UnwrapReader() StoreReader
}

// printedIndexes holds, per store, the printed names that differ from
// PrintedQName (the nodes whose qualified name others carry).
var printedIndexes sync.Map // StoreReader -> *printedIndex

type printedIndex struct {
	once  sync.Once
	names map[string]string
	err   error
}

// PrintedName returns the name a tool prints for n, as NameOf, from an index
// of the store's duplicated names built once per store (the graph a server
// reads does not change while it runs).
func PrintedName(r StoreReader, n types.Node) string {
	key := r
	for {
		u, ok := key.(ReaderUnwrapper)
		if !ok {
			break
		}
		key = u.UnwrapReader()
	}
	v, _ := printedIndexes.LoadOrStore(key, &printedIndex{})
	ix := v.(*printedIndex)
	ix.once.Do(func() { ix.names, ix.err = buildPrintedIndex(key) })
	if name, ok := ix.names[n.ID]; ok {
		return name
	}
	return PrintedQName(n)
}

func buildPrintedIndex(r StoreReader) (map[string]string, error) {
	all, err := r.AllNodes()
	if err != nil {
		return nil, fmt.Errorf("printed names: %w", err)
	}
	byQName := map[string][]types.Node{}
	for _, n := range all {
		byQName[n.QualifiedName] = append(byQName[n.QualifiedName], n)
	}
	out := map[string]string{}
	for _, group := range byQName {
		if len(group) < 2 {
			continue
		}
		for id, name := range printedNames(group) {
			out[id] = name
		}
	}
	return out, nil
}

// splitOrdinal removes "#<n>" from the end of the place of a site name.
func splitOrdinal(place string) (string, int) {
	at := strings.LastIndex(place, ordinalMark)
	if at < 0 {
		return place, 0
	}
	i, err := strconv.Atoi(place[at+len(ordinalMark):])
	if err != nil || i < 0 {
		return place, 0
	}
	return place[:at], i
}
