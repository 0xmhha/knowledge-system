package main

import (
	"go/types"
	"sort"
)

// groups is the set of methods that must keep one signature.
type groups struct {
	parent map[*types.Func]*types.Func
}

func (g *groups) find(f *types.Func) *types.Func {
	p, ok := g.parent[f]
	if !ok {
		g.parent[f] = f
		return f
	}
	if p == f {
		return f
	}
	r := g.find(p)
	g.parent[f] = r
	return r
}

func (g *groups) union(a, b *types.Func) { g.parent[g.find(a)] = g.find(b) }

func (p *probe) targets() []Target {
	out := append(p.signatureTargets(), p.reachTargets()...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.QName < b.QName
	})
	return out
}

// members groups the methods that the conversions of the build tie together.
func members(rel map[*types.Func]map[*types.Func]bool) (*groups, map[*types.Func][]*types.Func) {
	g := &groups{parent: map[*types.Func]*types.Func{}}
	for i, ms := range rel {
		for m := range ms {
			g.union(i, m)
		}
	}
	sets := map[*types.Func][]*types.Func{}
	keys := make([]*types.Func, 0, len(g.parent))
	for f := range g.parent {
		keys = append(keys, f)
	}
	for _, f := range keys {
		r := g.find(f)
		sets[r] = append(sets[r], f)
	}
	return g, sets
}

func (p *probe) signatureGold(ms []*types.Func) (gold map[Decl]bool, outside bool) {
	gold = map[Decl]bool{}
	for _, m := range ms {
		if d, ok := p.declAt(m.Pos()); ok {
			gold[d] = true
		} else {
			outside = true
		}
		for d := range p.users[m] {
			gold[d] = true
		}
	}
	return gold, outside
}

func (p *probe) signatureTargets() []Target {
	g, sets := members(p.conv)
	tg, tsets := members(p.byType)
	var out []Target
	for _, pkg := range p.pkgs {
		sc := pkg.Types.Scope()
		for _, name := range sc.Names() {
			tn, ok := sc.Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			it, ok := tn.Type().Underlying().(*types.Interface)
			if !ok {
				continue
			}
			for i := range it.NumExplicitMethods() {
				m := it.ExplicitMethod(i).Origin()
				own, ok := p.declAt(m.Pos())
				if _, tied := g.parent[m]; !ok || !tied {
					continue
				}
				ms := sets[g.find(m)]
				gold, outside := p.signatureGold(ms)
				byType := map[Decl]bool{}
				if _, tied := tg.parent[m]; tied {
					byType, _ = p.signatureGold(tsets[tg.find(m)])
				}
				plain := map[Decl]bool{}
				for d := range p.users[m] {
					plain[d] = true
				}
				delete(gold, own)
				delete(byType, own)
				delete(plain, own)
				file, line, _ := p.rel(m.Pos())
				out = append(out, Target{Kind: KindSignature, QName: pkg.Name + "." + name + "." + m.Name(), File: file, Line: line, Own: own,
					Gold: sorted(gold), Plain: len(plain), TypeGap: diff(gold, byType), Members: len(ms), Outside: outside})
			}
		}
	}
	return out
}

func (p *probe) reachTargets() []Target {
	conv, byType := reverse(p.conv), reverse(p.byType)
	var out []Target
	for d, objs := range p.objs {
		if d.Kind != "func" || len(objs) != 1 {
			continue
		}
		f, ok := objs[0].(*types.Func)
		if !ok {
			continue
		}
		gold := p.reach(f, d, conv)
		file, line, _ := p.rel(f.Pos())
		out = append(out, Target{Kind: KindReach, QName: qname(f), File: file, Line: line, Own: d, Gold: sorted(gold),
			Plain: len(p.reach(f, d, nil)), TypeGap: diff(gold, p.reach(f, d, byType))})
	}
	return out
}

func reverse(m map[*types.Func]map[*types.Func]bool) map[*types.Func][]*types.Func {
	out := map[*types.Func][]*types.Func{}
	for i, ms := range m {
		for f := range ms {
			out[f] = append(out[f], i)
		}
	}
	return out
}

// reach walks uses backwards for reachSteps steps. Going from a method to the
// interface methods in front of it is free, so those are closed over first.
func (p *probe) reach(f *types.Func, own Decl, behind map[*types.Func][]*types.Func) map[Decl]bool {
	seen := map[Decl]bool{own: true}
	front := []types.Object{f}
	for range reachSteps {
		var next []types.Object
		for _, o := range withInterfaces(front, behind) {
			for d := range p.users[o] {
				if d.Kind == "type" || seen[d] {
					continue
				}
				seen[d] = true
				next = append(next, p.objs[d]...)
			}
		}
		front = next
	}
	delete(seen, own)
	return seen
}

func withInterfaces(front []types.Object, behind map[*types.Func][]*types.Func) []types.Object {
	seen := map[types.Object]bool{}
	var out []types.Object
	stack := append([]types.Object(nil), front...)
	for len(stack) > 0 {
		o := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[o] {
			continue
		}
		seen[o] = true
		out = append(out, o)
		if f, ok := o.(*types.Func); ok {
			for _, i := range behind[f.Origin()] {
				stack = append(stack, i)
			}
		}
	}
	return out
}

func qname(f *types.Func) string {
	sig := f.Type().(*types.Signature)
	if r := sig.Recv(); r != nil {
		t := r.Type()
		if pt, ok := t.(*types.Pointer); ok {
			t = pt.Elem()
		}
		if n, ok := t.(*types.Named); ok {
			return f.Pkg().Name() + "." + n.Obj().Name() + "." + f.Name()
		}
	}
	return f.Pkg().Name() + "." + f.Name()
}

func sorted(m map[Decl]bool) []Decl {
	out := make([]Decl, 0, len(m))
	for d := range m {
		out = append(out, d)
	}
	sortDecls(out)
	return out
}

// diff counts the declarations in exactly one of the two sets.
func diff(a, b map[Decl]bool) int {
	n := 0
	for d := range a {
		if !b[d] {
			n++
		}
	}
	for d := range b {
		if !a[d] {
			n++
		}
	}
	return n
}
