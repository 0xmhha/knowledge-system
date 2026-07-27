package setup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PreflightOllama fails fast, before the (long) vector build starts, when the
// Ollama embedder backend is unusable: the daemon is unreachable, or the
// requested model is not pulled. Without this the build proceeds and only dies
// partway through with a per-chunk embed error — the same fail-fast the old
// build-knowledge.sh preflight gave.
//
// endpoint is the Ollama base URL (e.g. http://localhost:11434). model is the
// requested embedding model (e.g. "bge-m3"); an empty model checks reachability
// only. A tag matches the model when it is exactly the name, the name with an
// implicit ":latest", or any explicit tag of that model.
func PreflightOllama(endpoint, model string, emit func(Event)) error {
	base := strings.TrimRight(endpoint, "/")
	if base == "" {
		base = "http://localhost:11434"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/api/tags")
	if err != nil {
		return fmt.Errorf("preflight: Ollama unreachable at %s (%v) — start `ollama serve`", base, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("preflight: Ollama %s/api/tags returned HTTP %d", base, resp.StatusCode)
	}

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("preflight: decoding Ollama /api/tags: %w", err)
	}

	if model == "" {
		if emit != nil {
			emit(Event{Time: time.Now().UTC(), Step: "vector-preflight", Type: "output",
				Message: fmt.Sprintf("Ollama reachable at %s (%d models); no model name to verify", base, len(tags.Models))})
		}
		return nil
	}

	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		names = append(names, m.Name)
	}
	for _, name := range names {
		if name == model || name == model+":latest" || strings.HasPrefix(name, model+":") {
			if emit != nil {
				emit(Event{Time: time.Now().UTC(), Step: "vector-preflight", Type: "output",
					Message: fmt.Sprintf("Ollama reachable at %s; model %q present (%s)", base, model, name)})
			}
			return nil
		}
	}
	return fmt.Errorf("preflight: Ollama model %q not found at %s (have: %s) — run `ollama pull %s`",
		model, base, strings.Join(names, ", "), model)
}
