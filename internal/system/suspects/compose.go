// Package suspects composes objective CKG/CKV observations for an iterative
// agent walk. It never classifies a candidate as the cause of the symptom.
package suspects

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type Request struct {
	Symptom       string
	Symbol        string
	File          string
	Line          int
	Expand        []contract.Citation
	Commit        string
	Coordinates   *contract.V2Coordinates
	MaxCandidates int
	MaxSeeds      int
	MaxEnriched   int
	MaxCalls      int
	Depth         int
	BudgetMS      int
}

func (r *Request) Defaults() error {
	if strings.TrimSpace(r.Symptom) == "" {
		return fmt.Errorf("symptom is required")
	}
	if len(r.Symptom) > 4096 || len(r.Symbol) > 1024 || len(r.File) > 1024 {
		return fmt.Errorf("input exceeds bounds")
	}
	if r.MaxCandidates == 0 {
		r.MaxCandidates = 10
	}
	if r.MaxSeeds == 0 {
		r.MaxSeeds = 3
	}
	if r.MaxEnriched == 0 {
		r.MaxEnriched = 3
	}
	if r.MaxCalls == 0 {
		r.MaxCalls = 32
	}
	if r.Depth == 0 {
		r.Depth = 2
	}
	if r.BudgetMS == 0 {
		r.BudgetMS = 1000
	}
	if r.MaxCandidates < 1 || r.MaxCandidates > 20 || r.MaxSeeds < 1 || r.MaxSeeds > 8 || r.MaxEnriched < 1 || r.MaxEnriched > 8 || r.MaxCalls < 4 || r.MaxCalls > 64 || r.Depth < 1 || r.Depth > 3 || r.BudgetMS < 10 || r.BudgetMS > 5000 {
		return fmt.Errorf("candidate/seed/enrichment/call/depth/time budget outside bounds")
	}
	if (r.File == "") != (r.Line == 0) || r.Line < 0 {
		return fmt.Errorf("site requires file and positive line")
	}
	modes := 0
	for _, present := range []bool{r.Symbol != "", r.File != "", len(r.Expand) > 0} {
		if present {
			modes++
		}
	}
	if modes > 1 {
		return fmt.Errorf("choose one of symbol, site, or expansion seeds")
	}
	if r.File != "" && !safeFile(r.File) {
		return fmt.Errorf("invalid site file")
	}
	if len(r.Expand) > r.MaxSeeds {
		return fmt.Errorf("expansion seeds exceed max_seeds")
	}
	for _, c := range r.Expand {
		if !c.IsValid() || !safeFile(c.File) {
			return fmt.Errorf("invalid expansion citation")
		}
	}
	return nil
}

type meter struct {
	calls, max int
	reasons    map[string]bool
}

var errBudget = errors.New("budget_exceeded")

func observe[T any](ctx context.Context, b *meter, op func(context.Context) (T, error)) (T, error) {
	var zero T
	if ctx.Err() != nil {
		b.reasons["time_budget"] = true
		return zero, errBudget
	}
	if b.calls >= b.max {
		b.reasons["call_budget"] = true
		return zero, errBudget
	}
	b.calls++
	v, err := op(ctx)
	if ctx.Err() != nil {
		b.reasons["time_budget"] = true
		return zero, errBudget
	}
	return v, err
}

func unknown[T any](state, reason string) contract.Signal[T] {
	return contract.Signal[T]{Status: state, Reason: reason, Provenance: []contract.SignalFrom{}}
}
func fact[T any](v T, tool, snapshot string, c contract.Citation) contract.Signal[T] {
	return contract.Signal[T]{Value: &v, Status: "observed", Provenance: []contract.SignalFrom{{Tool: tool, Snapshot: snapshot, Anchor: c}}}
}
func failed[T any](err error, tool, snapshot string, c contract.Citation) contract.Signal[T] {
	state, reason := "error", "backend_error"
	if errors.Is(err, errBudget) {
		state, reason = "budget_exhausted", "budget_exhausted"
	} else if errors.Is(err, ckgclient.ErrSeedUnresolved) {
		reason = "seed_unresolved"
	}
	return contract.Signal[T]{Status: state, Reason: reason, Provenance: []contract.SignalFrom{{Tool: tool, Snapshot: snapshot, Anchor: c}}}
}

