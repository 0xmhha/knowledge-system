package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// chanflow.go records what a message on a channel makes run (schema 1.42):
//
//	handles   receive CallSite -> the code in the receive's range
//	flows_to  receive CallSite -> the uses its value reaches (kind received)
//	triggers  sender -> each receive CallSite on the same channel
//
// The facts come from SSA: the instructions of a function (calls, field
// accesses, channel operations, go statements) and the channel origins of
// the channel linker. The range of a receive is taken from the syntax the
// SSA function was built from: the rest of the block after a plain receive,
// the body of a range over a channel, the body of a select case.

// Dispatch kinds of a handles edge: what the handled code does.
const (
	handlesCall  = "call"
	handlesSend  = "send"
	handlesRecv  = "recv"
	handlesGo    = "go"
	handlesRead  = "read"
	handlesWrite = "write"
)

// receivedFlow is the dispatch kind of a flows_to edge from a receive to a
// use of the value it received.
const receivedFlow = types.DispatchReceived

// receivedFlowDepth bounds how many copies (a phi, a conversion, a local
// cell) the value of a receive is followed through.
const receivedFlowDepth = 8

// chanEvent is one channel operation with the origins of its channel.
type chanEvent struct {
	fn      *ssa.Function
	typ     types.EdgeType
	pos     token.Pos
	origins []chanOrigin
}

// siteIndex finds the statement nodes of a position: CallSite nodes by
// span, LoopStmt and Goroutine nodes by start.
type siteIndex struct {
	sites map[string][]*types.Node       // CallSite nodes by file, by start byte
	loops map[string]map[int]*types.Node // LoopStmt nodes by file and start byte
	gors  map[string]map[int]*types.Node // Goroutine nodes by file and start byte
	files map[string]string              // position file name -> node file path
}

func newSiteIndex(nodes []types.Node) *siteIndex {
	ix := &siteIndex{sites: map[string][]*types.Node{}, loops: map[string]map[int]*types.Node{},
		gors: map[string]map[int]*types.Node{}, files: map[string]string{}}
	put := func(m map[string]map[int]*types.Node, n *types.Node) {
		if m[n.FilePath] == nil {
			m[n.FilePath] = map[int]*types.Node{}
		}
		m[n.FilePath][n.StartByte] = n
	}
	for i := range nodes {
		n := &nodes[i]
		switch n.Type {
		case types.NodeCallSite:
			ix.sites[n.FilePath] = append(ix.sites[n.FilePath], n)
		case types.NodeLoopStmt:
			put(ix.loops, n)
		case types.NodeGoroutine:
			put(ix.gors, n)
		}
	}
	for _, s := range ix.sites {
		sort.Slice(s, func(i, j int) bool { return s[i].StartByte < s[j].StartByte })
	}
	return ix
}

// file returns the node file path of a position's file, "" when no node
// is in it.
func (ix *siteIndex) file(p token.Position) string {
	if f, ok := ix.files[p.Filename]; ok {
		return f
	}
	// The longest node path that ends the file name: eth/handler_test.go
	// also ends .../eth/protocols/eth/handler_test.go.
	f := ""
	for nf := range ix.sites {
		if sameFile(nf, p.Filename) && len(nf) > len(f) {
			f = nf
		}
	}
	ix.files[p.Filename] = f
	return f
}

// site returns the smallest CallSite of sub kind subKind whose span holds
// p (that starts at p when exact).
func (ix *siteIndex) site(p token.Position, subKind string, exact bool) *types.Node {
	var best *types.Node
	for _, n := range ix.sites[ix.file(p)] {
		if n.StartByte > p.Offset {
			break
		}
		if n.SubKind != subKind || p.Offset >= n.EndByte || exact && n.StartByte != p.Offset {
			continue
		}
		if best == nil || n.EndByte-n.StartByte < best.EndByte-best.StartByte {
			best = n
		}
	}
	return best
}

