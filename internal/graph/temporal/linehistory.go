package temporal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Item is one range of lines whose history TrackLines follows: a declaration
// or a whole file. Path is relative to the repository root, slash-separated;
// Start and End are 1-based inclusive lines of the working tree; End 0
// means through the last line (a whole file).
type Item struct {
	ID         string
	Path       string
	Start, End int
	// Name is the declaration's own name ("" for a whole file): following a
	// copy needs a kept line that names it (holdsLines).
	Name string
}

// Touch records that commit SHA changed an item's lines (LineChange.Touches).
// Path, Start and End locate the item in that commit, after the change.
type Touch struct {
	SHA        string
	Path       string
	Start, End int
	// PastCopy is the commit that created the item's file as a copy of
	// another file, when SHA is older than that copy: git log -L follows
	// some such copies and not others, so it may not list SHA.
	PastCopy string
	// Copy is set when SHA is the commit that created the item's file as a
	// copy of another file; git log -L lists it unless it follows the copy
	// and the copy left the item's lines as they were.
	Copy bool
}

// CommitMeta describes one commit of the walked history. Times are unix
// seconds. Parents are the parents in the walked history: git's parent
// rewriting for the walked path set replaces a parent that touched none of
// those paths with its nearest ancestor that did.
type CommitMeta struct {
	SHA        string
	Parents    []string
	Author     string
	AuthorTime int64
	CommitTime int64
	Subject    string
	Body       string
}

// LineHistory is the result of TrackLines.
type LineHistory struct {
	// Commits holds every commit of the walked history, by SHA.
	Commits map[string]CommitMeta
	// Order lists the walked commits newest first (git's topological
	// order: no commit before one of its descendants).
	Order []string
	// Touches lists, per item ID, the commits that changed its lines,
	// newest first.
	Touches map[string][]Touch
	// Blame counts, per item ID, the item's lines by the commit that last
	// changed each of them.
	Blame map[string]map[string]int
	// Uncommitted counts, per item ID, the lines that differ from HEAD in
	// the working tree, so no commit is their origin.
	Uncommitted map[string]int
	// BeyondBoundary counts, per item ID, the lines whose origin lies before
	// the history of a shallow clone begins.
	BeyondBoundary map[string]int
	// ReachesBoundary is set for items whose line history continues past a
	// shallow clone's boundary: older commits that changed them are missing.
	ReachesBoundary map[string]bool
	// Boundaries are the shallow-clone boundary commits the walk met.
	Boundaries []string
}

// historyHeaderFormat is the git log format TrackLines reads: one record per
// commit, fields separated by NUL, records by SOH. The body comes last so it
// may hold anything.
const (
	historyFieldSep     = "\x00"
	historyRecordSep    = "\x01"
	historyFieldCount   = 7
	historyHeaderFormat = "%H%x00%P%x00%an%x00%at%x00%ct%x00%s%x00%b%x01"
)

