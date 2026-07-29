// Package golang parses Go source via the stdlib go/parser+go/ast. This
// is the same idiom CKG uses — for Go specifically, stdlib is more
// accurate than tree-sitter (full type information available; resolves
// generics; canonical line/column numbers via go/token).
package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	cparse "github.com/0xmhha/knowledge-system/internal/vector/parse"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

// Parser is the Go-language SymbolSpan extractor.
type Parser struct{}

// New constructs a stateless parser. Parsers are cheap; instantiating
// one per file is fine.
func New() *Parser { return &Parser{} }

func (p *Parser) Language() string { return "go" }

// Parse extracts top-level func/method/type declarations. Nested
// closures inside functions are NOT lifted — they ride along with the
// enclosing function's chunk, which is the right granularity for
// retrieval (a closure rarely makes sense on its own).
func (p *Parser) Parse(file string, src []byte) ([]cparse.SymbolSpan, error) {
	fset := token.NewFileSet()
	// ParseComments so const/var block doc comments are attached (gen.Doc)
	// and can anchor the span start. Func/type spans are unaffected: they
	// slice from decl.Pos(), which never includes the doc comment.
	f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}

	var spans []cparse.SymbolSpan
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			spans = append(spans, p.funcSpan(fset, src, d))
		case *ast.GenDecl:
			spans = append(spans, p.genSpans(fset, src, d)...)
		}
	}
	return spans, nil
}

func (p *Parser) funcSpan(fset *token.FileSet, src []byte, fn *ast.FuncDecl) cparse.SymbolSpan {
	kind := types.KindFunction
	name := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		kind = types.KindMethod
		if recvName := receiverTypeName(fn.Recv.List[0]); recvName != "" {
			name = recvName + "." + fn.Name.Name
		}
	}
	// Start at the doc comment when present: the leading comment carries
	// the natural-language description a semantic query actually matches
	// ("// Greet returns a greeting."), and reviewers count those lines
	// as part of the symbol. ckgalign still resolves the canonical id via
	// its overlap tier (ckg nodes start at the declaration line).
	start := fn.Pos()
	if fn.Doc != nil {
		start = fn.Doc.Pos()
	}
	startPos := fset.Position(start)
	endPos := fset.Position(fn.End())
	return cparse.SymbolSpan{
		Name:      name,
		Kind:      kind,
		StartLine: startPos.Line,
		EndLine:   endPos.Line,
		Text:      sliceText(src, start, fn.End(), fset),
	}
}

func (p *Parser) genSpans(fset *token.FileSet, src []byte, gen *ast.GenDecl) []cparse.SymbolSpan {
	switch gen.Tok {
	case token.TYPE:
		var out []cparse.SymbolSpan
		for _, sp := range gen.Specs {
			ts, ok := sp.(*ast.TypeSpec)
			if !ok {
				continue
			}
			kind := types.KindType
			switch ts.Type.(type) {
			case *ast.StructType:
				kind = types.KindStruct
			case *ast.InterfaceType:
				kind = types.KindInterface
			}
			// Include the doc comment (same rationale as funcSpan). For the
			// common non-parenthesized `type X ...` the doc hangs on the
			// GenDecl, not the spec — take it only for single-spec decls so
			// a grouped block's header comment is not attributed to every
			// member.
			start := ts.Pos()
			switch {
			case ts.Doc != nil:
				start = ts.Doc.Pos()
			case gen.Doc != nil && len(gen.Specs) == 1:
				start = gen.Doc.Pos()
			}
			startPos := fset.Position(start)
			endPos := fset.Position(ts.End())
			out = append(out, cparse.SymbolSpan{
				Name:      ts.Name.Name,
				Kind:      kind,
				StartLine: startPos.Line,
				EndLine:   endPos.Line,
				Text:      sliceText(src, start, ts.End(), fset),
			})
		}
		return out
	case token.CONST, token.VAR:
		// One span per declaration BLOCK, doc comment included. These used
		// to be left to the chunker's file_header fallback (first 50 lines),
		// which made any const/var past line 50 — e.g. a long const enum
		// block with per-value doc comments — unreachable by retrieval.
		// Block granularity keeps grouped enums (iota ladders, intent
		// catalogs) as one coherent embedding; per-spec docs inside the
		// block ride along in the sliced text.
		return []cparse.SymbolSpan{p.valueBlockSpan(fset, src, gen)}
	default:
		// Imports stay uncovered on purpose — the file_header chunk has them.
		return nil
	}
}

// valueBlockSpan turns one const/var GenDecl into a single SymbolSpan named
// after its first declared identifier. The span starts at the block's doc
// comment when present so the natural-language signal ("ErrFailClosed is
// returned when ...") is part of the embedded text.
func (p *Parser) valueBlockSpan(fset *token.FileSet, src []byte, gen *ast.GenDecl) cparse.SymbolSpan {
	kind := types.KindConst
	if gen.Tok == token.VAR {
		kind = types.KindVar
	}
	name := ""
	for _, sp := range gen.Specs {
		if vs, ok := sp.(*ast.ValueSpec); ok && len(vs.Names) > 0 {
			name = vs.Names[0].Name
			break
		}
	}
	start := gen.Pos()
	if gen.Doc != nil {
		start = gen.Doc.Pos()
	}
	startPos := fset.Position(start)
	endPos := fset.Position(gen.End())
	return cparse.SymbolSpan{
		Name:      name,
		Kind:      kind,
		StartLine: startPos.Line,
		EndLine:   endPos.Line,
		Text:      sliceText(src, start, gen.End(), fset),
	}
}

// receiverTypeName turns the recv list of a method into the bare type
// name. Handles pointer (*T) and generic (T[U]) receivers — both common
// in modern Go.
func receiverTypeName(field *ast.Field) string {
	switch t := field.Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
		if idx, ok := t.X.(*ast.IndexExpr); ok {
			if id, ok := idx.X.(*ast.Ident); ok {
				return id.Name
			}
		}
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// sliceText returns the source bytes between start and end positions as
// a string. We use the *token.FileSet to translate to absolute offsets
// because token.Pos is opaque without it.
func sliceText(src []byte, start, end token.Pos, fset *token.FileSet) string {
	a := fset.Position(start).Offset
	b := fset.Position(end).Offset
	if a < 0 || b > len(src) || a > b {
		return ""
	}
	return string(src[a:b])
}
