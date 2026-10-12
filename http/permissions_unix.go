//go:build !windows

package fbhttp

import (
	"os"
	"syscall"
)

// fileOwner returns the numeric owner and group of a file.
func fileOwner(info os.FileInfo) (int, int, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}

	return int(stat.Uid), int(stat.Gid), true
}

// chownNoFollow changes the owner of the file itself, never of a symlink target.
func chownNoFollow(path string, uid, gid int) error {
	return os.Lchown(path, uid, gid)
}

// canChownProcess reports whether the process may give files to other users.
func canChownProcess() bool {
	return os.Geteuid() == 0
}
