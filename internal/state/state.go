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
	"io/fs"
	"os"

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
