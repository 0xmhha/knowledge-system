// Package chunk turns ([]parse.SymbolSpan, source) into ([]types.Chunk)
// — the records the embedder + vector store actually persist.
//
// Strategy:
//  1. Each symbol span (function/method/type/...) becomes one chunk.
//  2. A model-specific MaxTextBytes budget splits every oversized source
//     span without loss. With only the legacy token estimate, long functions
//     and Markdown sections split and other kinds are head-truncated.
//  3. A file_header chunk captures the first N lines of the file —
//     package decl + imports + top-level const/var. This lets queries
//     like "what package owns the metrics client" hit the right file
//     even when nothing function-level matches.
package chunk

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/0xmhha/knowledge-system/internal/vector/parse"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

// DefaultFileHeaderLines is the number of leading lines of each file
// captured as a "file_header" chunk when Options.FileHeaderLines is
// unset. 50 is enough to cover the package decl + a typical import
// block + top-level consts. Per-project ckv.yaml can override this
// via chunking.file_header_lines.
const DefaultFileHeaderLines = 50

// FileHeaderLines is the legacy export retained for callers that
// reference the constant directly. New code should set
// Options.FileHeaderLines and read Chunker.opts.
const FileHeaderLines = DefaultFileHeaderLines

// charsPerToken is a legacy estimate for MaxInputTokens. Tokenizers and
// contextual prefixes can exceed it; model-specific MaxTextBytes is used
// when strict input completeness is required.
const charsPerToken = 4

// Options configure chunking. Zero value uses documented defaults.
type Options struct {
	MaxInputTokens    int  // token-window estimate for raw text; 0 → no cap
	MaxTextBytes      int  // conservative raw-byte cap; 0 → use token estimate only
	IncludeFileHeader bool // emit a file_header chunk per file (default: true)
	// FileHeaderLines overrides DefaultFileHeaderLines (50). 0 keeps
	// the default. Sourced from project ckv.yaml.chunking.file_header_lines.
	FileHeaderLines int
	// IncludeFileFull emits an additional coarse chunk covering the whole file
	// (Phase B multi-granularity, roadmap §3.1). Off by default — opt-in for
	// measurement. Unlike file_header (first N lines) this spans the entire
	// file, giving file/module-level queries a coarse target alongside the fine
	// symbol chunks. Skipped for markdown (heading sections are already coarse).
	IncludeFileFull bool
}

// Input is everything the chunker needs about one file.
type Input struct {
	File       string // repo-relative path
	Language   string // "go" | "typescript" | "solidity" | "markdown"
	CommitHash string // built-time git HEAD
	Source     []byte // full file contents
	Spans      []parse.SymbolSpan
}

// Chunker turns parsed spans into Chunks.
type Chunker struct {
	opts Options
}

// New returns a Chunker with the given options. opts.MaxInputTokens of
// 0 disables truncation (appropriate for the mock embedder).
func New(opts Options) *Chunker {
	if !opts.IncludeFileHeader {
		// Make the default opt-in: explicit zero-value Options{} should
		// produce the typical "include the header" behavior. We flip
		// the bit here so the caller writes Options{} for the default
		// and Options{IncludeFileHeader: false} only when disabling.
		opts.IncludeFileHeader = true
	}
	return &Chunker{opts: opts}
}

