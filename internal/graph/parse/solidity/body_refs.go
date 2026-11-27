package solidity

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Tree-sitter node kinds and fields the body walker reads.
const (
	kindIdentifier           = "identifier"
	kindExpression           = "expression"
	kindParameter            = "parameter"
	kindVariableDeclaration  = "variable_declaration"
	kindCallExpression       = "call_expression"
	kindCallArgument         = "call_argument"
	kindMemberExpression     = "member_expression"
	kindArrayAccess          = "array_access"
	kindSliceAccess          = "slice_access"
	kindParenthesized        = "parenthesized_expression"
	kindTupleExpression      = "tuple_expression"
	kindUnaryExpression      = "unary_expression"
	kindBinaryExpression     = "binary_expression"
	kindUpdateExpression     = "update_expression"
	kindAssignment           = "assignment_expression"
	kindAugmentedAssignment  = "augmented_assignment_expression"
	kindStructExpression     = "struct_expression"
	kindRevertStatement      = "revert_statement"
	kindEmitStatement        = "emit_statement"
	kindAssemblyStatement    = "assembly_statement"
	kindUserDefinedType      = "user_defined_type"
	kindStateVariableDecl    = "state_variable_declaration"
	kindConstantVariableDecl = "constant_variable_declaration"
	fieldValue               = "value"
	kindTypeName             = "type_name"
	fieldFunction            = "function"
	fieldObject              = "object"
	fieldProperty            = "property"
	fieldBase                = "base"
	fieldLeft                = "left"
	fieldOperator            = "operator"
	fieldArgument            = "argument"
	fieldRight               = "right"
	fieldError               = "error"
	opDelete                 = "delete"
)

// builtinValues are the global names that never are a state variable. They
// are left out so the pending references hold only names worth resolving.
var builtinValues = map[string]bool{
	"msg": true, "block": true, "tx": true, "abi": true, "this": true,
	"super": true, "_": true, "now": true, "type": true,
}

// arrayMutators are the member functions that change a storage array.
var arrayMutators = map[string]bool{"push": true, "pop": true}

// runBodyRefs walks every function, modifier, constructor, fallback and
// receive and queues what its header arguments and its body say: calls by
// name, value identifiers read or written, and reverts with a named error.
// Pass 2 resolves each name in the scope of the enclosing contract.
//
// Before this walk the graph of the system contracts had no edge for about
// 175 internal calls `foo()`, about 300 state variable reads and 34
// non-mapping writes, any mapping read, 144 reverts with a custom error, and
// mapping writes other than `name[k] op= v` (8 of 78 sites had an edge).
func (v *declVisitor) runBodyRefs() {
	if v.root == nil {
		return
	}
	pointers := newPointerAnalysis(v)
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		switch n.Kind() {
		case kindFunctionDef, kindModifierDef, kindConstructorDef, kindFallbackReceiveDef:
			if id, ok := v.callableIDAt(n); ok {
				w := &bodyWalker{v: v, srcID: id, locals: map[string]bool{}, pointers: pointers.of(n)}
				collectLocalNames(n, v.src, w.locals)
				w.walkCallable(n)
			}
			return
		case kindStateVariableDecl, kindConstantVariableDecl:
			// The initialiser of a state variable or a constant: the
			// variable is the one that reads what it names. `memberVersion
			// = INITIAL_MEMBER_VERSION` was not walked, so the constant had
			// no reader.
			if value := n.ChildByFieldName(fieldValue); value != nil {
				if id, ok := v.variableIDAt(n); ok {
					(&bodyWalker{v: v, srcID: id, locals: map[string]bool{}}).walk(value)
				}
			}
			return
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if c := n.NamedChild(i); c != nil {
				visit(c)
			}
		}
	}
	visit(v.root)
}

// bodyWalker queues the references of one callable.
type bodyWalker struct {
	v     *declVisitor
	srcID string
	// locals holds the names the callable declares itself: parameters, named
	// returns and local variables. A use of such a name is not a use of a
	// state variable or of a function. Block scope is not tracked: a name
	// declared anywhere in the callable counts for all of it, which only
	// differs when a local shadows a state variable that an earlier
	// statement of the same callable used.
	locals map[string]bool
	// pointers tells what the locals declared `storage` point to. It is nil
	// for an initialiser, which declares no locals.
	pointers *callablePointers
}

