package verify

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	gotypes "go/types"
	"path/filepath"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

// chanlock.go finds the channels used as locks from the syntax, for the
// lock truth (underlock.go), sharing no code with the extraction. Two rules:
//
//   - a make(chan struct{}, 1) whose block's first later statement that
//     names the same variable or field sends on it: a receive takes the
//     token, a send returns it;
//   - inverted: a local make(chan struct{}, 1) that is not filled, whose
//     every send is a case of a select with a default (a try-acquire) and
//     every receive a plain statement, used for nothing else: a send takes
//     the lock, a receive returns it.
//
// The lock is named like the graph's Channel node: the make call's file and
// line and the name "channel". A variable, field or parameter is a name of
// the lock when a name of it is assigned to it, set in a composite literal
// or passed to it as an argument of a call of a declared function, followed
// to a fixed point over all covered functions.

// chanLockNames maps the declaration key of each variable, field and
// parameter naming a channel lock (declKey form) to the lock's key;
// inverted are the locks of the second rule.
type chanLockNames struct {
	names    map[string]string
	inverted map[string]bool
}

// lockOf returns the lock a declaration key names, "" when none.
func (c chanLockNames) lockOf(key string) string { return c.names[key] }

// findChanLocks returns the channel locks of the covered files.
func findChanLocks(pkgs []*packages.Package, root string, scope map[string]bool) chanLockNames {
	out := chanLockNames{names: map[string]string{}, inverted: map[string]bool{}}
	seen := map[string]bool{}
	type fnBody struct {
		k    *chanLockFinder
		body *ast.BlockStmt
	}
	var bodies []fnBody
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for i, f := range pkg.Syntax {
			if i >= len(pkg.CompiledGoFiles) {
				continue
			}
			rel, ok := relPath(root, pkg.CompiledGoFiles[i])
			if !ok || !scope[rel] || seen[rel] {
				continue
			}
			seen[rel] = true
			k := &chanLockFinder{info: pkg.TypesInfo, fset: pkg.Fset, root: root, scope: scope, rel: filepath.ToSlash(rel), out: out}
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
					k.function(fd.Body)
					bodies = append(bodies, fnBody{k, fd.Body})
				}
			}
		}
	}
	// The names reach other functions through calls and stores.
	for changed := len(out.names) > 0; changed; {
		changed = false
		for _, b := range bodies {
			if b.k.spread(b.body) {
				changed = true
			}
		}
	}
	return out
}

type chanLockFinder struct {
	info  *gotypes.Info
	fset  *token.FileSet
	root  string
	scope map[string]bool
	rel   string
	out   chanLockNames
}

// key is the declaration key of obj (file:line:name), "" outside scope.
func (k *chanLockFinder) key(obj gotypes.Object) string {
	if obj == nil || !obj.Pos().IsValid() {
		return ""
	}
	p := k.fset.Position(obj.Pos())
	rel, ok := relPath(k.root, p.Filename)
	if !ok || !k.scope[rel] {
		return ""
	}
	return fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), p.Line, obj.Name())
}

// name returns the declaration key of the variable or field e names.
func (k *chanLockFinder) name(e ast.Expr) string {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return k.key(k.info.ObjectOf(x))
	case *ast.SelectorExpr:
		if s := k.info.Selections[x]; s != nil && s.Kind() == gotypes.FieldVal {
			if v, ok := s.Obj().(*gotypes.Var); ok {
				return k.key(v.Origin())
			}
		}
	}
	return ""
}

// lockMake reports whether e is make(chan struct{}, 1).
func (k *chanLockFinder) lockMake(e ast.Expr) bool {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 2 {
		return false
	}
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	if !ok || id.Name != "make" {
		return false
	}
	if _, builtin := k.info.ObjectOf(id).(*gotypes.Builtin); !builtin {
		return false
	}
	ch, ok := k.info.TypeOf(c.Args[0]).Underlying().(*gotypes.Chan)
	if !ok {
		return false
	}
	if st, ok := ch.Elem().Underlying().(*gotypes.Struct); !ok || st.NumFields() != 0 {
		return false
	}
	tv, ok := k.info.Types[c.Args[1]]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return false
	}
	n, exact := constant.Int64Val(tv.Value)
	return exact && n == 1
}

// mentions reports whether st names the variable or field name anywhere.
func (k *chanLockFinder) mentions(st ast.Stmt, name string) bool {
	found := false
	ast.Inspect(st, func(n ast.Node) bool {
		if found {
			return false
		}
		if e, ok := n.(ast.Expr); ok && k.name(e) == name {
			found = true
		}
		return true
	})
	return found
}

