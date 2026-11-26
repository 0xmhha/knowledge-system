package solidity

import (
	"sort"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Storage pointers. A local declared `storage` is another name for a state
// variable, or for a part of one: after `Proposal storage p = proposals[id];`
// the statement `p.approved = n` writes `proposals` and `p.approved > 0`
// reads it. The body walker asks this file what each such local can point to
// and records a use through it as a use of that state variable.
//
// A pointer is followed when it is bound, in a declaration or by a later
// assignment, to
//   - a state variable or a part of it (`x`, `x[k]`, `x[k].member`),
//   - another pointer of the same function, or
//   - the storage value a function returns, when that function is declared in
//     the same file, in the contract or in one of its bases.
//
// It is not followed when it is a parameter (the caller decides what it
// points to) or when the function that returns it is declared in another
// file. Resolving that function would need Pass 2 and what its file's parse
// recorded; an incremental build does not hand that over for an unchanged
// file, and it would then lay the edges out differently from a full build.
//
// Before this, GovBase had no write edge for 9 such sites:
// `versionedMemberList` showed no write at all, and `_vote` and
// `_executeProposal` no write of `proposals`.

// Tree-sitter node kinds and fields read while following storage pointers.
const (
	kindVariableDeclStatement = "variable_declaration_statement"
	kindVariableDeclTuple     = "variable_declaration_tuple"
	kindReturnStatement       = "return_statement"
	kindReturnTypeDefinition  = "return_type_definition"
	kindComma                 = ","
	fieldLocation             = "location"
	fieldReturnType           = "return_type"
)

// pointerSource is one thing a storage pointer was bound to: the root name
// of an expression, or position pos of the values the function callee
// returns (callee is then not nil).
type pointerSource struct {
	name   string
	callee *sitter.Node
	pos    int
}

// callablePointers is what one callable's storage pointers can point to.
type callablePointers struct {
	// isPointer holds the locals and parameters declared `storage`.
	isPointer map[string]bool
	// isParameter holds the pointers that are parameters of the callable.
	isParameter map[string]bool
	// roots maps a pointer to the names it can stand for. A name is not
	// resolved here: Pass 2 keeps it when it is a state variable or a
	// mapping in scope.
	roots map[string][]string
	// returns holds, per return position, the names the returned storage
	// value can stand for.
	returns [][]string
}

// pointerAnalysis caches the pointers of the callables of one file.
type pointerAnalysis struct {
	v          *declVisitor
	done       map[uint]*callablePointers
	inProgress map[uint]bool
}

// newPointerAnalysis returns an empty cache for the visitor's file.
func newPointerAnalysis(v *declVisitor) *pointerAnalysis {
	return &pointerAnalysis{v: v, done: map[uint]*callablePointers{}, inProgress: map[uint]bool{}}
}

// of returns the storage pointers of a callable declaration. A callable that
// is reached again while it is being analysed (recursion) yields no pointers.
func (a *pointerAnalysis) of(decl *sitter.Node) *callablePointers {
	key := decl.StartByte()
	if cp, ok := a.done[key]; ok {
		return cp
	}
	empty := &callablePointers{isPointer: map[string]bool{}, isParameter: map[string]bool{}, roots: map[string][]string{}}
	if a.inProgress[key] {
		return empty
	}
	a.inProgress[key] = true
	defer delete(a.inProgress, key)

	cp := empty
	locals := map[string]bool{}
	collectLocalNames(decl, a.v.src, locals)
	sources := map[string][]pointerSource{}
	var returnNames []string
	var returnSources [][]pointerSource

	// Parameters and named returns declared `storage`.
	for i := uint(0); i < decl.NamedChildCount(); i++ {
		c := decl.NamedChild(i)
		if c == nil || c.Kind() != kindParameter {
			continue
		}
		if name := a.storageName(c); name != "" {
			cp.isPointer[name] = true
			cp.isParameter[name] = true
		}
	}
	if ret := decl.ChildByFieldName(fieldReturnType); ret != nil && ret.Kind() == kindReturnTypeDefinition {
		for i := uint(0); i < ret.NamedChildCount(); i++ {
			c := ret.NamedChild(i)
			if c == nil || c.Kind() != kindParameter {
				continue
			}
			name := a.storageName(c)
			if name != "" {
				cp.isPointer[name] = true
			}
			returnNames = append(returnNames, name)
		}
	}
	returnSources = make([][]pointerSource, len(returnNames))

	// Bindings in the body.
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		switch n.Kind() {
		case kindAssemblyStatement:
			return
		case kindVariableDeclStatement:
			a.bindDeclaration(n, cp, sources)
		case kindAssignment:
			a.bindAssignment(n, cp, sources)
		case kindReturnStatement:
			for pos, expr := range tupleElements(firstNamedChild(n)) {
				if pos < len(returnSources) {
					returnSources[pos] = append(returnSources[pos], a.sourceOf(expr, decl)...)
				}
			}
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if c := n.NamedChild(i); c != nil {
				walk(c)
			}
		}
	}
	if body := decl.ChildByFieldName(fieldBody); body != nil {
		walk(body)
	}

	// Resolve every pointer to names that are not locals of the callable.
	var resolve func(srcs []pointerSource, seen map[string]bool) []string
	resolve = func(srcs []pointerSource, seen map[string]bool) []string {
		var out []string
		for _, s := range srcs {
			switch {
			case s.callee != nil:
				if rets := a.of(s.callee).returns; s.pos < len(rets) {
					out = append(out, rets[s.pos]...)
				}
			case cp.isPointer[s.name]:
				if !seen[s.name] {
					seen[s.name] = true
					out = append(out, resolve(sources[s.name], seen)...)
				}
			case !locals[s.name]:
				out = append(out, s.name)
			}
		}
		return out
	}
	for name := range cp.isPointer {
		if roots := uniqueSorted(resolve(sources[name], map[string]bool{name: true})); len(roots) > 0 {
			cp.roots[name] = roots
		}
	}
	cp.returns = make([][]string, len(returnNames))
	for pos, name := range returnNames {
		roots := resolve(returnSources[pos], map[string]bool{})
		if name != "" {
			roots = append(roots, cp.roots[name]...)
		}
		cp.returns[pos] = uniqueSorted(roots)
	}
	a.done[key] = cp
	return cp
}