// walkCallable walks the modifier arguments of the header and the body. The
// parameter list, the return list and the callable's own name hold no uses.
func (w *bodyWalker) walkCallable(decl *sitter.Node) {
	for i := uint(0); i < decl.NamedChildCount(); i++ {
		c := decl.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == kindModifierInvocation {
			w.walkChildren(c)
		}
	}
	if body := decl.ChildByFieldName(fieldBody); body != nil {
		w.walk(body)
	}
}

// walkChildren walks every named child of n.
func (w *bodyWalker) walkChildren(n *sitter.Node) {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil {
			w.walk(c)
		}
	}
}

// walk visits one node of a callable.
func (w *bodyWalker) walk(n *sitter.Node) {
	switch n.Kind() {
	case kindAssemblyStatement, kindTypeName, kindUserDefinedType:
		// Inline assembly is Yul, with its own names. A type expression
		// names a type, not a value.
		return
	case kindIdentifier:
		w.valueIdentifier(n)
		return
	case kindRevertStatement:
		w.revert(n)
		return
	case kindEmitStatement:
		// The event name is queued by runEmits; the arguments hold uses,
		// and so does the qualifier of the name (`emit Lib.Done(x)`).
		w.qualifier(unwrapExpression(n.ChildByFieldName(fieldName)))
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if c := n.NamedChild(i); c != nil && c.Kind() == kindCallArgument {
				w.walk(c)
			}
		}
		return
	case kindCallExpression:
		w.call(n)
		return
	case kindMemberExpression:
		w.member(n)
	}
	w.walkChildren(n)
}

// member queues the member an access names (`item.owner`, `Phase.Open`,
// `Shapes.LIMIT`): the target is the member's name and the kind carries the
// path of the expression it is taken from, so Pass 2 can find the struct,
// enum or contract that declares the member. The callee of a call
// (`token.mint(x)`) is a function, handled as a call.
func (w *bodyWalker) member(n *sitter.Node) {
	property := n.ChildByFieldName(fieldProperty)
	if property == nil || callOfCallee(n) != nil {
		return
	}
	if property.Utf8Text(w.v.src) == memberSelector {
		// `I.fn.selector` names the function I.fn.
		if fn := dottedName(unwrapExpression(n.ChildByFieldName(fieldObject)), w.v.src); fn != "" {
			w.v.queue(w.srcID, types.EdgeReferences, refSelector, fn, n)
		}
		return
	}
	path, ok := accessPath(n.ChildByFieldName(fieldObject), w.v.src)
	if !ok {
		return
	}
	base := refMemberRead
	if write, _ := writeTarget(n, w.v.src); write {
		base = refMemberWrite
	}
	w.v.queue(w.srcID, types.EdgeReadsField, base+refSep+path, property.Utf8Text(w.v.src), property)
}

// Names the language gives to the two ways of naming a function for a
// low-level call.
const (
	abiEncodeCall  = "abi.encodeCall"
	memberSelector = "selector"
)

// firstArgument returns the expression of the first argument of a call, or
// nil.
func firstArgument(call *sitter.Node) *sitter.Node {
	for i := uint(0); i < call.NamedChildCount(); i++ {
		if c := call.NamedChild(i); c != nil && c.Kind() == kindCallArgument {
			return unwrapExpression(firstNamedChild(c))
		}
	}
	return nil
}

// Steps of an access path.
const (
	pathIndex  = "[]"
	pathMember = "."
)

// accessPath describes the expression a member is taken from as its root
// name followed by its steps: `[]` for an index and `.name` for a member
// (`items[id].box` is `items[].box`). ok is false for an expression that is
// not such a chain (a call, a cast) or whose root is a built-in global
// (`msg.sender`). The operand a member applies to is found through the
// grammar's operator quirk (see receiverOperand).
func accessPath(n *sitter.Node, src []byte) (string, bool) {
	n = receiverOperand(unwrapExpression(n), src)
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case kindIdentifier:
		name := n.Utf8Text(src)
		return name, !builtinValues[name]
	case kindParenthesized:
		return accessPath(firstNamedChild(n), src)
	case kindMemberExpression:
		property := n.ChildByFieldName(fieldProperty)
		object, ok := accessPath(n.ChildByFieldName(fieldObject), src)
		if !ok || property == nil {
			return "", false
		}
		return object + pathMember + property.Utf8Text(src), true
	case kindArrayAccess:
		base, ok := accessPath(n.ChildByFieldName(fieldBase), src)
		return base + pathIndex, ok
	}
	return "", false
}

