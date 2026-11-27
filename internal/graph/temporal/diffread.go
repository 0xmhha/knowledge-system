package temporal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// fileDiff is one file of a diff with no context lines: the file's path in
// the parent (differs from its path in the commit after a rename), whether
// the commit created it, and its changes in diff order.
type fileDiff struct {
	oldPath string
	added   bool
	// similarity is the similarity index of a rename or copy, in percent.
	similarity int
	// copied marks a file the commit created as a copy of oldPath, which
	// stays in place (a rename removes it).
	copied  bool
	changes []LineChange
}

// diffReadBufferBytes sizes the diff reader. Only header lines are parsed, so
// a longer (changed) line is read in pieces and skipped.
const diffReadBufferBytes = 64 << 10

// shaHexLen is the length of a full commit id in hex.
const shaHexLen = 40

// diffReader parses the output of git diff-tree --stdin --always -p -U0 (one
// block per input line, each opened by the commit id) or of git diff -U0
// (one block, no id line).
type diffReader struct {
	br      *bufio.Reader
	pending string // a block's id line read ahead, "" when none
	eof     bool
}

func newDiffReader(r io.Reader) *diffReader {
	return &diffReader{br: bufio.NewReaderSize(r, diffReadBufferBytes)}
}

// line returns the next line without its newline, the first piece of a line
// longer than the buffer (the rest is dropped), and io.EOF at the end.
func (r *diffReader) line() (string, error) {
	chunk, err := r.br.ReadSlice('\n')
	s := string(bytes.TrimRight(chunk, "\n"))
	for errors.Is(err, bufio.ErrBufferFull) {
		_, err = r.br.ReadSlice('\n')
	}
	if err == io.EOF && len(chunk) > 0 {
		return s, nil
	}
	return s, err
}

// next reads the block of the pair whose commit is sha.
func (r *diffReader) next(sha string) (map[string]fileDiff, error) {
	head := r.pending
	r.pending = ""
	if head == "" && !r.eof {
		l, err := r.line()
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read git diff-tree: %w", err)
		}
		r.eof = err == io.EOF
		head = l
	}
	if head != sha {
		return nil, fmt.Errorf("%w: want block of %s, got %q", errDiffOrder, sha, head)
	}
	return r.files()
}

// isIDLine reports whether l is a block's opening commit id.
func isIDLine(l string) bool {
	if len(l) != shaHexLen {
		return false
	}
	for i := 0; i < len(l); i++ {
		c := l[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// files reads file diffs up to the next id line or the end.
func (r *diffReader) files() (map[string]fileDiff, error) {
	out := map[string]fileDiff{}
	var cur *fileDiff
	var curPath, renameTo string
	// bodyOld and bodyNew count the body lines of the current change still
	// to come; a body line may look like a header ("--- x" removes "-- x").
	bodyOld, bodyNew := 0, 0
	flush := func() {
		if cur == nil {
			return
		}
		p := curPath
		if renameTo != "" {
			p = renameTo
		}
		if p != "" {
			if cur.oldPath == "" {
				cur.oldPath = p
			}
			out[p] = *cur
		}
		cur, curPath, renameTo = nil, "", ""
	}
	for !r.eof {
		l, err := r.line()
		if err == io.EOF {
			r.eof = true
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read git diff: %w", err)
		}
		if bodyOld > 0 || bodyNew > 0 {
			switch {
			case strings.HasPrefix(l, "-"):
				bodyOld--
			case strings.HasPrefix(l, "+"):
				bodyNew--
			case strings.HasPrefix(l, `\`):
			default:
				bodyOld, bodyNew = 0, 0 // malformed: resynchronise on headers
			}
			continue
		}
		switch {
		case isIDLine(l):
			r.pending = l
			flush()
			return out, nil
		case strings.HasPrefix(l, "diff --git "):
			flush()
			cur = &fileDiff{}
			curPath = parseDiffGitPath(l)
		case cur == nil:
			continue
		case strings.HasPrefix(l, "@@ "):
			oldStart, oldCount, newStart, newCount, ok := parseHunkHeader(l)
			if ok {
				cur.changes = append(cur.changes, LineChange{OldStart: oldStart, OldCount: oldCount, NewStart: newStart, NewCount: newCount})
				bodyOld, bodyNew = oldCount, newCount
			}
		case strings.HasPrefix(l, `\`):
			continue // "\ No newline at end of file" after a body
		case strings.HasPrefix(l, "new file mode"):
			cur.added = true
		case strings.HasPrefix(l, "rename from "):
			cur.oldPath = unquotePath(strings.TrimPrefix(l, "rename from "))
		case strings.HasPrefix(l, "rename to "):
			renameTo = unquotePath(strings.TrimPrefix(l, "rename to "))
		case strings.HasPrefix(l, "similarity index "):
			cur.similarity, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(l, "similarity index "), "%"))
		case strings.HasPrefix(l, "copy from "):
			cur.oldPath = unquotePath(strings.TrimPrefix(l, "copy from "))
		case strings.HasPrefix(l, "copy to "):
			renameTo = unquotePath(strings.TrimPrefix(l, "copy to "))
		case strings.HasPrefix(l, "--- "):
			if p := diffSidePath(strings.TrimPrefix(l, "--- "), "a/"); p != "" && cur.oldPath == "" {
				cur.oldPath = p
			}
		case strings.HasPrefix(l, "+++ "):
			if p := diffSidePath(strings.TrimPrefix(l, "+++ "), "b/"); p != "" {
				curPath = p
			}
		}
	}
	flush()
	return out, nil
}

// diffSidePath reads the path of a ---/+++ line: "" for /dev/null.
func diffSidePath(s, prefix string) string {
	s = strings.TrimSuffix(s, "\t")
	if s == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(unquotePath(s), prefix)
}

// unquotePath undoes git's C-style quoting of a path with unusual characters.
func unquotePath(s string) string {
	if len(s) >= 2 && s[0] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
	}
	return s
}
