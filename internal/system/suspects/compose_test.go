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

func wideAnchorFixture() (*graph, *ckvclient.Fake, contract.Citation, contract.Citation) {
	g, v := fixture()
	wide, canonical := cite("a.go"), cite("a.go")
	canonical.StartLine = 3
	g.SymbolCitations = []contract.Citation{wide}
	identity := ckgclient.SymbolIdentity{Symbol: "a.go", CanonicalID: "a.go", Citation: canonical}
	g.SymbolIdentities[wide.String()], g.SymbolIdentities[canonical.String()] = identity, identity
	forward := contract.Neighbor{Source: canonical, Target: cite("b.go"), Relation: contract.RelationCalls, Distance: 1}
	reverse := contract.Neighbor{Source: cite("b.go"), Target: canonical, Relation: contract.RelationCalledBy, Distance: 1}
	for _, src := range []contract.Citation{wide, canonical} {
		g.edges[src.String()+string(contract.RelationCalls)] = []contract.Neighbor{forward}
		g.edges[src.String()+string(contract.RelationCalledBy)] = []contract.Neighbor{reverse}
	}
	g.edges[cite("b.go").String()+string(contract.RelationCalls)] = []contract.Neighbor{{Source: cite("b.go"), Target: canonical, Relation: contract.RelationCalls, Distance: 1}}
	g.ImpactResult = contract.ImpactResult{Groups: []contract.ImpactGroup{{Category: contract.ImpactCallers, Hits: []contract.Citation{canonical, cite("b.go")}}}}
	g.ConcurrencyResult = contract.ConcurrencyResult{Modules: []contract.ConcurrencyModule{{Citation: canonical, Direction: "both"}}}
	return g, v, wide, canonical
}

func TestVerifiedWideAnchorPreservesRetrievalAndReachesCanonicalEdges(t *testing.T) {
	g, v, wide, canonical := wideAnchorFixture()
	v.SearchHits = []contract.Hit{{Citation: wide, Rank: 1, Score: .9, Source: contract.HitSourceCKV}}
	g.BM25Hits = []contract.Hit{{Citation: wide, Rank: 2, Score: 37, Source: contract.HitSourceCKG}}
	out, err := Compose(context.Background(), g, v, Request{Symptom: "wide source", Depth: 2, MaxCalls: 32})
	if err != nil || len(out.Candidates) != 2 {
		t.Fatal("verified canonical edges lost or cycle duplicated seed", err, out)
	}
	a, b := find(t, out, "a.go"), find(t, out, "b.go")
	if a.Citation != wide || a.Signals.Retrieval.Value == nil || len(*a.Signals.Retrieval.Value) != 2 || (*a.Signals.Retrieval.Value)[1].Score != 37 || (*a.Signals.Retrieval.Value)[1].Rank != 2 {
		t.Fatal("original retrieval row/rank/score changed", a)
	}
	for _, from := range a.Signals.Retrieval.Provenance {
		if from.Anchor != wide {
			t.Fatal("original retrieval provenance was normalized away")
		}
	}
	if b.Signals.Distance.Value == nil || *b.Signals.Distance.Value != 1 || a.Signals.Distance.Value == nil || *a.Signals.Distance.Value != 0 {
		t.Fatal("canonical alias changed hop distance", a, b)
	}
	if a.Signals.Impact.Value == nil || len(*a.Signals.Impact.Value) != 1 || a.Signals.Concurrency.Value == nil || !a.Signals.Concurrency.Value.Touched {
		t.Fatal("verified canonical membership became a negative", a.Signals)
	}
	queriedCanonical := false
	for _, call := range g.Calls.Neighbors {
		queriedCanonical = queriedCanonical || call.Src == canonical
		if call.Opts.Hops != 1 {
			t.Fatal("one-hop walk changed")
		}
	}
	if !queriedCanonical || out.Budget.Calls > out.Budget.MaxCalls {
		t.Fatal("canonical walk/call budget contract lost")
	}
}

