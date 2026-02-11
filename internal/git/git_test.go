package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
// Uses t.TempDir() for automatic cleanup.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Initialize git repo
	if err := runCmd(dir, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for commits
	if err := runCmd(dir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}
	if err := runCmd(dir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	// Create entities directory
	entitiesDir := filepath.Join(dir, "entities", "tickets")
	if err := os.MkdirAll(entitiesDir, 0o755); err != nil {
		t.Fatalf("mkdir entities failed: %v", err)
	}

	// Create initial file and commit
	testFile := filepath.Join(entitiesDir, "TKT-001.md")
	content := "---\nid: TKT-001\ntype: ticket\n---\nTest ticket\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	if err := runCmd(dir, "add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := runCmd(dir, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	return dir
}

// setupTestRepoWithRemote creates a test repo with a remote.
// Uses t.TempDir() for automatic cleanup.
//
//nolint:unparam // remoteDir is returned for potential future use
func setupTestRepoWithRemote(t *testing.T) (localDir, remoteDir string) {
	t.Helper()

	// Create "remote" (bare repo)
	remoteDir = t.TempDir()
	if err := runCmd(remoteDir, "init", "--bare"); err != nil {
		t.Fatalf("git init bare failed: %v", err)
	}

	// Create local repo
	localDir = setupTestRepo(t)

	// Add remote
	if err := runCmd(localDir, "remote", "add", "origin", remoteDir); err != nil {
		t.Fatalf("git remote add failed: %v", err)
	}

	// Push to remote
	if err := runCmd(localDir, "push", "-u", "origin", "master"); err != nil {
		// Try main branch
		if err := runCmd(localDir, "branch", "-M", "main"); err != nil {
			t.Fatalf("git branch rename failed: %v", err)
		}
		if err := runCmd(localDir, "push", "-u", "origin", "main"); err != nil {
			t.Fatalf("git push failed: %v", err)
		}
	}

	return localDir, remoteDir
}

func runCmd(dir string, args ...string) error {
	if dir == "" {
		panic("runCmd: dir is empty")
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		panic("runCmd: dir does not exist: " + dir)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// Explicitly set GIT_DIR to prevent git from using parent repos
	cmd.Env = append(os.Environ(), "GIT_DIR="+filepath.Join(dir, ".git"))
	return cmd.Run()
}

func TestIsRepo_NotARepo(t *testing.T) {
	dir := t.TempDir()

	if IsRepo(dir) {
		t.Error("expected IsRepo to return false for non-repo directory")
	}
}

func TestIsRepo_NoRemote(t *testing.T) {
	dir := setupTestRepo(t)

	// Repo without remote should return false
	if IsRepo(dir) {
		t.Error("expected IsRepo to return false for repo without remote")
	}
}

func TestIsRepo_WithRemote(t *testing.T) {
	dir, _ := setupTestRepoWithRemote(t)

	if !IsRepo(dir) {
		t.Error("expected IsRepo to return true for repo with remote")
	}
}

func TestGetStatus_NoRemote(t *testing.T) {
	dir := setupTestRepo(t)

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
	dir, _ := setupTestRepoWithRemote(t)

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
	dir, _ := setupTestRepoWithRemote(t)

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
	dir, _ := setupTestRepoWithRemote(t)

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
	dir, _ := setupTestRepoWithRemote(t)

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
	dir, _ := setupTestRepoWithRemote(t)

	ops := NewOps(dir, Config{})
	// AbortRebase when no rebase is in progress should fail
	err := ops.AbortRebase()
	if err == nil {
		t.Error("expected error when aborting non-existent rebase")
	}
}

func TestSync_RejectsWhenConflictInProgress(t *testing.T) {
	dir, _ := setupTestRepoWithRemote(t)

	// Simulate a rebase conflict by creating the rebase-merge directory
	// (this is what git creates during a rebase conflict)
	rebaseMergeDir := filepath.Join(dir, ".git", "rebase-merge")
	if err := os.MkdirAll(rebaseMergeDir, 0o755); err != nil {
		t.Fatalf("create rebase-merge dir failed: %v", err)
	}

	ops := NewOps(dir, Config{})
	err := ops.Sync("should fail")
	if err == nil {
		t.Error("expected Sync to fail when conflict in progress")
	}
	if !errors.Is(err, ErrConflictInProgress) {
		t.Errorf("expected ErrConflictInProgress, got: %v", err)
	}
}

func TestSync_RejectsWhenRebaseApplyInProgress(t *testing.T) {
	dir, _ := setupTestRepoWithRemote(t)

	// Simulate a rebase by creating the rebase-apply directory
	rebaseApplyDir := filepath.Join(dir, ".git", "rebase-apply")
	if err := os.MkdirAll(rebaseApplyDir, 0o755); err != nil {
		t.Fatalf("create rebase-apply dir failed: %v", err)
	}

	ops := NewOps(dir, Config{})
	err := ops.Sync("should fail")
	if err == nil {
		t.Error("expected Sync to fail when rebase-apply in progress")
	}
	if !errors.Is(err, ErrConflictInProgress) {
		t.Errorf("expected ErrConflictInProgress, got: %v", err)
	}
}
