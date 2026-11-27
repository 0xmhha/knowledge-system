package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// messages.go checks the message links (schema 1.42) from the syntax and
// go/types, sharing no code with the extraction:
//
//   - handles: a receive operation -> the code its message makes run: the
//     calls (by their site), channel operations, go statements and field
//     selections in its range. The range is the rest of the block after a
//     plain receive (from the end of the receive expression), the body
//     of a range over a channel, the body of a select case. Code in a
//     function literal written in the range is the literal's, not the
//     range's.
//   - triggers: a sender -> a receive on the same channel. Decided only
//     for a local channel that one make sets, that nothing else assigns
//     and that is used only as the channel of sends, receives, ranges,
//     close, len and cap (it reaches no other code). The sender of a send
//     in a go statement's literal is that goroutine; else the function
//     the send is written in.
//
// A CallSite is named by its file and span, a Goroutine by the file and
// start of its go statement, a field by its declaration.

// messageFacts are the handles and triggers truth of the covered files.
type messageFacts struct {
	handles map[[2]string]bool
	// triggers are the expected (sender, receive site) pairs; decided the
	// receive sites whose senders the check knows all of.
	triggers map[[2]string]bool
	decided  map[string]bool
}

func newMessageFacts() *messageFacts {
	return &messageFacts{handles: map[[2]string]bool{}, triggers: map[[2]string]bool{}, decided: map[string]bool{}}
}

// span names a CallSite: file and byte span.
func (pf *programFacts) span(rel string, from, to token.Pos) string {
	return fmt.Sprintf("%s@%d-%d", rel, pf.fset.Position(from).Offset, pf.fset.Position(to).Offset)
}

// messages records the message facts of file f.
func (pf *programFacts) messages(info *gotypes.Info, f *ast.File, rel string) {
	isChan := func(e ast.Expr) bool {
		t := info.TypeOf(e)
		if t == nil {
			return false
		}
		_, ok := t.Underlying().(*gotypes.Chan)
		return ok
	}
	ast.Inspect(f, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch x := n.(type) {
		case *ast.FuncDecl:
			body = x.Body
		case *ast.FuncLit:
			body = x.Body
		}
		if body != nil {
			pf.handlesIn(info, body, rel, isChan)
		}
		if d, ok := n.(*ast.FuncDecl); ok && d.Body != nil {
			pf.triggersIn(info, d, rel, isChan)
		}
		return true
	})
}

