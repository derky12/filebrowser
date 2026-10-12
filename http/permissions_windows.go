//go:build windows

package fbhttp

import (
	"errors"
	"os"
)

func fileOwner(os.FileInfo) (int, int, bool) { return 0, 0, false }

func chownNoFollow(string, int, int) error { return errors.ErrUnsupported }

func canChownProcess() bool { return false }
