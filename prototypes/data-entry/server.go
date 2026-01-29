// Prototype: data-entry web server (Option C)
//
// A Go HTTP server that reads metamodel + data-entry config, loads entities from
// disk via rela's internal packages, and serves an interactive UI using
// server-rendered HTML + HTMX.
//
// Usage:
//
//	go run server.go [-project ./project] [-port 8080]
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"gopkg.in/yaml.v3"
)

// ── Data Entry Config Types ──

type DataEntryConfig struct {
	Version    string                       `yaml:"version"`
	App        AppConfig                    `yaml:"app"`
	Styles     map[string]map[string]string `yaml:"styles"`
	Forms      map[string]Form              `yaml:"forms"`
	Lists      map[string]List              `yaml:"lists"`
	Views      map[string]ViewConfig        `yaml:"views"`
	Navigation []NavigationEntry            `yaml:"navigation"`
}

type AppConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Form struct {
	EntityType  string         `yaml:"entity_type"`
	Title       string         `yaml:"title"`
	Description string         `yaml:"description"`
	Mode        string         `yaml:"mode"`
	Body        *bool          `yaml:"body,omitempty"`
	Fields      []FormField    `yaml:"fields"`
	Relations   []FormRelation `yaml:"relations"`
}

type FormField struct {
	Property    string              `yaml:"property"`
	Label       string              `yaml:"label"`
	Placeholder string              `yaml:"placeholder"`
	Help        string              `yaml:"help"`
	Required    *bool               `yaml:"required,omitempty"`
	Default     string              `yaml:"default"`
	Hidden      bool                `yaml:"hidden"`
	Widget      string              `yaml:"widget"`
	Transitions map[string][]string `yaml:"transitions,omitempty"`
}

type FormRelation struct {
	Relation    string             `yaml:"relation"`
	Direction   string             `yaml:"direction"`
	TargetType  string             `yaml:"target_type"`
	Label       string             `yaml:"label"`
	Required    bool               `yaml:"required"`
	Widget      string             `yaml:"widget"`
	AllowCreate bool               `yaml:"allow_create"`
	CreateForm  string             `yaml:"create_form"`
	Properties  []RelationProperty `yaml:"properties"`
}

type RelationProperty struct {
	Property string `yaml:"property"`
	Label    string `yaml:"label"`
	Widget   string `yaml:"widget"`
	Required bool   `yaml:"required"`
}

type List struct {
	EntityType     string          `yaml:"entity_type"`
	Title          string          `yaml:"title"`
	Description    string          `yaml:"description"`
	Columns        []ListColumn    `yaml:"columns"`
	Sort           *SortConfig     `yaml:"sort,omitempty"`
	Filters        []FilterConfig  `yaml:"filters"`
	FilterControls []FilterControl `yaml:"filter_controls"`
	CreateForm     string          `yaml:"create_form"`
	EditForm       string          `yaml:"edit_form"`
	DetailView     string          `yaml:"detail_view"`
	PageSize       int             `yaml:"page_size"`
}

type ListColumn struct {
	Property string `yaml:"property"`
	Label    string `yaml:"label"`
	Sortable bool   `yaml:"sortable"`
	Link     bool   `yaml:"link"`
}

type SortConfig struct {
	Property  string `yaml:"property"`
	Direction string `yaml:"direction"`
}

type FilterConfig struct {
	Property string `yaml:"property"`
	Operator string `yaml:"operator"`
	Value    string `yaml:"value"`
}

type FilterControl struct {
	Property string `yaml:"property"`
	Widget   string `yaml:"widget"`
}

type NavigationEntry struct {
	Label string `yaml:"label"`
	List  string `yaml:"list"`
}

// ── View Config Types ──

type ViewConfig struct {
	Title    string         `yaml:"title"`
	Entry    ViewEntry      `yaml:"entry"`
	Traverse []ViewTraverse `yaml:"traverse"`
	Sections []ViewSection  `yaml:"sections"`
}

type ViewEntry struct {
	Type string `yaml:"type"`
}

type ViewTraverse struct {
	From           string `yaml:"from"`
	Follow         string `yaml:"follow,omitempty"`
	FollowIncoming string `yaml:"follow_incoming,omitempty"`
	CollectAs      string `yaml:"collect_as"`
	Recursive      bool   `yaml:"recursive,omitempty"`
	MaxDepth       int    `yaml:"max_depth,omitempty"`
}

type ViewSection struct {
	Heading      string             `yaml:"heading,omitempty"`
	Source       string             `yaml:"source"`
	Display      string             `yaml:"display"` // properties, content, table, cards, list
	Fields       []ViewSectionField `yaml:"fields,omitempty"`
	Columns      []ListColumn       `yaml:"columns,omitempty"` // for display: table
	GroupBy      string             `yaml:"group_by,omitempty"`
	EmptyMessage string             `yaml:"empty_message,omitempty"`
	Link         bool               `yaml:"link,omitempty"` // make entity titles clickable
}

type ViewSectionField struct {
	Property string `yaml:"property"`
	Label    string `yaml:"label,omitempty"`
}

// ── View Engine ──

// viewCollection holds entities grouped by collection name after traversal
type viewResult struct {
	Entry       *model.Entity
	Collections map[string][]*model.Entity
}

func (a *App) executeView(view ViewConfig, entryID string) (*viewResult, error) {
	entry, ok := a.g.GetNode(entryID)
	if !ok {
		return nil, fmt.Errorf("entry entity not found: %s", entryID)
	}
	if entry.Type != view.Entry.Type {
		return nil, fmt.Errorf("entry entity %s is type %s, expected %s", entryID, entry.Type, view.Entry.Type)
	}

	result := &viewResult{
		Entry:       entry,
		Collections: map[string][]*model.Entity{"entry": {entry}},
	}

	// Multi-pass traversal (up to 10 passes until stable)
	for pass := 0; pass < 10; pass++ {
		before := countViewEntities(result.Collections)
		for _, rule := range view.Traverse {
			a.applyViewTraverse(rule, result)
		}
		if countViewEntities(result.Collections) == before {
			break
		}
	}

	// Remove internal "entry" collection
	delete(result.Collections, "entry")

	return result, nil
}

func (a *App) applyViewTraverse(rule ViewTraverse, result *viewResult) {
	// Gather source entities
	var sources []*model.Entity
	if rule.From == "*" {
		seen := map[string]bool{}
		for _, entities := range result.Collections {
			for _, e := range entities {
				if !seen[e.ID] {
					sources = append(sources, e)
					seen[e.ID] = true
				}
			}
		}
	} else if entities, ok := result.Collections[rule.From]; ok {
		sources = entities
	}

	// Traverse from each source
	var found []*model.Entity
	for _, src := range sources {
		if rule.Recursive {
			maxD := rule.MaxDepth
			if maxD <= 0 {
				maxD = 10
			}
			found = append(found, a.traverseViewRecursive(src.ID, rule, 0, maxD, map[string]bool{})...)
		} else {
			found = append(found, a.traverseViewOnce(src.ID, rule)...)
		}
	}

	// Deduplicate into collection
	if result.Collections[rule.CollectAs] == nil {
		result.Collections[rule.CollectAs] = []*model.Entity{}
	}
	existing := map[string]bool{}
	for _, e := range result.Collections[rule.CollectAs] {
		existing[e.ID] = true
	}
	for _, e := range found {
		if !existing[e.ID] {
			result.Collections[rule.CollectAs] = append(result.Collections[rule.CollectAs], e)
			existing[e.ID] = true
		}
	}
}

func (a *App) traverseViewOnce(sourceID string, rule ViewTraverse) []*model.Entity {
	var out []*model.Entity
	if rule.Follow != "" {
		for _, edge := range a.g.OutgoingEdges(sourceID) {
			if edge.Type == rule.Follow {
				if target, ok := a.g.GetNode(edge.To); ok {
					out = append(out, target)
				}
			}
		}
	} else if rule.FollowIncoming != "" {
		for _, edge := range a.g.IncomingEdges(sourceID) {
			if edge.Type == rule.FollowIncoming {
				if src, ok := a.g.GetNode(edge.From); ok {
					out = append(out, src)
				}
			}
		}
	}
	return out
}

func (a *App) traverseViewRecursive(sourceID string, rule ViewTraverse, depth, maxDepth int, visited map[string]bool) []*model.Entity {
	if depth >= maxDepth || visited[sourceID] {
		return nil
	}
	visited[sourceID] = true
	immediate := a.traverseViewOnce(sourceID, rule)
	var all []*model.Entity
	all = append(all, immediate...)
	for _, e := range immediate {
		all = append(all, a.traverseViewRecursive(e.ID, rule, depth+1, maxDepth, visited)...)
	}
	return all
}

func countViewEntities(collections map[string][]*model.Entity) int {
	seen := map[string]bool{}
	for _, entities := range collections {
		for _, e := range entities {
			seen[e.ID] = true
		}
	}
	return len(seen)
}

// ── Server App ──

type App struct {
	cfg     *DataEntryConfig
	meta    *metamodel.Metamodel
	g       *graph.Graph
	projCtx *project.Context
	tmpl    *template.Template
	// styleMap: property type name → value → CSS class name
	// Built from config styles + auto-detection of enum/custom types
	styleMap map[string]map[string]string
	// styledTypes: set of property type names that have style entries
	styledTypes map[string]bool
}

func main() {
	projectDir := "project"
	port := "8080"

	// Parse simple flags
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-project":
			i++
			projectDir = args[i]
		case "-port":
			i++
			port = args[i]
		}
	}

	app, err := newApp(projectDir)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleIndex)
	mux.HandleFunc("/list/", app.handleList)
	mux.HandleFunc("/form/", app.handleForm)
	mux.HandleFunc("/entity/", app.handleEntity)
	mux.HandleFunc("/view/", app.handleView)
	mux.HandleFunc("/api/create", app.handleCreate)
	mux.HandleFunc("/api/update", app.handleUpdate)
	mux.HandleFunc("/api/delete", app.handleDelete)
	mux.HandleFunc("/api/inline-create", app.handleInlineCreate)
	mux.HandleFunc("/api/inline-form/", app.handleInlineForm)

	log.Printf("Starting %s on http://localhost:%s", app.cfg.App.Name, port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func newApp(projectDir string) (*App, error) {
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
	configPath := filepath.Join(projCtx.Root, "data-entry.yaml")
	cfgData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading data-entry.yaml: %w", err)
	}
	var cfg DataEntryConfig
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		return nil, fmt.Errorf("parsing data-entry.yaml: %w", err)
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
	tmpl, err := template.New("").Funcs(templateFuncs(styleMap, styledTypes, meta)).Parse(allTemplates)
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return &App{
		cfg:         &cfg,
		meta:        meta,
		g:           g,
		projCtx:     projCtx,
		tmpl:        tmpl,
		styleMap:    styleMap,
		styledTypes: styledTypes,
	}, nil
}

