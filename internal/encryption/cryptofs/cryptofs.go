// Package cryptofs provides a transparent encryption decorator for
// the byte-level filesystem interface consumed by fsstore.
//
// A cryptofs.FS wraps an inner byte-I/O FS (typically a
// *storage.SafeFS, which in turn wraps an OsFS for atomic writes).
// WriteFile prepends a small rela metadata header to the caller-
// supplied plaintext, then seals the combined bytes before
// delegating to the inner FS. ReadFile unseals whatever the inner
// FS returns, parses the header off the front, verifies it, and
// returns only the body bytes — matching what the caller originally
// wrote. Every other operation (Remove, Rename, Stat) is passed
// through unchanged.
//
// Callers above the decorator see plaintext in both directions and
// never learn the header exists; callers below see ciphertext.
// This lets fsstore remain completely unaware that encryption is
// happening — it reads and writes plain bytes, the decorator does
// the transform AND the integrity checks.
//
// The header carries a monotonic version and the repo-relative
// path. On read the decorator enforces:
//
//   - X-Rela-Version ≥ the last version this machine observed for
//     this repo (stored in XDG state). Catches rollback attacks
//     where the cloud adversary restores an older sealed file.
//   - X-Rela-Path equals the path the file was loaded from.
//     Catches swap/rename attacks where the cloud adversary
//     renames sealed A to B and vice versa.
//
// Error classification is preserved: a failed Unseal wraps
// encryption.ErrNoMatchingKey, ErrCorrupted, or ErrNoPrivateKey
// with %w, so callers upstream can still use errors.Is via
// IsNoMatchingKey / IsCorrupted / IsNoPrivateKey. Header-specific
// failures wrap ErrRollbackDetected / ErrFileRelocated /
// ErrMalformedHeader.
package cryptofs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

// Inner is the subset of fsstore.StoreFS that cryptofs.FS delegates
// to. Declared here (rather than imported from fsstore) so this
// package has no dependency on fsstore — avoiding an import cycle
// if fsstore ever needs to depend on cryptofs.
type Inner interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Remove(path string) error
	Rename(oldpath, newpath string) error
	Stat(path string) (os.FileInfo, error)
}

// FS is a transparent encryption decorator over Inner. The exported
// methods match fsstore.StoreFS exactly.
type FS struct {
	inner      Inner
	recipients []encryption.Recipient
	identity   encryption.Identity

	// repoRoot is the absolute path to the rela project root.
	// Used to convert the absolute paths fsstore hands us into
	// repo-relative paths inside the header, so the header is
	// stable across clones and movements of the repo directory.
	repoRoot string

	// writeVersion is the version this FS stamps into every header
	// it writes. Taken from the keyring's authoritative version at
	// construction time; mutations (keys add/remove) build a new FS
	// with a bumped version rather than mutating in place.
	writeVersion int

	// state is the per-machine last-seen-version tracker. May be
	// nil in tests that only exercise encode/decode round-trips
	// and don't care about rollback detection — in that case
	// ReadFile skips the monotonicity check but still verifies the
	// path. A nil state is NEVER acceptable in production wiring
	// and the factory refuses to build one.
	state *encryption.LocalState
}

// Config captures the construction parameters for cryptofs.FS.
// Using a struct rather than a long parameter list keeps the
// factory wiring readable as new fields arrive (e.g. the future
// resealing sentinel).
type Config struct {
	Inner        Inner
	Recipients   []encryption.Recipient
	Identity     encryption.Identity
	RepoRoot     string
	WriteVersion int
	State        *encryption.LocalState
}

// New returns a FS built from cfg. Validates invariants the
// decorator cannot enforce lazily: recipients must be non-empty
// (Seal needs at least one), identity must be non-nil (Unseal
// cannot work without it), RepoRoot must be absolute (we use it
// to compute relative paths in the header), WriteVersion must be
// ≥ 1 (0 would permanently fail the rollback check), state may
// be nil only in test contexts that accept the caveat.
func New(cfg Config) (*FS, error) {
	if len(cfg.Recipients) == 0 {
		return nil, errors.New("cryptofs: recipients required")
	}
	if cfg.Identity == nil {
		return nil, errors.New("cryptofs: identity required")
	}
	if cfg.RepoRoot == "" {
		return nil, errors.New("cryptofs: repo root required")
	}
	if !filepath.IsAbs(cfg.RepoRoot) {
		return nil, fmt.Errorf("cryptofs: repo root must be absolute, got %q", cfg.RepoRoot)
	}
	if cfg.WriteVersion < 1 {
		return nil, fmt.Errorf("cryptofs: write version must be ≥ 1, got %d", cfg.WriteVersion)
	}
	return &FS{
		inner:        cfg.Inner,
		recipients:   cfg.Recipients,
		identity:     cfg.Identity,
		repoRoot:     cfg.RepoRoot,
		writeVersion: cfg.WriteVersion,
		state:        cfg.State,
	}, nil
}