// Chunk produces all chunks for one file. The returned slice carries
// IDs and content_sha256 already computed — callers do not need to
// hash again before Upsert.
func (c *Chunker) Chunk(in Input) []types.Chunk {
	out := make([]types.Chunk, 0, len(in.Spans)+1)

	// Skip the file_header chunk for markdown inputs: every heading
	// section is already its own chunk, so the leading-N-lines slice
	// would duplicate the first section. Source-code languages keep
	// the header for package-level orientation (package doc + imports;
	// Go const/var blocks have their own spans since 2026-07-28) —
	// markdown's coverage is dense.
	if c.opts.IncludeFileHeader && in.Language != "markdown" {
		if hdr := c.fileHeaderChunk(in); hdr != nil {
			out = append(out, *hdr)
		}
	}

	// Phase B (opt-in): a coarse whole-file chunk alongside the fine symbol
	// chunks, for file/module-level queries. Skip markdown (dense heading
	// sections already provide coarse granularity).
	if c.opts.IncludeFileFull && in.Language != "markdown" {
		if full := c.fileFullChunk(in); full != nil {
			out = append(out, *full)
		}
	}

	for _, sp := range in.Spans {
		// A model-specific byte budget must preserve every source byte, even
		// for long type declarations or a function written on one line.
		if c.opts.MaxTextBytes > 0 && len(sp.Text) > c.maxBytes() {
			parts := c.splitLongDocSpan(in, sp)
			if sp.Kind == types.KindFunction || sp.Kind == types.KindMethod {
				for i := range parts {
					parts[i].ChunkKind = types.ChunkFunctionSplit
					parts[i].SymbolName = fmt.Sprintf("%s:chunk:%d", sp.Name, i+1)
				}
			}
			out = append(out, parts...)
			continue
		}
		// Long Markdown sections are split at source-line boundaries so
		// their tail remains searchable with accurate citations.
		if c.shouldSplitDoc(sp) {
			out = append(out, c.splitLongDocSpan(in, sp)...)
			continue
		}
		// Long functions use their existing source-window strategy.
		if c.shouldSplit(sp) {
			out = append(out, c.splitLongSpan(in, sp)...)
			continue
		}
		text := c.maybeTruncate(sp.Text)
		out = append(out, c.symbolChunk(in, sp, text))
	}
	return out
}

func (c *Chunker) shouldSplitDoc(sp parse.SymbolSpan) bool {
	if c.opts.MaxInputTokens <= 0 {
		return false
	}
	if sp.Kind != types.KindDocSection && sp.Kind != types.KindADRSection {
		return false
	}
	return len(sp.Text) > c.maxBytes()
}

func (c *Chunker) maxBytes() int {
	max := c.opts.MaxInputTokens * charsPerToken
	if c.opts.MaxTextBytes > 0 && (max <= 0 || c.opts.MaxTextBytes < max) {
		return c.opts.MaxTextBytes
	}
	return max
}

// splitLongDocSpan keeps every source line and prefers paragraph boundaries.
// A line longer than the embedding cap is split at UTF-8 boundaries; its
// fragments retain the original line citation. The caller may also use this
// lossless splitter for code when a model-specific byte budget is active.
func (c *Chunker) splitLongDocSpan(in Input, sp parse.SymbolSpan) []types.Chunk {
	maxChars := c.maxBytes()
	parentID := types.ChunkID(in.File, sp.StartLine, sp.EndLine, types.ContentSHA256(sp.Text))
	lines := strings.SplitAfter(sp.Text, "\n")
	var out []types.Chunk
	var pending []string
	startLine := sp.StartLine
	lineNo := sp.StartLine
	emit := func(parts []string, start int) {
		if len(parts) == 0 {
			return
		}
		text := strings.Join(parts, "")
		end := start + len(parts) - 1
		child := sp
		child.StartLine, child.EndLine = start, end
		part := c.symbolChunk(in, child, text)
		part.ParentID, part.ParentStartLine, part.ParentEndLine = parentID, sp.StartLine, sp.EndLine
		part.PartOrdinal = len(out) + 1
		out = append(out, part)
	}
	length := func(parts []string) int {
		n := 0
		for _, p := range parts {
			n += len(p)
		}
		return n
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		if len(line) > maxChars {
			emit(pending, startLine)
			pending = nil
			offset := 0
			for len(line) > 0 {
				end := min(maxChars, len(line))
				for end > 0 && end < len(line) && line[end]&0xc0 == 0x80 {
					end--
				}
				if end == 0 { // a multibyte rune wider than a tiny cap
					end = min(len(line), maxChars)
				}
				child := sp
				child.StartLine, child.EndLine = lineNo, lineNo
				part := c.symbolChunk(in, child, line[:end])
				part.ID = types.ChunkFragmentID(in.File, lineNo, lineNo, offset, part.ContentSHA256)
				part.ParentID, part.ParentStartLine, part.ParentEndLine = parentID, sp.StartLine, sp.EndLine
				part.PartOrdinal = len(out) + 1
				out = append(out, part)
				offset += end
				line = line[end:]
			}
			lineNo++
			startLine = lineNo
			continue
		}
		if length(pending)+len(line) > maxChars {
			// Prefer a paragraph, closed code fence, or list item boundary.
			// A blank line inside a fence is content, not a paragraph break.
			cut := docBoundaryCut(pending)
			emit(pending[:cut], startLine)
			startLine += cut
			pending = append([]string(nil), pending[cut:]...)
			if length(pending)+len(line) > maxChars {
				emit(pending, startLine)
				startLine += len(pending)
				pending = nil
			}
		}
		pending = append(pending, line)
		lineNo++
	}
	emit(pending, startLine)
	return out
}

