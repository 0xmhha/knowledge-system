package stage2

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// One ranked list credits a span once, at its best rank, and later distinct
// spans take the next rank — repeated entries must not sum into an outlier.
func TestAggregator_CreditsSpanOncePerList(t *testing.T) {
	t.Parallel()
	iface := contract.Citation{File: "balancer/balancer.go", StartLine: 151, EndLine: 151}
	impl := contract.Citation{File: "od/balancer.go", StartLine: 520, EndLine: 522}
	a := newAggregator(DefaultRRFK, DefaultBMWeight, DefaultSymbolWeight, DefaultCkvWeight)
	a.addSymbolList("RemoveSubConn", []contract.Citation{iface, iface, iface, impl, iface})
	a.addBM25List("RemoveSubConn", []contract.Hit{{Citation: iface}, {Citation: iface}})
	a.addCkvList([]contract.Hit{{Citation: impl}, {Citation: impl}})

	got := map[string]ScoredCitation{}
	for _, c := range a.results(0, false, false) {
		got[c.Citation.Key()] = c
	}
	wantIface := DefaultSymbolWeight/float64(DefaultRRFK+1) + DefaultBMWeight/float64(DefaultRRFK+1)
	wantImpl := DefaultSymbolWeight/float64(DefaultRRFK+2) + DefaultCkvWeight/float64(DefaultRRFK+1)
	if s := got[iface.Key()]; len(s.Sources) != 2 || !near(s.Score, wantIface) {
		t.Errorf("iface score=%v sources=%v, want %v from one credit per list", s.Score, s.Sources, wantIface)
	}
	if s := got[impl.Key()]; len(s.Sources) != 2 || !near(s.Score, wantImpl) {
		t.Errorf("impl score=%v sources=%v, want %v (symbol rank 2, ckv rank 1)", s.Score, s.Sources, wantImpl)
	}
}

func near(a, b float64) bool { d := a - b; return d < 1e-12 && d > -1e-12 }
