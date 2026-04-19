package fsstore

import (
	"io/fs"

	"github.com/Sourcehaven-BV/rela/internal/storage/integrity"
)

// dirReader combines the fsstore DirFS + RawReader handles into the
// integrity.FSReader surface. Both handles came from the same raw
// filesystem in New, so bundling them here is equivalent to passing
// the original storage.FS while keeping the narrow types at their
// normal call sites.
type dirReader struct {
	dirs DirFS
	raw  RawReader
}

func (d dirReader) Stat(path string) (fs.FileInfo, error)      { return d.dirs.Stat(path) }
func (d dirReader) ReadDir(path string) ([]fs.DirEntry, error) { return d.dirs.ReadDir(path) }
func (d dirReader) ReadFile(path string) ([]byte, error)       { return d.raw.ReadFile(path) }

// verifyEncryptionConsistency delegates to integrity.Verify using
// the raw filesystem handle and the declared wantSealed mode. The
// verifier lives in its own package because the check is
// layer-crossing (inspects on-disk bytes, not fsstore's plaintext
// view) and must run before fsstore.New returns a usable store.
func (s *FSStore) verifyEncryptionConsistency() error {
	dirs := []string{s.entitiesDir, s.relationsDir}
	if s.attachDir != "" {
		dirs = append(dirs, s.attachDir)
	}
	return integrity.Verify(dirReader{dirs: s.dirs, raw: s.rawReader}, s.wantSealed, dirs)
}
