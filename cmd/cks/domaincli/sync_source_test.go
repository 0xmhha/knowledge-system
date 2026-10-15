package domaincli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/inventory"
)

func TestSourcePolicyDeclarationsAndRefusals(t *testing.T) {
	root := t.TempDir()
	file := "folder/actual.go"
	if err := os.MkdirAll(filepath.Join(root, "folder"), 0755); err != nil {
		t.Fatal(err)
	}
	source := `package actual
const Version = "1"
var ErrFee = 1
type Box[T any] struct{ currentMutex int }
func Compile(){}
func (*Box[T]) Export(){}
type Other struct{}
func (*Other) Export(){}
`
	if err := os.WriteFile(filepath.Join(root, file), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, symbol, want string
		fail               bool
	}{
		{"package differs from directory", "Compile", "actual.Compile", false},
		{"explicit package", "actual.Compile", "actual.Compile", false},
		{"generic receiver", "Box.Export", "actual.Box.Export", false},
		{"explicit receiver and package", "actual.Box.Export", "actual.Box.Export", false},
		{"field", "Box.currentMutex", "actual.Box.currentMutex", false},
		{"constant", "Version", "actual.Version", false},
		{"variable", "ErrFee", "actual.ErrFee", false},
		{"type", "Box", "actual.Box", false},
		{"stale package prefix", "folder.Compile", "", true},
		{"missing receiver ambiguous", "Export", "", true},
		{"wrong file declaration", "Missing", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := []inventory.Entry{{ID: "p", CodeAnchors: []inventory.CodeAnchor{{File: file, Symbol: tc.symbol}}}}
			_, out := deriveViews(entries)
			err := resolveSourcePolicies(entries, &out, root)
			if tc.fail {
				if err == nil {
					t.Fatal("expected refusal")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Policies[0].Governs) != 1 || out.Policies[0].Governs[0] != tc.want {
				t.Fatalf("got %+v", out)
			}
		})
	}
	t.Run("unique bare receiver", func(t *testing.T) {
		entries := []inventory.Entry{{ID: "p", CodeAnchors: []inventory.CodeAnchor{{File: file, Symbol: "Compile"}}}}
		other := filepath.Join(root, "folder/one.go")
		if err := os.WriteFile(other, []byte("package actual\ntype Only struct{}\nfunc (*Only) Queue(){}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		entries[0].CodeAnchors[0] = inventory.CodeAnchor{File: "folder/one.go", Symbol: "Queue"}
		_, out := deriveViews(entries)
		if err := resolveSourcePolicies(entries, &out, root); err != nil {
			t.Fatal(err)
		}
		if out.Policies[0].Governs[0] != "actual.Only.Queue" {
			t.Fatal(out)
		}
	})
	for _, tc := range []struct{ name, path string }{
		{"parent escape", "../actual.go"}, {"absolute", filepath.Join(root, file)}, {"unclean", "folder/../folder/actual.go"}, {"non Go", "folder/doc.md"}, {"missing", "folder/missing.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := readPolicySource(root, tc.path); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		if err := os.Symlink("actual.go", filepath.Join(root, "folder/alias.go")); err != nil {
			t.Fatal(err)
		}
		if _, err := readPolicySource(root, "folder/alias.go"); err == nil {
			t.Fatal("symlink accepted")
		}
	})
	t.Run("invalid source", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(root, "folder/bad.go"), []byte("package actual\nfunc broken("), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readPolicySource(root, "folder/bad.go"); err == nil {
			t.Fatal("invalid syntax accepted")
		}
	})
	for _, tc := range []struct {
		name   string
		anchor inventory.CodeAnchor
		fail   bool
	}{
		{"loc enclosing", inventory.CodeAnchor{File: file, Kind: "loc", Symbol: "Compile", EnclosingSymbol: "Compile", Line: 5}, false},
		{"loc outside", inventory.CodeAnchor{File: file, Kind: "loc", Symbol: "Compile", EnclosingSymbol: "Compile", Line: 2}, true},
		{"loc missing enclosing", inventory.CodeAnchor{File: file, Kind: "loc", Symbol: "Compile", Line: 5}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := []inventory.Entry{{ID: "p", CodeAnchors: []inventory.CodeAnchor{tc.anchor}}}
			_, out := deriveViews(entries)
			err := resolveSourcePolicies(entries, &out, root)
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSourceSyncFailurePreservesBothOutputs(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	code := filepath.Join(root, "code")
	for _, p := range []string{filepath.Join(project, "entries"), code} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string]string{
		"project.yaml":    "id: p\nname: test\nschema_version: 1\n",
		"subsystems.yaml": "- id: core\n  name: Core\n",
		"entries/a.yaml":  "id: p\ntitle: Test\nsubsystem: core\nstatus: verified\ncode_anchors:\n  - file: source.go\n    symbol: Missing\n",
	} {
		if err := os.WriteFile(filepath.Join(project, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(code, "source.go"), []byte("package right\nfunc Existing(){}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ckv, ckg := filepath.Join(root, "ckv.yaml"), filepath.Join(root, "ckg.yaml")
	for _, p := range []string{ckv, ckg} {
		if err := os.WriteFile(p, []byte("preserved"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := runSourceSync(project, ckv, ckg, code)
	if err == nil || !strings.Contains(err.Error(), "expected unique declaration") {
		t.Fatalf("wrong refusal: %v", err)
	}
	for _, p := range []string{ckv, ckg} {
		data, err := os.ReadFile(p)
		if err != nil || string(data) != "preserved" {
			t.Fatalf("output changed: %s %v", data, err)
		}
	}
}