func emptySignals() contract.SuspectSignals {
	return contract.SuspectSignals{
		Distance: unknown[int]("unavailable", "not graph reached"), Relations: unknown[[]contract.Relation]("unavailable", "not graph reached"),
		Impact: unknown[[]contract.ImpactCategory]("not_requested", "enrichment budget"), Recency: unknown[contract.SuspectRecency]("not_requested", "enrichment budget"),
		Concurrency: unknown[contract.SuspectConcurrency]("not_requested", "enrichment budget"), Retrieval: unknown[[]contract.SuspectRetrieval]("unavailable", "not retrieved"),
		Security: unknown[[]string]("unsupported", "policy/security provider not yet connected"), Dataflow: unknown[bool]("unsupported", "value dependency provider not yet connected"),
	}
}

func key(c contract.Citation) string {
	return fmt.Sprintf("%s:%d:%d@%s", c.File, c.StartLine, c.EndLine, c.CommitHash)
}

type symbolResolution struct {
	identity ckgclient.SymbolIdentity
	err      error
}

// Containment guards a resolver result; it never establishes identity by
// itself. Walk edges must still exactly match the declared citation returned
// by the snapshot-bound resolver.
func validResolvedIdentity(c contract.Citation, identity ckgclient.SymbolIdentity, snapshot string) bool {
	d := identity.Citation
	return identity.Symbol != "" && d.IsValid() && safeFile(d.File) && d.File == c.File && d.CommitHash == snapshot &&
		((d.StartLine <= c.StartLine && c.EndLine <= d.EndLine) || (c.StartLine <= d.StartLine && d.EndLine <= c.EndLine))
}

func verificationError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("snapshot_verification_cancelled")
	}
	if ctx.Err() != nil || errors.Is(err, errBudget) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("snapshot_verification_budget_exhausted")
	}
	return fmt.Errorf("snapshot_verification_failed")
}

func seedError(ctx context.Context, prefix string, err error) error {
	reason := "backend_error"
	switch {
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(err, context.Canceled):
		reason = "request_cancelled"
	case ctx.Err() != nil, errors.Is(err, errBudget), errors.Is(err, context.DeadlineExceeded):
		reason = "budget_exhausted"
	case errors.Is(err, ckgclient.ErrSeedUnresolved):
		reason = "seed_unresolved"
	}
	return fmt.Errorf("%s: %s", prefix, reason)
}