func TestWideAnchorRejectsUnverifiedIdentityAndInterfaceBridge(t *testing.T) {
	for _, mode := range []string{"foreign_file", "foreign_commit", "disjoint", "overlap_only", "empty_symbol", "ambiguous", "interface_bridge"} {
		t.Run(mode, func(t *testing.T) {
			g, v, wide, canonical := wideAnchorFixture()
			identity := g.SymbolIdentities[wide.String()]
			switch mode {
			case "foreign_file":
				identity.Citation.File = "foreign.go"
			case "foreign_commit":
				identity.Citation.CommitHash = strings.Repeat("2", 40)
			case "disjoint":
				identity.Citation.StartLine, identity.Citation.EndLine = 8, 10
			case "overlap_only":
				identity.Citation.StartLine, identity.Citation.EndLine = 4, 8
			case "empty_symbol":
				identity.Symbol = ""
			case "ambiguous":
				g.SymbolIdentityErr = fmt.Errorf("private ambiguity: %w", ckgclient.ErrSeedUnresolved)
			case "interface_bridge":
				for _, src := range []contract.Citation{wide, canonical} {
					g.edges[src.String()+string(contract.RelationCalls)] = []contract.Neighbor{{Source: cite("interface.go"), Target: cite("b.go"), Relation: contract.RelationCalls, Distance: 1}}
					g.edges[src.String()+string(contract.RelationCalledBy)] = nil
				}
			}
			g.SymbolIdentities[wide.String()] = identity
			out := run(t, g, v, Request{})
			if len(out.Candidates) != 1 || !out.Partial {
				t.Fatal("unverified/foreign/bridge anchor admitted", out)
			}
		})
	}
}

type deadlineWalkGraph struct {
	*graph
	workDeadline, finalDeadline time.Time
	drift                       bool
}

func (g *deadlineWalkGraph) Neighbors(ctx context.Context, c contract.Citation, o ckgclient.NeighborsOpts) ([]contract.Neighbor, error) {
	g.workDeadline, _ = ctx.Deadline()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (g *deadlineWalkGraph) Health(ctx context.Context) (ckgclient.Health, error) {
	h, err := g.graph.Health(ctx)
	if g.Calls.Health > 1 {
		g.finalDeadline, _ = ctx.Deadline()
		if g.drift {
			h.GraphDigest = "changed"
		}
	}
	return h, err
}

func TestWorkDeadlineReservesFinalVerificationInsideTotalBudget(t *testing.T) {
	for _, drift := range []bool{false, true} {
		t.Run(fmt.Sprint(drift), func(t *testing.T) {
			base, v := fixture()
			g := &deadlineWalkGraph{graph: base, drift: drift}
			parentDeadline := time.Now().Add(time.Second)
			parent, cancel := context.WithDeadline(context.Background(), parentDeadline)
			defer cancel()
			out, err := Compose(parent, g, v, Request{Symptom: "bounded walk", Symbol: "a.go", BudgetMS: 1000})
			if drift {
				if err == nil || len(out.Candidates) != 0 || !strings.Contains(err.Error(), "snapshot_verification_failed") {
					t.Fatal("snapshot drift became publishable", out, err)
				}
			} else if err != nil || !out.Partial || len(out.Candidates) != 1 || out.Budget.Calls > out.Budget.MaxCalls {
				t.Fatal("cooperative work expiry prevented reserved verification", out, err)
			}
			if g.Calls.Health != 2 || !g.finalDeadline.After(g.workDeadline) || g.finalDeadline.After(parentDeadline) {
				t.Fatal("verification reservation extended/missed total deadline")
			}
		})
	}
}

type identityCountingGraph struct {
	*graph
	resolved map[string]int
	failOnce bool
}

func (g *identityCountingGraph) ResolveCitation(ctx context.Context, c contract.Citation) (ckgclient.SymbolIdentity, error) {
	g.resolved[c.String()]++
	if g.failOnce && g.resolved[c.String()] == 1 {
		return ckgclient.SymbolIdentity{}, ckgclient.ErrSeedUnresolved
	}
	return g.Fake.ResolveCitation(ctx, c)
}

func TestWalkIdentityCacheKeepsSuccessAndFailureObservations(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			base, v, wide, _ := wideAnchorFixture()
			g := &identityCountingGraph{graph: base, resolved: map[string]int{}, failOnce: fail}
			out, err := Compose(context.Background(), g, v, Request{Symptom: "cached identity", Symbol: "a.go", Depth: 2})
			if err != nil || g.resolved[wide.String()] != 1 {
				t.Fatal("identity observation retried/duplicated", err, g.resolved)
			}
			if fail && (len(out.Candidates) != 1 || out.Candidates[0].Symbol != nil || !out.Partial) {
				t.Fatal("failed identity was replaced by a later success", out)
			}
			if !fail && len(out.Candidates) != 2 {
				t.Fatal("successful canonical identity not reused", out)
			}
		})
	}
}

