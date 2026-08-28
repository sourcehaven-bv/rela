// Package secrets loads per-script secret values from .rela/secrets.yaml.
//
// The file has a flat global section (available to all scripts) and an
// optional "overrides" map keyed by script path. When a script is loaded
// its effective secrets are: global values merged with any per-script
// overrides (overrides win).
//
// Example .rela/secrets.yaml:
//
//	jira_api_key: sk-abc123
//	base_url: https://jira.example.com
//
//	overrides:
//	  reports/sync.lua:
//	    jira_api_key: sk-different-key
//
// The file lives in .rela/ which is gitignored by convention.
//
// # Sources
//
// Secrets come from one of two places, checked in order:
//
//  1. systemd's credentials directory, when the unit passed one for THIS
//     project (see [CredentialName]). Preferred because those files live in a
//     per-service tmpfs, are mode 0400 and owned by the service user, are not
//     inherited by child processes, and may be encrypted at rest against the
//     TPM via LoadCredentialEncrypted=.
//  2. The project's .rela/secrets.yaml.
//
// The environment variable is only ever read as a DIRECTORY path; the file
// name inside it is derived from rela's own constants and the project
// directory, never from request input.
package secrets

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the name of the secrets file inside .rela/.
const ConfigFile = "secrets.yaml"

// cacheDirName is the project cache directory holding the secrets file.
//
// Duplicated from project.CacheDir rather than imported: internal/secrets is a
// leaf in .go-arch-lint.yml with no declared dependencies, and depending on
// internal/project to read one constant would invert that.
const cacheDirName = ".rela"

// credentialsDirEnv is the environment variable systemd sets for a unit using
// LoadCredential= / LoadCredentialEncrypted=. systemd owns the directory and
// its modes.
const credentialsDirEnv = "CREDENTIALS_DIRECTORY"

// ErrNotFound indicates that no secrets source exists for the project.
var ErrNotFound = errors.New("secrets: no secrets source (no .rela/secrets.yaml, no systemd credential)")

// warnedPaths records the (file, mode) pairs already reported as
// over-permissive, and warnedCount bounds it.
//
// Keyed per entry rather than a single [sync.Once] because one process can
// serve several projects (appbuild.SharedBase assembles one Services per
// store), and a process-wide latch would silence every project after the first.
// Load runs per script execution and twice per mail send, so without this the
// warning would repeat on every document render.
//
// The cap keeps this from growing without bound: entries are never evicted, and
// the key set is "projects this process has served" — a cardinality the package
// does not control. Past the cap the warning is simply dropped, which is
// acceptable for an advisory diagnostic and preferable to unbounded retention.
var (
	warnedPaths sync.Map
	warnedCount atomic.Int64
)

// maxWarnedPaths caps the warn-once cache. Far above any realistic project
// count, so a normal deployment never reaches it.
const maxWarnedPaths = 1024

// warnKey identifies one (file, mode) pair already reported.
type warnKey struct {
	path string
	perm os.FileMode
}

// Load reads the project's secrets and returns the resolved values for the
// given script path. Global values are merged with per-script overrides
// (overrides take precedence).
//
// Returns ErrNotFound (wrapped) when no source exists — callers should treat
// this as "no secrets configured" and pass an empty map.
func Load(relaDir, scriptPath string) (map[string]string, error) {
	raw, err := readFile(relaDir)
	if err != nil {
		return nil, err
	}
	return resolve(raw, scriptPath), nil
}

// CredentialName returns the systemd credential name that supplies secrets for
// the project whose .rela directory is relaDir.
//
// The name is "rela-secrets-<project>-<hash>", where <project> is the directory
// CONTAINING .rela and <hash> is the first 4 bytes of the SHA-256 of that
// directory's ABSOLUTE path.
//
// # Why both halves are needed
//
// CREDENTIALS_DIRECTORY is process-global while secrets are per-project, so a
// single fixed name would hand one tenant's credential to every other project
// in the same process. The project component alone does not fix that: two
// tenants laid out as /srv/a/proj and /srv/b/proj share a basename, and
// "/srv/<tenant>/<project>" is exactly the layout appbuild.SharedBase exists to
// serve. A basename-only name would silently serve one tenant's live
// credentials to the other, failing OPEN. The path hash restores uniqueness;
// the readable project component is kept so an operator can still recognize
// which unit-file line belongs to which project.
//
// Print the name with `rela secrets credential-name` rather than deriving it by
// hand.
//
// # Rejected inputs
//
// Returns "" — disabling the credentials source — when relaDir is empty, is not
// a <project>/.rela path, or names no project (a bare ".rela", a filesystem
// root, a path that walks upward). A caller passing something else is a bug,
// and minting a credential name for it would let an unrelated file in the
// credentials directory be adopted on the strength of a path artifact.
func CredentialName(relaDir string) string {
	if relaDir == "" {
		return ""
	}
	// Relative paths must be resolved before hashing: "proj/.rela" and
	// "/srv/a/proj/.rela" would otherwise hash differently per working
	// directory, or collide with each other.
	abs, err := filepath.Abs(filepath.Clean(relaDir))
	if err != nil {
		return ""
	}
	// Resolve symlinks too. project.Discover only calls filepath.Abs, so the
	// same project reached through a symlinked parent (a /srv/current ->
	// /srv/releases/N deploy layout) would otherwise hash to a DIFFERENT name
	// than the one the operator printed — the credential would silently stop
	// being found after a switchover. Best-effort: a path that does not exist
	// yet keeps the unresolved form rather than losing the credential entirely.
	if resolved, resErr := filepath.EvalSymlinks(abs); resErr == nil {
		abs = resolved
	}
	// The last segment must actually be the cache directory. Accepting any
	// path would return a credential name for a project that does not exist.
	if filepath.Base(abs) != cacheDirName {
		return ""
	}

	parent := filepath.Dir(abs)
	project := filepath.Base(parent)
	if project == "." || project == ".." || project == string(filepath.Separator) {
		return ""
	}

	sum := sha256.Sum256([]byte(parent))
	return fmt.Sprintf("rela-secrets-%s-%x", project, sum[:4])
}