// Compose uses bounded backend calls. Deadlines are cooperative: old local
// stores that ignore context may finish one in-flight query after the budget;
// that result is discarded and no subsequent query is started.
func Compose(parent context.Context, g ckgclient.Client, v ckvclient.Client, r Request) (contract.SuspectCandidates, error) {
	if err := r.Defaults(); err != nil {
		return contract.SuspectCandidates{}, err
	}
	if g == nil || v == nil {
		return contract.SuspectCandidates{}, fmt.Errorf("missing backend")
	}
	start := time.Now()
	finalCtx, cancel := context.WithTimeout(parent, time.Duration(r.BudgetMS)*time.Millisecond)
	defer cancel()
	totalDeadline, _ := finalCtx.Deadline()
	reserve := time.Until(totalDeadline) / 10
	if reserve > 100*time.Millisecond {
		reserve = 100 * time.Millisecond
	}
	if reserve < 0 {
		reserve = 0
	}
	ctx, stopWork := context.WithDeadline(finalCtx, totalDeadline.Add(-reserve))
	defer stopWork()
	b := &meter{max: r.MaxCalls - 1, reasons: map[string]bool{}}
	gh, err := observe(ctx, b, g.Health)
	if err != nil || !gh.Reachable {
		return contract.SuspectCandidates{}, fmt.Errorf("graph_unavailable")
	}
	textSeed := r.Symbol == "" && r.File == "" && len(r.Expand) == 0
	var vh ckvclient.Health
	if textSeed {
		b.max-- // reserve both final identity checks
		vh, err = observe(ctx, b, v.Health)
		if err != nil || !vh.Reachable || !vh.ModelReachable {
			return contract.SuspectCandidates{}, fmt.Errorf("semantic_unavailable")
		}
		if gh.IndexedHead != "" && vh.IndexedHead != "" && gh.IndexedHead != vh.IndexedHead {
			return contract.SuspectCandidates{}, fmt.Errorf("snapshot_mismatch")
		}
		if gh.DatasetID != "" || vh.DatasetID != "" {
			if graphCoordinates(gh) != vectorCoordinates(vh) {
				return contract.SuspectCandidates{}, fmt.Errorf("snapshot_mismatch")
			}
		}
	}
	if r.Coordinates != nil {
		want := *r.Coordinates
		if graphCoordinates(gh) != want || want.ProjectID == "" || want.DatasetID == "" || want.SnapshotID == "" || want.SourceMode == "" {
			return contract.SuspectCandidates{}, fmt.Errorf("snapshot_mismatch")
		}
		if textSeed && vectorCoordinates(vh) != want {
			return contract.SuspectCandidates{}, fmt.Errorf("snapshot_mismatch")
		}
	}
	snapshot := gh.IndexedHead
	if snapshot == "" && r.Coordinates == nil {
		return contract.SuspectCandidates{}, fmt.Errorf("snapshot_unavailable")
	}
	if r.Commit != "" {
		if snapshot == "" || snapshot != r.Commit {
			return contract.SuspectCandidates{}, fmt.Errorf("snapshot_mismatch")
		}
	}
	out := contract.SuspectCandidates{SchemaVersion: 1, Symptom: r.Symptom, Snapshot: snapshot, Coordinates: r.Coordinates, Candidates: []contract.SuspectCandidate{}, Notes: []string{
		"Signals are observations, not cause probabilities. The agent supplies symptom-specific judgments.",
		"Search scores remain per backend; they are not comparable cause scores.",
		"Distance is the shortest observed path within bounded traversal; an incomplete walk may miss a shorter path.",
		"Empty impact or concurrency membership is scoped to returned bounded seed queries; it does not prove absence in the entire graph.",
		"History counts are returned PR references, not total commit counts. Empty results are scoped observations.",
		"Deadline is cooperative; a non-cancellable local store may finish one in-flight query after the budget.",
		"Verified declared-source aliases may anchor graph walks and membership; original retrieval citations, ranks and scores remain unchanged.",
		"Work ends early to reserve final backend identity checks inside the same total deadline (10 percent, at most 100ms).",
	}}
	if snapshot == "" && r.Coordinates == nil {
		out.Partial = true
		b.reasons["unknown_snapshot"] = true
	}
	index := map[string]int{}
	resolver, canResolve := g.(ckgclient.CitationResolver)
	identities := map[string]symbolResolution{}
	resolve := func(c contract.Citation) (ckgclient.SymbolIdentity, error) {
		if old, ok := identities[key(c)]; ok {
			return old.identity, old.err
		}
		identity, e := observe(ctx, b, func(ctx context.Context) (ckgclient.SymbolIdentity, error) {
			return resolver.ResolveCitation(ctx, c)
		})
		if e == nil && !validResolvedIdentity(c, identity, snapshot) {
			e = ckgclient.ErrSeedUnresolved
		}
		identities[key(c)] = symbolResolution{identity: identity, err: e}
		if e == nil {
			// A successful declaration also proves its own exact source key.
			// Never erase an earlier failed observation for another input.
			if _, exists := identities[key(identity.Citation)]; !exists {
				identities[key(identity.Citation)] = symbolResolution{identity: identity}
			}
		}
		return identity, e
	}
	bindAlias := func(original, canonical contract.Citation) {
		if i, ok := index[key(original)]; ok {
			if _, exists := index[key(canonical)]; !exists {
				index[key(canonical)] = i
			}
		}
	}
	knownKey := func(c contract.Citation) string {
		if resolved, ok := identities[key(c)]; ok && resolved.err == nil {
			return key(resolved.identity.Citation)
		}
		return key(c)
	}
	sameKnownCitation := func(a, b contract.Citation) bool {
		return key(a) == key(b) || knownKey(a) == knownKey(b)
	}
	seeds := []contract.Citation{}
	admit := func(c contract.Citation) int {
		if !c.IsValid() || !safeFile(c.File) {
			out.Partial = true
			b.reasons["invalid_citation"] = true
			return -1
		}
		if c.CommitHash != snapshot {
			out.Partial = true
			b.reasons["mixed_or_missing_snapshot"] = true
			return -1
		}
		k := key(c)
		if i, ok := index[k]; ok {
			return i
		}
		if len(out.Candidates) >= r.MaxCandidates {
			out.Partial = true
			b.reasons["candidate_budget"] = true
			return -1
		}
		i := len(out.Candidates)
		index[k] = i
		out.Candidates = append(out.Candidates, contract.SuspectCandidate{Citation: c, Signals: emptySignals()})
		return i
	}
	addSeed := func(c contract.Citation) {
		i := admit(c)
		if i < 0 {
			return
		}
		for _, old := range seeds {
			if key(old) == key(c) {
				return
			}
		}
		if len(seeds) >= r.MaxSeeds {
			out.Partial = true
			b.reasons["seed_budget"] = true
			return
		}
		seeds = append(seeds, c)
		out.Candidates[i].Signals.Distance = fact(0, "seed", snapshot, c)
		out.Candidates[i].Signals.Relations = fact([]contract.Relation{}, "seed", snapshot, c)
	}
	if len(r.Expand) > 0 {
		for _, c := range r.Expand {
			addSeed(c)
		}
	} else if r.File != "" {
		if !canResolve {
			return out, fmt.Errorf("site_resolver_unavailable")
		}
		site := contract.Citation{File: r.File, StartLine: r.Line, EndLine: r.Line, CommitHash: snapshot}
		identity, e := resolve(site)
		if e != nil || identity.Symbol == "" {
			return out, fmt.Errorf("seed_unresolved")
		}
		addSeed(identity.Citation)
	} else if r.Symbol != "" {
		citations, e := observe(ctx, b, func(ctx context.Context) ([]contract.Citation, error) {
			return g.FindSymbol(ctx, r.Symbol, ckgclient.SymbolOpts{CommitHash: snapshot})
		})
		if e != nil {
			return out, seedError(ctx, "seed_unresolved", e)
		}
		for _, c := range citations {
			addSeed(c)
		}
		if len(citations) > 1 {
			out.Partial = true
			b.reasons["ambiguous_symbol_candidates"] = true
		}
	} else {
		hits, e := observe(ctx, b, func(ctx context.Context) ([]contract.Hit, error) {
			return v.SemanticSearch(ctx, r.Symptom, ckvclient.SearchOpts{K: r.MaxSeeds, Filter: ckvclient.SearchFilter{CommitHash: snapshot, ExcludeTests: true}})
		})
		if e != nil {
			return out, seedError(ctx, "semantic_seed_failed", e)
		}
		keyword, e2 := observe(ctx, b, func(ctx context.Context) ([]contract.Hit, error) {
			return g.BM25Search(ctx, keywordQuery(r.Symptom), ckgclient.SearchOpts{K: r.MaxSeeds, Filter: ckgclient.SearchFilter{CommitHash: snapshot, ExcludeTests: true}})
		})
		if e2 != nil {
			out.Partial = true
			b.reasons["keyword_seed_failed"] = true
		}
		for backend, group := range [][]contract.Hit{hits, keyword} {
			if len(group) > r.MaxSeeds {
				out.Partial = true
				b.reasons["retrieval_budget"] = true
				group = group[:r.MaxSeeds]
			}
			for _, h := range group {
				if !h.IsValid() || math.IsNaN(h.Score) || math.IsInf(h.Score, 0) || (backend == 0 && h.Source != contract.HitSourceCKV) || (backend == 1 && h.Source != contract.HitSourceCKG) {
					out.Partial = true
					b.reasons["invalid_retrieval"] = true
					continue
				}
				i := admit(h.Citation)
				if i < 0 {
					continue
				}
				old := out.Candidates[i].Signals.Retrieval.Value
				observations := []contract.SuspectRetrieval{}
				if old != nil {
					observations = append(observations, (*old)...)
				}
				observations = append(observations, contract.SuspectRetrieval{Rank: h.Rank, Score: h.Score, Source: h.Source})
				from := out.Candidates[i].Signals.Retrieval.Provenance
				out.Candidates[i].Signals.Retrieval = fact(observations, string(h.Source)+".search", snapshot, h.Citation)
				out.Candidates[i].Signals.Retrieval.Provenance = appendFrom(from, out.Candidates[i].Signals.Retrieval.Provenance...)
				addSeed(h.Citation)
			}
		}
	}
	if len(seeds) == 0 {
		return out, fmt.Errorf("seed_unresolved")
	}
	// Use one-hop walks because legacy Neighbors flattens deep walks and
	// reports every edge as distance one. BFS here supplies actual hop counts.
	for _, seed := range seeds {
		for _, rel := range []contract.Relation{contract.RelationCalledBy, contract.RelationCalls} {
			type reached struct {
				citation contract.Citation
				distance int
			}
			queue := []reached{{seed, 0}}
			seen := map[string]bool{key(seed): true}
			for len(queue) > 0 {
				at := queue[0]
				queue = queue[1:]
				if at.distance >= r.Depth {
					continue
				}
				anchorSource := at.citation
				if known, ok := identities[key(at.citation)]; ok && known.err == nil {
					anchorSource = known.identity.Citation
					bindAlias(at.citation, anchorSource)
					seen[key(anchorSource)] = true
				}
				edges, e := observe(ctx, b, func(ctx context.Context) ([]contract.Neighbor, error) {
					return g.Neighbors(ctx, anchorSource, ckgclient.NeighborsOpts{Hops: 1, Relations: []contract.Relation{rel}, MaxTotal: r.MaxCandidates, ExcludeTests: true})
				})
				if e != nil {
					out.Partial = true
					b.reasons["traversal_incomplete"] = true
					break
				}
				if len(edges) >= r.MaxCandidates {
					out.Partial = true
					b.reasons["edge_budget"] = true
				}
				if len(edges) > r.MaxCandidates {
					edges = edges[:r.MaxCandidates]
				}
				for _, edge := range edges {
					anchor, c := edge.Source, edge.Target
					if rel == contract.RelationCalledBy {
						anchor, c = edge.Target, edge.Source
					}
					if edge.Relation == rel && edge.Distance == 1 && key(anchor) != key(anchorSource) && canResolve {
						identity, e := resolve(at.citation)
						if e == nil {
							anchorSource = identity.Citation
							bindAlias(at.citation, anchorSource)
							seen[key(anchorSource)] = true
						}
					}
					if edge.Relation != rel || edge.Distance != 1 || key(anchor) != key(anchorSource) {
						out.Partial = true
						b.reasons["unanchored_edge"] = true
						continue
					}
					i := admit(c)
					if i < 0 {
						continue
					}
					sig := &out.Candidates[i].Signals
					distance := at.distance + 1
					if sig.Distance.Value == nil || distance < *sig.Distance.Value {
						sig.Distance = fact(distance, "graph.neighbors", snapshot, seed)
					} else if distance == *sig.Distance.Value {
						sig.Distance.Provenance = appendFrom(sig.Distance.Provenance, contract.SignalFrom{Tool: "graph.neighbors", Snapshot: snapshot, Anchor: seed})
					}
					relations := []contract.Relation{}
					if sig.Relations.Value != nil {
						relations = append(relations, (*sig.Relations.Value)...)
					}
					relations = appendUnique(relations, rel)
					from := sig.Relations.Provenance
					sig.Relations = fact(relations, "graph.neighbors", snapshot, seed)
					sig.Relations.Provenance = appendFrom(from, sig.Relations.Provenance...)
					if !seen[key(c)] {
						seen[key(c)] = true
						queue = append(queue, reached{c, distance})
					}
				}
			}
		}
	}
	for i := range out.Candidates {
		c := &out.Candidates[i]
		if c.Symbol == nil && canResolve {
			identity, e := resolve(c.Citation)
			if e == nil && identity.Symbol != "" && (snapshot == "" || identity.Citation.CommitHash == snapshot) {
				name := identity.Symbol
				c.Symbol = &name
			} else {
				out.Partial = true
				b.reasons["symbol_identity_unavailable"] = true
			}
		}
		if c.Symbol == nil && !canResolve {
			out.Partial = true
			b.reasons["symbol_resolver_unsupported"] = true
		}
		if i >= r.MaxEnriched || c.Symbol == nil {
			continue
		}
		name := *c.Symbol
		prs, e := observe(ctx, b, func(ctx context.Context) ([]contract.PRRef, error) {
			return g.GetNodePRs(ctx, name, ckgclient.PRRefOpts{MaxCount: 10})
		})
		if len(prs) > 10 {
			e = fmt.Errorf("history exceeds requested cap")
		}
		if e != nil {
			c.Signals.Recency = failed[contract.SuspectRecency](e, "graph.change_history", snapshot, c.Citation)
			out.Partial = true
		} else {
			value := contract.SuspectRecency{PRCount: len(prs), Scope: "returned PR references (max 10), not commit count"}
			for _, pr := range prs {
				if !pr.MergedAt.IsZero() && (value.LastMergedAt == nil || pr.MergedAt.After(*value.LastMergedAt)) {
					at := pr.MergedAt
					value.LastMergedAt = &at
				}
			}
			c.Signals.Recency = fact(value, "graph.change_history", snapshot, c.Citation)
		}
	}
	// Membership and direction are seed-relative, not a summary of what each
	// candidate itself affects. Keep those two different questions distinct.
	for _, seed := range seeds {
		si, ok := index[key(seed)]
		if !ok || out.Candidates[si].Symbol == nil {
			cause := error(ckgclient.ErrSeedUnresolved)
			if observation, exists := identities[key(seed)]; exists && observation.err != nil {
				cause = observation.err
			}
			for i := range out.Candidates {
				if i >= r.MaxEnriched {
					break
				}
				c := &out.Candidates[i]
				impactFrom, concurrencyFrom := c.Signals.Impact.Provenance, c.Signals.Concurrency.Provenance
				c.Signals.Impact = failed[[]contract.ImpactCategory](cause, "graph.resolve_citation", snapshot, seed)
				c.Signals.Impact.Provenance = appendFrom(impactFrom, c.Signals.Impact.Provenance...)
				c.Signals.Concurrency = failed[contract.SuspectConcurrency](cause, "graph.resolve_citation", snapshot, seed)
				c.Signals.Concurrency.Provenance = appendFrom(concurrencyFrom, c.Signals.Concurrency.Provenance...)
				if !canResolve {
					c.Signals.Impact = unknown[[]contract.ImpactCategory]("unsupported", "symbol_resolver_unsupported")
					c.Signals.Concurrency = unknown[contract.SuspectConcurrency]("unsupported", "symbol_resolver_unsupported")
				}
			}
			out.Partial = true
			continue
		}
		name := *out.Candidates[si].Symbol
		impact, e := observe(ctx, b, func(ctx context.Context) (contract.ImpactResult, error) {
			return g.ImpactOfChange(ctx, name, ckgclient.ImpactOpts{Depth: r.Depth, MaxTotal: r.MaxCandidates})
		})
		if e == nil {
			for _, group := range impact.Groups {
				for _, hit := range group.Hits {
					if !hit.IsValid() || (snapshot != "" && hit.CommitHash != snapshot) {
						e = fmt.Errorf("invalid impact snapshot")
					}
				}
			}
		}
		for i := range out.Candidates {
			if i >= r.MaxEnriched {
				break
			}
			c := &out.Candidates[i]
			if e != nil {
				from := c.Signals.Impact.Provenance
				c.Signals.Impact = failed[[]contract.ImpactCategory](e, "graph.impact_analysis", snapshot, seed)
				c.Signals.Impact.Provenance = appendFrom(from, c.Signals.Impact.Provenance...)
				out.Partial = true
				continue
			}
			if c.Signals.Impact.Status == "error" || c.Signals.Impact.Status == "budget_exhausted" {
				c.Signals.Impact.Provenance = appendFrom(c.Signals.Impact.Provenance, contract.SignalFrom{Tool: "graph.impact_analysis", Snapshot: snapshot, Anchor: seed})
				continue
			}
			groups := []contract.ImpactCategory{}
			if c.Signals.Impact.Value != nil {
				groups = append(groups, (*c.Signals.Impact.Value)...)
			}
			for _, group := range impact.Groups {
				for _, hit := range group.Hits {
					if sameKnownCitation(hit, c.Citation) {
						seen := false
						for _, old := range groups {
							if old == group.Category {
								seen = true
							}
						}
						if !seen {
							groups = append(groups, group.Category)
						}
						break
					}
				}
			}
			old := c.Signals.Impact.Provenance
			c.Signals.Impact = fact(groups, "graph.impact_analysis", snapshot, seed)
			c.Signals.Impact.Provenance = appendFrom(old, c.Signals.Impact.Provenance...)
		}
		concurrency, e := observe(ctx, b, func(ctx context.Context) (contract.ConcurrencyResult, error) {
			return g.ConcurrencyImpact(ctx, name, ckgclient.ConcurrencyOpts{Depth: r.Depth, MaxTotal: r.MaxCandidates})
		})
		if e == nil {
			for _, module := range concurrency.Modules {
				if !module.Citation.IsValid() || (snapshot != "" && module.Citation.CommitHash != snapshot) || (module.Direction != "affects" && module.Direction != "affected_by" && module.Direction != "both") {
					e = fmt.Errorf("invalid concurrency observation")
				}
			}
		}
		for i := range out.Candidates {
			if i >= r.MaxEnriched {
				break
			}
			c := &out.Candidates[i]
			if e != nil || concurrency.NotFound {
				if e == nil {
					e = ckgclient.ErrSeedUnresolved
				}
				from := c.Signals.Concurrency.Provenance
				c.Signals.Concurrency = failed[contract.SuspectConcurrency](e, "graph.concurrency_impact", snapshot, seed)
				c.Signals.Concurrency.Provenance = appendFrom(from, c.Signals.Concurrency.Provenance...)
				out.Partial = true
				continue
			}
			if c.Signals.Concurrency.Status == "error" || c.Signals.Concurrency.Status == "budget_exhausted" {
				c.Signals.Concurrency.Provenance = appendFrom(c.Signals.Concurrency.Provenance, contract.SignalFrom{Tool: "graph.concurrency_impact", Snapshot: snapshot, Anchor: seed})
				continue
			}
			value := contract.SuspectConcurrency{Direction: []string{}}
			if c.Signals.Concurrency.Value != nil {
				value = *c.Signals.Concurrency.Value
			}
			for _, module := range concurrency.Modules {
				if sameKnownCitation(module.Citation, c.Citation) {
					value.Touched = true
					seen := false
					for _, old := range value.Direction {
						if old == module.Direction {
							seen = true
						}
					}
					if !seen {
						value.Direction = append(value.Direction, module.Direction)
					}
				}
			}
			old := c.Signals.Concurrency.Provenance
			c.Signals.Concurrency = fact(value, "graph.concurrency_impact", snapshot, seed)
			c.Signals.Concurrency.Provenance = appendFrom(old, c.Signals.Concurrency.Provenance...)
		}
	}
	// Reserve identity verification calls even when enrichment exhausts its
	// cap. Never publish observations across a backend revision change.
	b.max = r.MaxCalls
	finalGraph, e := observe(finalCtx, b, g.Health)
	if e != nil {
		return contract.SuspectCandidates{}, verificationError(finalCtx, e)
	}
	if !finalGraph.Reachable || graphCoordinates(finalGraph) != graphCoordinates(gh) || finalGraph.GraphDigest != gh.GraphDigest {
		return contract.SuspectCandidates{}, fmt.Errorf("snapshot_verification_failed")
	}
	if textSeed {
		finalVector, e := observe(finalCtx, b, v.Health)
		if e != nil {
			return contract.SuspectCandidates{}, verificationError(finalCtx, e)
		}
		if !finalVector.Reachable || !finalVector.ModelReachable || vectorCoordinates(finalVector) != vectorCoordinates(vh) || finalVector.StatsHash != vh.StatsHash {
			return contract.SuspectCandidates{}, fmt.Errorf("snapshot_verification_failed")
		}
	}
	for reason := range b.reasons {
		out.Budget.StopReasons = append(out.Budget.StopReasons, reason)
	}
	sort.Strings(out.Budget.StopReasons)
	out.Partial = out.Partial || len(b.reasons) > 0
	out.Budget.MaxCandidates = r.MaxCandidates
	out.Budget.MaxSeeds = r.MaxSeeds
	out.Budget.MaxEnriched = r.MaxEnriched
	out.Budget.MaxCalls = r.MaxCalls
	out.Budget.Calls = b.calls
	out.Budget.Depth = r.Depth
	out.Budget.ElapsedMS = time.Since(start).Milliseconds()
	out.Budget.TimeBudgetMS = r.BudgetMS
	if out.Budget.StopReasons == nil {
		out.Budget.StopReasons = []string{}
	}
	return out, nil
}