// handlesIn records the handles facts of the receives written in body (not
// in the function literals in it).
func (pf *programFacts) handlesIn(info *gotypes.Info, body *ast.BlockStmt, rel string, isChan func(ast.Expr) bool) {
	type recv struct {
		site     string
		from, to token.Pos
	}
	var recvs []recv
	// parent maps each node of body to the node around it, to find the
	// statement a receive is in.
	parent := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) > 0 {
			parent[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	// after is where the range of a plain receive starts: after the
	// innermost simple statement that holds it (a statement with no block
	// of its own), or after the receive in a condition.
	after := func(u *ast.UnaryExpr) token.Pos {
		for n := parent[u]; n != nil; n = parent[n] {
			switch n.(type) {
			case *ast.ExprStmt, *ast.AssignStmt, *ast.SendStmt, *ast.IncDecStmt, *ast.DeclStmt, *ast.ReturnStmt, *ast.GoStmt, *ast.DeferStmt:
				return n.End()
			case ast.Stmt, *ast.FuncLit:
				return u.End()
			}
		}
		return u.End()
	}
	var block func(list []ast.Stmt, end token.Pos)
	block = func(list []ast.Stmt, end token.Pos) {
		for _, st := range list {
			st := st
			ast.Inspect(st, func(x ast.Node) bool {
				switch y := x.(type) {
				case *ast.FuncLit:
					return false
				case *ast.BlockStmt:
					block(y.List, y.Rbrace)
					return false
				case *ast.CaseClause:
					block(y.Body, y.End())
					return false
				case *ast.CommClause:
					if y.Comm != nil {
						ast.Inspect(y.Comm, func(z ast.Node) bool {
							if u, ok := z.(*ast.UnaryExpr); ok && u.Op == token.ARROW && !pf.isChanLock(info, u.X) {
								recvs = append(recvs, recv{site: pf.span(rel, u.Pos(), u.End()), from: y.Colon + 1, to: y.End()})
							}
							return true
						})
					}
					block(y.Body, y.End())
					return false
				case *ast.RangeStmt:
					if isChan(y.X) && !pf.isChanLock(info, y.X) {
						recvs = append(recvs, recv{site: pf.span(rel, y.Range, y.X.End()), from: y.Body.Lbrace, to: y.Body.Rbrace})
					}
				case *ast.UnaryExpr:
					if y.Op == token.ARROW && !pf.isChanLock(info, y.X) {
						recvs = append(recvs, recv{site: pf.span(rel, y.Pos(), y.End()), from: after(y), to: end})
					}
				}
				return true
			})
		}
	}
	block(body.List, body.Rbrace)
	field := func(site string, v *gotypes.Var) {
		if k := pf.key(v); k != "" {
			pf.msg.handles[[2]string{site, k}] = true
		}
	}
	for _, r := range recvs {
		in := func(p token.Pos) bool { return p >= r.from && p < r.to }
		goCalls := map[*ast.CallExpr]bool{} // the call of a go statement is the goroutine
		ast.Inspect(body, func(x ast.Node) bool {
			switch y := x.(type) {
			case *ast.FuncLit:
				return false
			case *ast.GoStmt:
				goCalls[y.Call] = true
				if in(y.Pos()) {
					pf.msg.handles[[2]string{r.site, fmt.Sprintf("%s@%d", rel, pf.fset.Position(y.Pos()).Offset)}] = true
				}
			case *ast.CompositeLit:
				// T{f: v} sets the field f; T{v, w} sets them in order.
				if st, ok := typeStruct(info.TypeOf(y)); ok {
					for i, el := range y.Elts {
						if _, keyed := el.(*ast.KeyValueExpr); !keyed && i < st.NumFields() && in(el.Pos()) {
							field(r.site, st.Field(i))
						}
						if kv, ok := el.(*ast.KeyValueExpr); ok && in(kv.Key.Pos()) {
							if id, ok := kv.Key.(*ast.Ident); ok {
								for i := 0; i < st.NumFields(); i++ {
									if st.Field(i).Name() == id.Name {
										field(r.site, st.Field(i))
									}
								}
							}
						}
					}
				}
			case *ast.CallExpr:
				if tv, ok := info.Types[y.Fun]; ok && tv.IsType() || goCalls[y] {
					return true // a conversion, or a goroutine
				}
				if id, ok := ast.Unparen(y.Fun).(*ast.Ident); ok {
					if b, isB := info.ObjectOf(id).(*gotypes.Builtin); isB && (b.Name() == "make" || b.Name() == "new") {
						return true // an allocation, not a call
					}
				}
				if in(y.Lparen) {
					pf.msg.handles[[2]string{r.site, pf.span(rel, y.Pos(), y.End())}] = true
				}
			case *ast.SendStmt:
				if in(y.Arrow) {
					pf.msg.handles[[2]string{r.site, pf.span(rel, y.Pos(), y.End())}] = true
				}
			case *ast.UnaryExpr:
				if y.Op == token.ARROW && in(y.OpPos) {
					pf.msg.handles[[2]string{r.site, pf.span(rel, y.Pos(), y.End())}] = true
				}
			case *ast.RangeStmt:
				if isChan(y.X) && in(y.For) {
					pf.msg.handles[[2]string{r.site, pf.span(rel, y.Range, y.X.End())}] = true
				}
			case *ast.SelectorExpr:
				// The field the selection names; the embedded fields it only
				// passes through (p.Log() through an embedded *Logger) are
				// not used, as reads_field records it.
				if s := info.Selections[y]; s != nil && s.Kind() == gotypes.FieldVal && in(y.Sel.Pos()) {
					if v, ok := s.Obj().(*gotypes.Var); ok {
						field(r.site, v)
					}
				}
			}
			return true
		})
	}
}

// triggersIn records the triggers facts of the decided local channels of
// function d.
func (pf *programFacts) triggersIn(info *gotypes.Info, d *ast.FuncDecl, rel string, isChan func(ast.Expr) bool) {
	fnKey := fmt.Sprintf("%s:%d:%s", rel, pf.fset.Position(d.Pos()).Line, d.Name.Name)
	type local struct {
		made, escapes bool
		sets          int
		sends         []string // sender keys
		recvs         []string // receive sites
	}
	locals := map[gotypes.Object]*local{}
	get := func(e ast.Expr) (*local, bool) {
		id, ok := ast.Unparen(e).(*ast.Ident)
		if !ok {
			return nil, false
		}
		obj := info.ObjectOf(id)
		if obj == nil {
			return nil, false
		}
		v, ok := obj.(*gotypes.Var)
		if !ok || v.IsField() || obj.Parent() == obj.Pkg().Scope() {
			return nil, false
		}
		l := locals[obj]
		if l == nil {
			l = &local{}
			locals[obj] = l
		}
		return l, true
	}
	operand := map[*ast.Ident]bool{} // identifiers used as an operation's channel
	set := func(lhs, rhs ast.Expr) {
		l, ok := get(lhs)
		if !ok {
			return
		}
		operand[ast.Unparen(lhs).(*ast.Ident)] = true
		l.sets++
		if c, ok := ast.Unparen(rhs).(*ast.CallExpr); ok && isMakeChan(info, c) {
			l.made = true
		}
	}
	// A send inside a function literal that is not a go statement's runs
	// wherever the literal is called (go eventLoop() starts it in a
	// goroutine): its channel is not decided here.
	var walk func(n ast.Node, goroutine string, inClosure bool)
	walk = func(n ast.Node, goroutine string, inClosure bool) {
		sender := fnKey
		if goroutine != "" {
			sender = goroutine
		}
		ast.Inspect(n, func(x ast.Node) bool {
			switch y := x.(type) {
			case *ast.GoStmt:
				if lit, ok := ast.Unparen(y.Call.Fun).(*ast.FuncLit); ok {
					walk(lit.Body, fmt.Sprintf("%s@%d", rel, pf.fset.Position(y.Pos()).Offset), false)
					for _, a := range y.Call.Args {
						walk(a, goroutine, inClosure)
					}
					return false
				}
			case *ast.FuncLit:
				walk(y.Body, goroutine, true)
				return false
			case *ast.AssignStmt:
				for i, l := range y.Lhs {
					var r ast.Expr
					if len(y.Lhs) == len(y.Rhs) {
						r = y.Rhs[i]
					}
					set(l, r)
				}
			case *ast.ValueSpec:
				for i, name := range y.Names {
					var r ast.Expr
					if len(y.Names) == len(y.Values) {
						r = y.Values[i]
					}
					set(name, r)
				}
			case *ast.SendStmt:
				if l, ok := get(y.Chan); ok {
					operand[ast.Unparen(y.Chan).(*ast.Ident)] = true
					l.sends = append(l.sends, sender)
					if inClosure {
						l.escapes = true
					}
				}
			case *ast.UnaryExpr:
				if y.Op == token.ARROW {
					if l, ok := get(y.X); ok {
						operand[ast.Unparen(y.X).(*ast.Ident)] = true
						l.recvs = append(l.recvs, pf.span(rel, y.Pos(), y.End()))
					}
				}
			case *ast.RangeStmt:
				if isChan(y.X) {
					if l, ok := get(y.X); ok {
						operand[ast.Unparen(y.X).(*ast.Ident)] = true
						l.recvs = append(l.recvs, pf.span(rel, y.Range, y.X.End()))
					}
				}
			case *ast.CallExpr:
				if id, ok := ast.Unparen(y.Fun).(*ast.Ident); ok {
					if _, builtin := info.ObjectOf(id).(*gotypes.Builtin); builtin && (id.Name == "close" || id.Name == "len" || id.Name == "cap") {
						for _, a := range y.Args {
							if aid, ok := ast.Unparen(a).(*ast.Ident); ok {
								operand[aid] = true
							}
						}
					}
				}
			}
			return true
		})
	}
	walk(d.Body, "", false)
	// A use of the variable other than as an operation's channel lets it
	// reach other code (a call argument, a field, a return).
	ast.Inspect(d.Body, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && !operand[id] {
			if l := locals[info.ObjectOf(id)]; l != nil {
				l.escapes = true
			}
		}
		return true
	})
	for obj, l := range locals {
		if !l.made || l.sets != 1 || l.escapes || !isChanType(obj.Type()) || pf.chanLocks.lockOf(pf.key(obj)) != "" {
			continue
		}
		for _, r := range l.recvs {
			pf.msg.decided[r] = true
			for _, s := range l.sends {
				pf.msg.triggers[[2]string{s, r}] = true
			}
		}
	}
}

