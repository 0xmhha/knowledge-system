package solidity

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Sol low-level call dispatch detection.
//
// Detects: `target.call(...)`, `target.delegatecall(...)`,
// `target.staticcall(...)` where `target` is a bare identifier
// resolvable via lookupReceiverType (state-var / param / local-var).
//
// Scope (per design doc §2.1):
//   - Bare identifier receivers only. `address(x)` cast / chained
//     expressions drop.
//   - 3 method names: call / delegatecall / staticcall. Value-transfer
//     primitives (send / transfer) are left to the HasValueTransfer
//     marker.
//   - Resolver: receiver type → byName[NodeContract|NodeInterface] →
//     Dst. ConfAmbiguous regardless of file boundary (same as interface
//     dispatch).
//
// Re-uses every infrastructure piece:
//   - `unwrapExpression` + `member.ChildByFieldName("object"|"property")`
//     from using_for.go matchStateVarMethodCall.
//   - The using-for lookupReceiverType chain (resolveLowLevelCallRef).
//   - PendingRef.DispatchKind routing.
//
// dispatchKindLowLevelCall — tags PendingRefs from this walker so the
// Pass 2 dispatch switch routes them to resolveLowLevelCallRef.
const dispatchKindLowLevelCall = "low_level_call"

// isLowLevelMethod returns true iff name matches one of the three
// primitives. send / transfer (value-transfer) intentionally excluded
// — different semantics (no method dispatch, just balance change) and
// warrant a separate edge type (EdgeTransfersValue is a candidate).
func isLowLevelMethod(name string) bool {
	switch name {
	case "call", "delegatecall", "staticcall":
		return true
	}
	return false
}

// runLowLevelCalls walks every member_expression that fits the
// `<identifier>.<call|delegatecall|staticcall>(...)` shape and queues a
// PendingRef tagged dispatchKindLowLevelCall. The Pass 2 resolver looks
// up the receiver identifier's type and emits EdgeInvokes when it
// resolves to a Contract or Interface.
//
// Same overall idiom as runUsingForCalls — query every member_expression,
// filter by AST shape in Go, encode (receiver|method) in TargetQName.
// Anchored to the enclosing function via nearestFunctionQnameAndStart
// for SrcID consistency with the function node hash.
func (v *declVisitor) runLowLevelCalls() {
	const query = `(member_expression) @member`
	q, qErr := sitter.NewQuery(v.lang, query)
	if qErr != nil {
		return
	}
	defer func() { q.Close() }()
	cur := sitter.NewQueryCursor()
	defer func() { cur.Close() }()
	matches := cur.Matches(q, v.root, v.src)
	names := q.CaptureNames()
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, c := range m.Captures {
			if names[c.Index] != "member" {
				continue
			}
			memberNode := c.Node
			receiverName, methodName, ok := matchStateVarMethodCall(&memberNode, v.src)
			if !ok {
				continue
			}
			if !isLowLevelMethod(methodName) {
				continue
			}
			fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
			if !fnOK {
				// Top-level expression (free functions don't have low-
				// level calls in practice; drop defensively).
				continue
			}
			v.pending = append(v.pending, parse.PendingRef{
				SrcID:        parse.MakeID(fnQ, "sol", fnStart),
				EdgeType:     types.EdgeInvokes,
				TargetQName:  receiverName + "|" + methodName,
				Line:         int(memberNode.StartPosition().Row) + 1,
				ByteOffset:   int(memberNode.StartByte()),
				DispatchKind: dispatchKindLowLevelCall,
			})
		}
	}
}
