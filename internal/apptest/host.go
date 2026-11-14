package apptest

import (
	"fmt"
	"io/fs"
	"maps"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// testSecrets are the fake secret values a case declares with t.secrets.
// Safe for concurrent use: runtimes read them while the case sets them.
type testSecrets struct {
	mu     sync.Mutex
	values map[string]string
}

// set adds values, overwriting a name already set.
func (s *testSecrets) set(values map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = map[string]string{}
	}
	maps.Copy(s.values, values)
}

// all returns a copy of the values.
func (s *testSecrets) all() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.values)
}

// testHost is the host config of a case's world: mail.yaml from the
// project's .rela directory, as the default in-memory world serves it, and
// the case's fake secrets. .rela/secrets.yaml is never read, so a case, its
// recorded requests and its report cannot carry a real secret.
type testHost struct {
	dir     hostconfig.Dir
	secrets *testSecrets
}

// File serves mail.yaml and reports every other file as absent.
func (h testHost) File(name string) ([]byte, error) {
	if name != hostconfig.MailFile {
		return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
	}
	return h.dir.File(name)
}

// Secrets returns the case's fake secrets to every script; the script's
// capabilities still pick the names it sees.
func (h testHost) Secrets(string) (map[string]string, error) {
	v := h.secrets.all()
	if len(v) == 0 {
		return nil, secrets.ErrNotFound
	}
	return v, nil
}

// Path is the .rela directory.
func (h testHost) Path() string { return h.dir.Path() }
