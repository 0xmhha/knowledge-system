// Package temporal reads the git history of a source tree for the graph's
// history facts: the line history and blame of files and declarations
// (TrackLines), the hunks of recent commits (LoadHunks), the hunks of
// commits no longer reachable from HEAD (LoadUnreachableHunks), and the
// pull request numbers commit subjects name (PRNumber, NumberSpaces).
//
// Repos that aren't git checkouts return empty results and a nil error so
// callers degrade gracefully.
package temporal

import (
	"os/exec"
	"strconv"
	"strings"
)

// CommitInfo describes a single git commit as the hunk readers see it.
// Timestamp is unix seconds (the git `%at` author time).
type CommitInfo struct {
	SHA       string // full 40-char hex
	Timestamp int64  // unix seconds
	Subject   string // the whole subject line
}

// isGitCheckout returns true when `git -C dir rev-parse --show-toplevel`
// succeeds. Mirrors gitRepoRel in buildpipe but avoids the import cycle.
func isGitCheckout(dir string) bool {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// commitHeaderFormat is the --pretty format the hunk readers use;
// parseCommitHeader reads the line it produces.
const commitHeaderFormat = "COMMIT %H %at %s"

// parseCommitHeader parses one `COMMIT <sha> <ts> <subject>` line, keeping
// the subject whole. Returns (info, true) on success, (zero, false) on any
// malformed input.
func parseCommitHeader(line string) (CommitInfo, bool) {
	rest := strings.TrimPrefix(line, "COMMIT ")
	sp := strings.IndexByte(rest, ' ')
	if sp <= 0 {
		return CommitInfo{}, false
	}
	sha := rest[:sp]
	if len(sha) != shaHexLen {
		return CommitInfo{}, false
	}
	rest = rest[sp+1:]
	sp = strings.IndexByte(rest, ' ')
	if sp <= 0 {
		// Subject can legally be empty; in that case we have only "<sha> <ts>".
		ts, err := strconv.ParseInt(rest, 10, 64)
		if err != nil {
			return CommitInfo{}, false
		}
		return CommitInfo{SHA: sha, Timestamp: ts}, true
	}
	ts, err := strconv.ParseInt(rest[:sp], 10, 64)
	if err != nil {
		return CommitInfo{}, false
	}
	return CommitInfo{SHA: sha, Timestamp: ts, Subject: rest[sp+1:]}, true
}
