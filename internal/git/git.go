// Package git provides git operations for the data entry app.
package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// maxCommitTraversalLimit is the safety limit for commit traversal to avoid infinite loops.
const maxCommitTraversalLimit = 1000

// Config holds git configuration for the data entry app.
type Config struct {
	Enabled       bool   `yaml:"enabled"`
	Mode          string `yaml:"mode"`           // "direct" or "pr"
	Branch        string `yaml:"branch"`         // for direct mode
	BaseBranch    string `yaml:"base_branch"`    // for pr mode: merge from this
	PushBranch    string `yaml:"push_branch"`    // for pr mode: push to this
	FetchInterval int    `yaml:"fetch_interval"` // seconds, 0 = disabled
	Token         string `yaml:"-"`              // OAuth token for auth (not persisted)
	Username      string `yaml:"-"`              // Username for auth (not persisted)
}

// Status represents the current git state.
type Status struct {
	Available     bool     // true if git repo with remote
	Branch        string   // current branch name
	LocalChanges  int      // number of uncommitted files
	RemoteAhead   int      // commits ahead on remote
	Syncing       bool     // true if sync in progress
	Conflict      bool     // true if merge conflict
	ConflictFiles []string // files with conflicts
}

// Ops provides git operations for a repository.
type Ops struct {
	root   string
	config Config
	repo   *git.Repository
}

// NewOps creates a new git operations instance.
func NewOps(root string, cfg Config) *Ops {
	ops := &Ops{root: root, config: cfg}
	// Try to open the repo, but don't fail if it doesn't exist
	repo, err := git.PlainOpen(root)
	if err == nil {
		ops.repo = repo
	}
	return ops
}

// IsRepo checks if the directory is a git repository with a remote.
func IsRepo(root string) bool {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return false
	}

	// Check for at least one remote
	remotes, err := repo.Remotes()
	if err != nil {
		return false
	}
	return len(remotes) > 0
}

// GetStatus returns the current git status.
func (g *Ops) GetStatus() (*Status, error) {
	if g.repo == nil {
		return &Status{Available: false}, nil
	}

	if !IsRepo(g.root) {
		return &Status{Available: false}, nil
	}

	status := &Status{Available: true}

	// Get current branch
	head, err := g.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}
	status.Branch = head.Name().Short()

	// Count local changes (staged + unstaged + untracked in entities/relations)
	changes, err := g.countLocalChanges()
	if err != nil {
		return nil, fmt.Errorf("count changes: %w", err)
	}
	status.LocalChanges = changes

	// Check for merge conflict in progress
	if exists(filepath.Join(g.root, ".git", "MERGE_HEAD")) {
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
	if g.repo == nil {
		return errors.New("not a git repository")
	}

	fetchOpts := &git.FetchOptions{
		RemoteName: "origin",
	}

	// Add auth if token is set
	if g.config.Token != "" {
		username := g.config.Username
		if username == "" {
			username = defaultUsername
		}
		fetchOpts.Auth = &http.BasicAuth{
			Username: username,
			Password: g.config.Token,
		}
	}

	err := g.repo.Fetch(fetchOpts)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	return nil
}

// ErrConflictInProgress indicates a merge conflict that must be resolved first.
var ErrConflictInProgress = errors.New("merge conflict in progress, resolve before syncing")

// Sync performs commit + merge + push.
func (g *Ops) Sync(message string) error {
	if g.repo == nil {
		return errors.New("not a git repository")
	}

	// Check for merge conflict first
	if exists(filepath.Join(g.root, ".git", "MERGE_HEAD")) {
		return ErrConflictInProgress
	}

	wt, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	// Stage all changes in entities/ and relations/
	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	hasChanges := false
	for path, s := range status {
		if strings.HasPrefix(path, "entities/") || strings.HasPrefix(path, "relations/") {
			if s.Worktree != git.Unmodified || s.Staging != git.Unmodified {
				if _, addErr := wt.Add(path); addErr != nil {
					// Ignore errors for deleted files that don't exist
					continue
				}
				hasChanges = true
			}
		}
	}

	if hasChanges {
		_, err = wt.Commit(message, &git.CommitOptions{
			Author: &object.Signature{
				Name:  "rela",
				Email: "rela@local",
				When:  time.Now(),
			},
		})
		if err != nil {
			return fmt.Errorf("commit: %w", err)
		}
	}

	// Fetch latest
	if fetchErr := g.Fetch(); fetchErr != nil {
		return fmt.Errorf("fetch: %w", fetchErr)
	}

	// Check if merge is needed (are we behind remote?)
	targetBranch := g.getBaseBranch()
	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", targetBranch)

	remoteRef, err := g.repo.Reference(remoteBranchRef, true)
	if err != nil {
		// Remote branch doesn't exist, skip merge
		return g.push()
	}

	head, err := g.repo.Head()
	if err != nil {
		return fmt.Errorf("get HEAD: %w", err)
	}

	// Check if we're behind
	behind, err := g.commitsBehind(head.Hash(), remoteRef.Hash())
	if err != nil {
		return fmt.Errorf("check behind: %w", err)
	}

	if behind > 0 {
		// Perform merge
		if err := g.merge(remoteRef.Hash(), "Merge "+targetBranch); err != nil {
			return fmt.Errorf("merge: %w", err)
		}
	}

	return g.push()
}

