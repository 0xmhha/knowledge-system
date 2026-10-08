package domaincli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/inventory"
	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
)

func TestWorksheetWithoutPackHasNoOrganizationCatalog(t *testing.T) {
	p := &inventory.Project{ID: "generic-example", Dir: t.TempDir(), Subsystems: map[string]inventory.Subsystem{}}
	var out bytes.Buffer
	renderHeader(&out, p, "needs_verification", "", 1)
	renderEntry(&out, p, inventory.Entry{ID: "example", Title: "Concurrency lock review", Status: "needs_verification", Invariants: []string{"Hold the lock."}})
	renderFooter(&out, p)
	for _, privateDomain := range []string{"07 §9", "StableNet", "coding-agent/docs/r1-refactor/", "stake-weighted", "Core.current"} {
		if strings.Contains(out.String(), privateDomain) {
			t.Fatalf("unselected organization catalog leaked: %s", privateDomain)
		}
	}
	if !strings.Contains(out.String(), "APPROVE") || !strings.Contains(out.String(), "REVISE") {
		t.Fatal("lost review controls")
	}
}

func TestWorksheetSelectedLegacyCatalogAllEightMappings(t *testing.T) {
	root, err := filepath.Abs("../../../projects/stablenet/domain-knowledge")
	if err != nil {
		t.Fatal(err)
	}
	p, err := inventory.LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	selected := p.WorksheetPacks[0]
	catalogs, err := knowledgepack.LoadWorksheetCatalogs(root, []knowledgepack.Selection{{PackID: selected.PackID, Version: selected.Version, Source: selected.Source, SHA256: selected.SHA256}})
	if err != nil || len(catalogs) != 1 || len(catalogs[0].Items) != 8 {
		t.Fatalf("legacy selection: %v", err)
	}
	for _, risk := range catalogs[0].Items {
		e := inventory.Entry{Title: strings.Join(risk.Keywords, " ")}
		mapped, _ := mapToCatalog(e, catalogs)
		if !strings.HasPrefix(mapped, "**"+risk.MappingLabel+"** ("+risk.Label+")") || !strings.Contains(mapped, risk.Source.ContentSHA256) {
			t.Fatalf("lost mapping %s: %s", risk.ID, mapped)
		}
	}
	var output bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--project", root, "worksheet", "--status", "no-such-status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Entries in queue**: 0") || !strings.Contains(output.String(), selected.SHA256) || strings.Contains(output.String(), "### `A") {
		t.Fatal("empty queue or selected metadata broken")
	}
}

func TestWorksheetCommandFiltersGenericAndPreservesOutputOnBadPin(t *testing.T) {
	root := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "entries"), 0700); err != nil {
		t.Fatal(err)
	}
	write("project.yaml", "id: example\nschema_version: 1\n")
	write("subsystems.yaml", "- id: core\n  name: Core\n")
	write("entries/b.yaml", "id: b\nsubsystem: core\nstatus: needs_verification\npriority: P1\ntitle: Lock review\n")
	write("entries/a.yaml", "id: a\nsubsystem: core\nstatus: needs_verification\npriority: P0\ntitle: Lock review\ncode_anchors:\n  - file: core.go\n    line: 7\n    symbol: Hold\n    reason: hold lock\ninvariants:\n  - Hold lock\n")
	write("entries/c.yaml", "id: c\nsubsystem: core\nstatus: verified\npriority: P0\ntitle: Already reviewed\n")
	execute := func(args ...string) (string, error) {
		cmd := NewCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"--project", root, "worksheet"}, args...))
		err := cmd.Execute()
		return out.String(), err
	}
	output, err := execute("--priority", "P0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "### `a`") || strings.Contains(output, "### `b`") || strings.Contains(output, "### `c`") || !strings.Contains(output, "core.go:7") || !strings.Contains(output, "suggested: core.go:7") || strings.Contains(output, "07 §9") || strings.Contains(output, "already file+line") {
		t.Fatal("filter, anchor or generic review regression")
	}
	if !strings.Contains(output, "./bin/cks domain verify --project") || strings.Contains(output, "./cmd/cks-entry-verify") {
		t.Fatal("obsolete command printed")
	}
	output, err = execute()
	if err != nil || strings.Index(output, "### `a`") >= strings.Index(output, "### `b`") {
		t.Fatal("stable ID order lost")
	}
	sentinel := filepath.Join(root, "review.md")
	write("review.md", "keep previous review\n")
	write("project.yaml", "id: example\nworksheet_packs:\n  - pack_id: organization.missing\n    version: 1.0.0\n    source: packs/missing\n    sha256: "+strings.Repeat("a", 64)+"\n")
	if _, err = execute("--out", sentinel); err == nil {
		t.Fatal("missing selected pack accepted")
	}
	saved, _ := os.ReadFile(sentinel)
	if string(saved) != "keep previous review\n" {
		t.Fatal("bad pin overwrote prior review")
	}
}

func TestWorksheetShellQuotePreservesLiteralPath(t *testing.T) {
	value := "project's directory $(touch forbidden) `touch forbidden`"
	cmd := exec.Command("/bin/sh", "-c", "printf '%s' "+worksheetShellQuote(value))
	cmd.Dir = t.TempDir()
	output, err := cmd.Output()
	if err != nil || string(output) != value {
		t.Fatalf("path not literal: %q %v", output, err)
	}
	if _, err := os.Stat(filepath.Join(cmd.Dir, "forbidden")); !os.IsNotExist(err) {
		t.Fatal("path executed shell expansion")
	}
}