// storageName returns the name of a parameter or variable declaration that is
// declared `storage`, or "".
func (a *pointerAnalysis) storageName(decl *sitter.Node) string {
	location := decl.ChildByFieldName(fieldLocation)
	name := decl.ChildByFieldName(fieldName)
	if location == nil || name == nil || location.Utf8Text(a.v.src) != locationStorage {
		return ""
	}
	return name.Utf8Text(a.v.src)
}

// bindDeclaration records what the storage pointers of one declaration
// statement are bound to: `T storage p = expr;` or `(T storage p, ...) =
// f();`.
func (a *pointerAnalysis) bindDeclaration(stmt *sitter.Node, cp *callablePointers, sources map[string][]pointerSource) {
	value := unwrapExpression(stmt.ChildByFieldName(fieldValue))
	enclosing := enclosingCallable(stmt)
	for i := uint(0); i < stmt.NamedChildCount(); i++ {
		c := stmt.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case kindVariableDeclaration:
			if name := a.storageName(c); name != "" {
				cp.isPointer[name] = true
				if value != nil {
					sources[name] = append(sources[name], a.sourceOf(value, enclosing)...)
				}
			}
		case kindVariableDeclTuple:
			pos := 0
			for j := uint(0); j < c.ChildCount(); j++ {
				d := c.Child(j)
				if d == nil {
					continue
				}
				if d.Kind() == kindComma {
					pos++
					continue
				}
				if d.Kind() != kindVariableDeclaration {
					continue
				}
				if name := a.storageName(d); name != "" {
					cp.isPointer[name] = true
					sources[name] = append(sources[name], a.sourceAt(value, pos, enclosing)...)
				}
			}
		}
	}
}

// bindAssignment records `p = expr;` and `(p, q) = f();` when the left side
// names storage pointers: the pointer is bound again, nothing is written.
func (a *pointerAnalysis) bindAssignment(assign *sitter.Node, cp *callablePointers, sources map[string][]pointerSource) {
	left := unwrapExpression(assign.ChildByFieldName(fieldLeft))
	right := unwrapExpression(assign.ChildByFieldName(fieldRight))
	if left == nil || right == nil {
		return
	}
	enclosing := enclosingCallable(assign)
	for pos, target := range tupleElements(left) {
		if target == nil || target.Kind() != kindIdentifier {
			continue
		}
		name := target.Utf8Text(a.v.src)
		if !cp.isPointer[name] {
			continue
		}
		if left.Kind() == kindTupleExpression {
			sources[name] = append(sources[name], a.sourceAt(right, pos, enclosing)...)
		} else {
			sources[name] = append(sources[name], a.sourceOf(right, enclosing)...)
		}
	}
}

// sourceAt returns what position pos of a tuple-valued expression stands
// for: element pos of a tuple, or return value pos of a call.
func (a *pointerAnalysis) sourceAt(value *sitter.Node, pos int, enclosing *sitter.Node) []pointerSource {
	if value == nil {
		return nil
	}
	if value.Kind() == kindTupleExpression {
		if elements := tupleElements(value); pos < len(elements) {
			return a.sourceOf(elements[pos], enclosing)
		}
		return nil
	}
	if callee := a.calleeOf(value, enclosing); callee != nil {
		return []pointerSource{{callee: callee, pos: pos}}
	}
	return nil
}

