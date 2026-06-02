// rela-desktop runs the data entry application as a native desktop app using Wails.
//
// Usage:
//
//	rela-desktop [-project .]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/desktop"
	"github.com/Sourcehaven-BV/rela/internal/git"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// GitHubClientID is the OAuth App client ID for rela-desktop.
// Users can create their own OAuth App at https://github.com/settings/developers
// For development, use a test client ID.
const GitHubClientID = "" // Set via build flags or environment

// Desktop is the backend bound to the Wails frontend.
// It manages project lifecycle: opening a directory picker, loading a project,
// and persisting recent projects in user preferences.
type Desktop struct {
	ctx               context.Context //nolint:containedctx // Wails runtime ctx must live for the struct lifetime
	mu                sync.RWMutex
	app               *dataentry.App
	svc               *appbuild.Services // per-project services; closed on next LoadProject
	handler           http.Handler
	loadErr           string
	prefs             *desktop.Preferences
	cloneAuth         *cloneAuthState
	lastCloneDir      string           // tracks the most recent clone for project selection
	pendingSetupDir   string           // project dir awaiting data-entry.yaml setup
	pendingSetupFS    storage.FS       // fs for pending setup
	pendingSetupPaths *project.Context // project paths for pending setup
	stopScheduler     context.CancelFunc
}

// cloneAuthState tracks an in-progress OAuth device flow.
type cloneAuthState struct {
	deviceCode string
	userCode   string
	verifyURL  string
	expiresIn  int
	interval   int
}

// coverage-ignore: Wails lifecycle callback
func (d *Desktop) startup(ctx context.Context) {
	d.ctx = principal.With(ctx, principal.Principal{
		User: principal.SystemUser(),
		Tool: principal.ToolDesktop,
	})
}

// OpenProject opens a native directory picker and loads the selected project.
// It returns an error string (empty on success) so the JS frontend can react.
func (d *Desktop) OpenProject() string {
	dir, err := runtime.OpenDirectoryDialog(d.ctx, runtime.OpenDialogOptions{
		Title: "Open Rela Project",
	})
	if err != nil {
		return fmt.Sprintf("dialog error: %v", err)
	}
	if dir == "" {
		return "" // user cancelled
	}
	return d.LoadProject(dir)
}

// OpenRecentProject loads a project from the recent projects list.
// It returns an error string (empty on success) so the JS frontend can react.
func (d *Desktop) OpenRecentProject(path string) string {
	errMsg := d.LoadProject(path)
	if errMsg != "" {
		// Project no longer valid — remove from recent list.
		d.prefs.RemoveRecentProject(path)
		_ = d.prefs.Save()
		d.refreshMenu()
	}
	return errMsg
}

// failLoad records err as the current load error and returns its
// string form, so LoadProject's error branches stay one line each.
func (d *Desktop) failLoad(err error) string {
	d.mu.Lock()
	d.loadErr = err.Error()
	d.mu.Unlock()
	return err.Error()
}