// colorToCSSClass maps a color name from config to a CSS class
var colorToCSSClass = map[string]string{
	"blue":   "badge-blue",
	"purple": "badge-purple",
	"green":  "badge-green",
	"gray":   "badge-gray",
	"red":    "badge-red",
	"orange": "badge-orange",
	"yellow": "badge-yellow",
}

// autoColors assigns colors to enum values that have no explicit style
var autoColors = []string{"blue", "purple", "green", "orange", "yellow", "red", "gray"}

func buildStyleMap(cfg *DataEntryConfig, meta *metamodel.Metamodel) (map[string]map[string]string, map[string]bool) {
	sm := make(map[string]map[string]string)
	st := make(map[string]bool)

	// First: populate from explicit config styles
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

	// Second: auto-assign styles for custom types not already styled
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

// entityDisplayTitle returns the display title for an entity using the metamodel's primary property
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

// resolvePropertyType returns the metamodel type name for a property
// by checking which entity type owns it
func resolvePropertyType(prop string, entityType string, meta *metamodel.Metamodel) string {
	entDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	propDef, ok := entDef.Properties[prop]
	if !ok {
		return ""
	}
	return propDef.Type
}

func validateConfig(cfg *DataEntryConfig, meta *metamodel.Metamodel) []string {
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

// ── Handlers ──

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// Redirect to first nav item
	if len(a.cfg.Navigation) > 0 {
		http.Redirect(w, r, "/list/"+a.cfg.Navigation[0].List, http.StatusFound)
		return
	}
	http.Error(w, "No navigation configured", 500)
}

func (a *App) handleList(w http.ResponseWriter, r *http.Request) {
	listID := strings.TrimPrefix(r.URL.Path, "/list/")
	list, ok := a.cfg.Lists[listID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	entDef, _ := a.meta.GetEntityDef(list.EntityType)

	// Get entities of this type
	entities := a.g.NodesByType(list.EntityType)

	// Apply static filters
	entities = applyFilters(entities, list.Filters)

	// Apply query param filters
	for _, fc := range list.FilterControls {
		val := r.URL.Query().Get("filter_" + fc.Property)
		if val != "" {
			entities = applyFilters(entities, []FilterConfig{{
				Property: fc.Property,
				Operator: "=",
				Value:    val,
			}})
		}
	}

	// Sort
	sortEntities(entities, list.Sort)

	// Resolve columns with values
	type CellData struct {
		Value    string
		Property string
		PropType string // metamodel type name for badge resolution
		Link     bool
		EntityID string
	}
	type RowData struct {
		EntityID string
		Cells    []CellData
	}

	var rows []RowData
	for _, e := range entities {
		var cells []CellData
		for _, col := range list.Columns {
			val := fmt.Sprintf("%v", e.Properties[col.Property])
			if e.Properties[col.Property] == nil {
				val = ""
			}
			propType := resolvePropertyType(col.Property, list.EntityType, a.meta)
			cells = append(cells, CellData{
				Value:    val,
				Property: col.Property,
				PropType: propType,
				Link:     col.Link,
				EntityID: e.ID,
			})
		}
		rows = append(rows, RowData{EntityID: e.ID, Cells: cells})
	}

	// Resolve filter control values from metamodel
	type ResolvedFC struct {
		Property string
		Label    string
		Widget   string
		Values   []string
		Current  string
	}
	var filterControls []ResolvedFC
	for _, fc := range list.FilterControls {
		prop := entDef.Properties[fc.Property]
		vals := resolvePropertyValues(prop, a.meta)
		filterControls = append(filterControls, ResolvedFC{
			Property: fc.Property,
			Label:    titleCase(fc.Property),
			Widget:   fc.Widget,
			Values:   vals,
			Current:  r.URL.Query().Get("filter_" + fc.Property),
		})
	}

	// Resolve relation data for display (e.g., category for each ticket)
	type RelationInfo struct {
		TargetID    string
		TargetTitle string
	}
	entityRelations := make(map[string][]RelationInfo)
	for _, e := range entities {
		for _, edge := range a.g.OutgoingEdges(e.ID) {
			target, ok := a.g.GetNode(edge.To)
			if !ok {
				continue
			}
			entityRelations[e.ID] = append(entityRelations[e.ID], RelationInfo{
				TargetID:    target.ID,
				TargetTitle: a.entityDisplayTitle(target),
			})
		}
	}

	// Resolve detail link: if list has detail_view, link to /view/; otherwise /entity/
	detailLinkPrefix := "/entity/"
	if list.DetailView != "" {
		detailLinkPrefix = "/view/" + list.DetailView + "/"
	}

	data := map[string]interface{}{
		"App":              a.cfg.App,
		"Navigation":       a.cfg.Navigation,
		"ActiveList":       listID,
		"List":             list,
		"ListID":           listID,
		"Columns":          list.Columns,
		"Rows":             rows,
		"FilterControls":   filterControls,
		"EntityRelations":  entityRelations,
		"TotalCount":       len(entities),
		"EditForm":         list.EditForm,
		"DetailLinkPrefix": detailLinkPrefix,
		"IsHTMX":           r.Header.Get("HX-Request") == "true",
	}

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "list-content", data)
	} else {
		a.tmpl.ExecuteTemplate(w, "page", data)
	}
}

func (a *App) handleForm(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/form/"), "/", 2)
	formID := parts[0]
	var entityID string
	if len(parts) > 1 {
		entityID = parts[1]
	}

	form, ok := a.cfg.Forms[formID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	// Resolve fields
	type ResolvedField struct {
		Property    string
		Label       string
		Placeholder string
		Help        string
		Required    bool
		Default     string
		Value       string
		Hidden      bool
		Widget      string
		InputType   string
		Values      []string
		Transitions map[string][]string
	}

	var entity *model.Entity
	if entityID != "" {
		entity, _ = a.g.GetNode(entityID)
	}

	var fields []ResolvedField
	for _, f := range form.Fields {
		prop := entDef.Properties[f.Property]
		rf := ResolvedField{
			Property:    f.Property,
			Label:       coalesce(f.Label, titleCase(f.Property)),
			Placeholder: f.Placeholder,
			Help:        f.Help,
			Default:     coalesce(f.Default, prop.Default),
			Hidden:      f.Hidden,
			Widget:      resolveWidget(f.Widget, prop, a.meta),
			Values:      resolvePropertyValues(prop, a.meta),
			Transitions: f.Transitions,
		}
		if f.Required != nil {
			rf.Required = *f.Required
		} else {
			rf.Required = prop.Required
		}
		rf.InputType = widgetToInputType(rf.Widget)

		// Populate value from entity if editing
		if entity != nil {
			val := entity.Properties[f.Property]
			if val != nil {
				rf.Value = fmt.Sprintf("%v", val)
			}
		} else {
			rf.Value = rf.Default
		}

		fields = append(fields, rf)
	}

	// Resolve body content
	var bodyContent string
	showBody := form.Body != nil && *form.Body
	if showBody && entity != nil {
		bodyContent = entity.Content
	}

	// Resolve relation fields with actual target entities
	type ResolvedRelation struct {
		Relation    string
		Label       string
		Required    bool
		Widget      string
		TargetType  string
		TargetLabel string
		Options     []struct{ ID, Title string }
		Selected    []string
		AllowCreate bool
		CreateForm  string
		Properties  []RelationProperty
		// SelectedProps maps targetID -> propName -> value (for editing)
		SelectedProps map[string]map[string]string
	}
	var relations []ResolvedRelation
	for _, rel := range form.Relations {
		targetDef, _ := a.meta.GetEntityDef(rel.TargetType)
		targetLabel := ""
		if targetDef != nil {
			targetLabel = targetDef.Label
		}

		rr := ResolvedRelation{
			Relation:      rel.Relation,
			Label:         rel.Label,
			Required:      rel.Required,
			Widget:        coalesce(rel.Widget, "select"),
			TargetType:    rel.TargetType,
			TargetLabel:   targetLabel,
			AllowCreate:   rel.AllowCreate,
			CreateForm:    rel.CreateForm,
			Properties:    rel.Properties,
			SelectedProps: make(map[string]map[string]string),
		}

		// Get all entities of target type as options
		targets := a.g.NodesByType(rel.TargetType)
		for _, t := range targets {
			rr.Options = append(rr.Options, struct{ ID, Title string }{t.ID, a.entityDisplayTitle(t)})
		}

		// Get current selections and relation properties if editing
		if entity != nil {
			for _, edge := range a.g.OutgoingEdges(entity.ID) {
				if edge.Type == rel.Relation {
					rr.Selected = append(rr.Selected, edge.To)
					// Capture relation properties
					if len(rel.Properties) > 0 {
						props := make(map[string]string)
						for _, rp := range rel.Properties {
							if v, ok := edge.Properties[rp.Property]; ok {
								props[rp.Property] = fmt.Sprintf("%v", v)
							}
						}
						rr.SelectedProps[edge.To] = props
					}
				}
			}
		}

		relations = append(relations, rr)
	}

	mode := coalesce(form.Mode, "create")
	if entityID != "" {
		mode = "edit"
	}

	data := map[string]interface{}{
		"App":        a.cfg.App,
		"Navigation": a.cfg.Navigation,
		"FormID":     formID,
		"Form":       form,
		"Fields":     fields,
		"Relations":  relations,
		"Mode":       mode,
		"EntityID":   entityID,
		"EntityType": form.EntityType,
		"ShowBody":   showBody,
		"Body":       bodyContent,
		"IsHTMX":     r.Header.Get("HX-Request") == "true",
	}

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "form-content", data)
	} else {
		a.tmpl.ExecuteTemplate(w, "form-page", data)
	}
}

