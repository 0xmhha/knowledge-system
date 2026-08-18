package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// maxSeedCandidates bounds how many candidates an ambiguity error names. The
// list exists so the caller can pick one without a second round trip; past a
// handful it stops being a shortcut and starts being the whole symbol table.
const maxSeedCandidates = 8

// seedCitation resolves a caller-supplied symbol to the single citation a
// traversal must start from, or returns the tool error explaining why it
// cannot.
//
// It exists because an unresolved seed passed on to an exact-match backend
// produces an empty result the caller cannot tell from a real "nothing here",
// and an ambiguous one produces a confident answer about the wrong symbol.
// Neither is recoverable downstream: the response carries no sign that the
// seed was never understood.
//
// This is the mcp-side half of a contract ckgclient.Real.resolveSeedOrErr
// holds for the traversals that resolve a name inside the client. The
// traversals that resolve here — the ones that need a Citation rather than a
// qname — go through this instead, and TestSeededToolsRefuseUnresolvedSymbols
// asserts that every symbol-seeded tool honours it whichever half it uses.
func seedCitation(ctx context.Context, d Deps, toolName, symbol string) (contract.Citation, *mcpgo.CallToolResult) {
	cits, err := d.CKG.FindSymbol(ctx, symbol, ckgclient.SymbolOpts{})
	if err != nil {
		return contract.Citation{}, mcpgo.NewToolResultErrorf("%s: resolve symbol: %v", toolName, err)
	}
	switch {
	case len(cits) == 0:
		return contract.Citation{}, mcpgo.NewToolResultErrorf(
			"%s: seed symbol unresolved: %q matches no indexed symbol", toolName, symbol)
	case len(cits) > 1:
		return contract.Citation{}, mcpgo.NewToolResultErrorf(
			"%s: seed symbol unresolved: %q is ambiguous across %d definitions; "+
				"pass the canonical_id %s returns. Candidates: %s",
			toolName, symbol, len(cits), ToolNameFindSymbol, formatSeedCandidates(cits))
	}
	return cits[0], nil
}

// formatSeedCandidates renders candidate definition sites for an ambiguity
// error. Sorted so the same ambiguity always reads the same way — the store's
// row order is not a ranking, and presenting it as one is what let the first
// candidate be mistaken for the answer.
func formatSeedCandidates(cits []contract.Citation) string {
	seen := make(map[string]struct{}, len(cits))
	out := make([]string, 0, len(cits))
	for _, c := range cits {
		s := fmt.Sprintf("%s:%d", c.File, c.StartLine)
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	if len(out) > maxSeedCandidates {
		return strings.Join(out[:maxSeedCandidates], ", ") +
			fmt.Sprintf(", and %d more", len(out)-maxSeedCandidates)
	}
	return strings.Join(out, ", ")
}
