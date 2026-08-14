package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	kmcp "github.com/0xmhha/knowledge-system/pkg/mcp"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// GenerateOptions captures the resolvable inputs a caller supplies to build a
// cks runtime config. It is the Go counterpart to system/scripts/gen-cks-config.sh:
// each field maps to one config knob, with empty fields taking sensible defaults
// in Generate. Paths are expected to be absolute (resolved by the caller); this
// package does not resolve the filesystem or detect the LAN IP.
type GenerateOptions struct {
	// Name / Description are the instance identity echoed by cks.ops.health so
	// callers can tell which dataset/index they reached. Empty Name defaults to
	// the deployment namespace root (KNOWLEDGE_MCP_NAMESPACE / ldflag, "cks"
	// upstream).
	Name        string
	Description string

	// DatasetDir is the consolidated knowledge-setup output directory. Generate
	// derives Backends.CKG.Path = <DatasetDir>/graph/graph.db and
	// Backends.CKV.Path = <DatasetDir>/vector from it.
	DatasetDir string

	// GraphPath / VectorPath override the DatasetDir derivation for datasets
	// that predate the knowledge-setup layout (e.g. the dataset-toolkit's
	// graph-db/ + vector-db/ dirs). GraphPath is the graph.db FILE; VectorPath
	// is the vector index DIRECTORY (holding vector.db + manifest.json). Empty
	// falls back to the DatasetDir-derived path.
	GraphPath  string
	VectorPath string

	// SourceRoot is the working tree the index was built against (ckg citation
	// resolution). GraphBinary / VectorBinary are the ckg/ckv binaries used by
	// the cks.ops.index maintenance tool. PolicyFile is the ckg governance
	// policy passed to `ckg build --policy-file`.
	SourceRoot   string
	GraphBinary  string
	VectorBinary string
	PolicyFile   string

	// EmbedModel is the Ollama model the ckv index was built with (empty →
	// "bge-m3"). OllamaURL is the Ollama endpoint (empty → http://localhost:11434).
	EmbedModel string
	OllamaURL  string

	// HTTPAddr is the Streamable HTTP listen address (empty → 127.0.0.1:8080).
	// AllowRemote is the explicit opt-in to bind a routable address; Generate
	// additionally derives it as true whenever HTTPAddr is non-loopback so the
	// produced config passes Validate().
	HTTPAddr    string
	AllowRemote bool

	// SanitizeRulesPath points at the sanitize ruleset YAML.
	SanitizeRulesPath string

	// DomainProjectDir / DomainCorpusDir wire channel ② (domain-knowledge
	// embedding); both empty disables it. GlossaryPath enables vocab expansion.
	DomainProjectDir string
	DomainCorpusDir  string
	GlossaryPath     string

	// FootprintDir / AuditDir are the logging output directories.
	FootprintDir string
	AuditDir     string

	// ServiceLabelPrefix overrides the launchd label prefix. Empty leaves it
	// unset, which is the normal case; a deployment whose agents are already
	// installed under another prefix names it here so regenerating its config
	// does not silently orphan them.
	ServiceLabelPrefix string
}

// Generate builds a *Config from o, applying defaults for empty fields. The
// result is not written anywhere (use Save) and is guaranteed to pass
// Validate() for well-formed inputs (e.g. a loopback HTTPAddr or an explicit
// AllowRemote for a routable one).
func Generate(o GenerateOptions) *Config {
	name := o.Name
	if name == "" {
		// Default the instance identity to the deployment namespace root
		// (KNOWLEDGE_MCP_NAMESPACE / -ldflags BuildRoot), so a branded pack's
		// generated config matches its tool namespace instead of literal "cks".
		name = kmcp.Root("", "cks")
	}
	embedModel := o.EmbedModel
	if embedModel == "" {
		embedModel = "bge-m3"
	}
	ollamaURL := o.OllamaURL
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	httpAddr := o.HTTPAddr
	if httpAddr == "" {
		httpAddr = "127.0.0.1:8080"
	}
	// Default to the baseline ruleset so a generated config is safe by default:
	// an empty rules_path would load a NOOP ruleset (no redaction) and be
	// rejected by Validate outside dev mode.
	sanitizeRules := o.SanitizeRulesPath
	if sanitizeRules == "" {
		sanitizeRules = DefaultSanitizeRulesPath
	}
	// allow_remote is derived from the address (mirroring the shell script):
	// a non-loopback bind requires the opt-in, so force it true there to keep
	// the config valid. An explicit AllowRemote is always honored.
	allowRemote := o.AllowRemote || !isLoopbackAddr(httpAddr)

	graphPath := o.GraphPath
	if graphPath == "" {
		graphPath = filepath.Join(o.DatasetDir, "graph", "graph.db")
	}
	vectorPath := o.VectorPath
	if vectorPath == "" {
		vectorPath = filepath.Join(o.DatasetDir, "vector")
	}

	return &Config{
		Version:     configVersion,
		Name:        name,
		Description: o.Description,
		Backends: BackendsConfig{
			CKG: CKGConfig{
				Path:       graphPath,
				SourceRoot: o.SourceRoot,
				BinaryPath: o.GraphBinary,
				PolicyFile: o.PolicyFile,
				TimeoutMS:  5000,
			},
			CKV: CKVConfig{
				Path:       vectorPath,
				BinaryPath: o.VectorBinary,
				EmbedModel: embedModel,
				OllamaURL:  ollamaURL,
				TimeoutMS:  3000,
			},
		},
		Listen: ListenConfig{
			Transport:   "http",
			HTTPAddr:    httpAddr,
			AllowRemote: allowRemote,
		},
		Logging: LoggingConfig{
			Level:        "info",
			Mode:         "prod",
			FootprintDir: o.FootprintDir,
			AuditDir:     o.AuditDir,
		},
		Sanitize: SanitizeConfig{
			RulesPath:               sanitizeRules,
			DefaultAction:           contract.RedactionDrop,
			FailClosedOnUnknownRule: true,
		},
		Domain: DomainConfig{
			ProjectDir: o.DomainProjectDir,
			CorpusDir:  o.DomainCorpusDir,
		},
		Service: ServiceConfig{LabelPrefix: o.ServiceLabelPrefix},
		Vocab: VocabConfig{
			GlossaryPath: o.GlossaryPath,
		},
	}
}

// Save marshals c to YAML and writes it to path with 0644 permissions. It
// reuses the same YAML library Load uses, so Save → Load round-trips.
func Save(path string, c *Config) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("config: write %q: %w", path, err)
	}
	return nil
}

// isLoopbackAddr reports whether addr is a loopback host:port, reusing the same
// check Validate applies to reject non-loopback binds without AllowRemote.
func isLoopbackAddr(addr string) bool {
	return validateLoopback(addr) == nil
}
