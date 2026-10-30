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

// unretainedFiles returns the files enforceRetainedCitations would read but
// read cannot serve. A dataset built before document corpora were retained
// indexes out-of-tree documents under plain paths; any query hitting one of
// them fails, so the caller reports the dataset instead of failing per query.
func unretainedFiles(ctx context.Context, chunks []types.Chunk, read sourcepath.Reader) ([]string, error) {
	var missing []string
	seen := map[string]bool{}
	for _, c := range chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, _, reserved, valid := sourcepath.Parse(c.File)
		if c.File == "" || seen[c.File] || (reserved && !valid) || (!reserved && hasSyntheticCitation(c)) {
			continue
		}
		seen[c.File] = true
		if _, err := read(ctx, c.File); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			missing = append(missing, c.File)
		}
	}
	return missing, nil
}

// UnretainedFiles lists the indexed files the retained source inventory
// behind read cannot serve. Empty means every citable chunk can be verified.
func (e *Engine) UnretainedFiles(ctx context.Context, read sourcepath.Reader) ([]string, error) {
	if read == nil {
		return nil, nil
	}
	chunks, err := e.store.CitedFiles(ctx)
	if err != nil {
		return nil, err
	}
	return unretainedFiles(ctx, chunks, read)
}