// rangeSite returns the receive site of the range over a channel whose
// for keyword is at p: the first receive site inside the loop.
func (ix *siteIndex) rangeSite(p token.Position) *types.Node {
	f := ix.file(p)
	loop := ix.loops[f][p.Offset]
	if loop == nil {
		return nil
	}
	for _, n := range ix.sites[f] {
		if n.StartByte > p.Offset && n.StartByte < loop.EndByte && n.SubKind == subKindRecv {
			return n
		}
	}
	return nil
}

// recvSite returns the CallSite of the receive at p: the receive itself,
// or the range over a channel whose for keyword is at p.
func (ix *siteIndex) recvSite(p token.Position) *types.Node {
	if s := ix.site(p, subKindRecv, true); s != nil {
		return s
	}
	return ix.rangeSite(p)
}

// messageLinks returns the handles, flows_to (received) and triggers edges
// of funcs, and counts them in st.
func (c *chanLinker) messageLinks(funcs map[*ssa.Function]bool, nodes []types.Node, st *DispatchStats) []chanOp {
	ix := newSiteIndex(nodes)
	var out []chanOp
	handled := map[*types.Node]int{} // receive site -> handled items
	recvSites := map[*types.Node]bool{}
	fns := make([]*ssa.Function, 0, len(funcs))
	for fn := range funcs {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool {
		return fns[i].Pos() < fns[j].Pos() || fns[i].Pos() == fns[j].Pos() && fns[i].String() < fns[j].String()
	})
	for _, fn := range fns {
		for _, r := range c.receives(fn) {
			if _, isLock := c.locks[r.anchor]; isLock {
				continue // taking a lock's token carries no work (chanlock.go)
			}
			site := ix.recvSite(c.prog.Fset.Position(r.anchor))
			if site == nil {
				continue
			}
			// A file built twice (a package and its test variant) gives
			// two SSA functions for one receive: count its site once.
			if !recvSites[site] {
				recvSites[site] = true
				st.Receives++
			}
			for _, h := range c.handledIn(fn, r, ix) {
				out = append(out, chanOp{src: site, dst: h.node, typ: types.EdgeHandles, kind: h.kind, line: h.line, file: site.FilePath})
				handled[site]++
				st.Handles++
			}
		}
		for _, fl := range c.receivedFlows(fn, ix) {
			out = append(out, fl)
			st.ReceivedFlows++
		}
	}
	trig, pairs := c.triggers(ix, handled)
	st.Triggers += len(trig)
	st.HandledPairs += pairs
	return append(out, trig...)
}

// isSimpleStmt reports whether n is a simple statement: one that holds no
// block of its own.
func isSimpleStmt(n ast.Node) bool {
	switch n.(type) {
	case *ast.ExprStmt, *ast.AssignStmt, *ast.SendStmt, *ast.IncDecStmt, *ast.DeclStmt, *ast.ReturnStmt, *ast.GoStmt, *ast.DeferStmt:
		return true
	}
	return false
}

// receiveRange is a receive in a function's syntax and the range of code
// its message makes run: [from, to). For a plain receive the range starts
// after the receive expression; a call that takes the received value as an
// argument (append(errs, <-ch)) is where the value flows (flows_to,
// received), not code in the range.
type receiveRange struct {
	anchor   token.Pos // the receive's arrow, or the for keyword of a range
	from, to token.Pos
}

