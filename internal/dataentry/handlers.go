package dataentry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if len(a.Cfg.Navigation) > 0 {
		// Rewrite path so handleList picks up the first navigation list.
		// This avoids an HTTP redirect which Wails AssetServer does not follow.
		r.URL.Path = "/list/" + a.Cfg.Navigation[0].List
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
		val := r.URL.Query().Get("filter_" + fc.Property)
		if val != "" {
			entities = applyFilters(entities, []FilterConfig{{
				Property: fc.Property,
				Operator: "=",
				Value:    val,
			}})
		}
	}

	sortEntities(entities, list.Sort)

	// Resolve columns with values
	type CellData struct {
		Value      string
		Property   string
		PropType   string
		Link       bool
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
			val := fmt.Sprintf("%v", e.Properties[col.Property])
			if e.Properties[col.Property] == nil {
				val = ""
			}
			propType := resolvePropertyType(col.Property, list.EntityType, a.meta)
			cells = append(cells, CellData{
				Value:      val,
				Property:   col.Property,
				PropType:   propType,
				Link:       col.Link,
				EntityID:   e.ID,
				EntityType: e.Type,
			})
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

	data := map[string]interface{}{
		"App":              a.Cfg.App,
		"Navigation":       a.navItems(),
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
	relations := make([]ResolvedRelation, 0, len(form.Relations))
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

		targets := a.g.NodesByType(rel.TargetType)
		for _, t := range targets {
			rr.Options = append(rr.Options, struct{ ID, Title string }{t.ID, a.entityDisplayTitle(t)})
		}

		if entity != nil {
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

		relations = append(relations, rr)
	}

	mode := coalesce(form.Mode, "create")
	if entityID != "" {
		mode = "edit"
	}

	data := map[string]interface{}{
		"App":        a.Cfg.App,
		"Navigation": a.navItems(),
		"ActiveList": a.resolveActiveList(form.EntityType, r),
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

	var editFormID string
	for id, f := range a.Cfg.Forms {
		if f.EntityType == entity.Type && (f.Mode == "edit" || f.Mode == "") {
			editFormID = id
		}
	}

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
		rd := RelDisplay{e.Type, e.To, targetType, title, "outgoing", nil}
		for k, v := range e.Properties {
			rd.Properties = append(rd.Properties, RelPropDisplay{k, fmt.Sprintf("%v", v)})
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
		rd := RelDisplay{e.Type, e.From, sourceType, title, "incoming", nil}
		for k, v := range e.Properties {
			rd.Properties = append(rd.Properties, RelPropDisplay{k, fmt.Sprintf("%v", v)})
		}
		rels = append(rels, rd)
	}

	propTypes := make(map[string]string)
	if entDef != nil {
		for propName, propDef := range entDef.Properties {
			propTypes[propName] = propDef.Type
		}
	}

	data := map[string]interface{}{
		"App":        a.Cfg.App,
		"Navigation": a.navItems(),
		"ActiveList": a.resolveActiveList(entity.Type, r),
		"Entity":     entity,
		"EntityDef":  entDef,
		"EditFormID": editFormID,
		"Relations":  rels,
		"PropTypes":  propTypes,
		"IsHTMX":     r.Header.Get("HX-Request") == "true",
	}

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
		Value      string
		PropType   string
		Link       bool
		EntityID   string
		EntityType string
	}
	type SectionRowData struct {
		EntityID   string
		EntityType string
		Cells      []SectionColumnData
		Content    string
	}
	type GroupData struct {
		GroupName string
		Rows      []SectionRowData
		Entities  []SectionEntityData
	}
	type SectionData struct {
		Heading      string
		Display      string
		Fields       []SectionFieldData
		Entities     []SectionEntityData
		Columns      []ListColumn
		Rows         []SectionRowData
		Groups       []GroupData
		IsGrouped    bool
		EmptyMessage string
		IsEmpty      bool
		Link         bool
		Content      string
		HasContent   bool
	}

	sections := make([]SectionData, 0, len(view.Sections))

	for _, sec := range view.Sections {
		sd := SectionData{
			Heading:      sec.Heading,
			Display:      sec.Display,
			EmptyMessage: sec.EmptyMessage,
			Link:         sec.Link,
		}

		if sec.Source == "entry" {
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
						prop := strings.TrimPrefix(sec.GroupBy, "properties.")
						groupKey := "(none)"
						if v := e.Properties[prop]; v != nil {
							groupKey = fmt.Sprintf("%v", v)
						}
						if _, seen := groups[groupKey]; !seen {
							groupOrder = append(groupOrder, groupKey)
						}
						groups[groupKey] = append(groups[groupKey], e)
					}
					for _, gName := range groupOrder {
						gd := GroupData{GroupName: gName}
						for _, e := range groups[gName] {
							eDef, _ := a.meta.GetEntityDef(e.Type)
							row := SectionRowData{EntityID: e.ID, EntityType: e.Type}
							for _, col := range sec.Columns {
								val := ""
								if v := e.Properties[col.Property]; v != nil {
									val = fmt.Sprintf("%v", v)
								}
								propType := ""
								if eDef != nil {
									if pd, ok := eDef.Properties[col.Property]; ok {
										propType = pd.Type
									}
								}
								row.Cells = append(row.Cells, SectionColumnData{
									Value: val, PropType: propType, Link: col.Link, EntityID: e.ID, EntityType: e.Type,
								})
							}
							gd.Rows = append(gd.Rows, row)
						}
						sd.Groups = append(sd.Groups, gd)
					}
				} else {
					for _, e := range entities {
						eDef, _ := a.meta.GetEntityDef(e.Type)
						row := SectionRowData{EntityID: e.ID, EntityType: e.Type}
						for _, col := range sec.Columns {
							val := ""
							if v := e.Properties[col.Property]; v != nil {
								val = fmt.Sprintf("%v", v)
							}
							propType := ""
							if eDef != nil {
								if pd, ok := eDef.Properties[col.Property]; ok {
									propType = pd.Type
								}
							}
							row.Cells = append(row.Cells, SectionColumnData{
								Value: val, PropType: propType, Link: col.Link, EntityID: e.ID, EntityType: e.Type,
							})
						}
						sd.Rows = append(sd.Rows, row)
					}
				}

			case "content", "cards":
				for _, e := range entities {
					eDef, _ := a.meta.GetEntityDef(e.Type)
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
						if eDef != nil {
							if pd, ok := eDef.Properties[f.Property]; ok {
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
					eDef, _ := a.meta.GetEntityDef(e.Type)
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
						if eDef != nil {
							if pd, ok := eDef.Properties[f.Property]; ok {
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
		"App":        a.Cfg.App,
		"Navigation": a.navItems(),
		"ActiveList": a.resolveActiveList(result.Entry.Type, r),
		"View":       view,
		"ViewID":     viewID,
		"EntityID":   entityID,
		"Entry":      result.Entry,
		"EntryTitle": a.entityDisplayTitle(result.Entry),
		"Sections":   sections,
		"IsHTMX":     r.Header.Get("HX-Request") == "true",
	}

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "view-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "view-page", data) //nolint:errcheck // template errors logged by http
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

	var entityID string
	if entDef.GetIDType() == metamodel.IDTypeManual {
		entityID = r.FormValue("_entity_id")
		if entityID == "" {
			http.Error(w, "Manual ID required", http.StatusBadRequest)
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

	if form.Body != nil && *form.Body {
		entity.Content = r.FormValue("_body")
	}

	plural := entDef.GetDirPlural(form.EntityType)
	filePath := filepath.Join(a.projCtx.EntitiesDir, plural, entityID+".md")
	if err := markdown.WriteEntity(entity, filePath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write entity: %v", err), http.StatusInternalServerError)
		return
	}
	entity.FilePath = filePath
	entity.ModTime = time.Now()
	a.g.AddNode(entity)

	for _, rel := range form.Relations {
		values := r.Form[rel.Relation]
		for _, targetID := range values {
			if targetID == "" {
				continue
			}
			relation := model.NewRelation(entityID, rel.Relation, targetID)
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

	w.Header().Set("HX-Redirect", "/entity/"+form.EntityType+"/"+entityID)
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

	for _, f := range form.Fields {
		val := r.FormValue(f.Property)
		if val != "" {
			entity.Properties[f.Property] = val
		}
	}

	if form.Body != nil && *form.Body {
		entity.Content = r.FormValue("_body")
	}

	if err := markdown.WriteEntity(entity, entity.FilePath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write: %v", err), http.StatusInternalServerError)
		return
	}

	for _, rel := range form.Relations {
		for _, edge := range a.g.OutgoingEdges(entityID) {
			if edge.Type == rel.Relation {
				if delErr := markdown.DeleteRelation(edge.FilePath); delErr != nil {
					log.Printf("Failed to delete relation file: %v", delErr)
				}
				a.g.RemoveEdge(edge.From, edge.Type, edge.To)
			}
		}
		values := r.Form[rel.Relation]
		for _, targetID := range values {
			if targetID == "" {
				continue
			}
			relation := model.NewRelation(entityID, rel.Relation, targetID)
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

	w.Header().Set("HX-Redirect", "/entity/"+entity.Type+"/"+entityID)
	w.WriteHeader(http.StatusOK)
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

	for _, edge := range a.g.OutgoingEdges(entityID) {
		if delErr := markdown.DeleteRelation(edge.FilePath); delErr != nil {
			log.Printf("Failed to delete relation file: %v", delErr)
		}
		a.g.RemoveEdge(edge.From, edge.Type, edge.To)
	}
	for _, edge := range a.g.IncomingEdges(entityID) {
		if delErr := markdown.DeleteRelation(edge.FilePath); delErr != nil {
			log.Printf("Failed to delete relation file: %v", delErr)
		}
		a.g.RemoveEdge(edge.From, edge.Type, edge.To)
	}

	if err := markdown.DeleteEntity(entity.FilePath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete: %v", err), http.StatusInternalServerError)
		return
	}
	a.g.RemoveNode(entityID)

	log.Printf("Deleted %s", entityID)

	w.Header().Set("HX-Redirect", "/")
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

	var entityID string
	if entDef.GetIDType() == metamodel.IDTypeManual {
		entityID = r.FormValue("_entity_id")
		if entityID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "ID is required"}) //nolint:errcheck // best-effort JSON response
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

	for _, f := range form.Fields {
		val := r.FormValue(f.Property)
		if val == "" && f.Default != "" {
			val = f.Default
		}
		if val != "" {
			entity.Properties[f.Property] = val
		}
	}

	plural := entDef.GetDirPlural(form.EntityType)
	filePath := filepath.Join(a.projCtx.EntitiesDir, plural, entityID+".md")
	if err := markdown.WriteEntity(entity, filePath); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck // best-effort JSON response
		return
	}
	entity.FilePath = filePath
	entity.ModTime = time.Now()
	a.g.AddNode(entity)

	log.Printf("Inline-created %s %s", form.EntityType, entityID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck // best-effort JSON response
		"id":    entityID,
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

		switch {
		case widget == "checkbox":
			sb.WriteString(fmt.Sprintf(`<div class="form-row-checkbox"><input type="checkbox" name="%s" value="true" id="ic-%s"><label for="ic-%s">%s</label></div>`, f.Property, f.Property, f.Property, label))
		case widget == "textarea":
			sb.WriteString(fmt.Sprintf(`<label for="ic-%s">%s%s</label>`, f.Property, label, reqMark))
			sb.WriteString(fmt.Sprintf(`<textarea name="%s" id="ic-%s" placeholder="%s" style="min-height:60px;"></textarea>`, f.Property, f.Property, f.Placeholder))
		case widget == "select" || widget == "multi-select":
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
		default:
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
	w.Write([]byte(sb.String())) //nolint:errcheck // best-effort HTTP response
}
