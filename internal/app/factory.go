// Package app provides factories that construct the concrete services
// needed by each rela entry point (cli, data-entry server, desktop,
// MCP). Today that is a single factory: FSFactory, which opens an
// fsstore rooted at a project directory.
package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/encryption/cryptofs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/storage/integrity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// ErrEncryptedRepoNeedsIdentity is returned by OpenStore when the
// repository is encryption-enabled (recipients.age is present) but
// no local age identity could be found.
//
// Identity resolution tries $RELA_KEY_FILE first and then
// <user-config>/rela/repos/<repo-id>/key — both absent or
// unreadable means no key is configured. Opening an encrypted store
// without an identity would silently cripple every read path
// (GetEntity, ListEntities, rebuildPropCache all swallow errors
// and return empty). Failing loudly at factory time makes the
// misconfiguration visible instead.
var ErrEncryptedRepoNeedsIdentity = errors.New(
	"app: repository is encryption-enabled but no local identity is configured " +
		"(set $RELA_KEY_FILE or run `rela keys init` to create one)")

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
//
// UserState is the per-user, per-repo state service that the
// factory threads through to encryption helpers (LocalState,
// identity resolution, reseal sentinel). It must be non-nil in
// production; tests that don't touch encryption can pass
// userstate.NewForTest(t.TempDir()).
type FSFactory struct {
	FS        storage.FS
	Paths     *project.Context
	UserState userstate.FSService
}

// NewFSFactory constructs an FSFactory with explicit service wiring.
// All three arguments must be non-nil; passing a nil UserState is a
// programming error that would silently fall through to a
// filesystem-less encryption path in later construction.
//
// The struct-literal form remains available for tests that want to
// customize individual fields; production callers should prefer
// this constructor so the non-nil contract is enforced at the
// package boundary.
func NewFSFactory(fs storage.FS, paths *project.Context, us userstate.FSService) (*FSFactory, error) {
	if fs == nil {
		return nil, errors.New("app: NewFSFactory: nil FS")
	}
	if paths == nil {
		return nil, errors.New("app: NewFSFactory: nil Paths")
	}
	if us == nil {
		return nil, errors.New("app: NewFSFactory: nil UserState")
	}
	return &FSFactory{FS: fs, Paths: paths, UserState: us}, nil
}

// compile-time interface check
var _ store.Factory = (*FSFactory)(nil)

