// Package local has interface calls whose interface is declared in a
// function or written inline, and a struct that embeds an interface.
package local

// Bytes is implemented by two types.
type word struct{}

func (word) Bytes() []byte { return nil }

type pair struct{}

func (pair) Bytes() []byte { return []byte{1, 2} }

// Pricer is an interface a struct embeds.
type Pricer interface{ Price() int }

// Backend embeds Pricer and declares Price again.
type Backend interface {
	Pricer
	Price() int
	Name() string
}

// embedded satisfies Backend through the interface it embeds: its Price is
// Pricer.Price, which has no body.
type embedded struct{ Pricer }

func (embedded) Name() string { return "embedded" }

type fixed struct{}

func (fixed) Price() int   { return 1 }
func (fixed) Name() string { return "fixed" }

// Size asserts to an interface written inline.
func Size(v any) int {
	if b, ok := v.(interface{ Bytes() []byte }); ok {
		return len(b.Bytes())
	}
	return 0
}

// Count calls through an interface declared in the function.
func Count(v any) int {
	type sized interface{ Bytes() []byte }
	if s, ok := v.(sized); ok {
		return len(s.Bytes())
	}
	return 0
}

// Use calls Backend.Price.
func Use(b Backend) int { return b.Price() }

// Run gives the calls their values.
func Run() int {
	return Size(word{}) + Count(pair{}) + Use(fixed{}) + Use(embedded{fixed{}})
}