// LoadProject loads a rela project from the given directory.
func (d *Desktop) LoadProject(dir string) string {
	fs, projCtx, err := discoverProject(dir)
	if err != nil {
		return d.failLoad(err)
	}

	// Check if data-entry.yaml exists
	configPath := filepath.Join(dir, dataentry.ConfigFile)
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		// Store pending setup state
		d.mu.Lock()
		d.pendingSetupDir = dir
		d.pendingSetupFS = fs
		d.pendingSetupPaths = projCtx
		d.loadErr = ""
		d.mu.Unlock()
		return "needs_setup"
	}

	auditSink, auditErr := audit.NewFilesystem(filepath.Join(projCtx.CacheDir, "audit"))
	if auditErr != nil {
		d.mu.Lock()
		d.loadErr = auditErr.Error()
		d.mu.Unlock()
		return auditErr.Error()
	}
	svc, svcErr := appbuild.New(appbuild.Config{
		FS:           fs,
		Paths:        projCtx,
		ScriptEngine: script.NewEngine(),
		Audit:        auditSink,
	})
	if svcErr != nil {
		d.mu.Lock()
		d.loadErr = svcErr.Error()
		d.mu.Unlock()
		return svcErr.Error()
	}

	fieldResolver, err := dataentry.ResolverFromServices(svc)
	if err != nil {
		return d.failLoad(err)
	}

	app, err := dataentry.NewApp(
		fs, projCtx, svc.Meta(), svc.Store(),
		svc.EntityManager(), svc.Searcher(), svc.ACL(),
		fieldResolver,
		svc.Audit(),
	)
	if err != nil {
		return d.failLoad(err)
	}
	d.mu.Lock()
	// Stop previous scheduler and close previous services in
	// dependency order: scheduler first so no in-flight tick lands
	// on a closed store, then svc.Close() releases the store + bleve.
	if d.stopScheduler != nil {
		d.stopScheduler()
		d.stopScheduler = nil
	}
	prevSvc := d.svc
	d.svc = svc
	d.app = app
	d.handler = app.NewRouter()
	d.loadErr = ""
	d.pendingSetupDir = ""
	d.pendingSetupFS = nil
	d.pendingSetupPaths = nil

	// Start background scheduler for the new project.
	schedCtx, schedCancel := context.WithCancel(context.Background())
	d.stopScheduler = schedCancel
	d.mu.Unlock()

	// Close the previous project's services outside the lock so the
	// store/index Close doesn't block other Desktop methods.
	if prevSvc != nil {
		_ = prevSvc.Close()
	}

	scheduler.StartBackground(schedCtx, svc, slog.Default())

	if d.ctx != nil {
		runtime.WindowSetTitle(d.ctx, app.ProjectName())
	}

	// Update preferences with successfully opened project.
	d.prefs.AddRecentProject(app.ProjectRoot(), app.ProjectName())
	if saveErr := d.prefs.Save(); saveErr != nil {
		slog.Warn("could not save preferences", "error", saveErr)
	}
	d.refreshMenu()

	return ""
}

// NeedsSetup returns true if a project needs data-entry.yaml setup.
func (d *Desktop) NeedsSetup() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.pendingSetupDir != ""
}

// GetSetupInfo returns information about the project needing setup.
func (d *Desktop) GetSetupInfo() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.pendingSetupPaths == nil {
		return map[string]interface{}{"error": "No project pending setup"}
	}

	loader := metamodel.NewFSLoader(d.pendingSetupFS, d.pendingSetupPaths.MetamodelPath)
	meta, _, err := loader.Load(context.Background())
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Failed to load metamodel: %v", err)}
	}

	entityTypes := make([]string, 0, len(meta.Entities))
	for name := range meta.Entities {
		entityTypes = append(entityTypes, name)
	}

	return map[string]interface{}{
		"path":         d.pendingSetupDir,
		"entity_types": entityTypes,
	}
}

// GenerateDataEntryConfig creates a data-entry.yaml from the metamodel.
func (d *Desktop) GenerateDataEntryConfig(appName string) string {
	d.mu.Lock()
	fs := d.pendingSetupFS
	paths := d.pendingSetupPaths
	dir := d.pendingSetupDir
	d.mu.Unlock()

	if paths == nil {
		return "No project pending setup"
	}

	meta, _, err := metamodel.NewFSLoader(fs, paths.MetamodelPath).Load(context.Background())
	if err != nil {
		return fmt.Sprintf("Failed to load metamodel: %v", err)
	}

	config := generateDataEntryConfig(appName, meta)
	configPath := filepath.Join(dir, dataentry.ConfigFile)

	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		return fmt.Sprintf("Failed to write config: %v", err)
	}

	// Now load the project
	d.mu.Lock()
	d.pendingSetupDir = ""
	d.pendingSetupFS = nil
	d.pendingSetupPaths = nil
	d.mu.Unlock()

	return d.LoadProject(dir)
}

