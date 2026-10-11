package setup

import (
	"io/fs"
	"syscall"
)

func fileChangeTimeAndInode(info fs.FileInfo) (int64, uint64) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0
	}
	return st.Ctim.Nano(), st.Ino
}
