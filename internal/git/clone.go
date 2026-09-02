package git

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// CloneOptions configures the clone operation.
type CloneOptions struct {
	URL      string // Repository URL (HTTPS)
	Path     string // Local path to clone into
	Branch   string // Branch to checkout (optional, defaults to default branch)
	Token    string // OAuth token for authentication (optional for public repos)
	Username string // Username for authentication (defaults to "oauth2" when token is set)

	// BaseDir is the directory Path must stay inside. It is REQUIRED: Clone
	// refuses an empty BaseDir, and refuses a Path that escapes it.
	//
	// Required rather than optional because the guarantee this field provides
	// is for the caller who forgets (TKT-S2SFTG / issue #1270). Callers derive
	// the final segment of Path from remote-controlled values — notably
	// ExtractRepoName, which returns the URL's last path segment and can yield
	// ".." — and an optional BaseDir silently withheld containment from exactly
	// the caller who most needed it.
	//
	// Containment lives here rather than in the caller because Clone is the
	// point where an escaping path becomes dangerous: storeCredentials writes a
	// plaintext OAuth token into <Path>/.git/credentials, so a traversal both
	// clones outside the chosen directory and drops a credential there.
	BaseDir string
}

// Clone clones a git repository to the specified path.
// For private repositories, set Token from OAuth device flow.
func Clone(opts CloneOptions) error {
	if opts.URL == "" {
		return errors.New("repository URL is required")
	}
	if opts.Path == "" {
		return errors.New("clone path is required")
	}

	{
		target, cerr := containedPath(opts.BaseDir, opts.Path)
		if cerr != nil {
			return cerr
		}
		opts.Path = target
	}

	// Check if path already exists
	if _, err := os.Stat(opts.Path); err == nil {
		return fmt.Errorf("path already exists: %s", opts.Path)
	}

	// Ensure parent directory exists
	parent := filepath.Dir(opts.Path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	// Build clone options
	cloneOpts := &git.CloneOptions{
		URL:          opts.URL,
		SingleBranch: true,
	}

	// Set branch if specified
	if opts.Branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Branch)
	}

	// Add auth if token provided
	if opts.Token != "" {
		username := opts.Username
		if username == "" {
			username = defaultUsername
		}
		cloneOpts.Auth = &http.BasicAuth{
			Username: username,
			Password: opts.Token,
		}
	}

	// Clone the repository
	repo, err := git.PlainClone(opts.Path, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("clone: %w", err)
	}

	// Store credentials for future operations if token was provided
	if opts.Token != "" {
		_ = storeCredentials(repo, opts.Path, opts.URL, opts.Username, opts.Token)
	}

	return nil
}

const defaultUsername = "oauth2"

// containedPath cleans path and verifies it stays inside base, returning the
// cleaned absolute path.
//
// An empty base is an ERROR, not "skip the check" (TKT-S2SFTG / issue #1270).
// Skipping made the containment guarantee false in exactly the case it exists
// for: a caller who forgets to set BaseDir is the threat model, and that caller
// would have got no containment at all, silently. There is also no safe base to
// default to — falling back to the process CWD would contain the clone
// somewhere the caller never named, which is a different surprise rather than a
// smaller one.
//
// The check is string-level (Clean + Rel on absolute paths); it deliberately
// does not resolve symlinks, matching storage.RootedFS's threat model: the
// target is "the caller-supplied final segment contains traversal syntax", not
// "an attacker already has write access to the base directory".
func containedPath(base, path string) (string, error) {
	if base == "" {
		return "", errors.New("clone base directory is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve clone path: %w", err)
	}
	abs = filepath.Clean(abs)
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolve clone base directory: %w", err)
	}
	absBase = filepath.Clean(absBase)

	// A root base contains everything, so the check would pass for any path
	// while appearing to have verified something — the same silent-no-op shape
	// as the empty base above, reached by a different route (a config default
	// that resolves to "/", or a caller passing it deliberately). Refuse it:
	// there is no legitimate reason to scope a clone to the whole filesystem.
	if absBase == string(filepath.Separator) {
		return "", errors.New("clone base directory must not be the filesystem root")
	}

	rel, err := filepath.Rel(absBase, abs)
	if err != nil {
		return "", fmt.Errorf("clone path %q is not inside %q", path, base)
	}
	// "." means Path == BaseDir: a clone would target the base directory
	// itself rather than a subdirectory under it.
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("clone path %q escapes base directory %q", path, base)
	}
	return abs, nil
}