// receives finds the receives of fn's own syntax (not of the function
// literals in it, which are functions of their own) and their ranges.
func (c *chanLinker) receives(fn *ssa.Function) []receiveRange {
	var body *ast.BlockStmt
	switch s := fn.Syntax().(type) {
	case *ast.FuncDecl:
		body = s.Body
	case *ast.FuncLit:
		body = s.Body
	}
	if body == nil {
		return nil
	}
	// A range over a channel is a receive at its for keyword in SSA.
	ranged := map[token.Pos]bool{}
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			if u, ok := in.(*ssa.UnOp); ok && u.Op == token.ARROW && u.CommaOk {
				ranged[u.Pos()] = true
			}
		}
	}
	var out []receiveRange
	var block func(list []ast.Stmt, end token.Pos)
	stmt := func(st ast.Stmt, end token.Pos) {
		// simple is the stack of the simple statements around the node:
		// a plain receive's range starts after the innermost one.
		var simple []ast.Stmt
		var stack []ast.Node
		ast.Inspect(st, func(x ast.Node) bool {
			if x == nil {
				if top := stack[len(stack)-1]; len(simple) > 0 && simple[len(simple)-1] == top {
					simple = simple[:len(simple)-1]
				}
				stack = stack[:len(stack)-1]
				return true
			}
			if isSimpleStmt(x) {
				simple = append(simple, x.(ast.Stmt))
			}
			stack = append(stack, x)
			descend := true
			defer func() {
				if !descend {
					if len(simple) > 0 && simple[len(simple)-1] == x {
						simple = simple[:len(simple)-1]
					}
					stack = stack[:len(stack)-1]
				}
			}()
			switch y := x.(type) {
			case *ast.FuncLit:
				descend = false
			case *ast.BlockStmt:
				block(y.List, y.Rbrace)
				descend = false
			case *ast.CaseClause:
				block(y.Body, y.End())
				descend = false
			case *ast.CommClause:
				if y.Comm != nil {
					ast.Inspect(y.Comm, func(z ast.Node) bool {
						if u, ok := z.(*ast.UnaryExpr); ok && u.Op == token.ARROW {
							out = append(out, receiveRange{anchor: u.OpPos, from: y.Colon + 1, to: y.End()})
						}
						return true
					})
				}
				block(y.Body, y.End())
				descend = false
			case *ast.RangeStmt:
				if ranged[y.For] {
					out = append(out, receiveRange{anchor: y.For, from: y.Body.Lbrace, to: y.Body.Rbrace})
				}
			case *ast.UnaryExpr:
				if y.Op == token.ARROW {
					// The rest of the block after the statement that holds
					// the receive (<-c.quit <- nil sends in that statement,
					// not after it); a receive in a condition (if <-ch {...})
					// is followed by the rest of its statement.
					from := y.End()
					if len(simple) > 0 {
						from = simple[len(simple)-1].End()
					}
					out = append(out, receiveRange{anchor: y.OpPos, from: from, to: end})
				}
			}
			return descend
		})
	}
	block = func(list []ast.Stmt, end token.Pos) {
		for _, st := range list {
			stmt(st, end)
		}
	}
	block(body.List, body.Rbrace)
	return out
}

// handledItem is a node in a receive's range and what the code does there.
type handledItem struct {
	node *types.Node
	kind string
	line int
}

// handledIn returns the code of fn in the range of receive r: its calls,
// channel operations, go statements and field uses.
func (c *chanLinker) handledIn(fn *ssa.Function, r receiveRange, ix *siteIndex) []handledItem {
	var out []handledItem
	seen := map[[2]any]bool{}
	add := func(n *types.Node, kind string, pos token.Pos) {
		if n == nil || seen[[2]any{n, kind}] {
			return
		}
		seen[[2]any{n, kind}] = true
		out = append(out, handledItem{node: n, kind: kind, line: c.prog.Fset.Position(pos).Line})
	}
	in := func(p token.Pos) bool { return p.IsValid() && p >= r.from && p < r.to }
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch x := instr.(type) {
			case *ssa.Go:
				if p := x.Pos(); in(p) {
					pp := c.prog.Fset.Position(p)
					add(ix.gors[ix.file(pp)][pp.Offset], handlesGo, p)
				}
			case ssa.CallInstruction:
				p := x.Common().Pos()
				if !p.IsValid() {
					continue
				}
				if in(p) {
					add(ix.site(c.prog.Fset.Position(p), "", false), handlesCall, p)
				}
			case *ssa.Panic:
				// panic(v) is a call site in the syntax, an instruction of
				// its own in SSA.
				if p := x.Pos(); in(p) {
					add(ix.site(c.prog.Fset.Position(p), "", false), handlesCall, p)
				}
			case *ssa.Send:
				if p := x.Pos(); in(p) {
					add(ix.site(c.prog.Fset.Position(p), subKindSend, false), handlesSend, p)
				}
			case *ssa.UnOp:
				if p := x.Pos(); x.Op == token.ARROW && in(p) {
					add(ix.recvSite(c.prog.Fset.Position(p)), handlesRecv, p)
				}
			case *ssa.Select:
				for _, s := range x.States {
					if !in(s.Pos) {
						continue
					}
					if s.Dir == gotypes.SendOnly {
						add(ix.site(c.prog.Fset.Position(s.Pos), subKindSend, false), handlesSend, s.Pos)
					} else {
						add(ix.recvSite(c.prog.Fset.Position(s.Pos)), handlesRecv, s.Pos)
					}
				}
			case *ssa.FieldAddr:
				if p := x.Pos(); in(p) {
					kind := handlesRead
					if refs := x.Referrers(); refs != nil {
						for _, r := range *refs {
							if st, ok := r.(*ssa.Store); ok && st.Addr == x {
								kind = handlesWrite
							}
						}
					}
					add(c.explicitField(fn, x.X.Type(), x.Field, p, kind == handlesWrite), kind, p)
				}
			case *ssa.Field:
				if p := x.Pos(); in(p) {
					add(c.explicitField(fn, x.X.Type(), x.Field, p, false), handlesRead, p)
				}
			}
		}
	}
	return out
}

