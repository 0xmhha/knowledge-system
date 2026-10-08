package evalcli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/embedder"
)

// Runtime pins are a consistency check. Human input review and FINAL reservation
// are enforced by the refactoring runner; a standalone matrix remains diagnostic.
type matrixEvaluationBinding struct {
	SchemaVersion int    `json:"schema_version"`
	DatasetID     string `json:"dataset_id"`
	SourceCommit  string `json:"source_commit"`
	ConfigSHA256  string `json:"config_sha256"`
	BinarySHA256  string `json:"binary_sha256"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	ModelDigest   string `json:"model_digest"`
	Dimension     int    `json:"dimension"`
	Context       int    `json:"runtime_context_tokens"`
	Batch         int    `json:"runtime_batch_tokens"`
	RetrievalK    int    `json:"retrieval_k"`
}

// Reject duplicate keys, including nested objects, before typed JSON decoding.
func matrixUniqueJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		return fmt.Errorf("binding must be an object")
	}
	keys := map[string]bool{}
	for d.More() {
		t, err := d.Token()
		if err != nil {
			return err
		}
		key, ok := t.(string)
		if !ok || keys[key] {
			return fmt.Errorf("duplicate binding key")
		}
		keys[key] = true
		var value any
		if err := d.Decode(&value); err != nil {
			return err
		}
		// This flat schema permits no objects, arrays or null values.
		switch value.(type) {
		case string, float64:
		default:
			return fmt.Errorf("invalid binding value")
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("binding must be one JSON object")
	}
	return nil
}

func bindMatrixInputs(path string, read matrixInputReader) matrixInputReader {
	return func(cfg *config.Config, configPath, input, binary string, extra []string) (map[string]string, *setup.DatasetIdentity, error) {
		fail := func(reason string) (map[string]string, *setup.DatasetIdentity, error) {
			return nil, nil, fmt.Errorf("evaluation binding: %s", reason)
		}
		f, err := os.Open(path)
		if err != nil {
			return fail(err.Error())
		}
		raw, err := io.ReadAll(io.LimitReader(f, 65537))
		_ = f.Close()
		if err != nil || len(raw) > 65536 {
			return fail("input exceeds bound or unreadable")
		}
		if err := matrixUniqueJSON(json.NewDecoder(bytes.NewReader(raw))); err != nil {
			return fail(err.Error())
		}
		var expected matrixEvaluationBinding
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if err := d.Decode(&expected); err != nil {
			return fail(err.Error())
		}
		digest := regexp.MustCompile(`^[0-9a-f]{64}$`)
		if expected.SchemaVersion != 1 || expected.DatasetID == "" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(expected.SourceCommit) ||
			!digest.MatchString(expected.ConfigSHA256) || !digest.MatchString(expected.BinarySHA256) || !digest.MatchString(expected.ModelDigest) ||
			expected.Provider != "ollama" || expected.Model == "" || expected.Dimension <= 0 || expected.Context <= 0 || expected.Batch <= 0 || expected.RetrievalK <= 0 {
			return fail("incomplete pins")
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fail(err.Error())
		}
		locked, identity, err := read(cfg, configPath, input, binary, append(extra, absolute))
		if err != nil {
			return nil, nil, err
		}
		sum := sha256.Sum256(raw)
		if locked[absolute] != hex.EncodeToString(sum[:]) {
			return fail("binding changed during preflight")
		}
		if identity == nil || identity.DatasetID != expected.DatasetID || identity.Source.SourceCommit != expected.SourceCommit {
			return fail("dataset/source mismatch")
		}
		if locked[configPath] != expected.ConfigSHA256 || locked[binary] != expected.BinarySHA256 {
			return fail("config/binary mismatch")
		}
		var pin struct {
			Provider string `json:"Provider"`
			Model    string `json:"Model"`
			Dim      int    `json:"Dim"`
			Digest   string `json:"model_digest"`
			Context  int    `json:"runtime_context_tokens"`
			Batch    int    `json:"runtime_batch_tokens"`
		}
		if json.Unmarshal(identity.EmbeddingIdentity, &pin) != nil || pin.Provider != expected.Provider || pin.Model != expected.Model || pin.Dim != expected.Dimension ||
			pin.Digest != expected.ModelDigest || pin.Context != expected.Context || pin.Batch != expected.Batch {
			return fail("dataset model differs from frozen protocol")
		}
		provider := cfg.Backends.CKV.Provider
		if provider == "" {
			provider = embedder.DefaultProvider
		}
		if provider != expected.Provider {
			return fail("configured provider differs from protocol")
		}
		if cfg.Retrieval.EffectiveRecallK() != expected.RetrievalK {
			return fail("configured retrieval K differs from protocol")
		}
		return locked, identity, nil
	}
}
