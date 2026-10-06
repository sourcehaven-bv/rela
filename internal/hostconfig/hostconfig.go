// Package hostconfig supplies what a project reads from the machine it runs
// on rather than from its config: its secrets, and the AI and mail settings
// (ai.yaml, mail.yaml).
//
// The CLI and rela-server read all of it from the project's .rela directory,
// which is what [Dir] does. The desktop app keeps secrets in the OS keychain
// and the settings in the project's database, so a single-file document
// carries everything but its secrets. Both go through the same two methods,
// so the readers (the Lua runtime, mail, AI) do not know which they have.
//
// Consumers declare their own narrow interface over these methods; this
// package only provides the directory-backed implementation and the names.
package hostconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// The host config files a project may have. Only these names are served:
// a Source is not a general file reader.
const (
	AIFile   = "ai.yaml"
	MailFile = "mail.yaml"
)

// ErrUnknownFile is returned for a name that is not a host config file.
var ErrUnknownFile = errors.New("hostconfig: not a host config file")

// CheckName reports whether name is one of the host config files.
func CheckName(name string) error {
	if name != AIFile && name != MailFile {
		return fmt.Errorf("%w: %q", ErrUnknownFile, name)
	}
	return nil
}

// Dir reads host config from a project's .rela directory: secrets through
// [secrets.Load] (systemd credentials, then secrets.yaml) and the files
// directly.
type Dir string

// Secrets returns the secrets visible to scriptPath; "" gives the global
// ones. secrets.ErrNotFound when the project has none.
func (d Dir) Secrets(scriptPath string) (map[string]string, error) {
	return secrets.Load(string(d), scriptPath)
}

// File returns the named host config file. An error wrapping fs.ErrNotExist
// means the project does not have it.
func (d Dir) File(name string) ([]byte, error) {
	if err := CheckName(name); err != nil {
		return nil, err
	}
	if d == "" {
		return nil, fmt.Errorf("hostconfig: no .rela directory: %w", fs.ErrNotExist)
	}
	return os.ReadFile(filepath.Join(string(d), name))
}

// Path returns the .rela directory itself. Mail resolves a relative send
// script against its parent, the project root.
func (d Dir) Path() string { return string(d) }
