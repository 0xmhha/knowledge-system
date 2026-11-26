package valueflow

import (
	"regexp"
	"testing"
)

// place names one identifier in a test source: the nth (from 0) whole-word
// occurrence of name. Byte offsets stay out of the tables so a case can be
// read next to its source.
type place struct {
	name string
	nth  int
}

// offset is the retained byte offset of p in code.
func (p place) offset(t *testing.T, code string) int {
	t.Helper()
	found := regexp.MustCompile(`\b`+regexp.QuoteMeta(p.name)+`\b`).FindAllStringIndex(code, -1)
	if p.nth >= len(found) {
		t.Fatalf("%q has %d occurrences of %q, want index %d", code, len(found), p.name, p.nth)
	}
	return retained().StartByte + found[p.nth][0]
}

// sighting is one expected occurrence: the variable declared at declared is
// seen at at with role.
type sighting struct {
	declared, at place
	role         string
}

// reach is one def-use pair: the definition at from reaches the use at to.
type reach struct{ from, to place }

// occurrenceCase states, for one function source, what the engine must list.
type occurrenceCase struct {
	name      string
	code      string
	variables []place
	seen      []sighting
	reaches   []reach
	blocked   []reach
}

// declarations returns the declaration byte offset of every listed variable,
// keyed by variable id.
func declarations(r Result) map[string]int {
	out := map[string]int{}
	for _, v := range r.Variables {
		out[v.ID] = v.Declaration.StartByte
	}
	return out
}

// sightings returns the roles listed per (declaration byte, occurrence byte).
func sightings(r Result) map[[2]int]map[string]bool {
	declared := declarations(r)
	out := map[[2]int]map[string]bool{}
	for _, o := range r.Occurrences {
		key := [2]int{declared[o.Variable], o.Location.StartByte}
		if out[key] == nil {
			out[key] = map[string]bool{}
		}
		out[key][o.Role] = true
	}
	return out
}

// defUsePairs returns every def-use pair as (definition byte, use byte).
func defUsePairs(r Result) map[[2]int]bool {
	at := map[string]int{}
	for _, o := range r.Occurrences {
		at[o.ID] = o.Location.StartByte
	}
	out := map[[2]int]bool{}
	for _, pair := range r.DefUse {
		out[[2]int{at[pair.Definition], at[pair.Use]}] = true
	}
	return out
}

// check compares r with the case. exclusive lists roles that must be the only
// role at their position, so a write recorded as a read is caught too.
func (c occurrenceCase) check(t *testing.T, r Result, exclusive map[string]bool) {
	t.Helper()
	declared := map[int]bool{}
	for _, at := range declarations(r) {
		declared[at] = true
	}
	for _, p := range c.variables {
		if !declared[p.offset(t, c.code)] {
			t.Errorf("variable %s#%d not listed in %v", p.name, p.nth, r.Variables)
		}
	}
	seen, pairs := sightings(r), defUsePairs(r)
	for _, s := range c.seen {
		roles := seen[[2]int{s.declared.offset(t, c.code), s.at.offset(t, c.code)}]
		if !roles[s.role] || (exclusive[s.role] && len(roles) != 1) {
			t.Errorf("%s#%d at %s#%d: roles %v, want %s", s.declared.name, s.declared.nth, s.at.name, s.at.nth, roles, s.role)
		}
	}
	key := func(pair reach) [2]int { return [2]int{pair.from.offset(t, c.code), pair.to.offset(t, c.code)} }
	for _, pair := range c.reaches {
		if !pairs[key(pair)] {
			t.Errorf("missing pair %s#%d -> %s#%d", pair.from.name, pair.from.nth, pair.to.name, pair.to.nth)
		}
	}
	for _, pair := range c.blocked {
		if pairs[key(pair)] {
			t.Errorf("pair %s#%d -> %s#%d must not exist", pair.from.name, pair.from.nth, pair.to.name, pair.to.nth)
		}
	}
}

