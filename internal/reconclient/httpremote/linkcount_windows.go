//go:build windows

package httpremote

import (
	"os"
	"syscall"
)

// linkCount returns the number of hard links to the open file f.
func linkCount(f *os.File) (uint64, error) {
	var d syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &d); err != nil {
		return 0, err
	}
	return uint64(d.NumberOfLinks), nil
}
