package userstate

import (
	"os"
	"path/filepath"
)

// tagNotIndexed writes an empty .metadata_never_index file under
// the rela product directory so Spotlight skips the whole tree.
// The file is .gitignored in spirit (it never lives inside the
// project tree) and overwriting a zero-byte marker is cheap, so we
// tolerate "already exists" as success.
//
// base is the OS user-config directory; we place the marker at
// base/rela/ so every per-repo subdirectory inherits the skip.
func tagNotIndexed(base string) error {
	marker := filepath.Join(base, productDir, ".metadata_never_index")
	f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, stateFilePerm)
	if err != nil {
		return err
	}
	return f.Close()
}
