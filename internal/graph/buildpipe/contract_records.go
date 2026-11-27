package buildpipe

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/0xmhha/knowledge-system/internal/graph/contract"
	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/internal/logx"
)

// goThroughContract passes the Go pipeline's resolved graph through the
// contract records (increment 1 of design section 9.1): the graph that goes on
// is the one rebuilt from the records, so every Go build proves that the
// records hold what the Go pipeline produces. A field the records cannot hold
// fails the build instead of disappearing.
//
// The layer of a declaration is its file's role in the build scope when
// roles lists the file, and the path rule otherwise (FileRoles), as
// docs/graph/SYMBOL-CONTRACT-DESIGN.md section 9.1 plans for increment 3.
func goThroughContract(log *slog.Logger, rg *parse.ResolvedGraph, roles FileRoles) (*parse.ResolvedGraph, error) {
	if rg == nil {
		return nil, nil
	}
	start := time.Now()
	u, err := gop.ToUnit(rg, roles.layer)
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
