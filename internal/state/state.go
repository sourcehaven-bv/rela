// Package state provides a per-user key/value store for state that
// persists between runs but isn't part of the project's tracked source
// — UI state, render caches, scheduler bookkeeping.
//
// The KV interface is the swap boundary. FSKV is the default backend;
// callers can plug in Redis, DynamoDB, etc. by implementing KV.
package state

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// KV is the top-level state service. Keys are hierarchical (subdirectories
// separated by forward slashes) to match real callers that group related
// state under a common prefix — e.g. "documents/<hash>.html".
type KV interface {
	// Get reads the value at key. Implementations return an error that
	// satisfies os.IsNotExist (or an os.PathError wrapping one) when the
	// key has no value, so callers can distinguish missing from failing.
	Get(ctx context.Context, key string) ([]byte, error)

	// Put writes data at key, creating any intermediate structure.
	Put(ctx context.Context, key string, data []byte) error

	// Delete removes the value at key. Deleting a missing key is not an
	// error — callers using Delete to clear optional state shouldn't have
	// to special-case "already gone."
	Delete(ctx context.Context, key string) error
}

// ValidateKey reports whether key is acceptable to every [KV] backend.
//
// FSKV resolves keys to filesystem paths, so [storage.RootedFS] already rejects
// traversal, absolute paths, backslashes, colons and Windows reserved names. A
// database backend has no such constraint and would happily store `../../etc`
// — which would mean a key that works on PostgreSQL fails after a migration to
// the filesystem backend. Non-filesystem backends call this so the contract is
// the same everywhere; FSKV keeps relying on RootedFS, which is the stricter
// (and authoritative) barrier.
//
// The rules mirror RootedFS.resolve deliberately. If that ever gains a rule,
// this must gain it too — the shared conformance suite in state/statetest
// exercises both backends against one key table, so a divergence fails there.
func ValidateKey(key string) error {
	if key == "" {
		return errors.New("state: key must not be empty")
	}
	for _, c := range key {
		if c < 0x20 || c == 0x7f {
			return errors.New("state: control character (including NUL) not allowed in key")
		}
	}
	if strings.ContainsRune(key, '\\') {
		return errors.New("state: backslash not allowed in key (use forward slash)")
	}
	if strings.ContainsRune(key, ':') {
		return errors.New("state: colon not allowed in key")
	}
	if strings.HasPrefix(key, "/") {
		return errors.New("state: key must be relative")
	}
	for seg := range strings.SplitSeq(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("state: traversal or empty segment not allowed in key")
		}
		stem := strings.ToLower(seg)
		if i := strings.Index(stem, "."); i >= 0 {
			stem = stem[:i]
		}
		if windowsReservedKeySegment[stem] {
			return fmt.Errorf("state: Windows reserved name %q not allowed in key", seg)
		}
	}
	return nil
}

// windowsReservedKeySegment mirrors storage's list so a key valid on a database
// backend stays valid if the project is later served from the filesystem.
var windowsReservedKeySegment = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
	"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidatedKV wraps a KV that does not validate keys itself, applying
// [ValidateKey] before every operation.
//
// It exists so key policy has exactly one implementation. FSKV gets validation
// for free from [storage.RootedFS], which must reject traversal and
// Windows-hostile names because it resolves keys to real paths. A database
// backend has no such constraint and would happily store `../../etc` — so
// without this a key accepted on PostgreSQL would be refused after a migration
// to the filesystem backend, and the two backends would not be interchangeable.
//
// Architecture note: the database backend lives in a store package, which may
// not import this one (see .go-arch-lint.yml). Wrapping at the wiring site
// keeps the rules here, next to the contract they belong to, instead of copied
// into a backend where they could silently drift.
type ValidatedKV struct {
	inner KV
}

var _ KV = (*ValidatedKV)(nil)

// NewValidatedKV wraps inner with key validation. Returns an error if inner is
// nil — a KV that silently accepts nothing is worse than a boot failure.
func NewValidatedKV(inner KV) (*ValidatedKV, error) {
	if inner == nil {
		return nil, errors.New("state: NewValidatedKV requires an inner KV")
	}
	return &ValidatedKV{inner: inner}, nil
}

func (v *ValidatedKV) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	return v.inner.Get(ctx, key)
}

func (v *ValidatedKV) Put(ctx context.Context, key string, data []byte) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	return v.inner.Put(ctx, key, data)
}

func (v *ValidatedKV) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	return v.inner.Delete(ctx, key)
}

// FSKV stores state under a root directory on a filesystem. Key
// validation and parent-directory creation are handled by the embedded
// RootedFS.
type FSKV struct {
	fs *storage.RootedFS
}

var _ KV = (*FSKV)(nil)

// NewFSKV constructs a filesystem-backed KV rooted at the given
// RootedFS. The RootedFS is the single path-validation barrier.
func NewFSKV(fs *storage.RootedFS) *FSKV {
	return &FSKV{fs: fs}
}

func (s *FSKV) Get(_ context.Context, key string) ([]byte, error) {
	return s.fs.ReadFile(key)
}

func (s *FSKV) Put(_ context.Context, key string, data []byte) error {
	return s.fs.WriteFile(key, data, 0o644)
}

func (s *FSKV) Delete(_ context.Context, key string) error {
	if err := s.fs.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) && !os.IsNotExist(err) {
		return err
	}
	return nil
}
