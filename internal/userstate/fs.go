package userstate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// File permissions for the per-user state tree. Owner-only on
// POSIX; Windows ACLs inherit the user-config directory's ACL,
// which is already user-scoped by default.
const (
	stateDirPerm  os.FileMode = 0o700
	stateFilePerm os.FileMode = 0o600
)

// productDir is the single subdirectory under the OS user-config
// root that holds all rela state. Isolating under one name keeps
// the tree greppable and makes the Spotlight / Search opt-out
// a single-file operation.
const productDir = "rela"

// ErrRepoIDMismatch is returned when the .rela/repo-id on disk
// disagrees with a caller-provided canonical id (e.g. the keyring
// RepoID on an encrypted repo). The common trigger is a copied-in
// .rela/ from another project; the less-common trigger is a
// corrupted repo-id file. Either way, refusing to proceed is safer
// than silently sharing state across projects.
var ErrRepoIDMismatch = errors.New("userstate: .rela/repo-id disagrees with keyring repo id")

// ErrRepoIDTrackedInGit is returned when .rela/repo-id is found in
// git's index or working tree. A tracked repo-id collapses every
// collaborator's per-repo user-state directory onto the same name,
// cross-contaminating their independent state. We refuse to proceed
// rather than silently share.
var ErrRepoIDTrackedInGit = errors.New("userstate: .rela/repo-id is tracked by git (must be gitignored)")

// ErrOverrideInsideProject is returned when $RELA_USER_STATE_DIR
// resolves to a path inside the project root. The point of this
// package is to hold state *outside* the synced tree; pointing the
// override back into it would defeat every security property.
var ErrOverrideInsideProject = errors.New("userstate: override must not be inside the project tree")

// fsService is the production FSService implementation, rooted at
// a per-repo directory under the user config dir.
type fsService struct {
	root string
}

// NewFSWithRepoID constructs an FSService rooted at
// <base>/rela/repos/<repoID>/. repoID must be a canonical UUIDv4;
// callers obtain it via project.ResolveRepoID (cleartext repos) or
// Keyring.RepoID (encrypted repos).
//
// projectRoot is used only for override validation — we refuse to
// accept an override that resolves inside the project tree. If
// callers already validated the override elsewhere they may pass
// an empty projectRoot to skip the check, but production callers
// should always pass the resolved Context.Root.
func NewFSWithRepoID(projectRoot, repoID string) (FSService, error) {
	if err := validateRepoID(repoID); err != nil {
		return nil, err
	}
	base, err := resolveBase(os.Getenv, os.UserConfigDir)
	if err != nil {
		return nil, err
	}
	if projectRoot != "" && isInside(base, projectRoot) {
		return nil, fmt.Errorf("%w: %s inside %s", ErrOverrideInsideProject, base, projectRoot)
	}
	if frag := detectSyncDir(base); frag != "" {
		slog.Warn("user-state directory is under a known cloud-sync path",
			"base", base, "sync_fragment", frag,
			"hint", "unset "+EnvOverride+" or point it at a non-synced location")
	}

	root := resolveForRepo(base, repoID)
	if _, err := ensureDir(root); err != nil {
		return nil, err
	}
	// Indexer opt-out is a best-effort hint to the OS. Failures log
	// debug and do not block service creation — the user's state is
	// still at-rest-private regardless of whether Spotlight sees
	// filenames.
	if tagErr := tagNotIndexed(base); tagErr != nil {
		slog.Debug("userstate: indexer opt-out failed (non-fatal)",
			"err", tagErr)
	}
	return &fsService{root: root}, nil
}

// NewForTest returns an FSService rooted at an explicit directory.
// Tests use t.TempDir(); no environment variable gymnastics, no
// platform-detection, no indexer side-effects. Production code
// should not call this.
func NewForTest(root string) FSService {
	return &fsService{root: root}
}

// Root returns the absolute per-repo directory.
func (s *fsService) Root() string { return s.root }

// Path returns the absolute path of key under Root, without
// performing validation (callers already validate via Get/Put).
// Intended for non-KV writers (age identity), diagnostics, and
// lock-file location computation.
func (s *fsService) Path(key string) string {
	return filepath.Join(s.root, key)
}

// Get reads the value at key. Returns a wrapped os.ErrNotExist
// when the key has no value; callers can use errors.Is.
func (s *fsService) Get(_ context.Context, key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(s.root, key))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Put writes data atomically at key. Intermediate directories are
// created with stateDirPerm. The write goes through a temp file +
// rename so a crash mid-write leaves the previous value intact.
func (s *fsService) Put(_ context.Context, key string, data []byte) error {
	if err := validateKey(key); err != nil {
		return err
	}
	full := filepath.Join(s.root, key)
	if err := os.MkdirAll(filepath.Dir(full), stateDirPerm); err != nil {
		return err
	}
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, data, stateFilePerm); err != nil {
		return err
	}
	return os.Rename(tmp, full)
}

// Lock acquires an exclusive lock on <key>.lock. The lock file is
// created in the same directory as the key itself so a single
// MkdirAll covers both.
func (s *fsService) Lock(key string) (unlock func() error, err error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	lockFile := filepath.Join(s.root, key+".lock")
	if dirErr := os.MkdirAll(filepath.Dir(lockFile), stateDirPerm); dirErr != nil {
		return nil, dirErr
	}
	return lockPath(lockFile)
}