func (a *App) handleEntity(w http.ResponseWriter, r *http.Request) {
	entityID := strings.TrimPrefix(r.URL.Path, "/entity/")
	entity, ok := a.g.GetNode(entityID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	entDef, _ := a.meta.GetEntityDef(entity.Type)

	// Find the edit form for this entity type
	var editFormID string
	for id, f := range a.cfg.Forms {
		if f.EntityType == entity.Type && (f.Mode == "edit" || f.Mode == "") {
			editFormID = id
		}
	}

	// Get relations
	outgoing := a.g.OutgoingEdges(entityID)
	incoming := a.g.IncomingEdges(entityID)

	type RelPropDisplay struct {
		Key   string
		Value string
	}
	type RelDisplay struct {
		Type        string
		TargetID    string
		TargetTitle string
		Direction   string
		Properties  []RelPropDisplay
	}
	var rels []RelDisplay
	for _, e := range outgoing {
		target, ok := a.g.GetNode(e.To)
		title := e.To
		if ok {
			title = a.entityDisplayTitle(target)
		}
		rd := RelDisplay{e.Type, e.To, title, "outgoing", nil}
		for k, v := range e.Properties {
			rd.Properties = append(rd.Properties, RelPropDisplay{k, fmt.Sprintf("%v", v)})
		}
		rels = append(rels, rd)
	}
	for _, e := range incoming {
		source, ok := a.g.GetNode(e.From)
		title := e.From
		if ok {
			title = a.entityDisplayTitle(source)
		}
		rd := RelDisplay{e.Type, e.From, title, "incoming", nil}
		for k, v := range e.Properties {
			rd.Properties = append(rd.Properties, RelPropDisplay{k, fmt.Sprintf("%v", v)})
		}
		rels = append(rels, rd)
	}

	// Build property type map for badge rendering on detail page
	propTypes := make(map[string]string)
	if entDef != nil {
		for propName, propDef := range entDef.Properties {
			propTypes[propName] = propDef.Type
		}
	}

	data := map[string]interface{}{
		"App":        a.cfg.App,
		"Navigation": a.cfg.Navigation,
		"Entity":     entity,
		"EntityDef":  entDef,
		"EditFormID": editFormID,
		"Relations":  rels,
		"PropTypes":  propTypes,
		"IsHTMX":     r.Header.Get("HX-Request") == "true",
	}

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "entity-content", data)
	} else {
		a.tmpl.ExecuteTemplate(w, "entity-page", data)
	}
}

func (a *App) handleView(w http.ResponseWriter, r *http.Request) {
	// URL: /view/{viewID}/{entityID}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/view/"), "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "Usage: /view/{viewID}/{entityID}", 400)
		return
	}
	viewID := parts[0]
	entityID := parts[1]

	view, ok := a.cfg.Views[viewID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	result, err := a.executeView(view, entityID)
	if err != nil {
		http.Error(w, fmt.Sprintf("View error: %v", err), 400)
		return
	}

	// Build section data for the template
	type SectionFieldData struct {
		Label    string
		Value    string
		PropType string
	}

	type SectionEntityData struct {
		ID         string
		Title      string
		Type       string
		Fields     []SectionFieldData
		Content    string
		HasContent bool
	}

	type SectionColumnData struct {
		Value    string
		PropType string
		Link     bool
		EntityID string
	}

	type SectionRowData struct {
		EntityID string
		Cells    []SectionColumnData
		Content  string
	}

	type GroupData struct {
		GroupName string
		Rows      []SectionRowData
		Entities  []SectionEntityData
	}

	type SectionData struct {
		Heading      string
		Display      string // properties, content, table, cards, list
		Fields       []SectionFieldData
		Entities     []SectionEntityData
		Columns      []ListColumn
		Rows         []SectionRowData
		Groups       []GroupData
		IsGrouped    bool
		EmptyMessage string
		IsEmpty      bool
		Link         bool
		Content      string // for entry content display
		HasContent   bool
	}

	var sections []SectionData

	for _, sec := range view.Sections {
		sd := SectionData{
			Heading:      sec.Heading,
			Display:      sec.Display,
			EmptyMessage: sec.EmptyMessage,
			Link:         sec.Link,
		}

		if sec.Source == "entry" {
			// Single entry entity
			e := result.Entry
			entDef, _ := a.meta.GetEntityDef(e.Type)

			switch sec.Display {
			case "properties":
				for _, f := range sec.Fields {
					val := ""
					if v := e.Properties[f.Property]; v != nil {
						val = fmt.Sprintf("%v", v)
					}
					propType := ""
					if entDef != nil {
						if pd, ok := entDef.Properties[f.Property]; ok {
							propType = pd.Type
						}
					}
					label := f.Label
					if label == "" {
						label = titleCase(f.Property)
					}
					sd.Fields = append(sd.Fields, SectionFieldData{
						Label: label, Value: val, PropType: propType,
					})
				}

			case "content":
				sd.Content = e.Content
				sd.HasContent = e.Content != ""
			}
		} else {
			// Collection source
			entities, exists := result.Collections[sec.Source]
			if !exists {
				entities = []*model.Entity{}
			}
			sd.IsEmpty = len(entities) == 0

			switch sec.Display {
			case "table":
				sd.Columns = sec.Columns
				if sec.GroupBy != "" {
					sd.IsGrouped = true
					groups := map[string][]*model.Entity{}
					var groupOrder []string
					for _, e := range entities {
						groupKey := ""
						// Parse group_by: "properties.X" or just "X"
						prop := sec.GroupBy
						if strings.HasPrefix(prop, "properties.") {
							prop = strings.TrimPrefix(prop, "properties.")
						}
						if v := e.Properties[prop]; v != nil {
							groupKey = fmt.Sprintf("%v", v)
						} else {
							groupKey = "(none)"
						}
						if _, seen := groups[groupKey]; !seen {
							groupOrder = append(groupOrder, groupKey)
						}
						groups[groupKey] = append(groups[groupKey], e)
					}
					for _, gName := range groupOrder {
						gd := GroupData{GroupName: gName}
						for _, e := range groups[gName] {
							entDef, _ := a.meta.GetEntityDef(e.Type)
							row := SectionRowData{EntityID: e.ID}
							for _, col := range sec.Columns {
								val := ""
								if v := e.Properties[col.Property]; v != nil {
									val = fmt.Sprintf("%v", v)
								}
								propType := ""
								if entDef != nil {
									if pd, ok := entDef.Properties[col.Property]; ok {
										propType = pd.Type
									}
								}
								row.Cells = append(row.Cells, SectionColumnData{
									Value: val, PropType: propType, Link: col.Link, EntityID: e.ID,
								})
							}
							gd.Rows = append(gd.Rows, row)
						}
						sd.Groups = append(sd.Groups, gd)
					}
				} else {
					for _, e := range entities {
						entDef, _ := a.meta.GetEntityDef(e.Type)
						row := SectionRowData{EntityID: e.ID}
						for _, col := range sec.Columns {
							val := ""
							if v := e.Properties[col.Property]; v != nil {
								val = fmt.Sprintf("%v", v)
							}
							propType := ""
							if entDef != nil {
								if pd, ok := entDef.Properties[col.Property]; ok {
									propType = pd.Type
								}
							}
							row.Cells = append(row.Cells, SectionColumnData{
								Value: val, PropType: propType, Link: col.Link, EntityID: e.ID,
							})
						}
						sd.Rows = append(sd.Rows, row)
					}
				}

			case "content":
				for _, e := range entities {
					entDef, _ := a.meta.GetEntityDef(e.Type)
					sed := SectionEntityData{
						ID:         e.ID,
						Title:      a.entityDisplayTitle(e),
						Type:       e.Type,
						Content:    e.Content,
						HasContent: e.Content != "",
					}
					for _, f := range sec.Fields {
						val := ""
						if v := e.Properties[f.Property]; v != nil {
							val = fmt.Sprintf("%v", v)
						}
						propType := ""
						if entDef != nil {
							if pd, ok := entDef.Properties[f.Property]; ok {
								propType = pd.Type
							}
						}
						label := f.Label
						if label == "" {
							label = titleCase(f.Property)
						}
						sed.Fields = append(sed.Fields, SectionFieldData{
							Label: label, Value: val, PropType: propType,
						})
					}
					sd.Entities = append(sd.Entities, sed)
				}

			case "cards":
				for _, e := range entities {
					entDef, _ := a.meta.GetEntityDef(e.Type)
					sed := SectionEntityData{
						ID:         e.ID,
						Title:      a.entityDisplayTitle(e),
						Type:       e.Type,
						Content:    e.Content,
						HasContent: e.Content != "",
					}
					for _, f := range sec.Fields {
						val := ""
						if v := e.Properties[f.Property]; v != nil {
							val = fmt.Sprintf("%v", v)
						}
						propType := ""
						if entDef != nil {
							if pd, ok := entDef.Properties[f.Property]; ok {
								propType = pd.Type
							}
						}
						label := f.Label
						if label == "" {
							label = titleCase(f.Property)
						}
						sed.Fields = append(sed.Fields, SectionFieldData{
							Label: label, Value: val, PropType: propType,
						})
					}
					sd.Entities = append(sd.Entities, sed)
				}

			case "list":
				for _, e := range entities {
					entDef, _ := a.meta.GetEntityDef(e.Type)
					sed := SectionEntityData{
						ID:    e.ID,
						Title: a.entityDisplayTitle(e),
						Type:  e.Type,
					}
					for _, f := range sec.Fields {
						val := ""
						if v := e.Properties[f.Property]; v != nil {
							val = fmt.Sprintf("%v", v)
						}
						propType := ""
						if entDef != nil {
							if pd, ok := entDef.Properties[f.Property]; ok {
								propType = pd.Type
							}
						}
						label := f.Label
						if label == "" {
							label = titleCase(f.Property)
						}
						sed.Fields = append(sed.Fields, SectionFieldData{
							Label: label, Value: val, PropType: propType,
						})
					}
					sd.Entities = append(sd.Entities, sed)
				}
			}
		}

		sections = append(sections, sd)
	}

	data := map[string]interface{}{
		"App":        a.cfg.App,
		"Navigation": a.cfg.Navigation,
		"View":       view,
		"ViewID":     viewID,
		"EntityID":   entityID,
		"Entry":      result.Entry,
		"EntryTitle": a.entityDisplayTitle(result.Entry),
		"Sections":   sections,
		"IsHTMX":     r.Header.Get("HX-Request") == "true",
	}

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "view-content", data)
	} else {
		a.tmpl.ExecuteTemplate(w, "view-page", data)
	}
}

