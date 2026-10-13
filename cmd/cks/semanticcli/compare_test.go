package semanticcli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestCompareResponsesCommandRejectsBeforeReporting(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "off.json")
	b := filepath.Join(dir, "on.json")
	pack := contract.EvidencePackV2{FormatVersion: 2, Coordinates: contract.V2Coordinates{ProjectID: "fixture", DatasetID: strings.Repeat("a", 64), SnapshotID: strings.Repeat("b", 64), SourceMode: "snapshot-only"}, Query: "feature", EvidenceState: "complete", Metadata: contract.V2Metadata{IntegrityHashAlgo: "sha256-v2"}}
	if err := evidencev2.Stamp(&pack); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(pack)
	if err := os.WriteFile(a, data, 0600); err != nil {
		t.Fatal(err)
	}
	pack.Metadata.Ontology = &contract.OntologyDiagnostic{Mode: "relations", State: "no_match"}
	if err := evidencev2.Stamp(&pack); err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(pack)
	if err := os.WriteFile(b, data, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"compare-responses", "--off", a, "--on", b})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report evidencev2.ResponseComparison
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.OnState != "no_match" || report.QualityEvaluated {
		t.Fatal(report, err)
	}
	if err := os.WriteFile(b, append(data, []byte(" {}")...), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"compare-responses", "--off", a, "--on", b})
	if err := cmd.Execute(); err == nil || strings.Contains(out.String(), `"scope"`) {
		t.Fatal("trailing JSON accepted or report released")
	}
	f, err := os.Create(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate((4 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err := readComparisonPack(b); err == nil {
		t.Fatal("oversized input accepted")
	}
}
