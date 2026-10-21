package storage

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"
)

// ValidateDirSegment reports whether name is safe as one directory name
// below a fixed parent, such as an entity type's directory under
// entities/. A key is cleaned before [RootedFS] checks it, so a name like
// "../relations" would stay inside the project root but leave its parent;
// this check refuses it before it reaches a key.
//
// It refuses only what can escape the parent or behave as something other
// than one plain directory: an empty name, '/' or '\', "." and "..", a
// leading '.' (hidden, and the start of every relative escape), control
// characters including NUL, and any name filepath.Base or filepath.Clean
// would change. Spaces, accents, uppercase letters and inner dots are
// allowed, so existing projects keep loading.
func ValidateDirSegment(name string) error {
	switch {
	case name == "":
		return errors.New("must not be empty")
	case strings.ContainsAny(name, `/\`):
		return errors.New(`must not contain '/' or '\'`)
	case strings.HasPrefix(name, "."):
		return errors.New("must not start with '.'")
	case strings.ContainsFunc(name, unicode.IsControl):
		return errors.New("must not contain control characters")
	case filepath.Base(name) != name || filepath.Clean(name) != name:
		return errors.New("must be a single directory name")
	}
	return nil
}
