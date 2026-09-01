package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/user/repo.git", "repo"},
		{"https://github.com/user/repo", "repo"},
		{"https://gitlab.com/group/subgroup/project.git", "project"},
		{"https://bitbucket.org/team/repo.git", "repo"},
		{"git@github.com:user/repo.git", "repo"}, // SSH URL - extracts via fallback
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := ExtractRepoName(tt.url)
			if got != tt.expected {
				t.Errorf("ExtractRepoName(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

// ExtractRepoName feeds a filepath.Join in the desktop clone flow, so a URL
// whose last path segment is a traversal must not produce one.
func TestExtractRepoName_RejectsTraversal(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"dotdot segment", "https://github.com/user/.."},
		{"trailing dotdot", "https://github.com/user/../.."},
		{"dot segment", "https://github.com/user/."},
		{"encoded separator", "https://github.com/user/..%2f.."},
		{"dotdot git suffix", "https://github.com/user/...git"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRepoName(tt.url)
			if got == "" {
				return // rejected outright, which is the safe outcome
			}
			base := t.TempDir()
			joined := filepath.Join(base, got)
			if !strings.HasPrefix(joined, base+string(filepath.Separator)) {
				t.Errorf("ExtractRepoName(%q) = %q escapes base (join = %q)", tt.url, got, joined)
			}
		})
	}
}

// Clone must refuse a path that escapes BaseDir: storeCredentials writes a
// plaintext OAuth token under the clone path, so a traversal leaks it outside
// the directory the operator chose.
func TestClone_RejectsPathEscapingBaseDir(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{"parent of base", filepath.Join(base, "..")},
		{"sibling via traversal", filepath.Join(base, "..", "elsewhere")},
		{"base itself", base},
		{"unrelated absolute", "/tmp/somewhere-else"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Clone(CloneOptions{
				URL:     "https://github.com/user/repo.git",
				Path:    tt.path,
				BaseDir: base,
			})
			if err == nil {
				t.Fatalf("Clone(path=%q, base=%q) = nil, want containment error", tt.path, base)
			}
			contained := strings.Contains(err.Error(), "escapes base directory") ||
				strings.Contains(err.Error(), "not inside")
			if !contained {
				t.Errorf("Clone error = %v, want a containment error", err)
			}
		})
	}
}

// A path genuinely inside BaseDir must pass containment.
//
// Against containedPath directly, NOT through Clone. Driving this case through
// Clone meant reaching the real git fetch: the containment check passes, so
// there is nothing left to stop it, and the test made a live unauthenticated
// request to github.com on every run. That is slow, flaky when GitHub is, and
// behaves differently on a CI runner with egress blocked than on a laptop.
//
// The negative cases still go through Clone on purpose — there the boundary
// itself is the claim, and the check must reject before any network happens.
// Here the claim is only "this path is judged contained", which is precisely
// what containedPath returns.
func TestClone_AllowsPathInsideBaseDir(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "repo")

	got, err := containedPath(base, path)
	if err != nil {
		t.Fatalf("containedPath(%q, %q) = error %v, want the contained path", base, path, err)
	}
	if got != path {
		t.Errorf("containedPath(%q, %q) = %q, want %q", base, path, got, path)
	}
}

func TestIsValidRepoURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://github.com/user/repo.git", true},
		{"https://github.com/user/repo", true},
		{"https://gitlab.com/group/project", true},
		{"https://bitbucket.org/team/repo", true},
		{"https://git.example.com/org/repo", true},
		{"http://github.com/user/repo", false},  // HTTP not allowed
		{"git@github.com:user/repo.git", false}, // SSH not supported
		{"https://github.com", false},           // No repo path
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := IsValidRepoURL(tt.url)
			if got != tt.valid {
				t.Errorf("IsValidRepoURL(%q) = %v, want %v", tt.url, got, tt.valid)
			}
		})
	}
}

