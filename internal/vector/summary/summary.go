// Package summary writes one LLM summary chunk per symbol chunk: what the
// symbol is for and when someone would change it, in English and Korean.
//
// Why a separate chunk and not a longer prefix: the llmprefix sentence is
// embedded together with the code body, so the body still dominates the
// vector. A question in Korean about a purpose ("where does a new hardfork
// register its upgrade") shares no words with the body. A summary chunk is a
// vector of the purpose alone, in the language of the question, and it carries
// the symbol's own span so a hit on it cites the code.
//
// Only the code at the indexed commit is shown to the model — never PR text —
// so evaluation questions cannot leak into the index. A summary that names an
// identifier absent from the code is dropped: a wrong summary would pull
// retrieval to the wrong place, which is worse than no summary.
package summary

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/vector/llmprefix"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

// PromptVersion keys the cache together with the model, so a prompt change
// regenerates instead of reusing summaries written for the old prompt.
const PromptVersion = "v1"

// minLines skips one- and two-line symbols (field accessors, constants):
// their body already says everything and they are the bulk of the symbols.
const minLines = 3

// Summarizer appends summary chunks. Results are disk-cached by model,
// prompt version and chunk text, so a rebuild or another commit with the
// same symbol body reuses them.
type Summarizer struct {
	gen      llmprefix.Generator
	model    string
	cacheDir string
	stats    Stats
}

// Stats counts what Append did, for the build log.
type Stats struct {
	Eligible  int // symbol chunks offered for a summary
	Generated int // fresh model calls that produced a kept summary
	Cached    int // summaries read from the cache
	Rejected  int // model output dropped by validation
	Failed    int // generation errors
}

// New returns a Summarizer caching under cacheDir (created if absent). A nil
// generator makes Append a no-op.
func New(gen llmprefix.Generator, model, cacheDir string) (*Summarizer, error) {
	if cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return nil, fmt.Errorf("summary: cache dir: %w", err)
		}
	}
	return &Summarizer{gen: gen, model: model, cacheDir: cacheDir}, nil
}

// Stats returns the counts accumulated so far.
func (s *Summarizer) Stats() Stats {
	if s == nil {
		return Stats{}
	}
	return s.stats
}

// Eligible reports whether c gets a summary: a whole symbol with a source
// span, not a test, at least minLines long.
func Eligible(c types.Chunk) bool {
	return c.ChunkKind == types.ChunkSymbol && c.StartLine > 0 &&
		c.EndLine-c.StartLine+1 >= minLines && !c.IsTest && c.Language != "markdown"
}

// Append returns chunks followed by one summary chunk for every eligible
// chunk whose summary was generated (or cached) and passed validation.
func (s *Summarizer) Append(ctx context.Context, chunks []types.Chunk) []types.Chunk {
	if s == nil || s.gen == nil {
		return chunks
	}
	for _, c := range chunks { // range reads the original slice once; appends are not revisited
		if !Eligible(c) {
			continue
		}
		s.stats.Eligible++
		text, ok := s.summarize(ctx, c)
		if !ok {
			continue
		}
		chunks = append(chunks, summaryChunk(c, text))
	}
	return chunks
}

func (s *Summarizer) summarize(ctx context.Context, c types.Chunk) (string, bool) {
	key := s.cacheKey(c)
	if v, ok := s.readCache(key); ok {
		if v == "" {
			// A cached rejection: do not ask the model again.
			return "", false
		}
		s.stats.Cached++
		return v, true
	}
	out, err := s.gen.Generate(ctx, BuildPrompt(c))
	if err != nil {
		// Not cached: a model outage must not leave permanent gaps.
		s.stats.Failed++
		return "", false
	}
	text, ok := Validate(out, c)
	if !ok {
		s.stats.Rejected++
		s.writeCache(key, "")
		return "", false
	}
	s.stats.Generated++
	s.writeCache(key, text)
	return text, true
}

