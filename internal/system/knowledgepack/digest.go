package knowledgepack

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type FileRecord struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func DigestTree(root string) (string, error) {
	_, digest, err := inventory(root, false)
	return digest, err
}

func digestOverlay(root string) (string, error) {
	_, digest, err := inventory(root, true)
	return digest, err
}

func inventory(root string, overlay bool) ([]FileRecord, string, error) {
	var records []FileRecord
	seenFolded := map[string]bool{}
	fold := cases.Fold()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !utf8.ValidString(rel) || !norm.NFC.IsNormalString(rel) || strings.Contains(rel, "\\") ||
			strings.HasPrefix(rel, "../") || rel == ".." || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("pack_incompatible: nonportable path %q", rel)
		}
		for _, component := range strings.Split(rel, "/") {
			if component == "" || component == "." || component == ".." || strings.HasPrefix(component, ".") {
				return fmt.Errorf("pack_incompatible: hidden or unsafe path %q", rel)
			}
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pack_incompatible: linked path %q", rel)
		}
		if entry.IsDir() {
			if overlay && !validOverlayDir(rel) {
				return fmt.Errorf("pack_incompatible: unregistered overlay directory %q", rel)
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("pack_incompatible: nonregular path %q", rel)
		}
		if overlay {
			if rel == "knowledge.lock.json" {
				return nil
			}
			if !validOverlayFile(rel) {
				return fmt.Errorf("pack_incompatible: unregistered overlay file %q", rel)
			}
		}
		folded := fold.String(rel)
		if seenFolded[folded] {
			return fmt.Errorf("pack_incompatible: case-colliding path %q", rel)
		}
		seenFolded[folded] = true
		buf, err := setup.ReadSourceFileNoFollow(root, rel, 32<<20)
		if err != nil {
			return fmt.Errorf("pack_incompatible: read %q: %w", rel, err)
		}
		sum := sha256.Sum256(buf)
		records = append(records, FileRecord{Path: rel, Size: int64(len(buf)), SHA256: hex.EncodeToString(sum[:])})
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if len(records) == 0 {
		return nil, "", fmt.Errorf("pack_incompatible: empty input tree")
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	h := sha256.New()
	h.Write([]byte("cks.knowledge-files.v1\x00"))
	var length [8]byte
	for _, record := range records {
		for _, field := range []string{"regular", record.Path, fmt.Sprintf("%d", record.Size), record.SHA256} {
			binary.BigEndian.PutUint64(length[:], uint64(len(field)))
			h.Write(length[:])
			h.Write([]byte(field))
		}
	}
	return records, hex.EncodeToString(h.Sum(nil)), nil
}

func validOverlayDir(rel string) bool {
	first := strings.SplitN(rel, "/", 2)[0]
	switch first {
	case "domain", "policies", "decisions", "questions":
		return true
	}
	return false
}

func validOverlayFile(rel string) bool {
	if rel == "manifest.yaml" {
		return true
	}
	return strings.Contains(rel, "/") && validOverlayDir(strings.SplitN(rel, "/", 2)[0])
}
