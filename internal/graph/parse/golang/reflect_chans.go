package golang

import (
	"go/token"
	gotypes "go/types"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// reflect_chans.go follows channels operated on through package reflect
// (schema 1.45), the publish/subscribe pattern of event.Feed and the like:
// Subscribe(ch) stores reflect.ValueOf(ch) into a field (directly, or as
// the Chan of a reflect.SelectCase kept in a field), and Send delivers with
// reflect.Select on the cases loaded from that field. The rule:
//
//   - A field whose type holds reflect.Value or reflect.SelectCase (a
//     slice of them, a struct of them) is reflect-held: it holds every
//     channel whose value flows into a reflect.ValueOf call whose result
//     is stored into the field (through a SelectCase, append, a local
//     cell), and every channel a reflect-held field copied into it holds.
//     The Dir written into the same SelectCase, when it is a constant,
//     says whether the case sends or receives on that channel.
//   - reflect.Select on cases loaded from reflect-held fields sends on
//     each channel held there for sending (or of unknown direction) and
//     receives from each held for receiving (or unknown); Value.Send and
//     Value.TrySend on a value loaded from such a field send,
//     Value.Recv and Value.TryRecv receive. These are sends_to and
//     recvs_from edges of kind reflect from the function that does them,
//     and triggers follow from them by the usual rule.
//
// The holder is the field, as for every channel: all values of the type
// that declares it are one holder, so every channel subscribed to any feed
// of a type is one a Send on any feed of that type delivers to.

// reflectPkgPath is the import path of package reflect.
const reflectPkgPath = "reflect"

// The constants of reflect.SelectDir.
const (
	reflectSelectSend = 1
	reflectSelectRecv = 2
)

// reflectKind is the dispatch kind of a sends_to or recvs_from edge of an
// operation done through reflection.
const reflectKind = types.DispatchReflect

// heldChan is a channel origin a reflect-held field holds, with the
// direction of its case (0: unknown).
type heldChan struct {
	origin chanOrigin
	dir    int64
}

// reflectFlow is the state of one pass.
type reflectFlow struct {
	c       *chanLinker
	held    map[string]map[string]heldChan // field id -> channel key -> channel
	copies  map[string]map[string]bool     // field id -> fields copied into it
	fields  map[string]*types.Node
	visited map[ssa.Value]bool
}

// reflectChannels adds the channel operations funcs do through reflection
// (see the rule above) to c.events and c.ops.
func (c *chanLinker) reflectChannels(funcs map[*ssa.Function]bool, nodes []types.Node) {
	fns := make([]*ssa.Function, 0, len(funcs))
	for fn := range funcs {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].String() < fns[j].String() })
	r := &reflectFlow{c: c, held: map[string]map[string]heldChan{}, copies: map[string]map[string]bool{}, fields: map[string]*types.Node{}}
	for _, fn := range fns {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if st, ok := in.(*ssa.Store); ok {
					r.store(st)
				}
			}
		}
	}
	r.close()
	ix := newSiteIndex(nodes)
	for _, fn := range fns {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if call, ok := in.(ssa.CallInstruction); ok {
					r.use(fn, call, ix)
				}
			}
		}
	}
}

// holdsReflectValue reports whether t holds reflect.Value or
// reflect.SelectCase values.
func holdsReflectValue(t gotypes.Type) bool {
	switch x := gotypes.Unalias(t).(type) {
	case *gotypes.Named:
		if pkg, name := namedOf(x); pkg == reflectPkgPath && (name == "Value" || name == "SelectCase") {
			return true
		}
		return holdsReflectValue(x.Underlying())
	case *gotypes.Slice:
		return holdsReflectValue(x.Elem())
	case *gotypes.Array:
		return holdsReflectValue(x.Elem())
	case *gotypes.Pointer:
		return holdsReflectValue(x.Elem())
	}
	return false
}

