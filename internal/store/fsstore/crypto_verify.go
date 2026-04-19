package fsstore

import (
	"github.com/Sourcehaven-BV/rela/internal/storage/integrity"
)

// ErrRepoHasSealedFilesButNoConfig is re-exported from the integrity
// package so existing callers that errors.Is against fsstore symbols
// keep compiling. New code should import the canonical symbol from
// internal/storage/integrity.
//
// Deprecated: use integrity.ErrRepoHasSealedFilesButNoConfig.
var ErrRepoHasSealedFilesButNoConfig = integrity.ErrRepoHasSealedFilesButNoConfig

// ErrRepoHasCleartextFilesButEncryptionEnabled is re-exported from
// the integrity package for the same reason.
//
// Deprecated: use integrity.ErrRepoHasCleartextFilesButEncryptionEnabled.
var ErrRepoHasCleartextFilesButEncryptionEnabled = integrity.ErrRepoHasCleartextFilesButEncryptionEnabled

// verifyEncryptionConsistency delegates to integrity.Verify using
// the raw filesystem handle and the declared wantSealed mode. The
// verifier lives in its own package because the check is
// layer-crossing (inspects on-disk bytes, not fsstore's plaintext
// view) and must run before fsstore.New returns a usable store.
func (s *FSStore) verifyEncryptionConsistency() error {
	dirs := []string{s.entitiesDir, s.relationsDir}
	if s.attachDir != "" {
		dirs = append(dirs, s.attachDir)
	}
	return integrity.Verify(s.fs, s.wantSealed, dirs)
}
