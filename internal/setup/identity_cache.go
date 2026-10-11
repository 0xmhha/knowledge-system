package setup

import (
	"crypto/sha256"
	"encoding/binary"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// A retained version is immutable once published, yet every graph tool
// verified it in full on each request (about 0.6 s on the go-stablenet
// dataset: rehashing 2,227 retained blobs and reparsing a 27 MB manifest).
// The first verification in a process is kept with a fingerprint of every
// file under the version; a later request reuses it only while the
// fingerprint is unchanged. The fingerprint includes the change time, which
// a writer cannot set, so content changed behind a restored size and
// modification time is still found. Bit rot that alters no metadata goes
// unnoticed until the process restarts.

var (
	verifiedMu        sync.Mutex
	verifiedVersions  = map[string]verifiedVersion{}
	fullVerifications atomic.Int64 // full verifications run; for tests
)

type verifiedVersion struct {
	fingerprint [32]byte
	identity    DatasetIdentity
}

// cachedVersionIdentity returns the identity verified for dir when its
// fingerprint still matches.
func cachedVersionIdentity(dir string, fingerprint [32]byte) *DatasetIdentity {
	verifiedMu.Lock()
	defer verifiedMu.Unlock()
	v, ok := verifiedVersions[dir]
	if !ok || v.fingerprint != fingerprint {
		return nil
	}
	identity := v.identity
	return &identity
}

func rememberVersionIdentity(dir string, fingerprint [32]byte, identity DatasetIdentity) {
	verifiedMu.Lock()
	defer verifiedMu.Unlock()
	verifiedVersions[dir] = verifiedVersion{fingerprint: fingerprint, identity: identity}
}

// versionFingerprint hashes the path and permissions of every entry under
// dir, plus size, modification time, change time and inode of every
// non-directory, without following symbolic links. SQLite sidecars (-wal, -shm, -journal) are skipped: a
// read-only store may create and remove them, and they are not verified.
func versionFingerprint(dir string) ([32]byte, error) {
	h := sha256.New()
	var buf [8]byte
	put := func(v int64) {
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		h.Write(buf[:])
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") || strings.HasSuffix(name, "-journal") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		h.Write([]byte(rel))
		h.Write([]byte{0})
		put(int64(info.Mode()))
		if d.IsDir() {
			// A directory's times move whenever a sidecar comes and goes;
			// added or removed entries already change the walked paths.
			return nil
		}
		put(info.Size())
		put(info.ModTime().UnixNano())
		ctime, ino := fileChangeTimeAndInode(info)
		put(ctime)
		put(int64(ino))
		return nil
	})
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, err
}
