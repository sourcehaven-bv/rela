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

// A path genuinely inside BaseDir must pass containment. It still fails later
// (no network / not a real repo), but not with a containment error.
func TestClone_AllowsPathInsideBaseDir(t *testing.T) {
	base := t.TempDir()

	err := Clone(CloneOptions{
		URL:     "https://github.com/user/repo.git",
		Path:    filepath.Join(base, "repo"),
		BaseDir: base,
	})
	if err != nil && strings.Contains(err.Error(), "escapes base directory") {
		t.Fatalf("Clone rejected a contained path: %v", err)
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

	err := Clone(CloneOptions{
		URL:  "https://github.com/user/repo.git",
		Path: existingPath,
	})
	if err == nil {
		t.Error("expected error when path exists")
	}
}
