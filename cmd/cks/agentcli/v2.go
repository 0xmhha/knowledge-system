package agentcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	sysmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// V2Options forwards the explicitly selected options of the existing v2 tool.
// It never changes the server's ontology mode or approves retrieved knowledge.
type V2Options struct {
	Intent             string
	ExplainOntology    bool
	IncludeKnowledge   bool
	KnowledgeAsOf      string
	KnowledgeSubsystem string
}

type v2Error string

func (e v2Error) Error() string      { return string(e) }
func (e v2Error) PublicCode() string { return string(e) }

func (o V2Options) empty() bool { return o == (V2Options{}) }

func (o V2Options) validate() error {
	if o.Intent != "" {
		if _, ok := contract.ParseIntent(o.Intent); !ok {
			return v2Error("invalid_request")
		}
	}
	if !o.IncludeKnowledge {
		if o.KnowledgeAsOf != "" || o.KnowledgeSubsystem != "" {
			return v2Error("invalid_knowledge_scope")
		}
		return nil
	}
	if _, err := time.Parse("2006-01-02", o.KnowledgeAsOf); err != nil || strings.TrimSpace(o.KnowledgeSubsystem) == "" {
		return v2Error("invalid_knowledge_scope")
	}
	return nil
}

func publicV2CallError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return v2Error("request_canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return v2Error("request_timeout")
	default:
		return v2Error("service_unavailable")
	}
}

// FetchV2 verifies the published v2 DTO without reducing it to the v1 mirror.
// The caller owns the connection and must close it before publishing output.
func (a *Agent) FetchV2(ctx context.Context, prompt string, opts V2Options) (contract.EvidencePackV2, error) {
	var pack contract.EvidencePackV2
	if ctx == nil || strings.TrimSpace(prompt) == "" {
		return pack, v2Error("invalid_request")
	}
	if err := opts.validate(); err != nil {
		return pack, err
	}
	if a == nil || a.closed || a.client == nil {
		return pack, v2Error("service_unavailable")
	}
	args := map[string]any{"prompt": prompt}
	if opts.Intent != "" {
		args["intent"] = opts.Intent
	}
	if opts.ExplainOntology {
		args["explain_ontology"] = true
	}
	if opts.IncludeKnowledge {
		args["include_knowledge"] = true
		args["knowledge_as_of"] = opts.KnowledgeAsOf
		args["knowledge_subsystem"] = opts.KnowledgeSubsystem
	}
	req := mcpgo.CallToolRequest{}
	req.Params.Name, req.Params.Arguments = sysmcp.ToolNameGetForTaskV2, args
	result, err := a.client.CallTool(ctx, req)
	if err != nil {
		return pack, publicV2CallError(err)
	}
	if result == nil {
		return pack, v2Error("v2_evidence_failed")
	}
	if result.IsError {
		var data struct {
			Code string `json:"code"`
		}
		if raw, err := json.Marshal(result.StructuredContent); err == nil && json.Unmarshal(raw, &data) == nil {
			switch data.Code {
			case "embedding_failed", "request_canceled", "request_timeout", "retrieval_failed", "budget_exceeded", "requires_v2", "reindex_required", "snapshot_mismatch", "source_missing", "invalid_request", "service_unavailable", "compose_failed", "v2_evidence_failed", "invalid_knowledge_scope", "knowledge_context_failed":
				return pack, v2Error(data.Code)
			}
		}
		return pack, v2Error("operation_failed")
	}
	var raw []byte
	if result.StructuredContent != nil {
		raw, err = json.Marshal(result.StructuredContent)
	} else {
		raw = []byte(concatText(result))
	}
	if err != nil || len(raw) == 0 {
		return pack, v2Error("v2_evidence_failed")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(&pack); err != nil {
		return pack, v2Error("v2_evidence_failed")
	}
	if decoder.Decode(new(any)) != io.EOF || pack.Query != prompt || evidencev2.Verify(pack) != nil {
		return contract.EvidencePackV2{}, v2Error("v2_evidence_failed")
	}
	return pack, nil
}

func deliverV2(ctx context.Context, agent *Agent, prompt string, opts V2Options, output string, stdout io.Writer) (err error) {
	defer func() {
		if closeErr := agent.Close(); closeErr != nil {
			err = errors.Join(err, publicV2CallError(closeErr))
		}
	}()
	pack, err := agent.FetchV2(ctx, prompt, opts)
	if err != nil {
		return err
	}
	data, err := json.Marshal(pack)
	if err != nil {
		return v2Error("v2_evidence_failed")
	}
	data = append(data, '\n')
	// A failed or still-running owned server must not publish a successful file.
	if err := agent.Close(); err != nil {
		return publicV2CallError(err)
	}
	if err := ctx.Err(); err != nil {
		return publicV2CallError(err)
	}
	if output != "" {
		return writeV2Output(ctx, output, data)
	}
	n, err := stdout.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

// writeV2Output publishes a complete new file only after write/sync/close.
// Linking a same-directory temporary file preserves existing destinations.
func writeV2Output(ctx context.Context, output string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return publicV2CallError(err)
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".cks-context-v2-*")
	if err != nil {
		return v2Error("operation_failed")
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return v2Error("operation_failed")
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return v2Error("operation_failed")
	}
	if err := f.Close(); err != nil {
		return v2Error("operation_failed")
	}
	if err := ctx.Err(); err != nil {
		return publicV2CallError(err)
	}
	if err := os.Link(name, output); err != nil {
		if errors.Is(err, os.ErrExist) {
			return v2Error("output_exists")
		}
		return v2Error("operation_failed")
	}
	if err := os.Remove(name); err != nil {
		return v2Error("durability_uncertain")
	}
	parent, err := os.Open(filepath.Dir(output))
	if err != nil {
		return v2Error("durability_uncertain")
	}
	err = parent.Sync()
	closeErr := parent.Close()
	if err != nil || closeErr != nil {
		return v2Error("durability_uncertain")
	}
	return nil
}
