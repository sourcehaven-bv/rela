package dataentry

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/git"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/repository"
)

// ConfigFile is the conventional filename for data-entry configuration within a rela project.
const ConfigFile = dataentryconfig.ConfigFile

// uiStateFile is the filename for persisted UI state within the .rela directory.
const uiStateFile = "ui-state.json"

// userDefaultsFile is the filename for user-specific default values within the .rela directory.
const userDefaultsFile = "user-defaults.yaml"

// App is the central application struct holding config, metamodel, graph, and templates.
type App struct {
	Cfg  *Config
	meta *metamodel.Metamodel
	g    *graph.Graph
	repo repository.Store
	tmpl *template.Template
	// styleMap: property type name -> value -> CSS class name
	styleMap map[string]map[string]string
	// styledTypes: set of property type names that have style entries
	styledTypes map[string]bool
	// userDefaults holds the loaded user defaults (nil if not yet loaded or no file).
	userDefaults *UserDefaults
	// gitOps provides git operations when git is enabled.
	gitOps *git.Ops
	// automationEngine processes automation rules.
	automationEngine *automation.Engine
	// mu protects reloadable state (Cfg, meta, g, tmpl, styleMap, styledTypes)
	// during live-reload. Handlers acquire RLock; reload acquires Lock.
	mu sync.RWMutex
	// broker delivers SSE events to connected browsers for live-reload.
	broker *eventBroker
}

