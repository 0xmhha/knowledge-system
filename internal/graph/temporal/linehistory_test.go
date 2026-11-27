package temporal_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/temporal"
)

// fixtureRepo is a repository a test writes commits into.
type fixtureRepo struct {
	t   *testing.T
	dir string
}

func newFixtureRepo(t *testing.T) *fixtureRepo {
	t.Helper()
	r := &fixtureRepo{t: t, dir: initRepo(t)}
	return r
}

func (r *fixtureRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *fixtureRepo) write(rel string, lines []string) {
	r.t.Helper()
	full := filepath.Join(r.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *fixtureRepo) commit(msg string) string {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "--allow-empty", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

// gitLineLog is the truth for a line history: the commits git log -L lists.
func (r *fixtureRepo) gitLineLog(path string, start, end int) []string {
	out := r.git("log", "HEAD", "--format=%H", "--no-patch", fmt.Sprintf("-L%d,%d:%s", start, end, path))
	return strings.Fields(out)
}

var blameLine = regexp.MustCompile(`(?m)^([0-9a-f]{40}) \d+ (\d+)`)

// gitBlame is the truth for blame: per line of path, the commit git blame
// names.
func (r *fixtureRepo) gitBlame(path string) map[int]string {
	out := r.git("blame", "--porcelain", "HEAD", "--", path)
	got := map[int]string{}
	for _, m := range blameLine.FindAllStringSubmatch(out, -1) {
		var l int
		_, _ = fmt.Sscan(m[2], &l)
		got[l] = m[1]
	}
	return got
}

func touchSHAs(ts []temporal.Touch) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.SHA)
	}
	return out
}

// randomEdit changes lines of a file: inserts, deletes or replaces a few.
func randomEdit(rng *rand.Rand, lines []string, tag string, lo, hi int) []string {
	out := slices.Clone(lines)
	if hi > len(out) {
		hi = len(out)
	}
	if hi <= lo {
		return append(out, tag+"-tail")
	}
	at := lo + rng.Intn(hi-lo)
	switch rng.Intn(3) {
	case 0:
		ins := []string{}
		for i := 0; i < 1+rng.Intn(3); i++ {
			ins = append(ins, fmt.Sprintf("%s-ins-%d", tag, i))
		}
		out = slices.Insert(out, at, ins...)
	case 1:
		n := min(1+rng.Intn(2), hi-at)
		if len(out)-n < 4 {
			out[at] = tag + "-mod"
			break
		}
		out = slices.Delete(out, at, at+n)
	default:
		out[at] = tag + "-mod"
	}
	return out
}

// TestTrackLines_MatchesGitLineLogAndBlame builds histories with edits,
// branches merged back, and a rename, then compares every item's line
// history with git log -L and every line's origin with git blame.
func TestTrackLines_MatchesGitLineLogAndBlame(t *testing.T) {
	for seed := int64(1); seed <= 4; seed++ {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			r := newFixtureRepo(t)
			path := "pkg/a.go"
			lines := make([]string, 40)
			for i := range lines {
				lines[i] = fmt.Sprintf("orig-%d", i)
			}
			r.write(path, lines)
			r.commit("add a")
			for step := 0; step < 14; step++ {
				switch step {
				case 7:
					// A branch edits the top half while the main line edits the
					// bottom half; the merge combines both.
					r.git("checkout", "-q", "-b", fmt.Sprintf("side%d", step))
					side := randomEdit(rng, lines, fmt.Sprintf("s%d", step), 0, len(lines)/2)
					r.write(path, side)
					r.commit("side edit")
					r.git("checkout", "-q", "main")
					mainLines := randomEdit(rng, lines, fmt.Sprintf("m%d", step), len(lines)*3/4, len(lines))
					r.write(path, mainLines)
					r.commit("main edit")
					r.git("merge", "-q", "--no-edit", fmt.Sprintf("side%d", step))
					raw, err := os.ReadFile(filepath.Join(r.dir, path))
					if err != nil {
						t.Fatal(err)
					}
					lines = strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
				case 10:
					r.git("mv", path, "pkg/b.go")
					path = "pkg/b.go"
					r.commit("rename a to b")
				default:
					lines = randomEdit(rng, lines, fmt.Sprintf("e%d", step), 0, len(lines))
					r.write(path, lines)
					r.commit(fmt.Sprintf("edit %d", step))
				}
			}
			var items []temporal.Item
			for i := 0; i < 8; i++ {
				a := 1 + rng.Intn(len(lines)-1)
				b := min(len(lines), a+rng.Intn(8))
				items = append(items, temporal.Item{ID: fmt.Sprintf("d%d", i), Path: path, Start: a, End: b})
			}
			items = append(items, temporal.Item{ID: "file", Path: path, Start: 1, End: len(lines)})
			h, err := temporal.TrackLines(context.Background(), r.dir, []string{"*.go"}, items, nil)
			if err != nil {
				t.Fatalf("TrackLines: %v", err)
			}
			for _, it := range items {
				want := r.gitLineLog(it.Path, it.Start, it.End)
				if got := touchSHAs(h.Touches[it.ID]); !slices.Equal(got, want) {
					t.Errorf("%s [%d,%d]: line history = %v, git log -L = %v", it.ID, it.Start, it.End, got, want)
				}
			}
			truth := r.gitBlame(path)
			for _, it := range items {
				want := map[string]int{}
				for l := it.Start; l <= it.End; l++ {
					want[truth[l]]++
				}
				if got := h.Blame[it.ID]; fmt.Sprint(got) != fmt.Sprint(want) {
					t.Errorf("%s [%d,%d]: blame = %v, git blame = %v", it.ID, it.Start, it.End, got, want)
				}
			}
		})
	}
}

