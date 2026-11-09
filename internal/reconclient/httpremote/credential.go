package httpremote

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// MaxCredentialFile caps a credential file [Credential.Read] reads.
const MaxCredentialFile = 16 << 10

// Credential names where the credential for a remote comes from: an
// environment variable, a file, or a global project secret under
// [secrets.RemoteCredentialPrefix]. At most one is set. The value is never
// part of the configuration, so it does not end up in a config file, in
// ps or in shell history, and a credential is never picked up from a
// default and sent to whatever URL the caller names.
type Credential struct {
	Env    string `yaml:"env,omitempty"`
	File   string `yaml:"file,omitempty"`
	Secret string `yaml:"secret,omitempty"`
}

// Credential sources, as [Credential.Kind] reports them.
const (
	CredentialEnv    = "env"
	CredentialFile   = "file"
	CredentialSecret = "secret"
)

// IsZero reports whether c names no source.
func (c Credential) IsZero() bool { return c == Credential{} }

// Kind names the source c reads, or "" when it names none.
func (c Credential) Kind() string {
	switch {
	case c.Secret != "":
		return CredentialSecret
	case c.File != "":
		return CredentialFile
	case c.Env != "":
		return CredentialEnv
	}
	return ""
}

// Validate checks that c names at most one source, and that a secret is a
// remote credential. Only a name under [secrets.RemoteCredentialPrefix]
// is accepted, because no script can be granted one.
func (c Credential) Validate() error {
	n := 0
	for _, v := range []string{c.Env, c.File, c.Secret} {
		if v != "" {
			n++
		}
	}
	if n > 1 {
		return errors.New("name one credential source: env, file or secret")
	}
	if c.Secret != "" && !secrets.IsRemoteCredential(c.Secret) {
		return fmt.Errorf("the secret name must start with %q followed by a name, so that no script can read it",
			secrets.RemoteCredentialPrefix)
	}
	return nil
}

// Read returns the credential. getenv reads an environment variable;
// secret reads a value from the global section of the project's secrets.
// A source that is set but empty is an error, so a missing credential is
// never sent as no credential. No source returns "" and no error. Errors
// never hold the value.
//
// Nil: getenv and secret are accepted; the source that needs a nil one
// fails.
func (c Credential) Read(getenv func(string) string, secret func(string) (string, error)) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	var value string
	switch c.Kind() {
	case CredentialSecret:
		if secret == nil {
			return "", errors.New("this project has no secrets")
		}
		v, err := secret(c.Secret)
		if err != nil {
			return "", err
		}
		value = v
	case CredentialFile:
		v, err := readCredentialFile(c.File)
		if err != nil {
			return "", err
		}
		value = v
	case CredentialEnv:
		if getenv == nil {
			return "", errors.New("no environment to read")
		}
		value = getenv(c.Env)
	default:
		return "", nil
	}
	if value = strings.TrimSpace(value); value == "" {
		return "", errors.New("no credential (empty or unset)")
	}
	return value, nil
}

// warnedFiles holds the credential files, with their mode, already reported
// as readable by other users. It grows by one entry per such file and mode,
// which the operator's configuration bounds.
var warnedFiles sync.Map

// readCredentialFile reads a credential file of at most
// [MaxCredentialFile] bytes and warns when other users can read it.
func readCredentialFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if st, sErr := f.Stat(); sErr == nil && st.Mode().Perm()&0o077 != 0 {
		// Once per file and mode: a scheduled pull reads the file on every
		// run.
		if _, seen := warnedFiles.LoadOrStore(path+"\x00"+st.Mode().Perm().String(), true); !seen {
			slog.Warn("the credential file can be read by other users", "file", path,
				"mode", st.Mode().Perm().String())
		}
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxCredentialFile+1))
	if err != nil {
		return "", err
	}
	if len(b) > MaxCredentialFile {
		return "", fmt.Errorf("the file is larger than %d bytes", MaxCredentialFile)
	}
	return string(b), nil
}
