package fsimport

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// dataPaths are the source paths whose content the stores read, rather than
// the file copy. fsstore, filecomments and filemigstate open files through
// storage.OsFS, which follows a symlink wherever it points.
var dataPaths = []string{
	"entities",
	"relations",
	"attachments",
	"migrations/applied.json",
	".rela/comments",
	".rela/migration",
}

// checkSymlinks refuses a source with a symlink anywhere the stores read.
//
// The file copy skips symlinks itself (copyEntry), but the data copy reads
// through the stores, and a store follows a link. A cloned repository with
// attachments/DOC-1/file/key.pem pointing at ~/.ssh/id_ed25519 would
// otherwise put the key into the database, where the server serves it to
// anyone who can read DOC-1. Runs before the source is opened.
func checkSymlinks(src string, rep *Report) error {
	root, err := os.OpenRoot(src)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	before := len(rep.Errors)
	for _, top := range dataPaths {
		info, err := root.Lstat(top)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			rep.fail("%s is a symlink; the import reads only files inside the project", top)
			continue
		}
		if !info.IsDir() {
			continue
		}
		err = fs.WalkDir(root.FS(), top, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&fs.ModeSymlink != 0 {
				rep.fail("%s is a symlink; the import reads only files inside the project", p)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if n := len(rep.Errors) - before; n > 0 {
		return fmt.Errorf("%d symlink(s) in the source data; nothing was written", n)
	}
	return nil
}
