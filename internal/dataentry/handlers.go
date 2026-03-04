package dataentry

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	htmltemplate "html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	first := firstNavTarget(a.Cfg.Navigation)
	if first != nil {
		if first.Dashboard {
			r.URL.Path = "/dashboard"
			a.handleDashboard(w, r)
			return
		}
		if first.Graph {
			http.Redirect(w, r, "/graph", http.StatusFound)
			return
		}
		// Rewrite path so handleList picks up the first navigation list.
		// This avoids an HTTP redirect which Wails AssetServer does not follow.
		r.URL.Path = "/list/" + first.List
		a.handleList(w, r)
		return
	}
	http.Error(w, "No navigation configured", http.StatusInternalServerError)
}

func (a *App) handleList(w http.ResponseWriter, r *http.Request) {
	listID := strings.TrimPrefix(r.URL.Path, "/list/")
	list, ok := a.Cfg.Lists[listID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	entDef, _ := a.meta.GetEntityDef(list.EntityType)

	entities := a.g.NodesByType(list.EntityType)
	entities = applyFilters(entities, list.Filters)

	// Apply query param filters
	for _, fc := range list.FilterControls {
		val := r.URL.Query().Get("filter_" + fc.Key())
		if val == "" {
			continue
		}
		if fc.IsRelation() {
			entities = a.filterByRelation(entities, fc.Relation, val)
		} else {
			entities = applyFilters(entities, []FilterConfig{{
				Property: fc.Property,
				Operator: "=",
				Value:    val,
			}})
		}
	}

	// Build active filter query params for URL construction
	var filterParams string
	for _, fc := range list.FilterControls {
		if val := r.URL.Query().Get("filter_" + fc.Key()); val != "" {
			filterParams += "&filter_" + url.QueryEscape(fc.Key()) + "=" + url.QueryEscape(val)
		}
	}

	// Resolve effective sort (query params override config default)
	sortProp := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("sort_dir")
	if sortProp != "" {
		a.sortEntitiesMulti(entities, []model.SortSpec{{Property: sortProp, Direction: sortDir}})
	} else {
		a.sortEntitiesMulti(entities, list.Sort)
		if len(list.Sort) > 0 {
			sortProp = list.Sort[0].Property
			sortDir = list.Sort[0].Direction
		}
	}

	// Pagination
	totalCount := len(entities)
	page := 1
	totalPages := 1
	pageSize := list.PageSize
	if pageSize > 0 {
		if p := r.URL.Query().Get("page"); p != "" {
			if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
				page = pn
			}
		}
		totalPages = (totalCount + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		if page > totalPages {
			page = totalPages
		}
		start := (page - 1) * pageSize
		end := start + pageSize
		if end > totalCount {
			end = totalCount
		}
		if start < totalCount {
			entities = entities[start:end]
		} else {
			entities = nil
		}
	}

	// Resolve columns with values
	type CellData struct {
		Values     []string
		Property   string
		PropType   string
		Widget     string
		Link       string // resolved link URL or empty
		EntityID   string
		EntityType string
	}
	type RowData struct {
		EntityID   string
		EntityType string
		Cells      []CellData
	}

	rows := make([]RowData, 0, len(entities))
	for _, e := range entities {
		var cells []CellData
		for _, col := range list.Columns {
			cell := CellData{
				Property:   col.Property,
				Link:       a.resolveLinkTarget(col.Link, e.Type, e.ID),
				EntityID:   e.ID,
				EntityType: e.Type,
			}
			if col.Relation != "" {
				cell.Values = a.resolveRelationColumnValues(e.ID, col.Relation)
			} else {
				// Get property definition from metamodel
				var propDef metamodel.PropertyDef
				if entDef != nil {
					if pd, ok := entDef.Properties[col.Property]; ok {
						propDef = pd
					}
				}
				cell.PropType = propDef.Type
				cell.Widget = resolveWidget(propDef, a.meta)
				if vs := e.GetAttributeStrings(col.Property); vs != nil {
					cell.Values = vs
				} else if val := e.GetAttributeString(col.Property); val != "" {
					cell.Values = []string{val}
				}
			}
			cells = append(cells, cell)
		}
		rows = append(rows, RowData{EntityID: e.ID, EntityType: e.Type, Cells: cells})
	}

	// Resolve filter control values
	type ResolvedFC struct {
		Property string
		Label    string
		Widget   string
		Values   []string
		Current  string
	}
	filterControls := make([]ResolvedFC, 0, len(list.FilterControls))
	for _, fc := range list.FilterControls {
		label := fc.Label
		if label == "" {
			label = titleCase(fc.Key())
		}
		rfc := ResolvedFC{
			Property: fc.Key(),
			Label:    label,
			Current:  r.URL.Query().Get("filter_" + fc.Key()),
		}
		if fc.IsRelation() {
			allEntities := a.g.NodesByType(list.EntityType)
			rfc.Values = a.resolveRelationFilterValues(allEntities, fc.Relation)
			rfc.Widget = "select"
		} else {
			prop := entDef.Properties[fc.Property]
			rfc.Values = resolvePropertyValues(prop, a.meta)
			rfc.Widget = resolveWidget(prop, a.meta)
		}
		filterControls = append(filterControls, rfc)
	}

	// Resolve relation data for display
	type RelationInfo struct {
		TargetID    string
		TargetType  string
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
				TargetType:  target.Type,
				TargetTitle: a.entityDisplayTitle(target),
			})
		}
	}

	// Resolve detail link prefix
	detailLinkPrefix := "/entity/" + list.EntityType + "/"
	if list.DetailView != "" {
		detailLinkPrefix = "/view/" + list.DetailView + "/"
	}

	// Resolve columns with sort state
	type ResolvedColumn struct {
		Property string
		Label    string
		Sortable bool
		Link     string
		SortURL  string
		IsSorted bool
		SortDir  string
	}
	resolvedColumns := make([]ResolvedColumn, len(list.Columns))
	for i, col := range list.Columns {
		rc := ResolvedColumn{
			Property: col.Property,
			Label:    coalesce(col.Label, titleCase(col.Property)),
			Sortable: col.Sortable,
			Link:     col.Link,
		}
		if col.Sortable {
			newDir := "asc"
			if sortProp == col.Property && sortDir != "desc" {
				newDir = "desc"
			}
			rc.SortURL = fmt.Sprintf("/list/%s?sort=%s&sort_dir=%s%s", listID, url.QueryEscape(col.Property), newDir, filterParams)
			rc.IsSorted = sortProp == col.Property
			rc.SortDir = sortDir
		}
		resolvedColumns[i] = rc
	}

	// Build pagination URLs
	var prevPageURL, nextPageURL string
	if pageSize > 0 && totalPages > 1 {
		sortParams := ""
		if sortProp != "" {
			sortParams = "&sort=" + url.QueryEscape(sortProp) + "&sort_dir=" + url.QueryEscape(sortDir)
		}
		if page > 1 {
			prevPageURL = fmt.Sprintf("/list/%s?page=%d%s%s", listID, page-1, sortParams, filterParams)
		}
		if page < totalPages {
			nextPageURL = fmt.Sprintf("/list/%s?page=%d%s%s", listID, page+1, sortParams, filterParams)
		}
	}

	// Build scope params for detail links (sort + filter state for scope reconstruction)
	var scopeParams string
	if sortProp != "" {
		scopeParams += "&sort=" + url.QueryEscape(sortProp) + "&sort_dir=" + url.QueryEscape(sortDir)
	}
	scopeParams += filterParams

	data := map[string]interface{}{
		"App":              a.Cfg.App,
		"ConflictCount":    a.conflictCount(),
		"Navigation":       a.navElements(listID),
		"ActiveList":       listID,
		"List":             list,
		"ListID":           listID,
		"Columns":          resolvedColumns,
		"Rows":             rows,
		"FilterControls":   filterControls,
		"EntityRelations":  entityRelations,
		"TotalCount":       totalCount,
		"EditForm":         list.EditForm,
		"DetailLinkPrefix": detailLinkPrefix,
		"ScopeParams":      scopeParams,
		"IsHTMX":           r.Header.Get("HX-Request") == "true",
		"SortProperty":     sortProp,
		"SortDirection":    sortDir,
		"Page":             page,
		"TotalPages":       totalPages,
		"PrevPageURL":      prevPageURL,
		"NextPageURL":      nextPageURL,
		"HasPagination":    pageSize > 0 && totalPages > 1,
		"Commands":         a.resolveCommands("list", listID, list.EntityType),
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "list-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "page", data) //nolint:errcheck // template errors logged by http
	}
}

