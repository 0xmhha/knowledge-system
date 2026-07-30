package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// fileConfig is the YAML shape of a setup config file (typically a project
// pack's setup.yaml). Field names mirror Options; relative paths resolve
// against the config file's directory so a pack can reference its own data
// portably.
type fileConfig struct {
	Src                 string `yaml:"src"`
	Out                 string `yaml:"out"`
	GraphBin            string `yaml:"graph_bin"`
	VectorBin           string `yaml:"vector_bin"`
	PolicyFile          string `yaml:"policy_file"`
	SecurityPatternFile string `yaml:"security_pattern_file"`
	Embedder            string `yaml:"embedder"`
	ModelName           string `yaml:"model_name"`
	EmbedDim            int    `yaml:"embed_dim"`
	OllamaURL           string `yaml:"ollama_url"`
	VectorPolicy        string `yaml:"vector_policy"`
	SkipVector          bool   `yaml:"skip_vector"`
	Filelist            string `yaml:"filelist"`
	FilelistBin         string `yaml:"filelist_bin"`
	DomainKnowledge     string `yaml:"domain_knowledge"`
	DerivedDir          string `yaml:"derived_dir"`
	CodeRoot            string `yaml:"code_root"`
	GlossaryFile        string `yaml:"glossary_file"`
	FlowCorpus          string `yaml:"flow_corpus"`
	DomainExportBin     string `yaml:"domain_export_bin"`
	DomainSyncBin       string `yaml:"domain_sync_bin"`
	GlossaryGenBin      string `yaml:"glossary_gen_bin"`
}

// LoadConfig reads a setup config file into Options. Path-valued fields that
// are relative are resolved against the config file's directory.
func LoadConfig(path string) (Options, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return Options{}, err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(buf, &fc); err != nil {
		return Options{}, fmt.Errorf("%s: %w", path, err)
	}
	base := filepath.Dir(path)
	rel := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}
	return Options{
		Src:                 rel(fc.Src),
		Out:                 rel(fc.Out),
		GraphBin:            fc.GraphBin,
		VectorBin:           fc.VectorBin,
		PolicyFile:          rel(fc.PolicyFile),
		SecurityPatternFile: rel(fc.SecurityPatternFile),
		Embedder:            fc.Embedder,
		ModelName:           fc.ModelName,
		EmbedDim:            fc.EmbedDim,
		OllamaURL:           fc.OllamaURL,
		VectorPolicy:        rel(fc.VectorPolicy),
		SkipVector:          fc.SkipVector,
		FilelistConfig:      rel(fc.Filelist),
		FilelistBin:         fc.FilelistBin,
		DomainKnowledge:     rel(fc.DomainKnowledge),
		DerivedDir:          rel(fc.DerivedDir),
		// CodeRoot points at a checkout outside the pack, so ${VAR} forms
		// are expanded rather than resolved against the config directory.
		CodeRoot:        os.ExpandEnv(fc.CodeRoot),
		GlossaryFile:    rel(fc.GlossaryFile),
		FlowCorpus:      rel(fc.FlowCorpus),
		DomainExportBin: fc.DomainExportBin,
		DomainSyncBin:   fc.DomainSyncBin,
		GlossaryGenBin:  fc.GlossaryGenBin,
	}, nil
}
