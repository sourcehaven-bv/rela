package git

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// CloneOptions configures the clone operation.
type CloneOptions struct {
	URL      string // Repository URL (HTTPS)
	Path     string // Local path to clone into
	Branch   string // Branch to checkout (optional, defaults to default branch)
	Token    string // OAuth token for authentication (optional for public repos)
	Username string // Username for authentication (defaults to "oauth2" when token is set)
}

// Clone clones a git repository to the specified path.
// For private repositories, set Token from OAuth device flow.
func Clone(opts CloneOptions) error {
	if opts.URL == "" {
		return fmt.Errorf("repository URL is required")
	}
	if opts.Path == "" {
		return fmt.Errorf("clone path is required")
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

	// Build clone URL with authentication if token provided
	cloneURL := opts.URL
	if opts.Token != "" {
		authURL, err := injectAuth(opts.URL, opts.Username, opts.Token)
		if err != nil {
			return fmt.Errorf("inject auth: %w", err)
		}
		cloneURL = authURL
	}

	// Build clone command
	args := []string{"clone"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	args = append(args, "--single-branch", cloneURL, opts.Path)

	// Use parent directory as working directory
	_, err := runGit(parent, args...)
	if err != nil {
		return fmt.Errorf("clone: %w", err)
	}

	// Configure credential helper to avoid prompts (store token for future operations)
	if opts.Token != "" {
		// Non-fatal: clone succeeded, just log if we can't store credentials
		_ = configureCredentials(opts.Path, opts.URL, opts.Username, opts.Token)
	}

	return nil
}

// injectAuth injects username:password into an HTTPS URL.
func injectAuth(repoURL, username, token string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}

	if u.Scheme != "https" {
		return "", fmt.Errorf("only HTTPS URLs supported, got %s", u.Scheme)
	}

	if username == "" {
		username = defaultUsername
	}

	u.User = url.UserPassword(username, token)
	return u.String(), nil
}

const defaultUsername = "oauth2"

// credentialFileMode is the file permissions for credential files.
const credentialFileMode = 0o600

// configureCredentials stores credentials for future git operations.
func configureCredentials(repoPath, repoURL, username, token string) error {
	if username == "" {
		username = defaultUsername
	}

	// Parse URL to get host
	u, err := url.Parse(repoURL)
	if err != nil {
		return err
	}

	// Use git credential store for this repo
	// This creates a .git-credentials file or uses the system credential helper
	credentialURL := fmt.Sprintf("https://%s:%s@%s", username, token, u.Host)

	// Store credential using git credential approve
	_, _ = runGit(repoPath, "config", "credential.helper", "store")

	// Write credential directly to store file
	credFile := filepath.Join(repoPath, ".git", "credentials")
	if err := os.WriteFile(credFile, []byte(credentialURL+"\n"), credentialFileMode); err != nil {
		return err
	}

	// Point git to use this credentials file
	_, _ = runGit(repoPath, "config", "credential.helper", "store --file=.git/credentials")

	return nil
}

// ExtractRepoName extracts repository name from URL.
// e.g., "https://github.com/user/repo.git" -> "repo"
func ExtractRepoName(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		// Fallback: try to extract from path-like string
		parts := strings.Split(repoURL, "/")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			return strings.TrimSuffix(name, ".git")
		}
		return ""
	}

	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		return strings.TrimSuffix(name, ".git")
	}
	return ""
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
