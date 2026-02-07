package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	// Initialize git repo
	if err := runCmd(dir, "git", "init"); err != nil {
		cleanup()
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for commits
	if err := runCmd(dir, "git", "config", "user.email", "test@test.com"); err != nil {
		cleanup()
		t.Fatalf("git config email failed: %v", err)
	}
	if err := runCmd(dir, "git", "config", "user.name", "Test User"); err != nil {
		cleanup()
		t.Fatalf("git config name failed: %v", err)
	}

	// Create entities directory
	entitiesDir := filepath.Join(dir, "entities", "tickets")
	if err := os.MkdirAll(entitiesDir, 0o755); err != nil {
		cleanup()
		t.Fatalf("mkdir entities failed: %v", err)
	}

	// Create initial file and commit
	testFile := filepath.Join(entitiesDir, "TKT-001.md")
	content := "---\nid: TKT-001\ntype: ticket\n---\nTest ticket\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		cleanup()
		t.Fatalf("write file failed: %v", err)
	}

	if err := runCmd(dir, "git", "add", "."); err != nil {
		cleanup()
		t.Fatalf("git add failed: %v", err)
	}
	if err := runCmd(dir, "git", "commit", "-m", "Initial commit"); err != nil {
		cleanup()
		t.Fatalf("git commit failed: %v", err)
	}

	return dir, cleanup
}

// setupTestRepoWithRemote creates a test repo with a remote.
func setupTestRepoWithRemote(t *testing.T) (string, string, func()) {
	t.Helper()

	// Create "remote" (bare repo)
	remoteDir, err := os.MkdirTemp("", "git-remote-*")
	if err != nil {
		t.Fatalf("failed to create remote dir: %v", err)
	}
	if err := runCmd(remoteDir, "git", "init", "--bare"); err != nil {
		os.RemoveAll(remoteDir)
		t.Fatalf("git init bare failed: %v", err)
	}

	// Create local repo
	localDir, cleanupLocal := setupTestRepo(t)

	cleanup := func() {
		cleanupLocal()
		os.RemoveAll(remoteDir)
	}

	// Add remote
	if err := runCmd(localDir, "git", "remote", "add", "origin", remoteDir); err != nil {
		cleanup()
		t.Fatalf("git remote add failed: %v", err)
	}

	// Push to remote
	if err := runCmd(localDir, "git", "push", "-u", "origin", "master"); err != nil {
		// Try main branch
		if err := runCmd(localDir, "git", "branch", "-M", "main"); err != nil {
			cleanup()
			t.Fatalf("git branch rename failed: %v", err)
		}
		if err := runCmd(localDir, "git", "push", "-u", "origin", "main"); err != nil {
			cleanup()
			t.Fatalf("git push failed: %v", err)
		}
	}

	return localDir, remoteDir, cleanup
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func TestIsRepo_NotARepo(t *testing.T) {
	dir, err := os.MkdirTemp("", "not-a-repo-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	if IsRepo(dir) {
		t.Error("expected IsRepo to return false for non-repo directory")
	}
}

func TestIsRepo_NoRemote(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Repo without remote should return false
	if IsRepo(dir) {
		t.Error("expected IsRepo to return false for repo without remote")
	}
}

func TestIsRepo_WithRemote(t *testing.T) {
	dir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	if !IsRepo(dir) {
		t.Error("expected IsRepo to return true for repo with remote")
	}
}

func TestGetStatus_NoRemote(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	ops := NewOps(dir, Config{})
	status, err := ops.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Available {
		t.Error("expected Available to be false for repo without remote")
	}
}

func TestGetStatus_WithRemote(t *testing.T) {
	dir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	ops := NewOps(dir, Config{})
	status, err := ops.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !status.Available {
		t.Error("expected Available to be true")
	}
	if status.LocalChanges != 0 {
		t.Errorf("expected LocalChanges=0, got %d", status.LocalChanges)
	}
	if status.Conflict {
		t.Error("expected Conflict to be false")
	}
}

func TestGetStatus_LocalChanges(t *testing.T) {
	dir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	// Modify a file in entities/
	testFile := filepath.Join(dir, "entities", "tickets", "TKT-001.md")
	content := "---\nid: TKT-001\ntype: ticket\nstatus: open\n---\nUpdated\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	ops := NewOps(dir, Config{})
	status, err := ops.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.LocalChanges != 1 {
		t.Errorf("expected LocalChanges=1, got %d", status.LocalChanges)
	}
}

func TestGetStatus_IgnoresNonEntityChanges(t *testing.T) {
	dir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	// Create a file outside entities/
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	ops := NewOps(dir, Config{})
	status, err := ops.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	// Should not count README.md as a local change
	if status.LocalChanges != 0 {
		t.Errorf("expected LocalChanges=0 (ignoring non-entity files), got %d", status.LocalChanges)
	}
}

func TestFetch(t *testing.T) {
	dir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	ops := NewOps(dir, Config{})
	err := ops.Fetch()
	if err != nil {
		t.Errorf("Fetch failed: %v", err)
	}
}

func TestNewOps(t *testing.T) {
	cfg := Config{
		Enabled:       true,
		Mode:          "direct",
		Branch:        "main",
		FetchInterval: 30,
	}
	ops := NewOps("/tmp", cfg)
	if ops.root != "/tmp" {
		t.Errorf("expected root=/tmp, got %s", ops.root)
	}
	if ops.config.Mode != "direct" {
		t.Errorf("expected mode=direct, got %s", ops.config.Mode)
	}
}

func TestGetBaseBranch(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name:     "default",
			config:   Config{},
			expected: "main",
		},
		{
			name:     "direct mode with branch",
			config:   Config{Mode: "direct", Branch: "develop"},
			expected: "develop",
		},
		{
			name:     "pr mode with base branch",
			config:   Config{Mode: "pr", BaseBranch: "main"},
			expected: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := NewOps("/tmp", tt.config)
			got := ops.getBaseBranch()
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestAbortRebase_NoRebaseInProgress(t *testing.T) {
	dir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	ops := NewOps(dir, Config{})
	// AbortRebase when no rebase is in progress should fail
	err := ops.AbortRebase()
	if err == nil {
		t.Error("expected error when aborting non-existent rebase")
	}
}
