package appbuild

import (
	"context"
	"io"
	"io/fs"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// newTemplater builds the entity/relation templater. With project config in
// a database it reads templates/ through the layered view, so a template the
// database carries is used when the file is absent; generated templates are
// still written to disk, where an operator edits them.
func newTemplater(cfg Config) (templating.Templater, error) {
	if cfg.projectConfig == nil {
		return templating.NewFSTemplater(cfg.FS, cfg.Paths), nil
	}
	view, err := config.NewStorageFS(context.Background(), cfg.projectConfig, cfg.Paths.Root)
	if err != nil {
		return nil, err
	}
	return templating.NewFSTemplater(readThroughFS{FS: cfg.FS, view: view}, cfg.Paths), nil
}

// readThroughFS reads through a project config view and writes to the disk
// filesystem it embeds.
type readThroughFS struct {
	storage.FS
	view *config.StorageFS
}

func (r readThroughFS) ReadFile(path string) ([]byte, error)       { return r.view.ReadFile(path) }
func (r readThroughFS) Open(path string) (io.ReadCloser, error)    { return r.view.Open(path) }
func (r readThroughFS) Stat(path string) (os.FileInfo, error)      { return r.view.Stat(path) }
func (r readThroughFS) ReadDir(path string) ([]os.DirEntry, error) { return r.view.ReadDir(path) }
func (r readThroughFS) Walk(root string, fn fs.WalkDirFunc) error  { return r.view.Walk(root, fn) }
