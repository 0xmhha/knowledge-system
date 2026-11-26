package solidity

import (
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Sol HasLowLevelCall and HasValueTransfer
// presence markers on callables.
//
// Complement to the low-level call EdgeInvokes, which only fires when the
// receiver of a `.call` family invocation is a state-var / parameter
// of Contract or Interface type. Address-typed receivers (the common
// proxy / forwarder pattern), `address(x)` cast wrappers, and
// chained receivers all produce no edge but the markers light
// up so security tooling can run "which functions perform any
// low-level call shape" queries without dropping into AST analysis.
//
// Scope:
//   - HasLowLevelCall:   any `.call` / `.delegatecall` / `.staticcall`
//                        on any receiver shape. The low-level call walker still
//                        emits edges for resolvable receivers; this
//                        marker is a superset signal.
//   - HasValueTransfer:  any `.send` / `.transfer`. Sol semantics:
//                        ETH transfer with limited gas, not method
//                        dispatch — distinct from .call family even
//                        though the AST shape is identical.
//
// The walker is shape-only (no receiver resolution), so it triggers
// on any member_expression whose property text matches one of the
// six method names AND whose parent unwraps to a call_expression.
// It is the one walk that sees every low-level call, so it also hands
// each receiver to lowLevelReceiver, which decides the receiver shapes
// no other walker reads for HasExternalCall (see external_call.go).
// Yul-internal builtins (assembly delegatecall, sstore, ...) are
// out of scope here — they live under the HasAssembly marker and the
// Yul builtin / low-level call walkers.

func isValueTransferMethod(name string) bool {
	switch name {
	case "send", "transfer":
		return true
	}
	return false
}

// runLowLevelCallMarker walks every member_expression with a
// call-expression parent and property text in the low-level or
// value-transfer set. Resolves to the enclosing callable via
// nearestFunctionQnameAndStart and flips the corresponding marker
// on the matching node post-Pass-1.
//
// Same idiom as runAssemblyMarker: post-Pass-1 in-place
// mutation, so the four function-emit paths (runFunctionDecl /
// runConstructorDecl / runFallbackReceiveDecl / runDecl-for-modifier)
// don't need touching.
func (v *declVisitor) runLowLevelCallMarker() {
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

	hasLowLevel := map[string]bool{}
	hasValue := map[string]bool{}
	hasExternal := map[string]bool{}

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
			property := memberNode.ChildByFieldName("property")
			if property == nil || property.Kind() != "identifier" {
				continue
			}
			method := property.Utf8Text(v.src)
			isLow := isLowLevelMethod(method)
			isVal := isValueTransferMethod(method)
			if !isLow && !isVal {
				continue
			}
			// Must be the callee of a call, directly or under call
			// options (`x.call{value: v}("")`). The call-options form was
			// not recognised, so claimBurnRefund of GovMinter v2 had
			// neither marker.
			call := callOfCallee(&memberNode)
			if call == nil {
				continue
			}
			// A low-level call that passes value moves coins like send
			// and transfer do.
			if isLow && callSendsValue(&memberNode, v.src) {
				isVal = true
			}
			fnQ, fnStart, ok := nearestFunctionQnameAndStart(&memberNode, v.src)
			if !ok {
				continue
			}
			id := v.declID(fnQ, fnStart)
			if isLow {
				hasLowLevel[id] = true
				// Whether the call goes to an address is a question of
				// its receiver (see external_call.go).
				if v.lowLevelReceiver(id, &memberNode) {
					hasExternal[id] = true
				}
			}
			if isVal {
				hasValue[id] = true
			}
		}
	}

	if len(hasLowLevel) == 0 && len(hasValue) == 0 {
		return
	}
	for i := range v.nodes {
		if hasLowLevel[v.nodes[i].ID] {
			v.nodes[i].HasLowLevelCall = true
		}
		if hasValue[v.nodes[i].ID] {
			v.nodes[i].HasValueTransfer = true
		}
		if hasExternal[v.nodes[i].ID] {
			v.nodes[i].HasExternalCall = true
		}
	}
}

// optionValue is the call option that sends coins with a call.
const optionValue = "value"

// kindStructFieldAssignment is one `name: value` entry of call options.
const kindStructFieldAssignment = "struct_field_assignment"

// callSendsValue reports whether the call whose callee is member has a
// `value` call option (`x.call{value: v}(data)`).
func callSendsValue(member *sitter.Node, src []byte) bool {
	for cur := member.Parent(); cur != nil && cur.Kind() != kindCallExpression; cur = cur.Parent() {
		if cur.Kind() != kindStructExpression {
			continue
		}
		for i := uint(0); i < cur.NamedChildCount(); i++ {
			option := cur.NamedChild(i)
			if option == nil || option.Kind() != kindStructFieldAssignment {
				continue
			}
			if name := option.ChildByFieldName(fieldName); name != nil && name.Utf8Text(src) == optionValue {
				return true
			}
		}
	}
	return false
}
