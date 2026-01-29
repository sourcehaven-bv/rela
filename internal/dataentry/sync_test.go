package dataentry

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupBareRemote creates a bare git repo to serve as origin.
func setupBareRemote(t *testing.T) string {
	t.Helper()
	bare := t.TempDir()
	runGit(t, bare, "init", "--bare")
	return bare
}

// setupGitRepo creates a temp directory with a git repo, an initial commit,
// and optionally a remote. Returns the repo directory.
func setupGitRepo(t *testing.T, withRemote bool) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	// Create an initial commit
	writeTestFile(t, dir, "init.txt", "initial")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "initial commit")

	if withRemote {
		bare := setupBareRemote(t)
		runGit(t, dir, "remote", "add", "origin", bare)
		runGit(t, dir, "push", "-u", "origin", "HEAD")
	}

	return dir
}

// setupGitRepoWithBare creates a temp directory with a git repo, an initial commit,
// and a bare remote. Returns both the repo directory and the bare remote path.
func setupGitRepoWithBare(t *testing.T) (dir, bare string) {
	t.Helper()
	dir = t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	writeTestFile(t, dir, "init.txt", "initial")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "initial commit")

	bare = setupBareRemote(t)
	runGit(t, dir, "remote", "add", "origin", bare)
	runGit(t, dir, "push", "-u", "origin", "HEAD")

	return dir, bare
}

// cloneRepo clones a bare repo into a new temp directory and configures it.
func cloneRepo(t *testing.T, bare string) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "clone", bare, ".")
	runGit(t, dir, "config", "user.email", "other@example.com")
	runGit(t, dir, "config", "user.name", "Other User")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed in %s: %v\n%s", strings.Join(args, " "), dir, err, string(out))
	}
	return string(out)
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewSyncManager_NoGit(t *testing.T) {
	dir := t.TempDir()
	s := NewSyncManager(dir, SyncOptions{})

	if s.State() != SyncDisabled {
		t.Errorf("expected SyncDisabled, got %s", s.State())
	}
	if s.enabled {
		t.Error("expected enabled=false for non-git dir")
	}
	status := s.Status()
	if status.Message != "Git not configured" {
		t.Errorf("expected 'Git not configured', got %q", status.Message)
	}
}

func TestNewSyncManager_NoRemote(t *testing.T) {
	dir := setupGitRepo(t, false)
	s := NewSyncManager(dir, SyncOptions{})

	if s.State() != SyncDisabled {
		t.Errorf("expected SyncDisabled, got %s", s.State())
	}
	if s.enabled {
		t.Error("expected enabled=false for repo without remote")
	}
	status := s.Status()
	if status.Message != "No remote configured" {
		t.Errorf("expected 'No remote configured', got %q", status.Message)
	}
}

func TestNewSyncManager_WithRemote(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	if s.State() != SyncClean {
		t.Errorf("expected SyncClean, got %s", s.State())
	}
	if !s.enabled {
		t.Error("expected enabled=true")
	}
	if s.Branch() == "" {
		t.Error("expected non-empty branch")
	}
}