// makes returns the names a statement sets to a lock make, with the make's
// key: x := make(...), x = make(...), v.f = make(...), and the fields of a
// composite literal set to one (T{f: make(...)}).
func (k *chanLockFinder) makes(s ast.Stmt) map[string]string {
	out := map[string]string{}
	mark := func(name string, mk ast.Expr) {
		if name != "" {
			p := k.fset.Position(mk.Pos())
			out[name] = fmt.Sprintf("%s:%d:channel", k.rel, p.Line)
		}
	}
	if as, ok := s.(*ast.AssignStmt); ok && len(as.Lhs) == len(as.Rhs) {
		for i, r := range as.Rhs {
			if k.lockMake(r) {
				mark(k.name(as.Lhs[i]), r)
			}
		}
	}
	ast.Inspect(s, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		st, ok := typeStruct(k.info.TypeOf(cl))
		if !ok {
			return true
		}
		for i, el := range cl.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok && k.lockMake(kv.Value) {
					for j := 0; j < st.NumFields(); j++ {
						if st.Field(j).Name() == id.Name {
							mark(k.key(st.Field(j).Origin()), kv.Value)
						}
					}
				}
			} else if i < st.NumFields() && k.lockMake(el) {
				mark(k.key(st.Field(i).Origin()), el)
			}
		}
		return true
	})
	if ds, ok := s.(*ast.DeclStmt); ok {
		if gd, ok := ds.Decl.(*ast.GenDecl); ok {
			for _, sp := range gd.Specs {
				if vs, ok := sp.(*ast.ValueSpec); ok && len(vs.Names) == len(vs.Values) {
					for i, v := range vs.Values {
						if k.lockMake(v) {
							mark(k.key(k.info.ObjectOf(vs.Names[i])), v)
						}
					}
				}
			}
		}
	}
	return out
}

// spread adds the names a lock gets in body: the field or variable a name of
// it is assigned to or set in a composite literal, and the parameter of a
// declared function a name of it is passed to. It reports whether it added
// one.
func (k *chanLockFinder) spread(body *ast.BlockStmt) bool {
	added := false
	put := func(name, lock string) {
		if name != "" {
			if _, ok := k.out.names[name]; !ok {
				k.out.names[name] = lock
				added = true
			}
		}
	}
	lockOf := func(e ast.Expr) string { return k.out.names[k.name(e)] }
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, r := range x.Rhs {
					if lock := lockOf(r); lock != "" {
						put(k.name(x.Lhs[i]), lock)
					}
				}
			}
		case *ast.CompositeLit:
			st, ok := typeStruct(k.info.TypeOf(x))
			if !ok {
				return true
			}
			for i, el := range x.Elts {
				v, field := el, -1
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					v = kv.Value
					if id, ok := kv.Key.(*ast.Ident); ok {
						for j := 0; j < st.NumFields(); j++ {
							if st.Field(j).Name() == id.Name {
								field = j
							}
						}
					}
				} else if i < st.NumFields() {
					field = i
				}
				if lock := lockOf(v); lock != "" && field >= 0 {
					put(k.key(st.Field(field).Origin()), lock)
				}
			}
		case *ast.CallExpr:
			fn, ok := typeutil.Callee(k.info, x).(*gotypes.Func)
			if !ok {
				return true
			}
			sig, ok := fn.Origin().Type().(*gotypes.Signature)
			if !ok {
				return true
			}
			for i, a := range x.Args {
				if lock := lockOf(a); lock != "" && i < sig.Params().Len() && (!sig.Variadic() || i < sig.Params().Len()-1) {
					put(k.key(sig.Params().At(i)), lock)
				}
			}
		}
		return true
	})
	return added
}