func (a *App) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	r.ParseForm()

	formID := r.FormValue("_form_id")
	form, ok := a.cfg.Forms[formID]
	if !ok {
		http.Error(w, "Unknown form", 400)
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	// Generate ID
	var entityID string
	if entDef.GetIDType() == metamodel.IDTypeManual {
		entityID = r.FormValue("_entity_id")
		if entityID == "" {
			http.Error(w, "Manual ID required", 400)
			return
		}
	} else {
		prefix := ""
		prefixes := entDef.GetIDPrefixes()
		if len(prefixes) > 0 {
			prefix = prefixes[0]
		}
		entityID = model.GenerateNextID(a.g.IDsByType(form.EntityType), prefix)
	}

	entity := model.NewEntity(entityID, form.EntityType)

	// Set properties from form
	for _, f := range form.Fields {
		val := r.FormValue(f.Property)
		if val == "" && f.Default != "" {
			val = f.Default
		}
		if val == "" && f.Hidden {
			val = f.Default
		}
		if val != "" {
			entity.Properties[f.Property] = val
		}
	}

	// Set body content if form has body enabled
	if form.Body != nil && *form.Body {
		entity.Content = r.FormValue("_body")
	}

	// Write entity to disk
	plural := entDef.GetDirPlural(form.EntityType)
	filePath := filepath.Join(a.projCtx.EntitiesDir, plural, entityID+".md")
	if err := markdown.WriteEntity(entity, filePath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write entity: %v", err), 500)
		return
	}
	entity.FilePath = filePath
	entity.ModTime = time.Now()

	// Add to graph
	a.g.AddNode(entity)

	// Create relations
	for _, rel := range form.Relations {
		values := r.Form[rel.Relation]
		for _, targetID := range values {
			if targetID == "" {
				continue
			}
			relation := model.NewRelation(entityID, rel.Relation, targetID)
			// Set relation properties from form
			for _, rp := range rel.Properties {
				propKey := fmt.Sprintf("_relprop_%s_%s_%s", rel.Relation, targetID, rp.Property)
				if pv := r.FormValue(propKey); pv != "" {
					relation.Properties[rp.Property] = pv
				}
			}
			relPath := a.projCtx.RelationFilePath(entityID, rel.Relation, targetID)
			if err := markdown.WriteRelation(relation, relPath); err != nil {
				log.Printf("Failed to write relation: %v", err)
				continue
			}
			relation.FilePath = relPath
			a.g.AddEdge(relation)
		}
	}

	log.Printf("Created %s %s", form.EntityType, entityID)

	// HTMX: redirect to entity detail
	w.Header().Set("HX-Redirect", "/entity/"+entityID)
	w.WriteHeader(200)
}

func (a *App) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	r.ParseForm()

	entityID := r.FormValue("_entity_id")
	formID := r.FormValue("_form_id")

	entity, ok := a.g.GetNode(entityID)
	if !ok {
		http.Error(w, "Entity not found", 404)
		return
	}

	form, ok := a.cfg.Forms[formID]
	if !ok {
		http.Error(w, "Unknown form", 400)
		return
	}

	// Update properties
	for _, f := range form.Fields {
		val := r.FormValue(f.Property)
		if val != "" {
			entity.Properties[f.Property] = val
		}
	}

	// Update body content if form has body enabled
	if form.Body != nil && *form.Body {
		entity.Content = r.FormValue("_body")
	}

	// Write back to disk
	if err := markdown.WriteEntity(entity, entity.FilePath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write: %v", err), 500)
		return
	}

	// Update relations: remove old, add new
	for _, rel := range form.Relations {
		// Remove existing relations of this type
		for _, edge := range a.g.OutgoingEdges(entityID) {
			if edge.Type == rel.Relation {
				markdown.DeleteRelation(edge.FilePath)
				a.g.RemoveEdge(edge.From, edge.Type, edge.To)
			}
		}
		// Add new
		values := r.Form[rel.Relation]
		for _, targetID := range values {
			if targetID == "" {
				continue
			}
			relation := model.NewRelation(entityID, rel.Relation, targetID)
			// Set relation properties from form
			for _, rp := range rel.Properties {
				propKey := fmt.Sprintf("_relprop_%s_%s_%s", rel.Relation, targetID, rp.Property)
				if pv := r.FormValue(propKey); pv != "" {
					relation.Properties[rp.Property] = pv
				}
			}
			relPath := a.projCtx.RelationFilePath(entityID, rel.Relation, targetID)
			if err := markdown.WriteRelation(relation, relPath); err != nil {
				log.Printf("Failed to write relation: %v", err)
				continue
			}
			relation.FilePath = relPath
			a.g.AddEdge(relation)
		}
	}

	log.Printf("Updated %s", entityID)
	w.Header().Set("HX-Redirect", "/entity/"+entityID)
	w.WriteHeader(200)
}

func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	r.ParseForm()

	entityID := r.FormValue("_entity_id")
	entity, ok := a.g.GetNode(entityID)
	if !ok {
		http.Error(w, "Entity not found", 404)
		return
	}

	// Delete relation files
	for _, edge := range a.g.OutgoingEdges(entityID) {
		markdown.DeleteRelation(edge.FilePath)
		a.g.RemoveEdge(edge.From, edge.Type, edge.To)
	}
	for _, edge := range a.g.IncomingEdges(entityID) {
		markdown.DeleteRelation(edge.FilePath)
		a.g.RemoveEdge(edge.From, edge.Type, edge.To)
	}

	// Delete entity file
	if err := markdown.DeleteEntity(entity.FilePath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete: %v", err), 500)
		return
	}
	a.g.RemoveNode(entityID)

	log.Printf("Deleted %s", entityID)
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(200)
}

func (a *App) handleInlineCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(405)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}
	r.ParseMultipartForm(10 << 20) // 10 MB max

	formID := r.FormValue("_form_id")
	form, ok := a.cfg.Forms[formID]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown form"})
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	// Generate ID
	var entityID string
	if entDef.GetIDType() == metamodel.IDTypeManual {
		entityID = r.FormValue("_entity_id")
		if entityID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "ID is required"})
			return
		}
	} else {
		prefix := ""
		prefixes := entDef.GetIDPrefixes()
		if len(prefixes) > 0 {
			prefix = prefixes[0]
		}
		entityID = model.GenerateNextID(a.g.IDsByType(form.EntityType), prefix)
	}

	entity := model.NewEntity(entityID, form.EntityType)

	// Set properties from form
	for _, f := range form.Fields {
		val := r.FormValue(f.Property)
		if val == "" && f.Default != "" {
			val = f.Default
		}
		if val != "" {
			entity.Properties[f.Property] = val
		}
	}

	// Write entity to disk
	plural := entDef.GetDirPlural(form.EntityType)
	filePath := filepath.Join(a.projCtx.EntitiesDir, plural, entityID+".md")
	if err := markdown.WriteEntity(entity, filePath); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	entity.FilePath = filePath
	entity.ModTime = time.Now()
	a.g.AddNode(entity)

	log.Printf("Inline-created %s %s", form.EntityType, entityID)

	// Return JSON with new entity info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":    entityID,
		"title": a.entityDisplayTitle(entity),
	})
}

