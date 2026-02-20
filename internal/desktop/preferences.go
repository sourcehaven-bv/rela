// Package desktop provides desktop-specific functionality for the Wails desktop app,
// including user preferences persistence.
package desktop

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const (
	// appDir is the directory name within the user's config directory.
	appDir = "rela-desktop"
	// preferencesFile is the filename for stored preferences.
	preferencesFile = "preferences.json"
	// maxRecentProjects is the maximum number of recent projects to keep.
	maxRecentProjects = 10
)

// RecentProject records a recently opened project.
type RecentProject struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	LastOpened time.Time `json:"last_opened"`
}

// Preferences stores desktop application preferences.
type Preferences struct {
	LastProject    string          `json:"last_project,omitempty"`
	RecentProjects []RecentProject `json:"recent_projects"`
	GitHubToken    string          `json:"github_token,omitempty"`
	CloneDir       string          `json:"clone_dir,omitempty"` // default directory for cloning
}

// Load reads preferences from the user's config directory.
// It returns empty preferences (not an error) if the file does not exist.
func Load() (*Preferences, error) {
	path, err := preferencesPath()
	if err != nil {
		return &Preferences{}, nil //nolint:nilerr // no config dir is not an error
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return &Preferences{}, nil
		}
		return nil, readErr
	}

	var prefs Preferences
	if unmarshalErr := json.Unmarshal(data, &prefs); unmarshalErr != nil {
		// Corrupted file: return empty preferences rather than failing.
		return &Preferences{}, nil //nolint:nilerr // corrupted prefs file is not fatal
	}
	return &prefs, nil
}

// Save writes preferences to the user's config directory.
func (p *Preferences) Save() error {
	path, err := preferencesPath()
	if err != nil {
		return err
	}

	mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755)
	if mkdirErr != nil {
		return mkdirErr
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// AddRecentProject adds or updates a project in the recent list and sets it as
// the last opened project. The entry is moved to the front of the list.
// The list is capped at maxRecentProjects entries.
func (p *Preferences) AddRecentProject(path, name string) {
	p.LastProject = path

	// Remove existing entry for this path.
	filtered := make([]RecentProject, 0, len(p.RecentProjects))
	for _, rp := range p.RecentProjects {
		if rp.Path != path {
			filtered = append(filtered, rp)
		}
	}

	// Prepend new entry.
	entry := RecentProject{
		Path:       path,
		Name:       name,
		LastOpened: time.Now(),
	}
	p.RecentProjects = append([]RecentProject{entry}, filtered...)

	// Cap the list.
	if len(p.RecentProjects) > maxRecentProjects {
		p.RecentProjects = p.RecentProjects[:maxRecentProjects]
	}
}

// RemoveRecentProject removes a project from the recent list.
// If the removed project was the last opened, LastProject is cleared.
func (p *Preferences) RemoveRecentProject(path string) {
	filtered := make([]RecentProject, 0, len(p.RecentProjects))
	for _, rp := range p.RecentProjects {
		if rp.Path != path {
			filtered = append(filtered, rp)
		}
	}
	p.RecentProjects = filtered

	if p.LastProject == path {
		p.LastProject = ""
	}
}

// ClearRecentProjects removes all recent projects and clears the last project.
func (p *Preferences) ClearRecentProjects() {
	p.RecentProjects = nil
	p.LastProject = ""
}

// preferencesPath returns the full path to the preferences file.
func preferencesPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, appDir, preferencesFile), nil
}