// receivedFlows returns the flows_to edges from each receive of fn to the
// uses its value reaches: a field it is stored in, a call it is passed to,
// a send it is sent on. A receive from a channel of empty structs (a
// signal) carries no value and gets none.
func (c *chanLinker) receivedFlows(fn *ssa.Function, ix *siteIndex) []chanOp {
	var out []chanOp
	emit := func(site *types.Node, values []ssa.Value) {
		seen := map[*types.Node]bool{}
		for _, v := range values {
			c.uses(v, 0, map[ssa.Value]bool{}, func(n *types.Node, pos token.Pos) {
				if n == nil || n == site || seen[n] {
					return
				}
				seen[n] = true
				out = append(out, chanOp{src: site, dst: n, typ: types.EdgeFlowsTo, kind: receivedFlow,
					line: c.prog.Fset.Position(pos).Line, file: site.FilePath})
			}, ix)
		}
	}
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch x := instr.(type) {
			case *ssa.UnOp:
				if x.Op != token.ARROW || signal(x.X.Type()) {
					continue
				}
				site := ix.recvSite(c.prog.Fset.Position(x.Pos()))
				if site == nil {
					continue
				}
				if !x.CommaOk {
					emit(site, []ssa.Value{x})
					continue
				}
				emit(site, extracts(x, 0))
			case *ssa.Select:
				k := 0
				for _, s := range x.States {
					if s.Dir == gotypes.SendOnly {
						continue
					}
					// The select's tuple is (index, ok, the received values
					// in the order of the receive cases).
					idx := 2 + k
					k++
					if signal(s.Chan.Type()) {
						continue
					}
					if site := ix.recvSite(c.prog.Fset.Position(s.Pos)); site != nil {
						emit(site, extracts(x, idx))
					}
				}
			}
		}
	}
	return out
}

// signal reports whether t is a channel of empty structs: its messages
// carry no value.
func signal(t gotypes.Type) bool {
	ch, ok := t.Underlying().(*gotypes.Chan)
	if !ok {
		return false
	}
	s, ok := ch.Elem().Underlying().(*gotypes.Struct)
	return ok && s.NumFields() == 0
}

// extracts returns the values that take element i of tuple v.
func extracts(v ssa.Value, i int) []ssa.Value {
	var out []ssa.Value
	if refs := v.Referrers(); refs != nil {
		for _, r := range *refs {
			if e, ok := r.(*ssa.Extract); ok && e.Index == i {
				out = append(out, e)
			}
		}
	}
	return out
}

