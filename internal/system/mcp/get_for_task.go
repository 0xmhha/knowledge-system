package mcp

import (
	"context"
	"errors"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/internal/system/composer"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// handleGetForTask is the standalone handler for cks.context.get_for_task.
// Splitting it from registerGetForTask lets tests exercise the body
// without driving the mcp-go AddTool plumbing.
//
// Error convention: protocol-level problems (transport, marshaling) return
// a Go error. Domain problems (missing prompt, composer failure,
// fail_closed) come back as an IsError CallToolResult — that's what mcp-go
// surfaces to MCP clients as the "tool produced an error" branch.
func handleGetForTask(ctx context.Context, d Deps, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	prompt := req.GetString("prompt", "")
	if prompt == "" {
		return mcpgo.NewToolResultError("cks.context.get_for_task: missing required argument \"prompt\""), nil
	}

	// Optional caller-supplied intent. Callers that know their task's
	// intent (pipeline stages, the eval harness) should pass it: the
	// internal classifier degrades to Unknown on unrecognized prompts,
	// silently disabling every intent-conditioned retrieval stage. An
	// invalid value fails loud rather than silently reclassifying.
	callerIntent := contract.IntentUnknown
	if raw := req.GetString("intent", ""); raw != "" {
		parsed, ok := contract.ParseIntent(raw)
		if !ok {
			return mcpgo.NewToolResultError(fmt.Sprintf(
				"cks.context.get_for_task: unknown intent %q (valid: %v)", raw, contract.AllIntents())), nil
		}
		callerIntent = parsed
	}

	// Fail loud when the backend is not serviceable. A ckv-down ("degraded")
	// or fully down state cannot produce the semantic context an upper-layer
	// LLM needs to design correctly; returning a ckg-only pack here would let
	// the caller act on silently degraded evidence. Refuse with an actionable
	// reason so the caller waits for / provisions ckv instead of proceeding.
	if ok, reason := serviceable(ctx, d); !ok {
		return mcpgo.NewToolResultError(fmt.Sprintf(
			"cks.context.get_for_task: service unavailable — %s. "+
				"ckv semantic retrieval is required for design-grade context; "+
				"wait for ckv to become ready or provision it, then retry.", reason)), nil
	}

	pack, err := d.Composer.ComposeWithIntent(ctx, prompt, callerIntent)
	if err != nil {
		// Fail-closed is a policy outcome the caller needs to distinguish
		// from a transient compose failure. Wrap the rule id in the text
		// so the MCP client can branch on it.
		if errors.Is(err, composer.ErrFailClosed) {
			return mcpgo.NewToolResultError(fmt.Sprintf("cks.context.get_for_task: %v", err)), nil
		}
		return mcpgo.NewToolResultErrorf("cks.context.get_for_task: %v", err), nil
	}

	// NewToolResultStructured emits both a JSON-encoded text fallback
	// (for callers that ignore structured content) and a structured
	// payload (for callers that decode it). The pack itself carries the
	// integrity hash; the wire envelope adds nothing.
	return mcpgo.NewToolResultStructured(pack, "evidence pack"), nil
}
