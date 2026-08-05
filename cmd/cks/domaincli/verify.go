// Command cks-entry-verify promotes a single domain-knowledge entry to
// status: verified and refreshes the inventory.md count tables in one
// atomic-feeling operation.
//
// The promotion writes exactly three fields back into the entry YAML:
//
//	status:           verified
//	last_verified_at: <-date, default today>
//	verified_by:      <-by>
//
// All other fields, ordering, comments, and multi-line literal styles
// are preserved. The rewrite goes through yaml.Node mutation, not a
// struct re-marshal — see internal/inventory/verify.go for details.
//
// Before writing, the entry is simulated through ValidateEntry to make
// sure the verified transition passes every mechanical check (anchors
// exist, cross-references resolve, etc.). If validation fails, no file
// is touched and the issues print to stderr; the operator fixes the
// underlying problem in the entry YAML, then re-runs.
//
// Usage:
//
//	cks-entry-verify \
//	    -project projects/stablenet/domain-knowledge \
//	    -entry   A1.wbft_core.quorum_calc \
//	    -by      mhha
//
// Optional flags:
//
//	-date 2026-05-29    explicit verification date (defaults to today UTC)
//	-skip-inventory     do not rewrite <project>/inventory.md afterwards
//
// Exit codes:
//   - 0: entry promoted, inventory.md updated.
//   - 1: validation failed; entry unchanged.
//   - 2: usage error (missing flag, entry not found, IO error).
package domaincli

import (
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/system/inventory"
)

// resolveEntry accepts either an entry ID (as it appears in entries/*.yaml)
// or a path to an entry YAML file, and returns the canonical entry ID
// plus the absolute source path. Paths are resolved to absolute and
// matched against each entry's SourcePath so that callers can pass
// either form interchangeably.
func resolveEntry(p *inventory.Project, ref string) (id, path string, ok bool) {
	if e, found := p.Entries[ref]; found {
		return e.ID, e.SourcePath, true
	}
	abs, err := filepath.Abs(ref)
	if err != nil {
		return "", "", false
	}
	for _, e := range p.Entries {
		if e.SourcePath == abs {
			return e.ID, e.SourcePath, true
		}
	}
	return "", "", false
}
