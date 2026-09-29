package semantic

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"

	_ "github.com/mattn/go-sqlite3"
)

type docChunk struct {
	id, file, commit string
	start, end       int
}

func vectorDocChunks(ctx context.Context, vectorDir string) ([]docChunk, error) {
	dbPath, err := filepath.Abs(filepath.Join(vectorDir, "vector.db"))
	if err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: dbPath, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `SELECT id, file, start_line, end_line, commit_hash
		FROM chunks WHERE language = 'markdown' AND chunk_kind = 'doc'`)
	if err != nil {
		return nil, fmt.Errorf("read CKV document chunks: %w", err)
	}
	defer rows.Close()
	var chunks []docChunk
	for rows.Next() {
		var chunk docChunk
		if err := rows.Scan(&chunk.id, &chunk.file, &chunk.start, &chunk.end, &chunk.commit); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chunks, nil
}

// AttachCKVChunks joins extracted Markdown sections to actual CKV doc chunks
// by source commit, relative file and overlapping original source lines.
// Sections without a CKV chunk remain visibly unlinked.
func AttachCKVChunks(ctx context.Context, p *Projection, vectorDir string) (int, error) {
	if p == nil {
		return 0, fmt.Errorf("nil semantic projection")
	}
	if len(p.Sections) == 0 {
		return 0, nil
	}
	chunks, err := vectorDocChunks(ctx, vectorDir)
	if err != nil {
		return 0, err
	}
	evidence := map[string]EvidenceSpan{}
	for _, span := range p.Evidence {
		evidence[span.ID] = span
	}
	linked := 0
	for i := range p.Sections {
		section := &p.Sections[i]
		span := evidence[section.EvidenceID]
		section.ChunkIDs = nil
		for _, chunk := range chunks {
			if chunk.commit == p.Snapshot.Commit && chunk.file == span.Path &&
				chunk.start <= span.EndLine && chunk.end >= span.StartLine {
				section.ChunkIDs = append(section.ChunkIDs, chunk.id)
			}
		}
		sort.Strings(section.ChunkIDs)
		if len(section.ChunkIDs) > 0 {
			linked++
		}
	}
	return linked, nil
}

// ValidateCKVChunkLinks rejects a projection whose declared CKV IDs no longer
// resolve to the same source location in the active vector database.
func ValidateCKVChunkLinks(ctx context.Context, p Projection, vectorDir string) error {
	linked := false
	for _, section := range p.Sections {
		if len(section.ChunkIDs) > 0 {
			linked = true
			break
		}
	}
	if !linked {
		return nil
	}
	chunks, err := vectorDocChunks(ctx, vectorDir)
	if err != nil {
		return err
	}
	byID := map[string]docChunk{}
	for _, chunk := range chunks {
		byID[chunk.id] = chunk
	}
	evidence := map[string]EvidenceSpan{}
	for _, span := range p.Evidence {
		evidence[span.ID] = span
	}
	for _, section := range p.Sections {
		span := evidence[section.EvidenceID]
		for _, id := range section.ChunkIDs {
			chunk, ok := byID[id]
			if !ok || chunk.commit != p.Snapshot.Commit || chunk.file != span.Path ||
				chunk.start > span.EndLine || chunk.end < span.StartLine {
				return fmt.Errorf("section %q has stale or foreign CKV chunk %q", section.ID, id)
			}
		}
	}
	return nil
}