// sourceOf returns what a storage-valued expression stands for: the root
// name of `x`, `x[k]`, `x.member`, or the first return value of `f(...)`
// when f is declared in the file.
func (a *pointerAnalysis) sourceOf(expr, enclosing *sitter.Node) []pointerSource {
	for expr != nil {
		switch expr.Kind() {
		case kindExpression, kindParenthesized:
			expr = firstNamedChild(expr)
		case kindMemberExpression:
			expr = expr.ChildByFieldName(fieldObject)
		case kindArrayAccess:
			expr = expr.ChildByFieldName(fieldBase)
		case kindIdentifier:
			return []pointerSource{{name: expr.Utf8Text(a.v.src)}}
		case kindCallExpression:
			if callee := a.calleeOf(expr, enclosing); callee != nil {
				return []pointerSource{{callee: callee}}
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

// calleeOf returns the declaration of the function a call by bare name
// means, looked up among the functions of the enclosing contract and of its
// bases declared in the same file, or nil.
func (a *pointerAnalysis) calleeOf(call, enclosing *sitter.Node) *sitter.Node {
	if call == nil || call.Kind() != kindCallExpression || enclosing == nil {
		return nil
	}
	callee := unwrapExpression(call.ChildByFieldName(fieldFunction))
	if callee == nil || callee.Kind() != kindIdentifier {
		return nil
	}
	return a.findFunction(nearestContractNode(enclosing), callee.Utf8Text(a.v.src), argumentCount(call), map[uint]bool{})
}

// findFunction returns the function named name in contract or, failing
// that, in a base of it that the same file declares. Among overloads the one
// with argc parameters is preferred.
func (a *pointerAnalysis) findFunction(contract *sitter.Node, name string, argc int, seen map[uint]bool) *sitter.Node {
	if contract == nil || seen[contract.StartByte()] {
		return nil
	}
	seen[contract.StartByte()] = true
	body := contract.ChildByFieldName(fieldBody)
	var fallback *sitter.Node
	if body != nil {
		for i := uint(0); i < body.NamedChildCount(); i++ {
			fn := body.NamedChild(i)
			if fn == nil || fn.Kind() != kindFunctionDef {
				continue
			}
			ident := fn.ChildByFieldName(fieldName)
			if ident == nil || ident.Utf8Text(a.v.src) != name {
				continue
			}
			if countChildren(fn, kindParameter) == argc {
				return fn
			}
			if fallback == nil {
				fallback = fn
			}
		}
	}
	if fallback != nil {
		return fallback
	}
	for i := uint(0); i < contract.NamedChildCount(); i++ {
		spec := contract.NamedChild(i)
		if spec == nil || spec.Kind() != kindInheritanceSpecifier {
			continue
		}
		baseName := lastIdentifier(firstNamedChild(spec))
		if baseName == nil {
			continue
		}
		if found := a.findFunction(a.contractNamed(baseName.Utf8Text(a.v.src)), name, argc, seen); found != nil {
			return found
		}
	}
	return nil
}

// contractNamed returns the contract, library or interface declaration of
// that name at the top level of the file, or nil.
func (a *pointerAnalysis) contractNamed(name string) *sitter.Node {
	for i := uint(0); i < a.v.root.NamedChildCount(); i++ {
		c := a.v.root.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case kindContractDecl, kindLibraryDecl, kindInterfaceDecl:
			if ident := c.ChildByFieldName(fieldName); ident != nil && ident.Utf8Text(a.v.src) == name {
				return c
			}
		}
	}
	return nil
}

// enclosingCallable returns the callable declaration around n, or nil.
func enclosingCallable(n *sitter.Node) *sitter.Node {
	for cur := n; cur != nil; cur = cur.Parent() {
		if isCallableDecl(cur) {
			return cur
		}
	}
	return nil
}

// firstNamedChild returns the first named child of n, or nil.
func firstNamedChild(n *sitter.Node) *sitter.Node {
	if n == nil || n.NamedChildCount() == 0 {
		return nil
	}
	return n.NamedChild(0)
}

// countChildren returns how many named children of n have the given kind.
func countChildren(n *sitter.Node, kind string) int {
	count := 0
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil && c.Kind() == kind {
			count++
		}
	}
	return count
}

// tupleElements returns the elements of a tuple expression by position (nil
// for an empty position), or the expression itself as the only element.
func tupleElements(expr *sitter.Node) []*sitter.Node {
	expr = unwrapExpression(expr)
	if expr == nil {
		return nil
	}
	if expr.Kind() != kindTupleExpression {
		return []*sitter.Node{expr}
	}
	elements := []*sitter.Node{nil}
	for i := uint(0); i < expr.ChildCount(); i++ {
		c := expr.Child(i)
		if c == nil {
			continue
		}
		switch {
		case c.Kind() == kindComma:
			elements = append(elements, nil)
		case c.IsNamed():
			elements[len(elements)-1] = unwrapExpression(c)
		}
	}
	return elements
}

// collectLocalNames records every parameter and local variable name declared
// in a callable, including named returns and the parameters of try and
// catch. Inline assembly declares Yul names and is skipped.
func collectLocalNames(n *sitter.Node, src []byte, into map[string]bool) {
	switch n.Kind() {
	case kindParameter, kindVariableDeclaration:
		if name := n.ChildByFieldName(fieldName); name != nil {
			into[name.Utf8Text(src)] = true
		}
	case kindAssemblyStatement:
		return
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil {
			collectLocalNames(c, src, into)
		}
	}
}

// uniqueSorted returns the distinct values of names in sorted order.
func uniqueSorted(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	out := names[:1]
	for _, n := range names[1:] {
		if n != out[len(out)-1] {
			out = append(out, n)
		}
	}
	return out
}
