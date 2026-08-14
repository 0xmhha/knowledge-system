package mcpcli

import (
	"fmt"
	"io"
	"net"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/system/config"
	flag "github.com/spf13/pflag"
)

// runGenConfig implements the `cks-mcp gen-config` subcommand: it maps flags to
// config.GenerateOptions, builds and validates the config, and writes it to
// --out as YAML. It is the Go replacement for system/scripts/gen-cks-config.sh
// (the config half; the cks.env half stays in the shell wrappers). Filesystem
// resolution is the caller's job — pass absolute paths. Binding beyond
// loopback is opt-in via --lan.
func runGenConfig(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("gen-config", flag.ContinueOnError)
	fs.SetOutput(stdout)

	out := fs.String("out", "", "write the generated cks config YAML here (required)")
	name := fs.String("name", "", "instance name (MCP server name); empty defaults to \"cks\"")
	description := fs.String("description", "", "human-facing instance description")
	datasetDir := fs.String("dataset-dir", "", "consolidated dataset dir (holds graph/graph.db and vector/)")
	graphPath := fs.String("graph-path", "", "explicit graph.db FILE path — overrides the --dataset-dir derivation (pre-consolidation layouts, e.g. <dataset>/graph-db/graph.db)")
	vectorPath := fs.String("vector-path", "", "explicit vector index DIRECTORY — overrides the --dataset-dir derivation (e.g. <dataset>/vector-db)")
	sourceRoot := fs.String("source-root", "", "working tree the index was built against")
	graphBinary := fs.String("graph-binary", "", "ckg binary path (cks.ops.index)")
	vectorBinary := fs.String("vector-binary", "", "ckv binary path (cks.ops.index)")
	policyFile := fs.String("policy-file", "", "ckg governance policy file")
	embedModel := fs.String("embed-model", "", "Ollama embed model; empty defaults to \"bge-m3\"")
	ollamaURL := fs.String("ollama-url", "", "Ollama endpoint; empty defaults to http://localhost:11434")
	port := fs.String("port", "", "HTTP listen port; the host is filled in automatically — 127.0.0.1, or every interface with --lan")
	httpAddr := fs.String("http-addr", "", "full HTTP listen host:port when you need to name the interface yourself; prefer --port. Empty defaults to 127.0.0.1:8080")
	lan := fs.Bool("lan", false, "bind every interface so machines on this subnet can reach it (combine with --port); the printed URL uses this host's routable address")
	allowRemote := fs.Bool("allow-remote", false, "opt in to binding a routable address (derived true for non-loopback addrs)")
	sanitizeRules := fs.String("sanitize-rules", "", "sanitize ruleset YAML path")
	domainProjectDir := fs.String("domain-project-dir", "", "domain-knowledge project dir (enables channel 2)")
	domainCorpusDir := fs.String("domain-corpus-dir", "", "domain corpus export dir (enables channel 2)")
	glossaryPath := fs.String("glossary", "", "vocab glossary YAML path")
	footprintDir := fs.String("footprint-dir", "", "footprint log output dir")
	auditDir := fs.String("audit-dir", "", "audit log output dir")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out is required")
	}

	// A written config is used from an arbitrary working directory, so resolve
	// the sanitize ruleset to an absolute path (defaulting to the in-repo
	// baseline) rather than emitting a cwd-relative one that would fail to load.
	sanitizePath := *sanitizeRules
	if sanitizePath == "" {
		sanitizePath = config.DefaultSanitizeRulesPath
	}
	if abs, err := filepath.Abs(sanitizePath); err == nil {
		sanitizePath = abs
	}

	// The operator picks a port; the host is policy, not input. --port binds
	// loopback, --lan widens that to every interface so a remote agent can
	// reach it, and --http-addr remains the escape hatch for naming an
	// interface explicitly (its port wins when --lan rewrites the host).
	if *port != "" && *httpAddr != "" {
		return fmt.Errorf("--port and --http-addr are mutually exclusive (--http-addr already carries a port)")
	}
	addr := *httpAddr
	if *port != "" {
		addr = net.JoinHostPort("127.0.0.1", *port)
	}
	if *lan {
		p := "8080"
		if _, pp, err := net.SplitHostPort(addr); err == nil && pp != "" {
			p = pp
		}
		// Bind every interface rather than the address detected right now.
		// A pinned address stops existing when DHCP moves it — the server
		// then cannot bind at all — and it stops answering on loopback, which
		// every local health probe uses. The wildcard survives both, and
		// AdvertiseHost still resolves it to a routable URL for the operator.
		addr = net.JoinHostPort(wildcardBind, p)
	}

	cfg := config.Generate(config.GenerateOptions{
		Name:              *name,
		Description:       *description,
		DatasetDir:        *datasetDir,
		GraphPath:         *graphPath,
		VectorPath:        *vectorPath,
		SourceRoot:        *sourceRoot,
		GraphBinary:       *graphBinary,
		VectorBinary:      *vectorBinary,
		PolicyFile:        *policyFile,
		EmbedModel:        *embedModel,
		OllamaURL:         *ollamaURL,
		HTTPAddr:          addr,
		AllowRemote:       *allowRemote,
		SanitizeRulesPath: sanitizePath,
		DomainProjectDir:  *domainProjectDir,
		DomainCorpusDir:   *domainCorpusDir,
		GlossaryPath:      *glossaryPath,
		FootprintDir:      *footprintDir,
		AuditDir:          *auditDir,
	})
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("generated config invalid: %w", err)
	}
	if err := config.Save(*out, cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "generated %s (name=%s, listen=%s allow_remote=%v)\n",
		*out, cfg.Name, cfg.Listen.HTTPAddr, cfg.Listen.AllowRemote)
	return nil
}
