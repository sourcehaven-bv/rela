// Package app provides factories that construct the concrete services
// needed by each rela entry point (cli, data-entry server, desktop,
// MCP). Today that is a single factory: FSFactory, which opens an
// fsstore rooted at a project directory.
package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// FSFactory is a store.Factory that opens filesystem-backed stores
// (fsstore) rooted at the given project paths. Each OpenStore call
// returns a fresh, independent store — callers that want a single
// long-lived store should open it once and keep it alive.
type FSFactory struct {
	FS    storage.FS
	Paths *project.Context
}

// compile-time interface check
var _ store.Factory = (*FSFactory)(nil)

// OpenStore constructs a new fsstore configured for meta's entity-type
// schemas. If .rela/encryption.yaml exists, the factory loads the
// keyring from <root>/keys and the local identity via the standard
// precedence chain, then installs an age Crypto. Otherwise it
// installs IdentityCrypto (cleartext mode).
func (f *FSFactory) OpenStore(meta *metamodel.Metamodel) (store.Store, error) {
	crypto, err := f.loadCrypto()
	if err != nil {
		return nil, err
	}
	return fsstore.New(fsstore.Config{
		FS:           f.FS,
		EntitiesDir:  f.Paths.EntitiesDir,
		RelationsDir: f.Paths.RelationsDir,
		CacheDir:     f.Paths.CacheDir,
		Schemas:      buildSchemas(meta),
		Crypto:       crypto,
	})
}

// loadCrypto returns the Crypto to pass to fsstore.New, based on the
// presence of .rela/encryption.yaml.
func (f *FSFactory) loadCrypto() (fsstore.Crypto, error) {
	cfgPath := filepath.Join(f.Paths.CacheDir, encryption.ConfigFileName)
	if _, err := os.Stat(cfgPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fsstore.IdentityCrypto(), nil
		}
		return nil, fmt.Errorf("app: stat %s: %w", cfgPath, err)
	}
	kr, err := encryption.LoadFromDir(f.Paths.Root)
	if err != nil {
		return nil, fmt.Errorf("app: load keyring: %w", err)
	}
	return fsstore.NewAgeCrypto(kr), nil
}

// buildSchemas translates metamodel entity-type definitions into the
// store-facing EntityTypeSchema map used by fsstore.
func buildSchemas(meta *metamodel.Metamodel) map[string]store.EntityTypeSchema {
	if meta == nil {
		return nil
	}
	out := make(map[string]store.EntityTypeSchema, len(meta.Entities))
	for name, et := range meta.Entities {
		out[name] = store.EntityTypeSchema{
			Plural:        et.Plural,
			PropertyOrder: et.PropertyOrder,
		}
	}
	return out
}