func (a *App) handleInlineForm(w http.ResponseWriter, r *http.Request) {
	formID := strings.TrimPrefix(r.URL.Path, "/api/inline-form/")
	form, ok := a.cfg.Forms[formID]
	if !ok {
		http.Error(w, "Unknown form", 404)
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	// Render minimal form fields HTML
	var sb strings.Builder

	// Manual ID field
	if entDef.GetIDType() == metamodel.IDTypeManual {
		sb.WriteString(`<div class="form-group">`)
		sb.WriteString(`<label>ID<span class="required">*</span></label>`)
		sb.WriteString(`<input type="text" name="_entity_id" required placeholder="Unique ID...">`)
		sb.WriteString(`</div>`)
	}

	for _, f := range form.Fields {
		if f.Hidden {
			sb.WriteString(fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`, f.Property, f.Default))
			continue
		}
		prop := entDef.Properties[f.Property]
		widget := resolveWidget(f.Widget, prop, a.meta)
		label := coalesce(f.Label, titleCase(f.Property))
		required := prop.Required
		if f.Required != nil {
			required = *f.Required
		}

		sb.WriteString(`<div class="form-group">`)
		reqMark := ""
		if required {
			reqMark = `<span class="required">*</span>`
		}

		if widget == "checkbox" {
			sb.WriteString(fmt.Sprintf(`<div class="form-row-checkbox"><input type="checkbox" name="%s" value="true" id="ic-%s"><label for="ic-%s">%s</label></div>`, f.Property, f.Property, f.Property, label))
		} else if widget == "textarea" {
			sb.WriteString(fmt.Sprintf(`<label for="ic-%s">%s%s</label>`, f.Property, label, reqMark))
			sb.WriteString(fmt.Sprintf(`<textarea name="%s" id="ic-%s" placeholder="%s" style="min-height:60px;"></textarea>`, f.Property, f.Property, f.Placeholder))
		} else if widget == "select" || widget == "multi-select" {
			sb.WriteString(fmt.Sprintf(`<label for="ic-%s">%s%s</label>`, f.Property, label, reqMark))
			vals := resolvePropertyValues(prop, a.meta)
			defaultVal := coalesce(f.Default, prop.Default)
			sb.WriteString(fmt.Sprintf(`<select name="%s" id="ic-%s">`, f.Property, f.Property))
			sb.WriteString(`<option value="">Select...</option>`)
			for _, v := range vals {
				sel := ""
				if v == defaultVal {
					sel = " selected"
				}
				sb.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, v, sel, v))
			}
			sb.WriteString(`</select>`)
		} else {
			inputType := widgetToInputType(widget)
			sb.WriteString(fmt.Sprintf(`<label for="ic-%s">%s%s</label>`, f.Property, label, reqMark))
			reqAttr := ""
			if required {
				reqAttr = " required"
			}
			sb.WriteString(fmt.Sprintf(`<input type="%s" name="%s" id="ic-%s" placeholder="%s"%s>`, inputType, f.Property, f.Property, f.Placeholder, reqAttr))
		}
		sb.WriteString(`</div>`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(sb.String()))
}

// ── Helpers ──

func applyFilters(entities []*model.Entity, filters []FilterConfig) []*model.Entity {
	if len(filters) == 0 {
		return entities
	}
	var result []*model.Entity
	for _, e := range entities {
		match := true
		for _, f := range filters {
			if strings.HasPrefix(f.Value, "$") {
				continue // skip variable substitution in prototype
			}
			val := fmt.Sprintf("%v", e.Properties[f.Property])
			if e.Properties[f.Property] == nil {
				val = ""
			}
			switch f.Operator {
			case "=":
				if val != f.Value {
					match = false
				}
			case "!=":
				if val == f.Value {
					match = false
				}
			}
		}
		if match {
			result = append(result, e)
		}
	}
	return result
}

func sortEntities(entities []*model.Entity, cfg *SortConfig) {
	if cfg == nil || cfg.Property == "" {
		return
	}
	sort.Slice(entities, func(i, j int) bool {
		vi := fmt.Sprintf("%v", entities[i].Properties[cfg.Property])
		vj := fmt.Sprintf("%v", entities[j].Properties[cfg.Property])
		if entities[i].Properties[cfg.Property] == nil {
			vi = ""
		}
		if entities[j].Properties[cfg.Property] == nil {
			vj = ""
		}
		if cfg.Direction == "desc" {
			return vi > vj
		}
		return vi < vj
	})
}

func resolvePropertyValues(prop metamodel.PropertyDef, meta *metamodel.Metamodel) []string {
	if len(prop.Values) > 0 {
		return prop.Values
	}
	if ct, ok := meta.Types[prop.Type]; ok {
		return ct.Values
	}
	return nil
}

func resolveWidget(explicit string, prop metamodel.PropertyDef, meta *metamodel.Metamodel) string {
	if explicit != "" {
		return explicit
	}
	switch prop.Type {
	case "string":
		return "text"
	case "date":
		return "date"
	case "integer":
		return "number"
	case "boolean":
		return "checkbox"
	case "enum":
		return "select"
	default:
		if _, ok := meta.Types[prop.Type]; ok {
			return "select"
		}
		return "text"
	}
}

func widgetToInputType(widget string) string {
	switch widget {
	case "textarea":
		return "textarea"
	case "select", "multi-select":
		return "select"
	default:
		return widget
	}
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func titleCase(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// simpleMarkdownToHTML converts basic markdown to HTML.
// Handles: headers, paragraphs, bold, italic, inline code, code blocks, lists.
func simpleMarkdownToHTML(md string) template.HTML {
	if md == "" {
		return ""
	}
	lines := strings.Split(md, "\n")
	var out []string
	inCodeBlock := false
	inList := false
	var paragraph []string

	flushParagraph := func() {
		if len(paragraph) > 0 {
			text := strings.Join(paragraph, " ")
			text = inlineFormat(text)
			out = append(out, "<p>"+text+"</p>")
			paragraph = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code block toggle
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				out = append(out, "</code></pre>")
				inCodeBlock = false
			} else {
				flushParagraph()
				if inList {
					out = append(out, "</ul>")
					inList = false
				}
				out = append(out, "<pre><code>")
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			out = append(out, template.HTMLEscapeString(line))
			continue
		}

		// Empty line
		if trimmed == "" {
			flushParagraph()
			if inList {
				out = append(out, "</ul>")
				inList = false
			}
			continue
		}

		// Headers
		if strings.HasPrefix(trimmed, "### ") {
			flushParagraph()
			out = append(out, "<h5>"+inlineFormat(trimmed[4:])+"</h5>")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			flushParagraph()
			out = append(out, "<h4>"+inlineFormat(trimmed[3:])+"</h4>")
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			flushParagraph()
			out = append(out, "<h3>"+inlineFormat(trimmed[2:])+"</h3>")
			continue
		}

		// Unordered list
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			flushParagraph()
			if !inList {
				out = append(out, "<ul>")
				inList = true
			}
			out = append(out, "<li>"+inlineFormat(trimmed[2:])+"</li>")
			continue
		}

		// Ordered list
		if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			if idx := strings.Index(trimmed, ". "); idx > 0 && idx < 4 {
				flushParagraph()
				if !inList {
					out = append(out, "<ol>")
					inList = true
				}
				out = append(out, "<li>"+inlineFormat(trimmed[idx+2:])+"</li>")
				continue
			}
		}

		// Regular text — accumulate into paragraph
		if inList {
			out = append(out, "</ul>")
			inList = false
		}
		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	if inList {
		out = append(out, "</ul>")
	}
	if inCodeBlock {
		out = append(out, "</code></pre>")
	}

	return template.HTML(strings.Join(out, "\n"))
}

// inlineFormat handles bold, italic, inline code
func inlineFormat(s string) string {
	s = template.HTMLEscapeString(s)
	// Inline code (must be before bold/italic)
	for {
		start := strings.Index(s, "`")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+1:], "`")
		if end < 0 {
			break
		}
		end += start + 1
		s = s[:start] + "<code>" + s[start+1:end] + "</code>" + s[end+1:]
	}
	// Bold
	for {
		start := strings.Index(s, "**")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+2:], "**")
		if end < 0 {
			break
		}
		end += start + 2
		s = s[:start] + "<strong>" + s[start+2:end] + "</strong>" + s[end+2:]
	}
	// Italic
	for {
		start := strings.Index(s, "*")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+1:], "*")
		if end < 0 {
			break
		}
		end += start + 1
		s = s[:start] + "<em>" + s[start+1:end] + "</em>" + s[end+1:]
	}
	return s
}

func templateFuncs(styleMap map[string]map[string]string, styledTypes map[string]bool, meta *metamodel.Metamodel) template.FuncMap {
	return template.FuncMap{
		"join": strings.Join,
		"json": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"contains": func(slice []string, val string) bool {
			for _, s := range slice {
				if s == val {
					return true
				}
			}
			return false
		},
		// badgeClass looks up the CSS class for a value based on its property type.
		// propType is the metamodel type name (e.g., "ticket_status", "priority").
		"badgeClass": func(propType, val string) string {
			if vals, ok := styleMap[propType]; ok {
				if cls, ok := vals[val]; ok {
					return cls
				}
			}
			return "badge-gray"
		},
		// isBadgeType returns true if the property type has style definitions
		// (explicit or auto-assigned for enum/custom types).
		"isBadgeType": func(propType string) bool {
			return styledTypes[propType]
		},
		// renderMarkdown converts markdown content to HTML
		"renderMarkdown": simpleMarkdownToHTML,
		// formatValue formats a property value for display (e.g., dates)
		"formatValue": func(val string) string {
			// Try to detect and reformat date values
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				return t.Format("2006-01-02")
			}
			if t, err := time.Parse("2006-01-02 15:04:05 -0700 MST", val); err == nil {
				return t.Format("2006-01-02")
			}
			if t, err := time.Parse("2006-01-02 15:04:05 +0000 UTC", val); err == nil {
				return t.Format("2006-01-02")
			}
			return val
		},
	}
}

// ── HTML Templates ──

const allTemplates = `
{{- define "head" -}}
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<script src="https://unpkg.com/htmx.org@2.0.4"></script>
<link rel="stylesheet" href="https://unpkg.com/easymde@2.18.0/dist/easymde.min.css">
<script src="https://unpkg.com/easymde@2.18.0/dist/easymde.min.js"></script>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  --bg: #f8fafc; --bg-card: #fff; --bg-sidebar: #1e293b; --bg-sidebar-hover: #334155;
  --bg-sidebar-active: #0f172a; --text: #1e293b; --text-muted: #64748b;
  --text-sidebar: #cbd5e1; --text-sidebar-active: #fff; --border: #e2e8f0;
  --primary: #3b82f6; --primary-hover: #2563eb; --primary-light: #eff6ff;
  --danger: #ef4444; --radius: 8px; --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono: "SF Mono", "Fira Code", monospace;
  --shadow: 0 1px 3px rgba(0,0,0,0.08);
}
body { font-family: var(--font); background: var(--bg); color: var(--text); line-height: 1.6; display: flex; min-height: 100vh; }

.sidebar { width: 240px; background: var(--bg-sidebar); position: fixed; top: 0; left: 0; bottom: 0; overflow-y: auto; z-index: 100; display: flex; flex-direction: column; }
.sidebar-header { padding: 20px 20px 16px; border-bottom: 1px solid rgba(255,255,255,0.1); }
.sidebar-header h1 { font-size: 16px; font-weight: 700; color: #fff; }
.sidebar-header p { font-size: 12px; color: var(--text-sidebar); margin-top: 4px; }
.sidebar nav { padding: 8px 0; flex: 1; }
.sidebar nav a { display: flex; align-items: center; gap: 10px; padding: 8px 20px; color: var(--text-sidebar); text-decoration: none; font-size: 14px; font-weight: 500; transition: all 0.15s; border-left: 3px solid transparent; }
.sidebar nav a:hover { background: var(--bg-sidebar-hover); color: var(--text-sidebar-active); }
.sidebar nav a.active { background: var(--bg-sidebar-active); color: var(--text-sidebar-active); border-left-color: var(--primary); }

.main { margin-left: 240px; flex: 1; padding: 32px; max-width: 1100px; }
.page-header { margin-bottom: 24px; display: flex; align-items: center; justify-content: space-between; }
.page-header h2 { font-size: 22px; font-weight: 700; }
.page-header p { color: var(--text-muted); font-size: 14px; margin-top: 2px; }

.card { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow); }

.filter-bar { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; margin-bottom: 16px; }
.filter-bar label { font-size: 12px; color: var(--text-muted); font-weight: 500; }
.filter-bar select, .filter-bar input { padding: 6px 10px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; font-family: var(--font); background: var(--bg-card); min-width: 140px; }

table { width: 100%; border-collapse: collapse; font-size: 14px; }
thead th { text-align: left; padding: 10px 16px; font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); border-bottom: 2px solid var(--border); white-space: nowrap; }
thead th.sortable { cursor: pointer; }
thead th.sortable:hover { color: var(--text); }
tbody td { padding: 12px 16px; border-bottom: 1px solid var(--border); }
tbody tr:hover { background: var(--primary-light); }
tbody tr:last-child td { border-bottom: none; }
.cell-link { color: var(--primary); text-decoration: none; font-weight: 500; }
.cell-link:hover { text-decoration: underline; }

.badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.03em; }
.badge-blue { background: #dbeafe; color: #1e40af; }
.badge-purple { background: #e9d5ff; color: #6b21a8; }
.badge-green { background: #dcfce7; color: #166534; }
.badge-gray { background: #f1f5f9; color: #475569; }
.badge-red { background: #fee2e2; color: #991b1b; }
.badge-orange { background: #fed7aa; color: #9a3412; }
.badge-yellow { background: #fef9c3; color: #854d0e; }

.info-bar { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 12px; }
.info-chip { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; background: var(--primary-light); color: var(--primary); border-radius: 9999px; font-size: 12px; font-weight: 500; }

.btn { padding: 8px 20px; border: 1px solid var(--border); border-radius: 6px; font-size: 14px; font-weight: 500; cursor: pointer; font-family: var(--font); transition: all 0.15s; text-decoration: none; display: inline-flex; align-items: center; gap: 6px; }
.btn-primary { background: var(--primary); color: #fff; border-color: var(--primary); }
.btn-primary:hover { background: var(--primary-hover); }
.btn-secondary { background: var(--bg-card); color: var(--text); }
.btn-secondary:hover { background: var(--bg); }
.btn-sm { padding: 5px 12px; font-size: 13px; }
.btn-danger { background: #fff; color: var(--danger); border-color: var(--danger); }
.btn-danger:hover { background: #fef2f2; }

.form-card { padding: 28px; max-width: 640px; }
.form-desc { color: var(--text-muted); font-size: 13px; margin-bottom: 24px; }
.form-group { margin-bottom: 20px; }
.form-group label { display: block; font-size: 13px; font-weight: 600; margin-bottom: 6px; }
.form-group .required { color: var(--danger); margin-left: 2px; }
.form-group input[type="text"], .form-group input[type="date"], .form-group input[type="number"],
.form-group textarea, .form-group select { width: 100%; padding: 8px 12px; border: 1px solid var(--border); border-radius: 6px; font-size: 14px; font-family: var(--font); background: var(--bg-card); color: var(--text); transition: border-color 0.15s, box-shadow 0.15s; }
.form-group input:focus, .form-group textarea:focus, .form-group select:focus { outline: none; border-color: var(--primary); box-shadow: 0 0 0 3px var(--primary-light); }
.form-group textarea { min-height: 100px; resize: vertical; }
.form-group .help-text { font-size: 12px; color: var(--text-muted); margin-top: 4px; }
.form-group .field-meta { font-size: 11px; color: var(--text-muted); margin-top: 2px; font-family: var(--font-mono); }
.form-row-checkbox { display: flex; align-items: center; gap: 8px; }
.form-row-checkbox input[type="checkbox"] { width: 16px; height: 16px; accent-color: var(--primary); }
.form-row-checkbox label { margin-bottom: 0; font-weight: 500; }
.form-section-label { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); margin: 28px 0 16px; padding-top: 20px; border-top: 1px solid var(--border); }
.form-actions { margin-top: 28px; padding-top: 20px; border-top: 1px solid var(--border); display: flex; gap: 12px; }

.transitions-info { margin-top: 6px; padding: 8px 12px; background: #f8fafc; border-radius: 6px; border: 1px solid var(--border); }
.transitions-info .t-title { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); margin-bottom: 4px; }
.transitions-info .t-row { font-size: 12px; font-family: var(--font-mono); color: var(--text-muted); line-height: 1.8; }
.t-arrow { color: var(--primary); margin: 0 4px; }

.detail-grid { display: grid; grid-template-columns: 140px 1fr; gap: 8px 16px; font-size: 14px; }
.detail-label { color: var(--text-muted); font-weight: 500; font-size: 13px; }
.detail-value { font-weight: 400; }

.detail-section { margin-top: 24px; }
.detail-section h3 { font-size: 14px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); margin-bottom: 12px; }

.rel-list { list-style: none; }
.rel-list li { padding: 6px 0; border-bottom: 1px solid var(--border); font-size: 14px; display: flex; gap: 8px; align-items: center; }
.rel-list li:last-child { border-bottom: none; }
.rel-type { font-size: 11px; font-family: var(--font-mono); color: var(--text-muted); background: #f1f5f9; padding: 1px 6px; border-radius: 3px; }

.pagination { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; border-top: 1px solid var(--border); font-size: 13px; color: var(--text-muted); }

.toast { position: fixed; top: 16px; right: 16px; padding: 12px 20px; background: #166534; color: #fff; border-radius: 8px; font-size: 14px; z-index: 999; animation: fadeIn 0.2s; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(-8px); } to { opacity: 1; transform: translateY(0); } }

.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.4); z-index: 200; display: flex; align-items: center; justify-content: center; animation: fadeIn 0.15s; }
.modal { background: var(--bg-card); border-radius: 12px; box-shadow: 0 8px 32px rgba(0,0,0,0.2); width: 480px; max-width: 90vw; max-height: 80vh; overflow-y: auto; }
.modal-header { padding: 16px 20px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; }
.modal-header h3 { font-size: 16px; font-weight: 600; }
.modal-close { background: none; border: none; font-size: 20px; cursor: pointer; color: var(--text-muted); padding: 4px 8px; border-radius: 4px; }
.modal-close:hover { background: var(--bg); color: var(--text); }
.modal-body { padding: 20px; }
.modal-body .form-group { margin-bottom: 16px; }
.modal-footer { padding: 12px 20px; border-top: 1px solid var(--border); display: flex; gap: 8px; justify-content: flex-end; }

.rel-row { display: flex; gap: 8px; align-items: flex-start; margin-bottom: 8px; }
.rel-row .rel-select-wrap { flex: 1; }
.rel-row .rel-props { display: flex; gap: 6px; align-items: center; }
.rel-row .rel-props input { width: 120px; padding: 6px 8px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; }
.btn-icon { width: 34px; height: 34px; padding: 0; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--border); border-radius: 6px; background: var(--bg-card); cursor: pointer; font-size: 18px; color: var(--primary); transition: all 0.15s; flex-shrink: 0; }
.btn-icon:hover { background: var(--primary-light); border-color: var(--primary); }

.EasyMDEContainer { border: 1px solid var(--border); border-radius: 6px; }
.EasyMDEContainer .CodeMirror { border: none; border-radius: 0 0 6px 6px; font-family: var(--font-mono); font-size: 14px; }
.EasyMDEContainer .editor-toolbar { border-bottom: 1px solid var(--border); border-radius: 6px 6px 0 0; }

.view-section-heading { font-size: 15px; font-weight: 700; color: var(--text); margin: 0 0 10px; padding-bottom: 6px; border-bottom: 2px solid var(--border); }
.view-content-entity .markdown-body { font-size: 14px; line-height: 1.7; color: var(--text); }
.markdown-body h3 { font-size: 15px; font-weight: 600; margin: 16px 0 6px; }
.markdown-body h4 { font-size: 14px; font-weight: 600; margin: 14px 0 4px; }
.markdown-body h5 { font-size: 13px; font-weight: 600; margin: 12px 0 4px; }
.markdown-body p { margin: 8px 0; }
.markdown-body ul, .markdown-body ol { margin: 8px 0; padding-left: 24px; }
.markdown-body li { margin: 2px 0; }
.markdown-body pre { background: #f1f5f9; padding: 12px; border-radius: 6px; overflow-x: auto; font-family: var(--font-mono); font-size: 13px; margin: 8px 0; }
.markdown-body code { background: #f1f5f9; padding: 1px 4px; border-radius: 3px; font-family: var(--font-mono); font-size: 0.9em; }
.markdown-body pre code { background: none; padding: 0; }
.markdown-body strong { font-weight: 600; }
.markdown-body em { font-style: italic; }
</style>
{{- end -}}

{{- define "sidebar" -}}
<aside class="sidebar">
  <div class="sidebar-header">
    <h1>{{ .App.Name }}</h1>
    {{ if .App.Description }}<p>{{ .App.Description }}</p>{{ end }}
  </div>
  <nav>
    {{ range .Navigation }}
    <a href="/list/{{ .List }}"{{ if eq .List $.ActiveList }} class="active"{{ end }}
       hx-get="/list/{{ .List }}" hx-target="#content" hx-push-url="true">
      {{ .Label }}
    </a>
    {{ end }}
  </nav>
</aside>
{{- end -}}

{{- define "page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .List.Title }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "list-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "list-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .List.Title }}</h2>
    {{ if .List.Description }}<p>{{ .List.Description }}</p>{{ end }}
  </div>
  <div style="display:flex;gap:8px;align-items:center;">
    <span style="font-size:13px;color:var(--text-muted);">{{ .TotalCount }} items</span>
    {{ if .List.CreateForm }}
    <a href="/form/{{ .List.CreateForm }}" class="btn btn-primary btn-sm"
       hx-get="/form/{{ .List.CreateForm }}" hx-target="#content" hx-push-url="true">+ New</a>
    {{ end }}
  </div>
</div>

{{ if .FilterControls }}
<div class="filter-bar">
  {{ range .FilterControls }}
  <div>
    <label>{{ .Label }}</label><br>
    {{ if or (eq .Widget "select") (eq .Widget "multi-select") }}
    <select name="filter_{{ .Property }}"
            hx-get="/list/{{ $.ListID }}" hx-target="#content" hx-push-url="true"
            hx-include=".filter-bar select, .filter-bar input">
      <option value="">All</option>
      {{ $current := .Current }}
      {{ range .Values }}<option value="{{ . }}"{{ if eq . $current }} selected{{ end }}>{{ . }}</option>{{ end }}
    </select>
    {{ else }}
    <input type="text" placeholder="Search..." name="filter_{{ .Property }}"
           hx-get="/list/{{ $.ListID }}" hx-target="#content" hx-push-url="true"
           hx-trigger="keyup changed delay:300ms"
           hx-include=".filter-bar select, .filter-bar input">
    {{ end }}
  </div>
  {{ end }}
</div>
{{ end }}

{{ if .List.Filters }}
<div class="info-bar">
  {{ range .List.Filters }}
  <span class="info-chip">{{ .Property }} {{ .Operator }} {{ .Value }}</span>
  {{ end }}
</div>
{{ end }}

<div class="card">
  <div style="overflow-x:auto;">
    <table>
      <thead>
        <tr>
          {{ range .Columns }}<th{{ if .Sortable }} class="sortable"{{ end }}>{{ if .Label }}{{ .Label }}{{ else }}{{ .Property }}{{ end }}</th>{{ end }}
        </tr>
      </thead>
      <tbody>
        {{ range .Rows }}
        <tr>
          {{ $dlp := $.DetailLinkPrefix }}
          {{ range .Cells }}
          <td>
            {{ if .Link }}<a href="{{ $dlp }}{{ .EntityID }}" class="cell-link"
               hx-get="{{ $dlp }}{{ .EntityID }}" hx-target="#content" hx-push-url="true">{{ .Value }}</a>
            {{ else if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
            {{ else }}{{ if .Value }}{{ .Value }}{{ else }}&mdash;{{ end }}{{ end }}
          </td>
          {{ end }}
        </tr>
        {{ end }}
        {{ if not .Rows }}
        <tr><td colspan="{{ len .Columns }}" style="text-align:center;padding:32px;color:var(--text-muted);">No items found</td></tr>
        {{ end }}
      </tbody>
    </table>
  </div>
  <div class="pagination">
    <span>{{ .TotalCount }} items</span>
  </div>
</div>
{{- end -}}

{{- define "form-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .Form.Title }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "form-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "form-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .Form.Title }}{{ if .EntityID }} — {{ .EntityID }}{{ end }}</h2>
    {{ if .Form.Description }}<p>{{ .Form.Description }}</p>{{ end }}
  </div>
  <a href="javascript:history.back()" class="btn btn-secondary btn-sm">&larr; Back</a>
</div>

<div class="card form-card">
  <form {{ if eq .Mode "edit" }}hx-post="/api/update"{{ else }}hx-post="/api/create"{{ end }}
        hx-swap="none">
    <input type="hidden" name="_form_id" value="{{ .FormID }}">
    <input type="hidden" name="_entity_id" value="{{ .EntityID }}">

    {{ range .Fields }}
    {{ if .Hidden }}
    <input type="hidden" name="{{ .Property }}" value="{{ .Value }}">
    {{ else if eq .Widget "checkbox" }}
    <div class="form-group">
      <div class="form-row-checkbox">
        <input type="checkbox" name="{{ .Property }}" value="true" id="f-{{ .Property }}"{{ if eq .Value "true" }} checked{{ end }}>
        <label for="f-{{ .Property }}">{{ .Label }}</label>
      </div>
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
    </div>
    {{ else if eq .Widget "textarea" }}
    <div class="form-group">
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <textarea name="{{ .Property }}" id="f-{{ .Property }}" placeholder="{{ .Placeholder }}"{{ if .Required }} required{{ end }}>{{ .Value }}</textarea>
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
    </div>
    {{ else if or (eq .Widget "select") (eq .Widget "multi-select") }}
    <div class="form-group">
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <select name="{{ .Property }}" id="f-{{ .Property }}"{{ if eq .Widget "multi-select" }} multiple{{ end }}{{ if .Required }} required{{ end }}>
        {{ if ne .Widget "multi-select" }}<option value="">Select...</option>{{ end }}
        {{ $val := .Value }}
        {{ range .Values }}<option value="{{ . }}"{{ if eq . $val }} selected{{ end }}>{{ . }}</option>{{ end }}
      </select>
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
      {{ if .Transitions }}
      <div class="transitions-info">
        <p class="t-title">Allowed transitions</p>
        {{ range $from, $tos := .Transitions }}
        <div class="t-row">{{ $from }} <span class="t-arrow">&rarr;</span> {{ join $tos ", " }}</div>
        {{ end }}
      </div>
      {{ end }}
    </div>
    {{ else }}
    <div class="form-group">
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <input type="{{ .InputType }}" name="{{ .Property }}" id="f-{{ .Property }}"
             placeholder="{{ .Placeholder }}" value="{{ .Value }}"{{ if .Required }} required{{ end }}>
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
    </div>
    {{ end }}
    {{ end }}

    {{ if .ShowBody }}
    <p class="form-section-label">Content</p>
    <div class="form-group">
      <label for="body-editor">Body (Markdown)</label>
      <textarea name="_body" id="body-editor">{{ .Body }}</textarea>
    </div>
    {{ end }}

    {{ if .Relations }}
    <p class="form-section-label">Relations</p>
    {{ range .Relations }}
    <div class="form-group">
      <div style="display:flex;align-items:center;gap:8px;margin-bottom:6px;">
        <label for="r-{{ .Relation }}" style="margin-bottom:0;">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
        {{ if .AllowCreate }}
        <button type="button" class="btn-icon" onclick="openInlineCreate('{{ .CreateForm }}', '{{ .Relation }}', '{{ .TargetLabel }}')" title="Add new {{ .TargetLabel }}">+</button>
        {{ end }}
      </div>
      {{ if eq .Widget "multi-select" }}
      <select name="{{ .Relation }}" id="r-{{ .Relation }}" multiple{{ if .Required }} required{{ end }}>
        {{ $selected := .Selected }}
        {{ range .Options }}<option value="{{ .ID }}"{{ if contains $selected .ID }} selected{{ end }}>{{ .Title }}</option>{{ end }}
      </select>
      {{ else if eq .Widget "search" }}
      <input type="text" list="dl-{{ .Relation }}" name="{{ .Relation }}" placeholder="Search {{ .TargetLabel }}...">
      <datalist id="dl-{{ .Relation }}">
        {{ range .Options }}<option value="{{ .ID }}">{{ .Title }}</option>{{ end }}
      </datalist>
      {{ else }}
      <select name="{{ .Relation }}" id="r-{{ .Relation }}"{{ if .Required }} required{{ end }}>
        <option value="">Select {{ .TargetLabel }}...</option>
        {{ $selected := .Selected }}
        {{ range .Options }}<option value="{{ .ID }}"{{ if contains $selected .ID }} selected{{ end }}>{{ .Title }}</option>{{ end }}
      </select>
      {{ end }}
      {{ if .Properties }}
      <div class="rel-props-section" style="margin-top:8px;">
        <p style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.04em;color:var(--text-muted);margin-bottom:6px;">Relation Properties</p>
        {{ $rel := . }}
        {{ range .Properties }}
        {{ $rp := . }}
        <div style="display:flex;gap:8px;align-items:center;margin-bottom:4px;">
          <label style="font-size:12px;color:var(--text-muted);min-width:80px;margin-bottom:0;">{{ if .Label }}{{ .Label }}{{ else }}{{ .Property }}{{ end }}</label>
          <input type="text" name="_relprop_{{ $rel.Relation }}__{{ $rp.Property }}" placeholder="{{ .Property }}"
                 style="flex:1;padding:5px 8px;border:1px solid var(--border);border-radius:4px;font-size:13px;"
                 data-relprop-relation="{{ $rel.Relation }}" data-relprop-property="{{ $rp.Property }}">
        </div>
        {{ end }}
      </div>
      {{ end }}
      <p class="field-meta">{{ .Relation }} &rarr; {{ .TargetType }}</p>
    </div>
    {{ end }}
    {{ end }}

    <div class="form-actions">
      {{ if eq .Mode "edit" }}
      <button type="submit" class="btn btn-primary">Save Changes</button>
      <button type="button" class="btn btn-danger"
              hx-post="/api/delete" hx-vals='{"_entity_id":"{{ .EntityID }}"}'
              hx-confirm="Delete {{ .EntityID }}? This cannot be undone."
              hx-swap="none">Delete</button>
      {{ else }}
      <button type="submit" class="btn btn-primary">Create</button>
      {{ end }}
      <a href="javascript:history.back()" class="btn btn-secondary">Cancel</a>
    </div>
  </form>
</div>

<div id="inline-create-modal" class="modal-overlay" style="display:none;" onclick="if(event.target===this)closeInlineCreate()">
  <div class="modal">
    <div class="modal-header">
      <h3 id="inline-create-title">Add New</h3>
      <button class="modal-close" onclick="closeInlineCreate()">&times;</button>
    </div>
    <div class="modal-body" id="inline-create-body">
      <p style="color:var(--text-muted);">Loading...</p>
    </div>
    <div class="modal-footer">
      <button class="btn btn-secondary btn-sm" onclick="closeInlineCreate()">Cancel</button>
      <button class="btn btn-primary btn-sm" onclick="submitInlineCreate()">Create</button>
    </div>
  </div>
</div>

<script>
// EasyMDE initialization
(function() {
  var el = document.getElementById('body-editor');
  if (el) {
    new EasyMDE({
      element: el,
      spellChecker: false,
      status: false,
      minHeight: '200px',
      toolbar: ['bold', 'italic', 'heading', '|', 'unordered-list', 'ordered-list', '|', 'link', 'image', '|', 'preview', 'side-by-side', '|', 'guide'],
      sideBySideFullscreen: false,
    });
  }
})();

// Inline create modal
var _inlineRelation = '';
var _inlineFormID = '';

function openInlineCreate(formID, relation, targetLabel) {
  _inlineFormID = formID;
  _inlineRelation = relation;
  document.getElementById('inline-create-title').textContent = 'Add New ' + targetLabel;
  // Fetch form fields for the target form
  fetch('/api/inline-form/' + formID)
    .then(function(r) { return r.text(); })
    .then(function(html) {
      document.getElementById('inline-create-body').innerHTML = html;
    })
    .catch(function() {
      document.getElementById('inline-create-body').innerHTML = '<p style="color:var(--danger);">Failed to load form.</p>';
    });
  document.getElementById('inline-create-modal').style.display = 'flex';
}

function closeInlineCreate() {
  document.getElementById('inline-create-modal').style.display = 'none';
  document.getElementById('inline-create-body').innerHTML = '';
}

function submitInlineCreate() {
  var body = document.getElementById('inline-create-body');
  var inputs = body.querySelectorAll('input, textarea, select');
  var formData = new FormData();
  formData.append('_form_id', _inlineFormID);
  inputs.forEach(function(inp) {
    if (inp.name) {
      if (inp.type === 'checkbox') {
        if (inp.checked) formData.append(inp.name, inp.value);
      } else {
        formData.append(inp.name, inp.value);
      }
    }
  });

  fetch('/api/inline-create', { method: 'POST', body: formData })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.error) { alert('Error: ' + data.error); return; }
      // Add new option to the relation select
      var sel = document.getElementById('r-' + _inlineRelation);
      if (sel) {
        var opt = document.createElement('option');
        opt.value = data.id;
        opt.textContent = data.title;
        opt.selected = true;
        sel.appendChild(opt);
      }
      closeInlineCreate();
    })
    .catch(function(e) { alert('Error creating: ' + e); });
}
</script>
{{- end -}}

{{- define "entity-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .Entity.ID }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "entity-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "entity-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .Entity.Title }}{{ if not .Entity.Title }}{{ .Entity.ID }}{{ end }}</h2>
    <p style="font-family:var(--font-mono);font-size:13px;color:var(--text-muted);">{{ .Entity.ID }} &middot; {{ .Entity.Type }}</p>
  </div>
  <div style="display:flex;gap:8px;">
    {{ if .EditFormID }}
    <a href="/form/{{ .EditFormID }}/{{ .Entity.ID }}" class="btn btn-primary btn-sm"
       hx-get="/form/{{ .EditFormID }}/{{ .Entity.ID }}" hx-target="#content" hx-push-url="true">Edit</a>
    {{ end }}
    <a href="javascript:history.back()" class="btn btn-secondary btn-sm">&larr; Back</a>
  </div>
</div>

<div class="card" style="padding:24px;">
  <div class="detail-grid">
    {{ $propTypes := .PropTypes }}
    {{ range $key, $val := .Entity.Properties }}
    {{ $ptype := index $propTypes $key }}
    <div class="detail-label">{{ $key }}</div>
    <div class="detail-value">
      {{ if isBadgeType $ptype }}<span class="badge {{ badgeClass $ptype (printf "%v" $val) }}">{{ $val }}</span>
      {{ else }}{{ if $val }}{{ $val }}{{ else }}&mdash;{{ end }}{{ end }}
    </div>
    {{ end }}
  </div>

  {{ if .Relations }}
  <div class="detail-section">
    <h3>Relations</h3>
    <ul class="rel-list">
      {{ range .Relations }}
      <li>
        <span class="rel-type">{{ .Direction }} {{ .Type }}</span>
        <a href="/entity/{{ .TargetID }}" class="cell-link"
           hx-get="/entity/{{ .TargetID }}" hx-target="#content" hx-push-url="true">{{ .TargetTitle }}</a>
        {{ range .Properties }}
        <span style="font-size:11px;color:var(--text-muted);background:#f1f5f9;padding:1px 6px;border-radius:3px;">{{ .Key }}: {{ .Value }}</span>
        {{ end }}
      </li>
      {{ end }}
    </ul>
  </div>
  {{ end }}

  {{ if .Entity.Content }}
  <div class="detail-section">
    <h3>Content</h3>
    <div style="padding:12px;background:#f8fafc;border-radius:6px;font-size:14px;white-space:pre-wrap;font-family:var(--font-mono);">{{ .Entity.Content }}</div>
  </div>
  {{ end }}
</div>
{{- end -}}

{{- define "view-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .View.Title }}: {{ .EntryTitle }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "view-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "view-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .EntryTitle }}</h2>
    <p style="font-family:var(--font-mono);font-size:13px;color:var(--text-muted);">{{ .Entry.ID }} &middot; {{ .Entry.Type }} &middot; {{ .View.Title }}</p>
  </div>
  <a href="javascript:history.back()" class="btn btn-secondary btn-sm">&larr; Back</a>
</div>

{{ range .Sections }}
<div class="view-section" style="margin-bottom:24px;">

  {{ if .Heading }}<h3 class="view-section-heading">{{ .Heading }}</h3>{{ end }}

  {{/* ── display: properties ── */}}
  {{ if eq .Display "properties" }}
  <div class="card" style="padding:20px;">
    <div class="detail-grid">
      {{ range .Fields }}
      <div class="detail-label">{{ .Label }}</div>
      <div class="detail-value">
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}{{ if .Value }}{{ formatValue .Value }}{{ else }}&mdash;{{ end }}{{ end }}
      </div>
      {{ end }}
    </div>
  </div>
  {{ end }}

  {{/* ── display: content (entry) ── */}}
  {{ if and (eq .Display "content") .HasContent (not .Entities) }}
  <div class="card" style="padding:20px;">
    <div class="markdown-body">{{ renderMarkdown .Content }}</div>
  </div>
  {{ end }}

  {{/* ── display: content (collection) ── */}}
  {{ if and (eq .Display "content") .Entities }}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else }}
  {{ range .Entities }}
  <div class="card view-content-entity" style="padding:20px;margin-bottom:12px;">
    <div style="display:flex;align-items:center;gap:10px;margin-bottom:8px;">
      <a href="/entity/{{ .ID }}" class="cell-link" style="font-size:16px;font-weight:600;"
         hx-get="/entity/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
      <span style="font-size:11px;font-family:var(--font-mono);color:var(--text-muted);background:#f1f5f9;padding:1px 6px;border-radius:3px;">{{ .ID }}</span>
    </div>
    {{ if .Fields }}
    <div style="display:flex;gap:12px;flex-wrap:wrap;margin-bottom:10px;">
      {{ range .Fields }}
      {{ if .Value }}
      <span style="font-size:12px;color:var(--text-muted);">
        {{ .Label }}:
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}<strong>{{ formatValue .Value }}</strong>{{ end }}
      </span>
      {{ end }}
      {{ end }}
    </div>
    {{ end }}
    {{ if .HasContent }}
    <div class="markdown-body" style="border-top:1px solid var(--border);padding-top:12px;margin-top:4px;">
      {{ renderMarkdown .Content }}
    </div>
    {{ end }}
  </div>
  {{ end }}
  {{ end }}
  {{ end }}

  {{/* ── display: table ── */}}
  {{ if eq .Display "table" }}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else if .IsGrouped }}
  {{ range .Groups }}
  <h4 style="font-size:13px;font-weight:600;color:var(--text-muted);margin:16px 0 8px;text-transform:uppercase;letter-spacing:0.04em;">{{ .GroupName }}</h4>
  <div class="card" style="margin-bottom:12px;">
    <div style="overflow-x:auto;">
      <table>
        <tbody>
          {{ range .Rows }}
          <tr>
            {{ range .Cells }}
            <td>
              {{ if .Link }}<a href="/entity/{{ .EntityID }}" class="cell-link"
                 hx-get="/entity/{{ .EntityID }}" hx-target="#content" hx-push-url="true">{{ .Value }}</a>
              {{ else if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
              {{ else }}{{ if .Value }}{{ .Value }}{{ else }}&mdash;{{ end }}{{ end }}
            </td>
            {{ end }}
          </tr>
          {{ end }}
        </tbody>
      </table>
    </div>
  </div>
  {{ end }}
  {{ else }}
  <div class="card">
    <div style="overflow-x:auto;">
      <table>
        <thead>
          <tr>
            {{ range .Columns }}
            <th>{{ if .Label }}{{ .Label }}{{ else }}{{ .Property }}{{ end }}</th>
            {{ end }}
          </tr>
        </thead>
        <tbody>
          {{ range .Rows }}
          <tr>
            {{ range .Cells }}
            <td>
              {{ if .Link }}<a href="/entity/{{ .EntityID }}" class="cell-link"
                 hx-get="/entity/{{ .EntityID }}" hx-target="#content" hx-push-url="true">{{ .Value }}</a>
              {{ else if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
              {{ else }}{{ if .Value }}{{ .Value }}{{ else }}&mdash;{{ end }}{{ end }}
            </td>
            {{ end }}
          </tr>
          {{ end }}
        </tbody>
      </table>
    </div>
  </div>
  {{ end }}
  {{ end }}

  {{/* ── display: cards ── */}}
  {{ if eq .Display "cards" }}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else }}
  <div style="display:grid;grid-template-columns:repeat(auto-fill, minmax(300px, 1fr));gap:12px;">
    {{ range .Entities }}
    <div class="card" style="padding:16px;">
      <div style="margin-bottom:8px;">
        <a href="/entity/{{ .ID }}" class="cell-link" style="font-size:14px;font-weight:600;"
           hx-get="/entity/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
        <span style="font-size:10px;font-family:var(--font-mono);color:var(--text-muted);margin-left:6px;">{{ .ID }}</span>
      </div>
      {{ range .Fields }}
      {{ if .Value }}
      <div style="font-size:12px;margin-bottom:2px;">
        <span style="color:var(--text-muted);">{{ .Label }}:</span>
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}<strong>{{ formatValue .Value }}</strong>{{ end }}
      </div>
      {{ end }}
      {{ end }}
      {{ if .HasContent }}
      <div class="markdown-body" style="border-top:1px solid var(--border);padding-top:8px;margin-top:8px;font-size:13px;">
        {{ renderMarkdown .Content }}
      </div>
      {{ end }}
    </div>
    {{ end }}
  </div>
  {{ end }}
  {{ end }}

  {{/* ── display: list ── */}}
  {{ if eq .Display "list" }}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else }}
  <div class="card" style="padding:12px 20px;">
    <ul class="rel-list">
      {{ range .Entities }}
      <li>
        <a href="/entity/{{ .ID }}" class="cell-link"
           hx-get="/entity/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
        {{ range .Fields }}
        {{ if .Value }}
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}<span style="font-size:12px;color:var(--text-muted);">{{ .Value }}</span>{{ end }}
        {{ end }}
        {{ end }}
      </li>
      {{ end }}
    </ul>
  </div>
  {{ end }}
  {{ end }}

</div>
{{ end }}
{{- end -}}
`
