package mcphandlers

import (
	"context"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/concurrency"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

// RegisterConcurrencyImpact wires the concurrency_impact tool — the
// concurrency blast radius of a symbol over the five contract edge types
// (spawns, sends_to, recvs_from, acquires_lock, accessed_under_lock).
// The algorithm body lives in pkg/concurrency so cks consumes it
// in-process; this file is the MCP request envelope only (the dev-only
// ckg server), mirroring RegisterImpactOfChange.
func RegisterConcurrencyImpact(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("concurrency_impact",
		mcp.WithDescription(
			"Use for data races, deadlocks and goroutine, channel or lock questions. "+
				"Concurrency blast radius for a symbol: modules that affect or are affected by it via "+
				"goroutine/channel/lock edges (spawns, sends_to, recvs_from, acquires_lock, accessed_under_lock). "+
				"Seed a Channel/Mutex/Field node to recover producer<->consumer or lock-sharing peers; a function "+
				"seed reaches its own goroutine and the channel it writes, but not the peer across the channel. "+
				"The sends and receives of goroutines on the channels reached are listed apart under goroutines (goroutine, edge, channel, file_path, line) and are not walked through: modules and edges are the function-level answer. "+
				"A mutex is seeded by the qualified name of the field or variable that declares it, with or without \"#mutex\" after it, the same for a struct field, a package-level variable and a local variable "+
				"(\"pkg.T.mu\" or \"pkg.T.mu#mutex\", \"pkg.mu\" or \"pkg.mu#mutex\", \"pkg.F.mu\" or \"pkg.F.mu#mutex\"), and both forms give one answer: "+
				"mutex names the Mutex node and declared_by the field or variable that declares it; locked_by lists the functions that lock this mutex, one per lock line; "+
				"guarded_by lists the other mutexes held where the declaring field or variable itself is accessed; modules and edges are the walk from the Mutex node (the functions that lock it, the fields it guards, and on). "+
				"A channel used as a lock (chan struct{} with a buffer of one, filled where it is made: a receive takes its token, a send returns it) is a mutex too: seed it by the field or variable that holds it, and mutex names its Channel node (sub_kind lock). "+
				"A name that one function declares twice (two blocks) is ambiguous; its candidates name each declaration as \"<qname>@<file>:<line>\", which is a seed too. "+
				"seed_qname may be a canonical id, a qualified name or a bare name; an ambiguous bare name returns its candidates.",
		),
		mcp.WithString("seed_qname"),
		mcp.WithNumber("depth", mcp.DefaultNumber(2)),
		mcp.WithNumber("max_total", mcp.DefaultNumber(0)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := concurrency.Analyze(reader, req.GetString("seed_qname", ""), concurrency.Options{
			Depth:    int(req.GetFloat("depth", 2)),
			MaxTotal: int(req.GetFloat("max_total", 0)),
		})
		if err != nil {
			return nil, err
		}
		return textResult(out), nil
	})
}