func TestCanonicalAliasesDoNotMergeOriginalRetrievalRows(t *testing.T) {
	g, v, wide, canonical := wideAnchorFixture()
	v.SearchHits = []contract.Hit{{Citation: wide, Rank: 1, Score: .9, Source: contract.HitSourceCKV}, {Citation: canonical, Rank: 2, Score: .8, Source: contract.HitSourceCKV}}
	out, err := Compose(context.Background(), g, v, Request{Symptom: "distinct original citations", MaxCalls: 32})
	if err != nil || len(out.Candidates) != 3 || out.Candidates[0].Citation != wide || out.Candidates[1].Citation != canonical {
		t.Fatal("original retrieval rows reordered/merged", out, err)
	}
	for i, original := range []contract.Citation{wide, canonical} {
		s := out.Candidates[i].Signals.Retrieval
		if s.Value == nil || len(*s.Value) != 1 || (*s.Value)[0].Rank != i+1 || s.Provenance[0].Anchor != original {
			t.Fatal("per-row original retrieval provenance lost", s)
		}
	}
}

func TestVerificationFailuresDistinguishBudgetCancelAndDrift(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{errBudget, "snapshot_verification_budget_exhausted"},
		{fmt.Errorf("private: %w", context.DeadlineExceeded), "snapshot_verification_budget_exhausted"},
		{context.Canceled, "snapshot_verification_cancelled"},
		{errors.New("private backend"), "snapshot_verification_failed"},
	} {
		if got := verificationError(context.Background(), tc.err).Error(); got != tc.want {
			t.Errorf("safe verification error = %q, want %q", got, tc.want)
		}
	}
}

func TestWorkBudgetDoesNotMasqueradeAsMissingSeed(t *testing.T) {
	base, v := fixture()
	g := &deadlineWalkGraph{graph: base}
	out, err := Compose(context.Background(), g, v, Request{Symptom: "work deadline", Symbol: "a.go", BudgetMS: 1000})
	if err != nil || len(out.Candidates) != 1 || !out.Partial {
		t.Fatal("reserved verification did not preserve partial result", out, err)
	}
	s := out.Candidates[0].Signals
	if s.Impact.Status != "budget_exhausted" || s.Impact.Reason != "budget_exhausted" || s.Concurrency.Status != "budget_exhausted" || s.Concurrency.Reason != "budget_exhausted" || s.Impact.Value != nil || s.Concurrency.Value != nil {
		t.Fatalf("work budget mislabeled as source/provider absence: impact=%s/%s concurrency=%s/%s", s.Impact.Status, s.Impact.Reason, s.Concurrency.Status, s.Concurrency.Reason)
	}
}

type graphWithoutIdentity struct{ ckgclient.Client }

func TestIdentityDependencyDistinguishesProviderFailureAndUnsupported(t *testing.T) {
	for _, mode := range []string{"provider_error", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			g, v := fixture()
			var provider ckgclient.Client = g
			wantStatus, wantReason := "error", "backend_error"
			if mode == "unsupported" {
				provider = graphWithoutIdentity{Client: g}
				wantStatus, wantReason = "unsupported", "symbol_resolver_unsupported"
			} else {
				g.SymbolIdentityErr = errors.New("PRIVATE_PROVIDER_FAILURE")
			}
			out, err := Compose(context.Background(), provider, v, Request{Symptom: "dependency failure", Symbol: "a.go"})
			if err != nil || !out.Partial || len(out.Candidates) != 1 {
				t.Fatal(out, err)
			}
			s := out.Candidates[0].Signals
			if s.Impact.Status != wantStatus || s.Impact.Reason != wantReason || s.Impact.Value != nil || s.Concurrency.Status != wantStatus || s.Concurrency.Reason != wantReason || s.Concurrency.Value != nil {
				t.Fatal("identity dependency mislabeled", s)
			}
			if len(g.Calls.ImpactOfChange) != 0 || len(g.Calls.ConcurrencyImpact) != 0 {
				t.Fatal("unresolved identity contacted enrichment providers")
			}
			raw, _ := json.Marshal(out)
			if strings.Contains(string(raw), "PRIVATE_PROVIDER_FAILURE") {
				t.Fatal("provider message escaped")
			}
		})
	}
}

