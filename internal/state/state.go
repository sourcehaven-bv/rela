// Package state provides a per-user key/value store for state that
// persists between runs but isn't part of the project's tracked source
// — UI state, render caches, scheduler bookkeeping.
//
// The KV interface is the swap boundary. FSKV is the default backend;
// callers can plug in Redis, DynamoDB, etc. by implementing KV.
package state

import (
	"context"
	"fmt"
	"path/filepath"
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
}

// FSKV stores state under a root directory on a filesystem.
type FSKV struct {
	fs   storage.FS
	root string
}

var _ KV = (*FSKV)(nil)

// NewFSKV constructs a filesystem-backed KV rooted at dir (e.g. the
// project's .rela/ directory).
func NewFSKV(fs storage.FS, dir string) *FSKV {
	return &FSKV{fs: fs, root: dir}
}

func (s *FSKV) Get(_ context.Context, key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	return s.fs.ReadFile(filepath.Join(s.root, key))
}

func (s *FSKV) Put(_ context.Context, key string, data []byte) error {
	if err := validateKey(key); err != nil {
		return err
	}
	full := filepath.Join(s.root, key)
	if err := s.fs.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return s.fs.WriteFile(full, data, 0o644)
}

// validateKey rejects keys that would escape the KV root or map to paths
// the filesystem would misinterpret. Subdirectories are allowed (callers
// group state under prefixes); absolute paths, traversal segments,
// backslashes, control characters, and Windows drive letters are not.
func validateKey(name string) error {
	if name == "" {
		return fmt.Errorf("state: key must not be empty")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("state: control character (including NUL) not allowed")
		}
	}
	if strings.ContainsRune(name, '\\') {
		return fmt.Errorf("state: backslash not allowed (use forward slash)")
	}
	if strings.HasPrefix(name, "/") {
		return fmt.Errorf("state: key must be relative")
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("state: traversal or empty segment not allowed")
		}
	}
	if len(name) >= 2 && name[1] == ':' {
		return fmt.Errorf("state: drive letter not allowed")
	}
	return nil
}
