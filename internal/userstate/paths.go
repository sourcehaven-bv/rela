package userstate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvOverride names the environment variable that overrides the
// resolved base directory. Callers that want a custom location
// (tests, power users, scripted deployments) set this.
const EnvOverride = "RELA_USER_STATE_DIR"

// commonSyncDirFragments lists path substrings that typically
// indicate the user has pointed the override at a cloud-sync tree.
// The whole point of this package is to move state *out* of such
// trees; pointing the override back into one defeats the purpose.
// Detection is best-effort and fragment-based — short enough to be
// safe but specific enough to avoid false positives.
var commonSyncDirFragments = []string{
	"/Dropbox/",
	"/OneDrive",
	"/Library/Mobile Documents",
	"/Library/CloudStorage",
	"/Google Drive",
	"/pCloud",
	"/Box Sync",
}

// resolveBase returns the absolute base directory for rela user-state.
// Precedence:
//
//  1. $RELA_USER_STATE_DIR (absolute path only; rejected when empty or relative)
//  2. os.UserConfigDir() (stdlib: Linux XDG_CONFIG_HOME, macOS
//     Library/Application Support, Windows %AppData%)
//
// resolveBase is a pure function parameterized on env and
// userConfigDir so tests can exercise every branch without touching
// process environment or filesystem.
func resolveBase(
	env func(string) string,
	userConfigDir func() (string, error),
) (string, error) {
	if override := strings.TrimSpace(env(EnvOverride)); override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf(
				"userstate: %s must be an absolute path, got %q",
				EnvOverride, override)
		}
		if err := validateNoControlChars(override); err != nil {
			return "", fmt.Errorf("userstate: %s: %w", EnvOverride, err)
		}
		return filepath.Clean(override), nil
	}
	cfg, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("userstate: no user config dir available: %w", err)
	}
	return cfg, nil
}

// validateNoControlChars rejects NUL and ASCII control bytes in s.
// These never appear in legitimate path strings; their presence
// means a caller passed raw user input without validation.
func validateNoControlChars(s string) error {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return errors.New("control or NUL character in path")
		}
	}
	return nil
}

// isInside reports whether candidate resolves to a path equal to or
// beneath boundary. Both arguments are cleaned and compared on
// absolute paths; relative input is treated as "not inside" rather
// than silently evaluated against the process working directory.
func isInside(candidate, boundary string) bool {
	if !filepath.IsAbs(candidate) || !filepath.IsAbs(boundary) {
		return false
	}
	c := filepath.Clean(candidate)
	b := filepath.Clean(boundary)
	if c == b {
		return true
	}
	rel, err := filepath.Rel(b, c)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != "." && !filepath.IsAbs(rel)
}

// detectSyncDir returns a matching fragment if path looks like it
// sits under a cloud-sync directory, or "" if no fragment matches.
// The return value is suitable for diagnostic messages; callers
// decide whether to warn or fail.
func detectSyncDir(path string) string {
	for _, frag := range commonSyncDirFragments {
		if strings.Contains(path, frag) {
			return frag
		}
	}
	return ""
}

// resolveForRepo joins base with the product subpath and the
// repo-id. The repo-id segment is validated by validateRepoID at
// construction time; validation is not repeated here.
func resolveForRepo(base, repoID string) string {
	return filepath.Join(base, "rela", "repos", repoID)
}

// ensureDir creates dir with stateDirPerm, idempotent.
// Separated from the service so platform-specific indexer opt-out
// hooks can run on first creation.
func ensureDir(dir string) (created bool, err error) {
	info, statErr := os.Stat(dir)
	if statErr == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("userstate: %s exists but is not a directory", dir)
		}
		return false, nil
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return false, fmt.Errorf("userstate: stat %s: %w", dir, statErr)
	}
	if err := os.MkdirAll(dir, stateDirPerm); err != nil {
		return false, fmt.Errorf("userstate: create %s: %w", dir, err)
	}
	return true, nil
}
