package dataentry

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// ConfigFile is the conventional filename for data-entry configuration within a rela project.
const ConfigFile = "data-entry.yaml"

// uiStateFile is the filename for persisted UI state within the .rela directory.
const uiStateFile = "ui-state.json"

// App is the central application struct holding config, metamodel, graph, and templates.
type App struct {
	Cfg  *Config
	meta *metamodel.Metamodel
	g    *graph.Graph
	repo *repository.Repository
	tmpl *template.Template
	// styleMap: property type name -> value -> CSS class name
	styleMap map[string]map[string]string
	// styledTypes: set of property type names that have style entries
	styledTypes map[string]bool
	// projCtx and fs are stored for live-reload and UI state support.
	projCtx *project.Context
	fs      storage.FS
	// uiStatePath is the path to .rela/ui-state.json for persisting UI preferences.
	uiStatePath string
	// mu protects reloadable state (Cfg, meta, g, tmpl, styleMap, styledTypes)
	// during live-reload. Handlers acquire RLock; reload acquires Lock.
	mu sync.RWMutex
	// broker delivers SSE events to connected browsers for live-reload.
	broker *eventBroker
}

// NewApp creates and initializes an App using the given filesystem.
func NewApp(projectDir string, fs storage.FS) (*App, error) {
	// Discover rela project
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	projCtx, err := project.Discover(absDir, fs)
	if err != nil {
		return nil, fmt.Errorf("discovering project: %w", err)
	}

	// Load data-entry config from project root
	configPath := filepath.Join(projCtx.Root, ConfigFile)
	cfgData, err := fs.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	// Check for deprecated syntax that needs migration
	detections, detectErr := migration.Detect(configPath, migration.FileTypeDataEntry, fs)
	if detectErr == nil && len(detections) > 0 {
		return nil, &migration.Error{
			FilePath:   configPath,
			Detections: detections,
		}
	}

	var cfg Config
	if unmarshalErr := yaml.Unmarshal(cfgData, &cfg); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", ConfigFile, unmarshalErr)
	}

	// Create repository
	repo := repository.New(fs, projCtx)

	// Load metamodel
	meta, err := repo.LoadMetamodel()
	if err != nil {
		return nil, fmt.Errorf("loading metamodel: %w", err)
	}

	// Validate config against metamodel
	if errs := validateConfig(&cfg, meta); len(errs) > 0 {
		for _, e := range errs {
			log.Printf("Config warning: %s", e)
		}
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
	tmpl, err := template.New("").Funcs(templateFuncs(styleMap, styledTypes)).Parse(allTemplates)
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	tmpl, err = tmpl.Parse(graphTemplates)
	if err != nil {
		return nil, fmt.Errorf("parsing graph templates: %w", err)
	}
	return &App{
		Cfg:         &cfg,
		meta:        meta,
		g:           g,
		repo:        repo,
		tmpl:        tmpl,
		styleMap:    styleMap,
		styledTypes: styledTypes,
		projCtx:     projCtx,
		fs:          fs,
		uiStatePath: filepath.Join(projCtx.CacheDir, uiStateFile),
		broker:      newEventBroker(),
	}, nil
}

// NavItem is an enriched navigation entry that includes the entity type for client-side matching.
type NavItem struct {
	Label      string
	List       string
	Dashboard  bool
	Graph      bool
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
	item := NavItem{Label: nav.Label, List: nav.List, Dashboard: nav.Dashboard, Graph: nav.Graph}
	if nav.Dashboard || nav.Graph {
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
	if a.uiStatePath == "" {
		return state
	}
	data, err := a.fs.ReadFile(a.uiStatePath)
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
	if a.uiStatePath == "" {
		return nil
	}
	// Ensure .rela directory exists
	if err := a.fs.MkdirAll(filepath.Dir(a.uiStatePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return a.fs.WriteFile(a.uiStatePath, data, 0o644)
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
	sort.Strings(ids)
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
	sort.Strings(ids)
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

func validateConfig(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string
	// Validate navigation: reject nested groups
	for _, nav := range cfg.Navigation {
		if nav.IsGroup() {
			for _, child := range nav.Items {
				if child.IsGroup() {
					errs = append(errs, fmt.Sprintf(
						"navigation: group %q contains nested group %q — nested groups are not supported",
						nav.Group, child.Group))
				}
			}
		}
	}
	for formID, form := range cfg.Forms {
		if _, ok := meta.GetEntityDef(form.EntityType); !ok {
			errs = append(errs, fmt.Sprintf("form %q: unknown entity type %q", formID, form.EntityType))
			continue
		}
		entDef, _ := meta.GetEntityDef(form.EntityType)
		for _, f := range form.Fields {
			if _, ok := entDef.Properties[f.Property]; !ok {
				errs = append(errs, fmt.Sprintf("form %q: field %q not in metamodel for entity %q", formID, f.Property, form.EntityType))
			}
		}
		for _, r := range form.Relations {
			if _, ok := meta.GetRelationDef(r.Relation); !ok {
				errs = append(errs, fmt.Sprintf("form %q: unknown relation %q", formID, r.Relation))
			}
		}
	}
	for listID, list := range cfg.Lists {
		if _, ok := meta.GetEntityDef(list.EntityType); !ok {
			errs = append(errs, fmt.Sprintf("list %q: unknown entity type %q", listID, list.EntityType))
			continue
		}
		entDef, _ := meta.GetEntityDef(list.EntityType)
		for _, c := range list.Columns {
			if c.Relation != "" {
				if _, ok := meta.GetRelationDef(c.Relation); !ok {
					errs = append(errs, fmt.Sprintf("list %q: column relation %q not in metamodel", listID, c.Relation))
				}
			} else if _, ok := entDef.Properties[c.Property]; !ok {
				errs = append(errs, fmt.Sprintf("list %q: column %q not in metamodel for entity %q", listID, c.Property, list.EntityType))
			}
		}
	}
	validContexts := map[string]bool{"entity": true, "list": true, "view": true, "global": true}
	for cmdID, cmd := range cfg.Commands {
		if cmd.Label == "" {
			errs = append(errs, fmt.Sprintf("command %q: label is required", cmdID))
		}
		if cmd.Script == "" {
			errs = append(errs, fmt.Sprintf("command %q: script is required", cmdID))
		}
		if !validContexts[cmd.Context] {
			errs = append(errs, fmt.Sprintf("command %q: invalid context %q (must be entity, list, view, or global)", cmdID, cmd.Context))
		}
		if cmd.AvailableOn != nil {
			for _, v := range cmd.AvailableOn.Views {
				if _, ok := cfg.Views[v]; !ok {
					errs = append(errs, fmt.Sprintf("command %q: available_on references unknown view %q", cmdID, v))
				}
			}
			for _, l := range cmd.AvailableOn.Lists {
				if _, ok := cfg.Lists[l]; !ok {
					errs = append(errs, fmt.Sprintf("command %q: available_on references unknown list %q", cmdID, l))
				}
			}
			for _, et := range cmd.AvailableOn.EntityTypes {
				if _, ok := meta.GetEntityDef(et); !ok {
					errs = append(errs, fmt.Sprintf("command %q: available_on references unknown entity type %q", cmdID, et))
				}
			}
		}
	}
	return errs
}