func TestRangeClauseDefinesItsKeyAndValueOnEachIteration(t *testing.T) {
	tests := []occurrenceCase{
		{
			name: "short declaration reaches the uses in the body",
			code: "func F(xs []int)int{t:=0;for i,v:=range xs{t+=i+v};return t}",
			seen: []sighting{{place{"i", 0}, place{"i", 0}, "definition"}, {place{"v", 0}, place{"v", 0}, "definition"}},
			reaches: []reach{
				{place{"i", 0}, place{"i", 1}}, {place{"v", 0}, place{"v", 1}},
			},
		},
		{
			name:    "index only form",
			code:    "func F(xs []int){for i:=range xs{xs[i]=0}}",
			seen:    []sighting{{place{"i", 0}, place{"i", 0}, "definition"}},
			reaches: []reach{{place{"i", 0}, place{"i", 1}}},
		},
		{
			name: "nested clauses keep the outer binding inside the inner body",
			code: "func F(m map[int][]int)int{t:=0;for o,sub:=range m{for p,n:=range sub{t+=o+p+n};t+=len(sub)};return t}",
			seen: []sighting{
				{place{"o", 0}, place{"o", 0}, "definition"}, {place{"sub", 0}, place{"sub", 0}, "definition"},
				{place{"p", 0}, place{"p", 0}, "definition"}, {place{"n", 0}, place{"n", 0}, "definition"},
			},
			reaches: []reach{
				{place{"o", 0}, place{"o", 1}}, {place{"sub", 0}, place{"sub", 1}}, {place{"sub", 0}, place{"sub", 2}},
				{place{"p", 0}, place{"p", 1}}, {place{"n", 0}, place{"n", 1}},
			},
		},
		{
			// A range over nothing assigns nothing, so the value from before
			// the loop is still a possible value after it, but never inside it.
			name: "assignment form keeps the earlier value after the loop only",
			code: "func F(xs []int)int{k:=-1;for k=range xs{sink(k)};return k}",
			seen: []sighting{{place{"k", 0}, place{"k", 1}, "definition"}},
			reaches: []reach{
				{place{"k", 1}, place{"k", 2}}, {place{"k", 1}, place{"k", 3}}, {place{"k", 0}, place{"k", 3}},
			},
			blocked: []reach{{place{"k", 0}, place{"k", 2}}},
		},
		{
			name:    "a write in the body does not survive into the next iteration",
			code:    "func F(xs []int)int{s:=0;for _,v:=range xs{s+=v;v=0};return s}",
			seen:    []sighting{{place{"v", 0}, place{"v", 0}, "definition"}},
			reaches: []reach{{place{"v", 0}, place{"v", 1}}},
			blocked: []reach{{place{"v", 2}, place{"v", 1}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(t, analyze(t, test.code), map[string]bool{"definition": true})
		})
	}
}

// A literal body gets its own CFG without the branch validation the function
// body goes through, so a branch with no target must still end in an answer.
func TestMalformedBranchInsideLiteralStillListsItsIdentifiers(t *testing.T) {
	tests := []occurrenceCase{
		{name: "break outside a loop", code: "func F(){f:=func(){v:=1;break;sink(v)};sink(f)}"},
		{name: "continue outside a loop", code: "func F(){f:=func(){v:=1;continue;sink(v)};sink(f)}"},
		{name: "goto an undeclared label", code: "func F(){f:=func(){v:=1;goto missing;sink(v)};sink(f)}"},
		{name: "break to an undeclared label", code: "func F(){f:=func(){v:=1;for{break missing};sink(v)};sink(f)}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.variables = []place{{"v", 0}}
			test.seen = []sighting{{place{"v", 0}, place{"v", 0}, "definition"}, {place{"v", 0}, place{"v", 1}, "use"}}
			test.check(t, analyze(t, test.code), nil)
		})
	}
}