func TestClone_EmptyURL(t *testing.T) {
	err := Clone(CloneOptions{
		Path: "/tmp/test",
	})
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestClone_EmptyPath(t *testing.T) {
	err := Clone(CloneOptions{
		URL: "https://github.com/user/repo.git",
	})
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestClone_PathExists(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing")
	if err := os.MkdirAll(existingPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// BaseDir is REQUIRED (TKT-S2SFTG). Without it this test would fail at the
	// containment guard and never reach the os.Stat check it exists for --
	// still non-nil, so a bare err != nil assertion would keep passing while
	// the path-exists branch lost all coverage. Hence both the base and the
	// error-text assertion below.
	err := Clone(CloneOptions{
		URL:     "https://github.com/user/repo.git",
		Path:    existingPath,
		BaseDir: dir,
	})
	if err == nil {
		t.Fatal("expected error when path exists")
	}
	if !strings.Contains(err.Error(), "path already exists") {
		t.Errorf("Clone error = %v, want the path-already-exists error", err)
	}
}

// An empty BaseDir must be REJECTED, not treated as "skip the check" (TKT-S2SFTG /
// issue #1270, IB-review of #1247).
//
// The doc on CloneOptions.BaseDir says containment lives at the Clone boundary
// "so a future caller that forgets to sanitize is still safe". Skipping the
// check when BaseDir is empty made that claim false in exactly the case it
// describes: a caller who forgets BaseDir gets no containment at all, silently.
// The forgetful caller is the whole threat model, and it was the one case that
// failed open.
//
// Rejecting rather than defaulting: there is no safe directory to guess. A
// default of "the process CWD" would contain the clone somewhere the caller
// never named, which is a different surprise, not a smaller one.
func TestClone_RejectsEmptyBaseDir(t *testing.T) {
	// A path that would be perfectly fine under a real BaseDir — so this test
	// fails only because the base is missing, not because the path is bad.
	err := Clone(CloneOptions{
		URL:  "https://github.com/user/repo.git",
		Path: filepath.Join(t.TempDir(), "repo"),
	})
	if err == nil {
		t.Fatal("Clone with no BaseDir = nil, want a required-BaseDir error")
	}
	if !strings.Contains(err.Error(), "base directory is required") {
		t.Errorf("Clone error = %v, want a required-BaseDir error", err)
	}
}

// The traversal case, with BaseDir omitted. This is the shape the issue
// describes: ExtractRepoName can return "..", so a caller that forgets BaseDir
// would clone outside the intended directory AND drop a plaintext OAuth token
// there via storeCredentials.
func TestClone_EmptyBaseDir_DoesNotSilentlyAllowTraversal(t *testing.T) {
	base := t.TempDir()

	err := Clone(CloneOptions{
		URL:  "https://github.com/user/repo.git",
		Path: filepath.Join(base, ".."),
	})
	if err == nil {
		t.Fatal("Clone(path=<base>/.., no BaseDir) = nil, want rejection")
	}
	// It must fail on the MISSING BASE, not merely by accident downstream (a
	// network error or "path already exists" would also be non-nil, and would
	// leave the traversal unguarded on a machine where those happen to pass).
	if !strings.Contains(err.Error(), "base directory is required") {
		t.Errorf("Clone error = %v, want it to fail on the missing base directory", err)
	}
}

// A root base contains every path, so containment would pass unconditionally
// while looking like it had checked something — the same silent no-op as an
// empty base, reached by a different route (a config default that resolves to
// "/", say). Refused for the same reason.
func TestContainedPath_RejectsRootBase(t *testing.T) {
	if _, err := containedPath(string(filepath.Separator), "/etc/passwd"); err == nil {
		t.Fatal(`containedPath("/", "/etc/passwd") = nil error, want rejection`)
	} else if !strings.Contains(err.Error(), "filesystem root") {
		t.Errorf("containedPath error = %v, want it to name the filesystem root", err)
	}
}

// The root-base guard must recognise a WINDOWS root too (TKT-T7G7LT / issue
// #1498). rela-desktop ships as a Windows MSI, and until this test existed the
// guard compared only against "/" — so on Windows a base of `C:\` sailed
// through, filepath.Rel succeeded for every path on the drive, and the
// containment check returned "contained" for anything at all. That is exactly
// the fail-open PR #1496 was written to close, still open on a shipped
// platform.
//
// This test runs on Linux CI, which is the whole difficulty: Linux filepath has
// no concept of a volume, so it Cleans `C:\` to the relative filename `C:\` and
// Abs would prefix it with the working directory. Driving the Windows case
// through containedPath here would therefore assert on a path that never
// entered the branch under test — a green result proving nothing.
//
// So the table addresses isFilesystemRoot directly, feeding it the
// (VolumeName, Clean, Separator) triples a Windows filepath WOULD produce — all
// three vary per platform, which is why all three are parameters. The values
// come from the stdlib's own rules: volumeNameLen returns 2 for a drive-letter
// path and the full `\\host\share` prefix for UNC; Clean copies the volume
// verbatim; Separator is '\\' on Windows.
//
// The limit of this technique is worth stating, because it bit once already.
// Every input here is a hand transcription of what Clean is believed to return,
// and nothing in this file verifies that belief — so a wrong transcription
// yields a green test over a string Windows never emits. That is exactly what
// happened with `\\server\share` (RR-Q73HKS): the table asserted the
// trailing-separator form, Clean actually returns the bare volume, and the real
// spelling went unguarded. TestIsFilesystemRoot_RealWindowsPaths in
// clone_windows_test.go is the counterweight — it derives the triples from the
// host filepath instead of from memory. It does not run on Linux CI, which is
// precisely why this table exists too; neither is sufficient alone.
func TestIsFilesystemRoot(t *testing.T) {
	const (
		unixSep    = '/'
		windowsSep = '\\'
	)

	tests := []struct {
		name    string
		volume  string
		cleaned string
		sep     rune
		want    bool
	}{
		// Unix: the case #1496 already handled, kept as a row so a future
		// rewrite of the predicate cannot drop it silently.
		{"unix root", "", "/", unixSep, true},
		{"unix directory", "", "/home/dev", unixSep, false},
		{"unix nested directory", "", "/home/dev/clones", unixSep, false},

		// Windows drive roots: the defect this ticket fixes.
		{"windows drive root", "C:", `C:\`, windowsSep, true},
		{"windows drive root lowercase", "c:", `c:\`, windowsSep, true},
		{"windows directory on drive", "C:", `C:\Users\dev`, windowsSep, false},
		{"windows nested directory on drive", "C:", `C:\Users\dev\clones`, windowsSep, false},

		// UNC share roots are roots in the same sense: everything on the share
		// is below them, so a base of `\\server\share` constrains nothing.
		//
		// BOTH spellings must be caught, and the second is the one that
		// actually occurs. Clean(`\\server\share`) returns it UNCHANGED — the
		// whole string is the volume name, so there is no remainder to root and
		// no trailing separator is appended. An earlier version of this table
		// listed only the `\` form, asserting on a string Windows never
		// produces for that input; it passed green while the real form went
		// unguarded and filepath.Rel reported every path on the share as
		// contained. Found in code review (RR-Q73HKS).
		{"unc share root trailing separator", `\\server\share`, `\\server\share\`, windowsSep, true},
		{"unc share root bare volume", `\\server\share`, `\\server\share`, windowsSep, true},
		{"unc directory on share", `\\server\share`, `\\server\share\repos`, windowsSep, false},

		// Extended-length drive spelling. VolumeName(`\\?\C:`) is the whole
		// string (the stdlib's own volumenametests pins `\\?\x` → `\\?\x`), so
		// both the bare and trailing-separator forms are volume roots.
		//
		// NOT asserted here: `\\?\UNC\host\share`. The `\\.\UNC` special case
		// in volumeNameLen keys on the `.` form, so the `?` form falls to the
		// generic `\\?` branch and the volume is `\\?\UNC` rather than the
		// share — which would make `\\?\UNC\host\share` a path UNDER a volume,
		// not a root. That is a claim about stdlib behaviour this file cannot
		// verify (it is not in the stdlib's own test table either), and the
		// whole point of RR-S2X70O is that unverified transcriptions pass green
		// while being wrong. Left out rather than guessed.
		{"extended drive volume", `\\?\C:`, `\\?\C:`, windowsSep, true},
		{"extended drive root", `\\?\C:`, `\\?\C:\`, windowsSep, true},

		// Drive-relative `C:foo` has a volume but is NOT a root: it names a
		// path relative to the drive's working directory. The bare-volume
		// clause must not swallow it.
		{"windows drive relative", "C:", "C:foo", windowsSep, false},

		// A relative base is not a root. The predicate must not over-claim:
		// containedPath rejects this later via Abs/Rel, and a predicate that
		// swallowed it would report the wrong reason.
		{"relative base", "", ".", unixSep, false},

		// The bare-volume clause is guarded on a non-empty volume, so an empty
		// cleaned path is not a root on Unix. Clean never produces one, but the
		// predicate should not lie about an input it can be handed.
		{"empty path no volume", "", "", unixSep, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFilesystemRoot(tt.volume, tt.cleaned, tt.sep); got != tt.want {
				t.Errorf("isFilesystemRoot(%q, %q, %q) = %v, want %v",
					tt.volume, tt.cleaned, string(tt.sep), got, tt.want)
			}
		})
	}
}

// Deliberately absent: a Linux test asserting that containedPath consults
// isFilesystemRoot. Any such test can only reach the predicate through "/",
// where the old `absBase == string(filepath.Separator)` check and the new one
// agree — so it would pass identically before and after this change, which
// makes it evidence of nothing while looking like a wiring guarantee.
// TestContainedPath_RejectsRootBase above already covers the "/" wiring, and
// TestContainedPath_RejectsWindowsRootBases (clone_windows_test.go) covers the
// wiring for the roots that actually distinguish the two implementations.
