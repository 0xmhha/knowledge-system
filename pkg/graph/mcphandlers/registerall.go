package mcphandlers

import (
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/evidence"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

// RegisterAll wires every ckg tool to s in one call. Equivalent to
// invoking each Register* function individually plus
// [RegisterEvidenceForIntent] with a fresh [evidence.NewCache].
//
// The §11.3 retrieval boundary is enforced inside every Register*
// (each wraps its reader via safeReader, idempotently), so it holds for the
// whole tool set here and for any individual handler mounted on its own —
// callers can no longer forget to wrap. Callers that need a different
// lifecycle (one cache across multiple servers, a partial tool subset) can
// still compose the individual Register* calls directly.
//
// Existing callers within this module use this to keep server.go
// down to ~10 lines; sister-repo wiring (cks / ckv) is recommended
// to use this too unless there's a specific reason not to.
func RegisterAll(s *server.MCPServer, reader store.Reader) {
	RegisterFindSymbol(s, reader)
	RegisterFindCallers(s, reader)
	RegisterFindCallees(s, reader)
	RegisterFindReferences(s, reader)
	RegisterGetSubgraph(s, reader)
	RegisterSearchText(s, reader)
	RegisterSearchCode(s, reader)
	RegisterInvestigate(s, reader)
	RegisterGetContextForTask(s, reader)
	RegisterImpactOfChange(s, reader)
	RegisterConcurrencyImpact(s, reader)
	RegisterValueDependencies(s, reader)
	RegisterRuleCandidates(s, reader)
	RegisterChangeHistory(s, reader)
	RegisterExecutionFlow(s, reader)
	// One Cache per Run amortises BM25 corpus indexing across every
	// evidence_for_intent call. Manifest-keyed invalidation handles
	// the case where a concurrent `ckg build` rebuilds the graph.
	RegisterEvidenceForIntent(s, reader, evidence.NewCache())
	// The wrappers apply to every tool registered above, all 18. They nest
	// so that a call meets, in order: the argument check (arguments.go),
	// then the handler, whose answer the echo cut (echo.go) bounds after
	// the handler shaped it by content mode (content.go). The check runs
	// first so a call with a missing or unknown argument gets that error
	// before any work; the echo cut wraps the handler directly so the
	// handler's errors, which the check turns into error results, are cut
	// too. boundEchoes must therefore wrap the handlers before
	// requireDeclaredArguments wraps the result.
	boundEchoes(s)
	requireDeclaredArguments(s)
}