func TestSeedProviderFailuresExposeSafeClassificationOnly(t *testing.T) {
	for _, mode := range []string{"symbol", "semantic"} {
		t.Run(mode, func(t *testing.T) {
			g, v := fixture()
			request := Request{Symptom: "seed failure"}
			want := "semantic_seed_failed: backend_error"
			if mode == "symbol" {
				request.Symbol = "a.go"
				g.SymbolErr = errors.New("PRIVATE_PROVIDER_MESSAGE")
				want = "seed_unresolved: backend_error"
			} else {
				v.SearchErr = errors.New("PRIVATE_PROVIDER_MESSAGE")
			}
			out, err := Compose(context.Background(), g, v, request)
			if err == nil || err.Error() != want || len(out.Candidates) != 0 {
				t.Fatal("opaque seed provider error not safely classified", err)
			}
		})
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

func TestFailureReasonsDoNotConflateSeedBackendAndBudget(t *testing.T) {
	for _, tc := range []struct {
		name, state, reason string
		err                 error
	}{
		{"unresolved", "error", "seed_unresolved", ckgclient.ErrSeedUnresolved},
		{"wrapped_unresolved", "error", "seed_unresolved", fmt.Errorf("private seed detail: %w", ckgclient.ErrSeedUnresolved)},
		{"backend", "error", "backend_error", errors.New("private backend detail")},
		{"budget", "budget_exhausted", "budget_exhausted", errBudget},
		{"wrapped_budget", "budget_exhausted", "budget_exhausted", fmt.Errorf("private budget detail: %w", errBudget)},
		{"budget_precedes_joined_seed", "budget_exhausted", "budget_exhausted", errors.Join(errBudget, ckgclient.ErrSeedUnresolved)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := failed[[]contract.ImpactCategory](tc.err, "graph.impact_analysis", head, cite("a.go"))
			if s.Status != tc.state || s.Reason != tc.reason || s.Value != nil || len(s.Provenance) != 1 || s.Provenance[0].Anchor != cite("a.go") {
				t.Fatalf("failure classification or provenance lost: %+v", s)
			}
			raw, err := json.Marshal(s)
			if err != nil || strings.Contains(string(raw), "private") {
				t.Fatal("private backend detail leaked", err)
			}
		})
	}
}

func TestUnresolvedSeedSharedSignalsStayPartialBeforeOrAfterSuccess(t *testing.T) {
	for _, missingFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(missingFirst), func(t *testing.T) {
			g, v := fixture()
			missing := "a.go"
			if !missingFirst {
				missing = "b.go"
			}
			delete(g.SymbolIdentities, cite(missing).String())
			out := run(t, g, v, Request{Expand: []contract.Citation{cite("a.go"), cite("b.go")}, MaxEnriched: 2})
			if !out.Partial {
				t.Fatal("unresolved seed became a complete result")
			}
			for _, c := range out.Candidates {
				if c.Signals.Impact.Status != "error" || c.Signals.Concurrency.Status != "error" || c.Signals.Impact.Reason != "seed_unresolved" || c.Signals.Concurrency.Reason != "seed_unresolved" || c.Signals.Impact.Value != nil || c.Signals.Concurrency.Value != nil {
					t.Fatalf("missing seed was erased or mislabeled: %+v", c.Signals)
				}
				for _, provenance := range [][]contract.SignalFrom{c.Signals.Impact.Provenance, c.Signals.Concurrency.Provenance} {
					seen := map[contract.Citation]bool{}
					for _, from := range provenance {
						seen[from.Anchor] = true
					}
					if !seen[cite("a.go")] || !seen[cite("b.go")] {
						t.Fatal("shared failure lost a seed's provenance", provenance)
					}
				}
			}
			if len(g.Calls.ImpactOfChange) != 1 || len(g.Calls.ConcurrencyImpact) != 1 {
				t.Fatal("unresolved seed must not be sent to analysis providers", g.Calls)
			}
		})
	}
}
