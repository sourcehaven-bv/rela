package userstate

import (
	"path/filepath"
	"syscall"
)

// tagNotIndexed sets FILE_ATTRIBUTE_NOT_CONTENT_INDEXED on the rela
// product directory so Windows Search skips the whole tree.
// Failure is not fatal — the caller logs at debug and continues.
func tagNotIndexed(base string) error {
	dir := filepath.Join(base, productDir)
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	const fileAttributeNotContentIndexed = 0x2000
	return syscall.SetFileAttributes(p, fileAttributeNotContentIndexed)
}
