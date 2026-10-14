package ckgclient

import (
	"context"
	"fmt"
	"sort"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type SymbolIdentity struct {
	Symbol      string            `json:"symbol"`
	CanonicalID string            `json:"canonical_id,omitempty"`
	Citation    contract.Citation `json:"citation"`
}

// CitationResolver is optional for old/dummy providers. Unavailable identity
// must be disclosed, rather than guessed from a filename or token.
type CitationResolver interface {
	ResolveCitation(context.Context, contract.Citation) (SymbolIdentity, error)
}

func (r *Real) ResolveCitation(ctx context.Context, c contract.Citation) (SymbolIdentity, error) {
	if err := ctx.Err(); err != nil {
		return SymbolIdentity{}, err
	}
	if !c.IsValid() {
		return SymbolIdentity{}, fmt.Errorf("invalid citation")
	}
	head, err := r.commit()
	if err != nil {
		return SymbolIdentity{}, err
	}
	if c.CommitHash != "" && head != c.CommitHash {
		return SymbolIdentity{}, fmt.Errorf("snapshot_mismatch")
	}
	nodes, err := r.s.NodesByFilePath(c.File)
	if err != nil {
		return SymbolIdentity{}, err
	}
	eligible := make([]types.Node, 0)
	for _, n := range nodes {
		if isStructuralQname(n.QualifiedName) || !declaredSymbol(n.Type) || n.FilePath != c.File || n.StartLine < 1 || n.EndLine < n.StartLine {
			continue
		}
		if (n.StartLine <= c.StartLine && c.EndLine <= n.EndLine) || (c.StartLine <= n.StartLine && n.EndLine <= c.EndLine) {
			eligible = append(eligible, n)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		exactA, exactB := a.StartLine == c.StartLine && a.EndLine == c.EndLine, b.StartLine == c.StartLine && b.EndLine == c.EndLine
		if exactA != exactB {
			return exactA
		}
		containsA, containsB := a.StartLine <= c.StartLine && c.EndLine <= a.EndLine, b.StartLine <= c.StartLine && c.EndLine <= b.EndLine
		if containsA != containsB {
			return containsA
		}
		return a.EndLine-a.StartLine < b.EndLine-b.StartLine
	})
	if len(eligible) == 0 {
		return SymbolIdentity{}, ErrSeedUnresolved
	}
	n := eligible[0]
	if (n.StartLine > c.StartLine || c.EndLine > n.EndLine) && len(eligible) > 1 {
		return SymbolIdentity{}, fmt.Errorf("%w: citation spans several symbols", ErrSeedUnresolved)
	}
	if len(eligible) > 1 && eligible[1].StartLine == n.StartLine && eligible[1].EndLine == n.EndLine && (eligible[1].CanonicalID != n.CanonicalID || eligible[1].QualifiedName != n.QualifiedName) {
		return SymbolIdentity{}, fmt.Errorf("%w: ambiguous citation identity", ErrSeedUnresolved)
	}
	name := n.CanonicalID
	if name == "" {
		name = n.QualifiedName
	}
	return SymbolIdentity{Symbol: name, CanonicalID: n.CanonicalID, Citation: nodeToCitation(n, head)}, nil
}

func declaredSymbol(kind types.NodeType) bool {
	switch kind {
	case types.NodeFunction, types.NodeMethod, types.NodeConstructor, types.NodeModifier,
		types.NodeStruct, types.NodeInterface, types.NodeClass, types.NodeTypeAlias,
		types.NodeEnum, types.NodeContract, types.NodeField, types.NodeConstant, types.NodeVariable:
		return true
	}
	return false
}
