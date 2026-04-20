// Package integrity checks layer-crossing invariants about the
// on-disk state of a rela repository that neither the store nor the
// encryption decorator can verify in isolation.
//
// Today this is a single check: "every file under the configured
// data dirs is either sealed or cleartext, matching the declared
// mode." The check lives here (rather than inside fsstore or
// cryptofs) because:
//
//   - It must run BEFORE fsstore.New returns a usable store; a
//     half-migrated repo would otherwise silently misbehave (fsstore
//     reports "no entities" for a sealed repo opened in cleartext
//     mode, etc.).
//   - It inspects raw on-disk bytes. That is incompatible with a
//     plaintext-returning decorator sitting above it — the verifier
//     needs the ciphertext it does NOT see after the transform.
//   - It is a wiring-layer concern: the factory is where "was
//     encryption configured?" gets decided, and the factory calls
//     Verify with the same boolean it uses to install the decorator,
//     so the two cannot drift.
package integrity

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

// ErrRepoHasSealedFilesButNoConfig indicates that Verify was called
// with wantSealed=false, but the data directories contain files that
// look sealed. Refusing to continue prevents the CLI from silently
// reporting "no entities found" for a repo the user forgot to
// configure encryption for.
var ErrRepoHasSealedFilesButNoConfig = errors.New(
	"integrity: repository contains sealed files but encryption is not configured")

// ErrRepoHasCleartextFilesButEncryptionEnabled indicates the inverse:
// Verify was called with wantSealed=true, but some data files are
// still cleartext. Almost certainly the result of an interrupted
// migration or a merge that brought in cleartext files from a branch
// that didn't know about encryption.
var ErrRepoHasCleartextFilesButEncryptionEnabled = errors.New(
	"integrity: repository has cleartext data files but encryption is enabled")

// offendingFilesLimit bounds how many paths are included in the
// error message. A handful of examples is enough to diagnose; we
// don't need all 10k file paths.
const offendingFilesLimit = 5

// FSReader is the narrow filesystem surface Verify needs. The
// raw (undecorated) filesystem must be passed — Verify inspects
// on-disk bytes, not plaintext.
type FSReader interface {
	Stat(string) (fs.FileInfo, error)
	ReadDir(string) ([]fs.DirEntry, error)
	ReadFile(string) ([]byte, error)
}

// Verify walks dirs and asserts every non-hidden file matches the
// wantSealed flag. Missing dirs are treated as empty (no error).
//
// Callers: pass the SAME filesystem handle used to construct the
// transform stack. That handle is raw — the verifier must not see
// the plaintext view, because the invariant is about ciphertext
// on disk.
//
// Errors wrap ErrRepoHasSealedFilesButNoConfig or
// ErrRepoHasCleartextFilesButEncryptionEnabled (depending on the
// direction of the mismatch), each extended with up to five offender
// paths. Callers can errors.Is-check to distinguish.
func Verify(fsys FSReader, wantSealed bool, dirs []string) error {
	var offenders []string
	const peek = 64 // large enough for encryption.SealedMagic

	check := func(path string) error {
		head, err := peekHeader(fsys, path, peek)
		if err != nil {
			return err
		}
		looksSealed := encryption.LooksSealed(head)
		if wantSealed != looksSealed {
			offenders = append(offenders, path)
		}
		if len(offenders) >= offendingFilesLimit {
			return errStopVerify
		}
		return nil
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		err := walkForVerification(fsys, dir, check)
		if err != nil && !errors.Is(err, errStopVerify) {
			return fmt.Errorf("verify encryption consistency: %w", err)
		}
		if errors.Is(err, errStopVerify) {
			break
		}
	}

	if len(offenders) == 0 {
		return nil
	}

	examples := offenders
	if len(examples) > offendingFilesLimit {
		examples = examples[:offendingFilesLimit]
	}
	if wantSealed {
		return fmt.Errorf("%w: %s (run `rela keys migrate` to seal cleartext files)",
			ErrRepoHasCleartextFilesButEncryptionEnabled, strings.Join(examples, ", "))
	}
	return fmt.Errorf("%w: %s (run `rela keys migrate` to decrypt, or configure encryption)",
		ErrRepoHasSealedFilesButNoConfig, strings.Join(examples, ", "))
}

// errStopVerify signals from a check callback that the walk should
// stop (offender list hit its cap). Unwrapped and ignored by the
// top-level caller; not a real error.
var errStopVerify = errors.New("stop verify walk")

// peekHeader reads at most n bytes from path. Used by Verify to
// classify a file without reading its body in full.
func peekHeader(fsys FSReader, path string, n int) ([]byte, error) {
	data, err := fsys.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// The FSReader interface is byte-slice based; a streaming
	// read-first-N is not available. For small entity files this is
	// fine; for large attachments it's wasted I/O — acceptable v1
	// cost, optimize if profiling flags it.
	if len(data) > n {
		return data[:n], nil
	}
	return data, nil
}

// walkForVerification walks dir and invokes fn on every regular
// file (not just .md; attachments have arbitrary names). A missing
// dir is not an error. Skips temp/backup files and dotfiles.
func walkForVerification(fsys FSReader, dir string, fn func(string) error) error {
	if _, err := fsys.Stat(dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return walkRecursive(fsys, dir, fn)
}

// walkRecursive is a minimal recursive walker using the FSReader
// interface. Skips temp/backup files (.new, .bak, *~, dotfiles).
func walkRecursive(fsys FSReader, dir string, fn func(string) error) error {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if shouldSkipScan(name) {
			continue
		}
		full := filepath.Join(dir, name)
		if e.IsDir() {
			if err := walkRecursive(fsys, full, fn); err != nil {
				return err
			}
			continue
		}
		if err := fn(full); err != nil {
			return err
		}
	}
	return nil
}

// shouldSkipScan returns true for filenames that should be ignored
// by the consistency check: editor tempfiles, dotfiles, and fsstore
// temp/backup suffixes. The check is for diagnostic inconsistency
// detection, not a correctness invariant; false negatives are fine.
func shouldSkipScan(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") {
		return true
	}
	for _, suffix := range []string{".new", ".tmp", ".bak", "~"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
