// Package observe checks the runtime links of a Go graph against what a
// test run actually does (design decision D3: traces are the precision
// check of the static analysis, not its source).
//
// Instrument copies a source tree and inserts, in every in-scope Go file,
//   - at the start of each function body and function literal, a call that
//     records the caller's function and call line and the callee;
//   - before each statement that sends on, receives from or ranges over a
//     channel named by a plain identifier or selector, a call that records
//     the channel's pointer, the operation, the function and the line.
//
// Edits are inserted on the line they belong to, so every line number of
// the copy equals the original's and the records can be compared with the
// graph. The runtime package the calls go to is written into the copy
// (<module>/ckgobs); records go to $CKGOBS_DIR/obs-<pid>.jsonl, one line per
// distinct call site and callee, or channel operation site and channel.
// Compare reads them back against the graph.
package observe

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ImportName is the name the instrumented files import the runtime package as.
const ImportName = "ckgobs__"

// RuntimeDir is the runtime package's directory under the module root.
const RuntimeDir = "ckgobs"

// InstrumentStats summarises one instrumentation.
type InstrumentStats struct {
	Files          int `json:"files"`           // files rewritten
	SkippedCgo     int `json:"skipped_cgo"`     // files that import "C", left as they are
	Functions      int `json:"functions"`       // function bodies and literals instrumented
	SkippedNoSplit int `json:"skipped_nosplit"` // //go:nosplit functions left as they are
	ChannelOps     int `json:"channel_ops"`     // channel operations instrumented
	SkippedOps     int `json:"skipped_ops"`     // channel operations whose channel expression could not be evaluated early
}

// Instrument copies src to out (the .git directory left out) and rewrites
// the listed files (slash paths relative to src). modulePath is the module
// path of src's go.mod.
func Instrument(src, out, modulePath string, files []string) (InstrumentStats, error) {
	var st InstrumentStats
	if err := copyTree(src, out); err != nil {
		return st, err
	}
	sort.Strings(files)
	imp := modulePath + "/" + RuntimeDir
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		path := filepath.Join(out, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			return st, fmt.Errorf("observe: read %s: %w", rel, err)
		}
		nb, ok, err := instrumentFile(path, b, imp, &st)
		if err != nil {
			return st, fmt.Errorf("observe: %s: %w", rel, err)
		}
		if !ok {
			continue
		}
		if err := os.WriteFile(path, nb, 0o644); err != nil {
			return st, err
		}
		st.Files++
	}
	dir := filepath.Join(out, RuntimeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return st, err
	}
	if err := os.WriteFile(filepath.Join(dir, "ckgobs.go"), []byte(runtimeSource), 0o644); err != nil {
		return st, err
	}
	return st, nil
}

type edit struct {
	off  int
	text string
}

// instrumentFile returns the rewritten source, or false when the file is
// left as it is.
func instrumentFile(path string, src []byte, imp string, st *InstrumentStats) ([]byte, bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, false, err
	}
	for _, is := range f.Imports {
		if is.Path.Value == `"C"` {
			st.SkippedCgo++
			return nil, false, nil
		}
	}
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	var edits []edit
	enter := func(body *ast.BlockStmt) {
		if body == nil {
			return
		}
		edits = append(edits, edit{off(body.Lbrace) + 1, ImportName + ".Enter();"})
		st.Functions++
	}
	// before marks, per statement start, what to record ahead of it.
	before := map[int][]string{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		if noSplit(fd) {
			st.SkippedNoSplit++
			continue
		}
		enter(fd.Body)
		chanOps(fd.Body, fset, before, st)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok {
				enter(lit.Body)
			}
			return true
		})
	}
	if len(edits) == 0 {
		return nil, false, nil
	}
	for o, calls := range before {
		edits = append(edits, edit{o, strings.Join(calls, "")})
	}
	// The import goes on the package clause's line: "package p; import ...".
	edits = append(edits, edit{off(f.Name.End()), "; import " + ImportName + " " + strconv.Quote(imp)})
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].off > edits[j].off })
	out := append([]byte{}, src...)
	for _, e := range edits {
		out = append(out[:e.off], append([]byte(e.text), out[e.off:]...)...)
	}
	return out, true, nil
}

func noSplit(fd *ast.FuncDecl) bool {
	if fd.Doc == nil {
		return false
	}
	for _, c := range fd.Doc.List {
		if strings.HasPrefix(c.Text, "//go:nosplit") || strings.HasPrefix(c.Text, "//go:nowritebarrier") {
			return true
		}
	}
	return false
}

