//go:build ignore

// Command declspans prints the top-level declarations of one Go source file
// read from stdin as JSON lines, using only the standard parser. The
// retrieval evaluation takes its answer ranges from here rather than from the
// graph it measures.
//
// Usage: git show REV:path.go | go run scripts/retrieval_declspans.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
)

// Decl is one top-level declaration's 1-based inclusive line range.
type Decl struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Recv      string `json:"recv,omitempty"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Anchor    string `json:"anchor"`
}

const (
	kindFunc  = "func"
	kindType  = "type"
	kindValue = "value"
)

func main() {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "declspans: read:", err)
		os.Exit(1)
	}
	decls, err := parse(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "declspans:", err)
		os.Exit(1)
	}
	w := bufio.NewWriter(os.Stdout)
	enc := json.NewEncoder(w)
	for _, d := range decls {
		if err := enc.Encode(d); err != nil {
			fmt.Fprintln(os.Stderr, "declspans: write:", err)
			os.Exit(1)
		}
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "declspans: flush:", err)
		os.Exit(1)
	}
}

func parse(src []byte) ([]Decl, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(src), "\n")
	anchor := func(pos token.Pos) string {
		n := fset.Position(pos).Line
		return strings.TrimSpace(lines[n-1])
	}
	span := func(n ast.Node, doc *ast.CommentGroup) (int, int) {
		start := n.Pos()
		if doc != nil {
			start = doc.Pos()
		}
		return fset.Position(start).Line, fset.Position(n.End()).Line
	}
	var out []Decl
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			s, e := span(d, d.Doc)
			out = append(out, Decl{Kind: kindFunc, Name: d.Name.Name, Recv: recvName(d), StartLine: s, EndLine: e, Anchor: anchor(d.Pos())})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch sp := spec.(type) {
				case *ast.TypeSpec:
					doc := sp.Doc
					if len(d.Specs) == 1 {
						doc = d.Doc
					}
					s, e := span(sp, doc)
					if len(d.Specs) == 1 {
						e = fset.Position(d.End()).Line
					}
					out = append(out, Decl{Kind: kindType, Name: sp.Name.Name, StartLine: s, EndLine: e, Anchor: anchor(sp.Pos())})
				case *ast.ValueSpec:
					doc := sp.Doc
					if len(d.Specs) == 1 {
						doc = d.Doc
					}
					s, e := span(sp, doc)
					for _, name := range sp.Names {
						out = append(out, Decl{Kind: kindValue, Name: name.Name, StartLine: s, EndLine: e, Anchor: anchor(sp.Pos())})
					}
				}
			}
		}
	}
	return out, nil
}

// recvName returns the receiver's base type name, without pointer or type
// parameters.
func recvName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return ""
	}
	t := d.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
		case *ast.IndexExpr:
			t = x.X
		case *ast.IndexListExpr:
			t = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}
