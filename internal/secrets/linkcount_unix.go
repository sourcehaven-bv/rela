//go:build unix

package secrets

import (
	"errors"
	"os"
	"syscall"
)

// linkCount returns the number of hard links to the open file f.
func linkCount(f *os.File) (uint64, error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("the file system reports no link count")
	}
	return uint64(st.Nlink), nil //nolint:unconvert,nolintlint // Nlink is uint16 on darwin and uint64 on linux
}
