package solidity

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// What HasExternalCall says.
//
// The marker is true on a function, modifier, constructor, fallback or
// receive that makes a low-level call (`call`, `delegatecall`, `staticcall`,
// or their Yul forms) to an address: the code that runs is whatever lives at
// that address, and nothing in the source names it. It is the "call to an
// arbitrary address" a reentrancy review starts from.
//
// It is not the marker of every call that leaves the contract:
//   - A call through a variable or a cast typed as an interface or a
//     contract (`token.mint(x)`, `IToken(addr).mint(x)`) names its target.
//     It is an invokes edge to the method, and the marker stays false.
//   - A low-level call on the contract itself (`address(this).call(...)`)
//     re-enters the same code: HasSelfReentrantCall.
//   - `send` and `transfer` move value and run no chosen code:
//     HasValueTransfer. A low-level call with a `value` option sets both.
//
// The receiver of a low-level call decides, and each shape of receiver has
// one place that reads it:
//   - a variable (`target.call`): Pass 2, by its declared type
//     (resolveLowLevelCallRefExt);
//   - a member or an element of a variable (`route.hop.call`,
//     `targets[i].call`): Pass 2, by the type at the end of the path
//     (resolveAddressReceiver);
//   - a global address (`msg.sender.call`): Pass 1, this file;
//   - a cast (`address(x).call`, `payable(x).call`): Pass 1
//     (runExternalCallCastMarker);
//   - a function result (`getTarget().call`): Pass 2, by the return type
//     (the chained external call references);
//   - a Yul call: Pass 2, by the type of the target variable
//     (runYulLowLevelCalls).
//
// The member and global shapes were read by nothing. GovMinter v2's
// `claimBurnRefund` sends value with `msg.sender.call{value: amount}("")`
// and had the low-level and value markers without this one.

// Elementary address types.
const (
	typeAddress        = "address"
	typeAddressPayable = "address payable"
)

// isAddressType reports whether a declared type is an address.
func isAddressType(typeName string) bool {
	switch strings.TrimSpace(typeName) {
	case typeAddress, typeAddressPayable:
		return true
	}
	return false
}

// globalAddresses are the address values the language provides.
var globalAddresses = map[string]bool{
	"msg.sender": true, "tx.origin": true, "block.coinbase": true,
}

// lowLevelReceiver looks at the receiver of the low-level call whose callee
// is member, made in the callable srcID. It reports whether the receiver is
// a global address, which makes the call an external call at once. For a
// member or an element of a variable it queues a reference that Pass 2
// decides by the type the path ends at. Every other shape is read elsewhere
// (see the list above) and yields false.
func (v *declVisitor) lowLevelReceiver(srcID string, member *sitter.Node) (external bool) {
	// The grammar puts a prefix or binary operator around the receiver
	// instead of around the call (see receiverOperand).
	receiver := receiverOperand(unwrapExpression(member.ChildByFieldName(fieldObject)), v.src)
	if receiver == nil {
		return false
	}
	if globalAddresses[dottedName(receiver, v.src)] {
		return true
	}
	path, ok := accessPath(receiver, v.src)
	if !ok {
		return false
	}
	// A path without a step is a variable: read by its own reference.
	if root, steps := splitAccessPath(path); len(steps) > 0 {
		v.queue(srcID, types.EdgeInvokes, refAddressReceiver+refSep+path, root, member)
	}
	return false
}

// resolveAddressReceiver marks the callable of a low-level call whose
// receiver, a member or an element of a variable, is an address.
func (r *scopedResolver) resolveAddressReceiver(pr parse.PendingRef, path string) {
	if state, ok := r.stateAt(pr, path); ok && state.decl == nil && isAddressType(state.typeName) {
		r.externalCallers[pr.SrcID] = true
	}
}
