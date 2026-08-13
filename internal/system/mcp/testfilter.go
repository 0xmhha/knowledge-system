package mcp

import (
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"github.com/0xmhha/knowledge-system/pkg/system/testpath"
)

// excludeTestsArg reads the optional exclude_tests boolean. When true the
// tool drops test and test-only support citations from its result so the
// caller gets production code only. Defaults to false (backward compatible).
func excludeTestsArg(req mcpgo.CallToolRequest) bool {
	return req.GetBool("exclude_tests", false)
}

// withExcludeTests is the standard tool option documenting the flag.
func withExcludeTests() mcpgo.ToolOption {
	return mcpgo.WithBoolean("exclude_tests",
		mcpgo.Description("Drop test files and test-only helpers (testutil*.go, test/ dirs, *_test.go, *.t.sol, …) from the result. Use for design/implementation queries where tests are noise. Default false."))
}

// withExpand is the standard tool option documenting the glossary-expansion flag.
func withExpand() mcpgo.ToolOption {
	return mcpgo.WithBoolean("expand",
		mcpgo.Description("Expand the query with project glossary concept→symbol mappings before searching, the same step get_for_task uses. Helps natural-language/domain queries reach the right code when plain similarity drifts to generic infrastructure. Default false."))
}

// maybeExpand returns the query expanded with glossary code-keywords when
// the caller set expand=true and a resolver is wired. Otherwise it returns
// the query unchanged. The expanded string is appended (never replaces the
// original terms), mirroring the composer's Stage 1 behaviour.
func maybeExpand(d Deps, req mcpgo.CallToolRequest, query string) string {
	if !req.GetBool("expand", false) || d.Vocab == nil {
		return query
	}
	return d.Vocab.Resolve(query).Expanded
}

// filterCitationsTests returns citations that are not test paths.
func filterCitationsTests(cits []contract.Citation) []contract.Citation {
	out := make([]contract.Citation, 0, len(cits))
	for _, c := range cits {
		if testpath.IsTest(c.File) {
			continue
		}
		out = append(out, c)
	}
	return out
}