// store records what a store into a reflect-held field puts there.
func (r *reflectFlow) store(st *ssa.Store) {
	fa, ok := st.Addr.(*ssa.FieldAddr)
	if !ok {
		return
	}
	ptr, ok := fa.X.Type().Underlying().(*gotypes.Pointer)
	if !ok {
		return
	}
	s, ok := ptr.Elem().Underlying().(*gotypes.Struct)
	if !ok || fa.Field >= s.NumFields() || !holdsReflectValue(s.Field(fa.Field).Type()) {
		return
	}
	field := r.c.field(fa.X.Type(), fa.Field)
	if field == nil {
		return
	}
	r.fields[field.ID] = field
	r.visited = map[ssa.Value]bool{}
	chans, from := r.slice(st.Val, 0, 0)
	for _, h := range chans {
		r.hold(field.ID, h)
	}
	for _, f := range from {
		if f != field.ID {
			if r.copies[field.ID] == nil {
				r.copies[field.ID] = map[string]bool{}
			}
			r.copies[field.ID][f] = true
		}
	}
}

func (r *reflectFlow) hold(field string, h heldChan) {
	if r.held[field] == nil {
		r.held[field] = map[string]heldChan{}
	}
	k := h.origin.node.ID
	if old, ok := r.held[field][k]; ok && old.dir != h.dir {
		h.dir = 0
	}
	r.held[field][k] = h
}

