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
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the name of the secrets file inside .rela/.
const ConfigFile = "secrets.yaml"

// credentialsDirEnv is the environment variable systemd sets for a unit using
// LoadCredential= / LoadCredentialEncrypted=. systemd owns the directory and
// its modes.
const credentialsDirEnv = "CREDENTIALS_DIRECTORY"

// ErrNotFound indicates that no secrets source exists for the project.
var ErrNotFound = errors.New("secrets: no .rela/secrets.yaml")

// warnedPaths records the secrets files already reported as over-permissive.
//
// Keyed by path rather than a single [sync.Once] because one process can serve
// several projects (appbuild.SharedBase assembles one Services per store), and
// a process-wide latch would silence every project after the first. Load runs
// per script execution and twice per mail send, so without this the warning
// would repeat on every document render.
var warnedPaths sync.Map

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
// The name is project-scoped ("rela-secrets-<project>") because
// CREDENTIALS_DIRECTORY is process-global while secrets are per-project: a
// single fixed name would hand one tenant's credential to every other project
// in the same process. The project component is the name of the directory
// CONTAINING .rela, which is what an operator sees as the project.
//
// Returns "" when relaDir is empty, which disables the credentials source.
func CredentialName(relaDir string) string {
	if relaDir == "" {
		return ""
	}
	project := filepath.Base(filepath.Dir(filepath.Clean(relaDir)))
	// Base can yield "." or a separator for degenerate inputs; those name no
	// project, so there is nothing to scope a credential to.
	if project == "." || project == string(filepath.Separator) {
		return ""
	}
	return "rela-secrets-" + project
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
		return nil, fmt.Errorf("%w", ErrNotFound)
	}

	// systemd owns the modes inside its credentials directory (0400 already),
	// so only the operator-authored file is worth policing.
	if !fromCredentials {
		warnIfPermissive(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w", ErrNotFound)
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
// Returns "" when neither source exists.
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
	if _, loaded := warnedPaths.LoadOrStore(path, struct{}{}); loaded {
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
