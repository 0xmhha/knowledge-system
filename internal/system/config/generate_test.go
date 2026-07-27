package config

import (
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestGenerate_ConsolidatedPaths(t *testing.T) {
	t.Parallel()
	o := GenerateOptions{
		DatasetDir: "/abs/knowledge-data/pr-77",
	}
	c := Generate(o)

	// Consolidated layout: <dataset>/graph/graph.db and <dataset>/vector,
	// NOT the old graph-db/vector-db pair.
	wantCKG := filepath.Join("/abs/knowledge-data/pr-77", "graph", "graph.db")
	if c.Backends.CKG.Path != wantCKG {
		t.Errorf("CKG.Path = %q, want %q", c.Backends.CKG.Path, wantCKG)
	}
	wantCKV := filepath.Join("/abs/knowledge-data/pr-77", "vector")
	if c.Backends.CKV.Path != wantCKV {
		t.Errorf("CKV.Path = %q, want %q", c.Backends.CKV.Path, wantCKV)
	}
}

func TestGenerate_Defaults(t *testing.T) {
	t.Parallel()
	c := Generate(GenerateOptions{DatasetDir: "/d"})

	if c.Version != configVersion {
		t.Errorf("Version = %d, want %d", c.Version, configVersion)
	}
	if c.Listen.Transport != "http" {
		t.Errorf("Transport = %q, want http", c.Listen.Transport)
	}
	if c.Listen.HTTPAddr != "127.0.0.1:8080" {
		t.Errorf("HTTPAddr = %q, want 127.0.0.1:8080", c.Listen.HTTPAddr)
	}
	if c.Listen.AllowRemote {
		t.Error("AllowRemote should default to false for a loopback addr")
	}
	if c.Backends.CKV.OllamaURL != "http://localhost:11434" {
		t.Errorf("OllamaURL = %q, want http://localhost:11434", c.Backends.CKV.OllamaURL)
	}
	if c.Sanitize.DefaultAction != contract.RedactionDrop {
		t.Errorf("DefaultAction = %q, want drop", c.Sanitize.DefaultAction)
	}
	if !c.Sanitize.FailClosedOnUnknownRule {
		t.Error("FailClosedOnUnknownRule should default true")
	}
	if c.Sanitize.RulesPath != DefaultSanitizeRulesPath {
		t.Errorf("RulesPath = %q, want the baseline %q (safe-by-default redaction)", c.Sanitize.RulesPath, DefaultSanitizeRulesPath)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("generated default config did not validate: %v", err)
	}
}

func TestGenerate_NameFromNamespace(t *testing.T) {
	// Not parallel: mutates KNOWLEDGE_MCP_NAMESPACE via t.Setenv.

	// Upstream: no namespace injected → the historical "cks" default.
	t.Setenv("KNOWLEDGE_MCP_NAMESPACE", "")
	if got := Generate(GenerateOptions{DatasetDir: "/d"}).Name; got != "cks" {
		t.Errorf("default name = %q, want cks", got)
	}
	// Branded deployment: the generated instance name follows the namespace,
	// matching the tool namespace instead of the literal "cks".
	t.Setenv("KNOWLEDGE_MCP_NAMESPACE", "stablenet_knowledge")
	if got := Generate(GenerateOptions{DatasetDir: "/d"}).Name; got != "stablenet_knowledge" {
		t.Errorf("name with namespace = %q, want stablenet_knowledge", got)
	}
	// An explicit name always wins over the namespace default.
	if got := Generate(GenerateOptions{DatasetDir: "/d", Name: "custom"}).Name; got != "custom" {
		t.Errorf("explicit name = %q, want custom", got)
	}
}

func TestGenerate_AllowRemoteDerivedFromLANAddr(t *testing.T) {
	t.Parallel()
	// A non-loopback address must yield allow_remote=true so the produced
	// config passes Validate() (loopback is enforced otherwise).
	c := Generate(GenerateOptions{DatasetDir: "/d", HTTPAddr: "192.168.1.10:8080"})
	if !c.Listen.AllowRemote {
		t.Error("AllowRemote should be derived true for a LAN addr")
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("LAN config did not validate: %v", err)
	}
}

func TestGenerate_ExplicitAllowRemote(t *testing.T) {
	t.Parallel()
	// Explicit opt-in is honored even on a loopback address.
	c := Generate(GenerateOptions{DatasetDir: "/d", AllowRemote: true})
	if !c.Listen.AllowRemote {
		t.Error("explicit AllowRemote=true should be honored")
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("config did not validate: %v", err)
	}
}

func TestGenerate_MapsInputs(t *testing.T) {
	t.Parallel()
	o := GenerateOptions{
		Name:              "cks-stablenet",
		Description:       "go-stablenet (bge-m3)",
		DatasetDir:        "/data/pr-77",
		SourceRoot:        "/src/go-stablenet",
		GraphBinary:       "/bin/ckg",
		VectorBinary:      "/bin/ckv",
		PolicyFile:        "/policies/ckg-policy.yaml",
		EmbedModel:        "bge-m3",
		OllamaURL:         "http://ollama:11434",
		HTTPAddr:          "127.0.0.1:9090",
		SanitizeRulesPath: "/policies/sanitization_rules.yaml",
		DomainProjectDir:  "/docs/projects/go-stablenet",
		DomainCorpusDir:   "/generated/corpus/go-stablenet",
		GlossaryPath:      "/docs/glossary.yaml",
		FootprintDir:      "/logs/footprint",
		AuditDir:          "/logs/audit",
	}
	c := Generate(o)

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"Name", c.Name, "cks-stablenet"},
		{"Description", c.Description, "go-stablenet (bge-m3)"},
		{"CKG.SourceRoot", c.Backends.CKG.SourceRoot, "/src/go-stablenet"},
		{"CKG.BinaryPath", c.Backends.CKG.BinaryPath, "/bin/ckg"},
		{"CKG.PolicyFile", c.Backends.CKG.PolicyFile, "/policies/ckg-policy.yaml"},
		{"CKV.BinaryPath", c.Backends.CKV.BinaryPath, "/bin/ckv"},
		{"CKV.EmbedModel", c.Backends.CKV.EmbedModel, "bge-m3"},
		{"CKV.OllamaURL", c.Backends.CKV.OllamaURL, "http://ollama:11434"},
		{"Listen.HTTPAddr", c.Listen.HTTPAddr, "127.0.0.1:9090"},
		{"Sanitize.RulesPath", c.Sanitize.RulesPath, "/policies/sanitization_rules.yaml"},
		{"Domain.ProjectDir", c.Domain.ProjectDir, "/docs/projects/go-stablenet"},
		{"Domain.CorpusDir", c.Domain.CorpusDir, "/generated/corpus/go-stablenet"},
		{"Vocab.GlossaryPath", c.Vocab.GlossaryPath, "/docs/glossary.yaml"},
		{"Logging.FootprintDir", c.Logging.FootprintDir, "/logs/footprint"},
		{"Logging.AuditDir", c.Logging.AuditDir, "/logs/audit"},
	}
	for _, ck := range checks {
		if ck.got != ck.want {
			t.Errorf("%s = %q, want %q", ck.name, ck.got, ck.want)
		}
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("mapped config did not validate: %v", err)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	t.Parallel()
	o := GenerateOptions{
		Name:              "cks-stablenet",
		DatasetDir:        "/data/pr-77",
		SourceRoot:        "/src/go-stablenet",
		EmbedModel:        "bge-m3",
		HTTPAddr:          "127.0.0.1:8080",
		SanitizeRulesPath: "/policies/sanitization_rules.yaml",
	}
	c := Generate(o)

	path := filepath.Join(t.TempDir(), "cks.yaml")
	if err := Save(path, c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Backends.CKG.Path != c.Backends.CKG.Path {
		t.Errorf("CKG.Path round-trip: got %q, want %q", got.Backends.CKG.Path, c.Backends.CKG.Path)
	}
	if got.Backends.CKV.Path != c.Backends.CKV.Path {
		t.Errorf("CKV.Path round-trip: got %q, want %q", got.Backends.CKV.Path, c.Backends.CKV.Path)
	}
	if got.Listen.Transport != c.Listen.Transport {
		t.Errorf("Transport round-trip: got %q, want %q", got.Listen.Transport, c.Listen.Transport)
	}
	if got.Listen.HTTPAddr != c.Listen.HTTPAddr {
		t.Errorf("HTTPAddr round-trip: got %q, want %q", got.Listen.HTTPAddr, c.Listen.HTTPAddr)
	}
	if got.Listen.AllowRemote != c.Listen.AllowRemote {
		t.Errorf("AllowRemote round-trip: got %v, want %v", got.Listen.AllowRemote, c.Listen.AllowRemote)
	}
	if got.Sanitize.DefaultAction != c.Sanitize.DefaultAction {
		t.Errorf("DefaultAction round-trip: got %q, want %q", got.Sanitize.DefaultAction, c.Sanitize.DefaultAction)
	}
}
