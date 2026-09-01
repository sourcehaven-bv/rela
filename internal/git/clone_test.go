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
