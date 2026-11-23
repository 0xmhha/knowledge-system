package golang_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/tools/go/packages"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
)

// dispatchTimeout bounds a pass that must end in well under a second.
const dispatchTimeout = 30 * time.Second

// TestDispatchEdges_BodyThatCannotBeBuilt checks that a package whose body the
// SSA builder panics on does not stop the pass. Package a imports a module
// that is not there, so a.Bal returns an invalid type. Package b calls a
// method on it; the type checker stays silent about an invalid operand, so b
// has no error of its own and its body is built. The panic leaves the generic
// instance both b and c use unfinished, and building c used to wait for it
// forever.
func TestDispatchEdges_BodyThatCannotBeBuilt(t *testing.T) {
	dir := t.TempDir()
	for name, src := range map[string]string{
		"go.mod":           "module example.com/m\n\ngo 1.22\n",
		"shared/shared.go": "package shared\n\nfunc ID[T any](x T) T { return x }\n",
		"a/a.go": "package a\n\nimport \"example.com/missing/u\"\n\n" +
			"func Bal() *u.Int { return nil }\n",
		"b/b.go": "package b\n\nimport (\n\t\"example.com/m/a\"\n\t\"example.com/m/shared\"\n)\n\n" +
			"func F() int {\n\tn := shared.ID(1)\n\t_ = a.Bal().ToBig()\n\treturn n\n}\n",
		"c/c.go": "package c\n\nimport \"example.com/m/shared\"\n\n" +
			"type I interface{ M() }\n\ntype T struct{}\n\nfunc (T) M() {}\n\n" +
			"func G() int {\n\tvar i I = T{}\n\ti.M()\n\treturn shared.ID(1)\n}\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.LoadAllSyntax,
		Dir:  dir,
		Env:  append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off"),
	}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		if p.PkgPath == "example.com/m/b" && len(p.Errors) != 0 {
			t.Fatalf("package b has errors of its own, the case is not reproduced: %v", p.Errors)
		}
	}

	done := make(chan gop.DispatchStats, 1)
	go func() {
		_, st := gop.EmitDispatchEdges(pkgs, nil)
		done <- st
	}()
	select {
	case st := <-done:
		if st.BuildPanics != 1 {
			t.Errorf("BuildPanics = %d, want 1 (package b)", st.BuildPanics)
		}
		if st.OwnErrors != 1 {
			t.Errorf("OwnErrors = %d, want 1 (package a)", st.OwnErrors)
		}
		if st.SSAPackages != 2 {
			t.Errorf("SSAPackages = %d, want 2 (shared and c)", st.SSAPackages)
		}
		if st.DynamicSites == 0 {
			t.Error("no dynamic call site resolved: package c was lost with package b")
		}
	case <-time.After(dispatchTimeout):
		t.Fatal("EmitDispatchEdges did not return")
	}
}
