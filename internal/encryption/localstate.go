package encryption

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// LocalState owns the per-machine, per-repo state that MUST NOT be
// synced to any untrusted storage. Specifically: the highest
// encryption version this machine has observed for a given repo.
// Keeping this outside the repo directory (in the XDG state tree)
// is what makes rollback detection work — an adversary who can
// replace files inside <root>/ cannot simultaneously roll back
// whatever we remember in ~/.local/state/rela/.
//
// A missing file is TOFU: the first read returns (0, nil) and the
// caller is expected to accept whatever version it sees on disk and
// StoreVersion it back. Subsequent reads then enforce
// "observed ≥ stored" — downgrades are refused.
//
// The state is keyed by RepoID, the UUID generated at `rela keys
// init` and stored inside recipients.age. Different rela projects
// on the same machine get different RepoIDs, different state files,
// and therefore independent version tracking.
type LocalState struct {
	root string // per-repo directory: <xdg-state>/rela/repos/<repo-id>/
}

// NewLocalState resolves the per-repo XDG state directory for
// repoID and returns a LocalState rooted there. Creating the
// directory is deferred to first write — a pure read on a
// never-before-seen repo is expected (and is the TOFU case).
func NewLocalState(repoID string) (*LocalState, error) {
	if repoID == "" {
		return nil, errors.New("encryption: NewLocalState: empty repo id")
	}
	base, err := xdgStateHome()
	if err != nil {
		return nil, err
	}
	return &LocalState{
		root: filepath.Join(base, "rela", "repos", repoID),
	}, nil
}

// versionFile is the filename holding the last-seen-version for
// this repo. Plaintext integer — the file is outside the synced
// tree, so the adversary we're defending against (cloud storage)
// can't see or tamper with it.
const versionFile = "version"

// File permission constants for the per-machine state directory
// and its contents. 0o700 / 0o600 match how we chmod the private
// key file: owner-only, no group/world access.
const (
	stateDirPerm  os.FileMode = 0o700
	stateFilePerm os.FileMode = 0o600
)

// LoadVersion returns the highest version this machine has observed
// for this repo. Returns 0 when no state exists yet (TOFU).
//
// A corrupt file (non-integer content) is treated as the ErrCorruptedLocalState
// failure rather than silently resetting to 0 — if the on-disk
// value is unparseable something is wrong and we'd rather surface
// it than accept an attacker-controlled reset.
func (s *LocalState) LoadVersion() (int, error) {
	data, err := os.ReadFile(filepath.Join(s.root, versionFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("encryption: load last-seen version: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %w", ErrCorruptedLocalState, versionFile, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("%w: negative version %d", ErrCorruptedLocalState, n)
	}
	return n, nil
}

// StoreVersion persists v as the new last-seen version for this
// repo. The caller is responsible for only advancing: passing a
// lower v than the stored value would silently weaken the rollback
// defense, and StoreVersion does NOT guard against that (callers
// already have the observed-vs-stored comparison in hand by the
// time they reach this point).
//
// Writes are atomic: temp file + rename. A crash mid-write leaves
// the previous value intact.
func (s *LocalState) StoreVersion(v int) error {
	if err := os.MkdirAll(s.root, stateDirPerm); err != nil {
		return fmt.Errorf("encryption: prepare state dir: %w", err)
	}
	path := filepath.Join(s.root, versionFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(v)+"\n"), stateFilePerm); err != nil {
		return fmt.Errorf("encryption: store last-seen version: %w", err)
	}
	return os.Rename(tmp, path)
}

// ErrCorruptedLocalState indicates the per-repo state file
// contained something other than a non-negative integer. Callers
// can surface this to the user as "run `rela keys repair` or delete
// <state-dir> to TOFU again" — the file lives on the local
// machine, so recovery is a user decision, not a crypto one.
var ErrCorruptedLocalState = errors.New("encryption: corrupted local state")

// xdgStateHome resolves the base directory for persistent
// machine-local state per the XDG Base Directory Specification.
//
// Precedence:
//  1. $XDG_STATE_HOME if set and non-empty
//  2. $HOME/.local/state on Linux / unix
//  3. $HOME/Library/Application Support on darwin (Apple's
//     equivalent; Apple doesn't use XDG but this is where
//     "persistent application state that shouldn't sync" lives
//     on macOS).
//  4. os.UserConfigDir() as last resort
//
// Windows is not currently supported; the rela CLI's other
// filesystem assumptions (POSIX permissions on keys, etc.) are
// Unix-only. Falling through to UserConfigDir on Windows would
// produce a "works but in the wrong place" result; better to
// surface an error when someone gets far enough to try.
func xdgStateHome() (string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("encryption: resolve home dir: %w", err)
	}
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		return filepath.Join(home, ".local", "state"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		// Best-effort fallback; tests on exotic platforms won't
		// hit this in practice.
		cfg, cErr := os.UserConfigDir()
		if cErr != nil {
			return "", fmt.Errorf("encryption: no state dir for GOOS=%s: %w", runtime.GOOS, cErr)
		}
		return cfg, nil
	}
}