// TrackLines follows the lines of items back through the history of HEAD in
// repoRoot and returns, per item, the commits that changed them (its line
// history, the commits git log -L lists for the range) and the commits that
// last changed each of its lines (git blame). Renames are followed (git's
// rename detection); moved or copied lines are not traced to their source.
//
// pathspec limits the history walked (for example "*.go"); it must cover every
// item's file and the names those files had before a rename. boundaries are
// the commits at which a shallow clone's history was cut: they stand for
// history that is not there, so nothing is attributed to them.
//
// It runs three git commands whatever the number of items: git log for the
// commit graph, one git diff-tree reading every (commit, parent) pair from
// stdin, and git diff for the working tree.
func TrackLines(ctx context.Context, repoRoot string, pathspec []string, items []Item, boundaries map[string]bool) (*LineHistory, error) {
	h := &LineHistory{
		Commits:         map[string]CommitMeta{},
		Touches:         map[string][]Touch{},
		Blame:           map[string]map[string]int{},
		Uncommitted:     map[string]int{},
		BeyondBoundary:  map[string]int{},
		ReachesBoundary: map[string]bool{},
	}
	if len(items) == 0 || !isGitCheckout(repoRoot) {
		return h, nil
	}
	if err := h.loadCommits(ctx, repoRoot, pathspec); err != nil {
		return nil, err
	}
	pairs := walkPairs(h, boundaries)
	diffs, err := readPairDiffs(ctx, repoRoot, pathspec, pairs, renameFlags)
	if err != nil {
		return nil, err
	}
	// A file that a commit adds may be a copy of a file the commit left in
	// place (git log -L and --follow look for one among every file of the
	// parent). Each round looks for the sources of the files the last walk
	// saw born, and walks again with those copies followed.
	checked := map[birth]bool{}
	for round := 0; ; round++ {
		t, err := newTracker(ctx, repoRoot, pathspec, items)
		if err != nil {
			return nil, err
		}
		h.Boundaries = nil
		if len(h.Order) > 0 {
			t.states[h.Order[0]] = t.head
			t.walk(h, diffs, boundaries)
		}
		var todo []birth
		for _, b := range t.births {
			if !checked[b] {
				checked[b] = true
				todo = append(todo, b)
			}
		}
		if round == copyRoundsMax || len(todo) == 0 {
			t.finish(h)
			return h, nil
		}
		found, err := followCopies(ctx, repoRoot, pathspec, pairs, diffs, todo)
		if err != nil {
			return nil, err
		}
		if found == 0 {
			t.finish(h)
			return h, nil
		}
	}
}

// copyRoundsMax bounds how many times the walk looks for copy sources: each
// round can only reach older history through a copy found in the last one.
const copyRoundsMax = 4

// Every copy git reports with --find-copies-harder is a candidate; whether
// an item's history goes on into it is decided per item, by its own lines
// (holdsLines). git log -L follows some copies and not others; the commits
// the tool adds beyond git are marked (Touch.Copy, Touch.PastCopy).

// renameFlags and copyFlags are the diff-tree options of the walk and of the
// search for a copy's source.
var (
	renameFlags = []string{"-M"}
	copyFlags   = []string{"-M", "-C", "--find-copies-harder"}
)

// birth is a file a walked commit added: pair indexes the (commit, parent)
// pair, path is the file in the commit.
type birth struct {
	pair int
	path string
}

// followCopies looks for the source of each born file among every file of
// the parent and turns a found copy into a rename in diffs. It returns how
// many it found.
func followCopies(ctx context.Context, repoRoot string, pathspec []string, pairs []diffPair, diffs []map[string]fileDiff, todo []birth) (int, error) {
	idx := map[int]bool{}
	var sub []diffPair
	var at []int
	for _, b := range todo {
		if !idx[b.pair] {
			idx[b.pair] = true
			sub = append(sub, pairs[b.pair])
			at = append(at, b.pair)
		}
	}
	got, err := readPairDiffs(ctx, repoRoot, pathspec, sub, copyFlags)
	if err != nil {
		return 0, err
	}
	byPair := map[int]map[string]fileDiff{}
	for i, d := range got {
		byPair[at[i]] = d
	}
	found := 0
	for _, b := range todo {
		d, ok := byPair[b.pair][b.path]
		if ok && !d.added && d.oldPath != b.path {
			d.copied = true
			diffs[b.pair][b.path] = d
			found++
		}
	}
	return found, nil
}

// walkPairs lists the (commit, parent) pairs whose diffs the walk reads, in
// walk order.
func walkPairs(h *LineHistory, boundaries map[string]bool) []diffPair {
	var pairs []diffPair
	for _, sha := range h.Order {
		if boundaries[sha] {
			continue
		}
		for _, p := range h.Commits[sha].Parents {
			pairs = append(pairs, diffPair{commit: sha, parent: p})
		}
	}
	return pairs
}