// GetDefaultCloneDir returns the default directory for cloning repositories.
func (d *Desktop) GetDefaultCloneDir() string {
	if d.prefs.CloneDir != "" {
		return d.prefs.CloneDir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "rela-projects")
}

// PickCloneDirectory opens a directory picker and returns the selected path.
func (d *Desktop) PickCloneDirectory() string {
	dir, err := runtime.OpenDirectoryDialog(d.ctx, runtime.OpenDialogOptions{
		Title:            "Select Clone Destination",
		DefaultDirectory: d.GetDefaultCloneDir(),
	})
	if err != nil || dir == "" {
		return ""
	}
	// Save as new default
	d.prefs.CloneDir = dir
	_ = d.prefs.Save()
	return dir
}

// CloneProject clones a git repository and scans for rela projects.
// Returns a JSON response with status and any discovered projects.
func (d *Desktop) CloneProject(repoURL, baseDir string) map[string]interface{} {
	if !git.IsValidRepoURL(repoURL) {
		return map[string]interface{}{"error": "Invalid repository URL. Use HTTPS format: https://github.com/user/repo"}
	}

	// Use saved token if available
	token := d.prefs.GitHubToken

	// Determine target directory
	repoName := git.ExtractRepoName(repoURL)
	if repoName == "" {
		return map[string]interface{}{"error": "Could not determine repository name from URL"}
	}
	if baseDir == "" {
		baseDir = d.GetDefaultCloneDir()
	}
	targetDir := filepath.Join(baseDir, repoName)

	// Store clone dir for later use
	d.mu.Lock()
	d.lastCloneDir = targetDir
	d.mu.Unlock()

	err := git.Clone(git.CloneOptions{
		URL:   repoURL,
		Path:  targetDir,
		Token: token,
	})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Clone failed: %v", err)}
	}

	// Scan for rela projects (directories containing metamodel.yaml)
	projects := scanForRelaProjects(targetDir)

	if len(projects) == 0 {
		return map[string]interface{}{
			"status":    "no_projects",
			"clone_dir": targetDir,
		}
	}

	if len(projects) == 1 {
		// Single project found - open it directly
		if errMsg := d.LoadProject(projects[0]); errMsg != "" {
			return map[string]interface{}{"error": errMsg}
		}
		return map[string]interface{}{"status": "opened"}
	}

	// Multiple projects found - return list for user to pick
	// Convert to relative paths for display
	relPaths := make([]string, len(projects))
	for i, p := range projects {
		rel, _ := filepath.Rel(targetDir, p)
		if rel == "." {
			rel = "(root)"
		}
		relPaths[i] = rel
	}

	return map[string]interface{}{
		"status":    "multiple",
		"projects":  relPaths,
		"clone_dir": targetDir,
	}
}

// OpenClonedProject opens a specific project from a recently cloned repository.
func (d *Desktop) OpenClonedProject(subfolder string) string {
	d.mu.RLock()
	cloneDir := d.lastCloneDir
	d.mu.RUnlock()

	if cloneDir == "" {
		return "No recent clone. Please clone a repository first."
	}

	projectDir := cloneDir
	if subfolder != "" && subfolder != "(root)" {
		projectDir = filepath.Join(cloneDir, subfolder)
	}

	return d.LoadProject(projectDir)
}

