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
	"github.com/Sourcehaven-BV/rela/internal/encryption/cryptofs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// ErrEncryptedRepoNeedsIdentity is returned by OpenStore when the
// repository is encryption-enabled (.rela/encryption.yaml exists) but
// the caller has no local age identity configured ($RELA_KEY_FILE,
// .rela/key, ~/.config/rela/key all absent or unreadable).
//
// Opening an encrypted store without an identity would silently cripple
// every read path (GetEntity, ListEntities, rebuildPropCache all swallow
// errors and return empty). Failing loudly at factory time makes the
// misconfiguration visible instead.
var ErrEncryptedRepoNeedsIdentity = errors.New(
	"app: repository is encryption-enabled but no local identity is configured " +
		"(set $RELA_KEY_FILE or place .rela/key or ~/.config/rela/key)")

// ErrEncryptedRepoNeedsSafeFS is returned by OpenStore when the
// repository is encryption-enabled but the caller passed a raw FS
// (e.g. OsFS or MemFS) instead of a *storage.SafeFS. Without SafeFS
// we cannot install the OnPostWrite observer that keeps the watcher's
// self-echo LRU correct — every internal write would register as an
// external change and trigger spurious reconciles.
var ErrEncryptedRepoNeedsSafeFS = errors.New(
	"app: encrypted repos require the FS handle to be a *storage.SafeFS " +
		"(wrap your FS with storage.NewSafeFS at the entry point)")

// FSFactory is a store.Factory that opens filesystem-backed stores
// (fsstore) rooted at the given project paths. Each OpenStore call
// returns a fresh, independent store — callers that want a single
// long-lived store should open it once and keep it alive.
//
// FS should typically be a *storage.SafeFS in production so the
// factory can subscribe OnPostWrite for self-echo detection across
// any byte transform (encryption, future compression). When FS is
// NOT a SafeFS AND wantSealed is true, OpenStore returns an error
// rather than silently proceeding with a broken watcher — see
// ErrEncryptedRepoNeedsSafeFS. Tests that don't exercise the
// watcher may pass a MemFS on cleartext repos.
type FSFactory struct {
	FS    storage.FS
	Paths *project.Context
}

// compile-time interface check
var _ store.Factory = (*FSFactory)(nil)

// OpenStore constructs a new fsstore wired with the appropriate byte
// transform stack.
//
// Decision branch: if .rela/encryption.yaml exists, the factory
// loads the keyring and wraps the FS in a cryptofs.FS decorator;
// otherwise it passes the raw FS through unchanged. The same boolean
// (wantSealed) flows into fsstore.Config.WantSealed, so the
// "encrypted decorator installed?" and "consistency check expects
// sealed files?" answers come from one place and cannot drift.
//
// The factory subscribes the store's RecordWrite method as the SafeFS
// post-write observer. This is how the watcher's self-echo LRU stays
// correct across any transform: the hash is always taken of the
// bytes that actually landed on disk, at the only layer that performs
// the OS write.
//
// Returns ErrEncryptedRepoNeedsIdentity if the repo is encrypted but
// no local identity is configured.
func (f *FSFactory) OpenStore(meta *metamodel.Metamodel) (store.Store, error) {
	wantSealed, kr, err := f.loadEncryption()
	if err != nil {
		return nil, err
	}
	if wantSealed && kr.Identity() == nil {
		return nil, ErrEncryptedRepoNeedsIdentity
	}

	// Encrypted repos require SafeFS so we can install the OnPostWrite
	// observer that keeps self-echo detection correct. Fail loudly
	// when that invariant is violated; silent type-casts here have
	// bitten the watcher in the past.
	safe, hasSafeFS := f.FS.(*storage.SafeFS)
	if wantSealed && !hasSafeFS {
		return nil, ErrEncryptedRepoNeedsSafeFS
	}

	var bytes fsstore.StoreFS = f.FS
	if wantSealed {
		bytes = cryptofs.New(f.FS, kr.Recipients(), kr.Identity())
	}

	s, err := fsstore.New(fsstore.Config{
		FS:           f.FS,
		Bytes:        bytes,
		WantSealed:   wantSealed,
		EntitiesDir:  f.Paths.EntitiesDir,
		RelationsDir: f.Paths.RelationsDir,
		CacheDir:     f.Paths.CacheDir,
		Schemas:      buildSchemas(meta),
	})
	if err != nil {
		return nil, err
	}
	if hasSafeFS {
		safe.OnPostWrite(s.RecordWrite)
	}
	return s, nil
}

// loadEncryption decides whether encryption is on for this project
// by checking for .rela/encryption.yaml. When on, it also loads the
// keyring (recipients + local identity). Returns (false, nil, nil)
// when encryption is off.
func (f *FSFactory) loadEncryption() (bool, *encryption.Keyring, error) {
	cfgPath := filepath.Join(f.Paths.CacheDir, encryption.ConfigFileName)
	if _, err := os.Stat(cfgPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("app: stat %s: %w", cfgPath, err)
	}
	kr, err := encryption.LoadFromDir(f.Paths.Root)
	if err != nil {
		return false, nil, fmt.Errorf("app: load keyring: %w", err)
	}
	return true, kr, nil
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
