package dataentry

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/git"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/openapi"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// AppState bundles the reloadable fields of App into an immutable snapshot.
//
// During this PR, App still holds the same fields as plain struct fields
// (Cfg, meta, g, styleMap, styledTypes, userDefaults, palette, userPalette,
// openAPIGen) and the AppState is published in parallel via atomic.Pointer.
// Handlers will be migrated to read from the snapshot in a subsequent PR.
//
// The fields here mirror the workspace.workspaceState pattern: callers
// Load() once and work against a coherent snapshot, instead of holding a
// read lock around the entire request.
type AppState struct {
	Cfg          *Config
	Meta         *metamodel.Metamodel
	Graph        EntityGraph
	StyleMap     map[string]map[string]string
	StyledTypes  map[string]bool
	UserDefaults *UserDefaults
	Palette      *ResolvedPalette
	UserPalette  *PaletteConfig
	OpenAPIGen   *openapi.Generator
}

// ConfigFile is the conventional filename for data-entry configuration within a rela project.
const ConfigFile = dataentryconfig.ConfigFile

// uiStateFile is the filename for persisted UI state within the .rela directory.
const uiStateFile = "ui-state.json"

// userDefaultsFile is the filename for user-specific default values within the .rela directory.
const userDefaultsFile = "user-defaults.yaml"

// userPaletteFile is the filename for user-specific palette overrides within the .rela directory.
const userPaletteFile = "palette.yaml"

// App is the central application struct for the data-entry server.
//
// # Concurrency model
//
// All reloadable state (config, metamodel, graph, style map, palette,
// user defaults, OpenAPI generator) lives in an immutable AppState struct
// held via atomic.Pointer. Handlers call a.State() once at entry and work
// against a coherent snapshot for the duration of the request — no lock
// acquisition, no risk of observing a half-reloaded world.
//
// Reloads (triggered by the file watcher or by Reload) build a new
// AppState and publish it atomically via a.state.Store. The previous
// state is garbage-collected once no reader holds it.
//
// Mutations (CreateEntity, UpdateEntity, DeleteEntity, CreateRelation,
// UpdateRelation, DeleteRelation, SetProperty, action scripts) serialize
// via writeMu. writeMu excludes concurrent mutations but does NOT block
// readers — readers go through state.Load(). The workspace's internal
// reloadMu coordinates the reload itself with the mutation path.
type App struct {
	ws *workspace.Workspace

	// state holds the current reloadable snapshot. Readers: a.State().
	// Writers: onReload rebuilds and publishes a new state after file
	// changes. Initial state is published in NewApp.
	state atomic.Pointer[AppState]

	// writeMu serializes mutation handlers (CreateEntity, UpdateEntity,
	// etc.) against each other and against the reload path inside
	// Workspace. Readers never take it.
	writeMu sync.Mutex

	// gitOps provides git operations when git is enabled. Set once in
	// NewApp; never reloaded.
	gitOps *git.Ops

	// broker delivers SSE events to connected browsers for live-reload.
	broker *eventBroker

	// security holds the configured Host/Origin allowlists. Set via
	// SetSecurityConfig before NewRouter; nil disables the middlewares
	// (only sensible in unit tests where no HTTP layer is exercised).
	security *security
}

// State returns the current reloadable snapshot. Handlers should call
// State() once at entry and use the returned snapshot consistently
// throughout, instead of making multiple calls that could see different
// snapshots after a concurrent reload.
func (a *App) State() *AppState { return a.state.Load() }

// Cfg returns the current data-entry config (convenience accessor).
// Equivalent to a.State().Cfg.
func (a *App) Cfg() *Config { return a.State().Cfg }

// Meta returns the current metamodel (convenience accessor).
func (a *App) Meta() *metamodel.Metamodel { return a.State().Meta }

// Graph returns the current in-memory graph (convenience accessor).
func (a *App) Graph() EntityGraph { return a.State().Graph }

// mutateState atomically updates the published AppState. It takes
// writeMu, builds a shallow copy of the current snapshot, runs the
// caller's mutator on the copy, and publishes the copy via state.Store.
//
// This is the canonical way for mutation handlers to change reloadable
// fields like UserDefaults or UserPalette. Reaching through a.State()
// to assign field values directly is a bug — it scribbles on the shared
// snapshot pointer that lock-free readers also hold.
func (a *App) mutateState(fn func(*AppState)) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	cur := a.state.Load()
	next := *cur // shallow copy of the snapshot
	fn(&next)
	a.state.Store(&next)
}

// SetSecurityConfig configures the HTTP security middlewares applied by
// NewRouter. It must be called before NewRouter.
func (a *App) SetSecurityConfig(cfg SecurityConfig) error {
	s, err := newSecurity(cfg)
	if err != nil {
		return err
	}
	a.security = s
	return nil
}

