package main

import (
	"context"
	"fmt"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage3"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// seedScore is the stage 2 score every seed gets. Stage 3 multiplies it into
// each neighbour's score, so one value for all seeds keeps the walk alone
// responsible for the order.
const seedScore = 1.0

// graphVariant names one stage 3 configuration the runner measures.
type graphVariant string

const (
	// variantUncapped is the decision A arm: no per-seed or final cap, so
	// the list is every node the three-hop walk reaches, in stage 3 order.
	variantUncapped graphVariant = "graph_uncapped"
	// variantDefault is stage 3 with its shipped caps (50 per seed, 50
	// final): what the composer would hand on. Reported, not judged.
	variantDefault graphVariant = "graph_default_caps"
)

func variantConfig(v graphVariant) stage3.Config {
	cfg := stage3.DefaultConfig()
	if v == variantUncapped {
		cfg.NeighborsPerSeed = 0
		cfg.MaxFinalNeighbors = 0
	}
	return cfg
}

// graphArm runs stage 3 from the seed nodes with intent Unknown and maps
// each returned neighbour back to the graph node with the same file and
// span.
func graphArm(ctx context.Context, ckg ckgclient.Client, fn *fileNodes, seeds []types.Node, v graphVariant) ([]candidate, []loc, error) {
	exp, err := stage3.New(ckg, stage3.WithConfig(variantConfig(v)))
	if err != nil {
		return nil, nil, fmt.Errorf("stage3: %w", err)
	}
	in := make([]stage2.ScoredCitation, 0, len(seeds))
	for _, n := range seeds {
		in = append(in, stage2.ScoredCitation{
			Citation: contract.Citation{File: n.FilePath, StartLine: n.StartLine, EndLine: n.EndLine},
			Score:    seedScore,
		})
	}
	out, err := exp.Expand(ctx, in, contract.IntentUnknown)
	if err != nil {
		return nil, nil, fmt.Errorf("expand: %w", err)
	}
	var failed []loc
	for _, c := range out.FailedSeeds {
		failed = append(failed, loc{File: c.File, StartLine: c.StartLine, EndLine: c.EndLine})
	}

	cands := make([]candidate, 0, len(out.Neighbors))
	for i, sn := range out.Neighbors {
		t := sn.Edge.Target
		c := candidate{
			Rank:     i + 1,
			loc:      loc{File: t.File, StartLine: t.StartLine, EndLine: t.EndLine},
			Score:    sn.Score,
			Distance: sn.Edge.Distance,
			Relation: string(sn.Edge.Relation),
		}
		ns, err := fn.get(t.File)
		if err != nil {
			return nil, nil, err
		}
		if n, ok := nodeAtSpan(ns, c.loc); ok {
			c.QName, c.ID = n.QualifiedName, n.ID
		}
		cands = append(cands, c)
	}
	return cands, failed, nil
}

// nodeAtSpan returns the node with exactly this span, preferring a
// declaration node and then the lowest id.
func nodeAtSpan(ns []types.Node, l loc) (types.Node, bool) {
	var best types.Node
	found := false
	for _, n := range ns {
		if nodeLoc(n) != l {
			continue
		}
		better := !found ||
			(declTypes[n.Type] && !declTypes[best.Type]) ||
			(declTypes[n.Type] == declTypes[best.Type] && n.ID < best.ID)
		if better {
			best, found = n, true
		}
	}
	return best, found
}
