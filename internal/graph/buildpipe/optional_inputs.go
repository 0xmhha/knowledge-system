package buildpipe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// optional_inputs.go records which optional input files a build was given
// (the manifest's optional_inputs) and names each in the graph's rows by a
// path that does not depend on where the file sits on the build machine.
//
// The rule for the path the rows carry (Policy and SecurityPattern nodes,
// governed_by and has_security_pattern edges): the file's path relative to
// the source root, slash-separated, when the file is inside it; otherwise
// its base name and the start of its content hash ("graph.yaml@1a2b3c4d5e6f").
// The path exactly as given, and the full content hash, are in the manifest
// only. So two builds of one source with the same file, from different
// folders, write the same rows, and no build machine path reaches a row.

// Kinds of optional inputs.
const (
	inputPolicyFile          = "policy_file"
	inputSecurityPatternFile = "security_pattern_file"
)

// rowHashLen is how many hex digits of the content hash name a file
// outside the source root.
const rowHashLen = 12

// describeInput returns the manifest record of the optional input given
// (a path as given on the command line) for a build of srcRoot.
func describeInput(kind, given, srcRoot string) (persist.OptionalInput, error) {
	in := persist.OptionalInput{Kind: kind, Path: given}
	data, err := os.ReadFile(given)
	if err != nil {
		in.RowPath = filepath.Base(given)
		return in, fmt.Errorf("read %s %s: %w", kind, given, err)
	}
	sum := sha256.Sum256(data)
	in.SHA256 = hex.EncodeToString(sum[:])
	in.RowPath = rowPathOf(given, srcRoot, in.SHA256)
	return in, nil
}

// rowPathOf is the path the rows carry for the file given (see the rule at
// the top).
func rowPathOf(given, srcRoot, sha string) string {
	absFile, errFile := filepath.Abs(given)
	absRoot, errRoot := filepath.Abs(srcRoot)
	if errFile == nil && errRoot == nil {
		if rel, err := filepath.Rel(absRoot, absFile); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.Base(given) + "@" + sha[:rowHashLen]
}

// relocateRows sets the file path of the rows an optional input made to
// rowPath.
func relocateRows(nodes []types.Node, edges []types.Edge, rowPath string) {
	for i := range nodes {
		nodes[i].FilePath = rowPath
	}
	for i := range edges {
		edges[i].FilePath = rowPath
	}
}