// NewApp creates and initializes an App using the given Store.
func NewApp(repo repository.Store) (*App, error) {
	// Load data-entry config from project root
	cfgData, err := repo.ReadProjectFile(ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	// Check for deprecated syntax that needs migration
	configPath := filepath.Join(repo.Paths().Root, ConfigFile)
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

	// Load metamodel
	meta, err := repo.LoadMetamodel()
	if err != nil {
		return nil, fmt.Errorf("loading metamodel: %w", err)
	}

	// Validate config against metamodel
	if validationErr := ValidateConfig(cfgData, &cfg, meta); validationErr != nil {
		return nil, fmt.Errorf("invalid %s: %w", ConfigFile, validationErr)
	}

	// Build graph from files
	g := graph.New()
	result, err := repo.Sync(meta, g)
	if err != nil {
		return nil, fmt.Errorf("syncing graph: %w", err)
	}
	log.Printf("Loaded %d entities and %d relations", result.EntitiesLoaded, result.RelationsLoaded)

	// Build style map from config styles
	styleMap, styledTypes := buildStyleMap(&cfg, meta)

	// Parse templates with style-aware funcs
	tmpl, err := template.New("").Funcs(templateFuncs(styleMap, styledTypes)).Parse(allTemplates())
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	tmpl, err = tmpl.Parse(graphTemplates)
	if err != nil {
		return nil, fmt.Errorf("parsing graph templates: %w", err)
	}
	app := &App{
		Cfg:              &cfg,
		meta:             meta,
		g:                g,
		repo:             repo,
		tmpl:             tmpl,
		styleMap:         styleMap,
		styledTypes:      styledTypes,
		automationEngine: automation.NewEngineFromMetamodel(meta.Automations),
		broker:           newEventBroker(),
	}
	app.userDefaults = app.loadUserDefaults()

	// Initialize git ops if enabled and repo is a git repository
	if cfg.Git != nil && cfg.Git.Enabled && git.IsRepo(repo.Paths().Root) {
		app.gitOps = git.NewOps(repo.Paths().Root, *cfg.Git)
		log.Printf("Git sync enabled (mode: %s)", cfg.Git.Mode)
	}

	return app, nil
}

// NavItem is an enriched navigation entry that includes the entity type for client-side matching.
type NavItem struct {
	Label      string
	List       string
	Dashboard  bool
	Graph      bool
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

// gitTemplateData returns git-related fields for templates.
// Returns GitEnabled=true and GitBranch if git is configured.
func (a *App) gitTemplateData() (enabled bool, branch string) {
	if a.gitOps == nil {
		return false, ""
	}
	status, err := a.gitOps.GetStatus()
	if err != nil || !status.Available {
		return false, ""
	}
	return true, status.Branch
}

// addGitData adds git-related fields to a template data map.
func (a *App) addGitData(data map[string]interface{}) {
	enabled, branch := a.gitTemplateData()
	data["GitEnabled"] = enabled
	data["GitBranch"] = branch
}

// enrichNavEntry resolves a single NavigationEntry into a NavItem with entity type and count.
func (a *App) enrichNavEntry(nav NavigationEntry) NavItem {
	item := NavItem{Label: nav.Label, List: nav.List, Dashboard: nav.Dashboard, Graph: nav.Graph, Kanban: nav.Kanban}
	if nav.Dashboard || nav.Graph || nav.Kanban != "" {
		return item
	}
	if list, ok := a.Cfg.Lists[nav.List]; ok {
		item.EntityType = list.EntityType
		entities := a.g.NodesByType(list.EntityType)
		entities = applyFilters(entities, list.Filters)
		item.Count = len(entities)
	}
	return item
}

// navElements returns the navigation structure with groups and items resolved.
// The activeList parameter is used to auto-expand the group containing the active item.
func (a *App) navElements(activeList string) []NavElement {
	uiState := a.loadUIState()
	elements := make([]NavElement, 0, len(a.Cfg.Navigation))
	for _, nav := range a.Cfg.Navigation {
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
	if a.repo == nil {
		return state
	}
	data, err := a.repo.ReadCacheFile(uiStateFile)
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
	if a.repo == nil {
		return nil
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return a.repo.WriteCacheFile(uiStateFile, data)
}

// loadUserDefaults reads .rela/user-defaults.yaml and returns the parsed defaults.
// Returns nil if the file doesn't exist or can't be parsed.
func (a *App) loadUserDefaults() *UserDefaults {
	if a.repo == nil {
		return nil
	}
	data, err := a.repo.ReadCacheFile(userDefaultsFile)
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
	if a.repo == nil {
		return nil
	}
	data, err := yaml.Marshal(ud)
	if err != nil {
		return err
	}
	return a.repo.WriteCacheFile(userDefaultsFile, data)
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
// or "" if no edit form is found.
func (a *App) editFormForType(entityType string) string {
	ids := make([]string, 0, len(a.Cfg.Forms))
	for id := range a.Cfg.Forms {
		ids = append(ids, id)
	}
	natsort.Strings(ids)
	for _, id := range ids {
		f := a.Cfg.Forms[id]
		if f.EntityType == entityType && (f.Mode == "edit" || f.Mode == "") {
			return id
		}
	}
	return ""
}

// createFormForType returns the first form ID that can be used to create an entity
// of the given type. It prefers forms with mode "create" or unset, but falls back
// to edit-mode forms (which work for creation when no entity ID is provided).
func (a *App) createFormForType(entityType string) string {
	ids := make([]string, 0, len(a.Cfg.Forms))
	for id := range a.Cfg.Forms {
		ids = append(ids, id)
	}
	natsort.Strings(ids)
	fallback := ""
	for _, id := range ids {
		f := a.Cfg.Forms[id]
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

// entityDisplayTitle returns the display title for an entity using the metamodel's primary property.
func (a *App) entityDisplayTitle(e *model.Entity) string {
	entDef, ok := a.meta.GetEntityDef(e.Type)
	if ok {
		primary := entDef.GetPrimaryProperty()
		if primary != "" {
			if val := e.GetString(primary); val != "" {
				return val
			}
		}
	}
	return e.ID
}

// activeListForEntityType returns the first navigation list ID whose entity type
// matches the given type, or "" if none match. Walks into groups.
func (a *App) activeListForEntityType(entityType string) string {
	return a.findListByEntityType(a.Cfg.Navigation, entityType)
}

func (a *App) findListByEntityType(entries []NavigationEntry, entityType string) string {
	for _, nav := range entries {
		if nav.IsGroup() {
			if found := a.findListByEntityType(nav.Items, entityType); found != "" {
				return found
			}
			continue
		}
		if list, ok := a.Cfg.Lists[nav.List]; ok && list.EntityType == entityType {
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
	if _, ok := a.Cfg.Lists[listID]; ok {
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
		if _, ok := a.Cfg.Lists[from]; ok {
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
	return a.Cfg.App.Name
}

// ProjectRoot returns the root directory of the loaded project.
func (a *App) ProjectRoot() string {
	return a.repo.Paths().Root
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

// templatesForType returns all entity templates for a type, or nil on error.
func (a *App) templatesForType(entityType string) []*markdown.EntityTemplate {
	templates, err := a.repo.DiscoverEntityTemplates(entityType)
	if err != nil {
		return nil
	}
	return templates
}
