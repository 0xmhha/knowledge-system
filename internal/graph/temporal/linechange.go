package temporal

import (
	"bytes"
	"math"
)

// LineChange is one change of a unified diff without context lines:
// OldCount lines from OldStart in the parent became NewCount lines from
// NewStart in the commit. A zero count follows git's -U0 header convention:
// the start is then the line after which lines were inserted (OldCount 0) or
// deleted (NewCount 0).
type LineChange struct {
	OldStart, OldCount int
	NewStart, NewCount int
}

// Touches reports whether the change alters the commit-side lines
// [start, end] (1-based, inclusive): a line it added or replaced lies in the
// range, or it deleted lines between two lines of the range. Lines deleted
// just before or just after the range do not count.
//
// This is the one rule the build uses for "a commit changed these lines": it
// decides a symbol's line history (changed_in), which hunk modified which
// symbol (modifies) and, applied to every line, blame. It is the rule of
// git log -L.
func (c LineChange) Touches(start, end int) bool {
	if c.NewCount > 0 {
		return c.NewStart <= end && start <= c.newEnd()
	}
	return start <= c.NewStart && c.NewStart < end
}

// newEnd is the last commit-side line of a change that added lines.
func (c LineChange) newEnd() int { return c.NewStart + c.NewCount - 1 }

// endsBefore reports whether the change lies wholly before commit-side line x,
// so that it shifts x without touching it.
func (c LineChange) endsBefore(x int) bool {
	if c.NewCount > 0 {
		return c.newEnd() < x
	}
	return c.NewStart < x
}

// shift is how many lines the change adds on the commit side.
func (c LineChange) shift() int { return c.NewCount - c.OldCount }

// lineRange is a 1-based inclusive range of lines; it is empty when
// Start > End.
type lineRange struct{ Start, End int }

func (r lineRange) empty() bool { return r.Start > r.End }

// emptyRange is the identity of hull.
var emptyRange = lineRange{Start: math.MaxInt, End: math.MinInt}

// hull widens r to cover [a, b].
func (r lineRange) hull(a, b int) lineRange {
	return lineRange{Start: min(r.Start, a), End: max(r.End, b)}
}

// mapToParent maps the commit-side range r through changes (sorted by
// NewStart, as a diff lists them) to the parent side, the way git log -L
// follows a range: the lines no change touched move by the changes before
// them, and every change that touches the range brings its whole parent side
// along. touched reports whether any change touched r; the result is empty
// when every line of r was added by the commit.
func mapToParent(changes []LineChange, r lineRange) (parent lineRange, touched bool) {
	parent = emptyRange
	off, cur := 0, r.Start
	for _, c := range changes {
		if c.endsBefore(r.Start) {
			off += c.shift()
			continue
		}
		if !c.Touches(r.Start, r.End) {
			break
		}
		touched = true
		next := c.NewStart + 1 // a deletion sits after line NewStart
		last := c.NewStart     // unchanged lines before a deletion end at NewStart
		if c.NewCount > 0 {
			next, last = c.newEnd()+1, c.NewStart-1
		}
		if cur <= last {
			parent = parent.hull(cur-off, last-off)
		}
		if c.OldCount > 0 {
			parent = parent.hull(c.OldStart, c.OldStart+c.OldCount-1)
		}
		cur = max(cur, next)
		off += c.shift()
	}
	if cur <= r.End {
		parent = parent.hull(cur-off, r.End-off)
	}
	return parent, touched
}

// segment is a run of lines still unattributed by blame: n lines that are
// lines head.. of working-tree file file and lines cur.. of the commit being
// visited.
type segment struct {
	file         string
	head, cur, n int
}

// splitSegment splits s by changes (sorted by NewStart) into the lines the
// changes left alone, mapped to the parent side, and the lines a change added
// or replaced, still in commit-side coordinates.
func splitSegment(changes []LineChange, s segment) (kept, changed []segment) {
	off, cur, end := 0, s.cur, s.cur+s.n-1
	emit := func(dst *[]segment, from, to, delta int) {
		if from > to {
			return
		}
		*dst = append(*dst, segment{file: s.file, head: s.head + (from - s.cur), cur: from - delta, n: to - from + 1})
	}
	for _, c := range changes {
		if cur > end {
			break
		}
		if c.endsBefore(cur) {
			off += c.shift()
			continue
		}
		if c.NewCount == 0 {
			// A deletion changes no line of the commit; it moves the lines
			// after it.
			if c.NewStart >= end {
				break
			}
			emit(&kept, cur, c.NewStart, off)
			cur = c.NewStart + 1
			off += c.shift()
			continue
		}
		if c.NewStart > end {
			break
		}
		emit(&kept, cur, min(end, c.NewStart-1), off)
		from, to := max(cur, c.NewStart), min(end, c.newEnd())
		emit(&changed, from, to, 0)
		cur = to + 1
		off += c.shift()
	}
	emit(&kept, cur, end, off)
	return kept, changed
}

// PatchChanges reads the changes of one unified-diff hunk with context lines
// (an "@@" header and its body, as HunkInfo.Patch holds it): each run of
// removed and added lines is one change, numbered like a -U0 header, so a
// hunk's context lines never count as changed. ok is false when the patch
// has no header.
func PatchChanges(patch []byte) (changes []LineChange, ok bool) {
	lines := bytes.Split(patch, []byte("\n"))
	if len(lines) == 0 {
		return nil, false
	}
	oldLine, _, newLine, _, ok := parseHunkHeader(string(lines[0]))
	if !ok {
		return nil, false
	}
	var cur *LineChange
	flush := func() {
		if cur == nil {
			return
		}
		if cur.OldCount == 0 {
			cur.OldStart = oldLine - 1
		}
		if cur.NewCount == 0 {
			cur.NewStart = newLine - 1
		}
		changes = append(changes, *cur)
		cur = nil
	}
	for _, l := range lines[1:] {
		if len(l) == 0 {
			continue
		}
		switch l[0] {
		case '-':
			if cur == nil {
				cur = &LineChange{}
			}
			if cur.OldCount == 0 {
				cur.OldStart = oldLine
			}
			cur.OldCount++
			oldLine++
		case '+':
			if cur == nil {
				cur = &LineChange{}
			}
			if cur.NewCount == 0 {
				cur.NewStart = newLine
			}
			cur.NewCount++
			newLine++
		case '\\':
		default:
			flush()
			oldLine++
			newLine++
		}
	}
	flush()
	return changes, true
}
