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
// §11.3 "AMBIGUOUS" hunk class — see docs/design/hunk-graph.md
// for the storage / retrieval / recovery layering.
//
// Distinct from LoadHunks: that pass walks `git log HEAD --` only and
// produces the EXTRACTED-confidence baseline. This pass is additive —
// the caller merges the two result sets into one Commit/Hunk emission.
package temporal

import (
	"bufio"
	"bytes"
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

// loadCommitWithHunks runs `git show <sha>` and parses the result via
// parseHunkStream — same parser the HEAD-walk uses, so binary-files /
// rename / mode-only / multi-hunk handling is identical between the
// EXTRACTED and AMBIGUOUS paths.
func loadCommitWithHunks(repoRoot, sha string) (CommitInfo, []HunkInfo, error) {
	cmd := exec.Command("git", "-C", repoRoot,
		"show", "--no-color", "--no-renames",
		"--pretty=format:COMMIT %H %at %s",
		"--unified=3", sha)
	out, err := cmd.Output()
	if err != nil {
		return CommitInfo{}, nil, fmt.Errorf("git show %s: %w", sha, err)
	}
	hunks, err := parseHunkStream(bytes.NewReader(out))
	if err != nil {
		return CommitInfo{}, nil, fmt.Errorf("parse %s: %w", sha, err)
	}
	// Extract the COMMIT header line for the timestamp + subject. The
	// stream parser uses parseCommitHeader internally for the same job —
	// we re-scan here because the parser doesn't expose the parsed info
	// at the per-stream-segment level (it's stateful inside the hunk
	// loop).
	var info CommitInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "COMMIT ") {
			if ci, ok := parseCommitHeader(line); ok {
				info = ci
			}
			break
		}
	}
	if info.SHA == "" {
		// Fallback: at least populate the SHA so downstream Commit-node
		// emission works. Timestamp/Subject stay zero — viewers can
		// still display a "(unreachable)" placeholder.
		info.SHA = sha
	}
	return info, hunks, nil
}
