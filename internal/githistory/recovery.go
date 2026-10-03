// Package githistory selects Git commits used by the recovery graph and by
// pinned source identity. It has no dependency on the graph or setup engines.
package githistory

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Bound cold builds in repositories with many GC-pending commits. CKG and
// CKS must use this same limit for the graph input and its digest.
const defaultRecoveryCommits = 100

// RecoveryCommitIDs returns the sorted commits visible through local reflogs
// or fsck but unreachable from HEAD. The same cap and selection must be used
// by CKG and CKS so a pinned identity describes its recovery graph input.
// Non-Git roots have no recovery input; a failed Git inventory is an error.
func RecoveryCommitIDs(repoRoot string, maxCommits int) ([]string, error) {
	if maxCommits <= 0 {
		maxCommits = defaultRecoveryCommits
	}
	if out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--show-toplevel").Output(); err != nil || strings.TrimSpace(string(out)) == "" {
		return nil, nil
	}
	reachable, err := reachableSet(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("rev-list HEAD: %w", err)
	}
	reflog, err := reflogSHAs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("read Git reflog: %w", err)
	}
	fsck, err := fsckUnreachable(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("read unreachable Git objects: %w", err)
	}
	for sha := range fsck {
		reflog[sha] = struct{}{}
	}
	var selected []string
	for sha := range reflog {
		if _, hit := reachable[sha]; !hit {
			selected = append(selected, sha)
		}
	}
	sort.Strings(selected)
	if len(selected) > maxCommits {
		selected = selected[:maxCommits]
	}
	return selected, nil
}

func reachableSet(root string) (map[string]struct{}, error) {
	out, err := exec.Command("git", "-C", root, "rev-list", "HEAD", "--").Output()
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, 4096)
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		sha := strings.TrimSpace(string(line))
		if len(sha) == 40 {
			set[sha] = struct{}{}
		}
	}
	return set, nil
}

func reflogSHAs(root string) (map[string]struct{}, error) {
	out, err := exec.Command("git", "-C", root, "reflog", "--all", "--pretty=%H").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git reflog: %w: %s", err, out)
	}
	set := map[string]struct{}{}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		sha := strings.TrimSpace(string(line))
		if len(sha) == 40 {
			set[sha] = struct{}{}
		}
	}
	return set, nil
}

func fsckUnreachable(root string) (map[string]struct{}, error) {
	// Git versions differ in which stream carries fsck findings.
	out, err := exec.Command("git", "-C", root, "fsck", "--no-reflogs", "--unreachable").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git fsck: %w: %s", err, out)
	}
	set := map[string]struct{}{}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		fields := strings.Fields(string(line))
		if len(fields) == 3 && (fields[0] == "unreachable" || fields[0] == "dangling") &&
			fields[1] == "commit" && len(fields[2]) == 40 {
			set[fields[2]] = struct{}{}
		}
	}
	return set, nil
}
