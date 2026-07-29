package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VerifyAnchors checks every expected citation that declares an anchor:
// the anchor substring must occur within the citation's [StartLine,
// EndLine] slice of root/File. Returns one error per violation
// (missing file, span out of range, anchor not found) so a drifted
// scenario fails LOUD instead of silently scoring zero against stale
// line numbers.
func (s *Scenario) VerifyAnchors(root string) []error {
	var errs []error
	for i, c := range s.ExpectedCitations {
		if i >= len(s.Anchors) || s.Anchors[i] == "" {
			continue
		}
		anchor := s.Anchors[i]
		path := filepath.Join(root, filepath.FromSlash(c.File))
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: citation %s:%d-%d: read: %w",
				s.Name, c.File, c.StartLine, c.EndLine, err))
			continue
		}
		lines := strings.Split(string(data), "\n")
		if c.StartLine < 1 || c.StartLine > len(lines) {
			errs = append(errs, fmt.Errorf("%s: citation %s:%d-%d: start_line out of range (file has %d lines)",
				s.Name, c.File, c.StartLine, c.EndLine, len(lines)))
			continue
		}
		end := c.EndLine
		if end > len(lines) {
			end = len(lines)
		}
		span := strings.Join(lines[c.StartLine-1:end], "\n")
		if !strings.Contains(span, anchor) {
			errs = append(errs, fmt.Errorf("%s: citation %s:%d-%d: anchor %q not found in span — expected lines have drifted; re-anchor the scenario",
				s.Name, c.File, c.StartLine, c.EndLine, anchor))
		}
	}
	return errs
}
