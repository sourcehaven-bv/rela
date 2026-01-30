package dataentry

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// --- Test helpers for conflict_test.go (real git repos) ---

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// Use cleanGitEnv to prevent env var leaks from pre-commit hooks.
	cmd.Env = cleanGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed in %s: %v\n%s", strings.Join(args, " "), dir, err, string(out))
	}
}

// --- SyncManager tests using MemoryGitBackend ---

func TestNewSyncManager_NilBackend(t *testing.T) {
	s := NewSyncManager(nil, SyncOptions{})

	if s.State() != SyncDisabled {
		t.Errorf("expected SyncDisabled, got %s", s.State())
	}
	if s.enabled {
		t.Error("expected enabled=false for nil backend")
	}
	status := s.Status()
	if status.Message != "Git not configured" {
		t.Errorf("expected 'Git not configured', got %q", status.Message)
	}
}

func TestNewSyncManager_NoRemote(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", false)
	s := NewSyncManager(mem, SyncOptions{})

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
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
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
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.clean = false // simulate dirty working tree
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

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
	if len(mem.committed) != 1 || mem.committed[0] != "rela: test commit" {
		t.Errorf("expected commit message recorded, got %v", mem.committed)
	}
}

func TestSyncManager_CommitNoChanges(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	// clean=true: no changes to commit
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	err := s.Commit("rela: empty")
	if err != nil {
		t.Fatalf("Commit with no changes should not error: %v", err)
	}
	if s.State() != SyncClean {
		t.Errorf("expected SyncClean after no-op commit, got %s", s.State())
	}
}