// close adds to each field what the fields copied into it hold, until
// nothing changes.
func (r *reflectFlow) close() {
	ids := make([]string, 0, len(r.copies))
	for id := range r.copies {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for changed := true; changed; {
		changed = false
		for _, dst := range ids {
			for _, src := range sortedKeys(r.copies[dst]) {
				for _, k := range sortedHeld(r.held[src]) {
					if _, ok := r.held[dst][k]; !ok {
						r.hold(dst, r.held[src][k])
						changed = true
					}
				}
			}
		}
	}
}

func sortedHeld(m map[string]heldChan) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// slice follows v back to the channels whose reflect.Value it can hold
// (with the direction set beside them, dir) and the reflect-held fields it
// is loaded from.
func (r *reflectFlow) slice(v ssa.Value, dir int64, depth int) ([]heldChan, []string) {
	if v == nil || depth > chanDepth || r.visited[v] {
		return nil, nil
	}
	r.visited[v] = true
	var chans []heldChan
	var from []string
	add := func(c []heldChan, f []string) {
		chans = append(chans, c...)
		from = append(from, f...)
	}
	switch x := v.(type) {
	case *ssa.Call:
		if fn := x.Call.StaticCallee(); fn != nil && fn.Pkg != nil && fn.Pkg.Pkg.Path() == reflectPkgPath && fn.Name() == "ValueOf" &&
			len(x.Call.Args) == 1 {
			for _, o := range r.c.origins(x.Call.Args[0], 0, map[ssa.Value]bool{}) {
				chans = append(chans, heldChan{origin: o, dir: dir})
			}
			return chans, from
		}
		if b, ok := x.Call.Value.(*ssa.Builtin); ok && b.Name() == "append" {
			for _, a := range x.Call.Args {
				add(r.slice(a, dir, depth+1))
			}
		}
	case *ssa.UnOp:
		if x.Op != token.MUL {
			return nil, nil
		}
		if fa, ok := x.X.(*ssa.FieldAddr); ok {
			if f := r.c.field(fa.X.Type(), fa.Field); f != nil && r.fields[f.ID] != nil || f != nil && holdsReflectValue(x.Type()) {
				return nil, []string{f.ID}
			}
		}
		add(r.slice(x.X, dir, depth+1))
	case *ssa.Alloc:
		add(r.cellValues(x, dir, depth))
	case *ssa.IndexAddr:
		add(r.slice(x.X, dir, depth+1))
	case *ssa.FieldAddr:
		if f := r.c.field(x.X.Type(), x.Field); f != nil && holdsReflectValue(x.Type()) {
			from = append(from, f.ID)
		}
		add(r.slice(x.X, dir, depth+1))
	case *ssa.Slice:
		add(r.slice(x.X, dir, depth+1))
	case *ssa.Index:
		add(r.slice(x.X, dir, depth+1))
	case *ssa.Field:
		add(r.slice(x.X, dir, depth+1))
	case *ssa.Phi:
		for _, e := range x.Edges {
			add(r.slice(e, dir, depth+1))
		}
	case *ssa.ChangeType:
		add(r.slice(x.X, dir, depth+1))
	case *ssa.MakeInterface:
		add(r.slice(x.X, dir, depth+1))
	}
	return chans, from
}

// cellValues follows the values stored into a local cell and into its
// elements and fields; a reflect.SelectCase cell gives the channels stored
// into its Chan the direction a constant stored into its Dir says.
func (r *reflectFlow) cellValues(a *ssa.Alloc, dir int64, depth int) ([]heldChan, []string) {
	var chans []heldChan
	var from []string
	addrs := []ssa.Value{a}
	if refs := a.Referrers(); refs != nil {
		for _, ref := range *refs {
			switch x := ref.(type) {
			case *ssa.FieldAddr:
				addrs = append(addrs, x)
				if d, ok := constDir(x); ok {
					dir = d
				}
			case *ssa.IndexAddr:
				addrs = append(addrs, x)
			}
		}
	}
	for _, addr := range addrs {
		if refs := addr.Referrers(); refs != nil {
			for _, ref := range *refs {
				if st, ok := ref.(*ssa.Store); ok && st.Addr == addr {
					c, f := r.slice(st.Val, dir, depth+1)
					chans, from = append(chans, c...), append(from, f...)
				}
			}
		}
	}
	return chans, from
}

// constDir returns the constant stored into fa when fa is the Dir field of
// a reflect.SelectCase.
func constDir(fa *ssa.FieldAddr) (int64, bool) {
	ptr, ok := fa.X.Type().Underlying().(*gotypes.Pointer)
	if !ok {
		return 0, false
	}
	if pkg, name := namedOf(ptr.Elem()); pkg != reflectPkgPath || name != "SelectCase" {
		return 0, false
	}
	s := ptr.Elem().Underlying().(*gotypes.Struct)
	if fa.Field >= s.NumFields() || s.Field(fa.Field).Name() != "Dir" || fa.Referrers() == nil {
		return 0, false
	}
	for _, ref := range *fa.Referrers() {
		if st, ok := ref.(*ssa.Store); ok {
			if k, isConst := st.Val.(*ssa.Const); isConst && k.Value != nil {
				return k.Int64(), true
			}
		}
	}
	return 0, false
}

// use records the channel operations a call of reflect.Select or of a
// reflect.Value method does on reflect-held channels.
func (r *reflectFlow) use(fn *ssa.Function, call ssa.CallInstruction, ix *siteIndex) {
	common := call.Common()
	callee := common.StaticCallee()
	if callee == nil || callee.Pkg == nil || callee.Pkg.Pkg.Path() != reflectPkgPath || len(common.Args) == 0 {
		return
	}
	send, recv := false, false
	switch callee.Name() {
	case "Select":
		send, recv = true, true
	case "Send", "TrySend":
		send = callee.Signature.Recv() != nil
	case "Recv", "TryRecv":
		recv = callee.Signature.Recv() != nil
	default:
		return
	}
	r.visited = map[ssa.Value]bool{}
	chans, from := r.slice(common.Args[0], 0, 0)
	for _, f := range from {
		for _, k := range sortedHeld(r.held[f]) {
			chans = append(chans, r.held[f][k])
		}
	}
	if len(chans) == 0 {
		return
	}
	pos := call.Pos()
	site := ix.site(r.c.prog.Fset.Position(pos), "", false)
	seen := map[string]bool{}
	for _, h := range chans {
		for _, op := range []struct {
			on  bool
			dir int64
			typ types.EdgeType
		}{{send, reflectSelectSend, types.EdgeSendsTo}, {recv, reflectSelectRecv, types.EdgeRecvsFrom}} {
			if !op.on || h.dir != 0 && h.dir != op.dir {
				continue
			}
			k := string(op.typ) + "|" + h.origin.node.ID
			if seen[k] {
				continue
			}
			seen[k] = true
			r.c.reflectOp(fn, op.typ, pos, h.origin, site)
		}
	}
}

// reflectOp adds one channel operation done through reflection: the event
// (for triggers) and the edge from the function and its goroutines.
func (c *chanLinker) reflectOp(fn *ssa.Function, typ types.EdgeType, pos token.Pos, o chanOrigin, site *types.Node) {
	src := c.nodeOf(fn)
	if src == nil {
		return
	}
	line := c.prog.Fset.Position(pos).Line
	c.events = append(c.events, chanEvent{fn: fn, typ: typ, pos: pos, origins: []chanOrigin{o}, site: site})
	c.ops = append(c.ops, chanOp{src: src, dst: o.node, typ: typ, kind: reflectKind, line: line})
	for _, g := range c.goroutinesOf(fn) {
		c.ops = append(c.ops, chanOp{src: g, dst: o.node, typ: typ, kind: reflectKind, line: line, file: src.FilePath})
	}
}
