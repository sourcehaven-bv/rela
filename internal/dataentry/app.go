package dataentry

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// ConfigFile is the conventional filename for data-entry configuration within a rela project.
const ConfigFile = "data-entry.yaml"

// App is the central application struct holding config, metamodel, graph, and templates.
type App struct {
	Cfg     *Config
	meta    *metamodel.Metamodel
	g       *graph.Graph
	projCtx *project.Context
	tmpl    *template.Template
	// styleMap: property type name -> value -> CSS class name
	styleMap map[string]map[string]string
	// styledTypes: set of property type names that have style entries
	styledTypes map[string]bool
	// conflicts holds the current set of merge conflicts (nil when clean).
	conflicts *ConflictSet
	// sync manages git operations (auto-commit, branches, status).
	sync *SyncManager
}

// NewApp creates and initializes an App from a project directory.
// It discovers the rela project and loads data-entry.yaml from the project root.
func NewApp(projectDir string) (*App, error) {
	// Discover rela project
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	projCtx, err := project.Discover(absDir)
	if err != nil {
		return nil, fmt.Errorf("discovering project: %w", err)
	}

	// Load data-entry config from project root
	configPath := filepath.Join(projCtx.Root, ConfigFile)
	cfgData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	var cfg Config
	if unmarshalErr := yaml.Unmarshal(cfgData, &cfg); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", ConfigFile, unmarshalErr)
	}

	// Load metamodel
	meta, err := metamodel.Load(projCtx.MetamodelPath)
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
	result, err := markdown.SyncFromFiles(projCtx, meta, g)
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
	if _, err = tmpl.Parse(conflictTemplates); err != nil {
		return nil, fmt.Errorf("parsing conflict templates: %w", err)
	}
	if _, err = tmpl.Parse(syncTemplates); err != nil {
		return nil, fmt.Errorf("parsing sync templates: %w", err)
	}

	// Initialize git sync manager
	syncMgr := NewSyncManager(projCtx.Root, SyncOptions{
		ProtectedBranches: cfg.Git.RequirePR,
	})

	app := &App{
		Cfg:         &cfg,
		meta:        meta,
		g:           g,
		projCtx:     projCtx,
		tmpl:        tmpl,
		styleMap:    styleMap,
		styledTypes: styledTypes,
		sync:        syncMgr,
	}

	// Set the OnPull callback now that the App instance exists
	syncMgr.SetOnPull(func() {
		if err := app.rebuildGraph(); err != nil {
			log.Printf("Warning: graph rebuild after pull failed: %v", err)
		}
	})

	return app, nil
}

// NavItem is an enriched navigation entry that includes the entity type for client-side matching.
type NavItem struct {
	Label      string
	List       string
	EntityType string
}

// navItems returns enriched navigation entries with entity types resolved from list config.
func (a *App) navItems() []NavItem {
	items := make([]NavItem, len(a.Cfg.Navigation))
	for i, nav := range a.Cfg.Navigation {
		items[i] = NavItem{Label: nav.Label, List: nav.List}
		if list, ok := a.Cfg.Lists[nav.List]; ok {
			items[i].EntityType = list.EntityType
		}
	}
	return items
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
// matches the given type, or "" if none match.
func (a *App) activeListForEntityType(entityType string) string {
	for _, nav := range a.Cfg.Navigation {
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
// It first tries matching by entity type, then falls back to the Referer header.
func (a *App) resolveActiveList(entityType string, r *http.Request) string {
	if active := a.activeListForEntityType(entityType); active != "" {
		return active
	}
	return a.activeListFromReferer(r)
}

// entityPrimaryProperty returns the primary property name for an entity type, or "".
func (a *App) entityPrimaryProperty(entityType string) string {
	entDef, ok := a.meta.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	return entDef.GetPrimaryProperty()
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
			if _, ok := entDef.Properties[c.Property]; !ok {
				errs = append(errs, fmt.Sprintf("list %q: column %q not in metamodel for entity %q", listID, c.Property, list.EntityType))
			}
		}
	}
	return errs
}
