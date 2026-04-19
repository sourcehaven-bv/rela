// Package app provides factories that construct the concrete services
// needed by each rela entry point (cli, data-entry server, desktop,
// MCP). Today that is a single factory: FSFactory, which opens an
// fsstore rooted at a project directory.
package app

import (
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
//
// Optional encryption: setting Keyring + Groups enables transparent
// at-rest encryption of entity properties declared `encrypted:` in
// the metamodel. Both must be non-nil to activate; either nil leaves
// the store in cleartext-only mode.
type FSFactory struct {
	FS      storage.FS
	Paths   *project.Context
	Keyring *encryption.Keyring
	Groups  *metamodel.Groups
}

// compile-time interface check
var _ store.Factory = (*FSFactory)(nil)

// OpenStore constructs a new fsstore configured for meta's entity-type
// schemas. The returned store owns its own in-memory index and
// recentHashes cache; close it when done.
func (f *FSFactory) OpenStore(meta *metamodel.Metamodel) (store.Store, error) {
	return fsstore.New(fsstore.Config{
		FS:           f.FS,
		EntitiesDir:  f.Paths.EntitiesDir,
		RelationsDir: f.Paths.RelationsDir,
		CacheDir:     f.Paths.CacheDir,
		Schemas:      buildSchemas(meta),
		Crypto:       buildCrypto(meta, f.Groups, f.Keyring),
	})
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

// buildCrypto constructs the fsstore.Crypto adapter from the
// metamodel + groups + keyring. Returns nil when any of the three is
// missing — fsstore falls back to cleartext-only behavior.
func buildCrypto(meta *metamodel.Metamodel, groups *metamodel.Groups, kr *encryption.Keyring) fsstore.Crypto {
	if meta == nil || groups == nil || kr == nil {
		return nil
	}
	return &cryptoAdapter{meta: meta, groups: groups, keyring: kr}
}

// cryptoAdapter implements fsstore.Crypto by composing the three
// source-of-truth components. fsstore can't import metamodel or
// encryption directly at the interface level (per arch-lint), so the
// adapter lives here in app/.
type cryptoAdapter struct {
	meta    *metamodel.Metamodel
	groups  *metamodel.Groups
	keyring *encryption.Keyring
}

func (a *cryptoAdapter) PropertyGroup(entityType, property string) (string, bool) {
	def, ok := a.meta.Entities[entityType]
	if !ok {
		return "", false
	}
	prop, ok := def.Properties[property]
	if !ok {
		return "", false
	}
	if prop.Encrypted == "" {
		return "", false
	}
	return prop.Encrypted, true
}

func (a *cryptoAdapter) BodyGroup(entityType string) (string, bool) {
	def, ok := a.meta.Entities[entityType]
	if !ok {
		return "", false
	}
	return def.BodyGroup()
}

func (a *cryptoAdapter) Recipients(group string) ([]string, bool) {
	return a.groups.Recipients(group)
}

func (a *cryptoAdapter) Recipient(identity string) (*encryption.PublicKey, bool) {
	return a.keyring.Recipient(identity)
}

func (a *cryptoAdapter) HasPrivateKey() bool {
	return a.keyring.HasPrivateKey()
}

func (a *cryptoAdapter) UnwrapAny(wraps map[string][]byte) (dataKey []byte, matched string, err error) {
	if !a.keyring.HasPrivateKey() {
		return nil, "", encryption.ErrNoPrivateKey
	}
	// Try each offered wrap. Keyring.Unwrap only matches the local
	// private key, so wraps sealed for other identities return an
	// error — we skip past those and only surface a real failure if
	// a wrap the key should have opened fails.
	var corruption error
	for id, wrapped := range wraps {
		_ = id
		dk, err := a.keyring.Unwrap(wrapped)
		if err == nil {
			return dk, id, nil
		}
		// Record corruption from the first wrap that was attempted
		// against our actual private key. Keyring can't currently
		// tell us "wrong recipient" vs "corrupt" directly — if every
		// wrap fails we treat it as no-matching-key.
		corruption = err
	}
	_ = corruption
	return nil, "", encryption.ErrNoMatchingKey
}
