package evidence

import (
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// rankingStore holds an older commit whose subject is the intent, two newer
// commits that share some of its words, and a commit outside the hunk
// window (a Commit node with changed_in edges but no hunks).
func rankingStore() *fakeStore {
	sha := func(c byte) string { return strings.Repeat(string(c), 40) }
	commit := func(id string, c byte, sig string) types.Node {
		return types.Node{ID: id, Type: types.NodeCommit, QualifiedName: "commit:" + sha(c), Signature: sig,
			Confidence: types.ConfExtracted}
	}
	hunk := func(id string, c byte, file string) types.Node {
		return types.Node{ID: id, Type: types.NodeHunk, QualifiedName: "hunk:" + sha(c) + ":" + file + ":0",
			FilePath: file, StartLine: 1, EndLine: 9, Confidence: types.ConfExtracted}
	}
	return &fakeStore{
		nodes: []types.Node{
			commit("cOld", 'a', "1700000100: fix: reject stale-view justifications in isJustified (#85)"),
			commit("cNew1", 'b', "1700000300: fix: reject block with nil GasTip in verifyHeader (#90)"),
			commit("cNew2", 'c', "1700000400: fix: harden WBFT backlog against future rounds (#91)"),
			commit("cOut", 'd', "1600000000: core: add txlookup lock (#21000)"),
			hunk("hOld", 'a', "consensus/wbft/core/justification.go"),
			hunk("hNew1", 'b', "consensus/wbft/verify.go"),
			hunk("hNew2", 'c', "consensus/wbft/core/backlog.go"),
			{ID: "fnLookup", Type: types.NodeFunction, QualifiedName: "core.BlockChain.writeTxLookup",
				FilePath: "core/blockchain.go", StartLine: 10, EndLine: 20, Confidence: types.ConfExtracted},
		},
		edges: []types.Edge{
			{Src: "fnLookup", Dst: "cOut", Type: types.EdgeChangedIn, Count: 1, Confidence: types.ConfExtracted},
		},
		blobs: map[string][]byte{
			"hOld":  gz("@@ -1,3 +1,4 @@\n-if justified(view) {\n+if justified(view) && !stale(view) {\n"),
			"hNew1": gz("@@ -1,3 +1,4 @@\n+if header.GasTip == nil { return errNilGasTip }\n reject\n"),
			"hNew2": gz("@@ -1,3 +1,4 @@\n+if round > current+maxFuture { return }\n harden reject\n"),
		},
	}
}

// TestBuildPack_ExactSubjectRanksItsCommitFirst: with a commit's subject as
// the intent, that commit comes first even when newer commits share words
// with it — commits are ordered by score, not by recency.
func TestBuildPack_ExactSubjectRanksItsCommitFirst(t *testing.T) {
	pack, err := BuildPack(rankingStore(), Options{Intent: "fix: reject stale-view justifications in isJustified", K: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Hits) == 0 || pack.Hits[0].Commit.SHA != strings.Repeat("a", 40) {
		t.Fatalf("top hit = %v, want the commit whose subject is the intent", commitSHAs(pack.Hits))
	}
	if !pack.Hits[0].InHunkWindow || len(pack.Hits[0].Hunks) != 1 {
		t.Errorf("the commit's hunks are missing: %+v", pack.Hits[0])
	}
}

// TestBuildPack_FindsCommitsOutsideTheHunkWindow: a commit whose hunks the
// graph does not hold is still found by its subject, and comes with the
// declarations it changed; the pack says what it searched.
func TestBuildPack_FindsCommitsOutsideTheHunkWindow(t *testing.T) {
	pack, err := BuildPack(rankingStore(), Options{Intent: "core: add txlookup lock", K: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Hits) == 0 || pack.Hits[0].Commit.SHA != strings.Repeat("d", 40) {
		t.Fatalf("top hit = %v, want the commit outside the hunk window", commitSHAs(pack.Hits))
	}
	h := pack.Hits[0]
	if h.InHunkWindow || h.ChangedTotal != 1 || len(h.Changed) != 1 || h.Changed[0].Qname != "core.BlockChain.writeTxLookup" {
		t.Errorf("hit outside the window = %+v, want in_hunk_window false and the changed declaration", h)
	}
	if pack.Searched.HunkCommits != 3 || pack.Searched.Commits != 4 || pack.Searched.Summary == "" {
		t.Errorf("searched = %+v, want 3 commits with hunks of 4 recorded", pack.Searched)
	}
}