func TestFunctionLiteralLocalsAndOccurrencesAreListed(t *testing.T) {
	tests := []occurrenceCase{
		{
			name:      "parameter, local and captured read",
			code:      "func F(x int)int{fn:=func(a int)int{y:=a+x;return y};return fn(1)}",
			variables: []place{{"a", 0}, {"y", 0}},
			seen: []sighting{
				{place{"a", 0}, place{"a", 0}, "definition"}, {place{"a", 0}, place{"a", 1}, "use"},
				{place{"y", 0}, place{"y", 0}, "definition"}, {place{"y", 0}, place{"y", 1}, "use"},
				{place{"x", 0}, place{"x", 1}, "use"},
			},
		},
		{
			name:      "deferred literal writes a named result",
			code:      "func F()(n int){defer func(){if d:=delta();d>0{n=d}}();return 1}",
			variables: []place{{"d", 0}},
			seen: []sighting{
				{place{"d", 0}, place{"d", 0}, "definition"}, {place{"d", 0}, place{"d", 1}, "use"},
				{place{"d", 0}, place{"d", 2}, "use"}, {place{"n", 0}, place{"n", 1}, "definition"},
			},
		},
		{
			// The literal may run any number of times at any later point, so
			// its write is listed but never claimed to reach the caller's read.
			name: "increment is a write and a read that stays out of the caller's flow",
			code: "func F()int{n:=0;inc:=func(){n++};inc();return n}",
			seen: []sighting{
				{place{"n", 0}, place{"n", 1}, "definition"}, {place{"n", 0}, place{"n", 1}, "use"},
			},
			reaches: []reach{{place{"n", 0}, place{"n", 2}}},
			blocked: []reach{{place{"n", 1}, place{"n", 2}}},
		},
		{
			name:      "range clause inside a goroutine literal",
			code:      "func F(xs []int){go func(){for i,v:=range xs{sink(i+v)}}()}",
			variables: []place{{"i", 0}, {"v", 0}},
			seen: []sighting{
				{place{"i", 0}, place{"i", 0}, "definition"}, {place{"i", 0}, place{"i", 1}, "use"},
				{place{"v", 0}, place{"v", 0}, "definition"}, {place{"v", 0}, place{"v", 1}, "use"},
				{place{"xs", 0}, place{"xs", 1}, "reference"},
			},
		},
		{
			name:      "literal nested in a literal",
			code:      "func F(){f:=func(){g:=func(b int)int{return b};sink(g)};sink(f)}",
			variables: []place{{"g", 0}, {"b", 0}},
			seen: []sighting{
				{place{"b", 0}, place{"b", 0}, "definition"}, {place{"b", 0}, place{"b", 1}, "use"},
				{place{"g", 0}, place{"g", 0}, "definition"}, {place{"g", 0}, place{"g", 1}, "reference"},
			},
		},
		{
			name:      "constant declared in a literal",
			code:      "func F()int{f:=func()int{const k=2;return k};return f()}",
			variables: []place{{"k", 0}},
			seen: []sighting{
				{place{"k", 0}, place{"k", 0}, "constant_definition"}, {place{"k", 0}, place{"k", 1}, "use"},
			},
		},
		{
			// A bare return reads the named results of the function it leaves:
			// the literal's own, never the enclosing function's.
			name:      "bare return reads the literal's own named result",
			code:      "func F()(out int){f:=func()(in int){in=1;return};out=f();return}",
			variables: []place{{"in", 0}},
			seen: []sighting{
				{place{"in", 0}, place{"in", 0}, "definition"}, {place{"in", 0}, place{"in", 1}, "definition"},
				{place{"in", 0}, place{"return", 0}, "implicit_return"}, {place{"out", 0}, place{"return", 1}, "implicit_return"},
			},
			reaches: []reach{{place{"out", 1}, place{"return", 1}}},
			blocked: []reach{{place{"out", 0}, place{"return", 0}}, {place{"out", 1}, place{"return", 0}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := analyze(t, test.code)
			if !diag(r, "closure_value_unknown") || r.Status != "partial" {
				t.Errorf("literal boundary not reported: status %s", r.Status)
			}
			test.check(t, r, nil)
		})
	}
}
