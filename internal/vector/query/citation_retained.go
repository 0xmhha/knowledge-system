package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/sourcepath"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

func enforceRetainedCitations(ctx context.Context, hits []types.Hit, read sourcepath.Reader) (keep []types.Hit, dropped, stale int, err error) {
	keep = make([]types.Hit, 0, len(hits))
	files := map[string][]byte{}
	for _, hit := range hits {
		if err := ctx.Err(); err != nil {
			return nil, 0, 0, err
		}
		c := hit.Chunk
		_, _, reserved, valid := sourcepath.Parse(c.File)
		if reserved && !valid {
			dropped++
			continue
		}
		if !reserved && hasSyntheticCitation(c) {
			keep = append(keep, hit)
			continue
		}
		if c.File == "" || c.StartLine < 1 || c.EndLine < c.StartLine {
			dropped++
			continue
		}
		data, found := files[c.File]
		if !found {
			data, err = read(ctx, c.File)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("retained citation %q: %w", c.File, err)
			}
			files[c.File] = data
		}
		if c.ChunkKind == types.ChunkFlowStep {
			lines := strings.Count(string(data), "\n")
			if len(data) > 0 && data[len(data)-1] != '\n' {
				lines++
			}
			if c.StartLine > lines {
				hit.StaleCitation = true
				stale++
			}
		}
		keep = append(keep, hit)
	}
	return keep, dropped, stale, nil
}