// InitRelaProject initializes a new rela project in the cloned repository.
func (d *Desktop) InitRelaProject(subfolder string) string {
	d.mu.RLock()
	cloneDir := d.lastCloneDir
	d.mu.RUnlock()

	if cloneDir == "" {
		return "No recent clone. Please clone a repository first."
	}

	projectDir := cloneDir
	if subfolder != "" {
		projectDir = filepath.Join(cloneDir, subfolder)
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			return fmt.Sprintf("Failed to create directory: %v", err)
		}
	}

	// Create minimal metamodel.yaml
	metamodelPath := filepath.Join(projectDir, "metamodel.yaml")
	minimalMetamodel := `# Rela Project Configuration
# See https://github.com/Sourcehaven-BV/rela for documentation

entity_types: {}
relation_types: {}
`
	if err := os.WriteFile(metamodelPath, []byte(minimalMetamodel), 0o644); err != nil {
		return fmt.Sprintf("Failed to create metamodel.yaml: %v", err)
	}

	// Create entities directory
	entitiesDir := filepath.Join(projectDir, "entities")
	if err := os.MkdirAll(entitiesDir, 0o755); err != nil {
		return fmt.Sprintf("Failed to create entities directory: %v", err)
	}

	return d.LoadProject(projectDir)
}

// scanForRelaProjects recursively finds directories containing metamodel.yaml.
func scanForRelaProjects(root string) []string {
	var projects []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil {
			return filepath.SkipDir
		}
		// Skip hidden directories and common non-project dirs
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "metamodel.yaml" {
			projects = append(projects, filepath.Dir(path))
		}
		return nil
	})
	return projects
}

// StartGitHubAuth initiates the GitHub OAuth device flow.
// Returns the user code and verification URL, or an error string.
func (d *Desktop) StartGitHubAuth() map[string]string {
	clientID := GitHubClientID
	if clientID == "" {
		clientID = os.Getenv("RELA_GITHUB_CLIENT_ID")
	}
	if clientID == "" {
		return map[string]string{"error": "GitHub OAuth not configured. Set RELA_GITHUB_CLIENT_ID environment variable."}
	}

	oauth := git.NewOAuth(git.OAuthConfig{ClientID: clientID})
	resp, err := oauth.RequestDeviceCode(context.Background())
	if err != nil {
		return map[string]string{"error": fmt.Sprintf("Failed to start auth: %v", err)}
	}

	d.mu.Lock()
	d.cloneAuth = &cloneAuthState{
		deviceCode: resp.DeviceCode,
		userCode:   resp.UserCode,
		verifyURL:  resp.VerificationURI,
		expiresIn:  resp.ExpiresIn,
		interval:   resp.Interval,
	}
	d.mu.Unlock()

	return map[string]string{
		"user_code":        resp.UserCode,
		"verification_url": resp.VerificationURI,
	}
}

// CompleteGitHubAuth waits for the user to authorize and stores the token.
// Returns empty string on success, error message on failure.
func (d *Desktop) CompleteGitHubAuth() string {
	d.mu.RLock()
	auth := d.cloneAuth
	d.mu.RUnlock()

	if auth == nil {
		return "No auth in progress. Call StartGitHubAuth first."
	}

	clientID := GitHubClientID
	if clientID == "" {
		clientID = os.Getenv("RELA_GITHUB_CLIENT_ID")
	}

	oauth := git.NewOAuth(git.OAuthConfig{ClientID: clientID})

	const authTimeout = 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
	defer cancel()

	token, err := oauth.WaitForAuthorization(ctx, auth.deviceCode, auth.interval)
	if err != nil {
		if errors.Is(err, git.ErrAccessDenied) {
			return "Authorization denied by user"
		}
		if errors.Is(err, git.ErrExpiredToken) {
			return "Authorization expired. Please try again."
		}
		return fmt.Sprintf("Authorization failed: %v", err)
	}

	// Store token
	d.prefs.GitHubToken = token.AccessToken
	if err := d.prefs.Save(); err != nil {
		slog.Warn("could not save token", "error", err)
	}

	d.mu.Lock()
	d.cloneAuth = nil
	d.mu.Unlock()

	return ""
}

// HasGitHubToken returns true if a GitHub token is stored.
func (d *Desktop) HasGitHubToken() bool {
	return d.prefs.GitHubToken != ""
}

