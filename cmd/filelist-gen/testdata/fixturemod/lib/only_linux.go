//go:build linux

package lib

// LinuxOnly exists to prove the pinned build context governs file
// resolution: present under GOOS=linux, absent under GOOS=darwin.
func LinuxOnly() {}
