package git

import (
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

// credentialFileMode is the file permissions for credential files.
const credentialFileMode = 0o600

// storeCredentials stores credentials for future git operations.
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
		if err := os.WriteFile(configPath, []byte(configStr), 0o644); err != nil {
			return err
		}
	}

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