// typeStruct returns the struct t (or *t) denotes.
func typeStruct(t gotypes.Type) (*gotypes.Struct, bool) {
	if t == nil {
		return nil, false
	}
	if p, ok := t.Underlying().(*gotypes.Pointer); ok {
		t = p.Elem()
	}
	s, ok := t.Underlying().(*gotypes.Struct)
	return s, ok
}

// isChanLock reports whether the channel expression e names a channel used
// as a lock (findChanLocks).
func (pf *programFacts) isChanLock(info *gotypes.Info, e ast.Expr) bool {
	var obj gotypes.Object
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		obj = info.ObjectOf(x)
	case *ast.SelectorExpr:
		if s := info.Selections[x]; s != nil && s.Kind() == gotypes.FieldVal {
			if v, ok := s.Obj().(*gotypes.Var); ok {
				obj = v.Origin()
			}
		}
	}
	return obj != nil && pf.chanLocks.lockOf(pf.key(obj)) != ""
}

func isChanType(t gotypes.Type) bool {
	_, ok := t.Underlying().(*gotypes.Chan)
	return ok
}

// placeKey names a node as the message checks do: a CallSite by its span, a
// Goroutine by its start, a declaration by its file, line and name.
func placeKey(n types.Node) string {
	f := filepath.ToSlash(n.FilePath)
	switch n.Type {
	case types.NodeCallSite:
		return fmt.Sprintf("%s@%d-%d", f, n.StartByte, n.EndByte)
	case types.NodeGoroutine:
		return fmt.Sprintf("%s@%d", f, n.StartByte)
	}
	return fmt.Sprintf("%s:%d:%s", f, n.StartLine, n.Name)
}

// compareMessages compares the handles and triggers edges of g with the
// truth. Only triggers that end at a decided receive count.
func compareMessages(m *messageFacts, idx nodeIndex, g GraphReader, samples int) (handles, triggers UnderLockStats, err error) {
	read := func(t types.EdgeType) (map[[2]string]bool, error) {
		edges, err := g.QueryEdgesByType(string(t))
		if err != nil {
			return nil, fmt.Errorf("verify: read %s: %w", t, err)
		}
		out := map[[2]string]bool{}
		for _, e := range edges {
			out[[2]string{placeKey(idx.byID[e.Src]), placeKey(idx.byID[e.Dst])}] = true
		}
		return out, nil
	}
	h, err := read(types.EdgeHandles)
	if err != nil {
		return handles, triggers, err
	}
	tr, err := read(types.EdgeTriggers)
	if err != nil {
		return handles, triggers, err
	}
	handles = comparePairs(m.handles, h, nil, samples)
	triggers = comparePairs(m.triggers, tr, func(k [2]string) bool { return !m.decided[k[1]] }, samples)
	return handles, triggers, nil
}
