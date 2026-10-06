package setup

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// ErrDurabilityUncertain means current may already name the complete candidate.
// Callers must inspect/retry rather than infer an unchanged pointer or roll back.
var ErrDurabilityUncertain = errors.New("durability_uncertain")

// Only package tests inject failures. No public environment flag can disable
// persistence. Tests changing this hook must not run in parallel.
var durabilityFault func(stage, path string) error

func durabilityStep(stage, path string) error {
	if durabilityFault != nil {
		return durabilityFault(stage, path)
	}
	return nil
}

func syncDirectory(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), path)
	syncErr := f.Sync()
	return errors.Join(syncErr, f.Close())
}

func syncArtifact(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), path)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 {
		_ = f.Close()
		return fmt.Errorf("candidate artifact must be a regular file with one link")
	}
	return errors.Join(f.Sync(), f.Close())
}

func sqliteArtifact(path string) (bool, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return false, err
	}
	f := os.NewFile(uintptr(fd), path)
	header := make([]byte, 16)
	_, readErr := io.ReadFull(f, header)
	closeErr := f.Close()
	if closeErr != nil {
		return false, closeErr
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return false, readErr
	}
	return bytes.Equal(header, []byte("SQLite format 3\x00")), nil
}

func checkpointArtifact(path string) error {
	// Only mutable SQLite database artifacts, never retained source blobs, reach here.
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite3", u.String()+"?mode=rw&_busy_timeout=100")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	var busy, log, checkpointed int
	err = db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &log, &checkpointed)
	if err == nil && (busy != 0 || log > checkpointed) {
		err = fmt.Errorf("candidate checkpoint busy or incomplete")
	}
	closeErr := db.Close()
	return errors.Join(err, closeErr, durabilityStep("checkpoint-close", path))
}

// flushCandidate is called under the common mutation lock, after engines have
// exited. Inventory first prevents following a linked artifact while opening DBs.
// Recheck identity after checkpointing: persistence cannot silently change pins.
func flushCandidate(versionDir string) error {
	var dirs, files []string
	err := filepath.WalkDir(versionDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("candidate contains linked or nonregular artifact")
		}
		var stat unix.Stat_t
		if err := unix.Lstat(path, &stat); err != nil || stat.Nlink != 1 {
			return fmt.Errorf("candidate artifact must have one link")
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}
	var databases []string
	for _, path := range files {
		rel, _ := filepath.Rel(versionDir, path)
		if !strings.HasPrefix(rel, "sources"+string(filepath.Separator)) {
			isDB, err := sqliteArtifact(path)
			if err != nil {
				return err
			}
			if !isDB {
				if filepath.Ext(path) == ".db" {
					return fmt.Errorf("candidate database is not SQLite")
				}
				continue
			}
			databases = append(databases, path)
		}
	}
	// Detect every DB before any checkpoint removes its inventoried sidecars.
	for _, path := range databases {
		if err := durabilityStep("checkpoint", path); err != nil {
			return err
		}
		if err := checkpointArtifact(path); err != nil {
			return fmt.Errorf("checkpoint %s: %w", path, err)
		}
	}
	if _, err := verifyVersionIdentityIfPresent(versionDir); err != nil {
		return err
	}
	// Vector DB pins are physical bytes established after its engine checkpoint.
	// A changed DB cannot be made valid by rewriting a manifest during promotion.
	var manifest struct {
		DBSHA256 string `json:"db_sha256"`
	}
	if err := readJSON(filepath.Join(versionDir, "vector", "manifest.json"), &manifest); err != nil && !os.IsNotExist(err) {
		return err
	}
	if manifest.DBSHA256 != "" {
		f, err := os.Open(filepath.Join(versionDir, "vector", "vector.db"))
		if err != nil {
			return err
		}
		h := sha256.New()
		_, hashErr := io.Copy(h, f)
		if err := errors.Join(hashErr, f.Close()); err != nil {
			return err
		}
		if hex.EncodeToString(h.Sum(nil)) != manifest.DBSHA256 {
			return fmt.Errorf("candidate vector database differs from pinned bytes")
		}
	}
	// Checkpoint may remove sidecars. Re-enumerate all artifacts for file sync.
	err = filepath.WalkDir(versionDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if err := durabilityStep("candidate-file", path); err != nil {
			return err
		}
		return syncArtifact(path)
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := durabilityStep("candidate-dir", dirs[i]); err != nil {
			return err
		}
		if err := syncDirectory(dirs[i]); err != nil {
			return err
		}
	}
	parent := filepath.Dir(versionDir)
	if err := durabilityStep("candidate-parent", parent); err != nil {
		return err
	}
	return syncDirectory(parent)
}

func syncCurrentParent(dataset string) error {
	if err := durabilityStep("current-parent", dataset); err != nil {
		return fmt.Errorf("%w: %v", ErrDurabilityUncertain, err)
	}
	if err := syncDirectory(dataset); err != nil {
		return fmt.Errorf("%w: %v", ErrDurabilityUncertain, err)
	}
	if err := syncDirectory(filepath.Dir(dataset)); err != nil {
		return fmt.Errorf("%w: %v", ErrDurabilityUncertain, err)
	}
	return nil
}
