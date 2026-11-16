package buildpipe

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/0xmhha/knowledge-system/internal/graph/contract"
	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/internal/logx"
	"github.com/0xmhha/knowledge-system/pkg/system/testpath"
)

// pathLayer is the layer rule of today's is_test column: a file is test code
// by its path. Increment 3 of docs/graph/SYMBOL-CONTRACT-DESIGN.md section 9.1
// takes the layer from the build scope's file roles instead.
func pathLayer(file string) contract.Layer {
	if testpath.IsTest(file) {
		return contract.LayerTest
	}
	return contract.LayerProduction
}

// goThroughContract passes the Go pipeline's resolved graph through the
// contract records (increment 1 of design section 9.1): the graph that goes on
// is the one rebuilt from the records, so every Go build proves that the
// records hold what the Go pipeline produces. A field the records cannot hold
// fails the build instead of disappearing.
func goThroughContract(log *slog.Logger, rg *parse.ResolvedGraph) (*parse.ResolvedGraph, error) {
	if rg == nil {
		return nil, nil
	}
	start := time.Now()
	u, err := gop.ToUnit(rg, pathLayer)
	if err != nil {
		return nil, fmt.Errorf("go contract records: %w", err)
	}
	tests := 0
	for _, d := range u.Decls {
		if d.Layer == contract.LayerTest {
			tests++
		}
	}
	log.With(logx.AreaKey, "contract").Info("contract records",
		"language", u.Language, "decls", len(u.Decls), "test_decls", tests,
		"refs", len(u.Refs), "seconds", time.Since(start).Seconds())
	return gop.FromUnit(u), nil
}
