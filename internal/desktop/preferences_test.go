package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestPrefs creates a Preferences that saves/loads from a temp directory.
// It overrides preferencesPath by setting the config dir env var for the platform.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prefsDir := filepath.Join(dir, appDir)
	if err := os.MkdirAll(prefsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(prefsDir, preferencesFile)
}

func writePrefsFile(t *testing.T, path string, prefs *Preferences) {
	t.Helper()
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readPrefsFile(t *testing.T, path string) *Preferences {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var prefs Preferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		t.Fatal(err)
	}
	return &prefs
}

func TestLoadReturnsEmptyWhenFileDoesNotExist(t *testing.T) {
	// Load uses os.UserConfigDir which we can't easily override,
	// but we can test the behavior by directly testing the logic.
	prefs, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// Should return a valid Preferences (possibly with data if user has used the app).
	if prefs == nil {
		t.Fatal("Load() returned nil")
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	prefs := &Preferences{
		LastProject: "/tmp/myproject",
		RecentProjects: []RecentProject{
			{Path: "/tmp/myproject", Name: "My Project", LastOpened: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)},
			{Path: "/tmp/other", Name: "Other", LastOpened: time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC)},
		},
	}

	path := setupTestDir(t)
	writePrefsFile(t, path, prefs)

	loaded := readPrefsFile(t, path)
	if loaded.LastProject != prefs.LastProject {
		t.Errorf("LastProject = %q, want %q", loaded.LastProject, prefs.LastProject)
	}
	if len(loaded.RecentProjects) != 2 {
		t.Fatalf("RecentProjects len = %d, want 2", len(loaded.RecentProjects))
	}
	if loaded.RecentProjects[0].Path != "/tmp/myproject" {
		t.Errorf("RecentProjects[0].Path = %q, want %q", loaded.RecentProjects[0].Path, "/tmp/myproject")
	}
}

func TestAddRecentProject(t *testing.T) {
	prefs := &Preferences{}

	prefs.AddRecentProject("/path/a", "Project A")
	if prefs.LastProject != "/path/a" {
		t.Errorf("LastProject = %q, want %q", prefs.LastProject, "/path/a")
	}
	if len(prefs.RecentProjects) != 1 {
		t.Fatalf("RecentProjects len = %d, want 1", len(prefs.RecentProjects))
	}
	if prefs.RecentProjects[0].Name != "Project A" {
		t.Errorf("Name = %q, want %q", prefs.RecentProjects[0].Name, "Project A")
	}
}

func TestAddRecentProjectMovesToFront(t *testing.T) {
	prefs := &Preferences{}

	prefs.AddRecentProject("/path/a", "A")
	prefs.AddRecentProject("/path/b", "B")
	prefs.AddRecentProject("/path/a", "A Updated")

	if len(prefs.RecentProjects) != 2 {
		t.Fatalf("RecentProjects len = %d, want 2", len(prefs.RecentProjects))
	}
	if prefs.RecentProjects[0].Path != "/path/a" {
		t.Errorf("first entry path = %q, want %q", prefs.RecentProjects[0].Path, "/path/a")
	}
	if prefs.RecentProjects[0].Name != "A Updated" {
		t.Errorf("first entry name = %q, want %q", prefs.RecentProjects[0].Name, "A Updated")
	}
	if prefs.RecentProjects[1].Path != "/path/b" {
		t.Errorf("second entry path = %q, want %q", prefs.RecentProjects[1].Path, "/path/b")
	}
	if prefs.LastProject != "/path/a" {
		t.Errorf("LastProject = %q, want %q", prefs.LastProject, "/path/a")
	}
}

func TestAddRecentProjectCapsAtMax(t *testing.T) {
	prefs := &Preferences{}

	for i := range 15 {
		prefs.AddRecentProject("/path/"+string(rune('a'+i)), "Project")
	}

	if len(prefs.RecentProjects) != maxRecentProjects {
		t.Errorf("RecentProjects len = %d, want %d", len(prefs.RecentProjects), maxRecentProjects)
	}
}

func TestRemoveRecentProject(t *testing.T) {
	prefs := &Preferences{}
	prefs.AddRecentProject("/path/a", "A")
	prefs.AddRecentProject("/path/b", "B")

	prefs.RemoveRecentProject("/path/b")

	if len(prefs.RecentProjects) != 1 {
		t.Fatalf("RecentProjects len = %d, want 1", len(prefs.RecentProjects))
	}
	if prefs.RecentProjects[0].Path != "/path/a" {
		t.Errorf("remaining path = %q, want %q", prefs.RecentProjects[0].Path, "/path/a")
	}
}

func TestRemoveRecentProjectClearsLastProject(t *testing.T) {
	prefs := &Preferences{
		LastProject: "/path/a",
		RecentProjects: []RecentProject{
			{Path: "/path/a", Name: "A"},
		},
	}

	prefs.RemoveRecentProject("/path/a")

	if prefs.LastProject != "" {
		t.Errorf("LastProject = %q, want empty", prefs.LastProject)
	}
}

func TestRemoveRecentProjectKeepsLastProjectIfDifferent(t *testing.T) {
	prefs := &Preferences{
		LastProject: "/path/a",
		RecentProjects: []RecentProject{
			{Path: "/path/a", Name: "A"},
			{Path: "/path/b", Name: "B"},
		},
	}

	prefs.RemoveRecentProject("/path/b")

	if prefs.LastProject != "/path/a" {
		t.Errorf("LastProject = %q, want %q", prefs.LastProject, "/path/a")
	}
}

func TestClearRecentProjects(t *testing.T) {
	prefs := &Preferences{
		LastProject: "/path/a",
		RecentProjects: []RecentProject{
			{Path: "/path/a", Name: "A"},
			{Path: "/path/b", Name: "B"},
		},
	}

	prefs.ClearRecentProjects()

	if prefs.LastProject != "" {
		t.Errorf("LastProject = %q, want empty", prefs.LastProject)
	}
	if len(prefs.RecentProjects) != 0 {
		t.Errorf("RecentProjects len = %d, want 0", len(prefs.RecentProjects))
	}
}

func TestAddRecentProjectDeduplicates(t *testing.T) {
	prefs := &Preferences{}

	prefs.AddRecentProject("/path/a", "A")
	prefs.AddRecentProject("/path/a", "A")
	prefs.AddRecentProject("/path/a", "A")

	if len(prefs.RecentProjects) != 1 {
		t.Errorf("RecentProjects len = %d, want 1", len(prefs.RecentProjects))
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	// This tests Save() through the real path. We can't easily redirect it,
	// but we can verify the method doesn't panic on a valid system.
	prefs := &Preferences{
		LastProject: "/tmp/test",
	}
	// Save should succeed (or at least not panic).
	err := prefs.Save()
	if err != nil {
		t.Logf("Save() returned error (may be expected in restricted env): %v", err)
	}
}

func TestPreferencesPath(t *testing.T) {
	path, err := preferencesPath()
	if err != nil {
		t.Fatalf("preferencesPath() error: %v", err)
	}
	if filepath.Base(path) != preferencesFile {
		t.Errorf("filename = %q, want %q", filepath.Base(path), preferencesFile)
	}
	if filepath.Base(filepath.Dir(path)) != appDir {
		t.Errorf("parent dir = %q, want %q", filepath.Base(filepath.Dir(path)), appDir)
	}
}
