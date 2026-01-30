package dataentry

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GitBackend abstracts git operations for the SyncManager.
type GitBackend interface {
	// RepoRoot returns the git repository root directory.
	RepoRoot() string

	// CurrentBranch returns the current branch name.
	CurrentBranch() (string, error)

	// HasRemote returns true if the repo has at least one remote configured.
	HasRemote() bool

	// IsClean returns true if there are no staged or unstaged changes.
	IsClean() (bool, error)

	// RevCount counts revisions in a rev-list range (e.g. "origin/main..HEAD").
	RevCount(revRange string) (int, error)

	// StageAll stages all changes (git add -A).
	StageAll() error

	// Commit creates a commit with the given message.
	Commit(message string) error

	// Fetch fetches from origin.
	Fetch() error

	// Push pushes the given branch to origin. Returns the combined output and error.
	Push(branch string) (string, error)

	// Rebase rebases onto the given upstream ref.
	Rebase(upstream string) error

	// AbortRebase aborts an in-progress rebase.
	AbortRebase() error

	// FastForwardMerge performs a fast-forward-only merge of the given upstream ref.
	FastForwardMerge(upstream string) error

	// LogMessages returns commit subject lines in the given rev range.
	LogMessages(revRange string) ([]string, error)

	// SoftReset performs a soft reset to the given ref.
	SoftReset(ref string) error

	// ListBranches returns local and remote branch names.
	ListBranches() (local, remote []string, err error)

	// Checkout checks out an existing branch.
	Checkout(name string) error

	// CheckoutNewBranch creates and checks out a new branch from HEAD.
	CheckoutNewBranch(name string) error

	// CheckoutNewBranchFrom creates and checks out a new branch from a specific ref.
	CheckoutNewBranchFrom(name, from string) error

	// DeleteBranch deletes a local branch.
	DeleteBranch(name string) error

	// PushNewBranch pushes a branch with upstream tracking (-u).
	PushNewBranch(name string) error

	// Git runs an arbitrary git command and returns its stdout.
	// This is an escape hatch for one-off commands not covered by the interface
	// (e.g. conflict resolution helpers like git show, git diff, git merge-base).
	Git(args ...string) (string, error)
}

// ExecGitBackend implements GitBackend by shelling out to the git binary.
// coverage-ignore: all methods shell out to git and require a real repository
type ExecGitBackend struct {
	repoRoot string
}

// NewGitBackend creates an ExecGitBackend for the given project root.
// It verifies git is available, discovers the repo root, and checks that
// the project root matches the repo root (to prevent operating on a
// parent repository when the project is a subdirectory).
// coverage-ignore: requires git binary and real repository
func NewGitBackend(projectRoot string) (*ExecGitBackend, error) {
	// Find git repo root (supports worktrees)
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = projectRoot
	cmd.Env = cleanGitEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git not available in %s: %w", projectRoot, err)
	}
	repoRoot := strings.TrimSpace(string(out))

	// Verify the git repo root is the project root. If the project is a
	// subdirectory of a larger repo, sync would operate on the entire repo
	// (git add -A, branch switches, etc.) which is destructive.
	absProject, _ := filepath.Abs(projectRoot)
	absProject, _ = filepath.EvalSymlinks(absProject)
	resolvedRoot, _ := filepath.EvalSymlinks(repoRoot)
	if absProject != resolvedRoot {
		return nil, fmt.Errorf("project root %s differs from git root %s", absProject, resolvedRoot)
	}

	return &ExecGitBackend{repoRoot: repoRoot}, nil
}

func (g *ExecGitBackend) RepoRoot() string {
	return g.repoRoot
}

