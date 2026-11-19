package golang

import (
	"fmt"
	"reflect"

	"github.com/0xmhha/knowledge-system/internal/graph/contract"
	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// ToUnit turns the Go pipeline's resolved graph into contract records. layer
// gives the layer of a file. It fails, rather than dropping data, when a node
// or an edge carries a field the records do not hold (the Solidity markers,
// scores computed after the build) or a confidence without a contract value.
func ToUnit(rg *parse.ResolvedGraph, layer func(file string) contract.Layer) (contract.Unit, error) {
	u := contract.Unit{
		Language:          "go",
		Decls:             make([]contract.Decl, 0, len(rg.Nodes)),
		Refs:              make([]contract.Ref, 0, len(rg.Edges)),
		UnresolvedCounts:  rg.Unresolved,
		UnresolvedSamples: rg.UnresolvedSamples,
		LoadErrors:        rg.LoadErrors,
	}
	for i := range rg.Nodes {
		d, err := declOf(&rg.Nodes[i], layer)
		if err != nil {
			return contract.Unit{}, err
		}
		u.Decls = append(u.Decls, d)
	}
	for i := range rg.Edges {
		r, err := refOf(rg.Edges[i])
		if err != nil {
			return contract.Unit{}, err
		}
		u.Refs = append(u.Refs, r)
	}
	return u, nil
}

// FromUnit turns contract records back into a resolved graph, the form the
// pipeline merges and stores.
func FromUnit(u contract.Unit) *parse.ResolvedGraph {
	rg := &parse.ResolvedGraph{
		Nodes:             make([]types.Node, 0, len(u.Decls)),
		Edges:             make([]types.Edge, 0, len(u.Refs)),
		Unresolved:        u.UnresolvedCounts,
		UnresolvedSamples: u.UnresolvedSamples,
		LoadErrors:        u.LoadErrors,
	}
	for _, d := range u.Decls {
		rg.Nodes = append(rg.Nodes, types.Node{
			ID:            d.NodeID,
			Type:          d.LangKind,
			Name:          d.Name,
			QualifiedName: d.QualifiedName,
			CanonicalID:   contract.Identity(d.Language, d.SymbolID),
			FilePath:      d.File,
			StartLine:     d.Span.StartLine,
			EndLine:       d.Span.EndLine,
			StartByte:     d.Span.StartByte,
			EndByte:       d.Span.EndByte,
			Language:      d.Language,
			Visibility:    d.Visibility,
			Signature:     d.Signature,
			DocComment:    d.Doc,
			Complexity:    d.Complexity,
			Confidence:    d.Confidence.Stored(),
			SubKind:       d.SubKind,
			External:      d.External,
		})
	}
	for _, r := range u.Refs {
		rg.Edges = append(rg.Edges, types.Edge{
			Src:          r.From,
			Dst:          r.To,
			Type:         r.Kind,
			FilePath:     r.File,
			Line:         r.Line,
			Count:        r.Count,
			Confidence:   r.Confidence.Stored(),
			DispatchKind: r.DispatchKind,
			Order:        r.Order,
		})
	}
	return rg
}

func declOf(n *types.Node, layer func(string) contract.Layer) (contract.Decl, error) {
	conf, ok := contract.ConfidenceOf(n.Confidence)
	if !ok {
		return contract.Decl{}, fmt.Errorf("contract: node %s (%s): confidence %q has no contract value", n.ID, n.QualifiedName, n.Confidence)
	}
	// Every field the record holds is cleared on a copy; anything left is a
	// field the record would lose. Comparing against the zero value also
	// catches fields added to types.Node later.
	rest := *n
	rest.ID, rest.Type, rest.Name, rest.QualifiedName, rest.CanonicalID = "", "", "", "", ""
	rest.FilePath, rest.Language, rest.Visibility, rest.Signature, rest.DocComment, rest.SubKind = "", "", "", "", "", ""
	rest.StartLine, rest.EndLine, rest.StartByte, rest.EndByte, rest.Complexity = 0, 0, 0, 0, 0
	rest.Confidence = ""
	rest.External = nil
	// Scores are not frontend output: score.Compute sets them at the end of
	// every build. Nodes an incremental build reloads from the store still
	// hold the previous build's scores; they are not carried, so a reloaded
	// node enters the build as a parsed one does.
	rest.InDegree, rest.OutDegree, rest.PageRank, rest.UsageScore = 0, 0, 0, 0
	// The store reads absent lists back as empty ones.
	if len(rest.YulBuiltins) == 0 {
		rest.YulBuiltins = nil
	}
	if len(rest.RecentPRs) == 0 {
		rest.RecentPRs = nil
	}
	if !reflect.DeepEqual(rest, types.Node{}) {
		return contract.Decl{}, fmt.Errorf("contract: node %s (%s) has fields the records do not hold: %+v", n.ID, n.QualifiedName, rest)
	}
	return contract.Decl{
		NodeID:        n.ID,
		SymbolID:      contract.SymbolID(n.Language, n.CanonicalID),
		Language:      n.Language,
		Kind:          contract.KindOf(n.Type),
		LangKind:      n.Type,
		SubKind:       n.SubKind,
		Name:          n.Name,
		QualifiedName: n.QualifiedName,
		File:          n.FilePath,
		Span:          contract.Span{StartLine: n.StartLine, EndLine: n.EndLine, StartByte: n.StartByte, EndByte: n.EndByte},
		Layer:         layer(n.FilePath),
		Signature:     n.Signature,
		Doc:           n.DocComment,
		Visibility:    n.Visibility,
		Complexity:    n.Complexity,
		Confidence:    conf,
		External:      n.External,
	}, nil
}

func refOf(e types.Edge) (contract.Ref, error) {
	conf, ok := contract.ConfidenceOf(e.Confidence)
	if !ok {
		return contract.Ref{}, fmt.Errorf("contract: edge %s -%s-> %s: confidence %q has no contract value", e.Src, e.Type, e.Dst, e.Confidence)
	}
	if e.ID != 0 {
		return contract.Ref{}, fmt.Errorf("contract: edge %s -%s-> %s already has a store id %d", e.Src, e.Type, e.Dst, e.ID)
	}
	return contract.Ref{
		From:         e.Src,
		To:           e.Dst,
		Kind:         e.Type,
		File:         e.FilePath,
		Line:         e.Line,
		Count:        e.Count,
		Confidence:   conf,
		DispatchKind: e.DispatchKind,
		Order:        e.Order,
	}, nil
}
