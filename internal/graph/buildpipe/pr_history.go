package buildpipe

import (
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// cleanPRTitle strips the canonical (#NNN) suffix and the
// "Merge pull request #NNN from …" prefix that GitHub injects on
// non-squash merges. Returns the original subject untouched when
// neither pattern matches.
func cleanPRTitle(subj string) string {
	if strings.HasPrefix(subj, "Merge pull request #") {
		// "Merge pull request #NNN from owner/branch" — the title we
		// want lives downstream of this commit in the body; without
		// it the subject is just noise. Return empty so the caller's
		// fallback (Summary first line, then the (#NNN) marker
		// itself) is the surface contract.
		return ""
	}
	return strings.TrimSpace(prSuffixRE.ReplaceAllString(subj, ""))
}

// prSuffixRE is the "(#N)" suffix a squash merge appends to the title.
var prSuffixRE = regexp.MustCompile(`\(#\d+\)`)

// bodyExcerptMaxBytes caps the persisted Summary so a runaway PR
// template (CI logs pasted into the description, multi-MB changelogs)
// can't bloat node_prs row size beyond what CKV's semantic search
// usefully consumes. 2 KB comfortably holds the typical "왜 이 변경"
// paragraph plus an acceptance-criteria list; beyond that the excerpt
// is truncated at the nearest line boundary with an ellipsis marker.
const bodyExcerptMaxBytes = 2048

// bodyExcerpt returns the "why" portion of a commit body — the
// multi-line description sandwiched between the subject (already
// captured in Title) and any trailing git trailers (Signed-off-by:,
// Co-authored-by:, etc.). Previously this field
// captured only the first non-empty body line, which dropped the bulk
// of the rationale that CKV's semantic search depends on.
//
// Transformations, in order:
//  1. Right-trim trailing whitespace on every line.
//  2. Drop lines matching the well-known git trailer set so attribution
//     and review metadata don't leak into the "왜" text.
//  3. Trim leading and trailing blank lines.
//  4. Cap at bodyExcerptMaxBytes; truncation walks back to the
//     previous newline so the excerpt always ends on a clean line
//     and appends "…" as a clipping marker.
//
// Empty body → "". A body that is *entirely* trailers (no real
// description — rare but possible on squash merges with no PR
// description) also returns "" so consumers can fall back to Title.
func bodyExcerpt(body string) string {
	if body == "" {
		return ""
	}
	keep := make([]string, 0, 16)
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimRight(line, " \t\r")
		if isTrailerLine(trimmed) {
			continue
		}
		keep = append(keep, trimmed)
	}
	for len(keep) > 0 && strings.TrimSpace(keep[0]) == "" {
		keep = keep[1:]
	}
	for len(keep) > 0 && strings.TrimSpace(keep[len(keep)-1]) == "" {
		keep = keep[:len(keep)-1]
	}
	if len(keep) == 0 {
		return ""
	}
	out := strings.Join(keep, "\n")
	if len(out) <= bodyExcerptMaxBytes {
		return out
	}
	cut := strings.LastIndexByte(out[:bodyExcerptMaxBytes], '\n')
	if cut <= 0 {
		cut = bodyExcerptMaxBytes
	}
	return out[:cut] + "\n…"
}

// isTrailerLine reports whether line is a recognised git trailer that
// should be excluded from the "왜" excerpt. The set is intentionally
// conservative — only the exact, canonical prefixes from common
// workflow tooling (git-interpret-trailers, GitHub web UI, common
// Claude/Anthropic attribution lines) qualify. A prose line that
// happens to contain "Reviewed: foo" without the canonical capitalised
// form and trailing colon is kept.
func isTrailerLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "Signed-off-by:"),
		strings.HasPrefix(line, "Co-authored-by:"),
		strings.HasPrefix(line, "Acked-by:"),
		strings.HasPrefix(line, "Reviewed-by:"),
		strings.HasPrefix(line, "Tested-by:"),
		strings.HasPrefix(line, "Reported-by:"),
		strings.HasPrefix(line, "Suggested-by:"),
		strings.HasPrefix(line, "Cc:"):
		return true
	}
	return strings.HasPrefix(line, "Generated with ")
}

// shallowBoundaries returns the commits at which a shallow clone's history
// was cut, or nil when srcRoot is not a shallow clone.
func shallowBoundaries(srcRoot string) map[string]bool {
	cmd := exec.Command("git", "-C", srcRoot, "rev-parse",
		"--path-format=absolute", "--git-path", "shallow")
	stdout, err := cmd.Output()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(strings.TrimSpace(string(stdout)))
	if err != nil {
		return nil // no shallow file: full history
	}
	out := map[string]bool{}
	for _, sha := range strings.Fields(string(data)) {
		out[sha] = true
	}
	return out
}

// scanGitRemoteRepo runs `git config --get remote.origin.url` and
// reduces the URL to "owner/name". Returns "" on any failure — Repo
// is optional metadata.
//
// Accepts both HTTPS (https://github.com/owner/name.git) and SSH
// (git@github.com:owner/name.git) forms. Strips the .git suffix.
func scanGitRemoteRepo(srcRoot string) string {
	cmd := exec.Command("git", "-C", srcRoot, "config", "--get", "remote.origin.url")
	stdout, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(stdout))
	url = strings.TrimSuffix(url, ".git")
	if _, tail, ok := strings.Cut(url, "github.com"); ok {
		tail = strings.TrimPrefix(tail, "/")
		tail = strings.TrimPrefix(tail, ":")
		return tail
	}
	return ""
}

// dedupePRRefsByNode collapses duplicate PR rows (a pull request whose
// commits changed a node more than once), keeping the first, newest, one. The
// PRIMARY KEY (node_id, number) in node_prs would otherwise force
// INSERT OR REPLACE to thrash on duplicates. Sort by MergedAtUTC
// descending so the SQL writer can rely on a stable order for
// pagination.
func dedupePRRefsByNode(byNode map[string][]types.PRRef) {
	for id, refs := range byNode {
		seen := map[int]bool{}
		out := refs[:0]
		for _, r := range refs {
			if seen[r.Number] {
				continue
			}
			seen[r.Number] = true
			out = append(out, r)
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].MergedAtUTC.After(out[j].MergedAtUTC)
		})
		byNode[id] = out
	}
}