func docBoundaryCut(lines []string) int {
	lastBlank, lastFenceClose, lastListItem := 0, 0, 0
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if fence == "" {
				fence = marker
			} else if marker == fence {
				fence = ""
				if i+1 < len(lines) {
					lastFenceClose = i + 1
				}
			}
			continue
		}
		if fence != "" {
			continue
		}
		if trimmed == "" && i+1 < len(lines) {
			lastBlank = i + 1
		}
		if i > 0 && isMarkdownListItem(trimmed) {
			lastListItem = i
		}
	}
	switch {
	case lastBlank > 0:
		return lastBlank
	case lastFenceClose > 0:
		return lastFenceClose
	case lastListItem > 0:
		return lastListItem
	default:
		return len(lines)
	}
}

func isMarkdownListItem(line string) bool {
	if len(line) >= 2 && (line[0] == '-' || line[0] == '*' || line[0] == '+') && line[1] == ' ' {
		return true
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(line) && (line[i] == '.' || line[i] == ')') && line[i+1] == ' '
}

// fileFullChunk returns a coarse chunk spanning the entire file (Phase B).
// Distinct id/kind from the file_header chunk so both can coexist. The text is
// truncated to the embedder's input cap when necessary — a long file's coarse
// vector is still a useful file-level signal even head-truncated.
func (c *Chunker) fileFullChunk(in Input) *types.Chunk {
	if strings.TrimSpace(string(in.Source)) == "" {
		return nil
	}
	text := string(in.Source)
	endLine := sourceLineCount(text)
	text = c.maybeTruncate(text)
	contentHash := types.ContentSHA256(text)
	id := types.ChunkID(in.File, 0, endLine, contentHash)
	return &types.Chunk{
		ID:            id,
		File:          in.File,
		StartLine:     1,
		EndLine:       endLine,
		Language:      in.Language,
		IsTest:        types.IsTestPath(in.File, in.Language),
		SymbolKind:    types.KindFileHeader,
		ChunkKind:     types.ChunkFileFull,
		CommitHash:    in.CommitHash,
		ContentSHA256: contentHash,
		Text:          text,
	}
}

// shouldSplit reports whether a span gets the function-split treatment.
// Only function-like kinds (Function / Method) are eligible: type
// declarations and Solidity contracts/events are structurally too
// short to benefit, and markdown sections don't have signatures to
// preserve.
func (c *Chunker) shouldSplit(sp parse.SymbolSpan) bool {
	if c.opts.MaxInputTokens <= 0 {
		return false
	}
	maxChars := c.maxBytes()
	if len(sp.Text) <= maxChars {
		return false
	}
	switch sp.Kind {
	case types.KindFunction, types.KindMethod:
		return true
	default:
		return false
	}
}

// splitLongSpan slices the span body into multiple ChunkFunctionSplit
// chunks. Each chunk carries:
//
//   - SymbolName = "<original>:chunk:<n>" (1-indexed) so callers can
//     reassemble the order.
//   - ChunkKind = ChunkFunctionSplit so consumers can filter or
//     visually badge split results.
//   - StartLine/EndLine = the window's actual lines in the source
//     file. Citation lookups land on the right slice.
//
// The signature is *not* duplicated into each split's Text — the
// rule-based contextual prefix (BuildEmbedText) already prepends
// "symbol: X.Y" at embed time so the embedder still sees the symbol
// identity for every split. Snippet display reads Text directly, so
// the user sees the actual window content (signature shows only on
// chunk 1, which contains the function's opening line).
//
// Window math: divide len(sp.Text) by maxChars to get N (with one
// extra chunk for the remainder), then split sp.Text by line count
// evenly. No overlap — adding it is a future refinement when
// measurement shows recall improvement.
func (c *Chunker) splitLongSpan(in Input, sp parse.SymbolSpan) []types.Chunk {
	maxChars := c.maxBytes()
	lines := strings.Split(sp.Text, "\n")
	if len(lines) <= 1 {
		// Degenerate one-line giant — truncate as before.
		return []types.Chunk{c.symbolChunk(in, sp, c.maybeTruncate(sp.Text))}
	}

	// Aim each window for ~maxChars of content. avg chars per line of
	// this span gives us a windowSize that respects the cap.
	avgCharsPerLine := max(1, (len(sp.Text)+len(lines)-1)/len(lines))
	windowLines := max(1, maxChars/avgCharsPerLine)

	chunks := make([]types.Chunk, 0, (len(lines)+windowLines-1)/windowLines)
	for i, idx := 0, 0; i < len(lines); i += windowLines {
		end := min(i+windowLines, len(lines))
		windowText := strings.Join(lines[i:end], "\n")
		// File-relative line range: span starts at sp.StartLine, so the
		// window starts at sp.StartLine + i. Inclusive on both ends.
		startLine := sp.StartLine + i
		endLine := sp.StartLine + end - 1
		idx++

		contentHash := types.ContentSHA256(windowText)
		id := types.ChunkID(in.File, startLine, endLine, contentHash)
		chunks = append(chunks, types.Chunk{
			ID:            id,
			File:          in.File,
			StartLine:     startLine,
			EndLine:       endLine,
			Language:      in.Language,
			IsTest:        types.IsTestPath(in.File, in.Language),
			SymbolName:    fmt.Sprintf("%s:chunk:%d", sp.Name, idx),
			SymbolKind:    sp.Kind,
			ChunkKind:     types.ChunkFunctionSplit,
			CommitHash:    in.CommitHash,
			ContentSHA256: contentHash,
			Text:          windowText,
		})
	}
	return chunks
}

// sourceLineCount counts physical lines without inventing a line after EOF.
func sourceLineCount(text string) int {
	if text == "" {
		return 0
	}
	lines := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		lines++
	}
	return lines
}

