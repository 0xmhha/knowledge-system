package temporal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// prSuffix is GitHub's squash-merge "Title (#N)" suffix; prMerge its
// merge-commit subject "Merge pull request #N from owner/branch".
var (
	prSuffix = regexp.MustCompile(`\(#(\d+)\)`)
	prMerge  = regexp.MustCompile(`^Merge pull request #(\d+) `)
)

// SubjectNumbers are the pull request numbers a commit subject names.
//   - Own: the pull request that merged the commit — the last "(#N)" (GitHub
//     appends it when it squashes) or the N of "Merge pull request #N"; 0
//     when the subject names none.
//   - Ported: the earlier "(#N)" of the subject, oldest first: a port keeps
//     the number of the change it carries over.
//   - Reverts: the "(#N)" inside the quoted title of a `Revert "…"` subject,
//     the pull requests the commit undoes.
//
// A cross-repository reference such as "(ethereum#29563)" is none of them.
type SubjectNumbers struct {
	Own     int
	Ported  []int
	Reverts []int
}

// revertPrefix starts the subject git revert writes: Revert "<subject>".
const revertPrefix = `Revert "`

// PRNumbers reads the pull request numbers of a commit subject.
func PRNumbers(subject string) SubjectNumbers {
	if m := prMerge.FindStringSubmatch(subject); len(m) == 2 {
		own, _ := strconv.Atoi(m[1])
		return SubjectNumbers{Own: own}
	}
	var out SubjectNumbers
	rest := subject
	if strings.HasPrefix(subject, revertPrefix) {
		if end := strings.LastIndexByte(subject, '"'); end > len(revertPrefix) {
			out.Reverts = suffixNumbers(subject[len(revertPrefix):end])
			rest = subject[end+1:]
		}
	}
	nums := suffixNumbers(rest)
	if len(nums) > 0 {
		out.Own = nums[len(nums)-1]
		if len(nums) > 1 {
			out.Ported = nums[:len(nums)-1]
		}
	}
	return out
}

// suffixNumbers lists the "(#N)" numbers of s in order.
func suffixNumbers(s string) []int {
	var nums []int
	for _, m := range prSuffix.FindAllStringSubmatch(s, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			nums = append(nums, n)
		}
	}
	return nums
}

// PRNumber returns the number of the pull request that merged a commit
// (PRNumbers' Own), 0 when its subject names none.
func PRNumber(subject string) int {
	return PRNumbers(subject).Own
}

// NumberSpaces says which repository a pull request number belongs to. A
// fork's history holds two number spaces: the fork's own pull requests and
// those of the repository it was forked from, whose history it carries.
//   - Repo: the repository of the build's own commits ("owner/name").
//   - UpstreamRepo and UpstreamBase: the forked repository and a revision of
//     it in this history (the fork point); that commit and every commit
//     reachable from it are upstream commits, numbered in UpstreamRepo.
//
// Without an upstream every number is read as one of Repo.
type NumberSpaces struct {
	Repo         string
	UpstreamRepo string
	UpstreamBase string
}

// Rule describes in words how numbers are assigned to repositories.
func (s NumberSpaces) Rule() string {
	own := s.Repo
	if own == "" {
		own = "the repository of the source (its name is not known)"
	}
	if s.UpstreamBase == "" {
		return "every pull request number is read as one of " + own +
			"; no upstream is configured (history.upstream in the scope profile), so the numbers of history merged in from another repository are not told apart"
	}
	return fmt.Sprintf("commits reachable from %s (the fork point) are upstream commits and their numbers are pull requests of %s; the numbers of later commits are pull requests of %s",
		s.UpstreamBase, s.UpstreamRepo, own)
}

// UpstreamCommits lists the commits reachable from base, base included.
func UpstreamCommits(ctx context.Context, repoRoot, base string) (map[string]bool, error) {
	if base == "" {
		return nil, nil
	}
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-list", base, "--").Output()
	if err != nil {
		return nil, fmt.Errorf("upstream base %q: git rev-list: %w", base, err)
	}
	set := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			set[l] = true
		}
	}
	return set, nil
}
