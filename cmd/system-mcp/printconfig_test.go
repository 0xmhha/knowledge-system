package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

func writeCfg(t *testing.T, name string, o config.GenerateOptions) string {
	t.Helper()
	cfg := config.Generate(o)
	path := filepath.Join(t.TempDir(), name+".yaml")
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	return path
}

func runPrint(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if err := runPrintMCPConfig(args, &buf); err != nil {
		t.Fatalf("runPrintMCPConfig: %v", err)
	}
	var out struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode output %q: %v", buf.String(), err)
	}
	if len(out.MCPServers) != 1 {
		t.Fatalf("expected one server entry, got %v", out.MCPServers)
	}
	for _, entry := range out.MCPServers {
		return entry
	}
	return nil
}

func TestPrintMCPConfig_HTTP(t *testing.T) {
	path := writeCfg(t, "h", config.GenerateOptions{
		Name: "proj", DatasetDir: "/d", HTTPAddr: "127.0.0.1:8899",
	})
	entry := runPrint(t, "--config", path)
	if entry["type"] != "http" {
		t.Errorf("type = %v, want http", entry["type"])
	}
	url, _ := entry["url"].(string)
	if !strings.HasPrefix(url, "http://") || !strings.HasSuffix(url, ":8899/mcp") {
		t.Errorf("url = %q, want http://<host>:8899/mcp", url)
	}
}

func TestPrintMCPConfig_Stdio(t *testing.T) {
	// Generate an http config, then flip transport to stdio in the file.
	path := writeCfg(t, "s", config.GenerateOptions{Name: "proj2", DatasetDir: "/d"})
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Listen.Transport = "stdio"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	entry := runPrint(t, "--config", path)
	if entry["command"] == nil {
		t.Errorf("stdio entry should carry a command, got %v", entry)
	}
	args, _ := entry["args"].([]any)
	if len(args) != 2 || args[0] != "--config" {
		t.Errorf("args = %v, want [--config <abs>]", args)
	}
}

func TestPrintMCPConfig_NameOverride(t *testing.T) {
	path := writeCfg(t, "n", config.GenerateOptions{Name: "proj", DatasetDir: "/d", HTTPAddr: "127.0.0.1:9000"})
	var buf bytes.Buffer
	if err := runPrintMCPConfig([]string{"--config", path, "--name", "custom-key"}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"custom-key"`) {
		t.Errorf("output should key on the overridden name; got %s", buf.String())
	}
}

func TestPrintMCPConfig_RequiresConfig(t *testing.T) {
	var buf bytes.Buffer
	if err := runPrintMCPConfig(nil, &buf); err == nil {
		t.Error("expected an error when --config is omitted")
	}
}