// NewApp creates and initializes an App using the given workspace.
func NewApp(ws *workspace.Workspace) (*App, error) {
	// Load data-entry config from project root
	cfgData, err := ws.ReadProjectFile(ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	// Check for deprecated syntax that needs migration
	configPath := filepath.Join(ws.Paths().Root, ConfigFile)
	detections := migration.DetectBytes(cfgData, migration.FileTypeDataEntry)
	if len(detections) > 0 {
		return nil, &migration.Error{
			FilePath:   configPath,
			Detections: detections,
		}
	}

	var cfg Config
	if unmarshalErr := yaml.Unmarshal(cfgData, &cfg); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", ConfigFile, unmarshalErr)
	}

	snap := ws.Snapshot()
	meta := snap.Meta()
	g := snap.Graph()

	// Validate config against metamodel
	if validationErr := ValidateConfig(cfgData, &cfg, meta); validationErr != nil {
		return nil, fmt.Errorf("invalid %s: %w", ConfigFile, validationErr)
	}

	// Verify action scripts exist on disk (catches typos at startup).
	// Skip set-only actions which have no script.
	for id, action := range cfg.Actions {
		if action.Script == "" {
			continue
		}
		if err := script.CheckActionScriptExists(ws.Paths().Root, action.Script); err != nil {
			return nil, fmt.Errorf("invalid %s: action %q: %w", ConfigFile, id, err)
		}
	}

	slog.Info("loaded project graph", "entities", g.NodeCount(), "relations", g.EdgeCount())

	// Build style map from config styles
	styleMap, styledTypes := buildStyleMap(&cfg, meta)

	app := &App{
		ws:     ws,
		broker: newEventBroker(),
	}

	userDefaults := app.loadUserDefaults()
	userPalette, paletteErr := app.loadUserPalette()
	if paletteErr != nil {
		// Surface the error so users notice their palette is broken
		// rather than silently falling back to defaults (which would
		// then be persisted on the next save, destroying their data).
		return nil, fmt.Errorf("load user palette: %w", paletteErr)
	}

	// Build and publish the initial AppState snapshot. All reloadable
	// state lives here; there are no convenience aliases on App to keep
	// in sync.
	app.state.Store(&AppState{
		Cfg:          &cfg,
		Meta:         meta,
		Graph:        g,
		StyleMap:     styleMap,
		StyledTypes:  styledTypes,
		UserDefaults: userDefaults,
		Palette:      ResolvePalette(cfg.Palette, userPalette),
		UserPalette:  userPalette,
		OpenAPIGen: openapi.New(meta, openapi.Config{
			Title:       cfg.App.Name + " API",
			Description: cfg.App.Description,
			Version:     "1.0.0",
		}),
	})

	// Initialize git ops if enabled and repo is a git repository
	if cfg.Git != nil && cfg.Git.Enabled && git.IsRepo(ws.Paths().Root) {
		app.gitOps = git.NewOps(ws.Paths().Root, *cfg.Git)
		slog.Info("git sync enabled", "mode", cfg.Git.Mode)
	}

	return app, nil
}

// NavItem is an enriched navigation entry that includes the entity type for client-side matching.
type NavItem struct {
	Label      string
	List       string
	Dashboard  bool
	Kanban     string
	EntityType string
	Count      int
}

// NavGroup is an enriched navigation group containing resolved nav items.
type NavGroup struct {
	Group     string
	Collapsed bool
	Items     []NavItem
}

// NavElement is a union of either a direct NavItem or a NavGroup.
// Exactly one of Item or Group is non-nil.
type NavElement struct {
	Item  *NavItem
	Group *NavGroup
}

// enrichNavEntry resolves a single NavigationEntry into a NavItem with entity type and count.
func (a *App) enrichNavEntry(nav NavigationEntry) NavItem {
	item := NavItem{Label: nav.Label, List: nav.List, Dashboard: nav.Dashboard, Kanban: nav.Kanban}
	if nav.Dashboard || nav.Kanban != "" {
		return item
	}
	s := a.State()
	if list, ok := s.Cfg.Lists[nav.List]; ok {
		item.EntityType = list.EntityType
		entities := s.Graph.NodesByType(list.EntityType)
		entities = applyFilters(entities, list.Filters)
		item.Count = len(entities)
	}
	return item
}

