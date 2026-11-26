package solidity

import (
	"strconv"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Reference kinds: the DispatchKind of the pending references Pass 1 queues
// for Pass 2 to resolve in the scope of the contract that makes them. Each
// kind names what the source says; the scope lookup of Pass 2 decides the
// target. They are stored with the pending reference, so a partial rebuild
// resolves them the same way.
//
// The target of a reference is a plain name wherever it can be. An
// incremental build re-resolves the references of an unchanged file only
// when one of their targets equals the name of a node in a changed file;
// a target that packs other data with the name would never match, and the
// unchanged file's edges into the changed file would be lost.
const (
	// refEmit: `emit Name(...)`. Target: the event name.
	refEmit = "sol_emit"
	// refModifier: a modifier invocation in a function header. Target: the
	// modifier name. In a constructor header the name can be a base contract.
	refModifier = "sol_modifier"
	// refNameCall: `name(args)`. Target: the name. The kind carries the
	// number of arguments (`sol_name_call|2`), so the target stays a plain
	// name. The name can be a function (calls) or a custom error
	// (reverts_with).
	refNameCall = "sol_name_call"
	// refImport: a file imports another file. Target: the qualified name of
	// the imported File node. It makes no edge (the Import node and its
	// link come from nodes); it records the dependency, see below.
	refImport = "sol_import"
	// refRevert: `revert Name(...)`. Target: the error name, with its
	// qualifier when written (`Lib.Name`).
	refRevert = "sol_revert"
	// refRead: a value identifier that is read. Target: the name.
	refRead = "sol_read"
	// refWrite: a value identifier that is assigned, deleted, incremented or
	// pushed to, directly or through an index or a member. Target: the name.
	refWrite = "sol_write"
	// refMemberRead, refMemberWrite: a member access that is read or
	// written (`item.owner`, `Phase.Open`). Target: the member's name. The
	// kind carries the path of the expression the member is taken from
	// (`sol_member_read|items[]`).
	refMemberRead  = "sol_member_read"
	refMemberWrite = "sol_member_write"
	// refTypeUse: a user-defined type named as a type. Target: the type
	// name as written (`Item`, `Shapes.Box`).
	refTypeUse = "sol_type_use"
	// refEncodeCall, refSelector: a function named as the target of an
	// encoded call (`abi.encodeCall(I.fn, ...)`) or by its selector
	// (`I.fn.selector`). Target: the function as written (`I.fn`).
	refEncodeCall = "sol_encode_call"
	refSelector   = "sol_selector"
	// refUnfollowedPointer: a write through a local storage pointer whose
	// binding could not be followed (the function that returns it is
	// declared in another file). Target: the pointer's name. It makes no
	// edge; Pass 2 counts it.
	refUnfollowedPointer = "sol_unfollowed_pointer"
	// refImplicitOverride: a function declared without `override` in a
	// contract that has bases. Target: the function name. Pass 2 links it to
	// the interface function it implements, if any.
	refImplicitOverride = "sol_implicit_override"
)

// refSep separates the parts of a pending reference's target.
const refSep = "|"

// queue appends a pending reference from srcID, located at the line of at.
func (v *declVisitor) queue(srcID string, et types.EdgeType, kind, target string, at *sitter.Node) {
	v.pending = append(v.pending, parse.PendingRef{
		SrcID:        srcID,
		EdgeType:     et,
		TargetQName:  target,
		Line:         int(at.StartPosition().Row) + 1,
		ByteOffset:   int(at.StartByte()),
		DispatchKind: kind,
	})
}

// arityKind returns a reference kind that carries the number of arguments
// of the call it describes (`sol_name_call|2`).
func arityKind(base string, argc int) string {
	return base + refSep + strconv.Itoa(argc)
}

// arityOfKind splits a kind made by arityKind. ok is false when kind does not
// start with base. argc is unknownArity for a bare base kind, as stored by a
// graph built before the count was recorded.
func arityOfKind(kind, base string) (argc int, ok bool) {
	if kind == base {
		return unknownArity, true
	}
	prefix, count, found := strings.Cut(kind, refSep)
	if !found || prefix != base {
		return 0, false
	}
	argc, err := strconv.Atoi(count)
	return argc, err == nil
}

// unknownArity is the argument count of a call whose reference does not
// carry one. No overload is ruled out for it.
const unknownArity = -1