func (g *ExecGitBackend) CurrentBranch() (string, error) {
	out, err := g.git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (g *ExecGitBackend) HasRemote() bool {
	out, err := g.git("remote")
	return err == nil && strings.TrimSpace(out) != ""
}

func (g *ExecGitBackend) IsClean() (bool, error) {
	out, err := g.git("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

func (g *ExecGitBackend) RevCount(revRange string) (int, error) {
	out, err := g.git("rev-list", "--count", revRange)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("parsing rev count: %w", err)
	}
	return n, nil
}

func (g *ExecGitBackend) StageAll() error {
	_, err := g.git("add", "-A")
	return err
}

func (g *ExecGitBackend) Commit(message string) error {
	_, err := g.git("commit", "-m", message)
	return err
}

func (g *ExecGitBackend) Fetch() error {
	_, err := g.git("fetch", "origin")
	return err
}

func (g *ExecGitBackend) Push(branch string) (string, error) {
	cmd := exec.Command("git", "push", "origin", branch)
	cmd.Dir = g.repoRoot
	cmd.Env = cleanGitEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (g *ExecGitBackend) Rebase(upstream string) error {
	_, err := g.git("rebase", upstream)
	return err
}

func (g *ExecGitBackend) AbortRebase() error {
	_, err := g.git("rebase", "--abort")
	return err
}

func (g *ExecGitBackend) FastForwardMerge(upstream string) error {
	_, err := g.git("merge", "--ff-only", upstream)
	return err
}

func (g *ExecGitBackend) LogMessages(revRange string) ([]string, error) {
	out, err := g.git("log", "--format=%s", revRange)
	if err != nil {
		return nil, err
	}
	var messages []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			messages = append(messages, l)
		}
	}
	return messages, nil
}

func (g *ExecGitBackend) SoftReset(ref string) error {
	_, err := g.git("reset", "--soft", ref)
	return err
}

func (g *ExecGitBackend) ListBranches() (local, remote []string, err error) {
	out, err := g.git("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, nil, fmt.Errorf("listing local branches: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if b := strings.TrimSpace(line); b != "" {
			local = append(local, b)
		}
	}

	out, err = g.git("branch", "-r", "--format=%(refname:short)")
	if err != nil {
		return local, nil, nil //nolint:nilerr // remote branches are optional
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		b := strings.TrimSpace(line)
		if b == "" || strings.HasSuffix(b, "/HEAD") {
			continue
		}
		remote = append(remote, b)
	}

	return local, remote, nil
}

func (g *ExecGitBackend) Checkout(name string) error {
	_, err := g.git("checkout", name)
	return err
}

func (g *ExecGitBackend) CheckoutNewBranch(name string) error {
	_, err := g.git("checkout", "-b", name)
	return err
}

func (g *ExecGitBackend) CheckoutNewBranchFrom(name, from string) error {
	_, err := g.git("checkout", "-b", name, from)
	return err
}

func (g *ExecGitBackend) DeleteBranch(name string) error {
	_, err := g.git("branch", "-D", name)
	return err
}

func (g *ExecGitBackend) PushNewBranch(name string) error {
	_, err := g.git("push", "-u", "origin", name)
	return err
}

func (g *ExecGitBackend) Git(args ...string) (string, error) {
	return g.git(args...)
}

// git executes a git command in the repo root and returns its stdout.
// It clears GIT_DIR, GIT_WORK_TREE, and GIT_INDEX_FILE from the
// environment so that child git processes always discover the repo from
// cmd.Dir rather than inheriting potentially stale values (e.g. from a
// pre-commit hook context).
func (g *ExecGitBackend) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoRoot
	cmd.Env = cleanGitEnv()
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%w: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return string(out), nil
}

// cleanGitEnv returns os.Environ() without GIT_DIR, GIT_WORK_TREE, and
// GIT_INDEX_FILE so child git processes discover the repo from cmd.Dir.
func cleanGitEnv() []string {
	environ := os.Environ()
	env := make([]string, 0, len(environ))
	for _, e := range environ {
		switch {
		case strings.HasPrefix(e, "GIT_DIR="):
		case strings.HasPrefix(e, "GIT_WORK_TREE="):
		case strings.HasPrefix(e, "GIT_INDEX_FILE="):
		default:
			env = append(env, e)
		}
	}
	return env
}