// chanOps finds the channel operations of a body and, for each, the
// statement in a statement list it belongs to; the record call goes before
// that statement, on its line.
func chanOps(body *ast.BlockStmt, fset *token.FileSet, before map[int][]string, st *InstrumentStats) {
	var stack []ast.Node
	record := func(ch ast.Expr, op int, at token.Pos) {
		host := listStmt(stack)
		if host == nil || !earlyEval(ch, host) {
			st.SkippedOps++
			return
		}
		o := fset.Position(host.Pos()).Offset
		// The operation's own line: a receive in a select case is recorded
		// before the select statement but belongs to its case line.
		before[o] = append(before[o], fmt.Sprintf("%s.Chan(%s, %d, %d);", ImportName, exprText(ch), op, fset.Position(at).Line))
		st.ChannelOps++
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		stack = append(stack, n)
		switch x := n.(type) {
		case *ast.SendStmt:
			record(x.Chan, 1, x.Arrow)
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				record(x.X, 2, x.OpPos)
			}
		case *ast.RangeStmt:
			record(x.X, 2, x.For) // the runtime call ignores a value that is not a channel
		}
		return true
	})
}

// listStmt is the innermost statement on the stack (ending with the
// operation) that sits directly in a statement list, where a call can be
// inserted before it. A select's communication clause is not such a place;
// the select statement is.
func listStmt(stack []ast.Node) ast.Stmt {
	for i := len(stack) - 1; i > 0; i-- {
		s, ok := stack[i].(ast.Stmt)
		if !ok {
			continue
		}
		switch s.(type) {
		case *ast.CaseClause, *ast.CommClause:
			continue // "case ...:" is no place for a statement
		}
		switch p := stack[i-1].(type) {
		case *ast.BlockStmt:
			return s
		case *ast.CaseClause:
			for _, b := range p.Body {
				if b == s {
					return s
				}
			}
		case *ast.CommClause:
			for _, b := range p.Body {
				if b == s {
					return s
				}
			}
		}
	}
	return nil
}

// earlyEval reports whether ch can be evaluated before host without
// changing the program: an identifier or a selector chain on one, not
// declared inside host.
func earlyEval(ch ast.Expr, host ast.Stmt) bool {
	for {
		switch x := ch.(type) {
		case *ast.ParenExpr:
			ch = x.X
			continue
		case *ast.SelectorExpr:
			ch = x.X
			continue
		case *ast.Ident:
			if x.Obj == nil || x.Obj.Decl == nil {
				return true // package level, or another file's
			}
			if d, ok := x.Obj.Decl.(ast.Node); ok && d.Pos() >= host.Pos() && d.Pos() < host.End() {
				return false
			}
			return true
		}
		return false
	}
}

func exprText(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprText(x.X) + "." + x.Sel.Name
	case *ast.ParenExpr:
		return "(" + exprText(x.X) + ")"
	}
	return ""
}

// copyTree copies src to dst, leaving .git out.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		info, err := in.Stat()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}

// runtimeSource is the package the instrumented code calls.
const runtimeSource = `// Package ckgobs records calls and channel operations for ckg verify
// --observed. Written by ckg observe; not part of the project.
package ckgobs

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"
)

var (
	mu   sync.Mutex
	out  *os.File
	seen sync.Map
	// keep holds every channel Chan has seen, so the garbage collector
	// cannot free one and give its address to a later channel: a pointer
	// then names one channel for the whole run.
	keep sync.Map
)

func write(format string, a ...any) {
	mu.Lock()
	defer mu.Unlock()
	if out == nil {
		dir := os.Getenv("CKGOBS_DIR")
		if dir == "" {
			return
		}
		f, err := os.OpenFile(fmt.Sprintf("%s/obs-%d.jsonl", dir, os.Getpid()), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		out = f
	}
	fmt.Fprintf(out, format+"\n", a...)
}

type callKey struct{ callee, site uintptr }

// Enter records, once per call site and callee, who called the function it
// is placed in.
func Enter() {
	var pcs [2]uintptr
	if runtime.Callers(2, pcs[:]) < 2 {
		return
	}
	if _, dup := seen.LoadOrStore(callKey{pcs[0], pcs[1]}, true); dup {
		return
	}
	fr := runtime.CallersFrames(pcs[:])
	callee, more := fr.Next()
	if !more {
		return
	}
	caller, _ := fr.Next()
	write("{\"k\":\"call\",\"callee\":%q,\"caller\":%q,\"file\":%q,\"line\":%d}", callee.Function, caller.Function, caller.File, caller.Line)
}

type chanKey struct {
	ch, site uintptr
	op       int
}

// Chan records, once per site, channel and operation (1 send, 2 receive),
// the channel an operation on line is on. A value that is not a non-nil
// channel is ignored.
func Chan(c any, op, line int) {
	v := reflect.ValueOf(c)
	if !v.IsValid() || v.Kind() != reflect.Chan || v.IsNil() {
		return
	}
	keep.LoadOrStore(v.Pointer(), c)
	var pcs [1]uintptr
	if runtime.Callers(2, pcs[:]) < 1 {
		return
	}
	if _, dup := seen.LoadOrStore(chanKey{v.Pointer(), pcs[0], op}, true); dup {
		return
	}
	f, _ := runtime.CallersFrames(pcs[:]).Next()
	write("{\"k\":\"chan\",\"ch\":\"%x\",\"op\":%d,\"fn\":%q,\"file\":%q,\"line\":%d}", v.Pointer(), op, f.Function, f.File, line)
}
`