// OpenStore constructs a new fsstore wired with the appropriate byte
// transform stack.
//
// Decision branch: if <root>/recipients.age exists, the factory
// loads the keyring and wraps the FS in a cryptofs.FS decorator;
// otherwise it passes the raw FS through unchanged.
//
// Before opening the store, the factory runs a consistency check
// (integrity.Verify) to reject half-migrated repos where the
// on-disk state disagrees with the declared encryption mode. The
// same boolean (wantSealed) drives both the decorator install and
// the consistency check, so they cannot drift.
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
	// Past this point, wantSealed implies kr != nil and kr.Identity()
	// is non-nil — LoadFromDir refuses to succeed without one, and
	// the missing-identity case has already been translated to
	// ErrEncryptedRepoNeedsIdentity by loadEncryption.

	// Encrypted repos require SafeFS so we can install the OnPostWrite
	// observer that keeps self-echo detection correct. Fail loudly
	// when that invariant is violated; silent type-casts here have
	// bitten the watcher in the past.
	safe, hasSafeFS := f.FS.(*storage.SafeFS)
	if wantSealed && !hasSafeFS {
		return nil, ErrEncryptedRepoNeedsSafeFS
	}

	// Consistency check against the raw on-disk state — must happen
	// BEFORE fsstore.New so we refuse to open half-migrated repos.
	// Same wantSealed boolean that drives the decorator install below.
	//
	// Attachments are verified alongside entities/relations: on an
	// encrypted repo every data file must be sealed, including the
	// content-addressable attachment blobs and their metadata
	// sidecars, otherwise plaintext has leaked back into the tree.
	if verifyErr := integrity.Verify(f.FS, wantSealed, []string{
		f.Paths.EntitiesDir,
		f.Paths.RelationsDir,
		filepath.Join(f.Paths.Root, "attachments"),
	}); verifyErr != nil {
		return nil, verifyErr
	}

	var bytes fsstore.StoreFS = f.FS
	if wantSealed {
		bytes, err = f.newCryptoFS(f.FS, kr)
		if err != nil {
			return nil, fmt.Errorf("app: build cryptofs: %w", err)
		}
	}

	s, err := fsstore.New(fsstore.Config{
		FS:           f.FS,
		Bytes:        bytes,
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

// OpenBytesFS returns the byte-I/O handle appropriate for this
// project: cryptofs-decorated when encryption is enabled, the raw
// FS handle otherwise.
//
// This is the same handle the factory wires into the store's own
// bytes path, exposed so workspace-level components outside the
// store (today: the attachment store) can stay consistent with
// the store's encryption behavior. Callers that bypass this helper
// and reach for the raw FS directly silently skip seal/unseal on
// encrypted repos, landing plaintext on disk — use this method.
//
// The return type is attachment.BytesFS, the narrow subset of byte
// operations workspace components actually need. fsstore.StoreFS and
// storage.FS are both supersets and are returned transparently.
//
// Returns ErrEncryptedRepoNeedsIdentity if the repo is encrypted but
// no local identity is configured.
func (f *FSFactory) OpenBytesFS() (attachment.BytesFS, error) {
	wantSealed, kr, err := f.loadEncryption()
	if err != nil {
		return nil, err
	}
	if !wantSealed {
		return f.FS, nil
	}
	return f.newCryptoFS(f.FS, kr)
}

// newCryptoFS builds a cryptofs.FS wired from the keyring: its
// write-version, its loaded identity and recipients, and the
// per-machine last-seen-version state derived from the keyring's
// repo id. Called from both OpenStore (for the store's own bytes
// handle) and OpenBytesFS (for workspace-level components like
// attachments) so both consumers see the same configuration.
func (f *FSFactory) newCryptoFS(inner storage.FS, kr *encryption.Keyring) (*cryptofs.FS, error) {
	state, err := encryption.NewLocalState(f.UserState)
	if err != nil {
		return nil, err
	}
	return cryptofs.New(cryptofs.Config{
		Inner:        inner,
		Recipients:   kr.Recipients(),
		Identity:     kr.Identity(),
		RepoRoot:     f.Paths.Root,
		WriteVersion: kr.Version(),
		State:        state,
	})
}

// loadEncryption decides whether encryption is on for this project
// by checking for <root>/recipients.age — the authoritative
// encrypted recipient list. When present, it loads the keyring
// (recipients + local identity via LoadFromDir).
//
// Returns (false, nil, nil) when the repo is cleartext.
//
// Returns (false, nil, ErrEncryptedRepoNeedsIdentity) when the
// repo is encrypted but no local identity could be resolved. This
// error is translated from encryption.ErrNoPrivateKey (the inner
// package's sentinel) to the app-level sentinel so callers can use
// errors.Is at the app boundary without importing encryption.
func (f *FSFactory) loadEncryption() (bool, *encryption.Keyring, error) {
	enabled, err := encryption.IsEnabled(f.Paths.Root)
	if err != nil {
		return false, nil, fmt.Errorf("app: check encryption: %w", err)
	}
	if !enabled {
		return false, nil, nil
	}
	kr, err := encryption.LoadFromDir(f.Paths.Root, f.UserState)
	if err != nil {
		if errors.Is(err, encryption.ErrNoPrivateKey) {
			return false, nil, ErrEncryptedRepoNeedsIdentity
		}
		return false, nil, fmt.Errorf("app: load keyring: %w", err)
	}

	// Cross-check .rela/repo-id against the keyring's RepoID. A
	// mismatch means either a copied-in .rela/ from another project
	// or (on first open of a freshly cloned encrypted repo) a
	// missing .rela/repo-id — the latter is written on demand.
	//
	// This fires on EVERY factory open, not just CLI key commands,
	// so rollback-defense state (last_seen_version, reseal sentinel)
	// can never get keyed under the wrong per-repo directory on a
	// fresh clone. See TKT-XZ4EO review finding RR-ZW7E9.
	if vErr := userstate.VerifyKeyringRepoID(f.Paths.Root, kr.RepoID()); vErr != nil {
		return false, nil, fmt.Errorf("app: verify repo id: %w", vErr)
	}

	// Resume any interrupted `keys add` / `keys remove` rotation
	// from a prior crashed rela invocation. No-op in the normal
	// case (no sentinel, nothing to do). On the rare recovery
	// path, recipients.age gets rewritten — reload the keyring so
	// the rest of the store-open path sees the new state.
	resumed, err := encryption.ResumeInterruptedRotation(f.Paths.Root, kr, f.UserState)
	if err != nil {
		return false, nil, fmt.Errorf("app: resume interrupted rotation: %w", err)
	}
	if resumed {
		kr, err = encryption.LoadFromDir(f.Paths.Root, f.UserState)
		if err != nil {
			return false, nil, fmt.Errorf("app: reload keyring after recovery: %w", err)
		}
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