// uses follows value v through copies and reports each use the graph has a
// node for: the field it is stored in, the call site it is passed to, the
// send site it is sent on.
func (c *chanLinker) uses(v ssa.Value, depth int, busy map[ssa.Value]bool, report func(*types.Node, token.Pos), ix *siteIndex) {
	if depth > receivedFlowDepth || busy[v] {
		return
	}
	busy[v] = true
	refs := v.Referrers()
	if refs == nil {
		return
	}
	for _, r := range *refs {
		switch x := r.(type) {
		case *ssa.Store:
			if x.Val != v {
				continue
			}
			switch a := x.Addr.(type) {
			case *ssa.FieldAddr:
				report(c.field(a.X.Type(), a.Field), x.Pos())
			case *ssa.Alloc:
				c.cellUses(a, depth, busy, report, ix)
			case *ssa.IndexAddr:
				// An element of an array a variadic call (t.Errorf("%v", v),
				// append(xs, v)) builds: the value goes where the slice of
				// that array goes.
				if arr, ok := a.X.(*ssa.Alloc); ok {
					if refs := arr.Referrers(); refs != nil {
						for _, r := range *refs {
							if sl, ok := r.(*ssa.Slice); ok {
								c.uses(sl, depth+1, busy, report, ix)
							}
						}
					}
				}
			}
		case ssa.CallInstruction:
			// An argument or the receiver of a call (a method on the value).
			if p := x.Common().Pos(); p.IsValid() {
				report(ix.site(c.prog.Fset.Position(p), "", false), p)
			}
		case *ssa.Send:
			if x.X == v {
				report(ix.site(c.prog.Fset.Position(x.Pos()), subKindSend, false), x.Pos())
			}
		case *ssa.Select:
			// case out <- v: sends the value.
			for _, st := range x.States {
				if st.Dir == gotypes.SendOnly && st.Send == v {
					report(ix.site(c.prog.Fset.Position(st.Pos), subKindSend, false), st.Pos)
				}
			}
		case *ssa.Phi, *ssa.ChangeType, *ssa.MakeInterface, *ssa.Convert, *ssa.ChangeInterface, *ssa.Extract,
			*ssa.TypeAssert, *ssa.SliceToArrayPointer, *ssa.MultiConvert:
			c.uses(x.(ssa.Value), depth+1, busy, report, ix)
		case *ssa.Field:
			// An embedded field a promoted method is called through
			// (pd.ID() with ID of an embedded type) carries the value on.
			if embeddedField(x.X.Type(), x.Field) {
				c.uses(x, depth+1, busy, report, ix)
			}
		case *ssa.FieldAddr:
			if embeddedField(x.X.Type(), x.Field) {
				c.uses(x, depth+1, busy, report, ix)
			}
		}
	}
}

// senders returns who sends when fn sends: the goroutines whose own code fn
// is when fn is a goroutine's literal (or a literal it calls in place: the
// enclosing function only starts the goroutine); else the function fn is
// written in, and the goroutines that run it (go f(x) on that function, or
// a goroutine whose code reaches fn).
func (c *chanLinker) senders(fn *ssa.Function) []*types.Node {
	if lit := c.literalGoroutines(fn); len(lit) > 0 {
		return lit
	}
	gors := c.goroutinesOf(fn)
	if n := c.nodeOf(fn); n != nil {
		return append([]*types.Node{n}, gors...)
	}
	return gors
}

// cellUses follows a value stored in the local cell a: where its loads go,
// and the calls that take the cell's address (a method with a pointer
// receiver called on the local).
func (c *chanLinker) cellUses(a *ssa.Alloc, depth int, busy map[ssa.Value]bool, report func(*types.Node, token.Pos), ix *siteIndex) {
	lr := a.Referrers()
	if lr == nil {
		return
	}
	for _, l := range *lr {
		switch u := l.(type) {
		case *ssa.UnOp:
			if u.Op == token.MUL {
				c.uses(u, depth+1, busy, report, ix)
			}
		case ssa.CallInstruction:
			if p := u.Common().Pos(); p.IsValid() {
				report(ix.site(c.prog.Fset.Position(p), "", false), p)
			}
		case *ssa.FieldAddr:
			if embeddedField(u.X.Type(), u.Field) {
				c.uses(u, depth+1, busy, report, ix)
			}
		}
	}
}

// embeddedField reports whether field i of the struct t (or *t) denotes is
// embedded.
func embeddedField(t gotypes.Type, i int) bool {
	_, st := literalStruct(t)
	return st != nil && i < st.NumFields() && st.Field(i).Embedded()
}

