package query

import (
	"context"
	"errors"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

func TestRetainedCitationsUseAuthoritativeReaderAndKeepFlowDrift(t *testing.T) {
	calls := 0
	hits := []types.Hit{{Chunk: types.Chunk{File: "@cks/docs/0/a.md", StartLine: 1, EndLine: 1}}, {Chunk: types.Chunk{File: "a.go", StartLine: 3, EndLine: 3, ChunkKind: types.ChunkFlowStep}}, {Chunk: types.Chunk{File: "@cks/docs/00/a.md", StartLine: 1, EndLine: 1}}}
	reader := func(_ context.Context, path string) ([]byte, error) {
		calls++
		if path == "@cks/docs/0/a.md" {
			return []byte("corpus\n"), nil
		}
		if path == "a.go" {
			return []byte("code\n"), nil
		}
		return nil, errors.New("not retained")
	}
	keep, dropped, stale, err := enforceRetainedCitations(context.Background(), hits, reader)
	if err != nil || len(keep) != 2 || dropped != 1 || stale != 1 || !keep[1].StaleCitation || calls != 2 {
		t.Fatalf("retained enforcement: %v %d %d %v calls=%d", keep, dropped, stale, err, calls)
	}
	sentinel := errors.New("tampered archive")
	if keep, _, _, err := enforceRetainedCitations(context.Background(), hits, func(context.Context, string) ([]byte, error) { return nil, sentinel }); !errors.Is(err, sentinel) || keep != nil {
		t.Fatalf("archive failure allowed partial/live fallback: %v %v", keep, err)
	}
}

func TestUnretainedFilesListsOnlyFilesACitationCheckWouldFailOn(t *testing.T) {
	chunks := []types.Chunk{
		{File: "a.go", ChunkKind: types.ChunkSymbol},
		{File: "a.go", ChunkKind: types.ChunkFileHeader},
		{File: "entries/A1.md", ChunkKind: types.ChunkDoc},
		{File: "@cks/docs/0/entries/A1.md", ChunkKind: types.ChunkDoc},
		{File: "@cks/docs/00/bad.md", ChunkKind: types.ChunkDoc},
		{File: "conventions", ChunkKind: types.ChunkConvention},
	}
	reads := 0
	reader := func(_ context.Context, path string) ([]byte, error) {
		reads++
		if path == "a.go" || path == "@cks/docs/0/entries/A1.md" {
			return []byte("x\n"), nil
		}
		return nil, errors.New("source_missing: citation file is not in the retained inventory")
	}
	missing, err := unretainedFiles(context.Background(), chunks, reader)
	if err != nil || len(missing) != 1 || missing[0] != "entries/A1.md" || reads != 3 {
		t.Fatalf("unretained files: %v %v reads=%d", missing, err, reads)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := unretainedFiles(ctx, chunks, reader); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled check must fail, not report files: %v", err)
	}
}
