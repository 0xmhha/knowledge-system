package suspects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

const head = "1111111111111111111111111111111111111111"

func cite(file string) contract.Citation {
	return contract.Citation{File: file, StartLine: 1, EndLine: 5, CommitHash: head}
}

type graph struct {
	*ckgclient.Fake
	edges      map[string][]contract.Neighbor
	impact     func(string) (contract.ImpactResult, error)
	concurrent func(string) (contract.ConcurrencyResult, error)
	health     func(int) ckgclient.Health
	delay      time.Duration
}

func (g *graph) Neighbors(ctx context.Context, c contract.Citation, o ckgclient.NeighborsOpts) ([]contract.Neighbor, error) {
	if g.delay > 0 {
		time.Sleep(g.delay)
	} // emulate old non-cancellable stores
	g.Calls.Neighbors = append(g.Calls.Neighbors, ckgclient.NeighborsCall{Src: c, Opts: o})
	return g.edges[c.String()+string(o.Relations[0])], nil
}
func (g *graph) ImpactOfChange(ctx context.Context, name string, o ckgclient.ImpactOpts) (contract.ImpactResult, error) {
	if g.impact != nil {
		return g.impact(name)
	}
	return g.Fake.ImpactOfChange(ctx, name, o)
}
func (g *graph) ConcurrencyImpact(ctx context.Context, name string, o ckgclient.ConcurrencyOpts) (contract.ConcurrencyResult, error) {
	if g.concurrent != nil {
		return g.concurrent(name)
	}
	return g.Fake.ConcurrencyImpact(ctx, name, o)
}
func (g *graph) Health(ctx context.Context) (ckgclient.Health, error) {
	h, e := g.Fake.Health(ctx)
	if g.health != nil {
		h = g.health(g.Calls.Health)
	}
	return h, e
}
func fixture() (*graph, *ckvclient.Fake) {
	g := &graph{Fake: &ckgclient.Fake{HealthVal: ckgclient.Health{Reachable: true, IndexedHead: head}, SymbolCitations: []contract.Citation{cite("a.go")}, SymbolIdentities: map[string]ckgclient.SymbolIdentity{}}, edges: map[string][]contract.Neighbor{}}
	for _, file := range []string{"a.go", "b.go", "c.go", "d.go"} {
		c := cite(file)
		g.SymbolIdentities[c.String()] = ckgclient.SymbolIdentity{Symbol: file, Citation: c}
	}
	v := &ckvclient.Fake{HealthVal: ckvclient.Health{Reachable: true, ModelReachable: true, IndexedHead: head}}
	return g, v
}
func addEdge(g *graph, a, b string, rel contract.Relation) {
	from, to := cite(a), cite(b)
	anchor := from
	if rel == contract.RelationCalledBy {
		anchor = to
	}
	k := anchor.String() + string(rel)
	g.edges[k] = append(g.edges[k], contract.Neighbor{Source: from, Target: to, Relation: rel, Distance: 1})
}
func run(t *testing.T, g *graph, v *ckvclient.Fake, r Request) contract.SuspectCandidates {
	t.Helper()
	if r.Symptom == "" {
		r.Symptom = "hang"
	}
	if r.Symbol == "" && r.File == "" && len(r.Expand) == 0 {
		r.Symbol = "a.go"
	}
	out, e := Compose(context.Background(), g, v, r)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func find(t *testing.T, out contract.SuspectCandidates, file string) contract.SuspectCandidate {
	t.Helper()
	for _, c := range out.Candidates {
		if c.Citation.File == file {
			return c
		}
	}
	t.Fatalf("missing %s", file)
	return contract.SuspectCandidate{}
}
func TestBreadthFirstDistancesDirectionsAndCycles(t *testing.T) {
	g, v := fixture()
	addEdge(g, "a.go", "b.go", contract.RelationCalls)
	addEdge(g, "b.go", "c.go", contract.RelationCalls)
	addEdge(g, "c.go", "a.go", contract.RelationCalls)
	addEdge(g, "d.go", "a.go", contract.RelationCalledBy)
	out := run(t, g, v, Request{Depth: 2, MaxEnriched: 4})
	for file, want := range map[string]int{"a.go": 0, "b.go": 1, "c.go": 2, "d.go": 1} {
		c := find(t, out, file)
		if c.Signals.Distance.Value == nil || *c.Signals.Distance.Value != want {
			t.Errorf("%s distance=%+v", file, c.Signals.Distance)
		}
	}
	for _, call := range g.Calls.Neighbors {
		if call.Opts.Hops != 1 {
			t.Fatal("flattened depth query used")
		}
	}
	if got := find(t, out, "d.go").Signals.Relations; got.Value == nil || (*got.Value)[0] != contract.RelationCalledBy {
		t.Fatalf("caller direction: %+v", got)
	}
}
func TestSeedRelativeMembershipNotCandidateOutgoingImpact(t *testing.T) {
	g, v := fixture()
	addEdge(g, "a.go", "b.go", contract.RelationCalls)
	g.impact = func(name string) (contract.ImpactResult, error) {
		if name != "a.go" {
			t.Fatalf("candidate outgoing impact queried: %s", name)
		}
		return contract.ImpactResult{Groups: []contract.ImpactGroup{{Category: contract.ImpactCallers, Hits: []contract.Citation{cite("b.go")}}}}, nil
	}
	g.concurrent = func(name string) (contract.ConcurrencyResult, error) {
		if name != "a.go" {
			t.Fatal(name)
		}
		return contract.ConcurrencyResult{Modules: []contract.ConcurrencyModule{{Citation: cite("b.go"), Direction: "affected_by"}}}, nil
	}
	out := run(t, g, v, Request{})
	b := find(t, out, "b.go")
	if b.Signals.Impact.Value == nil || len(*b.Signals.Impact.Value) != 1 || (*b.Signals.Impact.Value)[0] != contract.ImpactCallers {
		t.Fatal(b.Signals.Impact)
	}
	if b.Signals.Concurrency.Value == nil || !b.Signals.Concurrency.Value.Touched || b.Signals.Concurrency.Value.Direction[0] != "affected_by" {
		t.Fatal(b.Signals.Concurrency)
	}
	if b.Signals.Impact.Provenance[0].Anchor != cite("a.go") {
		t.Fatal("wrong signal anchor")
	}
}
func TestMultipleSeedsPreserveProvenanceAndFailures(t *testing.T) {
	for _, failFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(failFirst), func(t *testing.T) {
			g, v := fixture()
			addEdge(g, "a.go", "c.go", contract.RelationCalls)
			addEdge(g, "b.go", "c.go", contract.RelationCalls)
			g.impact = func(name string) (contract.ImpactResult, error) {
				if (name == "a.go") == failFirst {
					return contract.ImpactResult{}, errors.New("secret backend detail")
				}
				return contract.ImpactResult{Groups: []contract.ImpactGroup{{Category: contract.ImpactCallers, Hits: []contract.Citation{cite("c.go")}}}}, nil
			}
			out := run(t, g, v, Request{Expand: []contract.Citation{cite("a.go"), cite("b.go")}, MaxEnriched: 3})
			c := find(t, out, "c.go")
			if !out.Partial || c.Signals.Impact.Value != nil || c.Signals.Impact.Status != "error" || len(c.Signals.Impact.Provenance) != 2 {
				t.Fatalf("failed seed became negative/complete: %+v", c.Signals.Impact)
			}
			if len(c.Signals.Distance.Provenance) != 2 || len(c.Signals.Relations.Provenance) != 2 {
				t.Fatal("multi-seed provenance discarded")
			}
			raw, _ := json.Marshal(out)
			if strings.Contains(string(raw), "secret backend detail") {
				t.Fatal("backend error leaked")
			}
		})
	}
}
func TestUnavailableSignalsAreExplicitNull(t *testing.T) {
	g, v := fixture()
	g.PRErr = errors.New("unavailable")
	g.ConcurrencyErr = errors.New("unavailable")
	out := run(t, g, v, Request{})
	s := out.Candidates[0].Signals
	if s.Security.Value != nil || s.Dataflow.Value != nil || s.Security.Status != "unsupported" || s.Recency.Value != nil || s.Concurrency.Value != nil {
		t.Fatalf("absence turned into safe: %+v", s)
	}
	raw, _ := json.Marshal(s)
	if !strings.Contains(string(raw), `"security_tag":{"value":null`) {
		t.Fatalf("missing explicit null: %s", raw)
	}
}
func TestScopedHistoryCountsAndRecency(t *testing.T) {
	g, v := fixture()
	at := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	g.PRRefs = []contract.PRRef{{MergedAt: at}, {MergedAt: at.Add(-time.Hour)}, {}}
	out := run(t, g, v, Request{})
	r := out.Candidates[0].Signals.Recency.Value
	if r == nil || r.PRCount != 3 || !r.LastMergedAt.Equal(at) || !strings.Contains(r.Scope, "not commit count") {
		t.Fatal(r)
	}
}
func TestTextSeedsKeepBackendScoresAndQuoteKeywords(t *testing.T) {
	g, v := fixture()
	v.SearchHits = []contract.Hit{{Citation: cite("a.go"), Rank: 1, Score: .9, Source: contract.HitSourceCKV}}
	g.BM25Hits = []contract.Hit{{Citation: cite("a.go"), Rank: 1, Score: 37, Source: contract.HitSourceCKG}}
	out, e := Compose(context.Background(), g, v, Request{Symptom: "hang: lock? lock"})
	if e != nil {
		t.Fatal(e)
	}
	got := out.Candidates[0].Signals.Retrieval
	if got.Value == nil || len(*got.Value) != 2 || len(got.Provenance) != 2 || (*got.Value)[1].Score != 37 {
		t.Fatal(got)
	}
	if g.Calls.BM25Search[0].Query != `"hang" OR "lock"` {
		t.Fatal(g.Calls.BM25Search[0].Query)
	}
	if len(out.Candidates) != 1 || out.Partial {
		t.Fatalf("duplicate seed or false cap: %+v", out)
	}
}
func TestInvalidRetrievalAndCrossRevisionCitationsAreOmitted(t *testing.T) {
	g, v := fixture()
	other := cite("b.go")
	other.CommitHash = strings.Repeat("2", 40)
	v.SearchHits = []contract.Hit{{Citation: cite("a.go"), Rank: 1, Score: .5, Source: contract.HitSourceCKV}, {Citation: other, Rank: 2, Score: .2, Source: contract.HitSourceCKV}, {Citation: cite("c.go"), Rank: 3, Score: math.NaN(), Source: contract.HitSourceCKV}}
	out, e := Compose(context.Background(), g, v, Request{Symptom: "hang"})
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Candidates) != 1 || !out.Partial {
		t.Fatal(out)
	}
}
func TestCandidateAndCallBudgets(t *testing.T) {
	g, v := fixture()
	addEdge(g, "a.go", "b.go", contract.RelationCalls)
	addEdge(g, "a.go", "c.go", contract.RelationCalls)
	out := run(t, g, v, Request{MaxCandidates: 2, MaxCalls: 5})
	if len(out.Candidates) > 2 || out.Budget.Calls > 5 || !out.Partial || g.Calls.Health != 2 {
		t.Fatal(out.Budget)
	}
	if find(t, out, "b.go").Signals.Recency.Value != nil {
		t.Fatal("budget exhaustion became zero history")
	}
}
func TestSiteUsesResolvedSymbolSpanAndIterativeExpansion(t *testing.T) {
	g, v := fixture()
	site := contract.Citation{File: "a.go", StartLine: 3, EndLine: 3, CommitHash: head}
	g.SymbolIdentities[site.String()] = g.SymbolIdentities[cite("a.go").String()]
	addEdge(g, "a.go", "b.go", contract.RelationCalls)
	addEdge(g, "b.go", "c.go", contract.RelationCalls)
	first := run(t, g, v, Request{File: "a.go", Line: 3, Depth: 1})
	if first.Candidates[0].Citation != cite("a.go") {
		t.Fatal("site citation not canonical")
	}
	second := run(t, g, v, Request{Expand: []contract.Citation{find(t, first, "b.go").Citation}, Depth: 1})
	if *find(t, second, "c.go").Signals.Distance.Value != 1 {
		t.Fatal(second)
	}
}
func TestRejectsUnanchoredAndMixedRevisionEdges(t *testing.T) {
	g, v := fixture()
	k := cite("a.go").String() + string(contract.RelationCalls)
	bad := cite("d.go")
	bad.CommitHash = strings.Repeat("2", 40)
	g.edges[k] = []contract.Neighbor{{Source: cite("b.go"), Target: cite("c.go"), Relation: contract.RelationCalls, Distance: 1}, {Source: cite("a.go"), Target: bad, Relation: contract.RelationCalls, Distance: 1}}
	out := run(t, g, v, Request{})
	if len(out.Candidates) != 1 || !out.Partial {
		t.Fatal(out)
	}
}
func TestRejectsMixedEnrichmentAndMissingSeedIdentity(t *testing.T) {
	for _, mode := range []string{"impact", "concurrency", "identity"} {
		t.Run(mode, func(t *testing.T) {
			g, v := fixture()
			bad := cite("b.go")
			bad.CommitHash = strings.Repeat("2", 40)
			switch mode {
			case "impact":
				g.ImpactResult = contract.ImpactResult{Groups: []contract.ImpactGroup{{Hits: []contract.Citation{bad}}}}
			case "concurrency":
				g.ConcurrencyResult = contract.ConcurrencyResult{Modules: []contract.ConcurrencyModule{{Citation: bad, Direction: "both"}}}
			case "identity":
				delete(g.SymbolIdentities, cite("a.go").String())
			}
			out := run(t, g, v, Request{})
			s := out.Candidates[0].Signals
			if !out.Partial {
				t.Fatal("missing partial")
			}
			if mode == "concurrency" {
				if s.Concurrency.Value != nil {
					t.Fatal(s)
				}
			} else {
				if s.Impact.Value != nil {
					t.Fatal(s)
				}
			}
		})
	}
}
func TestSnapshotIdentityAndDriftFailClosed(t *testing.T) {
	for _, mode := range []string{"missing", "requested", "vector", "dataset", "drift"} {
		t.Run(mode, func(t *testing.T) {
			g, v := fixture()
			r := Request{Symptom: "hang", Symbol: "a.go"}
			switch mode {
			case "missing":
				g.HealthVal.IndexedHead = ""
			case "requested":
				r.Commit = "other"
			case "vector":
				r.Symbol = ""
				v.HealthVal.IndexedHead = "other"
			case "dataset":
				r.Symbol = ""
				g.HealthVal.DatasetID = "one"
				v.HealthVal.DatasetID = "two"
			case "drift":
				g.health = func(n int) ckgclient.Health {
					h := g.HealthVal
					if n > 1 {
						h.GraphDigest = "changed"
					}
					return h
				}
			}
			if out, e := Compose(context.Background(), g, v, r); e == nil || len(out.Candidates) != 0 {
				t.Fatalf("published unverified snapshot: %v %+v", e, out)
			}
		})
	}
}
func TestPinnedCoordinatesCheckAllComponents(t *testing.T) {
	g, v := fixture()
	want := contract.V2Coordinates{ProjectID: "p", DatasetID: "d", SnapshotID: "s", SourceMode: "working-tree", BaseCommit: head}
	g.HealthVal.ProjectID = "p"
	g.HealthVal.DatasetID = "d"
	g.HealthVal.SnapshotID = "s"
	g.HealthVal.SourceMode = "working-tree"
	out := run(t, g, v, Request{Coordinates: &want})
	if *out.Coordinates != want {
		t.Fatal(out)
	}
	want.DatasetID = "other"
	if _, e := Compose(context.Background(), g, v, Request{Symptom: "hang", Symbol: "a.go", Coordinates: &want}); e == nil {
		t.Fatal("foreign retained dataset accepted")
	}
}
func TestDeadlineDiscardsLateBackendResults(t *testing.T) {
	g, v := fixture()
	g.delay = 15 * time.Millisecond
	addEdge(g, "a.go", "b.go", contract.RelationCalls)
	out, e := Compose(context.Background(), g, v, Request{Symptom: "hang", Symbol: "a.go", BudgetMS: 10})
	if e == nil || len(out.Candidates) != 0 {
		t.Fatal("late unverified results published")
	}
	if len(g.Calls.Neighbors) != 1 {
		t.Fatal("subsequent query started after deadline")
	}
}
func TestInvalidModesPathsAndBoundsRejectedBeforeBackend(t *testing.T) {
	for _, r := range []Request{{Symptom: ""}, {Symptom: "hang", Symbol: "x", File: "a.go", Line: 1}, {Symptom: "hang", File: "../a.go", Line: 1}, {Symptom: "hang", File: "logs/a.go", Line: 1}, {Symptom: "hang", File: ".claude/a.go", Line: 1}, {Symptom: "hang", MaxCalls: 3}, {Symptom: "hang", Depth: 4}, {Symptom: "hang", MaxCandidates: 21}, {Symptom: "hang", Expand: []contract.Citation{{File: "/tmp/a.go", StartLine: 1, EndLine: 1}}}} {
		g, v := fixture()
		if _, e := Compose(context.Background(), g, v, r); e == nil || g.Calls.Health != 0 {
			t.Fatalf("invalid request touched backend: %+v %v", r, e)
		}
	}
}