func appendUnique[T comparable](values []T, value T) []T {
	for _, old := range values {
		if old == value {
			return values
		}
	}
	return append(values, value)
}
func appendFrom(values []contract.SignalFrom, add ...contract.SignalFrom) []contract.SignalFrom {
	for _, value := range add {
		values = appendUnique(values, value)
	}
	return values
}
func graphCoordinates(h ckgclient.Health) contract.V2Coordinates {
	return contract.V2Coordinates{ProjectID: h.ProjectID, DatasetID: h.DatasetID, SnapshotID: h.SnapshotID, SourceMode: h.SourceMode, BaseCommit: h.IndexedHead}
}
func vectorCoordinates(h ckvclient.Health) contract.V2Coordinates {
	return contract.V2Coordinates{ProjectID: h.ProjectID, DatasetID: h.DatasetID, SnapshotID: h.SnapshotID, SourceMode: h.SourceMode, BaseCommit: h.IndexedHead}
}
func keywordQuery(query string) string {
	tokens := strings.FieldsFunc(query, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' })
	quoted := []string{}
	seen := map[string]bool{}
	for _, t := range tokens {
		if !seen[t] {
			seen[t] = true
			quoted = append(quoted, `"`+t+`"`)
		}
	}
	return strings.Join(quoted, " OR ")
}

func safeFile(file string) bool {
	if file == "" || path.IsAbs(file) || path.Clean(file) != file || strings.ContainsAny(file, "\\\x00") || file == ".." || strings.HasPrefix(file, "../") {
		return false
	}
	root := strings.Split(file, "/")[0]
	return root != ".claude" && root != "logs" && !strings.HasPrefix(file, "@cks/")
}
