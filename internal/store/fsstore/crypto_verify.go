package fsstore

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// Errors returned by verifyEncryptionConsistency when fsstore.New
// detects a half-migrated repository (cleartext and sealed files
// coexisting under the presence/absence of an encryption config).

// ErrRepoHasSealedFilesButNoConfig indicates that the FSStore was
// constructed without a real Crypto (cleartext mode), but the data
// directories contain files that look sealed. Refusing to open
// prevents the CLI from silently reporting "no entities found" for a
// repo the user forgot to configure encryption for.
var ErrRepoHasSealedFilesButNoConfig = errors.New(
	"fsstore: repository contains sealed files but encryption is not configured")

// ErrRepoHasCleartextFilesButEncryptionEnabled indicates the inverse:
// encryption is configured, but some data files are still cleartext.
// Almost certainly the result of an interrupted migration or a merge
// that brought in cleartext files from a branch that didn't know
// about encryption.
var ErrRepoHasCleartextFilesButEncryptionEnabled = errors.New(
	"fsstore: repository has cleartext data files but encryption is enabled")

// offendingFilesLimit bounds how many paths are included in the error
// message. The user doesn't need all 10k file paths to understand the
// problem; a handful of examples is enough to diagnose.
const offendingFilesLimit = 5

// verifyEncryptionConsistency scans the entity, relation, and
// attachment directories and fails fast if the on-disk state
// disagrees with the configured Crypto. Specifically:
//
//   - Crypto is identityCrypto (cleartext mode) AND any file looks
//     sealed -> ErrRepoHasSealedFilesButNoConfig.
//   - Crypto is a real age crypto (encryption enabled) AND any data
//     file does NOT look sealed -> ErrRepoHasCleartextFilesButEncryptionEnabled.
//
// The scan reads only the first SealedMagic-worth of bytes per file.
func (s *FSStore) verifyEncryptionConsistency() error {
	var offenders []string
	expectSealed := s.crypto.Enabled()

	check := func(path string) error {
		// Peek just enough bytes to check for the age header.
		// s.fs.ReadFile reads the whole file; for very small files
		// this is cheap, and most entity/relation files are a few
		// KiB at most. Optimize later if needed.
		data, err := s.fs.ReadFile(path)
		if err != nil {
			return err
		}
		looksSealed := len(data) > 0 && looksLikeAgeBlob(data)
		if expectSealed && !looksSealed {
			offenders = append(offenders, path)
		}
		if !expectSealed && looksSealed {
			offenders = append(offenders, path)
		}
		return nil
	}

	for _, dir := range s.dataDirsToVerify() {
		if err := walkMarkdownFiles(s.fs, dir, check); err != nil {
			return fmt.Errorf("verify encryption consistency: %w", err)
		}
		if len(offenders) >= offendingFilesLimit {
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
	if expectSealed {
		return fmt.Errorf("%w: %s (run `rela keys migrate` to seal cleartext files)",
			ErrRepoHasCleartextFilesButEncryptionEnabled, strings.Join(examples, ", "))
	}
	return fmt.Errorf("%w: %s (run `rela keys migrate` to decrypt, or configure encryption)",
		ErrRepoHasSealedFilesButNoConfig, strings.Join(examples, ", "))
}

// dataDirsToVerify returns the set of directories the consistency
// check walks. Attachments are included only when attachDir is set.
func (s *FSStore) dataDirsToVerify() []string {
	dirs := []string{s.entitiesDir, s.relationsDir}
	if s.attachDir != "" {
		dirs = append(dirs, s.attachDir)
	}
	return dirs
}

// looksLikeAgeBlob is a package-local copy of
// encryption.LooksSealed. Duplicated so the consistency check
// doesn't depend on a live Crypto (needed for the "we're in
// cleartext mode and want to detect sealed files" case). Kept in
// sync via the constant.
const ageMagic = "age-encryption.org/v1\n"

func looksLikeAgeBlob(b []byte) bool {
	return strings.HasPrefix(string(b), ageMagic)
}

// walkMarkdownFiles walks dir and invokes fn on every regular .md
// file. Attachment binaries have arbitrary names, so the walk
// accepts all regular files in attachment subtrees. A missing dir
// is not an error (empty repo).
func walkMarkdownFiles(fsys interface {
	Stat(string) (fs.FileInfo, error)
	ReadDir(string) ([]fs.DirEntry, error)
	ReadFile(string) ([]byte, error)
}, dir string, fn func(string) error) error {
	if _, err := fsys.Stat(dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return walkRecursive(fsys, dir, fn)
}

// walkRecursive is a minimal recursive walker using the fsstore FS
// interface. Skips temp/backup files (.new, .bak, *~, dotfiles).
func walkRecursive(fsys interface {
	Stat(string) (fs.FileInfo, error)
	ReadDir(string) ([]fs.DirEntry, error)
	ReadFile(string) ([]byte, error)
}, dir string, fn func(string) error) error {
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
	if strings.HasSuffix(name, ".new") || strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, "~") {
		return true
	}
	return false
}
