package golang

import (
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

func TestParseExtractsFuncMethodAndType(t *testing.T) {
	src := []byte(`package x

// Greet returns a greeting.
func Greet(name string) string {
	return "hello, " + name
}

type Server struct {
	addr string
}

func (s *Server) Serve() error {
	return nil
}

type Handler interface {
	Handle()
}
`)
	p := New()
	spans, err := p.Parse("x.go", src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := map[string]types.SymbolKind{
		"Greet":        types.KindFunction,
		"Server":       types.KindStruct,
		"Server.Serve": types.KindMethod,
		"Handler":      types.KindInterface,
	}
	if len(spans) != len(want) {
		t.Fatalf("got %d spans, want %d (%+v)", len(spans), len(want), spans)
	}
	for _, s := range spans {
		k, ok := want[s.Name]
		if !ok {
			t.Errorf("unexpected span: %s (%s)", s.Name, s.Kind)
			continue
		}
		if s.Kind != k {
			t.Errorf("%s: got kind %s, want %s", s.Name, s.Kind, k)
		}
		if s.StartLine == 0 || s.EndLine < s.StartLine {
			t.Errorf("%s: bad line range %d-%d", s.Name, s.StartLine, s.EndLine)
		}
		if s.Text == "" {
			t.Errorf("%s: empty Text", s.Name)
		}
	}
}

func TestParseExtractsConstAndVarBlocks(t *testing.T) {
	// Regression for the "invisible const" gap: top-level const/var used to
	// be left to the chunker's 50-line file_header fallback, so any
	// declaration past line 50 (long enum blocks, late sentinels) could not
	// be retrieved at all.
	src := []byte(`package x

// Intent enumerates retrieval intents.
const (
	// IntentBugFix — "fix this bug".
	IntentBugFix = "bug_fix"
	// IntentQAReview — "review this PR".
	IntentQAReview = "qa_review"
)

// ErrFailClosed is returned when a fail_closed rule matched.
var ErrFailClosed = errNew("fail closed")

func errNew(s string) error { return nil }
`)
	p := New()
	spans, err := p.Parse("x.go", src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var constSpan, varSpan *struct {
		start, end int
		text       string
	}
	for _, s := range spans {
		switch {
		case s.Kind == types.KindConst && s.Name == "IntentBugFix":
			constSpan = &struct {
				start, end int
				text       string
			}{s.StartLine, s.EndLine, s.Text}
		case s.Kind == types.KindVar && s.Name == "ErrFailClosed":
			varSpan = &struct {
				start, end int
				text       string
			}{s.StartLine, s.EndLine, s.Text}
		}
	}
	if constSpan == nil {
		t.Fatalf("no Const span named after the first ident; spans=%+v", spans)
	}
	// Block doc comment included: span starts at "// Intent enumerates" (line 3).
	if constSpan.start != 3 || constSpan.end != 9 {
		t.Errorf("const block span = %d-%d, want 3-9", constSpan.start, constSpan.end)
	}
	for _, needle := range []string{"Intent enumerates", "IntentQAReview", "review this PR"} {
		if !strings.Contains(constSpan.text, needle) {
			t.Errorf("const block text missing %q", needle)
		}
	}
	if varSpan == nil {
		t.Fatalf("no Var span named ErrFailClosed; spans=%+v", spans)
	}
	if varSpan.start != 11 || varSpan.end != 12 {
		t.Errorf("var span = %d-%d, want 11-12", varSpan.start, varSpan.end)
	}
	if !strings.Contains(varSpan.text, "fail_closed rule matched") {
		t.Errorf("var text missing its doc comment: %q", varSpan.text)
	}
}

func TestParseHandlesGenericReceivers(t *testing.T) {
	src := []byte(`package x

type Box[T any] struct {
	v T
}

func (b *Box[T]) Get() T {
	return b.v
}
`)
	p := New()
	spans, err := p.Parse("box.go", src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var found bool
	for _, s := range spans {
		if s.Name == "Box.Get" && s.Kind == types.KindMethod {
			found = true
		}
	}
	if !found {
		t.Errorf("generic-receiver method not extracted as Box.Get; got %+v", spans)
	}
}

func TestParseSurfacesSyntaxError(t *testing.T) {
	src := []byte(`package x
func busted( {
`)
	_, err := New().Parse("busted.go", src)
	if err == nil {
		t.Fatal("expected parse error for malformed file")
	}
}

// TestParseIncludesDocCommentsInFuncAndTypeSpans pins the doc-comment
// inclusion added 2026-07-29: the leading comment is where the
// natural-language signal lives, so it belongs to the span (text and
// line range both).
func TestParseIncludesDocCommentsInFuncAndTypeSpans(t *testing.T) {
	src := []byte(`package x

// Real is the in-process adapter.
// Concurrency: safe for reads.
type Real struct {
	v int
}

// Serve runs the loop until ctx is done.
func (r *Real) Serve() error {
	return nil
}
`)
	p := New()
	spans, err := p.Parse("x.go", src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	byName := map[string]struct {
		start int
		text  string
	}{}
	for _, s := range spans {
		byName[s.Name] = struct {
			start int
			text  string
		}{s.StartLine, s.Text}
	}

	real, ok := byName["Real"]
	if !ok {
		t.Fatalf("no Real span: %+v", spans)
	}
	if real.start != 3 {
		t.Errorf("Real span start = %d, want 3 (doc comment line)", real.start)
	}
	if !strings.Contains(real.text, "Concurrency: safe for reads") {
		t.Errorf("Real span text missing doc comment: %q", real.text)
	}

	serve, ok := byName["Real.Serve"]
	if !ok {
		t.Fatalf("no Real.Serve span: %+v", spans)
	}
	if serve.start != 9 {
		t.Errorf("Serve span start = %d, want 9 (doc comment line)", serve.start)
	}
	if !strings.Contains(serve.text, "runs the loop until ctx is done") {
		t.Errorf("Serve span text missing doc comment: %q", serve.text)
	}
}
