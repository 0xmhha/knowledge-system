package setup

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// ExtraDocumentRoots selects ordinary operator-owned Markdown inputs. These
// are byte-pinned documents, not verified semantic exports or policy records.
func ExtraDocumentRoots(o Options) ([]string, error) {
	if len(o.DocumentCorpora) > 16 {
		return nil, fmt.Errorf("at most 16 document corpora may be selected")
	}
	seen := map[string]bool{}
	existingRoots := []string{o.SemanticCorpus}
	if o.FlowCorpus != "" {
		existingRoots = append(existingRoots, filepath.Dir(o.FlowCorpus))
	}
	for _, root := range existingRoots {
		if root != "" {
			abs, err := filepath.Abs(root)
			if err != nil {
				return nil, err
			}
			seen[abs] = true
		}
	}
	if o.DomainKnowledge != "" {
		abs, err := filepath.Abs(o.DomainCorpusDir())
		if err != nil {
			return nil, err
		}
		seen[abs] = true
	}
	var roots []string
	for _, root := range o.DocumentCorpora {
		if strings.TrimSpace(root) == "" || len(root) > 4096 {
			return nil, fmt.Errorf("invalid document corpus root")
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		if seen[abs] {
			return nil, fmt.Errorf("duplicate document corpus root")
		}
		seen[abs] = true
		roots = append(roots, abs)
	}
	return roots, nil
}

// RetainedDocumentRoots must match CKV's docs-first, optional-flow-last order.
// Both CLI setup and registered maintenance use this one origin ordering.
func RetainedDocumentRoots(o Options) ([]string, error) {
	extra, err := ExtraDocumentRoots(o)
	if err != nil {
		return nil, err
	}
	var roots []string
	if o.DomainKnowledge != "" {
		roots = append(roots, o.DomainCorpusDir())
	}
	if o.SemanticCorpus != "" {
		roots = append(roots, o.SemanticCorpus)
	}
	roots = append(roots, extra...)
	if o.FlowCorpus != "" {
		root, duplicate := filepath.Dir(o.FlowCorpus), false
		for _, existing := range roots {
			duplicate = duplicate || filepath.Clean(existing) == filepath.Clean(root)
		}
		if !duplicate {
			roots = append(roots, root)
		}
	}
	return roots, nil
}

// VerifyDocumentCorpora rejects unreadable, linked, empty or non-text selected
// documents before embedding. Snapshot/input checks also bind their exact bytes
// before capture and before promotion; this is not a meaning/approval check.
func VerifyDocumentCorpora(ctx context.Context, roots []string, limits CaptureLimits) error {
	limits, err := limits.Normalized()
	if err != nil {
		return err
	}
	var files, visited int
	var total int64
	for _, root := range roots {
		found := false
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				return walkErr
			}
			visited++
			if visited > limits.MaxFiles*20 {
				return fmt.Errorf("%w: document traversal limit", ErrResourceLimit)
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("linked document corpus input is unavailable")
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("nonregular document corpus input is unavailable")
			}
			if !strings.EqualFold(filepath.Ext(path), ".md") {
				return nil
			}
			files++
			if files > limits.MaxFiles || info.Size() > limits.MaxFileBytes || info.Size() > limits.MaxTotalBytes-total {
				return fmt.Errorf("%w: document byte/file limit", ErrResourceLimit)
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			file, err := openCapturedNoFollow(root, rel)
			if err != nil {
				return err
			}
			body, readErr := io.ReadAll(io.LimitReader(file, limits.MaxFileBytes+1))
			closeErr := file.Close()
			if readErr != nil || closeErr != nil {
				return fmt.Errorf("document corpus input could not be read: %v %v", readErr, closeErr)
			}
			if int64(len(body)) > limits.MaxFileBytes || int64(len(body)) > limits.MaxTotalBytes-total {
				return fmt.Errorf("%w: document byte limit", ErrResourceLimit)
			}
			total += int64(len(body))
			if !utf8.Valid(body) || strings.IndexByte(string(body), 0) >= 0 {
				return fmt.Errorf("document corpus input is not UTF-8 text")
			}
			found = found || strings.TrimSpace(string(body)) != ""
			return nil
		})
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("document corpus has no nonempty Markdown")
		}
	}
	return nil
}