// navElements returns the navigation structure with groups and items resolved.
// The activeList parameter is used to auto-expand the group containing the active item.
func (a *App) navElements(activeList string) []NavElement {
	uiState := a.loadUIState()
	cfgNav := a.State().Cfg.Navigation
	elements := make([]NavElement, 0, len(cfgNav))
	for _, nav := range cfgNav {
		if nav.IsGroup() {
			grp := NavGroup{Group: nav.Group}
			// Determine collapsed state: UIState overrides config default
			if override, ok := uiState.CollapsedGroups[nav.Group]; ok {
				grp.Collapsed = override
			} else {
				grp.Collapsed = nav.Collapsed
			}
			grp.Items = make([]NavItem, len(nav.Items))
			for i, child := range nav.Items {
				grp.Items[i] = a.enrichNavEntry(child)
				// Auto-expand group if it contains the active list
				if child.List == activeList && activeList != "" {
					grp.Collapsed = false
				}
			}
			elements = append(elements, NavElement{Group: &grp})
		} else {
			item := a.enrichNavEntry(nav)
			elements = append(elements, NavElement{Item: &item})
		}
	}
	return elements
}

// loadUIState reads .rela/ui-state.json and returns the persisted state.
// Returns an empty UIState if the file doesn't exist or can't be parsed.
func (a *App) loadUIState() UIState {
	state := UIState{CollapsedGroups: make(map[string]bool)}
	if a.ws == nil {
		return state
	}
	data, err := a.ws.ReadCacheFile(uiStateFile)
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return UIState{CollapsedGroups: make(map[string]bool)}
	}
	if state.CollapsedGroups == nil {
		state.CollapsedGroups = make(map[string]bool)
	}
	return state
}

// saveUIState writes the UI state to .rela/ui-state.json.
func (a *App) saveUIState(state UIState) error {
	if a.ws == nil {
		return nil
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return a.ws.WriteCacheFile(uiStateFile, data)
}

// loadUserDefaults reads .rela/user-defaults.yaml and returns the parsed defaults.
// Returns nil if the file doesn't exist or can't be parsed.
func (a *App) loadUserDefaults() *UserDefaults {
	if a.ws == nil {
		return nil
	}
	data, err := a.ws.ReadCacheFile(userDefaultsFile)
	if err != nil {
		return nil
	}
	var ud UserDefaults
	if err := yaml.Unmarshal(data, &ud); err != nil {
		return nil
	}
	return &ud
}

// saveUserDefaults writes the user defaults to .rela/user-defaults.yaml.
func (a *App) saveUserDefaults(ud *UserDefaults) error {
	if a.ws == nil {
		return nil
	}
	data, err := yaml.Marshal(ud)
	if err != nil {
		return err
	}
	return a.ws.WriteCacheFile(userDefaultsFile, data)
}

// coverage-ignore: requires running workspace, tested via e2e

// loadUserPalette reads .rela/palette.yaml and returns the parsed
// palette. Returns (nil, nil) when the file does not exist (clean
// "no user palette" state — matches how ResolvePalette consumes a
// nil user palette pointer; a sentinel error or three-return shape
// would be more confusing for the only two callers). Returns a
// non-nil error if the file exists but cannot be read or parsed —
// callers MUST surface this instead of silently falling back to
// defaults, otherwise a subsequent save would silently overwrite
// the user's palette with framework defaults (RR-OA4A).
//
//nolint:nilnil // see comment above
func (a *App) loadUserPalette() (*PaletteConfig, error) {
	if a.ws == nil {
		return nil, nil
	}
	data, err := a.ws.ReadCacheFile(userPaletteFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", userPaletteFile, err)
	}
	var p PaletteConfig
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w (legacy `dark: auto` is no longer supported — remove the `dark` line or set it to `false` or an explicit object)", userPaletteFile, err)
	}
	return &p, nil
}

// saveUserPalette writes the user palette to .rela/palette.yaml.
func (a *App) saveUserPalette(p *PaletteConfig) error {
	if a.ws == nil {
		return nil
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return a.ws.WriteCacheFile(userPaletteFile, data)
}

// firstNavTarget returns the first navigable item from the navigation config,
// walking into groups as needed.
func firstNavTarget(nav []NavigationEntry) *NavigationEntry {
	for i := range nav {
		if nav[i].IsGroup() {
			if target := firstNavTarget(nav[i].Items); target != nil {
				return target
			}
			continue
		}
		return &nav[i]
	}
	return nil
}

// editFormForType returns the first edit form ID configured for the given entity type,
// or "" if no edit form is found. Forms with explicit mode="edit" are preferred.
func (a *App) editFormForType(entityType string) string {
	s := a.State()
	ids := make([]string, 0, len(s.Cfg.Forms))
	for id := range s.Cfg.Forms {
		ids = append(ids, id)
	}
	natsort.Strings(ids)
	// First pass: look for explicit edit mode
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType == entityType && f.Mode == "edit" {
			return id
		}
	}
	// Second pass: fall back to forms with no mode specified
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType == entityType && f.Mode == "" {
			return id
		}
	}
	return ""
}

