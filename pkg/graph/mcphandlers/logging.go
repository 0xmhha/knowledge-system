package mcphandlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/logx"
)

// maxLoggedArgs bounds the arguments kept in a tool call record; a task text
// or an intent can be long, and the record is for finding the call again,
// not for replaying it.
const maxLoggedArgs = 512

// LogToolCalls returns a tool handler middleware that writes one record per
// tool call to log (area "mcp"): a request id, the tool, its arguments, how
// long it took, the size of the result and how it ended. Without it the MCP
// server ran silently, so a wrong answer could not be traced to the call
// that produced it.
//
// Usage:
//
//	s := server.NewMCPServer(name, version, server.WithToolHandlerMiddleware(mcphandlers.LogToolCalls(log)))
func LogToolCalls(log *slog.Logger) server.ToolHandlerMiddleware {
	log = log.With(logx.AreaKey, "mcp")
	prefix := randomPrefix()
	var seq atomic.Uint64
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := fmt.Sprintf("%s-%d", prefix, seq.Add(1))
			start := time.Now()
			res, err := next(ctx, req)
			attrs := []any{
				"request_id", id, "tool", req.Params.Name,
				"arguments", loggedArgs(req.GetArguments()),
				"seconds", time.Since(start).Seconds(),
			}
			switch {
			case err != nil:
				log.Warn("tool call failed", append(attrs, "outcome", "error", "err", err.Error())...)
			case res != nil && res.IsError:
				log.Warn("tool call returned an error", append(attrs, "outcome", "tool_error", "result_bytes", resultBytes(res))...)
			default:
				log.Info("tool call", append(attrs, "outcome", "ok", "result_bytes", resultBytes(res))...)
			}
			return res, err
		}
	}
}

// randomPrefix tells the calls of one server process from another's in a
// shared log file.
func randomPrefix() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "mcp"
	}
	return hex.EncodeToString(b)
}

func loggedArgs(args map[string]any) string {
	raw, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("unencodable arguments: %v", err)
	}
	if len(raw) > maxLoggedArgs {
		return string(raw[:maxLoggedArgs]) + "...(cut)"
	}
	return string(raw)
}

// resultBytes is the encoded size of a result, -1 when it cannot be encoded.
func resultBytes(res *mcp.CallToolResult) int {
	if res == nil {
		return 0
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return -1
	}
	return len(raw)
}
