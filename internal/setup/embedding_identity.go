package setup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// ResolveEmbeddingIdentity asks the same CKV binary used by BuildPlan for its
// complete space identity. It is done before graph/vector build so dataset_id
// is independent of engine outputs. CKV verifies Ollama model bytes again
// during embedding and before publishing its own manifest.
func ResolveEmbeddingIdentity(ctx context.Context, o Options) (json.RawMessage, error) {
	bin := o.VectorBin
	if bin == "" {
		bin = "ckv"
	}
	args := []string{}
	if o.Embedder != "" {
		args = append(args, "--embedder", o.Embedder)
	}
	if o.ModelName != "" {
		args = append(args, "--model-name", o.ModelName)
	}
	if o.EmbedDim != 0 {
		args = append(args, "--embed-dim", fmt.Sprint(o.EmbedDim))
	}
	if o.QueryPrefixPolicy != "" {
		args = append(args, "--query-prefix-policy", o.QueryPrefixPolicy)
	}
	args = append(args, "identity")
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()
	if o.OllamaURL != "" {
		cmd.Env = append(cmd.Env, "CKV_OLLAMA_ENDPOINT="+o.OllamaURL)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("resolve CKV embedding identity: %w: %s", err, stderr.String())
	}
	var response struct {
		Identity json.RawMessage `json:"embedding_identity"`
	}
	if err := json.Unmarshal(out, &response); err != nil || len(response.Identity) == 0 {
		return nil, fmt.Errorf("CKV returned no valid embedding identity: %v", err)
	}
	return response.Identity, nil
}
