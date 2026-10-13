// Package sourcepath defines collision-free paths for retained external corpora.
package sourcepath

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

const Prefix = "@cks/docs/"

func Docs(index int, file string) string { return fmt.Sprintf("%s%d/%s", Prefix, index, file) }
func Origin(index int) string            { return fmt.Sprintf("docs:corpus.%d", index) }

// Parse returns reserved=true for malformed namespaces too, so they never
// fall back to a repository file or another document root.
func Parse(file string) (index int, relative string, reserved bool, valid bool) {
	if !strings.HasPrefix(file, "@cks/") {
		return 0, file, false, true
	}
	if !strings.HasPrefix(file, Prefix) {
		return 0, "", true, false
	}
	number, relative, found := strings.Cut(strings.TrimPrefix(file, Prefix), "/")
	index, err := strconv.Atoi(number)
	valid = found && err == nil && index >= 0 && strconv.Itoa(index) == number && relative != "" &&
		!strings.HasPrefix(relative, "/") && path.Clean(relative) == relative && relative != ".." &&
		!strings.HasPrefix(relative, "../") && !strings.Contains(relative, "\\")
	return index, relative, true, valid
}
