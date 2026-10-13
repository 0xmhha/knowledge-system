package mcpcli

import (
	"fmt"
	"io"
	"net"
	"os"
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
	embedDim := fs.Int("embed-dim", 0, "Ollama MRL output dimension; 0 uses native dimension")
	queryPrefixPolicy := fs.String("query-prefix-policy", "registry", "Ollama query transformation: registry | none")
	ollamaURL := fs.String("ollama-url", "", "Ollama endpoint; empty defaults to http://localhost:11434")
	port := fs.String("port", "", "HTTP listen port; the host is filled in automatically — 127.0.0.1, or every interface with --lan")
	httpAddr := fs.String("http-addr", "", "full HTTP listen host:port when you need to name the interface yourself; prefer --port. Empty defaults to 127.0.0.1:8080")
	lan := fs.Bool("lan", false, "bind every interface so machines on this subnet can reach it (combine with --port); the printed URL uses this host's routable address")
	allowRemote := fs.Bool("allow-remote", false, "opt in to binding a routable address (derived true for non-loopback addrs)")
	sanitizeRules := fs.String("sanitize-rules", "", "sanitize ruleset YAML path")
	domainProjectDir := fs.String("domain-project-dir", "", "domain-knowledge project dir (enables channel 2)")
	domainCorpusDir := fs.String("domain-corpus-dir", "", "domain corpus export dir (enables channel 2)")
	glossaryPath := fs.String("glossary", "", "vocab glossary YAML path")
	semanticStore := fs.String("semantic-store", "", "optional SQLite semantic store for reviewed v2 trace paths")
	ontologyMode := fs.String("ontology-mode", "off", "optional reviewed ontology arm: off | baseline | concept_text | relations | combined")
	ontologyBudget := fs.Int("ontology-budget-ms", 1000, "ontology preparation/retrieval budget in milliseconds (1..5000)")
	footprintDir := fs.String("footprint-dir", "", "footprint log output dir")
	auditDir := fs.String("audit-dir", "", "audit log output dir")
	labelPrefix := fs.String("service-label-prefix", "", "launchd label prefix for `cks mcp service`; empty uses the engine's own name (set it only when agents are already installed under another prefix, or two distributions share a host)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out is required")
	}

	if (*ontologyMode == "relations" || *ontologyMode == "concept_text" || *ontologyMode == "combined") && *semanticStore == "" {
		return fmt.Errorf("--semantic-store is required for an enabled --ontology-mode")
	}
	if *ontologyBudget < 1 || *ontologyBudget > 5000 {
		return fmt.Errorf("--ontology-budget-ms must be 1..5000")
	}

	// A written config is used from an arbitrary working directory, so the
	// sanitize ruleset is written as an absolute path. An explicit path is
	// taken as given (it may name a file on the serving host); the default
	// baseline is located on disk and must exist.
	sanitizePath := *sanitizeRules
	if sanitizePath == "" {
		found, err := locateBaselineRuleset()
		if err != nil {
			return err
		}
		sanitizePath = found
	} else if abs, err := filepath.Abs(sanitizePath); err == nil {
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

	semanticPath := *semanticStore
	if semanticPath != "" {
		var err error
		semanticPath, err = filepath.Abs(semanticPath)
		if err != nil {
			return err
		}
	}
	cfg := config.Generate(config.GenerateOptions{
		Name:                 *name,
		Description:          *description,
		DatasetDir:           *datasetDir,
		GraphPath:            *graphPath,
		VectorPath:           *vectorPath,
		SourceRoot:           *sourceRoot,
		GraphBinary:          *graphBinary,
		VectorBinary:         *vectorBinary,
		PolicyFile:           *policyFile,
		EmbedModel:           *embedModel,
		EmbedDim:             *embedDim,
		QueryPrefixPolicy:    *queryPrefixPolicy,
		OllamaURL:            *ollamaURL,
		HTTPAddr:             addr,
		AllowRemote:          *allowRemote,
		SanitizeRulesPath:    sanitizePath,
		DomainProjectDir:     *domainProjectDir,
		DomainCorpusDir:      *domainCorpusDir,
		GlossaryPath:         *glossaryPath,
		SemanticStorePath:    semanticPath,
		SemanticOntologyMode: *ontologyMode,
		SemanticBudgetMS:     *ontologyBudget,
		FootprintDir:         *footprintDir,
		AuditDir:             *auditDir,
		ServiceLabelPrefix:   *labelPrefix,
	})
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("generated config invalid: %w", err)
	}
	if err := config.SaveNew(*out, cfg); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "generated %s (name=%s, listen=%s allow_remote=%v)\n",
		*out, cfg.Name, cfg.Listen.HTTPAddr, cfg.Listen.AllowRemote)
	return nil
}

// executablePath is os.Executable, replaceable in tests.
var executablePath = os.Executable

// locateBaselineRuleset finds the shipped sanitize baseline. It looks beside
// the running binary first (a checkout's bin/ next to system/policies, or a
// package root holding policies/), then in the working directory and its
// parents, so the result does not depend on where gen-config is run from.
func locateBaselineRuleset() (string, error) {
	var candidates []string
	if exe, err := executablePath(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "..", config.DefaultSanitizeRulesPath),
			filepath.Join(dir, "policies", "sanitization_rules.yaml"),
			filepath.Join(dir, "..", "policies", "sanitization_rules.yaml"))
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; dir = filepath.Dir(dir) {
			candidates = append(candidates, filepath.Join(dir, config.DefaultSanitizeRulesPath))
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.Mode().IsRegular() {
			return filepath.Abs(filepath.Clean(c))
		}
	}
	return "", fmt.Errorf("config_invalid: no sanitize baseline ruleset found beside the binary or in the working tree; pass --sanitize-rules")
}