// TestTrackLines_TouchRangeIsTheRangeAtTheCommit checks that a touch locates
// the item where it was in that commit, not where it is now.
func TestTrackLines_TouchRangeIsTheRangeAtTheCommit(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("a.go", []string{"package a", "", "func F() {", "\tx()", "}"})
	r.commit("add F")
	r.write("a.go", []string{"package a", "", "func F() {", "\ty()", "}"})
	edit := r.commit("edit F")
	r.write("a.go", []string{"package a", "", "// one", "// two", "func F() {", "\ty()", "}"})
	r.commit("comment above F")
	h, err := temporal.TrackLines(context.Background(), r.dir, []string{"*.go"},
		[]temporal.Item{{ID: "F", Path: "a.go", Start: 5, End: 7}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got *temporal.Touch
	for i, tc := range h.Touches["F"] {
		if tc.SHA == edit {
			got = &h.Touches["F"][i]
		}
	}
	if got == nil || got.Start != 3 || got.End != 5 {
		t.Fatalf("touch of the edit = %+v, want lines 3-5 of that commit", got)
	}
	if n := len(h.Touches["F"]); n != 2 {
		t.Errorf("F has %d commits in its line history, want 2 (the comment above it is not a change of F)", n)
	}
}

// TestTrackLines_UncommittedLines keeps working-tree changes out of every
// commit.
func TestTrackLines_UncommittedLines(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("a.go", []string{"package a", "func F() {", "\tx()", "}"})
	c1 := r.commit("add F")
	r.write("a.go", []string{"package a", "", "func F() {", "\tz()", "}"})
	h, err := temporal.TrackLines(context.Background(), r.dir, []string{"*.go"},
		[]temporal.Item{{ID: "F", Path: "a.go", Start: 3, End: 5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := touchSHAs(h.Touches["F"]); !slices.Equal(got, []string{c1}) {
		t.Errorf("line history = %v, want [%s]", got, c1)
	}
	if h.Uncommitted["F"] != 1 || h.Blame["F"][c1] != 2 {
		t.Errorf("uncommitted = %d, blame = %v; want 1 uncommitted line and 2 lines of %s", h.Uncommitted["F"], h.Blame["F"], c1)
	}
}

// TestTrackLines_MergeFollowsTheFirstUntouchedParent: a branch changes F
// and reverts it, the main line changes G, and the branch is merged. The
// merge's file differs from both parents, so a range passes to the first
// parent whose diff leaves it untouched, as git log -L does — not to the
// parent holding the same file — and blame still follows git blame.
func TestTrackLines_MergeFollowsTheFirstUntouchedParent(t *testing.T) {
	r := newFixtureRepo(t)
	base := []string{"package a", "func F() {", "\tf()", "}", "", "func G() {", "\tg()", "}"}
	r.write("a.go", base)
	r.commit("add")
	r.git("checkout", "-q", "-b", "side")
	changed := slices.Clone(base)
	changed[2] = "\tf2()"
	r.write("a.go", changed)
	r.commit("change F")
	other := slices.Clone(base)
	other[0] = "package a // side"
	r.write("a.go", other)
	r.commit("revert F, mark the package")
	r.git("checkout", "-q", "main")
	mainLines := slices.Clone(base)
	mainLines[6] = "\tg2()"
	r.write("a.go", mainLines)
	r.commit("change G")
	r.git("merge", "-q", "--no-edit", "side")
	items := []temporal.Item{{ID: "F", Path: "a.go", Start: 2, End: 4}, {ID: "G", Path: "a.go", Start: 6, End: 8}, {ID: "file", Path: "a.go"}}
	h, err := temporal.TrackLines(context.Background(), r.dir, []string{"*.go"}, items, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		end := it.End
		if end == 0 {
			end = len(mainLines)
		}
		want := r.gitLineLog(it.Path, max(it.Start, 1), end)
		if got := touchSHAs(h.Touches[it.ID]); !slices.Equal(sortedCopy(got), sortedCopy(want)) {
			t.Errorf("%s: line history = %v, git log -L = %v", it.ID, got, want)
		}
	}
	truth := r.gitBlame("a.go")
	for l := 1; l <= len(mainLines); l++ {
		if h.Blame["file"][truth[l]] == 0 {
			t.Errorf("line %d: git blame names %s, which the file's blame lacks: %v", l, truth[l], h.Blame["file"])
		}
	}
}

// TestTrackLines_InsertionBesideOneLineDeclaration: lines inserted right
// before or right after a one-line declaration, or removed beside it, do not
// change it; git log -L agrees on every such boundary case.
func TestTrackLines_InsertionBesideOneLineDeclaration(t *testing.T) {
	r := newFixtureRepo(t)
	lines := []string{"package a"}
	for i := 0; i < 10; i++ {
		lines = append(lines, fmt.Sprintf("func F%d() int { return %d }", i, i))
	}
	r.write("a.go", lines)
	r.commit("add")
	// Insert after F2, before F5, after F7 (twice), and delete F9's
	// neighbour F8: twenty boundary cases over the ten declarations.
	edits := []func([]string) []string{
		func(l []string) []string { return slices.Insert(l, 4, "func G() int { return 0 }") },
		func(l []string) []string { return slices.Insert(l, 6, "// before F5") },
		func(l []string) []string {
			return slices.Insert(l, 10, "func H() int { return 1 }", "func I() int { return 2 }")
		},
		func(l []string) []string { return slices.Delete(l, 11, 12) },
	}
	for i, e := range edits {
		lines = e(lines)
		r.write("a.go", lines)
		r.commit(fmt.Sprintf("edit %d", i))
	}
	var items []temporal.Item
	for i, l := range lines {
		if strings.HasPrefix(l, "func F") {
			items = append(items, temporal.Item{ID: l[:7], Path: "a.go", Start: i + 1, End: i + 1})
		}
	}
	h, err := temporal.TrackLines(context.Background(), r.dir, []string{"*.go"}, items, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		want := r.gitLineLog(it.Path, it.Start, it.End)
		if got := touchSHAs(h.Touches[it.ID]); !slices.Equal(got, want) {
			t.Errorf("%s at line %d: line history = %v, git log -L = %v", it.ID, it.Start, got, want)
		}
	}
}

// TestTrackLines_CopyFollowsOnlyLinesItHolds: a file added as a copy of
// another (git reports it, the license header making them alike) carries an
// item into the source only when the source holds the item's code. G, copied
// with A's file, goes on into a.go; B, new code under the same header, is
// born where it was written.
func TestTrackLines_CopyFollowsOnlyLinesItHolds(t *testing.T) {
	r := newFixtureRepo(t)
	var header []string
	for i := 0; i < 30; i++ {
		header = append(header, fmt.Sprintf("// Copyright line %d of the shared license header.", i))
	}
	g := []string{"func G() int {", "\treturn 7", "}"}
	a := append(append(append([]string{}, header...), "package a", ""), append(g, "", "func A() int {", "\treturn 1", "}")...)
	r.write("a.go", a)
	r.commit("add a")
	a[len(a)-2] = "\treturn 2"
	r.write("a.go", a)
	editA := r.commit("edit A")
	gEdit := slices.Clone(a)
	gEdit[len(header)+3] = "\treturn 8"
	r.write("a.go", gEdit)
	editG := r.commit("edit G")
	b := append(append(append([]string{}, header...), "package a", ""), append(slices.Clone(g[:1]), "\treturn 8", "}", "", "func B() int {", "\treturn 3", "}")...)
	r.write("b.go", b)
	addB := r.commit("add b")
	gStart := len(header) + 3
	bStart := gStart + 4
	h, err := temporal.TrackLines(context.Background(), r.dir, []string{"*.go"}, []temporal.Item{
		{ID: "G", Path: "b.go", Start: gStart, End: gStart + 2, Name: "G"},
		{ID: "B", Path: "b.go", Start: bStart, End: bStart + 2, Name: "B"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	gotB := touchSHAs(h.Touches["B"])
	if !slices.Equal(gotB, []string{addB}) {
		t.Errorf("B's history = %v, want only the commit that wrote it (%s); %s changed A, not B", gotB, addB, editA)
	}
	gotG := touchSHAs(h.Touches["G"])
	if !slices.Contains(gotG, editG) || slices.Contains(gotG, editA) {
		t.Errorf("G's history = %v, want it to go on into a.go (%s) without A's edit (%s)", gotG, editG, editA)
	}
	for _, tc := range h.Touches["G"] {
		if tc.SHA == editG && tc.PastCopy != addB {
			t.Errorf("a commit before the copy is not marked past_copy: %+v", tc)
		}
	}
}

func sortedCopy(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}
