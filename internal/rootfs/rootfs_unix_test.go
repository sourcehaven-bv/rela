//go:build unix

package rootfs_test

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/rootfs"
)

// Only regular files are read: opening a FIFO would block the caller.
func TestDir_RefusesNonRegularFiles(t *testing.T) {
	root := project(t)
	if err := syscall.Mkfifo(filepath.Join(root, "custom", "pipe.css"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	if _, err := rootfs.New(root).Load(context.Background(), "custom/pipe.css"); err == nil {
		t.Fatal("read a FIFO")
	}
}