// ClearGitHubToken removes the stored GitHub token.
func (d *Desktop) ClearGitHubToken() {
	d.prefs.GitHubToken = ""
	_ = d.prefs.Save()
}

// ServeHTTP dispatches to the loaded app router or the welcome page.
func (d *Desktop) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	h := d.handler
	loadErr := d.loadErr
	d.mu.RUnlock()

	if h != nil {
		h.ServeHTTP(w, r)
		return
	}

	serveWelcomePage(w, d.prefs, loadErr)
}

// openProjectFromMenu handles File > Open Project from the native menu bar.
// coverage-ignore: menu callback - requires Wails runtime
func (d *Desktop) openProjectFromMenu(_ *menu.CallbackData) {
	dir, err := runtime.OpenDirectoryDialog(d.ctx, runtime.OpenDialogOptions{
		Title: "Open Rela Project",
	})
	if err != nil || dir == "" {
		return
	}
	if errMsg := d.LoadProject(dir); errMsg != "" {
		runtime.MessageDialog(d.ctx, runtime.MessageDialogOptions{ //nolint:errcheck // best-effort
			Type:    runtime.ErrorDialog,
			Title:   "Failed to open project",
			Message: errMsg,
		})
		return
	}
	runtime.WindowReloadApp(d.ctx)
}

// cloneFromGitMenu handles File > Clone from Git from the native menu bar.
// It navigates to the welcome page and triggers the clone dialog.
// coverage-ignore: menu callback - requires Wails runtime
func (d *Desktop) cloneFromGitMenu(_ *menu.CallbackData) {
	// Unload current project to show welcome page
	d.mu.Lock()
	d.handler = nil
	d.loadErr = ""
	d.mu.Unlock()

	// Reload app to show welcome page, then emit event to show clone dialog
	runtime.WindowReloadApp(d.ctx)

	// Give the page time to load before emitting the event
	go func() {
		time.Sleep(100 * time.Millisecond)
		runtime.EventsEmit(d.ctx, "show-clone-dialog")
	}()
}

// showAbout displays a dialog with version and build information.
// coverage-ignore: menu callback - requires Wails runtime
func (d *Desktop) showAbout(_ *menu.CallbackData) {
	runtime.MessageDialog(d.ctx, runtime.MessageDialogOptions{ //nolint:errcheck // best-effort
		Type:    runtime.InfoDialog,
		Title:   "About Rela Desktop",
		Message: fmt.Sprintf("Rela Desktop\nVersion %s\n\n%s/%s", Version, goruntime.GOOS, goruntime.GOARCH),
	})
}

// buildAppMenu constructs the application menu bar including recent projects.
func (d *Desktop) buildAppMenu() *menu.Menu {
	appMenu := menu.NewMenu()

	if goruntime.GOOS == "darwin" {
		macAppMenu := appMenu.AddSubmenu("Rela Desktop")
		macAppMenu.AddText("About Rela Desktop", nil, d.showAbout)
		macAppMenu.AddSeparator()
		macAppMenu.Append(menu.AppMenu())
	}

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Open Project...", keys.CmdOrCtrl("o"), d.openProjectFromMenu)
	fileMenu.AddText("Clone from Git...", keys.CmdOrCtrl("shift+o"), d.cloneFromGitMenu)
	fileMenu.AddSeparator()

	// Recent Projects submenu
	if len(d.prefs.RecentProjects) > 0 {
		recentMenu := fileMenu.AddSubmenu("Recent Projects")
		for _, rp := range d.prefs.RecentProjects {
			proj := rp // capture for closure
			label := proj.Name
			if label == "" {
				label = filepath.Base(proj.Path)
			}
			recentMenu.AddText(label, nil, func(_ *menu.CallbackData) {
				if errMsg := d.LoadProject(proj.Path); errMsg != "" {
					runtime.MessageDialog(d.ctx, runtime.MessageDialogOptions{ //nolint:errcheck // best-effort
						Type:    runtime.ErrorDialog,
						Title:   "Failed to open project",
						Message: errMsg,
					})
					return
				}
				runtime.WindowReloadApp(d.ctx)
			})
		}
		recentMenu.AddSeparator()
		recentMenu.AddText("Clear Recent Projects", nil, func(_ *menu.CallbackData) {
			d.prefs.ClearRecentProjects()
			if err := d.prefs.Save(); err != nil {
				slog.Warn("could not save preferences", "error", err)
			}
			d.refreshMenu()
		})
		fileMenu.AddSeparator()
	}

	if goruntime.GOOS != "darwin" {
		fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
			runtime.Quit(d.ctx)
		})
	} else {
		fileMenu.AddText("Close Window", keys.CmdOrCtrl("w"), func(_ *menu.CallbackData) {
			runtime.Quit(d.ctx)
		})
	}

	if goruntime.GOOS == "darwin" {
		appMenu.Append(menu.EditMenu())
	}

	if goruntime.GOOS != "darwin" {
		helpMenu := appMenu.AddSubmenu("Help")
		helpMenu.AddText("About Rela Desktop", nil, d.showAbout)
	}

	return appMenu
}

