package setupcli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

func TestGCCLIDefaultDryRunAndReviewedApply(t *testing.T) {
	dataset := t.TempDir()
	path := filepath.Join(t.TempDir(), "plan.json")
	c := NewGCCmd()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--out", dataset, "--plan-file", path})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	var plan setup.GCPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil || plan.Digest == "" || plan.Policy.KeepRecent != 2 {
		t.Fatal("default is not a reviewable dry-run", err)
	}
	c = NewGCCmd()
	out.Reset()
	c.SetOut(&out)
	c.SetArgs([]string{"--out", dataset, "--apply", "--plan-file", path})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["status"] != "applied" {
		t.Fatal("missing apply result", err)
	}
	for _, args := range [][]string{{"--out", dataset, "--apply"}, {"--out", dataset, "--apply", "--plan-file", path, "--min-age", "0s"}, {"--out", dataset, "--apply", "--resume"}, {"--out", dataset, "--dry-run=false"}, {"--out", dataset, "--keep-recent", "1"}} {
		c = NewGCCmd()
		c.SilenceUsage = true
		c.SilenceErrors = true
		c.SetArgs(args)
		if err := c.Execute(); err == nil {
			t.Fatal("invalid GC mode accepted", args)
		}
	}
}