// credentialFileMode is owner-only: the credentials file holds the access token
// in cleartext (git's store helper has no other format), so any group/other read
// bit would hand the token to every local user.
const credentialFileMode = 0o600

// storeCredentials stores credentials for future git operations.
//
// repoPath MUST already have passed containedPath (Clone assigns the validated
// path back to opts.Path before calling this). Both files written here live at
// fixed names under repoPath/.git, so no caller-controlled segment reaches the
// filename — the containment of repoPath is the whole path-safety argument.
func storeCredentials(_ *git.Repository, repoPath, repoURL, username, token string) error {
	if username == "" {
		username = defaultUsername
	}

	// Parse URL to get host
	u, err := url.Parse(repoURL)
	if err != nil {
		return err
	}

	// Create credential URL for the store
	credentialURL := fmt.Sprintf("https://%s:%s@%s", username, token, u.Host)

	// Write credential directly to store file in .git directory
	credFile := filepath.Join(repoPath, ".git", "credentials")
	if writeErr := os.WriteFile(credFile, []byte(credentialURL+"\n"), credentialFileMode); writeErr != nil {
		return writeErr
	}

	// Configure git to use this credentials file
	// We need to set the config in the repository
	configPath := filepath.Join(repoPath, ".git", "config")
	config, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Append credential helper config if not present
	configStr := string(config)
	if !strings.Contains(configStr, "credential") {
		configStr += "\n[credential]\n\thelper = store --file=.git/credentials\n"
		// #nosec G703 -- configPath is filepath.Join(repoPath, ".git", "config"):
		// two literal segments under a repoPath that Clone already forced through
		// containedPath (see storeCredentials' doc comment). gosec's taint
		// analysis cannot see that barrier, so the containment is asserted here.
		if err := os.WriteFile(configPath, []byte(configStr), 0o644); err != nil {
			return err
		}
	}

	return nil
}

// ExtractRepoName extracts repository name from URL.
// e.g., "https://github.com/user/repo.git" -> "repo"
//
// The result is always a safe single path segment or "": the name comes from a
// remote-controlled URL, and callers join it onto a local directory. A URL
// ending in "/.." would otherwise yield "..", so names that are not a plain
// segment are rejected (callers already treat "" as "cannot determine name").
func ExtractRepoName(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		// Fallback: try to extract from path-like string
		return safeSegment(lastSegment(repoURL))
	}
	return safeSegment(lastSegment(strings.Trim(u.Path, "/")))
}

// lastSegment returns the final "/"-separated component of s, minus a trailing
// ".git".
func lastSegment(s string) string {
	parts := strings.Split(s, "/")
	return strings.TrimSuffix(parts[len(parts)-1], ".git")
}

// safeSegment returns name if it is a single, non-traversing path component,
// else "". It rejects ".", "..", separators, and NUL.
func safeSegment(name string) string {
	if name == "" || name == "." || name == ".." {
		return ""
	}
	if strings.ContainsAny(name, `/\:`) || strings.ContainsRune(name, 0) {
		return ""
	}
	return name
}

// IsValidRepoURL checks if the URL looks like a valid git repository URL.
func IsValidRepoURL(repoURL string) bool {
	u, err := url.Parse(repoURL)
	if err != nil {
		return false
	}

	// Must be HTTPS
	if u.Scheme != "https" {
		return false
	}

	// Must have a path (repo name)
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return false
	}

	// Common git hosts
	validHosts := []string{"github.com", "gitlab.com", "bitbucket.org"}
	for _, host := range validHosts {
		if u.Host == host || strings.HasSuffix(u.Host, "."+host) {
			return true
		}
	}

	// Accept any HTTPS URL with a path (could be self-hosted)
	return strings.Contains(path, "/")
}
