package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadScope: a scope is the whole file list of the build: listed files
// of every language pass, and nothing else does, other languages included.
func TestLoadScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scope.json")
	body := `{"schema_version":2,"config_id":"app-linux","root":"/x","target":{"goos":"linux","goarch":"amd64","cgo_enabled":"1","tags":["netgo"]},
"files":[{"path":"lib/lib.go","language":"go","role":"production","package":"m/lib"},
{"path":"lib/lib_test.go","language":"go","role":"test","package":"m/lib"},
{"path":"lib/x_amd64.s","language":"asm","role":"production","package":"m/lib"},
{"path":"contracts/Token.sol","language":"sol","role":"production"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, b, fl, err := loadScope(path)
	if err != nil {
		t.Fatalf("loadScope: %v", err)
	}
	if s.ConfigID != "app-linux" || b.GOOS != "linux" || b.GOARCH != "amd64" || b.CGOEnabled != "1" || len(b.Tags) != 1 {
		t.Errorf("config = %+v, build = %+v", s, b)
	}
	for _, p := range []string{"lib/lib.go", "lib/lib_test.go", "contracts/Token.sol"} {
		if !fl.Allow(p) {
			t.Errorf("filter rejects %s", p)
		}
	}
	for _, p := range []string{"lib/other.go", "unused/unused.go", "web/app.ts", "contracts/Other.sol"} {
		if fl.Allow(p) {
			t.Errorf("filter allows %s, which the scope does not list", p)
		}
	}
}

func TestLoadScope_RejectsOtherSchemaAndEmpty(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"schema": `{"schema_version":1,"files":[{"path":"a.go","language":"go"}]}`,
		"empty":  `{"schema_version":2,"files":[]}`,
	} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := loadScope(path); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestBuildLogFile(t *testing.T) {
	out := t.TempDir()
	p, err := buildLogFile("", out)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(p) != filepath.Join(out, "logs") || filepath.Ext(p) != ".jsonl" {
		t.Errorf("default build log = %s, want <out>/logs/build-*.jsonl", p)
	}
	if got, _ := buildLogFile("/tmp/x.jsonl", out); got != "/tmp/x.jsonl" {
		t.Errorf("explicit --log-file not kept: %s", got)
	}
	if got := serverLogFile("/tmp/y.jsonl", "mcp"); got != "/tmp/y.jsonl" {
		t.Errorf("explicit server log not kept: %s", got)
	}
}
