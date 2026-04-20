package project

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// repoIDFileMode is the permission for .rela/repo-id — owner-
// readable only. The file doesn't contain secrets but there's no
// reason to hand out metadata cheaply.
const repoIDFileMode os.FileMode = 0o600

// RepoIDFile is the conventional filename under .rela/ holding the
// per-repo fingerprint (32 lowercase hex chars). It scopes the
// per-machine user-state directory so each project on a user's
// machine gets its own state tree.
//
// The file is local-only: .gitignore must list it. rela refuses to
// use a repo-id that is tracked by git — every collaborator would
// otherwise collapse onto the same per-repo state dir.
const RepoIDFile = "repo-id"

// repoIDHeader prefixes the generated file so users opening it see
// the commit hazard up front. The parser strips any '#' prefixed
// lines before validating the body.
const repoIDHeader = `# DO NOT COMMIT — this file identifies your per-machine user-state
# directory. Committing it collapses every collaborator's state
# onto the same directory on their own machine. rela refuses to
# use a tracked repo-id.
`

// ErrRepoIDTracked is returned when .rela/repo-id is present in
// git's index or working tree. See RepoIDFile docs.
var ErrRepoIDTracked = errors.New("project: .rela/repo-id is tracked by git")

// ErrRepoIDMalformed is returned when .rela/repo-id exists but
// doesn't decode to 32 lowercase hex chars. We refuse to silently
// regenerate: a regenerated id shadows existing user-state and the
// user deserves to know their file is corrupt.
var ErrRepoIDMalformed = errors.New("project: .rela/repo-id is malformed")

var repoIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ResolveRepoID returns the canonical repo-id for the project at
// root.
//
//   - If .rela/repo-id exists, its content is validated (32 lowercase
//     hex) and returned; anything else is ErrRepoIDMalformed.
//   - If .rela/repo-id is missing and keyringRepoID is non-empty (the
//     encrypted-repo case), the keyring value is written to disk and
//     returned.
//   - If both are missing (cleartext repo, first access), a new id is
//     generated with WriteRepoID and returned.
//
// When keyringRepoID is non-empty and .rela/repo-id already holds a
// different value, ResolveRepoID returns an error wrapping
// ErrRepoIDMismatch from the caller's perspective (we emit a clear
// message the caller can surface to the user).
//
// ResolveRepoID also runs the git-tracked check: a repo-id present
// in the index refuses with ErrRepoIDTracked. Non-git projects and
// projects without git on $PATH skip the check.
func ResolveRepoID(root, keyringRepoID string) (string, error) {
	path := filepath.Join(root, ".rela", RepoIDFile)

	if err := refuseIfTracked(root, path); err != nil {
		return "", err
	}

	existing, err := readRepoID(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if keyringRepoID != "" {
			if vErr := validate(keyringRepoID); vErr != nil {
				return "", fmt.Errorf("project: keyring repo-id: %w", vErr)
			}
			if wErr := WriteRepoID(root, keyringRepoID); wErr != nil {
				return "", wErr
			}
			return keyringRepoID, nil
		}
		fresh, gErr := newRepoID()
		if gErr != nil {
			return "", gErr
		}
		if wErr := WriteRepoID(root, fresh); wErr != nil {
			return "", wErr
		}
		return fresh, nil
	case err != nil:
		return "", err
	}

	if keyringRepoID != "" && existing != keyringRepoID {
		return "", fmt.Errorf(
			"project: .rela/repo-id (%s) disagrees with keyring repo id (%s) — "+
				"likely a copied-in .rela/ directory; remove .rela/repo-id "+
				"to reset, or restore the correct one", existing, keyringRepoID)
	}
	return existing, nil
}

// WriteRepoID writes id to .rela/repo-id atomically, with the
// do-not-commit header. Callers that already hold a valid id (e.g.
// freshly generated or keyring-derived) call this; callers that
// don't know the id yet go through ResolveRepoID.
func WriteRepoID(root, id string) error {
	if err := validate(id); err != nil {
		return err
	}
	dir := filepath.Join(root, ".rela")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("project: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, RepoIDFile)
	tmp := path + ".tmp"
	contents := repoIDHeader + id + "\n"
	if err := os.WriteFile(tmp, []byte(contents), repoIDFileMode); err != nil {
		return fmt.Errorf("project: write %s: %w", tmp, err)
	}
	return os.Rename(tmp, path)
}

// readRepoID parses the repo-id file, stripping comment lines and
// blank lines. Expects exactly one non-comment line containing the
// id. Other shapes → ErrRepoIDMalformed.
func readRepoID(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var body string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if body != "" {
			return "", fmt.Errorf("%w: multiple content lines", ErrRepoIDMalformed)
		}
		body = line
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("project: read %s: %w", path, err)
	}
	if err := validate(body); err != nil {
		return "", fmt.Errorf("%w: %s", ErrRepoIDMalformed, body)
	}
	return body, nil
}

func validate(id string) error {
	if !repoIDPattern.MatchString(id) {
		return ErrRepoIDMalformed
	}
	return nil
}

func newRepoID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("project: generate repo id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// refuseIfTracked runs `git ls-files --error-unmatch` to detect
// whether path is in the git index.
//
// Outcomes:
//
//   - `git ls-files` exits 0: file IS tracked → ErrRepoIDTracked.
//   - `git ls-files` exits non-zero (ExitError) with no deadline
//     or context error: file is NOT tracked → nil.
//   - No .git anywhere up the tree, or git not on $PATH, or the
//     path is outside the repo: not a meaningful check → nil.
//   - Anything else (context deadline exceeded, non-ExitError from
//     exec, permission denied, etc.): we don't know the answer.
//     Returning nil would let a tracked repo-id slip through on a
//     hung git; returning the error surfaces the ambiguity.
func refuseIfTracked(root, path string) error {
	if !isGitRepo(root) {
		return nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--error-unmatch", rel)
	cmd.Stdout = nil
	cmd.Stderr = nil
	runErr := cmd.Run()
	if runErr == nil {
		return fmt.Errorf("%w: add '.rela/%s' to .gitignore "+
			"(or to .rela/.gitignore) and remove it from the index with "+
			"'git rm --cached .rela/%s'",
			ErrRepoIDTracked, RepoIDFile, RepoIDFile)
	}
	// A clean non-zero exit is the "definitely not tracked" signal.
	// Any other failure mode (deadline exceeded, I/O error, startup
	// failure) is ambiguous and we refuse to proceed.
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return nil
	}
	return fmt.Errorf("project: git ls-files check failed: %w", runErr)
}

// gitTimeout caps how long we wait for `git ls-files` to report.
// The call is cheap in normal conditions; a long delay typically
// means git is blocked on a lock or an unresponsive filesystem.
const gitTimeout = 5 * time.Second

// isGitRepo walks up from root looking for a .git entry. We don't
// require the project root itself to hold .git — rela projects
// nested inside a larger repo are common.
func isGitRepo(root string) bool {
	dir := root
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