// function finds the locks made in body and the names they are stored in.
func (k *chanLockFinder) function(body *ast.BlockStmt) {
	k.inverted(body)
	local := map[string]string{}
	ast.Inspect(body, func(n ast.Node) bool {
		b, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i := range b.List {
			for name, lock := range k.makes(b.List[i]) {
				// The first later statement of the block that names the
				// channel must be a send on it.
				for _, st := range b.List[i+1:] {
					if send, ok := st.(*ast.SendStmt); ok && k.name(send.Chan) == name {
						local[name] = lock
						break
					}
					if k.mentions(st, name) {
						break
					}
				}
			}
		}
		return true
	})
	if len(local) == 0 {
		return
	}
	// A variable holding the lock, assigned to a field or set in a
	// composite literal in the same function, names it too.
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, r := range x.Rhs {
					if lock, ok := local[k.name(r)]; ok {
						if name := k.name(x.Lhs[i]); name != "" {
							local[name] = lock
						}
					}
				}
			}
		case *ast.CompositeLit:
			st, ok := typeStruct(k.info.TypeOf(x))
			if !ok {
				return true
			}
			for i, el := range x.Elts {
				v, field := el, -1
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					v = kv.Value
					if id, ok := kv.Key.(*ast.Ident); ok {
						for j := 0; j < st.NumFields(); j++ {
							if st.Field(j).Name() == id.Name {
								field = j
							}
						}
					}
				} else if i < st.NumFields() {
					field = i
				}
				if lock, ok := local[k.name(v)]; ok && field >= 0 {
					local[k.key(st.Field(field).Origin())] = lock
				}
			}
		}
		return true
	})
	for name, lock := range local {
		k.out.names[name] = lock
	}
}

// inverted finds the locks of the inverted rule made in body: a local
// make(chan struct{}, 1), not filled, whose every send is a case of a select
// with a default and every receive a plain statement, the variable used for
// nothing else.
func (k *chanLockFinder) inverted(body *ast.BlockStmt) {
	made := map[gotypes.Object]string{}
	ast.Inspect(body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && as.Tok == token.DEFINE && len(as.Lhs) == len(as.Rhs) {
			for i, r := range as.Rhs {
				if id, ok := as.Lhs[i].(*ast.Ident); ok && k.lockMake(r) {
					made[k.info.ObjectOf(id)] = fmt.Sprintf("%s:%d:channel", k.rel, k.fset.Position(r.Pos()).Line)
				}
			}
		}
		if vs, ok := n.(*ast.ValueSpec); ok && len(vs.Names) == len(vs.Values) {
			for i, v := range vs.Values {
				if k.lockMake(v) {
					made[k.info.ObjectOf(vs.Names[i])] = fmt.Sprintf("%s:%d:channel", k.rel, k.fset.Position(v.Pos()).Line)
				}
			}
		}
		return true
	})
	if len(made) == 0 {
		return
	}
	type use struct{ tryAcquire, release, other int }
	uses := map[gotypes.Object]*use{}
	allowed := map[*ast.Ident]bool{} // the identifiers in an allowed position
	comms := map[ast.Stmt]bool{}     // a select case's receive is not a plain one
	ast.Inspect(body, func(n ast.Node) bool {
		if cc, ok := n.(*ast.CommClause); ok && cc.Comm != nil {
			comms[cc.Comm] = true
		}
		return true
	})
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectStmt:
			hasDefault := false
			for _, c := range x.Body.List {
				if cc, ok := c.(*ast.CommClause); ok && cc.Comm == nil {
					hasDefault = true
				}
			}
			for _, c := range x.Body.List {
				cc, ok := c.(*ast.CommClause)
				if !ok || cc.Comm == nil {
					continue
				}
				if s, ok := cc.Comm.(*ast.SendStmt); ok && hasDefault {
					if id, ok := ast.Unparen(s.Chan).(*ast.Ident); ok {
						if obj := k.info.ObjectOf(id); made[obj] != "" {
							allowed[id] = true
							uses[obj] = orNew(uses[obj])
							uses[obj].tryAcquire++
						}
					}
				}
			}
		case *ast.ExprStmt:
			if u, ok := ast.Unparen(x.X).(*ast.UnaryExpr); ok && u.Op == token.ARROW && !comms[x] {
				if id, ok := ast.Unparen(u.X).(*ast.Ident); ok {
					if obj := k.info.ObjectOf(id); made[obj] != "" {
						allowed[id] = true
						uses[obj] = orNew(uses[obj])
						uses[obj].release++
					}
				}
			}
		}
		return true
	})
	ast.Inspect(body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && !allowed[id] {
			if obj := k.info.ObjectOf(id); made[obj] != "" && k.info.Defs[id] == nil {
				uses[obj] = orNew(uses[obj])
				uses[obj].other++
			}
		}
		return true
	})
	for obj, lock := range made {
		u := uses[obj]
		if u == nil || u.tryAcquire == 0 || u.release == 0 || u.other > 0 {
			continue
		}
		k.out.names[k.key(obj)] = lock
		k.out.inverted[lock] = true
	}
}

// orNew returns u, or a new zero value when u is nil.
func orNew[T any](u *T) *T {
	if u == nil {
		return new(T)
	}
	return u
}