// triggers returns, for each channel origin, an edge from every sender on
// it (senders)
// to every receive site on it, and the number of sender x handled-code
// pairs these edges and the receives' handles edges stand for.
func (c *chanLinker) triggers(ix *siteIndex, handled map[*types.Node]int) ([]chanOp, int) {
	type send struct {
		sender *types.Node
		line   int
		file   string
	}
	channel := c.channelIdentity()
	senders := map[*types.Node][]send{}
	receivers := map[*types.Node][]*types.Node{}
	sendSeen := map[*types.Node]map[send]bool{}
	recvSeen := map[*types.Node]map[*types.Node]bool{}
	var channels []*types.Node
	known := map[*types.Node]bool{}
	for _, ev := range c.events {
		if _, isLock := c.locks[ev.pos]; isLock {
			continue // a lock's token wakes no handled code (chanlock.go)
		}
		p := c.prog.Fset.Position(ev.pos)
		for _, o := range ev.origins {
			ch := channel(o.node)
			if !known[ch] {
				known[ch] = true
				channels = append(channels, ch)
				sendSeen[ch], recvSeen[ch] = map[send]bool{}, map[*types.Node]bool{}
			}
			switch ev.typ {
			case types.EdgeSendsTo:
				for _, s := range c.senders(ev.fn) {
					// One send names one channel once, whichever of its
					// names (origins) it reaches.
					if snd := (send{sender: s, line: p.Line, file: ix.file(p)}); s != nil && !sendSeen[ch][snd] {
						sendSeen[ch][snd] = true
						senders[ch] = append(senders[ch], snd)
					}
				}
			case types.EdgeRecvsFrom:
				if site := ix.recvSite(p); site != nil && !recvSeen[ch][site] {
					recvSeen[ch][site] = true
					receivers[ch] = append(receivers[ch], site)
				}
			}
		}
	}
	var out []chanOp
	pairs := 0
	for _, ch := range channels {
		distinct := map[*types.Node]bool{}
		for _, s := range senders[ch] {
			distinct[s.sender] = true
			for _, r := range receivers[ch] {
				out = append(out, chanOp{src: s.sender, dst: r, typ: types.EdgeTriggers, kind: ch.QualifiedName, line: s.line, file: s.file})
			}
		}
		for _, r := range receivers[ch] {
			pairs += len(distinct) * handled[r]
		}
	}
	return out, pairs
}

// channelIdentity returns the identity of the channel an origin names: the
// origins the channel aliases join (a make site stored in a field, the field
// copied to a package-level variable) are one channel. Its node is the make
// site when the group has one, else the name that sorts first, so senders
// and receivers that name one channel differently are joined.
func (c *chanLinker) channelIdentity() func(*types.Node) *types.Node {
	parent := map[string]string{}
	var find func(string) string
	find = func(id string) string {
		p, ok := parent[id]
		if !ok || p == id {
			return id
		}
		r := find(p)
		parent[id] = r
		return r
	}
	better := func(a, b *types.Node) bool { // a represents the group before b
		if (a.Type == types.NodeChannel) != (b.Type == types.NodeChannel) {
			return a.Type == types.NodeChannel
		}
		if a.QualifiedName != b.QualifiedName {
			return a.QualifiedName < b.QualifiedName
		}
		return a.ID < b.ID
	}
	pairs := make([][2]string, 0, len(c.aliases))
	for k := range c.aliases {
		pairs = append(pairs, k)
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i][0] < pairs[j][0] || pairs[i][0] == pairs[j][0] && pairs[i][1] < pairs[j][1]
	})
	for _, k := range pairs {
		a, b := find(k[0]), find(k[1])
		if a == b {
			continue
		}
		if better(c.aliasNodes[a], c.aliasNodes[b]) {
			parent[b] = a
		} else {
			parent[a] = b
		}
	}
	return func(n *types.Node) *types.Node {
		if r, ok := c.aliasNodes[find(n.ID)]; ok {
			return r
		}
		return n
	}
}