func (a *App) handleForm(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/form/"), "/", 2)
	formID := parts[0]
	var entityID string
	if len(parts) > 1 {
		entityID = parts[1]
	}

	form, ok := a.Cfg.Forms[formID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	// Load templates (only relevant for create mode)
	var templates []*markdown.EntityTemplate
	var selectedTemplate *markdown.EntityTemplate
	selectedTemplateName := r.URL.Query().Get("template")

	if entityID == "" { // create mode
		templates = a.templatesForType(form.EntityType)
		// Find the selected template
		for _, t := range templates {
			if t.Name == selectedTemplateName {
				selectedTemplate = t
				break
			}
		}
		// If no template selected and templates exist, use the first (default)
		if selectedTemplate == nil && len(templates) > 0 {
			selectedTemplate = templates[0]
			selectedTemplateName = selectedTemplate.Name
		}
	}

	// Parse prop.* and rel.* query params for pre-filling (create:// link support)
	queryProps := make(map[string][]string) // prop name -> list of values (supports multi-select)
	queryRels := make(map[string][]string)  // rel type -> list of entity IDs
	for key, values := range r.URL.Query() {
		if strings.HasPrefix(key, "prop.") && len(values) > 0 {
			propName := strings.TrimPrefix(key, "prop.")
			queryProps[propName] = append(queryProps[propName], values...)
		} else if strings.HasPrefix(key, "rel.") && len(values) > 0 {
			relType := strings.TrimPrefix(key, "rel.")
			queryRels[relType] = append(queryRels[relType], values...)
		}
	}

	// Resolve fields
	type ResolvedField struct {
		Property       string
		Label          string
		Placeholder    string
		Help           string
		Required       bool
		Default        string
		Value          string
		SelectedValues []string // for multi-select widgets
		Hidden         bool
		Widget         string
		InputType      string
		Values         []string
		Transitions    map[string][]string
		Error          string
	}

	var entity *model.Entity
	if entityID != "" {
		entity, _ = a.g.GetNode(entityID)
	}

	fields := make([]ResolvedField, 0, len(form.Fields))
	for _, f := range form.Fields {
		prop := entDef.Properties[f.Property]
		userDefault := ""
		if a.userDefaults != nil {
			userDefault = a.userDefaults.ResolvePropertyDefault(form.EntityType, f.Property)
		}
		// Get template default if available
		templateDefault := ""
		if selectedTemplate != nil {
			if val, ok := selectedTemplate.Properties[f.Property]; ok {
				templateDefault = fmt.Sprintf("%v", val)
			}
		}
		// Query param prop.* takes highest priority for create links
		queryPropValues := queryProps[f.Property]
		var queryPropDefault string
		if len(queryPropValues) > 0 {
			queryPropDefault = queryPropValues[0]
		}
		rf := ResolvedField{
			Property:    f.Property,
			Label:       coalesce(f.Label, titleCase(f.Property)),
			Placeholder: f.Placeholder,
			Help:        f.Help,
			Default:     coalesce(queryPropDefault, userDefault, templateDefault, f.Default, prop.Default),
			Hidden:      f.Hidden,
			Widget:      resolveWidget(prop, a.meta),
			Values:      resolvePropertyValues(prop, a.meta),
			Transitions: f.Transitions,
		}
		// For multi-select widgets, also populate SelectedValues from query params
		if len(queryPropValues) > 0 && entity == nil {
			rf.SelectedValues = queryPropValues
		}
		if f.Required != nil {
			rf.Required = *f.Required
		} else {
			rf.Required = prop.Required
		}
		rf.InputType = widgetToInputType(rf.Widget)

		if entity != nil {
			if vs := entity.GetAttributeStrings(f.Property); vs != nil {
				rf.SelectedValues = vs
				rf.Value = strings.Join(vs, ", ")
			} else if val := entity.Properties[f.Property]; val != nil {
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
	if showBody {
		if entity != nil {
			bodyContent = entity.Content
		} else if selectedTemplate != nil && selectedTemplate.Content != "" {
			bodyContent = selectedTemplate.Content
		}
	}

	// Resolve relation fields
	type ResolvedRelation struct {
		Relation      string
		Label         string
		Required      bool
		Widget        string
		TargetType    string
		TargetLabel   string
		Options       []struct{ ID, Title string }
		Selected      []string
		AllowCreate   bool
		CreateForm    string
		Properties    []RelationProperty
		SelectedProps map[string]map[string]string
	}
	linkRelation := r.URL.Query().Get("link_relation")
	linkPeer := r.URL.Query().Get("link_peer")

	relations := make([]ResolvedRelation, 0, len(form.Relations))
	for _, rel := range form.Relations {
		// display-only relations (cards, etc.) are not editable form fields
		if rel.Display != "" {
			continue
		}

		// Resolve direction from metamodel if not specified.
		direction := rel.Direction
		relDef, relDefOK := a.meta.GetRelationDef(rel.Relation)
		if direction == "" && relDefOK {
			inFrom := containsString(relDef.From, form.EntityType)
			inTo := containsString(relDef.To, form.EntityType)
			if inFrom && !inTo {
				direction = DirectionOutgoing
			} else if inTo && !inFrom {
				direction = DirectionIncoming
			}
			// If in both or neither, direction remains empty (caller must specify)
		}

		// Resolve target type from metamodel if not specified.
		targetType := rel.TargetType
		if targetType == "" && relDefOK {
			if direction == DirectionIncoming {
				if len(relDef.From) == 1 {
					targetType = relDef.From[0]
				}
			} else {
				if len(relDef.To) == 1 {
					targetType = relDef.To[0]
				}
			}
		}

		// Resolve label from metamodel if not specified.
		label := rel.Label
		if label == "" && relDefOK {
			label = relDef.Label
		}
		if label == "" {
			label = titleCase(rel.Relation)
		}

		targetDef, _ := a.meta.GetEntityDef(targetType)
		targetLabel := ""
		if targetDef != nil {
			targetLabel = targetDef.Label
		}

		rr := ResolvedRelation{
			Relation:      rel.Relation,
			Label:         label,
			Required:      rel.Required,
			Widget:        WidgetSelect,
			TargetType:    targetType,
			TargetLabel:   targetLabel,
			AllowCreate:   rel.AllowCreate,
			CreateForm:    rel.CreateForm,
			Properties:    rel.Properties,
			SelectedProps: make(map[string]map[string]string),
		}

		targets := a.g.NodesByType(targetType)
		for _, t := range targets {
			rr.Options = append(rr.Options, struct{ ID, Title string }{t.ID, a.entityDisplayTitle(t)})
		}

		if entity != nil {
			if direction == DirectionIncoming {
				for _, edge := range a.g.IncomingEdges(entity.ID) {
					if edge.Type == rel.Relation {
						rr.Selected = append(rr.Selected, edge.From)
						if len(rel.Properties) > 0 {
							props := make(map[string]string)
							for _, rp := range rel.Properties {
								if v, ok := edge.Properties[rp.Property]; ok {
									props[rp.Property] = fmt.Sprintf("%v", v)
								}
							}
							rr.SelectedProps[edge.From] = props
						}
					}
				}
			} else {
				for _, edge := range a.g.OutgoingEdges(entity.ID) {
					if edge.Type == rel.Relation {
						rr.Selected = append(rr.Selected, edge.To)
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
		}

		// Prefill relation from rel.* query params (highest priority for create:// links).
		if entity == nil {
			if targets, ok := queryRels[rel.Relation]; ok && len(targets) > 0 {
				rr.Selected = append(rr.Selected, targets...)
			}
		}

		// Prefill relation from link params (when creating from a view's "Add" button).
		if entity == nil && len(rr.Selected) == 0 && linkPeer != "" && linkRelation != "" {
			if a.isRelationLinked(rel.Relation, linkRelation) {
				rr.Selected = append(rr.Selected, linkPeer)
			}
		}

		// Prefill relation from user defaults (only when creating and not already selected).
		// User defaults take precedence over template defaults.
		if entity == nil && len(rr.Selected) == 0 && a.userDefaults != nil {
			if defaultTarget := a.userDefaults.ResolveRelationDefault(form.EntityType, rel.Relation); defaultTarget != "" {
				rr.Selected = append(rr.Selected, defaultTarget)
			}
		}

		// Prefill relation from template (only when creating and not already selected).
		if entity == nil && len(rr.Selected) == 0 && selectedTemplate != nil {
			for _, tr := range selectedTemplate.Relations {
				if tr.Relation == rel.Relation && tr.Target != "" {
					rr.Selected = append(rr.Selected, tr.Target)
				}
			}
		}

		relations = append(relations, rr)
	}

	var mode string
	if entityID != "" {
		mode = "edit"
	} else {
		mode = "create"
	}

	// Build side panel sections (only for edit mode with an existing entity).
	var sidePanelSections []SectionData
	if form.SidePanel != nil && entity != nil {
		sidePanelSections = a.executeSidePanel(form.SidePanel, entityID, form.EntityType)
	}

	activeList := a.resolveActiveList(form.EntityType, r)
	returnTo := r.URL.Query().Get("return_to")
	backURL := returnTo
	switch {
	case backURL != "":
		// keep explicit return_to
	case mode == "edit" && entityID != "":
		backURL = "/entity/" + form.EntityType + "/" + entityID
	case activeList != "":
		backURL = "/list/" + activeList
	default:
		backURL = "/"
	}

	// Build template options for the UI
	type TemplateOption struct {
		Name     string
		Label    string
		Selected bool
	}
	templateOptions := make([]TemplateOption, 0, len(templates))
	for _, t := range templates {
		label := t.Name
		if label == "" {
			label = "Default"
		} else {
			label = titleCase(t.Name)
		}
		templateOptions = append(templateOptions, TemplateOption{
			Name:     t.Name,
			Label:    label,
			Selected: t.Name == selectedTemplateName,
		})
	}
	showTemplates := len(templates) > 1
	usePills := len(templates) <= 4

	data := map[string]interface{}{
		"App":               a.Cfg.App,
		"ConflictCount":     a.conflictCount(),
		"Navigation":        a.navElements(activeList),
		"ActiveList":        activeList,
		"FormID":            formID,
		"Form":              form,
		"Fields":            fields,
		"Relations":         relations,
		"Mode":              mode,
		"EntityID":          entityID,
		"EntityType":        form.EntityType,
		"ShowBody":          showBody,
		"Body":              bodyContent,
		"ReturnTo":          returnTo,
		"BackURL":           backURL,
		"LinkRelation":      r.URL.Query().Get("link_relation"),
		"LinkPeer":          r.URL.Query().Get("link_peer"),
		"LinkAs":            r.URL.Query().Get("link_as"),
		"Prefix":            r.URL.Query().Get("prefix"),
		"IsHTMX":            r.Header.Get("HX-Request") == "true",
		"SidePanelSections": sidePanelSections,
		"Templates":         templateOptions,
		"ShowTemplates":     showTemplates,
		"UsePills":          usePills,
		"SelectedTemplate":  selectedTemplateName,
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "form-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "form-page", data) //nolint:errcheck // template errors logged by http
	}
}

func (a *App) handleEntity(w http.ResponseWriter, r *http.Request) {
	// Parse /entity/{type}/{id} or legacy /entity/{id}
	path := strings.TrimPrefix(r.URL.Path, "/entity/")
	parts := strings.SplitN(path, "/", 2)
	var entityID string
	if len(parts) == 2 && parts[1] != "" {
		// /entity/{type}/{id}
		entityID = parts[1]
	} else {
		// Legacy: /entity/{id}
		entityID = parts[0]
	}
	entity, ok := a.g.GetNode(entityID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Redirect legacy URLs to canonical /entity/{type}/{id}
	canonical := "/entity/" + entity.Type + "/" + entity.ID
	if r.URL.Path != canonical {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Replace-Url", canonical)
		} else {
			http.Redirect(w, r, canonical, http.StatusMovedPermanently)
			return
		}
	}

	entDef, _ := a.meta.GetEntityDef(entity.Type)

	editFormID := a.editFormForType(entity.Type)

	outgoing := a.g.OutgoingEdges(entityID)
	incoming := a.g.IncomingEdges(entityID)

	type RelPropDisplay struct {
		Key   string
		Value string
	}
	type RelDisplay struct {
		Type        string
		TargetID    string
		TargetType  string
		TargetTitle string
		Direction   string
		Properties  []RelPropDisplay
	}
	rels := make([]RelDisplay, 0, len(outgoing)+len(incoming))
	for _, e := range outgoing {
		target, ok := a.g.GetNode(e.To)
		title := e.To
		targetType := ""
		if ok {
			title = a.entityDisplayTitle(target)
			targetType = target.Type
		}
		rd := RelDisplay{e.Type, e.To, targetType, title, DirectionOutgoing, nil}
		propKeys := make([]string, 0, len(e.Properties))
		for k := range e.Properties {
			propKeys = append(propKeys, k)
		}
		natsort.Strings(propKeys)
		for _, k := range propKeys {
			rd.Properties = append(rd.Properties, RelPropDisplay{k, fmt.Sprintf("%v", e.Properties[k])})
		}
		rels = append(rels, rd)
	}
	for _, e := range incoming {
		source, ok := a.g.GetNode(e.From)
		title := e.From
		sourceType := ""
		if ok {
			title = a.entityDisplayTitle(source)
			sourceType = source.Type
		}
		rd := RelDisplay{e.Type, e.From, sourceType, title, DirectionIncoming, nil}
		propKeys := make([]string, 0, len(e.Properties))
		for k := range e.Properties {
			propKeys = append(propKeys, k)
		}
		natsort.Strings(propKeys)
		for _, k := range propKeys {
			rd.Properties = append(rd.Properties, RelPropDisplay{k, fmt.Sprintf("%v", e.Properties[k])})
		}
		rels = append(rels, rd)
	}

	propTypes := make(map[string]string)
	if entDef != nil {
		propTypeKeys := make([]string, 0, len(entDef.Properties))
		for propName := range entDef.Properties {
			propTypeKeys = append(propTypeKeys, propName)
		}
		natsort.Strings(propTypeKeys)
		for _, propName := range propTypeKeys {
			propTypes[propName] = entDef.Properties[propName].Type
		}
	}

	// Build return URL for edit links (preserves scope params)
	returnTo := r.URL.Path
	if r.URL.RawQuery != "" {
		returnTo += "?" + r.URL.RawQuery
	}

	entityActiveList := a.resolveActiveList(entity.Type, r)
	backURL := "/"
	if scope := r.URL.Query().Get("scope"); strings.HasPrefix(scope, "search:") {
		query := strings.TrimPrefix(scope, "search:")
		backURL = "/search?q=" + url.QueryEscape(query)
	} else if entityActiveList != "" {
		backURL = "/list/" + entityActiveList
	}
	data := map[string]interface{}{
		"App":           a.Cfg.App,
		"ConflictCount": a.conflictCount(),
		"Navigation":    a.navElements(entityActiveList),
		"ActiveList":    entityActiveList,
		"Entity":        entity,
		"EntityDef":     entDef,
		"EditFormID":    editFormID,
		"Relations":     rels,
		"PropTypes":     propTypes,
		"ReturnTo":      returnTo,
		"Scope":         a.resolveScope(entity.ID, r),
		"Commands":      a.resolveCommands("entity", "", entity.Type),
		"BackURL":       backURL,
		"IsHTMX":        r.Header.Get("HX-Request") == "true",
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "entity-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "entity-page", data) //nolint:errcheck // template errors logged by http
	}
}

func (a *App) handleView(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/view/"), "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "Usage: /view/{viewID}/{entityID}", http.StatusBadRequest)
		return
	}
	viewID := parts[0]
	entityID := parts[1]

	view, ok := a.Cfg.Views[viewID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	result, err := a.executeView(view, entityID)
	if err != nil {
		http.Error(w, fmt.Sprintf("View error: %v", err), http.StatusBadRequest)
		return
	}

	// Build section data for the template
	sections := a.buildSections(view.Sections, result)

	// Resolve add/link info for sections populated by traversal from "entry".
	for i, sec := range view.Sections {
		if sec.Source == "entry" {
			continue
		}
		for _, rule := range view.Traverse {
			if rule.CollectAs != sec.Source || rule.From != "entry" {
				continue
			}
			relName := rule.Follow
			linkAs := "to" // new entity is the target (outgoing from entry)
			if rule.FollowIncoming != "" {
				relName = rule.FollowIncoming
				linkAs = "from" // new entity is the source (incoming to entry)
			}
			relDef, ok := a.meta.GetRelationDef(relName)
			if !ok {
				break
			}
			// Determine valid target types for creation
			var candidateTypes []string
			if linkAs == "to" {
				candidateTypes = relDef.To
			} else {
				candidateTypes = relDef.From
			}
			var targets []SectionAddTarget
			for _, et := range candidateTypes {
				formID := a.createFormForType(et)
				if formID == "" {
					continue
				}
				label := et
				if ed, ok := a.meta.GetEntityDef(et); ok && ed.Label != "" {
					label = ed.Label
				}
				targets = append(targets, SectionAddTarget{
					EntityType: et, FormID: formID, Label: label,
				})
			}
			if len(targets) > 0 {
				sections[i].AddInfo = &SectionAddInfo{
					Relation: relName,
					LinkAs:   linkAs,
					PeerID:   result.Entry.ID,
					Targets:  targets,
				}
			}
			// Link existing: always available when candidate types exist
			if len(candidateTypes) > 0 {
				sections[i].LinkInfo = &SectionLinkInfo{
					Relation:    relName,
					LinkAs:      linkAs,
					PeerID:      result.Entry.ID,
					EntityTypes: candidateTypes,
				}
			}
			break
		}
	}

	// Build the return URL for edit links to redirect back to this view
	returnTo := r.URL.Path
	if r.URL.RawQuery != "" {
		returnTo += "?" + r.URL.RawQuery
	}

	viewActiveList := a.resolveActiveList(result.Entry.Type, r)
	backURL := "/"
	if viewActiveList != "" {
		backURL = "/list/" + viewActiveList
	}
	data := map[string]interface{}{
		"App":           a.Cfg.App,
		"ConflictCount": a.conflictCount(),
		"Navigation":    a.navElements(viewActiveList),
		"ActiveList":    viewActiveList,
		"View":          view,
		"ViewID":        viewID,
		"EntityID":      entityID,
		"Entry":         result.Entry,
		"EntryTitle":    a.entityDisplayTitle(result.Entry),
		"EditFormID":    a.editFormForType(result.Entry.Type),
		"ReturnTo":      returnTo,
		"Sections":      sections,
		"Scope":         a.resolveScope(entityID, r),
		"Commands":      a.resolveCommands("view", viewID, result.Entry.Type),
		"BackURL":       backURL,
		"IsHTMX":        r.Header.Get("HX-Request") == "true",
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "view-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "view-page", data) //nolint:errcheck // template errors logged by http
	}
}

// renderFormWithErrors re-renders the form with validation errors.
// fieldErrors is a map of property name to error message.
func (a *App) renderFormWithErrors(w http.ResponseWriter, r *http.Request, formID string, entity *model.Entity, fieldErrors map[string]string) {
	form, ok := a.Cfg.Forms[formID]
	if !ok {
		http.Error(w, "Unknown form", http.StatusBadRequest)
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	// Resolve fields with errors
	type ResolvedField struct {
		Property       string
		Label          string
		Placeholder    string
		Help           string
		Required       bool
		Default        string
		Value          string
		SelectedValues []string // for multi-select widgets
		Hidden         bool
		Widget         string
		InputType      string
		Values         []string
		Transitions    map[string][]string
		Error          string
	}

	fields := make([]ResolvedField, 0, len(form.Fields))
	for _, f := range form.Fields {
		prop := entDef.Properties[f.Property]
		rf := ResolvedField{
			Property:    f.Property,
			Label:       coalesce(f.Label, titleCase(f.Property)),
			Placeholder: f.Placeholder,
			Help:        f.Help,
			Default:     coalesce(f.Default, prop.Default),
			Hidden:      f.Hidden,
			Widget:      resolveWidget(prop, a.meta),
			Values:      resolvePropertyValues(prop, a.meta),
			Transitions: f.Transitions,
			Error:       fieldErrors[f.Property],
		}
		if f.Required != nil {
			rf.Required = *f.Required
		} else {
			rf.Required = prop.Required
		}
		rf.InputType = widgetToInputType(rf.Widget)

		// Use submitted value from entity properties
		if vs := entity.GetAttributeStrings(f.Property); vs != nil {
			rf.SelectedValues = vs
			rf.Value = strings.Join(vs, ", ")
		} else if val := entity.Properties[f.Property]; val != nil {
			rf.Value = fmt.Sprintf("%v", val)
		}

		fields = append(fields, rf)
	}

	// Resolve body content
	var bodyContent string
	showBody := form.Body != nil && *form.Body
	if showBody {
		bodyContent = entity.Content
	}

	// Resolve relation fields (similar to handleForm but using form values)
	type ResolvedRelation struct {
		Relation      string
		Label         string
		Required      bool
		Widget        string
		TargetType    string
		TargetLabel   string
		Options       []struct{ ID, Title string }
		Selected      []string
		AllowCreate   bool
		CreateForm    string
		Properties    []RelationProperty
		SelectedProps map[string]map[string]string
	}
	linkRelation := r.FormValue("_link_relation")
	linkPeer := r.FormValue("_link_peer")

	relations := make([]ResolvedRelation, 0, len(form.Relations))
	for _, rel := range form.Relations {
		if rel.Display != "" {
			continue
		}

		targetDef, _ := a.meta.GetEntityDef(rel.TargetType)
		targetLabel := ""
		if targetDef != nil {
			targetLabel = targetDef.Label
		}

		rr := ResolvedRelation{
			Relation:      rel.Relation,
			Label:         rel.Label,
			Required:      rel.Required,
			Widget:        WidgetSelect,
			TargetType:    rel.TargetType,
			TargetLabel:   targetLabel,
			AllowCreate:   rel.AllowCreate,
			CreateForm:    rel.CreateForm,
			Properties:    rel.Properties,
			SelectedProps: make(map[string]map[string]string),
		}

		targets := a.g.NodesByType(rel.TargetType)
		for _, t := range targets {
			rr.Options = append(rr.Options, struct{ ID, Title string }{t.ID, a.entityDisplayTitle(t)})
		}

		// Preserve submitted relation values from form
		rr.Selected = r.Form[rel.Relation]

		relations = append(relations, rr)
	}

	var mode string
	if entity.ID != "" {
		if _, exists := a.g.GetNode(entity.ID); exists {
			mode = "edit"
		} else {
			mode = "create"
		}
	} else {
		mode = "create"
	}

	activeList := a.resolveActiveList(form.EntityType, r)
	returnTo := r.FormValue("_return_to")
	backURL := returnTo
	switch {
	case backURL != "":
		// keep explicit return_to
	case mode == "edit" && entity.ID != "":
		backURL = "/entity/" + form.EntityType + "/" + entity.ID
	case activeList != "":
		backURL = "/list/" + activeList
	default:
		backURL = "/"
	}

	data := map[string]interface{}{
		"App":           a.Cfg.App,
		"ConflictCount": a.conflictCount(),
		"Navigation":    a.navElements(activeList),
		"ActiveList":    activeList,
		"FormID":        formID,
		"Form":          form,
		"Fields":        fields,
		"Relations":     relations,
		"Mode":          mode,
		"EntityID":      entity.ID,
		"EntityType":    form.EntityType,
		"ShowBody":      showBody,
		"Body":          bodyContent,
		"ReturnTo":      returnTo,
		"BackURL":       backURL,
		"LinkRelation":  linkRelation,
		"LinkPeer":      linkPeer,
		"LinkAs":        r.FormValue("_link_as"),
		"IsHTMX":        true, // Always true since this is a form submission response
		"HasErrors":     len(fieldErrors) > 0,
	}
	a.addGitData(data)

	// Return 422 Unprocessable Entity for validation errors.
	// HX-Retarget and HX-Reswap tell HTMX to swap the response into #content,
	// overriding the form's hx-swap="none" which is used for successful submissions.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("HX-Retarget", "#content")
	w.Header().Set("HX-Reswap", "innerHTML")
	w.WriteHeader(http.StatusUnprocessableEntity)
	a.tmpl.ExecuteTemplate(w, "form-content", data) //nolint:errcheck // template errors logged by http
}

// validationErrorsToFieldMap converts structured ValidationErrors to a map of field names to error messages.
func validationErrorsToFieldMap(errs []*metamodel.ValidationError) map[string]string {
	fieldErrors := make(map[string]string)
	for _, err := range errs {
		if err.Property != "" {
			fieldErrors[err.Property] = err.Message
		}
	}
	return fieldErrors
}

// generateEntityID returns a new entity ID, either from a manual form field or auto-generated.
// buildProperties reads form field values into a properties map, applying the default chain
// (user defaults → form default) for empty values. Used by create handlers to supply
// properties to workspace.CreateEntity.
func (a *App) buildProperties(fields []FormField, entDef *metamodel.EntityDef, entityType string, r *http.Request) map[string]interface{} {
	props := make(map[string]interface{})
	for _, f := range fields {
		prop := entDef.Properties[f.Property]
		widget := resolveWidget(prop, a.meta)
		if widget == WidgetMultiSelect {
			values := r.Form[f.Property]
			if len(values) > 0 {
				props[f.Property] = values
			}
		} else {
			val := r.FormValue(f.Property)
			if val == "" && a.userDefaults != nil {
				val = a.userDefaults.ResolvePropertyDefault(entityType, f.Property)
			}
			if val == "" && f.Default != "" {
				val = f.Default
			}
			if val == "" && f.Hidden {
				val = f.Default
			}
			if val != "" {
				props[f.Property] = val
			}
		}
	}
	return props
}

// createFormRelations creates relations from form relation fields, applying user-default
// relations when no value is submitted.
func (a *App) createFormRelations(entityID string, relations []FormRelation, entityType string, r *http.Request) {
	for _, rel := range relations {
		if rel.Display != "" {
			continue
		}
		// Resolve direction from metamodel if not specified in config (mirrors handleForm logic).
		direction := rel.Direction
		if direction == "" {
			if relDef, ok := a.meta.GetRelationDef(rel.Relation); ok {
				inFrom := containsString(relDef.From, entityType)
				inTo := containsString(relDef.To, entityType)
				if inFrom && !inTo {
					direction = DirectionOutgoing
				} else if inTo && !inFrom {
					direction = DirectionIncoming
				}
			}
		}
		values := r.Form[rel.Relation]
		if len(values) == 0 && a.userDefaults != nil {
			if defaultTarget := a.userDefaults.ResolveRelationDefault(entityType, rel.Relation); defaultTarget != "" {
				values = []string{defaultTarget}
			}
		}
		for _, targetID := range values {
			if targetID == "" {
				continue
			}
			var fromID, toID string
			if direction == DirectionIncoming {
				fromID, toID = targetID, entityID
			} else {
				fromID, toID = entityID, targetID
			}
			// Collect relation properties from form.
			relProps := make(map[string]interface{})
			for _, rp := range rel.Properties {
				propKey := fmt.Sprintf("_relprop_%s_%s_%s", rel.Relation, targetID, rp.Property)
				if pv := r.FormValue(propKey); pv != "" {
					relProps[rp.Property] = pv
				}
			}
			var opts []workspace.CreateRelationOptions
			if len(relProps) > 0 {
				opts = append(opts, workspace.CreateRelationOptions{Properties: relProps})
			}
			if _, err := a.ws.CreateRelation(fromID, rel.Relation, toID, opts...); err != nil {
				log.Printf("Failed to write relation: %v", err)
				continue
			}
		}
	}
}

// createLinkRelation creates a single relation to a peer entity when triggered from a
// view's "Add" button (_link_relation, _link_peer, _link_as form params).
func (a *App) createLinkRelation(entityID string, r *http.Request) {
	linkRelation := r.FormValue("_link_relation")
	if linkRelation == "" {
		return
	}
	linkPeer := r.FormValue("_link_peer")
	linkAs := r.FormValue("_link_as")
	_, relOK := a.meta.GetRelationDef(linkRelation)
	_, peerOK := a.g.GetNode(linkPeer)
	if linkPeer == "" || (linkAs != "from" && linkAs != "to") || !relOK || !peerOK {
		return
	}
	var fromID, toID string
	if linkAs == "from" {
		fromID, toID = entityID, linkPeer
	} else {
		fromID, toID = linkPeer, entityID
	}
	if _, err := a.ws.CreateRelation(fromID, linkRelation, toID); err != nil {
		log.Printf("Failed to write link relation: %v", err)
	}
}

// createTemplateRelations creates relations defined in the selected (or default) entity
// template, skipping any that were already created via form or link relations.
func (a *App) createTemplateRelations(entityID, entityType string, r *http.Request) {
	templateName := r.FormValue("_template")
	templates := a.templatesForType(entityType)
	var selectedTemplate *markdown.EntityTemplate
	for _, t := range templates {
		if t.Name == templateName {
			selectedTemplate = t
			break
		}
	}
	if selectedTemplate == nil && len(templates) > 0 {
		selectedTemplate = templates[0]
	}
	if selectedTemplate == nil {
		return
	}

	// Build set of already-created relations to avoid duplicates
	createdRelations := make(map[string]bool)
	for _, edge := range a.g.OutgoingEdges(entityID) {
		createdRelations[edge.Type+"->"+edge.To] = true
	}
	for _, edge := range a.g.IncomingEdges(entityID) {
		createdRelations[edge.Type+"<-"+edge.From] = true
	}

	for _, tr := range selectedTemplate.Relations {
		if tr.Target == "" {
			continue
		}
		if _, exists := a.g.GetNode(tr.Target); !exists {
			log.Printf("Template relation target %s not found, skipping", tr.Target)
			continue
		}

		relDef, relOK := a.meta.GetRelationDef(tr.Relation)
		if !relOK {
			log.Printf("Unknown relation type %s in template, skipping", tr.Relation)
			continue
		}

		isFrom := containsString(relDef.From, entityType)
		isTo := containsString(relDef.To, entityType)

		var fromID, toID, key string
		switch {
		case isFrom && !isTo:
			fromID, toID = entityID, tr.Target
			key = tr.Relation + "->" + tr.Target
		case isTo && !isFrom:
			fromID, toID = tr.Target, entityID
			key = tr.Relation + "<-" + tr.Target
		default:
			fromID, toID = entityID, tr.Target
			key = tr.Relation + "->" + tr.Target
		}

		if createdRelations[key] {
			continue
		}

		if _, err := a.ws.CreateRelation(fromID, tr.Relation, toID); err != nil {
			log.Printf("Failed to write template relation: %v", err)
			continue
		}
		createdRelations[key] = true
	}
}

func (a *App) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm() //nolint:errcheck // form parse errors are handled by empty values

	formID := r.FormValue("_form_id")
	form, ok := a.Cfg.Forms[formID]
	if !ok {
		http.Error(w, "Unknown form", http.StatusBadRequest)
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	// Build properties from form fields.
	props := a.buildProperties(form.Fields, entDef, form.EntityType, r)

	var content string
	if form.Body != nil && *form.Body {
		content = r.FormValue("_body")
	}

	opts := workspace.CreateOptions{
		Properties: props,
		Content:    content,
		Prefix:     r.FormValue("_prefix"),
	}

	// Manual-ID types provide the ID directly; auto-ID types let workspace generate.
	if entDef.IsManualID() {
		opts.ID = r.FormValue("_entity_id")
		if opts.ID == "" {
			http.Error(w, "manual ID required", http.StatusBadRequest)
			return
		}
	}

	entity, _, err := a.ws.CreateEntity(form.EntityType, opts)
	if err != nil {
		var ve *workspace.ValidationError
		if errors.As(err, &ve) {
			// Re-render the form with validation errors.
			stub := model.NewEntity(opts.ID, form.EntityType)
			stub.Properties = props
			stub.Content = content
			a.renderFormWithErrors(w, r, formID, stub, validationErrorsToFieldMap(ve.Errors))
			return
		}
		http.Error(w, fmt.Sprintf("Failed to create entity: %v", err), http.StatusInternalServerError)
		return
	}

	a.createFormRelations(entity.ID, form.Relations, form.EntityType, r)
	a.createLinkRelation(entity.ID, r)
	a.createTemplateRelations(entity.ID, form.EntityType, r)

	log.Printf("Created %s %s", form.EntityType, entity.ID)

	redirect := "/entity/" + form.EntityType + "/" + entity.ID
	if returnTo := r.FormValue("_return_to"); returnTo != "" && strings.HasPrefix(returnTo, "/") {
		redirect = returnTo
		// Add hash fragment to scroll to new entity (if no hash already present)
		if !strings.Contains(redirect, "#") {
			redirect = redirect + "#" + strings.ToLower(entity.ID)
		}
	}
	w.Header().Set("HX-Redirect", appendToastParam(redirect, "Created "+entity.ID))
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm() //nolint:errcheck // form parse errors are handled by empty values

	entityID := r.FormValue("_entity_id")
	formID := r.FormValue("_form_id")

	entity, ok := a.g.GetNode(entityID)
	if !ok {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	form, ok := a.Cfg.Forms[formID]
	if !ok {
		http.Error(w, "Unknown form", http.StatusBadRequest)
		return
	}

	entDef, ok := a.meta.GetEntityDef(form.EntityType)
	if !ok {
		http.Error(w, "Unknown entity type", http.StatusBadRequest)
		return
	}

	oldEntity := entity.Clone()

	for _, f := range form.Fields {
		prop := entDef.Properties[f.Property]
		widget := resolveWidget(prop, a.meta)
		if widget == WidgetMultiSelect {
			values := r.Form[f.Property]
			if len(values) > 0 {
				entity.Properties[f.Property] = values
			} else {
				delete(entity.Properties, f.Property)
			}
		} else {
			val := r.FormValue(f.Property)
			if val != "" {
				entity.Properties[f.Property] = val
			} else {
				if widget == WidgetCheckbox {
					entity.Properties[f.Property] = "false"
				} else {
					delete(entity.Properties, f.Property)
				}
			}
		}
	}

	if form.Body != nil && *form.Body {
		entity.Content = r.FormValue("_body")
	}

	if _, err := a.ws.UpdateEntity(entity, oldEntity); err != nil {
		var ve *workspace.ValidationError
		if errors.As(err, &ve) {
			a.renderFormWithErrors(w, r, formID, entity, validationErrorsToFieldMap(ve.Errors))
			return
		}
		http.Error(w, fmt.Sprintf("Failed to write: %v", err), http.StatusInternalServerError)
		return
	}

	// Reconcile relations: delete existing, recreate from form values.
	for _, rel := range form.Relations {
		if rel.Display != "" {
			continue
		}
		// Resolve direction from metamodel if not specified in config (mirrors handleForm logic).
		direction := rel.Direction
		if direction == "" {
			if relDef, ok := a.meta.GetRelationDef(rel.Relation); ok {
				inFrom := containsString(relDef.From, form.EntityType)
				inTo := containsString(relDef.To, form.EntityType)
				if inFrom && !inTo {
					direction = DirectionOutgoing
				} else if inTo && !inFrom {
					direction = DirectionIncoming
				}
			}
		}
		if direction == DirectionIncoming {
			for _, edge := range a.g.IncomingEdges(entityID) {
				if edge.Type == rel.Relation {
					if delErr := a.ws.DeleteRelation(edge.From, edge.Type, edge.To); delErr != nil {
						log.Printf("Failed to delete relation: %v", delErr)
					}
				}
			}
		} else {
			for _, edge := range a.g.OutgoingEdges(entityID) {
				if edge.Type == rel.Relation {
					if delErr := a.ws.DeleteRelation(edge.From, edge.Type, edge.To); delErr != nil {
						log.Printf("Failed to delete relation: %v", delErr)
					}
				}
			}
		}
		values := r.Form[rel.Relation]
		for _, targetID := range values {
			if targetID == "" {
				continue
			}
			var fromID, toID string
			if direction == DirectionIncoming {
				fromID, toID = targetID, entityID
			} else {
				fromID, toID = entityID, targetID
			}
			// Collect relation properties from form.
			relProps := make(map[string]interface{})
			for _, rp := range rel.Properties {
				propKey := fmt.Sprintf("_relprop_%s_%s_%s", rel.Relation, targetID, rp.Property)
				if pv := r.FormValue(propKey); pv != "" {
					relProps[rp.Property] = pv
				}
			}
			var opts []workspace.CreateRelationOptions
			if len(relProps) > 0 {
				opts = append(opts, workspace.CreateRelationOptions{Properties: relProps})
			}
			if _, err := a.ws.CreateRelation(fromID, rel.Relation, toID, opts...); err != nil {
				log.Printf("Failed to write relation: %v", err)
				continue
			}
		}
	}

	log.Printf("Updated %s", entityID)

	redirect := "/entity/" + entity.Type + "/" + entityID
	if returnTo := r.FormValue("_return_to"); returnTo != "" && strings.HasPrefix(returnTo, "/") {
		redirect = returnTo
	}
	w.Header().Set("HX-Redirect", appendToastParam(redirect, "Saved "+entityID))
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleToggleCheckbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm() //nolint:errcheck // form parse errors are handled by empty values

	entityID := r.FormValue("entity_id")
	indexStr := r.FormValue("index")

	entity, ok := a.g.GetNode(entityID)
	if !ok {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	idx, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "Invalid checkbox index", http.StatusBadRequest)
		return
	}

	newContent, err := toggleCheckbox(entity.Content, idx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	oldEntity := entity.Clone()
	entity.Content = newContent
	if _, err := a.ws.UpdateEntity(entity, oldEntity); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, simpleMarkdownToHTML(entity.Content))
}

func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm() //nolint:errcheck // form parse errors are handled by empty values

	entityID := r.FormValue("_entity_id")
	entity, ok := a.g.GetNode(entityID)
	if !ok {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	if _, err := a.ws.DeleteEntity(entity.Type, entityID, true); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Deleted %s", entityID)

	redirect := "/"
	if returnTo := r.FormValue("_return_to"); returnTo != "" && strings.HasPrefix(returnTo, "/") && !strings.Contains(returnTo, entityID) {
		redirect = returnTo
	}
	w.Header().Set("HX-Redirect", appendToastParam(redirect, "Deleted "+entityID))
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleInlineCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errcheck // best-effort JSON response
		return
	}
	r.ParseMultipartForm(10 << 20) //nolint:errcheck // parse errors handled by empty values

	formID := r.FormValue("_form_id")
	form, ok := a.Cfg.Forms[formID]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown form"}) //nolint:errcheck // best-effort JSON response
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	opts := workspace.CreateOptions{
		Properties: a.buildProperties(form.Fields, entDef, form.EntityType, r),
	}
	if entDef.IsManualID() {
		opts.ID = r.FormValue("_entity_id")
		if opts.ID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "ID is required"}) //nolint:errcheck // best-effort JSON response
			return
		}
	}

	entity, _, err := a.ws.CreateEntity(form.EntityType, opts)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck // best-effort JSON response
		return
	}

	log.Printf("Inline-created %s %s", form.EntityType, entity.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck // best-effort JSON response
		"id":    entity.ID,
		"title": a.entityDisplayTitle(entity),
	})
}

func (a *App) handleInlineForm(w http.ResponseWriter, r *http.Request) {
	formID := strings.TrimPrefix(r.URL.Path, "/api/inline-form/")
	form, ok := a.Cfg.Forms[formID]
	if !ok {
		http.Error(w, "Unknown form", http.StatusNotFound)
		return
	}

	entDef, _ := a.meta.GetEntityDef(form.EntityType)

	var sb strings.Builder
	esc := html.EscapeString

	// Manual ID field
	if entDef.GetIDType() == metamodel.IDTypeManual {
		sb.WriteString(`<div class="form-group">`)
		sb.WriteString(`<label>ID<span class="required">*</span></label>`)
		sb.WriteString(`<input type="text" name="_entity_id" required placeholder="Unique ID...">`)
		sb.WriteString(`</div>`)
	}

	for _, f := range form.Fields {
		if f.Hidden {
			sb.WriteString(fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`, esc(f.Property), esc(f.Default)))
			continue
		}
		prop := entDef.Properties[f.Property]
		widget := resolveWidget(prop, a.meta)
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

		switch {
		case widget == WidgetCheckbox:
			sb.WriteString(fmt.Sprintf(`<div class="form-row-checkbox"><input type="checkbox" name="%s" value="true" id="ic-%s"><label for="ic-%s">%s</label></div>`, esc(f.Property), esc(f.Property), esc(f.Property), esc(label)))
		case widget == WidgetTextarea:
			sb.WriteString(fmt.Sprintf(`<label for="ic-%s">%s%s</label>`, esc(f.Property), esc(label), reqMark))
			sb.WriteString(fmt.Sprintf(`<textarea name="%s" id="ic-%s" placeholder="%s" style="min-height:60px;"></textarea>`, esc(f.Property), esc(f.Property), esc(f.Placeholder)))
		case widget == WidgetSelect || widget == WidgetMultiSelect:
			sb.WriteString(fmt.Sprintf(`<label for="ic-%s">%s%s</label>`, esc(f.Property), esc(label), reqMark))
			vals := resolvePropertyValues(prop, a.meta)
			defaultVal := coalesce(f.Default, prop.Default)
			sb.WriteString(fmt.Sprintf(`<select name="%s" id="ic-%s">`, esc(f.Property), esc(f.Property)))
			sb.WriteString(`<option value="">Select...</option>`)
			for _, v := range vals {
				sel := ""
				if v == defaultVal {
					sel = " selected"
				}
				sb.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, esc(v), sel, esc(v)))
			}
			sb.WriteString(`</select>`)
		default:
			inputType := widgetToInputType(widget)
			sb.WriteString(fmt.Sprintf(`<label for="ic-%s">%s%s</label>`, esc(f.Property), esc(label), reqMark))
			reqAttr := ""
			if required {
				reqAttr = " required"
			}
			sb.WriteString(fmt.Sprintf(`<input type="%s" name="%s" id="ic-%s" placeholder="%s"%s>`, inputType, esc(f.Property), esc(f.Property), esc(f.Placeholder), reqAttr))
		}

		sb.WriteString(`</div>`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(sb.String())) //nolint:errcheck // best-effort HTTP response
}

func (a *App) handleLinkCandidates(w http.ResponseWriter, r *http.Request) {
	relation := r.URL.Query().Get("relation")
	linkAs := r.URL.Query().Get("link_as")
	peerID := r.URL.Query().Get("peer")
	entityTypesStr := r.URL.Query().Get("entity_types")
	q := strings.ToLower(r.URL.Query().Get("q"))

	if relation == "" || peerID == "" || entityTypesStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing required parameters"}) //nolint:errcheck // best-effort JSON response
		return
	}

	entityTypes := strings.Split(entityTypesStr, ",")

	// Collect already-linked entity IDs to exclude them
	alreadyLinked := map[string]bool{}
	if linkAs == "to" {
		for _, edge := range a.g.OutgoingEdges(peerID) {
			if edge.Type == relation {
				alreadyLinked[edge.To] = true
			}
		}
	} else {
		for _, edge := range a.g.IncomingEdges(peerID) {
			if edge.Type == relation {
				alreadyLinked[edge.From] = true
			}
		}
	}

	type candidate struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	var candidates []candidate
	for _, et := range entityTypes {
		for _, e := range a.g.NodesByType(et) {
			if alreadyLinked[e.ID] || e.ID == peerID {
				continue
			}
			title := a.entityDisplayTitle(e)
			if q != "" && !strings.Contains(strings.ToLower(title), q) && !strings.Contains(strings.ToLower(e.ID), q) {
				continue
			}
			candidates = append(candidates, candidate{ID: e.ID, Title: title, Type: e.Type})
		}
	}
	if candidates == nil {
		candidates = []candidate{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(candidates) //nolint:errcheck // best-effort JSON response
}

func (a *App) handleLinkExisting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errcheck // best-effort JSON response
		return
	}
	r.ParseForm() //nolint:errcheck // parse errors handled by empty values

	relation := r.FormValue("relation")
	linkAs := r.FormValue("link_as")
	peerID := r.FormValue("peer")
	targetID := r.FormValue("target")

	if relation == "" || linkAs == "" || peerID == "" || targetID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing required parameters"}) //nolint:errcheck // best-effort JSON response
		return
	}

	if _, ok := a.meta.GetRelationDef(relation); !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown relation type"}) //nolint:errcheck // best-effort JSON response
		return
	}
	if _, ok := a.g.GetNode(peerID); !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown peer entity"}) //nolint:errcheck // best-effort JSON response
		return
	}
	if _, ok := a.g.GetNode(targetID); !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown target entity"}) //nolint:errcheck // best-effort JSON response
		return
	}

	var fromID, toID string
	if linkAs == "to" {
		fromID, toID = peerID, targetID
	} else {
		fromID, toID = targetID, peerID
	}

	if _, err := a.ws.CreateRelation(fromID, relation, toID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck // best-effort JSON response
		return
	}

	log.Printf("Linked %s --%s--> %s", fromID, relation, toID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ok": "true"}) //nolint:errcheck // best-effort JSON response
}

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	sq := searchparser.ParseQuery(query)

	type SearchResult struct {
		ID         string
		Title      string
		EntityType string
		PropType   string // metamodel type for badge
		Properties []struct{ Key, Value, PropType string }
	}

	var results []SearchResult
	var parseErrors string
	if errStr := sq.ErrorString(); errStr != "" {
		parseErrors = errStr
	}

	if !sq.IsEmpty() {
		entities := a.executeQuery(query)
		// Only sort by ID when there's no free-text query (preserve relevance ranking)
		if !sq.HasFreeText() && !sq.HasSort() {
			sort.Slice(entities, func(i, j int) bool { return entities[i].ID < entities[j].ID })
		}
		const maxResults = 100
		for _, e := range entities {
			if len(results) >= maxResults {
				break
			}

			sr := SearchResult{
				ID:         e.ID,
				Title:      a.entityDisplayTitle(e),
				EntityType: e.Type,
			}

			entDef, _ := a.meta.GetEntityDef(e.Type)
			if entDef != nil {
				propNames := make([]string, 0, len(entDef.Properties))
				for pn := range entDef.Properties {
					propNames = append(propNames, pn)
				}
				natsort.Strings(propNames)
				count := 0
				for _, propName := range propNames {
					if count >= 3 {
						break
					}
					propDef := entDef.Properties[propName]
					if v := e.Properties[propName]; v != nil {
						val := fmt.Sprintf("%v", v)
						if val != "" {
							sr.Properties = append(sr.Properties, struct{ Key, Value, PropType string }{
								Key:      titleCase(propName),
								Value:    val,
								PropType: propDef.Type,
							})
							count++
						}
					}
				}
			}

			results = append(results, sr)
		}
	}

	// Build search suggestions from metamodel for autocomplete
	type searchSuggestion struct {
		Value    string `json:"value"`
		Category string `json:"category"`
	}
	suggestions := make([]searchSuggestion, 0, len(a.meta.EntityTypes()))
	for _, et := range a.meta.EntityTypes() {
		suggestions = append(suggestions, searchSuggestion{Value: "type:" + et, Category: "type"})
	}
	seen := make(map[string]bool)
	for _, entityTypeName := range a.meta.EntityTypes() {
		entDef, ok := a.meta.GetEntityDef(entityTypeName)
		if !ok {
			continue
		}
		for propName, propDef := range entDef.Properties {
			var values []string
			if len(propDef.Values) > 0 {
				values = propDef.Values
			} else if ct, ok := a.meta.Types[propDef.Type]; ok {
				values = ct.Values
			}
			if len(values) == 0 {
				continue
			}
			for _, v := range values {
				var key string
				var cat string
				if propName == "status" {
					key = "status:" + v
					cat = "status"
				} else {
					key = "prop:" + propName + "=" + v
					cat = "property"
				}
				if !seen[key] {
					suggestions = append(suggestions, searchSuggestion{Value: key, Category: cat})
					seen[key] = true
				}
			}
		}
	}
	// Add sort suggestions: virtual properties + all entity properties
	sortSeen := make(map[string]bool)
	for _, vp := range []string{"id", "modified"} {
		key := "sort:" + vp
		suggestions = append(suggestions, searchSuggestion{Value: key, Category: "sort"})
		sortSeen[key] = true
	}
	for _, entityTypeName := range a.meta.EntityTypes() {
		entDef, ok := a.meta.GetEntityDef(entityTypeName)
		if !ok {
			continue
		}
		for propName := range entDef.Properties {
			key := "sort:" + propName
			if !sortSeen[key] {
				suggestions = append(suggestions, searchSuggestion{Value: key, Category: "sort"})
				sortSeen[key] = true
			}
		}
	}

	suggestionsJSON, _ := json.Marshal(suggestions)

	data := map[string]interface{}{
		"App":             a.Cfg.App,
		"ConflictCount":   a.conflictCount(),
		"Navigation":      a.navElements(""),
		"ActiveList":      "",
		"Query":           query,
		"Results":         results,
		"ResultCount":     len(results),
		"HasQuery":        query != "",
		"ParseErrors":     parseErrors,
		"ScopeParams":     "scope=search:" + url.QueryEscape(query),
		"IsHTMX":          r.Header.Get("HX-Request") == "true",
		"SuggestionsJSON": htmltemplate.JS(suggestionsJSON), //nolint:gosec // controlled data from metamodel
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		if r.Header.Get("HX-Target") == "search-results" {
			a.tmpl.ExecuteTemplate(w, "search-results", data) //nolint:errcheck // template errors logged by http
		} else {
			a.tmpl.ExecuteTemplate(w, "search-content", data) //nolint:errcheck // template errors logged by http
		}
	} else {
		a.tmpl.ExecuteTemplate(w, "search-page", data) //nolint:errcheck // template errors logged by http
	}
}

// appendToastParam appends a _toast query parameter to a redirect URL,
// preserving any existing query string and fragment.
func appendToastParam(redirectURL, message string) string {
	base, fragment, hasFragment := strings.Cut(redirectURL, "#")
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	base += sep + "_toast=" + url.QueryEscape(message)
	if hasFragment {
		base += "#" + fragment
	}
	return base
}

// coverage-ignore: dashboard handler - tested via integration/manual testing
func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if a.Cfg.Dashboard == nil {
		http.NotFound(w, r)
		return
	}
	dash := a.Cfg.Dashboard

	type BreakdownItem struct {
		Value      string
		Count      int
		PropType   string
		Percentage float64
	}

	type CellData struct {
		Value    string
		PropType string
		Link     string
	}

	type ResolvedCard struct {
		Title   string
		Display string
		Query   string
		// count display
		Count int
		// breakdown display
		BreakdownItems []BreakdownItem
		GroupByLabel   string
		// table display
		Columns []ListColumn
		Rows    [][]CellData
	}

	cards := make([]ResolvedCard, len(dash.Cards))
	for i, card := range dash.Cards {
		entities := a.executeQuery(card.Query)
		rc := ResolvedCard{
			Title:   card.Title,
			Display: card.Display,
			Query:   card.Query,
			Count:   len(entities),
		}

		switch card.Display {
		case "breakdown":
			// Group entities by property value
			groups := make(map[string]int)
			var orderedValues []string
			for _, e := range entities {
				val := ""
				if v := e.Properties[card.GroupBy]; v != nil {
					val = fmt.Sprintf("%v", v)
				}
				if val == "" {
					val = "(empty)"
				}
				if groups[val] == 0 {
					orderedValues = append(orderedValues, val)
				}
				groups[val]++
			}
			// Sort values in natural order for consistent display
			natsort.Strings(orderedValues)
			// Determine property type for badge styling
			propType := ""
			if len(entities) > 0 {
				propType = resolvePropertyType(card.GroupBy, entities[0].Type, a.meta)
			}
			total := len(entities)
			for _, val := range orderedValues {
				pct := 0.0
				if total > 0 {
					pct = float64(groups[val]) / float64(total) * 100
				}
				rc.BreakdownItems = append(rc.BreakdownItems, BreakdownItem{
					Value:      val,
					Count:      groups[val],
					PropType:   propType,
					Percentage: pct,
				})
			}
			rc.GroupByLabel = titleCase(card.GroupBy)

		case "table":
			a.sortEntitiesMulti(entities, card.Sort)
			if card.Limit > 0 && len(entities) > card.Limit {
				entities = entities[:card.Limit]
			}
			rc.Columns = card.Columns
			for _, e := range entities {
				row := make([]CellData, len(card.Columns))
				for j, col := range card.Columns {
					var val string
					var propType string
					if col.Relation != "" {
						val = strings.Join(a.resolveRelationColumnValues(e.ID, col.Relation), ", ")
					} else {
						if v := e.Properties[col.Property]; v != nil {
							val = fmt.Sprintf("%v", v)
						}
						propType = resolvePropertyType(col.Property, e.Type, a.meta)
					}
					cd := CellData{
						Value:    val,
						PropType: propType,
						Link:     a.resolveLinkTarget(col.Link, e.Type, e.ID),
					}
					row[j] = cd
				}
				rc.Rows = append(rc.Rows, row)
			}
		}

		cards[i] = rc
	}

	// Compute analysis summary for the validation card
	analysisErrors, analysisWarnings := a.analysisIssueCounts()

	data := map[string]interface{}{
		"App":              a.Cfg.App,
		"ConflictCount":    a.conflictCount(),
		"Navigation":       a.navElements("_dashboard"),
		"ActiveList":       "_dashboard",
		"Dashboard":        dash,
		"Cards":            cards,
		"Commands":         a.resolveCommands("dashboard", "", ""),
		"AnalysisErrors":   analysisErrors,
		"AnalysisWarnings": analysisWarnings,
		"IsHTMX":           r.Header.Get("HX-Request") == "true",
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "dashboard-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "dashboard-page", data) //nolint:errcheck // template errors logged by http
	}
}

// coverage-ignore: analyze handler - tested via integration/manual testing
func (a *App) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	result := a.runAnalysis()

	data := map[string]interface{}{
		"App":           a.Cfg.App,
		"ConflictCount": a.conflictCount(),
		"Navigation":    a.navElements("_analyze"),
		"ActiveList":    "_analyze",
		"Analysis":      result,
		"IsHTMX":        r.Header.Get("HX-Request") == "true",
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "analyze-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "analyze-page", data) //nolint:errcheck // template errors logged by http
	}
}

func (a *App) handleToggleGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm() //nolint:errcheck // form parse errors handled by empty values
	group := r.FormValue("group")
	if group == "" {
		http.Error(w, "Missing group parameter", http.StatusBadRequest)
		return
	}

	state := a.loadUIState()
	// Toggle: if currently collapsed, expand; if expanded (or absent), collapse
	state.CollapsedGroups[group] = !state.CollapsedGroups[group]
	// Clean up false entries to keep the file tidy
	if !state.CollapsedGroups[group] {
		delete(state.CollapsedGroups, group)
	}
	if err := a.saveUIState(state); err != nil {
		log.Printf("Failed to save UI state: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSettings renders the settings page for user defaults.
// coverage-ignore: UI handler
func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	ud := a.userDefaults
	if ud == nil {
		ud = &UserDefaults{}
	}

	// Collect all property names across entity types with their type info.
	type PropertyInfo struct {
		Name   string
		Type   string
		Values []string
	}
	propMap := make(map[string]PropertyInfo)
	for _, entTypeName := range a.meta.EntityTypes() {
		entDef, ok := a.meta.GetEntityDef(entTypeName)
		if !ok {
			continue
		}
		for propName, propDef := range entDef.Properties {
			if _, exists := propMap[propName]; !exists {
				propMap[propName] = PropertyInfo{
					Name:   propName,
					Type:   propDef.Type,
					Values: resolvePropertyValues(propDef, a.meta),
				}
			} else {
				// Merge values (union) for properties that appear on multiple types.
				existing := propMap[propName]
				seen := make(map[string]bool)
				for _, v := range existing.Values {
					seen[v] = true
				}
				for _, v := range resolvePropertyValues(propDef, a.meta) {
					if !seen[v] {
						existing.Values = append(existing.Values, v)
						seen[v] = true
					}
				}
				propMap[propName] = existing
			}
		}
	}
	propNames := make([]string, 0, len(propMap))
	for name := range propMap {
		propNames = append(propNames, name)
	}
	natsort.Strings(propNames)
	allProperties := make([]PropertyInfo, 0, len(propNames))
	for _, name := range propNames {
		allProperties = append(allProperties, propMap[name])
	}

	// Collect all relation types with their target entity types.
	type RelationInfo struct {
		Name       string
		Label      string
		TargetType string
		Targets    []struct{ ID, Title string }
	}
	relNames := a.meta.RelationTypes()
	natsort.Strings(relNames)
	allRelations := make([]RelationInfo, 0, len(relNames))
	for _, relName := range relNames {
		relDef, ok := a.meta.GetRelationDef(relName)
		if !ok {
			continue
		}
		ri := RelationInfo{
			Name:  relName,
			Label: relDef.Label,
		}
		// Use the first "to" type as the target type for default selection.
		if len(relDef.To) > 0 {
			ri.TargetType = relDef.To[0]
			for _, targetType := range relDef.To {
				for _, e := range a.g.NodesByType(targetType) {
					ri.Targets = append(ri.Targets, struct{ ID, Title string }{e.ID, a.entityDisplayTitle(e)})
				}
			}
		}
		allRelations = append(allRelations, ri)
	}

	// Entity types for override type selection.
	entityTypes := a.meta.EntityTypes()
	natsort.Strings(entityTypes)

	activeList := "_settings"
	data := map[string]interface{}{
		"App":           a.Cfg.App,
		"ConflictCount": a.conflictCount(),
		"Navigation":    a.navElements(activeList),
		"ActiveList":    activeList,
		"UserDefaults":  ud,
		"AllProperties": allProperties,
		"AllRelations":  allRelations,
		"EntityTypes":   entityTypes,
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "settings-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "settings-page", data) //nolint:errcheck // template errors logged by http
	}
}

// handleSaveSettings persists user defaults from the settings form.
func (a *App) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm() //nolint:errcheck // form parse errors handled by empty values

	ud := UserDefaults{
		Defaults:         make(map[string]string),
		RelationDefaults: make(map[string]string),
	}

	// Parse global property defaults: default_prop[<name>] = value
	for key, vals := range r.Form {
		if strings.HasPrefix(key, "default_prop[") && strings.HasSuffix(key, "]") {
			propName := key[len("default_prop[") : len(key)-1]
			if len(vals) > 0 && vals[0] != "" {
				ud.Defaults[propName] = vals[0]
			}
		}
	}

	// Parse global relation defaults: default_rel[<name>] = value
	for key, vals := range r.Form {
		if strings.HasPrefix(key, "default_rel[") && strings.HasSuffix(key, "]") {
			relName := key[len("default_rel[") : len(key)-1]
			if len(vals) > 0 && vals[0] != "" {
				ud.RelationDefaults[relName] = vals[0]
			}
		}
	}

	// Parse override groups: override[<idx>][types], override[<idx>][prop][<name>], override[<idx>][rel][<name>]
	overrideTypes := make(map[string][]string)          // idx -> types
	overrideProps := make(map[string]map[string]string) // idx -> prop -> val
	overrideRels := make(map[string]map[string]string)  // idx -> rel -> val
	for key, vals := range r.Form {
		if !strings.HasPrefix(key, "override[") {
			continue
		}
		rest := key[len("override["):]
		idx, rest, ok := strings.Cut(rest, "]")
		if !ok {
			continue
		}
		switch {
		case rest == "[types]":
			// Multiple values (multi-select)
			overrideTypes[idx] = vals
		case strings.HasPrefix(rest, "[prop][") && strings.HasSuffix(rest, "]"):
			propName := rest[len("[prop][") : len(rest)-1]
			if overrideProps[idx] == nil {
				overrideProps[idx] = make(map[string]string)
			}
			if len(vals) > 0 && vals[0] != "" {
				overrideProps[idx][propName] = vals[0]
			}
		case strings.HasPrefix(rest, "[rel][") && strings.HasSuffix(rest, "]"):
			relName := rest[len("[rel][") : len(rest)-1]
			if overrideRels[idx] == nil {
				overrideRels[idx] = make(map[string]string)
			}
			if len(vals) > 0 && vals[0] != "" {
				overrideRels[idx][relName] = vals[0]
			}
		}
	}

	// Collect override indices and sort them for deterministic order.
	idxSet := make(map[string]bool)
	for idx := range overrideTypes {
		idxSet[idx] = true
	}
	for idx := range overrideProps {
		idxSet[idx] = true
	}
	for idx := range overrideRels {
		idxSet[idx] = true
	}
	indices := make([]string, 0, len(idxSet))
	for idx := range idxSet {
		indices = append(indices, idx)
	}
	natsort.Strings(indices)

	for _, idx := range indices {
		types := overrideTypes[idx]
		if len(types) == 0 {
			continue
		}
		o := DefaultOverride{
			Types:            types,
			Defaults:         overrideProps[idx],
			RelationDefaults: overrideRels[idx],
		}
		ud.Overrides = append(ud.Overrides, o)
	}

	if err := a.saveUserDefaults(&ud); err != nil {
		log.Printf("Failed to save user defaults: %v", err)
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}
	a.userDefaults = &ud

	w.Header().Set("HX-Redirect", appendToastParam("/settings", "Settings saved"))
	w.WriteHeader(http.StatusOK)
}
