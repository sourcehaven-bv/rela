package git

import (
	"os"
	"path/filepath"
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

func TestInjectAuth(t *testing.T) {
	tests := []struct {
		url      string
		username string
		token    string
		expected string
		wantErr  bool
	}{
		{
			"https://github.com/user/repo.git",
			"oauth2",
			"token123",
			"https://oauth2:token123@github.com/user/repo.git",
			false,
		},
		{
			"https://github.com/user/repo.git",
			"", // empty username defaults to oauth2
			"token123",
			"https://oauth2:token123@github.com/user/repo.git",
			false,
		},
		{
			"http://github.com/user/repo.git",
			"oauth2",
			"token123",
			"",
			true, // HTTP not allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got, err := injectAuth(tt.url, tt.username, tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("injectAuth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("injectAuth() = %q, want %q", got, tt.expected)
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