// createFormForType returns the first form ID that can be used to create an entity
// of the given type. It prefers forms with mode "create" or unset, but falls back
// to edit-mode forms (which work for creation when no entity ID is provided).
func (a *App) createFormForType(entityType string) string {
	s := a.State()
	ids := make([]string, 0, len(s.Cfg.Forms))
	for id := range s.Cfg.Forms {
		ids = append(ids, id)
	}
	natsort.Strings(ids)
	fallback := ""
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType != entityType {
			continue
		}
		if f.Mode != "edit" {
			return id
		}
		if fallback == "" {
			fallback = id
		}
	}
	return fallback
}


// resolveLinkTarget resolves a link configuration value to a URL.
// Supported values:
//   - "" or empty: no link (returns "")
//   - "detail": link to entity detail view (/entity/{type}/{id})
//   - "document/<name>": link to document preview (/document/<name>/{id})
func (a *App) resolveLinkTarget(link, entityType, entityID string) string {
	switch {
	case link == "":
		return ""
	case link == "detail":
		return "/entity/" + entityType + "/" + entityID
	case strings.HasPrefix(link, "document/"):
		docName := strings.TrimPrefix(link, "document/")
		return "/document/" + docName + "/" + entityID
	default:
		return ""
	}
}

// activeListForEntityType returns the first navigation list ID whose entity type
// matches the given type, or "" if none match. Walks into groups.
func (a *App) activeListForEntityType(entityType string) string {
	s := a.State()
	return a.findListByEntityType(s, s.Cfg.Navigation, entityType)
}

func (a *App) findListByEntityType(s *AppState, entries []NavigationEntry, entityType string) string {
	for _, nav := range entries {
		if nav.IsGroup() {
			if found := a.findListByEntityType(s, nav.Items, entityType); found != "" {
				return found
			}
			continue
		}
		if list, ok := s.Cfg.Lists[nav.List]; ok && list.EntityType == entityType {
			return nav.List
		}
	}
	return ""
}

// activeListFromReferer extracts a list ID from the Referer header path
// (e.g. "/list/tickets" -> "tickets"). Returns "" if the referer doesn't
// point to a known list.
func (a *App) activeListFromReferer(r *http.Request) string {
	ref := r.Header.Get("Referer")
	if ref == "" {
		return ""
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	path := parsed.Path
	if !strings.HasPrefix(path, "/list/") {
		return ""
	}
	listID := strings.TrimPrefix(path, "/list/")
	if _, ok := a.State().Cfg.Lists[listID]; ok {
		return listID
	}
	return ""
}

// resolveActiveList returns the best active list for the sidebar.
// It first checks for an explicit "from" query parameter (set when navigating
// from a list), then tries matching by entity type, then falls back to the
// Referer header.
func (a *App) resolveActiveList(entityType string, r *http.Request) string {
	if from := r.URL.Query().Get("from"); from != "" {
		if _, ok := a.State().Cfg.Lists[from]; ok {
			return from
		}
	}
	if active := a.activeListForEntityType(entityType); active != "" {
		return active
	}
	return a.activeListFromReferer(r)
}

// ProjectName returns the display name of the loaded project.
func (a *App) ProjectName() string {
	return a.State().Cfg.App.Name
}

// ProjectRoot returns the root directory of the loaded project.
func (a *App) ProjectRoot() string {
	return a.ws.Paths().Root
}

// colorToCSSClass maps a color name from config to a CSS class.
var colorToCSSClass = map[string]string{
	"blue":   "badge-blue",
	"purple": "badge-purple",
	"green":  "badge-green",
	"gray":   "badge-gray",
	"red":    "badge-red",
	"orange": "badge-orange",
	"yellow": "badge-yellow",
}

// autoColors assigns colors to enum values that have no explicit style.
var autoColors = []string{"blue", "purple", "green", "orange", "yellow", "red", "gray"}

func buildStyleMap(cfg *Config, meta *metamodel.Metamodel) (styleMap map[string]map[string]string, styledTypes map[string]bool) {
	sm := make(map[string]map[string]string)
	st := make(map[string]bool)

	// Populate from explicit config styles
	for typeName, valueColors := range cfg.Styles {
		sm[typeName] = make(map[string]string)
		st[typeName] = true
		for val, color := range valueColors {
			if cls, ok := colorToCSSClass[color]; ok {
				sm[typeName][val] = cls
			} else {
				sm[typeName][val] = "badge-gray"
			}
		}
	}

	// Auto-assign styles for custom types not already styled
	for typeName, ct := range meta.Types {
		if _, alreadyStyled := sm[typeName]; alreadyStyled {
			continue
		}
		sm[typeName] = make(map[string]string)
		st[typeName] = true
		for i, val := range ct.Values {
			sm[typeName][val] = colorToCSSClass[autoColors[i%len(autoColors)]]
		}
	}

	return sm, st
}

