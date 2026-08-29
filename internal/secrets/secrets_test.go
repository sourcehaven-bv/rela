package secrets

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestLoad_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir, "test.lua")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLoad_GlobalOnly(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
api_key: sk-abc123
base_url: https://example.com
`)

	sec, err := Load(dir, "any-script.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["api_key"] != "sk-abc123" {
		t.Errorf("expected sk-abc123, got %q", sec["api_key"])
	}
	if sec["base_url"] != "https://example.com" {
		t.Errorf("expected https://example.com, got %q", sec["base_url"])
	}
}

func TestLoad_OverrideMerge(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
api_key: global-key
shared: shared-value
overrides:
  special.lua:
    api_key: special-key
    extra: extra-value
`)

	// Script with override
	sec, err := Load(dir, "special.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["api_key"] != "special-key" {
		t.Errorf("expected override key, got %q", sec["api_key"])
	}
	if sec["shared"] != "shared-value" {
		t.Errorf("expected global shared value, got %q", sec["shared"])
	}
	if sec["extra"] != "extra-value" {
		t.Errorf("expected override extra, got %q", sec["extra"])
	}

	// Script without override gets globals only
	sec2, err := Load(dir, "other.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec2["api_key"] != "global-key" {
		t.Errorf("expected global key, got %q", sec2["api_key"])
	}
	if _, ok := sec2["extra"]; ok {
		t.Error("other.lua should not have extra key")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "")

	sec, err := Load(dir, "test.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sec) != 0 {
		t.Errorf("expected empty map, got %v", sec)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "{{invalid")

	_, err := Load(dir, "test.lua")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_OverridesOnly(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
overrides:
  my-script.lua:
    token: secret
`)

	sec, err := Load(dir, "my-script.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["token"] != "secret" {
		t.Errorf("expected secret, got %q", sec["token"])
	}

	// Script not in overrides gets empty map
	sec2, err := Load(dir, "other.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sec2) != 0 {
		t.Errorf("expected empty map, got %v", sec2)
	}
}

func writeYAML(t *testing.T, dir, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(content), 0600)
	if err != nil {
		t.Fatalf("write secrets.yaml: %v", err)
	}
}

// --- Permission warning (TKT-RX7I97) ---

// captureWarnings redirects slog to a buffer for the duration of the test and
// clears the warn-once cache, which is package-level state that would
// otherwise make these assertions depend on test execution order.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	resetPermissionWarnings()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(prev)
		resetPermissionWarnings()
	})
	return &buf
}

// writeMode writes a secrets file at an explicit mode, defeating umask.
func writeMode(t *testing.T, dir, content string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(dir, ConfigFile)
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write secrets.yaml: %v", err)
	}
	// WriteFile applies umask, so set the mode explicitly.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
}

func TestLoad_WarnsOnPermissiveMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	for _, tc := range []struct {
		name     string
		mode     os.FileMode
		wantWarn bool
	}{
		{"owner only", 0o600, false},
		{"owner rw, group r", 0o640, true},
		{"world readable", 0o644, true},
		{"world writable", 0o666, true},
		{"owner read only", 0o400, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureWarnings(t)
			dir := t.TempDir()
			writeMode(t, dir, "api_key: sk-abc123\n", tc.mode)

			sec, err := Load(dir, "s.lua")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// The warning is advisory: secrets must still load.
			if sec["api_key"] != "sk-abc123" {
				t.Errorf("secrets not returned: got %q", sec["api_key"])
			}

			got := strings.Contains(buf.String(), "readable by other users")
			if got != tc.wantWarn {
				t.Errorf("warning present = %v, want %v (mode %04o)\nlog: %s",
					got, tc.wantWarn, tc.mode, buf.String())
			}
		})
	}
}

func TestLoad_WarningNeverEchoesSecrets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	buf := captureWarnings(t)
	dir := t.TempDir()
	writeMode(t, dir, "api_key: sk-super-secret\ntoken: tok-confidential\n", 0o644)

	if _, err := Load(dir, "s.lua"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log := buf.String()
	if !strings.Contains(log, "readable by other users") {
		t.Fatalf("expected a warning, got: %s", log)
	}
	// A log line that quoted the file would defeat the point of the check.
	for _, leak := range []string{"sk-super-secret", "tok-confidential", "api_key", "token"} {
		if strings.Contains(log, leak) {
			t.Errorf("warning leaked %q into the log: %s", leak, log)
		}
	}
}

func TestLoad_WarnsOncePerPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	buf := captureWarnings(t)
	dir := t.TempDir()
	writeMode(t, dir, "api_key: sk-abc123\n", 0o644)

	// Load runs per script execution and twice per mail send; a warning per
	// call would spam a document-rendering server (RR-V1G22E).
	for range 3 {
		if _, err := Load(dir, "s.lua"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if n := strings.Count(buf.String(), "readable by other users"); n != 1 {
		t.Errorf("warning count = %d, want 1\nlog: %s", n, buf.String())
	}
}

func TestLoad_WarnsAgainWhenModeChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	buf := captureWarnings(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFile)
	writeMode(t, dir, "api_key: sk-abc123\n", 0o640)

	if _, err := Load(dir, "s.lua"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fixed, then regressed to something WORSE. Suppressing the second warning
	// would leave the log naming 0640 while the file is world-writable.
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if _, err := Load(dir, "s.lua"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if _, err := Load(dir, "s.lua"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log := buf.String()
	if n := strings.Count(log, "readable by other users"); n != 2 {
		t.Errorf("warning count = %d, want 2 (0640 then 0666)\nlog: %s", n, log)
	}
	if !strings.Contains(log, "0666") {
		t.Errorf("the regressed mode was not reported\nlog: %s", log)
	}
}

func TestLoad_WarnsPerProjectNotGlobally(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	buf := captureWarnings(t)
	// One process can serve several projects; a global latch would silence
	// every project after the first.
	dirA, dirB := t.TempDir(), t.TempDir()
	writeMode(t, dirA, "api_key: a\n", 0o644)
	writeMode(t, dirB, "api_key: b\n", 0o644)

	for _, d := range []string{dirA, dirB} {
		if _, err := Load(d, "s.lua"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if n := strings.Count(buf.String(), "readable by other users"); n != 2 {
		t.Errorf("warning count = %d, want 2 (one per project)\nlog: %s", n, buf.String())
	}
}

// --- systemd credentials directory (TKT-RX7I97) ---

// writeCredential places a project-scoped credential in a fake systemd
// credentials directory.
func writeCredential(t *testing.T, credDir, relaDir, content string, mode os.FileMode) {
	t.Helper()
	name := CredentialName(relaDir)
	if name == "" {
		t.Fatalf("CredentialName(%q) is empty", relaDir)
	}
	path := filepath.Join(credDir, name)
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write credential: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod credential: %v", err)
	}
}

// projectRelaDir builds a realistic <project>/.rela layout, which
// CredentialName derives the project name from.
func projectRelaDir(t *testing.T, project string) string {
	t.Helper()
	return projectRelaDirUnder(t, t.TempDir(), project)
}

// projectRelaDirUnder builds <root>/<project>/.rela, so two projects can share
// a basename under different roots.
func projectRelaDirUnder(t *testing.T, root, project string) string {
	t.Helper()
	dir := filepath.Join(root, project, ".rela")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return dir
}

func TestLoad_PrefersCredentialsDirectory(t *testing.T) {
	captureWarnings(t)
	relaDir := projectRelaDir(t, "acme")
	credDir := t.TempDir()
	t.Setenv(credentialsDirEnv, credDir)

	// Distinct values prove which source won.
	writeMode(t, relaDir, "api_key: from-project-file\n", 0o600)
	writeCredential(t, credDir, relaDir, "api_key: from-systemd\n", 0o400)

	sec, err := Load(relaDir, "s.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["api_key"] != "from-systemd" {
		t.Errorf("api_key = %q, want from-systemd", sec["api_key"])
	}
}

func TestLoad_FallsBackWhenNoCredentialForProject(t *testing.T) {
	captureWarnings(t)
	relaDir := projectRelaDir(t, "acme")
	credDir := t.TempDir()
	// systemd sets the variable for ANY unit loading ANY credential, so an
	// unrelated credential must not shadow the project file.
	if err := os.WriteFile(filepath.Join(credDir, "unrelated-thing"), []byte("x: y\n"), 0o400); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv(credentialsDirEnv, credDir)
	writeMode(t, relaDir, "api_key: from-project-file\n", 0o600)

	sec, err := Load(relaDir, "s.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["api_key"] != "from-project-file" {
		t.Errorf("api_key = %q, want from-project-file", sec["api_key"])
	}
}

func TestLoad_CredentialsAreProjectScoped(t *testing.T) {
	captureWarnings(t)
	// The regression this guards: CREDENTIALS_DIRECTORY is process-global but
	// secrets are per-project, so an unscoped lookup would serve one tenant's
	// credential to every other project in the process (RR-Y2O7C6).
	credDir := t.TempDir()
	t.Setenv(credentialsDirEnv, credDir)

	relaA := projectRelaDir(t, "alpha")
	relaB := projectRelaDir(t, "beta")
	writeCredential(t, credDir, relaA, "api_key: alpha-secret\n", 0o400)
	writeCredential(t, credDir, relaB, "api_key: beta-secret\n", 0o400)

	for _, tc := range []struct{ relaDir, want string }{
		{relaA, "alpha-secret"},
		{relaB, "beta-secret"},
	} {
		sec, err := Load(tc.relaDir, "s.lua")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sec["api_key"] != tc.want {
			t.Errorf("api_key = %q, want %q — projects saw each other's secrets",
				sec["api_key"], tc.want)
		}
	}
}

func TestLoad_SameBasenameProjectsDoNotShareACredential(t *testing.T) {
	captureWarnings(t)
	// End-to-end form of TestCredentialName_DistinctProjectsSharingABasename:
	// two tenants each with a project directory named "proj".
	credDir := t.TempDir()
	t.Setenv(credentialsDirEnv, credDir)

	relaA := projectRelaDirUnder(t, t.TempDir(), "proj")
	relaB := projectRelaDirUnder(t, t.TempDir(), "proj")

	// Only tenant A has a credential; B must fall back to its own file rather
	// than adopt A's secrets.
	writeCredential(t, credDir, relaA, "api_key: tenant-a-secret\n", 0o400)
	writeMode(t, relaB, "api_key: tenant-b-own-file\n", 0o600)

	secA, err := Load(relaA, "s.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secA["api_key"] != "tenant-a-secret" {
		t.Errorf("tenant A api_key = %q, want tenant-a-secret", secA["api_key"])
	}

	secB, err := Load(relaB, "s.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secB["api_key"] != "tenant-b-own-file" {
		t.Errorf("tenant B api_key = %q, want tenant-b-own-file — it received another tenant's credential",
			secB["api_key"])
	}
}

func TestLoad_ConcurrentLoadsWarnOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	buf := captureWarnings(t)
	dir := t.TempDir()
	writeMode(t, dir, "api_key: sk-abc123\n", 0o644)

	// Emitting exactly one warning is the whole reason warnedPaths is a
	// sync.Map; parallel script executions are the realistic caller.
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if _, err := Load(dir, "s.lua"); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()

	if n := strings.Count(buf.String(), "readable by other users"); n != 1 {
		t.Errorf("warning count = %d, want 1 under concurrency\nlog: %s", n, buf.String())
	}
}

func TestLoad_NoPermissionWarningForCredentials(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	buf := captureWarnings(t)
	relaDir := projectRelaDir(t, "acme")
	credDir := t.TempDir()
	t.Setenv(credentialsDirEnv, credDir)
	// systemd owns these modes; policing them would be noise the operator
	// cannot act on.
	writeCredential(t, credDir, relaDir, "api_key: k\n", 0o644)

	if _, err := Load(relaDir, "s.lua"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "readable by other users") {
		t.Errorf("warned about a systemd-owned credential: %s", buf.String())
	}
}

func TestLoad_CredentialsDirectoryEdgeCases(t *testing.T) {
	captureWarnings(t)
	relaDir := projectRelaDir(t, "acme")
	writeMode(t, relaDir, "api_key: from-project-file\n", 0o600)

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T)
	}{
		{"unset", func(t *testing.T) {
			t.Helper()
			// Genuinely absent, not set-to-empty: Getenv cannot tell them
			// apart, but Unsetenv is what a non-systemd process actually sees.
			t.Setenv(credentialsDirEnv, "")
			if err := os.Unsetenv(credentialsDirEnv); err != nil {
				t.Fatalf("unsetenv: %v", err)
			}
		}},
		{"empty string", func(t *testing.T) {
			t.Helper()
			t.Setenv(credentialsDirEnv, "")
		}},
		{"nonexistent directory", func(t *testing.T) {
			t.Helper()
			t.Setenv(credentialsDirEnv, filepath.Join(t.TempDir(), "does-not-exist"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			sec, err := Load(relaDir, "s.lua")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sec["api_key"] != "from-project-file" {
				t.Errorf("api_key = %q, want from-project-file", sec["api_key"])
			}
		})
	}
}

func TestLoad_CredentialsDirectoryIgnoresNonRegularFile(t *testing.T) {
	captureWarnings(t)
	relaDir := projectRelaDir(t, "acme")
	credDir := t.TempDir()
	t.Setenv(credentialsDirEnv, credDir)
	writeMode(t, relaDir, "api_key: from-project-file\n", 0o600)

	// A directory where the credential should be must not be read as YAML.
	if err := os.MkdirAll(filepath.Join(credDir, CredentialName(relaDir)), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	sec, err := Load(relaDir, "s.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["api_key"] != "from-project-file" {
		t.Errorf("api_key = %q, want from-project-file", sec["api_key"])
	}
}

func TestLoad_InvalidYAMLInCredentialDoesNotFallBack(t *testing.T) {
	captureWarnings(t)
	relaDir := projectRelaDir(t, "acme")
	credDir := t.TempDir()
	t.Setenv(credentialsDirEnv, credDir)
	// Falling back here would let a corrupt credential silently resolve to a
	// stale project file, hiding the misconfiguration.
	writeMode(t, relaDir, "api_key: from-project-file\n", 0o600)
	writeCredential(t, credDir, relaDir, "{{invalid", 0o400)

	if _, err := Load(relaDir, "s.lua"); err == nil {
		t.Fatal("expected an error for invalid YAML in the credential")
	}
}

func TestCredentialName(t *testing.T) {
	sep := string(filepath.Separator)

	t.Run("names the project and is stable", func(t *testing.T) {
		dir := filepath.Join(sep, "srv", "acme", ".rela")
		got := CredentialName(dir)
		if !strings.HasPrefix(got, "rela-secrets-acme-") {
			t.Errorf("CredentialName(%q) = %q, want a rela-secrets-acme- prefix", dir, got)
		}
		// The operator writes this into a unit file; it must not move between
		// runs or a working deployment would break on restart.
		if again := CredentialName(dir); again != got {
			t.Errorf("not stable: %q then %q", got, again)
		}
	})

	t.Run("trailing separator is the same project", func(t *testing.T) {
		base := filepath.Join(sep, "srv", "acme", ".rela")
		if CredentialName(base) != CredentialName(base+sep) {
			t.Error("a trailing separator changed the credential name")
		}
	})

	t.Run("symlinked path resolves to the same name as its target", func(t *testing.T) {
		// A /srv/current -> /srv/releases/N deploy layout must not change the
		// credential name on switchover, or the credential silently stops
		// being found.
		root := t.TempDir()
		target := filepath.Join(root, "real", "acme", ".rela")
		if err := os.MkdirAll(target, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		link := filepath.Join(root, "link")
		if err := os.Symlink(filepath.Join(root, "real"), link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		viaLink := filepath.Join(link, "acme", ".rela")
		if got, want := CredentialName(viaLink), CredentialName(target); got != want {
			t.Errorf("via symlink %q != via target %q", got, want)
		}
	})

	t.Run("relative path resolves to the same name as its absolute form", func(t *testing.T) {
		// Otherwise the name would depend on the working directory.
		root := t.TempDir()
		abs := filepath.Join(root, "acme", ".rela")
		if err := os.MkdirAll(abs, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Chdir(root)
		if got, want := CredentialName(filepath.Join("acme", ".rela")), CredentialName(abs); got != want {
			t.Errorf("relative %q != absolute %q", got, want)
		}
	})

	for _, tc := range []struct {
		name    string
		relaDir string
	}{
		{"empty disables the source", ""},
		{"filesystem root", filepath.Join(sep, ".rela")},
		{"not a .rela directory", filepath.Join(sep, "srv", "acme")},
		{"deep path that is not a project", filepath.Join(sep, "some", "deep", "path")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CredentialName(tc.relaDir); got != "" {
				t.Errorf("CredentialName(%q) = %q, want \"\" (source disabled)", tc.relaDir, got)
			}
		})
	}
}

// TestCredentialName_DistinctProjectsSharingABasename guards the cross-tenant
// leak: /srv/a/proj and /srv/b/proj are different projects, and
// "/srv/<tenant>/<project>" is the layout appbuild.SharedBase serves. A
// basename-only name would give both the same credential, failing OPEN — one
// tenant would receive the other's live secrets.
func TestCredentialName_DistinctProjectsSharingABasename(t *testing.T) {
	sep := string(filepath.Separator)
	a := CredentialName(filepath.Join(sep, "srv", "tenant-a", "proj", ".rela"))
	b := CredentialName(filepath.Join(sep, "srv", "tenant-b", "proj", ".rela"))

	if a == "" || b == "" {
		t.Fatalf("expected names for both projects, got %q and %q", a, b)
	}
	if a == b {
		t.Fatalf("distinct projects collided on %q — one tenant would receive the other's secrets", a)
	}
	// Both should still be recognizable to the operator.
	for _, got := range []string{a, b} {
		if !strings.HasPrefix(got, "rela-secrets-proj-") {
			t.Errorf("%q lost the readable project component", got)
		}
	}
}

func TestLoad_EmptyRelaDirWithoutCredentials(t *testing.T) {
	captureWarnings(t)
	t.Setenv(credentialsDirEnv, "")
	// script.cacheDirFor passes "" for a zero-value deps; that must stay a
	// clean ErrNotFound rather than reading the process's working directory.
	if _, err := Load("", "s.lua"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