// argumentCount returns the number of arguments of a call expression.
func argumentCount(call *sitter.Node) int {
	count := 0
	for i := uint(0); i < call.NamedChildCount(); i++ {
		if c := call.NamedChild(i); c != nil && c.Kind() == kindCallArgument {
			count++
		}
	}
	return count
}

// call handles `callee(args)`. A bare name as callee is queued as a call by
// name; any other callee is walked like a value, so the receiver of
// `token.mint(x)` counts as a read and the one of `list.push(x)` as a write.
func (w *bodyWalker) call(n *sitter.Node) {
	callee := unwrapExpression(n.ChildByFieldName(fieldFunction))
	argc := argumentCount(n)
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil && c.Kind() == kindCallArgument {
			w.walk(c)
		}
	}
	if callee == nil {
		return
	}
	if dottedName(callee, w.v.src) == abiEncodeCall {
		// `abi.encodeCall(I.fn, (args))` names the function the encoded
		// call is for.
		if fn := dottedName(firstArgument(n), w.v.src); fn != "" {
			w.v.queue(w.srcID, types.EdgeReferences, refEncodeCall, fn, n)
		}
	}
	if callee.Kind() == kindIdentifier {
		name := callee.Utf8Text(w.v.src)
		if !w.locals[name] {
			w.v.queue(w.srcID, types.EdgeCalls, arityKind(refNameCall, argc), name, callee)
		}
		return
	}
	w.walk(callee)
}

// revert handles `revert Name(args);`: the error is queued by name and the
// arguments are walked for uses. `revert("text")` names no error.
func (w *bodyWalker) revert(n *sitter.Node) {
	if errExpr := unwrapExpression(n.ChildByFieldName(fieldError)); errExpr != nil {
		if name := dottedName(errExpr, w.v.src); name != "" {
			w.v.queue(w.srcID, types.EdgeRevertsWith, refRevert, name, errExpr)
		}
		w.qualifier(errExpr)
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c != nil && n.FieldNameForNamedChild(uint32(i)) != fieldError {
			w.walk(c)
		}
	}
}

// qualifier walks what stands before the last dot of a qualified name
// (`Lib` of `Lib.NotOwner`), where the name itself is queued by its own
// reference (a revert, an emit). The qualifier names a library or a
// contract, which the walk queues like any other identifier.
func (w *bodyWalker) qualifier(name *sitter.Node) {
	if name == nil || name.Kind() != kindMemberExpression {
		return
	}
	if object := name.ChildByFieldName(fieldObject); object != nil {
		w.walk(object)
	}
}

// dottedName returns `a` for an identifier and `a.b` for a member access on
// an identifier, and "" for any other expression.
func dottedName(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case kindIdentifier:
		return n.Utf8Text(src)
	case kindMemberExpression:
		object := unwrapExpression(n.ChildByFieldName(fieldObject))
		property := n.ChildByFieldName(fieldProperty)
		if object != nil && property != nil && object.Kind() == kindIdentifier {
			return object.Utf8Text(src) + qnameSep + property.Utf8Text(src)
		}
	}
	return ""
}

// valueIdentifier queues an identifier that stands for a value. Names in
// label positions (a declared name, a member name, a named argument) and
// names the callable declares itself are skipped. Pass 2 keeps the reference
// only when the name is a state variable, a constant or a mapping in scope.
func (w *bodyWalker) valueIdentifier(n *sitter.Node) {
	parent := n.Parent()
	if parent == nil {
		return
	}
	switch fieldOfChild(parent, n) {
	case fieldName, fieldProperty:
		return
	}
	if parent.Kind() == kindModifierInvocation {
		return
	}
	name := n.Utf8Text(w.v.src)
	if builtinValues[name] {
		return
	}
	write, rebind := writeTarget(n, w.v.src)
	kind := refRead
	if write {
		kind = refWrite
	}
	if w.locals[name] {
		// Binding a pointer again (`p = items[k]`) uses nothing it pointed
		// to; the right side is walked on its own.
		if !rebind {
			w.pointerAccess(n, name, kind)
		}
		return
	}
	w.v.queue(w.srcID, types.EdgeReadsField, kind, name, n)
}

