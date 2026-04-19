// Package cryptofs provides a transparent encryption decorator for
// the byte-level filesystem interface consumed by fsstore.
//
// A cryptofs.FS wraps an inner byte-I/O FS (typically a
// *storage.SafeFS, which in turn wraps an OsFS for atomic writes).
// WriteFile seals the caller-supplied plaintext before delegating to
// the inner FS; ReadFile unseals whatever the inner FS returns
// before handing it back. Every other operation (Remove, Rename,
// Stat) is passed through unchanged.
//
// Callers above the decorator see plaintext in both directions;
// callers below see ciphertext. This lets fsstore remain completely
// unaware that encryption is happening — it reads and writes plain
// bytes, the decorator does the transform.
//
// Error classification is preserved: a failed Unseal wraps
// encryption.ErrNoMatchingKey, ErrCorrupted, or ErrNoPrivateKey with
// %w, so callers upstream (e.g. cli/show.go) can still use
// encryption.IsNoMatchingKey / IsCorrupted / IsNoPrivateKey via
// errors.Is.
package cryptofs

import (
	"os"

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
}

// New returns a FS that seals WriteFile calls for every recipient
// and unseals ReadFile calls with identity. Both parameters are
// required; passing nil or empty recipients makes Seal fail at
// write time (cannot seal for nobody), and a nil identity makes
// Unseal return ErrNoPrivateKey.
func New(inner Inner, recipients []encryption.Recipient, identity encryption.Identity) *FS {
	return &FS{inner: inner, recipients: recipients, identity: identity}
}

// ReadFile reads path through inner and unseals the result.
func (f *FS) ReadFile(path string) ([]byte, error) {
	raw, err := f.inner.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return encryption.Unseal(raw, f.identity)
}

// WriteFile seals data for every configured recipient, then
// delegates to inner.WriteFile. The inner FS sees the ciphertext;
// callers see plaintext.
func (f *FS) WriteFile(path string, data []byte, perm os.FileMode) error {
	sealed, err := encryption.Seal(data, f.recipients)
	if err != nil {
		return err
	}
	return f.inner.WriteFile(path, sealed, perm)
}

// Remove passes through unchanged.
func (f *FS) Remove(path string) error { return f.inner.Remove(path) }

// Rename passes through unchanged.
func (f *FS) Rename(oldpath, newpath string) error {
	return f.inner.Rename(oldpath, newpath)
}

// Stat passes through unchanged. File metadata (size, mtime, mode)
// is about the ciphertext on disk, not the plaintext — callers that
// need plaintext size must ReadFile and measure.
func (f *FS) Stat(path string) (os.FileInfo, error) { return f.inner.Stat(path) }
