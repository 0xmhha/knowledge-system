package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(path, []byte(`
port_base: 9100
bind: 0.0.0.0
ollama_url: http://127.0.0.1:11434
instances:
  - name: alpha
    dataset: /data/alpha
  - name: beta
    config: /etc/beta.yaml
    port: 9200
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r.PortBase != 9100 || r.Bind != "0.0.0.0" || len(r.Instances) != 2 {
		t.Fatalf("parsed registry mismatch: %+v", r)
	}
	if r.Instances[1].Port != 9200 || r.Instances[1].Config != "/etc/beta.yaml" {
		t.Errorf("beta entry mismatch: %+v", r.Instances[1])
	}
}

func TestLoadRegistryEngineBinaries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(path, []byte(`
graph_binary: /opt/ckg
vector_binary: /opt/ckv
instances:
  - name: a
    dataset: /data/a
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r.GraphBinary != "/opt/ckg" || r.VectorBinary != "/opt/ckv" {
		t.Errorf("engine binaries not parsed: %+v", r)
	}
}

func TestLoadRegistryRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"no instances": `port_base: 9100`,
		"missing name": "instances:\n  - dataset: /data/x\n",
		"no source":    "instances:\n  - name: x\n",
		"duplicate":    "instances:\n  - name: x\n    dataset: /a\n  - name: x\n    dataset: /b\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".yaml")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadRegistry(path); err == nil {
				t.Errorf("expected error for %q", name)
			}
		})
	}
}

func TestPickFreePort(t *testing.T) {
	// first two ports busy → picks base+2.
	busy := map[int]bool{9100: true, 9101: true}
	p, err := PickFreePort(9100, func(port int) bool { return !busy[port] })
	if err != nil || p != 9102 {
		t.Errorf("PickFreePort = %d, %v; want 9102", p, err)
	}
	// nothing free → error.
	if _, err := PickFreePort(9100, func(int) bool { return false }); err == nil {
		t.Error("expected error when no port is free")
	}
}

func TestResolveAddrs(t *testing.T) {
	r := &Registry{
		PortBase: 9100,
		Bind:     "0.0.0.0",
		Instances: []RegistryEntry{
			{Name: "fixed", Dataset: "/a", Port: 9100}, // pinned; auto must skip it
			{Name: "auto1", Dataset: "/b"},
			{Name: "auto2", Dataset: "/c"},
		},
	}
	addrs, err := r.resolveAddrs(func(int) bool { return true }) // all ports "free"
	if err != nil {
		t.Fatalf("resolveAddrs: %v", err)
	}
	want := map[string]string{
		"fixed": "0.0.0.0:9100",
		"auto1": "0.0.0.0:9101", // skips the reserved 9100
		"auto2": "0.0.0.0:9102",
	}
	for name, w := range want {
		if addrs[name] != w {
			t.Errorf("addr[%s] = %q, want %q", name, addrs[name], w)
		}
	}
}

func TestUpWritesEngineBinariesIntoGeneratedConfig(t *testing.T) {
	s := newTestSupervisor(t)
	r := &Registry{
		PortBase:     9500,
		GraphBinary:  "/opt/ckg",
		VectorBinary: "/opt/ckv",
		Instances:    []RegistryEntry{{Name: "eng", Dataset: "/data/eng"}},
	}
	t.Cleanup(func() { _ = s.Down(r) })
	if _, err := s.Up(r, func(int) bool { return true }); err != nil {
		t.Fatalf("up: %v", err)
	}
	// The generated config must carry the engine binary paths so ops.reindex /
	// ops.index can run without ckg/ckv being on PATH.
	buf, err := os.ReadFile(filepath.Join(s.RunDir, "eng.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/opt/ckg", "/opt/ckv"} {
		if !bytes.Contains(buf, []byte(want)) {
			t.Errorf("generated config missing binary path %q:\n%s", want, buf)
		}
	}
}

func TestUpDownWithGeneratedConfig(t *testing.T) {
	s := newTestSupervisor(t)
	r := &Registry{
		PortBase: 9300,
		Instances: []RegistryEntry{
			{Name: "gen-a", Dataset: "/data/a"},
			{Name: "gen-b", Dataset: "/data/b"},
		},
	}
	t.Cleanup(func() { _ = s.Down(r) })

	insts, err := s.Up(r, func(int) bool { return true })
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if len(insts) != 2 || !insts[0].Running || !insts[1].Running {
		t.Fatalf("up produced %+v", insts)
	}
	if insts[0].Addr == "" || insts[1].Addr == "" {
		t.Errorf("up should report a bind addr per instance: %+v", insts)
	}
	// a config was generated per instance from its dataset.
	for _, name := range []string{"gen-a", "gen-b"} {
		cfgPath := filepath.Join(s.RunDir, name+".yaml")
		if _, err := os.Stat(cfgPath); err != nil {
			t.Errorf("expected generated config %s: %v", cfgPath, err)
		}
	}
	if err := s.Down(r); err != nil {
		t.Fatalf("down: %v", err)
	}
	if st := s.Status("gen-a"); st.Running {
		t.Errorf("gen-a still running after down")
	}
}
