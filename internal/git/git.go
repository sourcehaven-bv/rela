// Package git provides git operations for the data entry app.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds git configuration for the data entry app.
type Config struct {
	Enabled       bool   `yaml:"enabled"`
	Mode          string `yaml:"mode"`           // "direct" or "pr"
	Branch        string `yaml:"branch"`         // for direct mode
	BaseBranch    string `yaml:"base_branch"`    // for pr mode: rebase onto this
	PushBranch    string `yaml:"push_branch"`    // for pr mode: push to this
	FetchInterval int    `yaml:"fetch_interval"` // seconds, 0 = disabled
}

// Status represents the current git state.
type Status struct {
	Available     bool     // true if git repo with remote
	Branch        string   // current branch name
	LocalChanges  int      // number of uncommitted files
	RemoteAhead   int      // commits ahead on remote
	Syncing       bool     // true if sync in progress
	Conflict      bool     // true if rebase conflict
	ConflictFiles []string // files with conflicts
}

// Ops provides git operations for a repository.
type Ops struct {
	root   string
	config Config
}

// NewOps creates a new git operations instance.
func NewOps(root string, cfg Config) *Ops {
	return &Ops{root: root, config: cfg}
}

// IsRepo checks if the directory is a git repository with a remote.
func IsRepo(root string) bool {
	gitDir := filepath.Join(root, ".git")
	if !exists(gitDir) {
		return false
	}
	// Check for at least one remote
	out, err := runGit(root, "remote")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

// GetStatus returns the current git status.
func (g *Ops) GetStatus() (*Status, error) {
	if !IsRepo(g.root) {
		return &Status{Available: false}, nil
	}

	status := &Status{Available: true}

	// Get current branch
	branch, err := runGit(g.root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("get branch: %w", err)
	}
	status.Branch = strings.TrimSpace(branch)

	// Count local changes (staged + unstaged + untracked in entities/relations)
	changes, err := g.countLocalChanges()
	if err != nil {
		return nil, fmt.Errorf("count changes: %w", err)
	}
	status.LocalChanges = changes

	// Check for rebase in progress
	if exists(filepath.Join(g.root, ".git", "rebase-merge")) ||
		exists(filepath.Join(g.root, ".git", "rebase-apply")) {

		status.Conflict = true
		files, _ := g.getConflictFiles()
		status.ConflictFiles = files
	}

	// Count remote ahead (requires fetch first)
	ahead, err := g.countRemoteAhead()
	if err == nil {
		status.RemoteAhead = ahead
	}

	return status, nil
}

// Fetch fetches from remote.
func (g *Ops) Fetch() error {
	_, err := runGit(g.root, "fetch", "origin")
	return err
}

// Sync performs commit + rebase + push.
func (g *Ops) Sync(message string) error {
	// Stage all changes in entities/ and relations/
	_, _ = runGit(g.root, "add", "entities/")
	_, _ = runGit(g.root, "add", "relations/")

	// Check if there's anything staged to commit
	staged, err := runGit(g.root, "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf("check staged: %w", err)
	}

	hasChanges := false
	for _, line := range strings.Split(staged, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			hasChanges = true
			break
		}
	}

	if hasChanges {
		if _, err := runGit(g.root, "commit", "-m", message); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
	}

	// Fetch latest
	if err := g.Fetch(); err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	// Check if rebase is needed (are we behind remote?)
	targetBranch := g.getBaseBranch()
	behind, _ := runGit(g.root, "rev-list", "--count", "HEAD..origin/"+targetBranch)
	behindCount := strings.TrimSpace(behind)

	if behindCount != "" && behindCount != "0" {
		// Rebase onto target branch, autostash to handle any unstaged changes
		if _, err := runGit(g.root, "rebase", "--autostash", "origin/"+targetBranch); err != nil {
			return fmt.Errorf("rebase: %w", err)
		}
	}

	// Check if we have commits to push
	pushBranch := g.getPushBranch()
	ahead, _ := runGit(g.root, "rev-list", "--count", "origin/"+pushBranch+"..HEAD")
	aheadCount := strings.TrimSpace(ahead)

	if aheadCount != "" && aheadCount != "0" {
		if _, err := runGit(g.root, "push", "origin", "HEAD:"+pushBranch); err != nil {
			return fmt.Errorf("push: %w", err)
		}
	}

	return nil
}

// AbortRebase aborts an in-progress rebase.
func (g *Ops) AbortRebase() error {
	_, err := runGit(g.root, "rebase", "--abort")
	return err
}

func (g *Ops) getBaseBranch() string {
	if g.config.Mode == "pr" && g.config.BaseBranch != "" {
		return g.config.BaseBranch
	}
	if g.config.Branch != "" {
		return g.config.Branch
	}
	return "main"
}

func (g *Ops) getPushBranch() string {
	if g.config.Mode == "pr" && g.config.PushBranch != "" {
		return g.config.PushBranch
	}
	// Get current branch
	branch, err := runGit(g.root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return g.getBaseBranch()
	}
	return strings.TrimSpace(branch)
}

func (g *Ops) countLocalChanges() (int, error) {
	out, err := runGit(g.root, "status", "--porcelain")
	if err != nil {
		return 0, err
	}

	count := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Only count changes in entities/ or relations/
		if strings.Contains(line, "entities/") || strings.Contains(line, "relations/") {
			count++
		}
	}
	return count, nil
}

func (g *Ops) countRemoteAhead() (int, error) {
	branch := g.getBaseBranch()
	out, err := runGit(g.root, "rev-list", "--count", "HEAD..origin/"+branch)
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (g *Ops) getConflictFiles() ([]string, error) {
	out, err := runGit(g.root, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", errors.New(stderr.String())
	}
	return stdout.String(), nil
}

func exists(path string) bool {
	cmd := exec.Command("test", "-e", path)
	return cmd.Run() == nil
}
