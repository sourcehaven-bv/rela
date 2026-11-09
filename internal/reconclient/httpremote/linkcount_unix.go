//go:build unix

package httpremote

import (
	"io/fs"
	"syscall"
)

// linkCount returns the number of hard links to the file fi describes.
func linkCount(fi fs.FileInfo) (uint64, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Nlink), true //nolint:unconvert,nolintlint // Nlink is uint16 on darwin and uint64 on linux
}
