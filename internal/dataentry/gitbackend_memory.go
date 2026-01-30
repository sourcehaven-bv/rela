package dataentry

import (
	"fmt"
	"strings"
)

// MemoryGitBackend is an in-memory fake GitBackend for testing.
// No disk I/O, no process spawning, no git binary required.
type MemoryGitBackend struct {
	root           string
	branch         string
	branches       []string
	remoteBranches []string
	commits        []memCommit
	hasRemote      bool
	clean          bool
	unpushed       int
	behind         int

	// fetchErr simulates a fetch failure (e.g. network error).
	fetchErr error
	// pushErr simulates a push failure (e.g. protected branch).
	pushErr error
	// pushOutput is returned as combined output from Push.
	pushOutput string
	// rebaseErr simulates a rebase conflict.
	rebaseErr error

	// gitFunc handles arbitrary Git() calls for conflict resolution tests.
	// If nil, Git() returns an error.
	gitFunc func(args ...string) (string, error)

	// Track operations for assertions
	staged    bool
	committed []string // commit messages
	fetched   bool
	pushed    []string // branch names
	rebased   []string // upstream refs
}

type memCommit struct {
	message string
}

// NewMemoryGitBackend creates a MemoryGitBackend with common defaults.
func NewMemoryGitBackend(root string, hasRemote bool) *MemoryGitBackend {
	branch := "main"
	return &MemoryGitBackend{
		root:      root,
		branch:    branch,
		branches:  []string{branch},
		hasRemote: hasRemote,
		clean:     true,
	}
}

func (m *MemoryGitBackend) RepoRoot() string {
	return m.root
}

func (m *MemoryGitBackend) CurrentBranch() (string, error) {
	return m.branch, nil
}

func (m *MemoryGitBackend) HasRemote() bool {
	return m.hasRemote
}

func (m *MemoryGitBackend) IsClean() (bool, error) {
	return m.clean, nil
}

func (m *MemoryGitBackend) RevCount(revRange string) (int, error) {
	// Parse the rev range to determine direction.
	// "origin/main..HEAD" means unpushed (ahead).
	// "HEAD..origin/main" means behind.
	if strings.HasSuffix(revRange, "..HEAD") {
		return m.unpushed, nil
	}
	if strings.HasPrefix(revRange, "HEAD..") {
		return m.behind, nil
	}
	return 0, nil
}

func (m *MemoryGitBackend) StageAll() error {
	m.staged = true
	return nil
}

func (m *MemoryGitBackend) Commit(message string) error {
	m.commits = append(m.commits, memCommit{message: message})
	m.committed = append(m.committed, message)
	m.unpushed++
	m.clean = true
	return nil
}

func (m *MemoryGitBackend) Fetch() error {
	m.fetched = true
	return m.fetchErr
}

func (m *MemoryGitBackend) Push(branch string) (string, error) {
	m.pushed = append(m.pushed, branch)
	if m.pushErr != nil {
		return m.pushOutput, m.pushErr
	}
	m.unpushed = 0
	return "", nil
}

func (m *MemoryGitBackend) Rebase(upstream string) error {
	m.rebased = append(m.rebased, upstream)
	return m.rebaseErr
}

func (m *MemoryGitBackend) AbortRebase() error {
	return nil
}

func (m *MemoryGitBackend) FastForwardMerge(_ string) error {
	m.behind = 0
	return nil
}

func (m *MemoryGitBackend) LogMessages(_ string) ([]string, error) {
	// Return the last N commit messages based on unpushed count
	start := len(m.commits) - m.unpushed
	if start < 0 {
		start = 0
	}
	msgs := make([]string, 0, len(m.commits)-start)
	for i := start; i < len(m.commits); i++ {
		msgs = append(msgs, m.commits[i].message)
	}
	return msgs, nil
}

func (m *MemoryGitBackend) SoftReset(_ string) error {
	// Simulate squash: remove unpushed commits, keep state dirty
	if m.unpushed > 1 {
		m.commits = m.commits[:len(m.commits)-m.unpushed]
		m.unpushed = 0
	}
	return nil
}

func (m *MemoryGitBackend) ListBranches() (local, remote []string, err error) {
	return m.branches, m.remoteBranches, nil
}

func (m *MemoryGitBackend) Checkout(name string) error {
	// Check if branch exists
	for _, b := range m.branches {
		if b == name {
			m.branch = name
			return nil
		}
	}
	return fmt.Errorf("branch %q not found", name)
}

func (m *MemoryGitBackend) CheckoutNewBranch(name string) error {
	m.branches = append(m.branches, name)
	m.branch = name
	return nil
}

func (m *MemoryGitBackend) CheckoutNewBranchFrom(name, _ string) error {
	return m.CheckoutNewBranch(name)
}

func (m *MemoryGitBackend) DeleteBranch(name string) error {
	for i, b := range m.branches {
		if b == name {
			m.branches = append(m.branches[:i], m.branches[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("branch %q not found", name)
}

func (m *MemoryGitBackend) PushNewBranch(name string) error {
	m.pushed = append(m.pushed, name)
	if m.pushErr != nil {
		return m.pushErr
	}
	m.remoteBranches = append(m.remoteBranches, "origin/"+name)
	return nil
}

func (m *MemoryGitBackend) Git(args ...string) (string, error) {
	if m.gitFunc != nil {
		return m.gitFunc(args...)
	}
	return "", fmt.Errorf("Git() not configured in MemoryGitBackend for args: %v", args)
}
