package markdown

import (
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// FileIO provides file I/O operations for markdown entity and relation files.
// It wraps a storage.FS to allow swapping the filesystem (e.g., for tests).
type FileIO struct {
	FS storage.FS
}

// NewFileIO creates a FileIO backed by the given filesystem.
func NewFileIO(fs storage.FS) *FileIO {
	return &FileIO{FS: fs}
}