// loadCommits reads the walked commit graph and its metadata.
func (h *LineHistory) loadCommits(ctx context.Context, repoRoot string, pathspec []string) error {
	args := append([]string{"-C", repoRoot, "-c", "core.quotePath=false", "log", "--topo-order", "--parents",
		"--format=" + historyHeaderFormat, "HEAD", "--"}, pathspec...)
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return fmt.Errorf("git log for line history: %w", err)
	}
	for _, c := range parseCommitRecords(out) {
		h.Commits[c.SHA] = c
		h.Order = append(h.Order, c.SHA)
	}
	return nil
}

// parseCommitRecords reads git output in historyHeaderFormat.
func parseCommitRecords(out []byte) []CommitMeta {
	var cs []CommitMeta
	for rec := range strings.SplitSeq(string(out), historyRecordSep) {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, historyFieldSep, historyFieldCount)
		if len(f) < historyFieldCount {
			continue
		}
		at, _ := strconv.ParseInt(f[3], 10, 64)
		ct, _ := strconv.ParseInt(f[4], 10, 64)
		cs = append(cs, CommitMeta{
			SHA: f[0], Parents: strings.Fields(f[1]), Author: f[2],
			AuthorTime: at, CommitTime: ct, Subject: f[5], Body: strings.TrimRight(f[6], "\n"),
		})
	}
	return cs
}

// ReadCommits reads the metadata of the given commits from the object
// store, reachable or not.
func ReadCommits(ctx context.Context, repoRoot string, shas []string) (map[string]CommitMeta, error) {
	out := map[string]CommitMeta{}
	if len(shas) == 0 {
		return out, nil
	}
	args := append([]string{"-C", repoRoot, "show", "-s", "--no-walk=unsorted", "--format=" + historyHeaderFormat}, shas...)
	raw, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git show for commit metadata: %w", err)
	}
	for _, c := range parseCommitRecords(raw) {
		out[c.SHA] = c
	}
	return out, nil
}

// itemState is an item's range in the commit being visited.
type itemState struct {
	id string
	r  lineRange
	// past is the copy commit the walk followed to reach this range, ""
	// before any copy.
	past string
}

// fileState is what is still being followed in one file of one commit: the
// items' ranges and blame's unattributed lines.
type fileState struct {
	items []itemState
	segs  []segment
}

func (fs *fileState) empty() bool { return len(fs.items) == 0 && len(fs.segs) == 0 }

// commitState maps a path in the commit to what is followed in it.
type commitState map[string]*fileState

// tracker holds the walk: the states waiting at not yet visited commits and
// the results gathered so far.
type tracker struct {
	head     commitState
	states   map[string]commitState
	itemOf   map[string]Item
	origins  map[string][]int32 // worktree path -> per line: index into shas, -1 unattributed
	shas     []string
	shaIdx   map[string]int32
	seen     map[string]map[string]bool // item -> commits already recorded
	touches  map[string][]Touch
	reaches  map[string]bool
	ctx      context.Context
	repoRoot string
	blobs    map[string][]string // "<sha>:<path>" -> lines, for copies
	// pair is the index of the visited commit's first (commit, parent)
	// pair; births are the files the walk saw added with lines to follow.
	pair   int
	births []birth
}

// Origin markers for lines with no commit.
const (
	originNone        int32 = -1
	originUncommitted int32 = -2
	originBoundary    int32 = -3
)