// summaryChunk copies the identity and span of the symbol chunk c and
// replaces the text. The ID hashes the summary text, so it never collides
// with the code chunk of the same span.
func summaryChunk(c types.Chunk, text string) types.Chunk {
	sha := types.ContentSHA256(text)
	return types.Chunk{
		ID:            types.ChunkID(c.File, c.StartLine, c.EndLine, sha),
		File:          c.File,
		StartLine:     c.StartLine,
		EndLine:       c.EndLine,
		Language:      c.Language,
		IsTest:        c.IsTest,
		SymbolName:    c.SymbolName,
		SymbolKind:    c.SymbolKind,
		ChunkKind:     types.ChunkSymbolSummary,
		CommitHash:    c.CommitHash,
		ContentSHA256: sha,
		CanonicalID:   c.CanonicalID,
		Text:          text,
	}
}

// BuildPrompt asks for two lines: the purpose in English, then the same in
// Korean. It names the symbol and file so the model does not have to guess
// them, and asks for the situations that would lead someone to change the
// code, since questions name the situation, not the function.
func BuildPrompt(c types.Chunk) string {
	lang := c.Language
	if lang == "" {
		lang = "code"
	}
	return fmt.Sprintf("You are indexing a codebase so that developers can find code by describing what they want "+
		"to change. Read the %s symbol %q from %q below.\n"+
		"Write exactly two lines and nothing else:\n"+
		"EN: what this symbol is for and in which situations a developer would need to change or extend it "+
		"(at most two sentences).\n"+
		"KO: the same content written in Korean.\n"+
		"Only mention identifiers that appear in the code below.\n\n%s",
		lang, c.SymbolName, c.File, c.Text)
}

// maxLineRunes caps each line; longer output is a model ignoring the format.
const maxLineRunes = 600

var (
	// identLike matches tokens that look like code identifiers rather than
	// prose: dotted names, snake_case, or camelCase with an inner capital.
	identLike = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]+(?:\.[A-Za-z_][A-Za-z0-9_]+)+|[A-Za-z0-9]*_[A-Za-z0-9_]+|[a-z][a-z0-9]*[A-Z][A-Za-z0-9]*|[A-Z][a-z0-9]+[A-Z][A-Za-z0-9]*`)
	hangul    = regexp.MustCompile(`\p{Hangul}`)
)

// Validate parses the model output into "EN: ...\nKO: ..." and rejects it
// when either line is missing, too long, the Korean line has no Hangul, or
// the text names an identifier that the symbol, its file path and its body
// do not contain.
func Validate(out string, c types.Chunk) (string, bool) {
	var en, ko string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "*"))
		switch {
		case en == "" && strings.HasPrefix(line, "EN:"):
			en = strings.TrimSpace(strings.TrimPrefix(line, "EN:"))
		case ko == "" && strings.HasPrefix(line, "KO:"):
			ko = strings.TrimSpace(strings.TrimPrefix(line, "KO:"))
		}
	}
	if en == "" || ko == "" || len([]rune(en)) > maxLineRunes || len([]rune(ko)) > maxLineRunes {
		return "", false
	}
	if !hangul.MatchString(ko) {
		return "", false
	}
	known := c.Text + "\n" + c.SymbolName + "\n" + c.File
	for _, id := range identLike.FindAllString(en+" "+ko, -1) {
		if !strings.Contains(known, strings.TrimRight(id, ".")) {
			return "", false
		}
	}
	return en + "\n" + ko, true
}

func (s *Summarizer) cacheKey(c types.Chunk) string {
	sum := sha256.Sum256([]byte(PromptVersion + "\x00" + s.model + "\x00" + c.SymbolName + "\x00" + c.Text))
	return hex.EncodeToString(sum[:])
}

func (s *Summarizer) cachePath(key string) string {
	return filepath.Join(s.cacheDir, key[:2], key+".txt")
}

func (s *Summarizer) readCache(key string) (string, bool) {
	if s.cacheDir == "" {
		return "", false
	}
	b, err := os.ReadFile(s.cachePath(key))
	if err != nil {
		return "", false
	}
	return string(b), true
}

func (s *Summarizer) writeCache(key, val string) {
	if s.cacheDir == "" {
		return
	}
	p := s.cachePath(key)
	if os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return
	}
	tmp := p + ".tmp"
	if os.WriteFile(tmp, []byte(val), 0o644) == nil {
		_ = os.Rename(tmp, p)
	}
}
