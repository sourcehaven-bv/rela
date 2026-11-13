package httpremote

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

// ValidateDeclared is [Credential.Validate] for a credential a config file
// names, such as remotes.yaml. The value is sent to the URL the file names,
// so a source the file may name freely would let whoever edits it send any
// host secret off the machine: RELA_DATABASE_URL, the token key, or a file
// under $CREDENTIALS_DIRECTORY. So an env name starts with
// [secrets.RemoteCredentialEnvPrefix], and a file is an absolute path whose base name is
// a remote credential name, as a secret is. A relative path is refused
// because it would resolve against whatever directory the process runs in.
func (c Credential) ValidateDeclared() error {
	if err := c.Validate(); err != nil {
		return err
	}
	switch {
	case c.Env != "":
		// Upper case only, unlike the refusal in secrets.IsRemoteCredentialEnv:
		// a lower-case name is refused everywhere else, so it is never read.
		prefix := secrets.RemoteCredentialEnvPrefix
		if !strings.HasPrefix(c.Env, prefix) || len(c.Env) == len(prefix) {
			return fmt.Errorf("env %q: the variable name must start with %q followed by a name",
				c.Env, secrets.RemoteCredentialEnvPrefix)
		}
	case c.File != "":
		if !filepath.IsAbs(c.File) {
			return fmt.Errorf("file %q: the path must be absolute", c.File)
		}
		if !secrets.IsRemoteCredential(filepath.Base(c.File)) {
			return fmt.Errorf("file %q: the file name must start with %q followed by a name",
				c.File, secrets.RemoteCredentialPrefix)
		}
	}
	return nil
}

// GlobalSecrets returns a secret reader for [Credential.Read] over load,
// which returns the global section of the project's secrets. A project
// without secrets reads every name as empty, which Read reports as no
// credential.
//
// Nil: load is accepted; the reader then fails.
func GlobalSecrets(load func() (map[string]string, error)) func(string) (string, error) {
	return func(name string) (string, error) {
		if load == nil {
			return "", errors.New("this project has no secrets")
		}
		all, err := load()
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return "", err
		}
		return all[name], nil
	}
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
	return c.read(getenv, secret, false)
}

// ReadDeclared is [Credential.Read] for a credential a config file names. It
// checks c with [Credential.ValidateDeclared], and a file must still have a
// remote credential name once its symlinks are resolved. Without that
// check, a symlink with a remote credential name could point at any file
// the process can read, and that file would be sent to the remote. A
// symlink to a file with such a name, as a Kubernetes secret mount makes,
// is accepted. A file with more than one hard link is refused for the same
// reason.
//
// Nil: getenv and secret are accepted; the source that needs a nil one
// fails.
func (c Credential) ReadDeclared(getenv func(string) string, secret func(string) (string, error)) (string, error) {
	if err := c.ValidateDeclared(); err != nil {
		return "", err
	}
	return c.read(getenv, secret, true)
}

func (c Credential) read(
	getenv func(string) string, secret func(string) (string, error), declared bool,
) (string, error) {
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
		v, err := readCredentialFile(c.File, declared)
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
// [MaxCredentialFile] bytes and warns when other users can read it. A
// declared file goes through [secrets.OpenRemoteCredentialFile]; any other path is opened
// as it is, so an operator can name a pipe such as /dev/stdin.
func readCredentialFile(path string, declared bool) (string, error) {
	open := os.Open
	if declared {
		open = secrets.OpenRemoteCredentialFile
	}
	f, err := open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if st.Mode().Perm()&0o077 != 0 {
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
