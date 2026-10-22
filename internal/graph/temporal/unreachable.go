// Package temporal — unreachable.go collects hunks from commits that
// exist in the local git object store but are NOT reachable from HEAD.
// Those commits live in two surfaces:
//
//  1. `git reflog --all --pretty=%H`        — local HEAD/branch movement
//     records. Captures force-pushed-away SHAs that haven't been GC'd
//     yet (default 90 days). Misses commits that landed via fetch but
//     were never moved into a ref's history.
//
//  2. `git fsck --no-reflogs --unreachable` — dangling objects that
//     no ref or reflog points at. Catches the second category above
//     and any commit explicitly excluded from a fetch's tip walk.
//
// Together: a near-complete view of the local object store's
// "history humans rolled back". Used to populate the schema-1.8
// §11.3 "AMBIGUOUS" hunk class — see docs/graph/design/hunk-graph.md
// for the storage / retrieval / recovery layering.
//
// Distinct from LoadHunks: that pass walks `git log HEAD --` only and
// produces the EXTRACTED-confidence baseline. This pass is additive —
// the caller merges the two result sets into one Commit/Hunk emission.
package temporal

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/githistory"
)

// LoadUnreachableHunks returns commits + hunks for SHAs reachable via
// reflog or fsck-unreachable but NOT from HEAD. maxCommits ≤ 0 uses
// unreachableCommitsDefault.
//
// Returns (nil, nil, nil) for non-git directories — same graceful
// degrade contract as LoadHunks. Other git failures bubble up.
//
// Performance: reflog + fsck typically run in < 1s on repos with
// 10K+ commits. The per-SHA `git show` is ~50ms each, so a 100-commit
// cap keeps the worst-case under 5s on commodity hardware.
func LoadUnreachableHunks(repoRoot string, maxCommits int) ([]CommitInfo, []HunkInfo, error) {
	unreachable, err := githistory.RecoveryCommitIDs(repoRoot, maxCommits)
	if err != nil {
		return nil, nil, err
	}
	if len(unreachable) == 0 {
		return nil, nil, nil
	}
	var commits []CommitInfo
	var hunks []HunkInfo
	for _, sha := range unreachable {
		ci, hs, err := loadCommitWithHunks(repoRoot, sha)
		if err != nil {
			// Skip individual failures — a SHA might fail because of
			// shallow-clone truncation or a transient git error. The
			// rest of the unreachable set is still useful.
			continue
		}
		commits = append(commits, ci)
		hunks = append(hunks, hs...)
	}
	return commits, hunks, nil
}

// loadCommitWithHunks streams `git show <sha>` through parseHunkStream —
// the same parser the HEAD-walk uses, so binary-files / rename /
// mode-only / multi-hunk handling is identical between the EXTRACTED and
// AMBIGUOUS paths. The commit header comes from a separate header-only
// `git show -s`, because the stream parser does not expose it.
func loadCommitWithHunks(repoRoot, sha string) (CommitInfo, []HunkInfo, error) {
	header, err := exec.Command("git", "-C", repoRoot,
		"show", "-s", "--no-color", "--pretty=format:"+commitHeaderFormat, sha).Output()
	if err != nil {
		return CommitInfo{}, nil, fmt.Errorf("git show -s %s: %w", sha, err)
	}
	hunks, err := streamHunks(exec.Command("git", "-C", repoRoot,
		"show", "--no-color", "--no-renames",
		"--pretty=format:"+commitHeaderFormat,
		"--unified=3", sha), "git show "+sha)
	if err != nil {
		return CommitInfo{}, nil, err
	}
	info, ok := parseCommitHeader(strings.TrimSpace(string(header)))
	if !ok {
		// Fallback: at least populate the SHA so downstream Commit-node
		// emission works. Timestamp/Subject stay zero — viewers can
		// still display a "(unreachable)" placeholder.
		info = CommitInfo{SHA: sha}
	}
	return info, hunks, nil
}
