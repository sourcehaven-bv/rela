package secrets

import (
	"fmt"
	"os"
	"path/filepath"
)

// OpenRemoteCredentialFile opens a remote credential file that a config
// file names. It resolves
// the symlinks in path, and the file they lead to must have a remote
// credential name (see [IsRemoteCredential]), because the name is what lets
// the value leave the machine. It must be a regular
// file with one link, because a hard link with a remote credential name can
// lead to any file on the same file system. The file is inspected before it
// is opened and compared with the opened file after, so a file swapped in
// between is refused.
func OpenRemoteCredentialFile(path string) (*os.File, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if base := filepath.Base(resolved); !IsRemoteCredential(base) {
		return nil, fmt.Errorf("file %q leads to %q: the file name must start with %q followed by a name",
			path, base, RemoteCredentialPrefix)
	}
	before, err := os.Lstat(resolved)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("file %q: not a regular file", path)
	}
	f, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !os.SameFile(before, st) {
		f.Close()
		return nil, fmt.Errorf("file %q: the file changed while it was opened", path)
	}
	n, err := linkCount(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("file %q: count hard links: %w", path, err)
	}
	if n != 1 {
		f.Close()
		return nil, fmt.Errorf("file %q: the file has %d hard links; it must have one", path, n)
	}
	return f, nil
}