// push pushes to remote.
func (g *Ops) push() error {
	pushBranch := g.getPushBranch()

	head, err := g.repo.Head()
	if err != nil {
		return fmt.Errorf("get HEAD: %w", err)
	}

	pushOpts := &git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(string(head.Name()) + ":refs/heads/" + pushBranch),
		},
	}

	// Add auth if token is set
	if g.config.Token != "" {
		username := g.config.Username
		if username == "" {
			username = defaultUsername
		}
		pushOpts.Auth = &http.BasicAuth{
			Username: username,
			Password: g.config.Token,
		}
	}

	err = g.repo.Push(pushOpts)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("push: %w", err)
	}
	return nil
}

// merge performs a merge of the given commit into HEAD.
func (g *Ops) merge(commitHash plumbing.Hash, message string) error {
	wt, err := g.repo.Worktree()
	if err != nil {
		return err
	}

	// Get current HEAD
	head, err := g.repo.Head()
	if err != nil {
		return err
	}

	// Get the commit to merge
	mergeCommit, err := g.repo.CommitObject(commitHash)
	if err != nil {
		return err
	}

	// Create merge commit
	_, err = wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "rela",
			Email: "rela@local",
			When:  time.Now(),
		},
		Parents: []plumbing.Hash{head.Hash(), mergeCommit.Hash},
	})

	return err
}

// commitsBehind returns how many commits HEAD is behind the target.
func (g *Ops) commitsBehind(headHash, targetHash plumbing.Hash) (int, error) {
	if headHash == targetHash {
		return 0, nil
	}

	// Walk from target back to find head
	count := 0
	iter, err := g.repo.Log(&git.LogOptions{From: targetHash})
	if err != nil {
		return 0, err
	}

	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == headHash {
			return errors.New("found")
		}
		count++
		if count > maxCommitTraversalLimit {
			return errors.New("too many commits")
		}
		return nil
	})

	if err != nil && err.Error() == "found" {
		return count, nil
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}

// AbortMerge aborts an in-progress merge.
func (g *Ops) AbortMerge() error {
	mergeHeadPath := filepath.Join(g.root, ".git", "MERGE_HEAD")
	if !exists(mergeHeadPath) {
		return errors.New("no merge in progress")
	}

	// Remove merge state files
	for _, f := range []string{"MERGE_HEAD", "MERGE_MSG", "MERGE_MODE"} {
		_ = os.Remove(filepath.Join(g.root, ".git", f))
	}

	// Reset to HEAD
	if g.repo != nil {
		wt, err := g.repo.Worktree()
		if err != nil {
			return err
		}
		return wt.Reset(&git.ResetOptions{Mode: git.HardReset})
	}
	return nil
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
	if g.repo != nil {
		head, err := g.repo.Head()
		if err == nil {
			return head.Name().Short()
		}
	}
	return g.getBaseBranch()
}

func (g *Ops) countLocalChanges() (int, error) {
	if g.repo == nil {
		return 0, errors.New("not a git repository")
	}

	wt, err := g.repo.Worktree()
	if err != nil {
		return 0, err
	}

	status, err := wt.Status()
	if err != nil {
		return 0, err
	}

	count := 0
	for path, s := range status {
		// Only count changes in entities/ or relations/
		if strings.HasPrefix(path, "entities/") || strings.HasPrefix(path, "relations/") {
			if s.Worktree != git.Unmodified || s.Staging != git.Unmodified {
				count++
			}
		}
	}
	return count, nil
}

func (g *Ops) countRemoteAhead() (int, error) {
	if g.repo == nil {
		return 0, errors.New("not a git repository")
	}

	branch := g.getBaseBranch()
	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branch)

	remoteRef, err := g.repo.Reference(remoteBranchRef, true)
	if err != nil {
		return 0, err
	}

	head, err := g.repo.Head()
	if err != nil {
		return 0, err
	}

	return g.commitsBehind(head.Hash(), remoteRef.Hash())
}

func (g *Ops) getConflictFiles() ([]string, error) {
	if g.repo == nil {
		return nil, errors.New("not a git repository")
	}

	wt, err := g.repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := wt.Status()
	if err != nil {
		return nil, err
	}

	var files []string
	for path, s := range status {
		// Both modified indicates conflict
		if s.Worktree == git.UpdatedButUnmerged || s.Staging == git.UpdatedButUnmerged {
			files = append(files, path)
		}
	}
	return files, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