// pointerAccess queues a use of the local name, when it is a storage
// pointer, as a use of each state variable it can point to: a read
// (`p.owner`, `list.length`, `f(p)`) or a write (`p.owner = x`), by kind. A
// pointer declared in the callable whose binding could not be followed is
// queued as such, so the use is counted instead of lost without a trace; a
// pointer that is a parameter is bound by the caller and queues nothing.
//
// Only writes used to go through here. GovBase reads through a pointer at 49
// places and none had an edge: `_addMember` and `_removeMember` showed no
// read of `versionedMemberList` at all.
func (w *bodyWalker) pointerAccess(at *sitter.Node, name, kind string) {
	if w.pointers == nil || !w.pointers.isPointer[name] {
		return
	}
	roots := w.pointers.roots[name]
	if len(roots) == 0 && !w.pointers.isParameter[name] {
		w.v.queue(w.srcID, types.EdgeReadsField, refUnfollowedPointer, name, at)
	}
	for _, root := range roots {
		w.v.queue(w.srcID, types.EdgeReadsField, kind, root, at)
	}
}

// fieldOfChild returns the field name under which child hangs on parent.
func fieldOfChild(parent, child *sitter.Node) string {
	for i := uint(0); i < parent.ChildCount(); i++ {
		c := parent.Child(i)
		if c != nil && c.StartByte() == child.StartByte() && c.EndByte() == child.EndByte() && c.Kind() == child.Kind() {
			return parent.FieldNameForChild(uint32(i))
		}
	}
	return ""
}

// writeTarget reports whether the identifier is the root of something that
// is assigned, deleted, incremented, decremented, or pushed to or popped
// from: `x = v`, `x[k] = v`, `x[a][b] += v`, `x[k].member = v`, `(x, y) =
// ...`, `delete x[k]`, `x++`, `x.push(v)`. An identifier inside an index
// (`m[x] = v`) or on the right side is read. rebind is true when the
// identifier itself is what an assignment replaces (`x = v`, `(x, y) = f()`)
// and not a part of it: for a storage pointer that binds the pointer again
// and writes nothing.
//
// The climb goes from the identifier up through the wrappers that keep it
// the root (an expression node, parentheses, a tuple, a member access on it,
// an index on it) and decides at the first other node. The grammar parses
// `delete x[k]` as an index on `delete x`, so the delete is met before the
// index; that is why delete is decided on the way up and not from the
// statement down.
func writeTarget(ident *sitter.Node, src []byte) (write, rebind bool) {
	cur := ident
	part := false // a member or an index of the identifier was crossed
	for {
		parent := cur.Parent()
		if parent == nil {
			return false, false
		}
		field := fieldOfChild(parent, cur)
		switch parent.Kind() {
		case kindExpression, kindParenthesized, kindTupleExpression:
		case kindMemberExpression:
			if field != fieldObject {
				return false, false
			}
			if isArrayMutatorCall(parent, src) {
				return true, false
			}
			part = true
		case kindArrayAccess, kindSliceAccess:
			if field != fieldBase {
				return false, false
			}
			part = true
		case kindUnaryExpression:
			op := parent.ChildByFieldName(fieldOperator)
			return op != nil && op.Utf8Text(src) == opDelete, false
		case kindUpdateExpression:
			return true, false
		case kindAssignment, kindAugmentedAssignment:
			return field == fieldLeft, field == fieldLeft && !part && parent.Kind() == kindAssignment
		default:
			return false, false
		}
		cur = parent
	}
}

// isArrayMutatorCall reports whether member is `x.push` or `x.pop` in the
// callee position of a call.
func isArrayMutatorCall(member *sitter.Node, src []byte) bool {
	property := member.ChildByFieldName(fieldProperty)
	if property == nil || !arrayMutators[property.Utf8Text(src)] {
		return false
	}
	return callOfCallee(member) != nil
}

// callOfCallee returns the call expression whose callee is n, looking
// through the expression wrapper and through call options
// (`f{value: 1}(x)`), or nil when n is not a callee.
func callOfCallee(n *sitter.Node) *sitter.Node {
	cur := n.Parent()
	for cur != nil && (cur.Kind() == kindExpression || cur.Kind() == kindStructExpression) {
		cur = cur.Parent()
	}
	if cur == nil || cur.Kind() != kindCallExpression {
		return nil
	}
	callee := cur.ChildByFieldName(fieldFunction)
	if callee == nil || callee.StartByte() > n.StartByte() || callee.EndByte() < n.EndByte() {
		return nil
	}
	return cur
}
