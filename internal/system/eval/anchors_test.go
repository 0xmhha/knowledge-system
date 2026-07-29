package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestVerifyAnchors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := "package x\n\n// Register wires the tools.\nfunc Register() {}\n\nfunc other() {}\n"
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Scenario{
		Name: "t",
		ExpectedCitations: []contract.Citation{
			{File: "pkg/a.go", StartLine: 3, EndLine: 4}, // contains anchor
			{File: "pkg/a.go", StartLine: 6, EndLine: 6}, // drifted: anchor NOT here
			{File: "pkg/a.go", StartLine: 1, EndLine: 2}, // no anchor declared
			{File: "pkg/missing.go", StartLine: 1, EndLine: 2},
		},
		Anchors: []string{"func Register(", "func Register(", "", "anything"},
	}
	errs := s.VerifyAnchors(root)
	if len(errs) != 2 {
		t.Fatalf("violations = %d (%v), want 2 (drifted span + missing file)", len(errs), errs)
	}
}

func TestParseScenario_AnchorField(t *testing.T) {
	t.Parallel()
	y := []byte(`version: 1
name: n
prompt: "p"
expected_citations:
  - file: a.go
    start_line: 1
    end_line: 2
    anchor: "func A("
  - file: b.go
    start_line: 3
    end_line: 4
`)
	s, err := ParseScenario(y)
	if err != nil {
		t.Fatalf("ParseScenario: %v", err)
	}
	if len(s.Anchors) != 2 || s.Anchors[0] != "func A(" || s.Anchors[1] != "" {
		t.Errorf("Anchors = %v, want [\"func A(\" \"\"]", s.Anchors)
	}
}
