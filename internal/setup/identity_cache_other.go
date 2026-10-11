//go:build !darwin && !linux

package setup

import "io/fs"

// Without a change time the fingerprint still covers size, permissions and
// modification time.
func fileChangeTimeAndInode(fs.FileInfo) (int64, uint64) { return 0, 0 }