func TestSyncManager_Commit(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	// Create a new file
	writeTestFile(t, dir, "new.txt", "new content")

	err := s.Commit("rela: test commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should be AHEAD now
	if s.State() != SyncAhead {
		t.Errorf("expected SyncAhead after commit, got %s", s.State())
	}
	if s.Status().Unpushed != 1 {
		t.Errorf("expected 1 unpushed, got %d", s.Status().Unpushed)
	}
}

func TestSyncManager_CommitNoChanges(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	// Commit with no changes should be a no-op
	err := s.Commit("rela: empty")
	if err != nil {
		t.Fatalf("Commit with no changes should not error: %v", err)
	}
	if s.State() != SyncClean {
		t.Errorf("expected SyncClean after no-op commit, got %s", s.State())
	}
}

func TestSyncManager_CommitDisabled(t *testing.T) {
	dir := t.TempDir()
	s := NewSyncManager(dir, SyncOptions{})

	// Commit on disabled manager should be a no-op
	err := s.Commit("rela: noop")
	if err != nil {
		t.Fatalf("Commit on disabled sync should not error: %v", err)
	}
}

func TestSyncManager_CommitMessage(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	writeTestFile(t, dir, "entity.md", "# Test")
	msg := CommitMessageCreate("TKT-001", "Fix login bug")
	err := s.Commit(msg)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify the commit message
	out := runGit(t, dir, "log", "-1", "--format=%s")
	got := strings.TrimSpace(out)
	expected := `rela: create TKT-001 "Fix login bug"`
	if got != expected {
		t.Errorf("commit message = %q, want %q", got, expected)
	}
}

func TestSyncManager_Branches(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	bl, err := s.Branches()
	if err != nil {
		t.Fatalf("Branches failed: %v", err)
	}
	if bl.Current == "" {
		t.Error("expected non-empty current branch")
	}
	if len(bl.Local) == 0 {
		t.Error("expected at least one local branch")
	}
}

func TestSyncManager_CreateBranch(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	err := s.CreateBranch("feature/test")
	if err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if s.Branch() != "feature/test" {
		t.Errorf("expected branch 'feature/test', got %q", s.Branch())
	}
}

func TestSyncManager_SwitchBranch(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	originalBranch := s.Branch()

	// Create a new branch
	err := s.CreateBranch("other")
	if err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if s.Branch() != "other" {
		t.Errorf("expected branch 'other', got %q", s.Branch())
	}

	// Switch back
	err = s.SwitchBranch(originalBranch)
	if err != nil {
		t.Fatalf("SwitchBranch failed: %v", err)
	}
	if s.Branch() != originalBranch {
		t.Errorf("expected branch %q, got %q", originalBranch, s.Branch())
	}
}

func TestSyncManager_BranchesDisabled(t *testing.T) {
	dir := t.TempDir()
	s := NewSyncManager(dir, SyncOptions{})

	bl, err := s.Branches()
	if err != nil {
		t.Fatalf("Branches on disabled should not error: %v", err)
	}
	if len(bl.Local) != 0 {
		t.Errorf("expected empty branch list for disabled sync, got %d", len(bl.Local))
	}
}

func TestSyncManager_CommitAsync(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})

	writeTestFile(t, dir, "async.txt", "async content")
	s.CommitAsync("rela: async test")

	// Close flushes pending commits
	s.Close()

	// Verify the commit happened
	out := runGit(t, dir, "log", "-1", "--format=%s")
	got := strings.TrimSpace(out)
	if got != "rela: async test" {
		t.Errorf("commit message = %q, want %q", got, "rela: async test")
	}
}

func TestSyncManager_CommitAsyncDebounce(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})

	// Send multiple rapid commits — only the last message should be used
	writeTestFile(t, dir, "f1.txt", "one")
	s.CommitAsync("rela: first")
	writeTestFile(t, dir, "f2.txt", "two")
	s.CommitAsync("rela: second")
	writeTestFile(t, dir, "f3.txt", "three")
	s.CommitAsync("rela: third")

	// Close flushes with the last message
	s.Close()

	// Should have exactly one commit after the initial
	out := runGit(t, dir, "rev-list", "--count", "HEAD")
	count := strings.TrimSpace(out)
	if count != "2" { // 1 initial + 1 debounced
		t.Errorf("expected 2 commits total, got %s", count)
	}

	// The commit message should be the last one
	out = runGit(t, dir, "log", "-1", "--format=%s")
	got := strings.TrimSpace(out)
	if got != "rela: third" {
		t.Errorf("commit message = %q, want %q", got, "rela: third")
	}

	// All files should be committed
	status := runGit(t, dir, "status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Errorf("expected clean working tree, got: %s", status)
	}
}

func TestSyncManager_CommitAsyncDisabled(t *testing.T) {
	dir := t.TempDir()
	s := NewSyncManager(dir, SyncOptions{})

	// CommitAsync on disabled manager should not panic
	s.CommitAsync("rela: noop")
	s.Close() // should not panic
}

func TestSyncManager_Close(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})

	// Close without pending commits should not block
	done := make(chan struct{})
	go func() {
		s.Close()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked for too long")
	}
}

