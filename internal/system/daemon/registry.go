package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

// defaultPortBase is the first port auto-assigned to registry instances that do
// not pin one, chosen to sit above the common single-server default (8080).
const defaultPortBase = 8801

// RegistryEntry declares one fused-server instance to supervise. An entry names
// its dataset (a config is generated from it) or points at an explicit config.
type RegistryEntry struct {
	Name    string `yaml:"name"`
	Dataset string `yaml:"dataset"` // dataset dir; a config is generated from it when Config is empty
	Config  string `yaml:"config"`  // explicit config path; overrides dataset-based generation
	Port    int    `yaml:"port"`    // fixed port; 0 = auto-assign from the registry port base
}

// Registry is a set of instances plus shared defaults, loaded from instances.yaml.
// It lets one `daemon up` bring up several datasets as separate MCP servers,
// each on its own port, so one host can serve multiple projects to agents.
type Registry struct {
	PortBase     int             `yaml:"port_base"`     // first auto-assigned port (default 8801)
	Bind         string          `yaml:"bind"`          // bind host (default 127.0.0.1; 0.0.0.0 to reach remote agents)
	AllowRemote  bool            `yaml:"allow_remote"`  // opt-in propagated to generated configs
	OllamaURL    string          `yaml:"ollama_url"`    // embedding endpoint for generated configs
	GraphBinary  string          `yaml:"graph_binary"`  // ckg binary written into generated configs (enables ops.reindex/ops.index)
	VectorBinary string          `yaml:"vector_binary"` // ckv binary written into generated configs
	Instances    []RegistryEntry `yaml:"instances"`
}

// LoadRegistry reads and validates a registry YAML file.
func LoadRegistry(path string) (*Registry, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("registry: read %s: %w", path, err)
	}
	var r Registry
	if err := yaml.Unmarshal(buf, &r); err != nil {
		return nil, fmt.Errorf("registry: parse %s: %w", path, err)
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *Registry) validate() error {
	if len(r.Instances) == 0 {
		return fmt.Errorf("registry: no instances declared")
	}
	seen := map[string]bool{}
	for _, e := range r.Instances {
		if e.Name == "" {
			return fmt.Errorf("registry: an instance is missing a name")
		}
		if seen[e.Name] {
			return fmt.Errorf("registry: duplicate instance name %q", e.Name)
		}
		seen[e.Name] = true
		if e.Dataset == "" && e.Config == "" {
			return fmt.Errorf("registry: instance %q needs a dataset or config", e.Name)
		}
	}
	return nil
}

func (r *Registry) bindHost() string {
	if r.Bind == "" {
		return "127.0.0.1"
	}
	return r.Bind
}

// PortFree reports whether a TCP port can currently be bound on loopback.
func PortFree(port int) bool {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// PickFreePort returns the first port >= base (within a 1000-port window) for
// which free reports true. free is injectable so tests need not open sockets.
func PickFreePort(base int, free func(int) bool) (int, error) {
	if base <= 0 {
		base = defaultPortBase
	}
	for p := base; p < base+1000; p++ {
		if free(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port in [%d,%d)", base, base+1000)
}

// resolveAddrs assigns each instance a bind address: fixed ports are honored and
// reserved first, the rest are auto-assigned from the port base without
// collisions. free stubs port probing for tests.
func (r *Registry) resolveAddrs(free func(int) bool) (map[string]string, error) {
	base := r.PortBase
	if base <= 0 {
		base = defaultPortBase
	}
	used := map[int]bool{}
	for _, e := range r.Instances {
		if e.Port != 0 {
			used[e.Port] = true
		}
	}
	addrs := make(map[string]string, len(r.Instances))
	for _, e := range r.Instances {
		port := e.Port
		if port == 0 {
			p, err := PickFreePort(base, func(p int) bool { return !used[p] && free(p) })
			if err != nil {
				return nil, fmt.Errorf("instance %q: %w", e.Name, err)
			}
			port = p
			used[port] = true
		}
		addrs[e.Name] = net.JoinHostPort(r.bindHost(), strconv.Itoa(port))
	}
	return addrs, nil
}

// configFor returns the config path for an instance, generating one from its
// dataset into RunDir/<name>.yaml when no explicit config is given.
func (s *Supervisor) configFor(r *Registry, e RegistryEntry, addr string) (string, error) {
	if e.Config != "" {
		return e.Config, nil
	}
	cfg := config.Generate(config.GenerateOptions{
		Name:         e.Name,
		DatasetDir:   e.Dataset,
		HTTPAddr:     addr, // a non-loopback bind derives AllowRemote in Generate
		AllowRemote:  r.AllowRemote,
		OllamaURL:    r.OllamaURL,
		GraphBinary:  r.GraphBinary, // so ops.reindex/ops.index can run the engines
		VectorBinary: r.VectorBinary,
	})
	if err := os.MkdirAll(s.RunDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(s.RunDir, e.Name+".yaml")
	if err := config.Save(path, cfg); err != nil {
		return "", fmt.Errorf("instance %q: write config: %w", e.Name, err)
	}
	return path, nil
}

// Started is an instance brought up by Up together with the bind address
// assigned to it, so the caller can print a reachable connection URL.
type Started struct {
	Instance
	Addr string
}

// Up starts every registry instance, assigning ports and generating configs as
// needed. It is idempotent: instances already running are left in place (Start
// short-circuits on a live pidfile). free probes port availability (nil →
// PortFree). On the first failure it returns the instances started so far.
func (s *Supervisor) Up(r *Registry, free func(int) bool) ([]Started, error) {
	if free == nil {
		free = PortFree
	}
	addrs, err := r.resolveAddrs(free)
	if err != nil {
		return nil, err
	}
	out := make([]Started, 0, len(r.Instances))
	for _, e := range r.Instances {
		cfg, err := s.configFor(r, e, addrs[e.Name])
		if err != nil {
			return out, err
		}
		inst, err := s.Start(e.Name, cfg, addrs[e.Name])
		if err != nil {
			return out, err
		}
		out = append(out, Started{Instance: inst, Addr: addrs[e.Name]})
	}
	return out, nil
}

// Down stops every registry instance, continuing past individual failures and
// returning the first error encountered.
func (s *Supervisor) Down(r *Registry) error {
	var first error
	for _, e := range r.Instances {
		if err := s.Stop(e.Name); err != nil && first == nil {
			first = err
		}
	}
	return first
}