func newTracker(ctx context.Context, repoRoot string, pathspec []string, items []Item) (*tracker, error) {
	t := &tracker{
		head: commitState{}, states: map[string]commitState{}, itemOf: map[string]Item{},
		origins: map[string][]int32{}, shaIdx: map[string]int32{},
		seen: map[string]map[string]bool{}, touches: map[string][]Touch{}, reaches: map[string]bool{},
		ctx: ctx, repoRoot: repoRoot, blobs: map[string][]string{},
	}
	inHead, err := headPaths(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	work := map[string]*fileState{}
	for _, it := range items {
		if _, ok := t.origins[it.Path]; !ok {
			n, err := countLines(repoRoot, it.Path)
			if err != nil {
				return nil, err
			}
			o := make([]int32, n)
			for i := range o {
				o[i] = originNone
			}
			t.origins[it.Path] = o
			fs := &fileState{}
			if n > 0 {
				fs.segs = []segment{{file: it.Path, head: 1, cur: 1, n: n}}
			}
			work[it.Path] = fs
		}
		if it.End <= 0 {
			it.Start, it.End = max(it.Start, 1), len(t.origins[it.Path]) // End 0: through the last line
		}
		t.itemOf[it.ID] = it
		if it.Start >= 1 && it.End >= it.Start {
			work[it.Path].items = append(work[it.Path].items, itemState{id: it.ID, r: lineRange{it.Start, it.End}})
		}
	}
	diffs, err := worktreeDiffs(ctx, repoRoot, pathspec)
	if err != nil {
		return nil, err
	}
	for p, fs := range work {
		if !inHead[p] {
			t.markHead(fs.segs, originUncommitted)
			continue
		}
		d, changed := diffs[p]
		if !changed {
			t.head[p] = fs
			continue
		}
		next := &fileState{}
		for _, it := range fs.items {
			if r, _ := mapToParent(d.changes, it.r); !r.empty() {
				next.items = append(next.items, itemState{id: it.id, r: r})
			}
		}
		for _, s := range fs.segs {
			kept, changed := splitSegment(d.changes, s)
			t.markHead(changed, originUncommitted)
			next.segs = append(next.segs, kept...)
		}
		t.head[d.oldPath] = next
	}
	return t, nil
}

// diffTreeMaxWorkers bounds the git diff-tree processes run side by side;
// each reads a contiguous slice of the (commit, parent) pairs.
const diffTreeMaxWorkers = 8

// diffPair is one (commit, parent) pair whose diff the walk reads.
type diffPair struct{ commit, parent string }

// walk visits the commits in h.Order, newest first, moving what is followed
// from each commit to its parents.
func (t *tracker) walk(h *LineHistory, diffs []map[string]fileDiff, boundaries map[string]bool) {
	next := 0
	for _, sha := range h.Order {
		st := t.states[sha]
		delete(t.states, sha)
		if boundaries[sha] {
			h.Boundaries = append(h.Boundaries, sha)
			t.cut(st)
			continue
		}
		parents := h.Commits[sha].Parents
		own := diffs[next : next+len(parents)]
		t.pair = next
		next += len(parents)
		if st != nil {
			t.visit(sha, parents, st, own)
		}
	}
}

// readPairDiffs returns the diff of every pair, in order, read by up to
// diffTreeMaxWorkers git diff-tree processes.
func readPairDiffs(ctx context.Context, repoRoot string, pathspec []string, pairs []diffPair, flags []string) ([]map[string]fileDiff, error) {
	out := make([]map[string]fileDiff, len(pairs))
	workers := max(1, min(runtime.GOMAXPROCS(0), diffTreeMaxWorkers))
	chunk := (len(pairs) + workers - 1) / max(workers, 1)
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for w := 0; w < workers; w++ {
		lo, hi := w*chunk, min(len(pairs), (w+1)*chunk)
		if lo >= hi {
			break
		}
		wg.Add(1)
		go func(w, lo, hi int) {
			defer wg.Done()
			errs[w] = diffTree(ctx, repoRoot, pathspec, flags, pairs[lo:hi], out[lo:hi])
		}(w, lo, hi)
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return out, nil
}

// diffTree runs one git diff-tree over pairs and stores their diffs in out.
func diffTree(ctx context.Context, repoRoot string, pathspec, flags []string, pairs []diffPair, out []map[string]fileDiff) error {
	args := []string{"-C", repoRoot, "-c", "core.quotePath=false", "diff-tree", "--stdin", "--always", "-p", "-U0"}
	args = append(args, flags...)
	args = append(append(args, "--no-color", "--full-index", "--"), pathspec...)
	cmd := exec.CommandContext(ctx, "git", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("git diff-tree stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git diff-tree stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git diff-tree start: %w", err)
	}
	writeErr := make(chan error, 1)
	go func() { writeErr <- writePairs(stdin, pairs) }()
	r := newDiffReader(stdout)
	var readErr error
	for i, p := range pairs {
		d, err := r.next(p.commit)
		if err != nil {
			readErr = err
			_ = cmd.Process.Kill()
			break
		}
		out[i] = d
	}
	_, _ = io.Copy(io.Discard, stdout)
	werr := <-writeErr
	waitErr := cmd.Wait()
	switch {
	case readErr != nil:
		return readErr
	case werr != nil:
		return fmt.Errorf("git diff-tree feed: %w", werr)
	case waitErr != nil:
		return fmt.Errorf("git diff-tree: %w (stderr: %s)", waitErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// writePairs feeds git diff-tree one "<commit> <parent>" line per pair, then
// closes stdin.
func writePairs(w io.WriteCloser, pairs []diffPair) error {
	bw := bufio.NewWriter(w)
	for _, p := range pairs {
		if _, err := fmt.Fprintf(bw, "%s %s\n", p.commit, p.parent); err != nil {
			_ = w.Close()
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// visit moves the state of commit sha to its parents.
func (t *tracker) visit(sha string, parents []string, st commitState, diffs []map[string]fileDiff) {
	for p, fs := range st {
		if len(parents) == 0 {
			t.born(sha, p, fs)
			continue
		}
		if len(parents) == 1 {
			t.single(sha, parents[0], p, fs, diffs[0])
			continue
		}
		t.merge(sha, parents, p, fs, diffs)
	}
}

// born attributes everything still followed in file p to sha, which created
// the file (a root commit, or the file is new in sha).
func (t *tracker) born(sha, p string, fs *fileState) {
	for _, it := range fs.items {
		t.record(it.id, sha, p, it.r, it.past)
	}
	t.attribute(fs.segs, sha)
}

// single handles a commit with one parent.
func (t *tracker) single(sha, parent, p string, fs *fileState, diffs map[string]fileDiff) {
	d, changed := diffs[p]
	if !changed {
		t.pass(parent, p, fs)
		return
	}
	if d.added {
		t.births = append(t.births, birth{pair: t.pair, path: p})
		t.born(sha, p, fs)
		return
	}
	next := &fileState{}
	for _, it := range fs.items {
		r, touched := mapToParent(d.changes, it.r)
		past := it.past
		if d.copied {
			if !t.holdsLines(sha, p, t.itemOf[it.id].Name, d.changes, it.r) {
				// The copy's source does not hold this item's lines: the
				// item was written here.
				t.record(it.id, sha, p, it.r, it.past)
				continue
			}
			// The copying commit is listed, marked: git log -L lists it as
			// the item's birth when it does not follow the copy, and when it
			// follows and the copy changed the lines.
			t.recordTouch(it.id, Touch{SHA: sha, Path: p, Start: it.r.Start, End: it.r.End, PastCopy: it.past, Copy: true})
			if past == "" {
				past = sha
			}
		} else if touched {
			t.record(it.id, sha, p, it.r, it.past)
		}
		if !r.empty() {
			next.items = append(next.items, itemState{id: it.id, r: r, past: past})
		}
	}
	if d.copied {
		// git blame does not follow a copy: the commit that made the file
		// is the origin of every line still unattributed.
		t.attribute(fs.segs, sha)
	} else {
		for _, s := range fs.segs {
			kept, changed := splitSegment(d.changes, s)
			t.attribute(changed, sha)
			next.segs = append(next.segs, kept...)
		}
	}
	t.pass(parent, d.oldPath, next)
}

// Copy-following criterion. When a commit creates an item's file as a copy,
// the copy's diff aligns the new file with its source (the hunk structure:
// lines outside every hunk correspond, by offset, to lines of the source).
// The item's history goes on into the source only when
//   - at least copyKeptNum/copyKeptDen of the item's telling lines are such
//     corresponding lines, and at least copyKeptMinLines of them: a telling
//     line holds an identifier or literal that is not a lone keyword (not
//     blank, not only a comment, not only braces, parentheses or a bare
//     "return", "return nil", "return err"), and
//   - for a declaration, its header (the first telling line of its range,
//     the "func" or "type" line) is a corresponding line, or another
//     corresponding line names it: the declaration moved with the file,
//     under its name then, which may differ from today's.
//
// A license header, or a few braces and returns, then carry no declaration
// into another file; a declaration moved with its file goes on.
const (
	copyKeptNum      = 1
	copyKeptDen      = 2
	copyKeptMinLines = 1
)

// trivialWords are the words a line may hold and still tell nothing.
var trivialWords = map[string]bool{"return": true, "nil": true, "err": true, "break": true, "continue": true, "else": true, "default": true, "true": true, "false": true}

// holdsLines reports whether the copy that created path in sha keeps the
// item's lines r from its source, by the copy-following criterion.
func (t *tracker) holdsLines(sha, path, name string, changes []LineChange, r lineRange) bool {
	lines := t.blobLines(sha, path)
	telling, kept := 0, 0
	// anchored: the declaration's header (its first telling line) is a
	// corresponding line, or a corresponding line names it.
	anchored := name == ""
	for l := r.Start; l <= r.End && l <= len(lines); l++ {
		if !isTellingLine(lines[l-1]) {
			continue
		}
		telling++
		header := telling == 1
		if lineChanged(changes, l) {
			continue
		}
		kept++
		anchored = anchored || header || namesIdent(lines[l-1], name)
	}
	return anchored && kept >= copyKeptMinLines && kept*copyKeptDen >= telling*copyKeptNum
}

// identRE matches the identifiers and literals of a line of code.
var identRE = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|[0-9]+`)

// isTellingLine reports whether a line holds code that tells it apart: an
// identifier or literal other than trivialWords, outside a comment.
func isTellingLine(l string) bool {
	l = strings.TrimSpace(l)
	for _, p := range commentPrefixes {
		if strings.HasPrefix(l, p) {
			return false
		}
	}
	if i := strings.Index(l, "//"); i >= 0 {
		l = l[:i]
	}
	for _, w := range identRE.FindAllString(l, -1) {
		if !trivialWords[w] {
			return true
		}
	}
	return false
}

// namesIdent reports whether line holds name as a whole identifier.
func namesIdent(line, name string) bool {
	for _, w := range identRE.FindAllString(line, -1) {
		if w == name {
			return true
		}
	}
	return false
}

// lineChanged reports whether a change added or replaced commit-side line l.
func lineChanged(changes []LineChange, l int) bool {
	for _, c := range changes {
		if c.NewCount > 0 && c.NewStart <= l && l <= c.newEnd() {
			return true
		}
	}
	return false
}

// commentPrefixes start a line that holds only a comment.
var commentPrefixes = []string{"//", "/*", "*", "#"}

// blobLines reads path at commit sha, cached.
func (t *tracker) blobLines(sha, path string) []string {
	key := sha + ":" + path
	if l, ok := t.blobs[key]; ok {
		return l
	}
	out, err := exec.CommandContext(t.ctx, "git", "-C", t.repoRoot, "show", key).Output()
	var lines []string
	if err == nil {
		lines = strings.Split(string(out), "\n")
	}
	t.blobs[key] = lines
	return lines
}

// merge handles a merge commit the way git log -L and git blame do: what a
// parent left unchanged passes to that parent (the first such parent) and is
// not the merge's; what differs from every parent is the merge's own change.
func (t *tracker) merge(sha string, parents []string, p string, fs *fileState, diffs []map[string]fileDiff) {
	// side[i] is the file in parent i: its diff, or none when the parent
	// holds the same file (an empty diff, nothing touched).
	type side struct {
		d       fileDiff
		same    bool
		missing bool // the merge created the file relative to this parent
	}
	sides := make([]side, len(parents))
	same := -1
	for i := range parents {
		d, changed := diffs[i][p]
		sides[i] = side{d: d, same: !changed, missing: changed && d.added}
		if !changed {
			sides[i].d.oldPath = p
			if same < 0 {
				same = i
			}
		}
	}
	next := make([]*fileState, len(parents))
	for i := range next {
		next[i] = &fileState{}
	}
	// Line history follows git log -L: an item's ranges pass to the first
	// parent whose diff touches none of them, and the merge is in its
	// history only when every parent's diff touches one of them.
	var order []string
	byID := map[string][]lineRange{}
	pastOf := map[string]string{}
	for _, it := range fs.items {
		if _, ok := byID[it.id]; !ok {
			order = append(order, it.id)
			pastOf[it.id] = it.past
		}
		byID[it.id] = append(byID[it.id], it.r)
	}
	for _, id := range order {
		ranges := byID[id]
		mapped := make([][]lineRange, len(parents))
		free := -1
		for i, s := range sides {
			if s.missing {
				continue
			}
			touched := false
			for _, r := range ranges {
				pr, tc := mapToParent(s.d.changes, r)
				touched = touched || tc
				if !pr.empty() {
					mapped[i] = append(mapped[i], pr)
				}
			}
			if !touched && free < 0 {
				free = i
			}
		}
		if free >= 0 {
			for _, r := range mapped[free] {
				next[free].items = append(next[free].items, itemState{id: id, r: r, past: pastOf[id]})
			}
			continue
		}
		t.record(id, sha, p, ranges[0], pastOf[id])
		for i := range mapped {
			for _, r := range mapped[i] {
				next[i].items = append(next[i].items, itemState{id: id, r: r, past: pastOf[id]})
			}
		}
	}
	// Blame follows git blame: a parent holding the same file takes every
	// line; otherwise each line passes to the first parent that left it
	// unchanged, and the lines changed against every parent are the merge's.
	if same >= 0 {
		next[same].segs = append(next[same].segs, fs.segs...)
	} else {
		rest := fs.segs
		for i, s := range sides {
			if s.missing {
				continue
			}
			var changedAll []segment
			for _, sg := range rest {
				kept, changed := splitSegment(s.d.changes, sg)
				next[i].segs = append(next[i].segs, kept...)
				changedAll = append(changedAll, changed...)
			}
			rest = changedAll
		}
		t.attribute(rest, sha)
	}
	for i, parent := range parents {
		if !sides[i].missing {
			t.pass(parent, sides[i].d.oldPath, next[i])
		}
	}
}

// pass adds fs to what parent follows in path.
func (t *tracker) pass(parent, path string, fs *fileState) {
	if fs.empty() {
		return
	}
	st := t.states[parent]
	if st == nil {
		st = commitState{}
		t.states[parent] = st
	}
	cur := st[path]
	if cur == nil {
		st[path] = fs
		return
	}
	// An item reaching one commit from two children keeps both ranges, as
	// git's range set does: ranges that overlap or touch merge, others stay
	// apart, so the lines between them are not taken into the item.
	at := make(map[string][]int, len(cur.items))
	for i, it := range cur.items {
		at[it.id] = append(at[it.id], i)
	}
	for _, it := range fs.items {
		merged := false
		for _, i := range at[it.id] {
			if r := cur.items[i].r; it.r.Start <= r.End+1 && r.Start <= it.r.End+1 {
				cur.items[i].r = r.hull(it.r.Start, it.r.End)
				merged = true
				break
			}
		}
		if !merged {
			at[it.id] = append(at[it.id], len(cur.items))
			cur.items = append(cur.items, it)
		}
	}
	cur.segs = append(cur.segs, fs.segs...)
}

// cut ends the walk at a shallow boundary: the items still followed have
// older history the clone lacks, and the lines still unattributed have their
// origin there.
func (t *tracker) cut(st commitState) {
	for _, fs := range st {
		for _, it := range fs.items {
			t.reaches[it.id] = true
		}
		t.markHead(fs.segs, originBoundary)
	}
}

// record adds sha to the line history of item id once.
func (t *tracker) record(id, sha, path string, r lineRange, past string) {
	t.recordTouch(id, Touch{SHA: sha, Path: path, Start: r.Start, End: r.End, PastCopy: past})
}

// recordTouch adds a touch to the line history of item id once per commit.
func (t *tracker) recordTouch(id string, tc Touch) {
	sha := tc.SHA
	s := t.seen[id]
	if s == nil {
		s = map[string]bool{}
		t.seen[id] = s
	}
	if s[sha] {
		return
	}
	s[sha] = true
	t.touches[id] = append(t.touches[id], tc)
}

// attribute makes sha the origin of the lines of segs.
func (t *tracker) attribute(segs []segment, sha string) {
	if len(segs) == 0 {
		return
	}
	idx, ok := t.shaIdx[sha]
	if !ok {
		idx = int32(len(t.shas))
		t.shas = append(t.shas, sha)
		t.shaIdx[sha] = idx
	}
	t.markHead(segs, idx)
}

// markHead sets the origin of the working-tree lines segs cover.
func (t *tracker) markHead(segs []segment, v int32) {
	for _, s := range segs {
		o := t.origins[s.file]
		for i := 0; i < s.n; i++ {
			if l := s.head + i - 1; l >= 0 && l < len(o) {
				o[l] = v
			}
		}
	}
}

// finish turns the gathered origins into per-item results.
func (t *tracker) finish(h *LineHistory) {
	h.Touches = t.touches
	h.ReachesBoundary = t.reaches
	for id, it := range t.itemOf {
		o := t.origins[it.Path]
		counts := map[string]int{}
		for l := max(it.Start, 1); l <= it.End && l <= len(o); l++ {
			switch v := o[l-1]; {
			case v >= 0:
				counts[t.shas[v]]++
			case v == originUncommitted:
				h.Uncommitted[id]++
			case v == originBoundary:
				h.BeyondBoundary[id]++
			}
		}
		if len(counts) > 0 {
			h.Blame[id] = counts
		}
	}
}

// errDiffOrder means git diff-tree's output did not follow the pairs fed.
var errDiffOrder = errors.New("git diff-tree output out of order")

// headPaths lists the paths of HEAD's tree (ls-tree takes path prefixes, not
// pathspec patterns, so the whole tree is listed).
func headPaths(ctx context.Context, repoRoot string) (map[string]bool, error) {
	args := []string{"-C", repoRoot, "-c", "core.quotePath=false", "ls-tree", "-r", "--name-only", "-z", "HEAD"}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		// A repository without commits has no HEAD tree: nothing is committed.
		return map[string]bool{}, nil //nolint:nilerr // an unborn HEAD is an empty tree
	}
	paths := map[string]bool{}
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			paths[p] = true
		}
	}
	return paths, nil
}

// worktreeDiffs reads the working tree's changes against HEAD.
func worktreeDiffs(ctx context.Context, repoRoot string, pathspec []string) (map[string]fileDiff, error) {
	args := append([]string{"-C", repoRoot, "-c", "core.quotePath=false", "diff", "-p", "-U0", "-M",
		"--no-color", "--no-ext-diff", "--full-index", "HEAD", "--"}, pathspec...)
	cmd := exec.CommandContext(ctx, "git", args...)
	// The source is only read: git diff must not refresh and rewrite the index.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	if err != nil {
		return map[string]fileDiff{}, nil //nolint:nilerr // no HEAD: handled by headPaths
	}
	r := newDiffReader(bytes.NewReader(out))
	return r.files()
}

// countLines counts the lines of a working-tree file; a last line without a
// newline counts.
func countLines(repoRoot, path string) (int, error) {
	b, err := os.ReadFile(repoRoot + "/" + path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("line history: read %s: %w", path, err)
	}
	n := bytes.Count(b, []byte{'\n'})
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n, nil
}
