package coreml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// modelContentDigest covers bundle-relative files and the actual tokenizer.
// The model directory is immutable while an adapter is open; its location is
// not an embedding identity, and two bundles with changed bytes must not mix.
func modelContentDigest(model, tokenizer string) (string, error) {
	ext := filepath.Ext(model)
	if ext != ".mlpackage" && ext != ".mlmodelc" {
		return "", fmt.Errorf("coreml: expected a .mlpackage or .mlmodelc directory")
	}
	info, err := os.Lstat(model)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("coreml: model directory is unavailable")
	}
	h := sha256.New()
	encoder := json.NewEncoder(h)
	files := 0
	record := func(kind, path, relative string) error {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("coreml: model/tokenizer requires regular files")
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		sum := sha256.New()
		n, readErr := io.Copy(sum, f)
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if n != info.Size() {
			return fmt.Errorf("coreml: artifact changed while hashing")
		}
		return encoder.Encode(struct {
			Kind, Path, SHA256 string
			Bytes              int64
		}{kind, filepath.ToSlash(relative), hex.EncodeToString(sum.Sum(nil)), n})
	}
	err = filepath.WalkDir(model, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(model, path)
		if err != nil {
			return err
		}
		files++
		return record("model", path, relative)
	})
	if err != nil {
		return "", err
	}
	if files == 0 {
		return "", fmt.Errorf("coreml: model bundle is empty")
	}
	if err := record("tokenizer", tokenizer, "tokenizer.json"); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