// refreshMenu rebuilds and applies the application menu.
func (d *Desktop) refreshMenu() {
	if d.ctx == nil {
		return
	}
	m := d.buildAppMenu()
	runtime.MenuSetApplicationMenu(d.ctx, m)
	runtime.MenuUpdateApplicationMenu(d.ctx)
}

// coverage-ignore: main function - entry point
func main() {
	projectDir := flag.String("project", ".", "Path to the rela project directory")
	verbose := flag.Bool("verbose", false, "Verbose (debug) logging")
	quiet := flag.Bool("quiet", false, "Quiet (warn-only) logging")
	flag.Parse()

	configureLogging(*verbose, *quiet)

	// Fail fast if the embedded SPA is missing (BUG-W144 class regression).
	if err := dataentry.CheckEmbeddedSPA(); err != nil {
		slog.Error("embedded SPA check failed", "error", err)
		os.Exit(1)
	}

	// Load desktop preferences.
	prefs, err := desktop.Load()
	if err != nil {
		slog.Warn("could not load preferences", "error", err)
		prefs = &desktop.Preferences{}
	}

	d := &Desktop{prefs: prefs}

	// Determine which project to open.
	projectToLoad := resolveProjectDir(*projectDir, prefs)
	if projectToLoad != "" {
		if errMsg := d.LoadProject(projectToLoad); errMsg != "" {
			slog.Warn("could not load project", "path", projectToLoad, "error", errMsg)
		}
	}

	title := "Rela Desktop"
	if d.app != nil {
		title = d.app.ProjectName()
	}

	wailsErr := wails.Run(&options.App{
		Title:  title,
		Width:  1280,
		Height: 800,
		Menu:   d.buildAppMenu(),
		AssetServer: &assetserver.Options{
			Handler: d,
		},
		OnStartup: d.startup,
		Bind:      []interface{}{d},
	})
	if wailsErr != nil {
		slog.Error("wails error", "error", wailsErr)
		os.Exit(1)
	}
}

// configureLogging sets the default slog logger based on verbose/quiet flags.
func configureLogging(verbose, quiet bool) {
	level := slog.LevelInfo
	switch {
	case verbose:
		level = slog.LevelDebug
	case quiet:
		level = slog.LevelWarn
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

// resolveProjectDir determines which project directory to load at startup.
// Priority: explicit -project flag > current dir if rela project > last project from preferences.
func resolveProjectDir(flagValue string, prefs *desktop.Preferences) string {
	// Explicit flag or current directory is a rela project.
	if flagValue != "." || isRelaProject(flagValue) {
		return flagValue
	}
	// Fall back to last project from preferences.
	if prefs.LastProject != "" && isRelaProject(prefs.LastProject) {
		return prefs.LastProject
	}
	return ""
}

// isRelaProject checks if the directory looks like a rela project.
func isRelaProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "metamodel.yaml"))
	return err == nil
}

