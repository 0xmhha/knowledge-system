package chunk

import (
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/vector/parse"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

func TestChunkSymbolAndFileHeader(t *testing.T) {
	src := []byte(`package x

import "fmt"

func A() { fmt.Println("a") }
func B() { fmt.Println("b") }
`)
	in := Input{
		File:       "x.go",
		Language:   "go",
		CommitHash: "abc",
		Source:     src,
		Spans: []parse.SymbolSpan{
			{Name: "A", Kind: types.KindFunction, StartLine: 5, EndLine: 5, Text: `func A() { fmt.Println("a") }`},
			{Name: "B", Kind: types.KindFunction, StartLine: 6, EndLine: 6, Text: `func B() { fmt.Println("b") }`},
		},
	}
	chunks := New(Options{}).Chunk(in)
	stats := Summarize(chunks)
	if stats.Total != 3 || stats.Symbol != 2 || stats.FileHeader != 1 {
		t.Fatalf("expected 1 file_header + 2 symbol chunks, got %+v", stats)
	}
}

func TestIncludeFileFullEmitsCoarseChunk(t *testing.T) {
	src := []byte(`package x

import "fmt"

func A() { fmt.Println("a") }
func B() { fmt.Println("b") }
`)
	in := Input{
		File:       "x.go",
		Language:   "go",
		CommitHash: "abc",
		Source:     src,
		Spans: []parse.SymbolSpan{
			{Name: "A", Kind: types.KindFunction, StartLine: 5, EndLine: 5, Text: `func A() { fmt.Println("a") }`},
			{Name: "B", Kind: types.KindFunction, StartLine: 6, EndLine: 6, Text: `func B() { fmt.Println("b") }`},
		},
	}

	// Off by default: no file_full chunk.
	for _, c := range New(Options{}).Chunk(in) {
		if c.ChunkKind == types.ChunkFileFull {
			t.Fatal("file_full emitted without IncludeFileFull")
		}
	}

	// On: exactly one additive file_full chunk spanning the whole file, with a
	// distinct ID from the file_header chunk (so both coexist in the store).
	chunks := New(Options{IncludeFileFull: true}).Chunk(in)
	var full, header *types.Chunk
	for i := range chunks {
		switch chunks[i].ChunkKind {
		case types.ChunkFileFull:
			if full != nil {
				t.Fatal("more than one file_full chunk")
			}
			full = &chunks[i]
		case types.ChunkFileHeader:
			header = &chunks[i]
		}
	}
	if full == nil || header == nil {
		t.Fatalf("want both file_full and file_header, got full=%v header=%v", full != nil, header != nil)
	}
	if full.Text != string(src) {
		t.Errorf("file_full text should be the whole file")
	}
	if full.ID == header.ID {
		t.Errorf("file_full and file_header must have distinct IDs")
	}
}

func TestChunkIDsDeterministic(t *testing.T) {
	in := Input{
		File:       "x.go",
		Language:   "go",
		CommitHash: "abc",
		Source:     []byte("package x\n\nfunc A() {}\n"),
		Spans: []parse.SymbolSpan{
			{Name: "A", Kind: types.KindFunction, StartLine: 3, EndLine: 3, Text: "func A() {}"},
		},
	}
	a := New(Options{}).Chunk(in)
	b := New(Options{}).Chunk(in)
	if len(a) != len(b) {
		t.Fatalf("chunk count differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Errorf("chunk %d id differs: %s vs %s", i, a[i].ID, b[i].ID)
		}
	}
}

func TestLongMarkdownSectionKeepsTailAndLineCitations(t *testing.T) {
	text := "# Decision\n\n" + strings.Repeat("A short opening paragraph.\n", 8) +
		"\nThe final requirement is QUORUM_TAIL.\n"
	in := Input{
		File: "docs/decision.md", Language: "markdown", CommitHash: "abc",
		Source: []byte(text),
		Spans: []parse.SymbolSpan{{Name: "decision", Kind: types.KindDocSection,
			StartLine: 1, EndLine: strings.Count(text, "\n"), Text: text,
			HeadingPath: []string{"Architecture", "Decision"}}},
	}
	chunks := New(Options{MaxInputTokens: 20}).Chunk(in)
	if len(chunks) < 2 {
		t.Fatalf("long section was not split: %d chunks", len(chunks))
	}
	var combined strings.Builder
	parentID := chunks[0].ParentID
	for i, ch := range chunks {
		if ch.ChunkKind != types.ChunkDoc {
			t.Fatalf("unexpected kind: %s", ch.ChunkKind)
		}
		if len(ch.Text) > 20*charsPerToken {
			t.Fatalf("child too long: %d", len(ch.Text))
		}
		if parentID == "" || ch.ParentID != parentID || ch.ParentStartLine != 1 || ch.ParentEndLine != in.Spans[0].EndLine || ch.PartOrdinal != i+1 {
			t.Fatalf("invalid parent/ordinal metadata: %+v", ch)
		}
		if len(ch.HeadingPath) != 2 || ch.HeadingPath[1] != "Decision" {
			t.Fatalf("heading path lost: %+v", ch.HeadingPath)
		}
		combined.WriteString(ch.Text)
	}
	if combined.String() != text {
		t.Fatalf("section content changed or lost")
	}
	last := chunks[len(chunks)-1]
	if !strings.Contains(last.Text, "QUORUM_TAIL") || last.EndLine != in.Spans[0].EndLine {
		t.Fatalf("tail missing or cited at wrong line: %+v", last)
	}
}

func TestLongSingleMarkdownLineHasDistinctChildIDs(t *testing.T) {
	text := "# Heading\n" + strings.Repeat("A", 160) + "\n"
	in := Input{File: "long.md", Language: "markdown", CommitHash: "abc", Source: []byte(text),
		Spans: []parse.SymbolSpan{{Name: "heading", Kind: types.KindDocSection, StartLine: 1, EndLine: 2, Text: text}}}
	chunks := New(Options{MaxInputTokens: 20}).Chunk(in)
	seen := map[string]bool{}
	var joined strings.Builder
	for _, ch := range chunks {
		if seen[ch.ID] {
			t.Fatalf("duplicate ID for repeated fragment: %s", ch.ID)
		}
		seen[ch.ID] = true
		joined.WriteString(ch.Text)
	}
	if joined.String() != text {
		t.Fatalf("single-line split lost source text")
	}
}

func TestLongMarkdownSplitPrefersFenceAndListBoundaries(t *testing.T) {
	if got := docBoundaryCut([]string{"```go\n", "alpha\n", "\n", "beta\n", "```\n", "tail\n"}); got != 5 {
		t.Fatalf("split inside fenced code block at %d", got)
	}
	if got := docBoundaryCut([]string{"- first\n", "  continuation\n", "- second\n"}); got != 2 {
		t.Fatalf("split inside list item at %d", got)
	}
	text := "# Guide\n" + "```go\n" + strings.Repeat("fmt.Println(1)\n", 5) + "```\n" +
		"- first item\n" + "  continuation\n" + "- second item\n" + strings.Repeat("tail line\n", 8)
	in := Input{File: "guide.md", Language: "markdown", CommitHash: "abc", Source: []byte(text),
		Spans: []parse.SymbolSpan{{Name: "guide", Kind: types.KindDocSection, StartLine: 1,
			EndLine: strings.Count(text, "\n"), Text: text}}}
	chunks := New(Options{MaxInputTokens: 30}).Chunk(in)
	if len(chunks) < 2 {
		t.Fatalf("expected split, got %d", len(chunks))
	}
	var combined strings.Builder
	for _, part := range chunks {
		if len(part.Text) > 30*charsPerToken {
			t.Fatalf("child exceeds embedding cap: %d bytes", len(part.Text))
		}
		combined.WriteString(part.Text)
	}
	if combined.String() != text {
		t.Fatal("split did not preserve fenced/list source bytes")
	}
}

func TestTruncationKeepsHeadAndMarker(t *testing.T) {
	long := strings.Repeat("a", 1000)
	in := Input{
		File:       "x.go",
		Language:   "go",
		CommitHash: "abc",
		Source:     []byte("package x\n"),
		Spans: []parse.SymbolSpan{
			{Name: "Big", Kind: types.KindFunction, StartLine: 1, EndLine: 1, Text: long},
		},
	}
	// MaxInputTokens=50 → ~200 chars
	chunks := New(Options{MaxInputTokens: 50}).Chunk(in)
	// Find the symbol chunk (not file_header).
	var bigChunk types.Chunk
	for _, c := range chunks {
		if c.ChunkKind == types.ChunkSymbol {
			bigChunk = c
			break
		}
	}
	if len(bigChunk.Text) > 50*charsPerToken {
		t.Errorf("text not truncated: len=%d", len(bigChunk.Text))
	}
	if !strings.Contains(bigChunk.Text, "[CKV-TRUNCATED]") {
		t.Errorf("truncation marker missing: %q", bigChunk.Text)
	}
	if !strings.HasPrefix(bigChunk.Text, "aaa") {
		t.Errorf("head not preserved: %q", bigChunk.Text)
	}
}

// TestLongFunctionSplitsIntoMultipleChunks verifies that a multi-line
// function body exceeding MaxInputTokens gets split into N
// ChunkFunctionSplit chunks instead of being head-truncated.
func TestLongFunctionSplitsIntoMultipleChunks(t *testing.T) {
	// 60 distinct lines, each ~20 chars → ~1200 chars total.
	bodyLines := []string{}
	for range 60 {
		bodyLines = append(bodyLines, "  doWork(stepNum)  ")
	}
	long := "func Big() {\n" + strings.Join(bodyLines, "\n") + "\n}"

	// MaxInputTokens=50 → ~200 char cap. Body is ~1200 chars → must split.
	chunks := New(Options{MaxInputTokens: 50}).Chunk(Input{
		File:       "x.go",
		Language:   "go",
		CommitHash: "abc",
		Source:     []byte("package x\n"),
		Spans: []parse.SymbolSpan{
			{Name: "Big", Kind: types.KindFunction, StartLine: 10, EndLine: 71, Text: long},
		},
	})

	// Find every function_split chunk.
	splits := []types.Chunk{}
	for _, c := range chunks {
		if c.ChunkKind == types.ChunkFunctionSplit {
			splits = append(splits, c)
		}
	}
	if len(splits) < 2 {
		t.Fatalf("expected ≥2 function_split chunks; got %d (chunks=%d)", len(splits), len(chunks))
	}

	// Every split must carry the :chunk:N suffix and a distinct line range.
	seenLines := map[int]bool{}
	for i, c := range splits {
		want := "Big:chunk:" + itoa(i+1)
		if c.SymbolName != want {
			t.Errorf("splits[%d].SymbolName = %q, want %q", i, c.SymbolName, want)
		}
		if c.SymbolKind != types.KindFunction {
			t.Errorf("splits[%d].SymbolKind = %q, want Function", i, c.SymbolKind)
		}
		if c.StartLine < 10 || c.EndLine > 71 {
			t.Errorf("splits[%d] line range out of span: %d-%d (span 10-71)", i, c.StartLine, c.EndLine)
		}
		if seenLines[c.StartLine] {
			t.Errorf("splits[%d] StartLine=%d duplicated; windows must be disjoint", i, c.StartLine)
		}
		seenLines[c.StartLine] = true
		if c.ID == "" {
			t.Errorf("splits[%d] missing chunk_id", i)
		}
	}
	// Splits must be ordered by start_line so reassembly is trivial.
	for i := 1; i < len(splits); i++ {
		if splits[i].StartLine <= splits[i-1].StartLine {
			t.Errorf("splits not in line order: [%d].StartLine=%d ≤ [%d].StartLine=%d",
				i, splits[i].StartLine, i-1, splits[i-1].StartLine)
		}
	}
}

// TestShortFunctionNotSplit verifies the threshold side: a function
// comfortably under MaxInputTokens still goes through the regular
// single-chunk path (ChunkSymbol, no :chunk:N suffix).
func TestShortFunctionNotSplit(t *testing.T) {
	short := "func Small() {\n  return 1\n}"
	chunks := New(Options{MaxInputTokens: 50}).Chunk(Input{
		File: "x.go", Language: "go", CommitHash: "abc",
		Source: []byte("package x\n"),
		Spans: []parse.SymbolSpan{
			{Name: "Small", Kind: types.KindFunction, StartLine: 1, EndLine: 3, Text: short},
		},
	})
	for _, c := range chunks {
		if c.ChunkKind == types.ChunkFunctionSplit {
			t.Errorf("short function should not split; got %+v", c)
		}
		if strings.Contains(c.SymbolName, ":chunk:") {
			t.Errorf("short function should not carry chunk suffix; got %q", c.SymbolName)
		}
	}
}

// TestSplitOnlyForFunctionLikeKinds verifies splitting is restricted
// to Function/Method. Types, structs, interfaces, contracts, and
// markdown sections fall back to truncation — splitting prose loses
// structure and Solidity/TS types are typically short anyway.
func TestSplitOnlyForFunctionLikeKinds(t *testing.T) {
	long := "struct Big {\n" + strings.Repeat("  field T\n", 60) + "}"
	chunks := New(Options{MaxInputTokens: 50}).Chunk(Input{
		File: "x.go", Language: "go", CommitHash: "abc",
		Source: []byte("package x\n"),
		Spans: []parse.SymbolSpan{
			{Name: "BigStruct", Kind: types.KindStruct, StartLine: 1, EndLine: 62, Text: long},
		},
	})
	for _, c := range chunks {
		if c.ChunkKind == types.ChunkFunctionSplit {
			t.Errorf("non-function kind should not split; got %+v", c)
		}
	}
}

// TestSplitChunkIDsAreDistinct ensures every split gets a unique
// chunk_id. Critical for the store's PRIMARY KEY constraint and for
// incremental reindex (DeleteByFile + Upsert sequence).
func TestSplitChunkIDsAreDistinct(t *testing.T) {
	bodyLines := []string{}
	for range 80 {
		bodyLines = append(bodyLines, "  step()")
	}
	long := "func Big() {\n" + strings.Join(bodyLines, "\n") + "\n}"
	chunks := New(Options{MaxInputTokens: 30}).Chunk(Input{
		File: "x.go", Language: "go", CommitHash: "abc",
		Source: []byte("package x\n"),
		Spans: []parse.SymbolSpan{
			{Name: "Big", Kind: types.KindFunction, StartLine: 1, EndLine: 82, Text: long},
		},
	})
	ids := map[string]bool{}
	for _, c := range chunks {
		if ids[c.ID] {
			t.Errorf("duplicate chunk_id: %s", c.ID)
		}
		ids[c.ID] = true
	}
}

// itoa is a tiny helper kept local so the test file doesn't pull in
// strconv just for one call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := []byte{}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}

func TestEmptyFileNoHeader(t *testing.T) {
	chunks := New(Options{}).Chunk(Input{File: "empty.go", Language: "go", CommitHash: "x", Source: nil})
	if len(chunks) != 0 {
		t.Errorf("empty file should produce no chunks, got %d", len(chunks))
	}
}

// Markdown inputs skip the file_header chunk because every heading
// section is already its own chunk — emitting a leading-N-lines chunk
// on top would duplicate the first section verbatim and inflate
// retrieval noise.
func TestMarkdownSkipsFileHeader(t *testing.T) {
	src := []byte("# Title\n\nbody\n\n## Sub\n\nmore\n")
	in := Input{
		File:       "x.md",
		Language:   "markdown",
		CommitHash: "abc",
		Source:     src,
		Spans: []parse.SymbolSpan{
			{Name: "title", Kind: types.KindDocSection, StartLine: 1, EndLine: 4, Text: "# Title\n\nbody\n\n"},
			{Name: "sub", Kind: types.KindDocSection, StartLine: 5, EndLine: 7, Text: "## Sub\n\nmore\n"},
		},
	}
	chunks := New(Options{}).Chunk(in)
	stats := Summarize(chunks)
	if stats.FileHeader != 0 {
		t.Errorf("markdown should not produce file_header chunks, got %d", stats.FileHeader)
	}
	if stats.Symbol != 0 {
		t.Errorf("DocSection spans should not produce symbol chunks (got %d) — chunk_kind=doc is the new classification", stats.Symbol)
	}
	if stats.Doc != 2 {
		t.Errorf("expected 2 doc chunks, got %d", stats.Doc)
	}
	for _, c := range chunks {
		if c.ChunkKind != types.ChunkDoc {
			t.Errorf("expected ChunkDoc for %s, got %s", c.SymbolName, c.ChunkKind)
		}
	}
}

// TestSummarize_CanonicalAndFlow covers the added counters: canonical_id
// coverage and the flow-corpus chunk kinds.
func TestSummarize_CanonicalAndFlow(t *testing.T) {
	chunks := []types.Chunk{
		{ChunkKind: types.ChunkSymbol, CanonicalID: "pkg.A", StartLine: 1},
		{ChunkKind: types.ChunkSymbol}, // unaligned: no canonical_id
		{ChunkKind: types.ChunkFlowStep, CanonicalID: "pkg.B"},
		{ChunkKind: types.ChunkFlowSpine},
		{ChunkKind: types.ChunkFileHeader, CanonicalID: "pkg.C"},
		{ChunkKind: types.ChunkFunctionSplit, CanonicalID: "pkg.D", StartLine: 2},
	}
	s := Summarize(chunks)
	if s.CanonicalID != 2 {
		t.Errorf("CanonicalID = %d, want 2", s.CanonicalID)
	}
	if s.FlowStep != 1 || s.FlowSpine != 1 {
		t.Errorf("flow counts = step %d / spine %d, want 1 / 1", s.FlowStep, s.FlowSpine)
	}
	if s.Symbol != 3 || s.Total != 6 {
		t.Errorf("symbol=%d total=%d, want 3 / 6", s.Symbol, s.Total)
	}
}
