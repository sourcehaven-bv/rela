package mobileapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// CloneResult contains the result of a clone operation.
type CloneResult struct {
	Path        string   `json:"path"`
	ProjectName string   `json:"projectName"`
	Projects    []string `json:"projects,omitempty"` // Multiple projects found
	Error       string   `json:"error,omitempty"`
}

// CloneRepository clones a git repository to the specified directory.
// If token is provided, it's used for authentication (for private repos).
// Returns JSON with the clone result.
//
// Parameters:
//   - repoURL: Git repository URL (https://github.com/user/repo.git or github.com/user/repo)
//   - destDir: Base directory to clone into (repo name will be appended)
//   - token: GitHub token for authentication (empty for public repos)
func CloneRepository(repoURL, destDir, token string) string {
	result := CloneResult{}

	// Normalize URL
	repoURL = normalizeGitURL(repoURL)

	// Extract repo name from URL
	repoName := extractRepoName(repoURL)
	if repoName == "" {
		result.Error = "could not extract repository name from URL"
		return toJSON(result)
	}

	// Full destination path
	destPath := filepath.Join(destDir, repoName)
	result.Path = destPath

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		result.Error = "directory already exists: " + destPath
		return toJSON(result)
	}

	// Clone options
	cloneOpts := &git.CloneOptions{
		URL:      repoURL,
		Progress: nil, // Could add progress callback later
	}

	// Add authentication if token provided
	if token != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "x-access-token", // GitHub accepts any username with token
			Password: token,
		}
	}

	// Perform clone
	_, err := git.PlainClone(destPath, false, cloneOpts)
	if err != nil {
		result.Error = fmt.Sprintf("clone failed: %v", err)
		return toJSON(result)
	}

	// Find rela projects in the cloned repo
	projects := findRelaProjects(destPath)
	switch len(projects) {
	case 0:
		// No metamodel.yaml found - might not be a rela project
		result.ProjectName = repoName
	case 1:
		result.ProjectName = filepath.Base(projects[0])
		result.Path = projects[0]
	default:
		// Multiple projects found
		result.Projects = projects
	}

	return toJSON(result)
}

// ListProjects scans a directory for rela projects (directories containing metamodel.yaml).
// Returns JSON array of ProjectInfo objects.
func ListProjects(baseDir string) string {
	paths := findRelaProjects(baseDir)
	projects := make([]ProjectInfo, 0, len(paths))
	for _, path := range paths {
		info := getProjectInfoStruct(path)
		projects = append(projects, info)
	}
	out, _ := json.Marshal(projects)
	return string(out)
}

func getProjectInfoStruct(projectPath string) ProjectInfo {
	info := ProjectInfo{
		Path: projectPath,
		Name: filepath.Base(projectPath),
	}

	// Check for git info (direct or in parent directories)
	info.HasGit, info.GitBranch = findGitInfo(projectPath)

	// Check for entities directory
	info.EntityDirs = findEntityDirs(projectPath)

	return info
}

// ProjectInfo contains metadata about a rela project.
type ProjectInfo struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	HasGit     bool     `json:"hasGit"`
	GitBranch  string   `json:"gitBranch,omitempty"`
	EntityDirs []string `json:"entityDirs,omitempty"`
}

// findGitInfo checks for a .git directory in the project path or parent directories.
// Returns whether git was found and the current branch name.
func findGitInfo(projectPath string) (hasGit bool, branch string) {
	// Check direct .git directory
	gitDir := filepath.Join(projectPath, ".git")
	if isDir(gitDir) {
		return true, getBranchName(projectPath)
	}

	// Check parent directories (nested project scenario)
	parent := filepath.Dir(projectPath)
	for i := 0; i < 3; i++ {
		gitDir := filepath.Join(parent, ".git")
		if isDir(gitDir) {
			return true, getBranchName(parent)
		}
		parent = filepath.Dir(parent)
	}
	return false, ""
}

// getBranchName returns the current branch name for a git repository.
func getBranchName(repoPath string) string {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return ""
	}
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	return head.Name().Short()
}

// isDir checks if the given path is a directory.
func isDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

// findEntityDirs returns the list of entity type directories in a project.
func findEntityDirs(projectPath string) []string {
	entitiesDir := filepath.Join(projectPath, "entities")
	if !isDir(entitiesDir) {
		return nil
	}
	entries, err := os.ReadDir(entitiesDir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs
}

// GetProjectInfo returns information about a project at the given path.
func GetProjectInfo(projectPath string) string {
	info := getProjectInfoStruct(projectPath)
	return toJSON(info)
}

// DeleteProject removes a project directory and all its contents.
func DeleteProject(projectPath string) error {
	// Safety check - make sure it looks like a valid path
	if projectPath == "" || projectPath == "/" {
		return fmt.Errorf("invalid project path")
	}

	return os.RemoveAll(projectPath)
}

// --- Helper functions ---

func normalizeGitURL(url string) string {
	url = strings.TrimSpace(url)

	// Handle shorthand: github.com/user/repo -> https://github.com/user/repo.git
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "git@") {
		if strings.HasPrefix(url, "github.com/") || strings.HasPrefix(url, "gitlab.com/") || strings.HasPrefix(url, "bitbucket.org/") {
			url = "https://" + url
		}
	}

	// Ensure .git suffix for HTTPS URLs
	if strings.HasPrefix(url, "https://") && !strings.HasSuffix(url, ".git") {
		url += ".git"
	}

	return url
}

func extractRepoName(url string) string {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Get last path component
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func findRelaProjects(baseDir string) []string {
	return findRelaProjectsRecursive(baseDir, 0, 3) // Search up to 3 levels deep
}

func findRelaProjectsRecursive(dir string, depth, maxDepth int) []string {
	var projects []string

	if depth >= maxDepth {
		return projects
	}

	// Check if dir itself is a project
	metamodel := filepath.Join(dir, "metamodel.yaml")
	if _, err := os.Stat(metamodel); err == nil {
		projects = append(projects, dir)
		return projects // Don't recurse into rela projects
	}

	// Check subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return projects
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories and common non-project directories
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			continue
		}
		subDir := filepath.Join(dir, name)
		subProjects := findRelaProjectsRecursive(subDir, depth+1, maxDepth)
		projects = append(projects, subProjects...)
	}

	return projects
}

func toJSON(v interface{}) string {
	out, _ := json.Marshal(v)
	return string(out)
}