// rawConfig is the on-disk YAML structure. Top-level keys (except
// "overrides") are global secrets. The "overrides" key maps script
// paths to per-script secret maps.
type rawConfig struct {
	Global    map[string]string            `yaml:",inline"`
	Overrides map[string]map[string]string `yaml:"overrides"`
}

// readFile locates the active secrets source and parses it.
func readFile(relaDir string) (*rawConfig, error) {
	path, fromCredentials := sourcePath(relaDir)
	if path == "" {
		return nil, ErrNotFound
	}

	// systemd owns the modes inside its credentials directory (0400 already),
	// so only the operator-authored file is worth policing.
	if !fromCredentials {
		warnIfPermissive(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &raw, nil
}

// sourcePath picks the secrets file to read, reporting whether it came from the
// systemd credentials directory.
//
// A credential is used only when it exists for THIS project; otherwise the
// project file is used. systemd sets CREDENTIALS_DIRECTORY for any unit loading
// any credential, so its mere presence says nothing about rela.
//
// The returned project-file path is NOT checked for existence — that is
// os.ReadFile's job, which distinguishes "absent" from "unreadable" with a
// better error than a bool could. Returns "" only when there is no project
// directory and no credential, i.e. nothing to read at all.
func sourcePath(relaDir string) (path string, fromCredentials bool) {
	if dir := os.Getenv(credentialsDirEnv); dir != "" {
		if name := CredentialName(relaDir); name != "" {
			// gosec flags the environment variable reaching a path. It is
			// systemd-supplied, not request input, and the file name is
			// rela's own constant plus the project directory — so there is
			// no attacker-controlled component to traverse with.
			candidate := filepath.Join(dir, name)
			//nolint:gosec // G703: path is systemd-supplied config, not request input
			if st, err := os.Stat(candidate); err == nil && st.Mode().IsRegular() {
				return candidate, true
			}
		}
	}
	if relaDir == "" {
		return "", false
	}
	return filepath.Join(relaDir, ConfigFile), false
}

// warnIfPermissive logs once per path when the secrets file is readable beyond
// its owner.
//
// Advisory only: the file is operator-authored, and refusing to load it would
// break working deployments on upgrade. Because nothing is granted or denied on
// the result, the gap between this Stat and the later ReadFile is not a
// security-relevant TOCTOU — an attacker who can swap the file already has
// write access to it.
//
// Logs the path and mode only, never a key name or value.
func warnIfPermissive(path string) {
	// Windows reports synthetic permission bits, so the check would fire on a
	// condition no operator can fix.
	if runtime.GOOS == "windows" {
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		// Missing or unreadable is the read path's problem to report, with a
		// better error than this advisory check could give.
		return
	}

	perm := info.Mode().Perm()
	if perm&0o077 == 0 {
		return
	}
	// Key on path AND mode: a file fixed to 0600 and later regressed to 0666
	// must warn again, or the log would still name the mode it had the first
	// time. A mode that simply stays wrong stays quiet.
	if _, loaded := warnedPaths.LoadOrStore(warnKey{path: path, perm: perm}, struct{}{}); loaded {
		return
	}
	// Count AFTER the store so each distinct entry is counted once. Racing
	// goroutines may overshoot the cap slightly; the bound is to stop unbounded
	// growth, not to be exact.
	if warnedCount.Add(1) > maxWarnedPaths {
		warnedPaths.Delete(warnKey{path: path, perm: perm})
		warnedCount.Add(-1)
		return
	}

	slog.Warn("secrets: file is readable by other users",
		"path", path,
		"mode", fmt.Sprintf("%04o", perm),
		"fix", "chmod 600 "+path)
}

// resetPermissionWarnings clears the warn-once cache.
//
// Test-only: package-level state would otherwise make warning assertions
// depend on test execution order.
func resetPermissionWarnings() {
	warnedPaths.Range(func(k, _ any) bool {
		warnedPaths.Delete(k)
		return true
	})
	warnedCount.Store(0)
}

// resolve merges global secrets with per-script overrides.
func resolve(raw *rawConfig, scriptPath string) map[string]string {
	result := make(map[string]string, len(raw.Global))
	maps.Copy(result, raw.Global)

	if overrides, ok := raw.Overrides[scriptPath]; ok {
		maps.Copy(result, overrides)
	}

	return result
}