// discoverProject returns the filesystem and project context for the
// project rooted at projectDir.
func discoverProject(projectDir string) (storage.FS, *project.Context, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, nil, err
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	projCtx, err := project.Discover(absDir, fs)
	if err != nil {
		return nil, nil, fmt.Errorf("discovering project: %w", err)
	}
	return fs, projCtx, nil
}

// generateDataEntryConfig creates a minimal data-entry.yaml from the metamodel.
// Builds a typed structure and marshals via yaml.v3 so every string value is
// correctly escaped regardless of the characters a user's metamodel contains.
func generateDataEntryConfig(appName string, meta *metamodel.Metamodel) string {
	entityTypes := make([]string, 0, len(meta.Entities))
	for name := range meta.Entities {
		entityTypes = append(entityTypes, name)
	}
	sort.Strings(entityTypes)

	forms := yaml.Node{Kind: yaml.MappingNode}
	lists := yaml.Node{Kind: yaml.MappingNode}
	navigation := yaml.Node{Kind: yaml.SequenceNode}

	const maxColumns = 4
	for _, typeName := range entityTypes {
		entDef := meta.Entities[typeName]
		formID := strings.ReplaceAll(typeName, "-", "_")
		listID := formID + "s"

		propNames := make([]string, 0, len(entDef.Properties))
		for name := range entDef.Properties {
			propNames = append(propNames, name)
		}
		sort.Strings(propNames)

		fields := make([]map[string]string, 0, len(propNames))
		for _, propName := range propNames {
			fields = append(fields, map[string]string{"property": propName, "label": titleCase(propName)})
		}
		appendMapEntry(&forms, formID, map[string]any{
			"entity_type": typeName,
			"title":       titleCase(typeName),
			"fields":      fields,
		})

		columns := make([]map[string]string, 0, maxColumns)
		for i, propName := range propNames {
			if i >= maxColumns {
				break
			}
			columns = append(columns, map[string]string{"property": propName, "label": titleCase(propName)})
		}
		appendMapEntry(&lists, listID, map[string]any{
			"entity_type": typeName,
			"title":       titleCase(typeName) + "s",
			"columns":     columns,
			"create_form": formID,
			"edit_form":   formID,
		})

		navItem := yaml.Node{Kind: yaml.MappingNode}
		appendMapEntry(&navItem, "label", titleCase(typeName)+"s")
		appendMapEntry(&navItem, "list", listID)
		navigation.Content = append(navigation.Content, &navItem)
	}

	root := yaml.Node{Kind: yaml.MappingNode}
	appendMapEntry(&root, "version", "1")
	appendMapEntry(&root, "app", map[string]any{
		"name":        appName,
		"description": "Generated data entry configuration",
	})
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "forms"}, &forms,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "lists"}, &lists,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "navigation"}, &navigation,
	)

	out, err := yaml.Marshal(&root)
	if err != nil {
		// yaml.Marshal on a manually-built node graph shouldn't fail; surface
		// as a comment so the generated file is still valid YAML.
		return fmt.Sprintf("# failed to generate config: %v\n", err)
	}
	return string(out)
}

// appendMapEntry adds a key/value pair to a yaml MappingNode. Value may be any
// Go value yaml.Marshal accepts (string, map, slice, etc.) or a pre-built
// *yaml.Node.
func appendMapEntry(m *yaml.Node, key string, value any) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	var valNode *yaml.Node
	if n, ok := value.(*yaml.Node); ok {
		valNode = n
	} else {
		valNode = &yaml.Node{}
		_ = valNode.Encode(value)
	}
	m.Content = append(m.Content, keyNode, valNode)
}

// titleCase converts a-kebab-case or snake_case to Title Case.
func titleCase(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
