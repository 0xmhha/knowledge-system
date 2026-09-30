package doctorcli

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type InitResult struct {
	ProjectID  string `json:"project_id"`
	ConfigPath string `json:"config_path"`
	SourceRoot string `json:"source_root"`
	Dataset    string `json:"dataset"`
	Embedder   string `json:"embedder"`
}

func NewInitCmd() *cobra.Command {
	var src, dataset, configPath, embedder, model, projectID string
	cmd := &cobra.Command{Use: "init", Short: "Write a project-specific setup config without indexing",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := InitWithProjectID(src, dataset, configPath, embedder, model, projectID)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().StringVar(&src, "src", "", "Git source repository")
	cmd.Flags().StringVar(&dataset, "dataset", "", "dataset root outside the source tree")
	cmd.Flags().StringVar(&configPath, "config-out", "", "new setup YAML path; existing files are never overwritten")
	cmd.Flags().StringVar(&embedder, "embedder", "", "embedding backend: ollama, bgeonnx, or mock (test only)")
	cmd.Flags().StringVar(&model, "model-name", "", "embedding model name (required for real backends)")
	cmd.Flags().StringVar(&projectID, "project-id", "", "stable project ID; generated once when omitted")
	for _, flag := range []string{"src", "dataset", "config-out", "embedder"} {
		_ = cmd.MarkFlagRequired(flag)
	}
	return cmd
}

// Init writes only the setup config. The user chooses a model explicitly;
// a mock backend is accepted solely when selected by name for structural tests.
func Init(src, dataset, configPath, embedder, model string) (InitResult, error) {
	return InitWithProjectID(src, dataset, configPath, embedder, model, "")
}

var projectIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// InitWithProjectID preserves a caller-supplied project identity or generates
// one from cryptographic randomness. It never derives identity from a path,
// repository name, or remote URL, which can change or collide.
func InitWithProjectID(src, dataset, configPath, embedder, model, projectID string) (InitResult, error) {
	if embedder != "ollama" && embedder != "bgeonnx" && embedder != "mock" {
		return InitResult{}, fmt.Errorf("init: unknown embedder %q", embedder)
	}
	if embedder != "mock" && model == "" {
		return InitResult{}, fmt.Errorf("init: --model-name is required for %s", embedder)
	}
	if projectID == "" {
		var entropy [16]byte
		if _, err := rand.Read(entropy[:]); err != nil {
			return InitResult{}, fmt.Errorf("init: create project ID: %w", err)
		}
		projectID = fmt.Sprintf("p-%x", entropy)
	}
	if !projectIDPattern.MatchString(projectID) {
		return InitResult{}, fmt.Errorf("init: invalid project ID %q", projectID)
	}
	report, err := Inspect(src, "")
	if err != nil {
		return InitResult{}, err
	}
	dataset, err = resolvedFuturePath(dataset)
	if err != nil {
		return InitResult{}, err
	}
	rel, err := filepath.Rel(report.SourceRoot, dataset)
	if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return InitResult{}, fmt.Errorf("init: dataset must be outside the source tree")
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return InitResult{}, err
	}
	config := struct {
		ProjectID string `yaml:"project_id"`
		Src       string `yaml:"src"`
		Out       string `yaml:"out"`
		Embedder  string `yaml:"embedder"`
		ModelName string `yaml:"model_name,omitempty"`
	}{ProjectID: projectID, Src: report.SourceRoot, Out: dataset, Embedder: embedder, ModelName: model}
	buf, err := yaml.Marshal(config)
	if err != nil {
		return InitResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return InitResult{}, err
	}
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return InitResult{}, fmt.Errorf("init: create config without replacing existing file: %w", err)
	}
	if _, err := file.Write(buf); err != nil {
		file.Close()
		os.Remove(configPath)
		return InitResult{}, err
	}
	if err := file.Close(); err != nil {
		os.Remove(configPath)
		return InitResult{}, err
	}
	return InitResult{ProjectID: projectID, ConfigPath: configPath, SourceRoot: report.SourceRoot, Dataset: dataset, Embedder: embedder}, nil
}

func resolvedFuturePath(path string) (string, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}