func TestSyncManager_CommitDisabled(t *testing.T) {
	s := NewSyncManager(nil, SyncOptions{})

	err := s.Commit("rela: noop")
	if err != nil {
		t.Fatalf("Commit on disabled sync should not error: %v", err)
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

func TestSyncManager_Branches(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.branches = []string{"main", "feature/foo"}
	mem.remoteBranches = []string{"origin/main", "origin/develop"}
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	bl, err := s.Branches()
	if err != nil {
		t.Fatalf("Branches failed: %v", err)
	}
	if bl.Current == "" {
		t.Error("expected non-empty current branch")
	}
	if len(bl.Local) != 2 {
		t.Errorf("expected 2 local branches, got %d", len(bl.Local))
	}
	// "develop" is remote-only, "main" is both → Remote should contain "develop"
	if len(bl.Remote) != 1 || bl.Remote[0] != "develop" {
		t.Errorf("expected remote=[develop], got %v", bl.Remote)
	}
}

func TestSyncManager_CreateBranch(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
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
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.branches = []string{"main", "other"}
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	originalBranch := s.Branch()

	// Switch to other
	err := s.SwitchBranch("other")
	if err != nil {
		t.Fatalf("SwitchBranch failed: %v", err)
	}
	if s.Branch() != "other" {
		t.Errorf("expected branch 'other', got %q", s.Branch())
	}

	// Switch back
	err = s.SwitchBranch(originalBranch)
	if err != nil {
		t.Fatalf("SwitchBranch back failed: %v", err)
	}
	if s.Branch() != originalBranch {
		t.Errorf("expected branch %q, got %q", originalBranch, s.Branch())
	}
}

func TestSyncManager_BranchesDisabled(t *testing.T) {
	s := NewSyncManager(nil, SyncOptions{})

	bl, err := s.Branches()
	if err != nil {
		t.Fatalf("Branches on disabled should not error: %v", err)
	}
	if len(bl.Local) != 0 {
		t.Errorf("expected empty branch list for disabled sync, got %d", len(bl.Local))
	}
}

func TestSyncManager_CommitAsync(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.clean = false
	s := NewSyncManager(mem, SyncOptions{})

	s.CommitAsync("rela: async test")

	// Close flushes pending commits
	s.Close()

	if len(mem.committed) != 1 || mem.committed[0] != "rela: async test" {
		t.Errorf("expected async commit message, got %v", mem.committed)
	}
}

func TestSyncManager_CommitAsyncDebounce(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.clean = false
	s := NewSyncManager(mem, SyncOptions{})

	// Send multiple rapid commits — only the last message should be used
	s.CommitAsync("rela: first")
	s.CommitAsync("rela: second")
	s.CommitAsync("rela: third")

	// Close flushes with the last message
	s.Close()

	// Should have exactly one commit (the last debounced)
	if len(mem.committed) != 1 {
		t.Errorf("expected 1 debounced commit, got %d: %v", len(mem.committed), mem.committed)
	}
	if len(mem.committed) > 0 && mem.committed[0] != "rela: third" {
		t.Errorf("commit message = %q, want %q", mem.committed[0], "rela: third")
	}
}

func TestSyncManager_CommitAsyncDisabled(_ *testing.T) {
	s := NewSyncManager(nil, SyncOptions{})

	// CommitAsync on disabled manager should not panic
	s.CommitAsync("rela: noop")
	s.Close() // should not panic
}

func TestSyncManager_Close(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})

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

// --- Sync tests ---

func TestSyncManager_Push(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.clean = false
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	// Create a local commit
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

	if s.State() != SyncClean {
		t.Errorf("expected SyncClean after push, got %s", s.State())
	}
	if s.Status().Unpushed != 0 {
		t.Errorf("expected 0 unpushed, got %d", s.Status().Unpushed)
	}
}

func TestSyncManager_FastForward(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.behind = 2
	pullCalled := false
	s := NewSyncManager(mem, SyncOptions{
		OnPull: func() { pullCalled = true },
	})
	defer s.Close()

	// Pull should fast-forward
	if err := s.Pull(); err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	if !pullCalled {
		t.Error("expected OnPull callback to be called after fast-forward")
	}
	if !mem.fetched {
		t.Error("expected fetch to be called")
	}
}

func TestSyncManager_RebaseConflict(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.clean = false
	mem.rebaseErr = fmt.Errorf("CONFLICT (content): merge conflict")
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	// Create a local commit
	if err := s.Commit("rela: local change"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Simulate diverged state: ahead=1, behind=1
	mem.behind = 1

	// Push should detect conflict
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
}

func TestSyncManager_ProtectedBranch(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{
		ProtectedBranches: []string{"main"},
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
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{
		ProtectedBranches: []string{"main", "master"},
	})
	defer s.Close()

	// Default branch is "main"
	if !s.isProtected() {
		t.Error("expected branch 'main' to match protection pattern")
	}
}

func TestSyncManager_PushDisabled(t *testing.T) {
	s := NewSyncManager(nil, SyncOptions{})

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
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
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
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
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
	d0 := backoffDuration(0)
	if d0 != fetchInterval {
		t.Errorf("backoffDuration(0) = %v, want %v", d0, fetchInterval)
	}

	d1 := backoffDuration(1)
	if d1 != 60*time.Second {
		t.Errorf("backoffDuration(1) = %v, want %v", d1, 60*time.Second)
	}

	d2 := backoffDuration(2)
	if d2 != 120*time.Second {
		t.Errorf("backoffDuration(2) = %v, want %v", d2, 120*time.Second)
	}

	d3 := backoffDuration(3)
	if d3 != 240*time.Second {
		t.Errorf("backoffDuration(3) = %v, want %v", d3, 240*time.Second)
	}

	d10 := backoffDuration(10)
	if d10 != maxBackoff {
		t.Errorf("backoffDuration(10) = %v, want %v (maxBackoff)", d10, maxBackoff)
	}
}

func TestSyncManager_SetOffline(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

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

	s.mu.RLock()
	failures := s.consecutiveFailures
	s.mu.RUnlock()
	if failures != 1 {
		t.Errorf("expected consecutiveFailures=1, got %d", failures)
	}
}

func TestSyncManager_OfflineStateSticky(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	s.setOffline()
	if s.State() != SyncOffline {
		t.Fatalf("expected SyncOffline, got %s", s.State())
	}

	// updateState should NOT clear offline while consecutiveFailures > 0
	s.updateState()
	if s.State() != SyncOffline {
		t.Errorf("expected SyncOffline to be sticky, got %s", s.State())
	}

	// Clear failures (simulating a successful fetch)
	s.mu.Lock()
	s.consecutiveFailures = 0
	s.mu.Unlock()

	s.updateState()
	if s.State() == SyncOffline {
		t.Errorf("expected state to clear from SyncOffline after failures reset, got %s", s.State())
	}
}

func TestSyncManager_SubscribeNotify(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	id, ch := s.Subscribe()
	defer s.Unsubscribe(id)

	s.setState(SyncAhead, "1 unpushed commit(s)")

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
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	id, ch := s.Subscribe()
	s.Unsubscribe(id)

	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after Unsubscribe")
	}
}

func TestSyncManager_RepoRoot(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test-project", true)
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	if s.RepoRoot() != "/tmp/test-project" {
		t.Errorf("expected RepoRoot=/tmp/test-project, got %q", s.RepoRoot())
	}
}

func TestSyncManager_RepoRootDisabled(t *testing.T) {
	s := NewSyncManager(nil, SyncOptions{})
	if s.RepoRoot() != "" {
		t.Errorf("expected empty RepoRoot for disabled manager, got %q", s.RepoRoot())
	}
}

func TestSyncManager_AbsPath(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/project", true)
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	got := s.AbsPath("entities/ticket/TKT-001.md")
	want := "/tmp/project/entities/ticket/TKT-001.md"
	if got != want {
		t.Errorf("AbsPath = %q, want %q", got, want)
	}
}

func TestSyncManager_Backend(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	if s.Backend() != mem {
		t.Error("expected Backend() to return the injected backend")
	}
}

func TestSyncManager_BackendNil(t *testing.T) {
	s := NewSyncManager(nil, SyncOptions{})
	if s.Backend() != nil {
		t.Error("expected Backend() to return nil for disabled manager")
	}
}

func TestSyncManager_FetchError(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.fetchErr = fmt.Errorf("Could not resolve host: github.com")
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	err := s.Push()
	if err == nil {
		t.Fatal("expected error from Push with fetch failure")
	}
	if s.State() != SyncOffline {
		t.Errorf("expected SyncOffline after network error, got %s", s.State())
	}
}

func TestSyncManager_PushProtectedBranch(t *testing.T) {
	mem := NewMemoryGitBackend("/tmp/test", true)
	mem.clean = false
	mem.pushErr = fmt.Errorf("push rejected")
	mem.pushOutput = "remote: error: GH006 Protected branch"
	s := NewSyncManager(mem, SyncOptions{})
	defer s.Close()

	// Create a commit first
	if err := s.Commit("rela: test"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Push should detect protected branch
	err := s.Push()
	if err == nil {
		t.Fatal("expected error from Push with protected branch")
	}
	if !strings.Contains(err.Error(), "protected branch") {
		t.Errorf("expected protected branch error, got: %v", err)
	}
}
