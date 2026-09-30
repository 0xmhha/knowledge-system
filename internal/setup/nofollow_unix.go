//go:build darwin || linux

package setup

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// openCapturedNoFollow walks one path component at a time with O_NOFOLLOW.
// A symlink introduced between Lstat and Open cannot redirect the read even
// when an attacker restores the original path before the post-read check.
func openCapturedNoFollow(rootPath, rel string) (*os.File, error) {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "\\") {
		return nil, fmt.Errorf("invalid captured path")
	}
	fd, err := unix.Open(rootPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for i, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			unix.Close(fd)
			return nil, fmt.Errorf("invalid captured path component")
		}
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
		if i < strings.Count(rel, "/") {
			flags |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(fd, part, flags, 0)
		unix.Close(fd)
		if openErr != nil {
			return nil, openErr
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), rel), nil
}
