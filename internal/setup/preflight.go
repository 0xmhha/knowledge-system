package setup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
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
// only. A tag matches exactly, or as an implicit ":latest" alias. Ambiguous
// names and absent/invalid digests are rejected before a long build starts.
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
			Name   string `json:"name"`
			Digest string `json:"digest"`
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
	matches := 0
	var selectedName, selectedDigest string
	for _, m := range tags.Models {
		names = append(names, m.Name)
		if m.Name == model || (!strings.Contains(model, ":") && m.Name == model+":latest") {
			matches++
			selectedName, selectedDigest = m.Name, m.Digest
		}
	}
	if matches > 1 {
		return fmt.Errorf("preflight: Ollama model %q is ambiguous; specify an exact tag", model)
	}
	if matches == 1 {
		if !regexp.MustCompile(`^[0-9a-fA-F]{64}$`).MatchString(selectedDigest) {
			return fmt.Errorf("preflight: Ollama model %q has no valid digest", selectedName)
		}
		if emit != nil {
			emit(Event{Time: time.Now().UTC(), Step: "vector-preflight", Type: "output",
				Message: fmt.Sprintf("Ollama reachable at %s; model %q present (%s)", base, model, selectedName)})
		}
		return nil
	}
	return fmt.Errorf("preflight: Ollama model %q not found at %s (have: %s) — run `ollama pull %s`",
		model, base, strings.Join(names, ", "), model)
}
