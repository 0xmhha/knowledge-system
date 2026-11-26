package solidity

import (
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Resolution of member accesses and type uses. Before it, only `defines`
// reached an enum value, a struct member, a struct or an enum: the system
// contracts name `ProposalStatus.X` and `ExecutionCheckResult.X` at 36 sites
// and none had an edge.

// mappingArrow separates the key type and the value type of a mapping type.
const mappingArrow = "=>"

// accessState is what a prefix of an access path stands for: a value of a
// declared type, or a declaration that is not a value (a container or an
// enum, as in `Shapes.Kind.Circle`).
type accessState struct {
	// decl is the container or enum the path names; nil for a value.
	decl *types.Node
	// typeName is the declared type of the value, as written where it was
	// declared; containerID and file are the scope it was written in.
	typeName    string
	containerID string
	file        string
}

// resolveMember emits the edge of one member access: reads_field or
// writes_field to the struct member, enum value, constant or state variable
// the access names. An access that names none of them (`list.length`,
// `x.selector`, a member of a type the graph does not declare) is not a use
// of a declaration and is dropped.
func (r *scopedResolver) resolveMember(pr parse.PendingRef, path string, write bool) {
	root, steps := splitAccessPath(path)
	state, ok := r.rootState(pr, root)
	for _, step := range steps {
		if !ok {
			return
		}
		state, ok = r.advance(state, step)
	}
	if !ok {
		return
	}
	target := r.memberOf(state, pr.TargetQName)
	if target == nil || !isStorageType(target.Type) {
		// An enum reached through its container (`Shapes.Kind`) is a
		// step towards a value, not a value that is read.
		return
	}
	r.edge(pr, accessEdgeType(target.Type, write), target.ID)
}

// splitAccessPath splits a path made by accessPath into its root name and
// its steps: "[]" for an index, a member name otherwise.
func splitAccessPath(path string) (root string, steps []string) {
	end := strings.IndexAny(path, pathMember+pathIndex)
	if end < 0 {
		return path, nil
	}
	root, rest := path[:end], path[end:]
	for rest != "" {
		if strings.HasPrefix(rest, pathIndex) {
			steps = append(steps, pathIndex)
			rest = rest[len(pathIndex):]
			continue
		}
		rest = rest[len(pathMember):]
		next := strings.IndexAny(rest, pathMember+pathIndex)
		if next < 0 {
			next = len(rest)
		}
		steps = append(steps, rest[:next])
		rest = rest[next:]
	}
	return root, steps
}

// rootState resolves the first name of an access path: a local variable or
// a parameter of the callable (its declared type), a state variable, a
// constant or a mapping in scope (its declared type), or a container or an
// enum in scope.
func (r *scopedResolver) rootState(pr parse.PendingRef, name string) (accessState, bool) {
	containerID, file := r.scope.contextOf(pr.SrcID)
	if typeName, isLocal := lookupReceiver(name, "", pr.SrcID, pr.Line, pr.ByteOffset,
		nil, r.paramTypes, r.localVarTypes); isLocal {
		return accessState{typeName: typeName, containerID: containerID, file: file}, true
	}
	ids := r.scope.lookup(containerID, file, name, anyType)
	if len(ids) == 0 {
		return accessState{}, false
	}
	return r.stateOf(r.scope.node[ids[0]])
}

// stateOf turns a declaration an access path reached into the state to go
// on from: a value for a state variable, a constant, a mapping or a struct
// member; the declaration itself for a container or an enum.
func (r *scopedResolver) stateOf(n *types.Node) (accessState, bool) {
	switch {
	case isStorageType(n.Type):
		containerID, file := r.scope.contextOf(n.ID)
		return accessState{typeName: n.Signature, containerID: containerID, file: file}, true
	case isContainerType(n.Type), n.Type == types.NodeEnum:
		return accessState{decl: n}, true
	}
	return accessState{}, false
}

// advance applies one step of an access path.
func (r *scopedResolver) advance(state accessState, step string) (accessState, bool) {
	if step != pathIndex {
		if next := r.memberOf(state, step); next != nil {
			return r.stateOf(next)
		}
		return accessState{}, false
	}
	if state.decl != nil {
		return accessState{}, false
	}
	element, ok := elementType(state.typeName)
	state.typeName = element
	return state, ok
}

// memberOf returns the declaration `name` means after state: a member of
// the struct a value has as its type, a value of an enum, or a member of a
// container (a constant, a state variable, an enum). nil when there is none.
func (r *scopedResolver) memberOf(state accessState, name string) *types.Node {
	switch {
	case state.decl == nil:
		structID := r.scope.structOf(state.containerID, state.file, state.typeName)
		return r.scope.node[r.scope.children[structID][name]]
	case state.decl.Type == types.NodeEnum:
		return r.scope.node[r.scope.children[state.decl.ID][name]]
	}
	ids := r.scope.lookupInherited(r.scope.linearisation(state.decl.ID), name, func(t types.NodeType) bool {
		return isStorageType(t) || t == types.NodeEnum
	})
	if len(ids) == 0 {
		return nil
	}
	return r.scope.node[ids[0]]
}

// elementType returns the type an index into a value of typeName gives: the
// value type of a mapping (`mapping(K => V)`), the element type of an array
// (`T[]`, `T[3]`). ok is false for any other type.
func elementType(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if strings.HasPrefix(typeName, mappingKeyword) {
		open := strings.IndexByte(typeName, '(')
		if open < 0 {
			return "", false
		}
		end := matchingParen(typeName, open)
		arrow := topLevelIndex(typeName[open+1:max(end, open+1)], mappingArrow)
		if end < 0 || arrow < 0 {
			return "", false
		}
		return strings.TrimSpace(typeName[open+1+arrow+len(mappingArrow) : end]), true
	}
	if strings.HasSuffix(typeName, "]") {
		if open := strings.LastIndexByte(typeName, '['); open > 0 {
			return strings.TrimSpace(typeName[:open]), true
		}
	}
	return "", false
}

// topLevelIndex returns the index of the first sep in text that is outside
// any parentheses, or -1.
func topLevelIndex(text, sep string) int {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && strings.HasPrefix(text[i:], sep) {
			return i
		}
	}
	return -1
}

// resolveTypeUse emits a uses_type edge from the declaration that names a
// type to the struct, enum, contract or interface it names. A name that is
// none of them (a user-defined value type, a type of a file outside the
// graph) is dropped.
func (r *scopedResolver) resolveTypeUse(pr parse.PendingRef) {
	containerID, file := r.scope.contextOf(pr.SrcID)
	if id := r.scope.typeDeclaration(containerID, file, pr.TargetQName); id != "" {
		r.edge(pr, types.EdgeUsesType, id)
	}
}