func TestCommitMessages(t *testing.T) {
	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{
			name: "create with title",
			fn:   func() string { return CommitMessageCreate("TKT-001", "Fix the bug") },
			want: `rela: create TKT-001 "Fix the bug"`,
		},
		{
			name: "create without title",
			fn:   func() string { return CommitMessageCreate("TKT-002", "") },
			want: "rela: create TKT-002",
		},
		{
			name: "update with fields",
			fn:   func() string { return CommitMessageUpdate("TKT-003", []string{"status", "priority"}) },
			want: "rela: update TKT-003 (status, priority)",
		},
		{
			name: "update without fields",
			fn:   func() string { return CommitMessageUpdate("TKT-004", nil) },
			want: "rela: update TKT-004",
		},
		{
			name: "delete",
			fn:   func() string { return CommitMessageDelete("TKT-005") },
			want: "rela: delete TKT-005",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Phase 3 tests ---

func TestSyncManager_SyncNow_PushOnly(t *testing.T) {
	dir, _ := setupGitRepoWithBare(t)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	// Create a local commit
	writeTestFile(t, dir, "push-test.txt", "push me")
	if err := s.Commit("rela: create push-test"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if s.State() != SyncAhead {
		t.Fatalf("expected SyncAhead, got %s", s.State())
	}

	// Push via the public API
	if err := s.Push(); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Should be clean after push
	if s.State() != SyncClean {
		t.Errorf("expected SyncClean after push, got %s", s.State())
	}
	if s.Status().Unpushed != 0 {
		t.Errorf("expected 0 unpushed, got %d", s.Status().Unpushed)
	}
}

func TestSyncManager_SyncNow_FastForward(t *testing.T) {
	dir, bare := setupGitRepoWithBare(t)

	pullCalled := false
	s := NewSyncManager(dir, SyncOptions{
		OnPull: func() { pullCalled = true },
	})
	defer s.Close()

	// Simulate remote changes by cloning and pushing from another repo
	other := cloneRepo(t, bare)
	writeTestFile(t, other, "remote-change.txt", "remote content")
	runGit(t, other, "add", "-A")
	runGit(t, other, "commit", "-m", "rela: remote change")
	runGit(t, other, "push", "origin", "HEAD")

	// Sync should fast-forward
	if err := s.Push(); err != nil {
		t.Fatalf("Push (fast-forward) failed: %v", err)
	}

	if s.State() != SyncClean {
		t.Errorf("expected SyncClean after fast-forward, got %s", s.State())
	}
	if !pullCalled {
		t.Error("expected OnPull callback to be called after fast-forward")
	}

	// Verify the remote file exists locally
	if _, err := os.Stat(filepath.Join(dir, "remote-change.txt")); err != nil {
		t.Errorf("expected remote-change.txt to exist after fast-forward: %v", err)
	}
}

func TestSyncManager_SyncNow_RebaseAndPush(t *testing.T) {
	dir, bare := setupGitRepoWithBare(t)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	// Create a local commit (different file)
	writeTestFile(t, dir, "local.txt", "local content")
	if err := s.Commit("rela: create local"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Create a remote commit (different file to avoid conflicts)
	other := cloneRepo(t, bare)
	writeTestFile(t, other, "remote.txt", "remote content")
	runGit(t, other, "add", "-A")
	runGit(t, other, "commit", "-m", "rela: remote change")
	runGit(t, other, "push", "origin", "HEAD")

	// Sync should rebase + push
	if err := s.Push(); err != nil {
		t.Fatalf("Push (rebase+push) failed: %v", err)
	}

	if s.State() != SyncClean {
		t.Errorf("expected SyncClean after rebase+push, got %s", s.State())
	}

	// Both files should be present
	if _, err := os.Stat(filepath.Join(dir, "local.txt")); err != nil {
		t.Error("expected local.txt to exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "remote.txt")); err != nil {
		t.Error("expected remote.txt to exist")
	}
}

func TestSyncManager_SyncNow_RebaseConflict(t *testing.T) {
	dir, bare := setupGitRepoWithBare(t)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	// Create a local commit modifying the same file
	writeTestFile(t, dir, "init.txt", "local version")
	if err := s.Commit("rela: update init locally"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Create a conflicting remote commit
	other := cloneRepo(t, bare)
	writeTestFile(t, other, "init.txt", "remote version")
	runGit(t, other, "add", "-A")
	runGit(t, other, "commit", "-m", "rela: update init remotely")
	runGit(t, other, "push", "origin", "HEAD")

	// Sync should detect conflict
	err := s.Push()
	if err == nil {
		t.Fatal("expected Push to fail on conflict")
	}
	if !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected conflict error, got: %v", err)
	}

	if s.State() != SyncConflict {
		t.Errorf("expected SyncConflict, got %s", s.State())
	}
	if s.Status().ErrorMsg == "" {
		t.Error("expected non-empty error message for conflict")
	}
}

func TestSyncManager_SquashCommits(t *testing.T) {
	dir, _ := setupGitRepoWithBare(t)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	// Create multiple local commits
	writeTestFile(t, dir, "a.txt", "a")
	if err := s.Commit("rela: create a"); err != nil {
		t.Fatalf("Commit 1 failed: %v", err)
	}
	writeTestFile(t, dir, "b.txt", "b")
	if err := s.Commit("rela: create b"); err != nil {
		t.Fatalf("Commit 2 failed: %v", err)
	}
	writeTestFile(t, dir, "c.txt", "c")
	if err := s.Commit("rela: create c"); err != nil {
		t.Fatalf("Commit 3 failed: %v", err)
	}

	if s.Status().Unpushed != 3 {
		t.Fatalf("expected 3 unpushed, got %d", s.Status().Unpushed)
	}

	// Push should squash the 3 commits into 1
	if err := s.Push(); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// After push, there should be 2 total commits (initial + squashed)
	out := runGit(t, dir, "rev-list", "--count", "HEAD")
	count := strings.TrimSpace(out)
	if count != "2" {
		t.Errorf("expected 2 commits total after squash, got %s", count)
	}

	// Verify the squashed commit message
	out = runGit(t, dir, "log", "-1", "--format=%s")
	got := strings.TrimSpace(out)
	if !strings.Contains(got, "3 changes") {
		t.Errorf("expected squash message with '3 changes', got %q", got)
	}
}

func TestSyncManager_ProtectedBranch(t *testing.T) {
	dir := setupGitRepo(t, true)
	branch := strings.TrimSpace(runGit(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))

	s := NewSyncManager(dir, SyncOptions{
		ProtectedBranches: []string{branch},
	})
	defer s.Close()

	if !s.isProtected() {
		t.Error("expected branch to be detected as protected")
	}

	status := s.Status()
	if !status.Protected {
		t.Error("expected Protected=true in status")
	}
}

func TestSyncManager_ProtectedBranchGlob(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{
		ProtectedBranches: []string{"main", "master"},
	})
	defer s.Close()

	// The default branch name may be main or master depending on git config
	branch := s.Branch()
	if branch == "main" || branch == "master" {
		if !s.isProtected() {
			t.Errorf("expected branch %q to match protection pattern", branch)
		}
	}
}

func TestSyncManager_PushDisabled(t *testing.T) {
	dir := t.TempDir()
	s := NewSyncManager(dir, SyncOptions{})

	// Push on disabled manager should be a no-op
	err := s.Push()
	if err != nil {
		t.Errorf("expected Push on disabled manager to return nil, got: %v", err)
	}
}

func TestIsProtectedPushError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   bool
	}{
		{"github GH006", "remote: error: GH006 Protected branch", true},
		{"generic protected", "remote: error: protected branch update denied", true},
		{"remote rejected", "error: remote rejected, pre-receive hook declined", true},
		{"pre-receive hook", "remote: error: pre-receive hook declined", true},
		{"required status", "remote: error: required status check is expected", true},
		{"changes must be made", "remote: changes must be made through a pull request", true},
		{"normal error", "error: failed to push some refs", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isProtectedPushError(tt.stderr)
			if got != tt.want {
				t.Errorf("isProtectedPushError(%q) = %v, want %v", tt.stderr, got, tt.want)
			}
		})
	}
}

func TestSyncManager_SetOnPull(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	called := false
	s.SetOnPull(func() { called = true })

	s.mu.RLock()
	hasCb := s.onPull != nil
	s.mu.RUnlock()

	if !hasCb {
		t.Error("expected onPull to be set after SetOnPull")
	}
	_ = called // just verifying callback was stored
}

func TestSyncManager_StatusFields(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	status := s.Status()
	if !status.Enabled {
		t.Error("expected Enabled=true")
	}
	if status.Branch == "" {
		t.Error("expected non-empty Branch")
	}
	if status.State != SyncClean {
		t.Errorf("expected State=clean, got %s", status.State)
	}
	if status.Protected {
		t.Error("expected Protected=false with no protection patterns")
	}
}

// --- Phase 5C tests ---

func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{"DNS failure", "fatal: unable to access 'https://github.com/...': Could not resolve host: github.com", true},
		{"connection refused", "fatal: unable to access '...': Failed to connect to ... port 443: Connection refused", true},
		{"connection timed out", "fatal: unable to access '...': Connection timed out", true},
		{"network unreachable", "fatal: unable to access '...': Network is unreachable", true},
		{"no route", "fatal: unable to access '...': No route to host", true},
		{"SSL error", "fatal: unable to access '...': SSL certificate problem", true},
		{"unable to access", "fatal: unable to access 'https://example.com/repo.git/'", true},
		{"failed to connect", "fatal: Failed to connect to github.com", true},
		{"timed out generic", "error: RPC failed; result=7, HTTP code = 0\nfatal: operation timed out", true},
		{"rebase conflict", "error: could not apply abc123... commit message", false},
		{"auth failure", "remote: Invalid username or password", false},
		{"normal push error", "error: failed to push some refs", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNetworkError(tt.errMsg)
			if got != tt.want {
				t.Errorf("isNetworkError(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestBackoffDuration(t *testing.T) {
	// 0 failures -> base interval (30s)
	d0 := backoffDuration(0)
	if d0 != fetchInterval {
		t.Errorf("backoffDuration(0) = %v, want %v", d0, fetchInterval)
	}

	// 1 failure -> 60s
	d1 := backoffDuration(1)
	if d1 != 60*time.Second {
		t.Errorf("backoffDuration(1) = %v, want %v", d1, 60*time.Second)
	}

	// 2 failures -> 120s
	d2 := backoffDuration(2)
	if d2 != 120*time.Second {
		t.Errorf("backoffDuration(2) = %v, want %v", d2, 120*time.Second)
	}

	// 3 failures -> 240s
	d3 := backoffDuration(3)
	if d3 != 240*time.Second {
		t.Errorf("backoffDuration(3) = %v, want %v", d3, 240*time.Second)
	}

	// Large number should be capped at maxBackoff (5m)
	d10 := backoffDuration(10)
	if d10 != maxBackoff {
		t.Errorf("backoffDuration(10) = %v, want %v (maxBackoff)", d10, maxBackoff)
	}
}

func TestSyncManager_SetOffline(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	// Manually trigger offline state
	s.setOffline()

	if s.State() != SyncOffline {
		t.Errorf("expected SyncOffline, got %s", s.State())
	}
	status := s.Status()
	if status.Message != "Offline" {
		t.Errorf("expected message 'Offline', got %q", status.Message)
	}
	if status.ErrorMsg != "Remote unreachable" {
		t.Errorf("expected error 'Remote unreachable', got %q", status.ErrorMsg)
	}

	// consecutiveFailures should be 1
	s.mu.RLock()
	failures := s.consecutiveFailures
	s.mu.RUnlock()
	if failures != 1 {
		t.Errorf("expected consecutiveFailures=1, got %d", failures)
	}
}

func TestSyncManager_OfflineStateSticky(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	// Set offline
	s.setOffline()
	if s.State() != SyncOffline {
		t.Fatalf("expected SyncOffline, got %s", s.State())
	}

	// updateState should NOT clear offline while consecutiveFailures > 0
	s.updateState()
	if s.State() != SyncOffline {
		t.Errorf("expected SyncOffline to be sticky, got %s", s.State())
	}

	// Now clear failures (simulating a successful fetch)
	s.mu.Lock()
	s.consecutiveFailures = 0
	s.mu.Unlock()

	// updateState should now transition away from offline
	s.updateState()
	if s.State() == SyncOffline {
		t.Errorf("expected state to clear from SyncOffline after failures reset, got %s", s.State())
	}
}

func TestSyncManager_SubscribeNotify(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	id, ch := s.Subscribe()
	defer s.Unsubscribe(id)

	// Trigger a state change
	s.setState(SyncAhead, "1 unpushed commit(s)")

	// Should receive notification
	select {
	case status := <-ch:
		if status.State != SyncAhead {
			t.Errorf("expected SyncAhead in notification, got %s", status.State)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber notification")
	}
}

func TestSyncManager_UnsubscribeClosesChannel(t *testing.T) {
	dir := setupGitRepo(t, true)
	s := NewSyncManager(dir, SyncOptions{})
	defer s.Close()

	id, ch := s.Subscribe()
	s.Unsubscribe(id)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after Unsubscribe")
	}
}