// ReadFile reads path through inner, unseals the result, verifies
// the rela header (path match + version monotonicity), and returns
// the body bytes (identical to what WriteFile was handed).
func (f *FS) ReadFile(path string) ([]byte, error) {
	raw, err := f.inner.ReadFile(path)
	if err != nil {
		return nil, err
	}
	plaintext, err := encryption.Unseal(raw, f.identity)
	if err != nil {
		return nil, err
	}
	header, body, err := encryption.ParseHeader(plaintext)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	expectedPath, err := f.relPath(path)
	if err != nil {
		return nil, err
	}
	if header.Path != expectedPath {
		return nil, fmt.Errorf("%w: header path %q != file path %q",
			encryption.ErrFileRelocated, header.Path, expectedPath)
	}
	if err := f.checkRollback(header.Version); err != nil {
		return nil, err
	}
	return body, nil
}

// checkRollback enforces that the observed header version is ≥ the
// last version this machine has seen for this repo. On success it
// advances the local state so subsequent reads catch a rollback
// below the higher-water mark. Missing state (TOFU) or nil state
// (test mode) skip the check.
func (f *FS) checkRollback(observed int) error {
	if f.state == nil {
		return nil
	}
	stored, err := f.state.LoadVersion()
	if err != nil {
		return err
	}
	if observed < stored {
		return fmt.Errorf("%w: observed=%d, last-seen=%d",
			encryption.ErrRollbackDetected, observed, stored)
	}
	if observed > stored {
		// Advance the high-water mark. Silent TOFU on first-ever
		// read (stored == 0) or on legitimate version bumps.
		if err := f.state.StoreVersion(observed); err != nil {
			return err
		}
	}
	return nil
}

// WriteFile prepends the rela header (with this FS's version and
// the repo-relative form of path), seals the combined bytes for
// every configured recipient, then delegates to inner.WriteFile.
// The inner FS sees the ciphertext; callers see plaintext.
func (f *FS) WriteFile(path string, data []byte, perm os.FileMode) error {
	rel, err := f.relPath(path)
	if err != nil {
		return err
	}
	header := &encryption.Header{
		Version: f.writeVersion,
		Path:    rel,
	}
	plaintext := append(header.Encode(), data...)
	sealed, err := encryption.Seal(plaintext, f.recipients)
	if err != nil {
		return err
	}
	return f.inner.WriteFile(path, sealed, perm)
}

// relPath converts the (possibly absolute) path callers pass into
// a repo-relative form for the header. Uses forward slashes
// regardless of host OS so a repo cloned across platforms round-
// trips cleanly. Refuses paths outside the repo root — they'd
// break the path-binding guarantee.
func (f *FS) relPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		// Callers sometimes pass already-relative paths (tests
		// exercise the decorator with MemFS paths like "/x.md"
		// that are absolute-looking but treated as rooted).
		// Normalize by joining to repoRoot and taking Rel.
		path = filepath.Join(f.repoRoot, path)
	}
	rel, err := filepath.Rel(f.repoRoot, path)
	if err != nil {
		return "", fmt.Errorf("cryptofs: path %q not under repo root %q: %w",
			path, f.repoRoot, err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("cryptofs: path %q escapes repo root %q",
			path, f.repoRoot)
	}
	// Normalize separators — stored relative paths use forward
	// slashes so a Windows-written file is readable on Linux.
	return filepath.ToSlash(rel), nil
}

// Remove passes through unchanged.
func (f *FS) Remove(path string) error { return f.inner.Remove(path) }

// Rename passes through unchanged. Note: the on-disk sealed bytes
// carry the OLD path in their header, so a subsequent read of the
// renamed file will trip ErrFileRelocated. Legitimate renames
// must go through a re-seal pass (unseal → seal to new path) —
// the cli/keys.go reencryptAll path handles this for its own
// use cases; bare fs.Rename is reserved for atomic-write temp
// files (where both paths are short-lived) and is safe there
// because the post-rename read never happens through cryptofs.
func (f *FS) Rename(oldpath, newpath string) error {
	return f.inner.Rename(oldpath, newpath)
}

// Stat passes through unchanged. File metadata (size, mtime, mode)
// is about the ciphertext on disk, not the plaintext — callers that
// need plaintext size must ReadFile and measure.
func (f *FS) Stat(path string) (os.FileInfo, error) { return f.inner.Stat(path) }