// fileHeaderChunk emits the leading-lines chunk. Returns nil for empty
// or entirely blank header text.
func (c *Chunker) fileHeaderChunk(in Input) *types.Chunk {
	if len(in.Source) == 0 {
		return nil
	}
	limit := c.opts.FileHeaderLines
	if limit <= 0 {
		limit = DefaultFileHeaderLines
	}
	lines := strings.SplitN(string(in.Source), "\n", limit+1)
	if len(lines) == 0 {
		return nil
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	text := strings.Join(lines, "\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	text = c.maybeTruncate(text)
	contentHash := types.ContentSHA256(text)
	// SplitN includes an empty sentinel after a final newline. It is not
	// another source line; preserve the text bytes without citing that sentinel.
	endLine := min(len(lines), sourceLineCount(string(in.Source)))
	id := types.ChunkID(in.File, 1, endLine, contentHash)
	return &types.Chunk{
		ID:            id,
		File:          in.File,
		StartLine:     1,
		EndLine:       endLine,
		Language:      in.Language,
		IsTest:        types.IsTestPath(in.File, in.Language),
		SymbolKind:    types.KindFileHeader,
		ChunkKind:     types.ChunkFileHeader,
		CommitHash:    in.CommitHash,
		ContentSHA256: contentHash,
		Text:          text,
	}
}

func (c *Chunker) symbolChunk(in Input, sp parse.SymbolSpan, text string) types.Chunk {
	contentHash := types.ContentSHA256(text)
	id := types.ChunkID(in.File, sp.StartLine, sp.EndLine, contentHash)
	kind := types.ChunkSymbol
	if sp.Kind == types.KindDocSection || sp.Kind == types.KindADRSection {
		kind = types.ChunkDoc
	}
	return types.Chunk{
		ID:            id,
		File:          in.File,
		StartLine:     sp.StartLine,
		EndLine:       sp.EndLine,
		Language:      in.Language,
		IsTest:        types.IsTestPath(in.File, in.Language),
		SymbolName:    sp.Name,
		SymbolKind:    sp.Kind,
		ChunkKind:     kind,
		HeadingPath:   append([]string(nil), sp.HeadingPath...),
		CommitHash:    in.CommitHash,
		ContentSHA256: contentHash,
		Text:          text,
	}
}

// maybeTruncate enforces the embedder's input cap. Keeps the prefix so
// the signature stays embedded. The trailing "// ..." marker makes the
// truncation explicit in audit/eval logs without changing semantics.
func (c *Chunker) maybeTruncate(text string) string {
	if c.opts.MaxInputTokens <= 0 && c.opts.MaxTextBytes <= 0 {
		return text
	}
	max := c.maxBytes()
	if len(text) <= max {
		return text
	}
	const marker = "\n// ... [CKV-TRUNCATED]"
	if max <= len(marker) {
		return utf8Prefix(text, max)
	}
	return utf8Prefix(text, max-len(marker)) + marker
}

func utf8Prefix(text string, limit int) string {
	if limit >= len(text) {
		return text
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return text[:limit]
}

// Stats summarizes the output of one Chunk() call; useful for build
// progress and the bootstrap report.
type Stats struct {
	Total         int
	Symbol        int
	FileHeader    int
	Doc           int
	FunctionSplit int
	PRDoc         int
	Invariant     int
	Truncated     int
	// FlowStep / FlowSpine count the flow-corpus chunks (the bridge layer).
	FlowStep  int
	FlowSpine int
	// CanonicalID counts alignable code-symbol chunks carrying a non-empty
	// canonical_id. Headers, invariants and other kinds may carry a join key
	// too, but they must not inflate the Symbol denominator's coverage.
	CanonicalID int
}

// Summarize counts chunk kinds. Cheap O(n) pass over the slice.
func Summarize(chunks []types.Chunk) Stats {
	var s Stats
	for _, c := range chunks {
		s.Total++
		switch c.ChunkKind {
		case types.ChunkSymbol:
			s.Symbol++
		case types.ChunkFileHeader:
			s.FileHeader++
		case types.ChunkDoc:
			s.Doc++
		case types.ChunkFunctionSplit:
			s.Symbol++
			s.FunctionSplit++
		case types.ChunkPRBackground, types.ChunkPRSolution, types.ChunkCommitMessage:
			s.PRDoc++
		case types.ChunkInvariant:
			s.Invariant++
		case types.ChunkFlowStep:
			s.FlowStep++
		case types.ChunkFlowSpine:
			s.FlowSpine++
		}
		if c.CanonicalID != "" && (c.ChunkKind == types.ChunkSymbol || c.ChunkKind == types.ChunkFunctionSplit) && c.StartLine > 0 {
			s.CanonicalID++
		}
		if strings.HasSuffix(c.Text, "\n// ... [CKV-TRUNCATED]") {
			s.Truncated++
		}
	}
	return s
}

// formatLineRange is a tiny utility kept here (not in pkg/types) because
// it's only used for log + progress strings, not the chunk model.
func formatLineRange(start, end int) string {
	return fmt.Sprintf("%d-%d", start, end)
}

var _ = formatLineRange // reserved for future progress logging
