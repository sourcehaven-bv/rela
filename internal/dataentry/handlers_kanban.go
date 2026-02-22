package dataentry

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// ResolvedColumn holds a column's value and display label.
type ResolvedColumn struct {
	Value string
	Label string
}

// ResolvedSwimlane holds a swimlane's value and display label.
type ResolvedSwimlane struct {
	Value string
	Label string
}

// KanbanCardData holds the display data for a single card on the board.
type KanbanCardData struct {
	ID          string
	EntityType  string
	Title       string
	Fields      []KanbanCardField
	AccentType  string // Property type for accent color (column property)
	AccentValue string // Value for accent color lookup
}

// KanbanCardField holds a single field displayed on a card.
type KanbanCardField struct {
	Label    string
	Value    string   // Single value (for backwards compatibility)
	Values   []string // Multiple values for multi-select fields
	PropType string
}

// handleKanban renders a kanban board view.
func (a *App) handleKanban(w http.ResponseWriter, r *http.Request) {
	kanbanID := strings.TrimPrefix(r.URL.Path, "/kanban/")
	kanban, ok := a.Cfg.Kanbans[kanbanID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	entDef, _ := a.meta.GetEntityDef(kanban.EntityType)

	// Get entities and apply filters
	entities := a.g.NodesByType(kanban.EntityType)
	entities = applyFilters(entities, kanban.Filters)

	// Apply query param filters
	for _, fc := range kanban.FilterControls {
		val := r.URL.Query().Get("filter_" + fc.Property)
		if val != "" {
			entities = applyFilters(entities, []FilterConfig{{
				Property: fc.Property,
				Operator: "=",
				Value:    val,
			}})
		}
	}

	// Resolve columns (from config or metamodel)
	columns := a.resolveKanbanColumns(kanban, entDef)

	// Resolve swimlanes if configured
	var swimlanes []ResolvedSwimlane
	hasSwimlanes := kanban.SwimlaneProperty != ""
	if hasSwimlanes {
		swimlanes = a.resolveKanbanSwimlanes(kanban, entDef)
	}

	// Group entities into cells
	// cells[columnValue][swimlaneValue] = []KanbanCardData
	// For no swimlanes, use "" as the swimlane key
	cells := make(map[string]map[string][]KanbanCardData)
	for _, col := range columns {
		cells[col.Value] = make(map[string][]KanbanCardData)
		if hasSwimlanes {
			for _, lane := range swimlanes {
				cells[col.Value][lane.Value] = nil
			}
		} else {
			cells[col.Value][""] = nil
		}
	}

	// Populate cells with entity cards
	for _, e := range entities {
		colVal := e.GetAttributeString(kanban.ColumnProperty)
		if _, ok := cells[colVal]; !ok {
			// Entity's column value not in configured columns, skip
			continue
		}

		laneVal := ""
		if hasSwimlanes {
			laneVal = e.GetAttributeString(kanban.SwimlaneProperty)
			if _, ok := cells[colVal][laneVal]; !ok {
				// Entity's swimlane value not in configured swimlanes, skip
				continue
			}
		}

		card := a.buildKanbanCard(e, kanban)
		cells[colVal][laneVal] = append(cells[colVal][laneVal], card)
	}

	// Resolve filter control values
	type ResolvedFC struct {
		Property string
		Label    string
		Widget   string
		Values   []string
		Current  string
	}
	filterControls := make([]ResolvedFC, 0, len(kanban.FilterControls))
	for _, fc := range kanban.FilterControls {
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

	// Build filter params for URLs
	var filterParams string
	for _, fc := range kanban.FilterControls {
		if val := r.URL.Query().Get("filter_" + fc.Property); val != "" {
			filterParams += "&filter_" + fc.Property + "=" + val
		}
	}

	data := map[string]interface{}{
		"App":            a.Cfg.App,
		"Navigation":     a.navElements("_kanban_" + kanbanID),
		"ActiveKanban":   kanbanID,
		"Kanban":         kanban,
		"KanbanID":       kanbanID,
		"Columns":        columns,
		"Swimlanes":      swimlanes,
		"HasSwimlanes":   hasSwimlanes,
		"Cells":          cells,
		"FilterControls": filterControls,
		"FilterParams":   filterParams,
		"TotalCount":     len(entities),
		"EditForm":       kanban.EditForm,
		"CreateForm":     kanban.CreateForm,
		"IsHTMX":         r.Header.Get("HX-Request") == "true",
	}

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "kanban-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "kanban-page", data) //nolint:errcheck // template errors logged by http
	}
}

// resolveKanbanColumns returns the columns for a kanban board.
// If columns are explicitly configured, uses those. Otherwise infers from metamodel.
func (a *App) resolveKanbanColumns(kanban Kanban, entDef *metamodel.EntityDef) []ResolvedColumn {
	if len(kanban.Columns) > 0 {
		cols := make([]ResolvedColumn, len(kanban.Columns))
		for i, c := range kanban.Columns {
			cols[i] = ResolvedColumn{
				Value: c.Value,
				Label: coalesce(c.Label, titleCase(c.Value)),
			}
		}
		return cols
	}

	// Infer from metamodel
	propDef := entDef.Properties[kanban.ColumnProperty]
	values := getValidEnumValues(propDef, a.meta)
	cols := make([]ResolvedColumn, len(values))
	for i, v := range values {
		cols[i] = ResolvedColumn{
			Value: v,
			Label: titleCase(v),
		}
	}
	return cols
}

// resolveKanbanSwimlanes returns the swimlanes for a kanban board.
// If swimlanes are explicitly configured, uses those. Otherwise infers from metamodel.
func (a *App) resolveKanbanSwimlanes(kanban Kanban, entDef *metamodel.EntityDef) []ResolvedSwimlane {
	if len(kanban.Swimlanes) > 0 {
		lanes := make([]ResolvedSwimlane, len(kanban.Swimlanes))
		for i, l := range kanban.Swimlanes {
			lanes[i] = ResolvedSwimlane{
				Value: l.Value,
				Label: coalesce(l.Label, titleCase(l.Value)),
			}
		}
		return lanes
	}

	// Infer from metamodel
	propDef := entDef.Properties[kanban.SwimlaneProperty]
	values := getValidEnumValues(propDef, a.meta)
	lanes := make([]ResolvedSwimlane, len(values))
	for i, v := range values {
		lanes[i] = ResolvedSwimlane{
			Value: v,
			Label: titleCase(v),
		}
	}
	return lanes
}

// buildKanbanCard creates the display data for an entity card.
func (a *App) buildKanbanCard(e *model.Entity, kanban Kanban) KanbanCardData {
	card := KanbanCardData{
		ID:         e.ID,
		EntityType: e.Type,
	}

	// Resolve title
	if kanban.Card.Title != "" {
		card.Title = e.GetAttributeString(kanban.Card.Title)
	}
	if card.Title == "" {
		card.Title = a.entityDisplayTitle(e)
	}

	// Set accent based on column property (primary grouping dimension)
	card.AccentType = resolvePropertyType(kanban.ColumnProperty, e.Type, a.meta)
	card.AccentValue = e.GetAttributeString(kanban.ColumnProperty)

	// Resolve fields
	for _, f := range kanban.Card.Fields {
		rawVal := e.GetAttribute(f.Property)
		if rawVal == nil {
			continue
		}
		propType := resolvePropertyType(f.Property, e.Type, a.meta)
		field := KanbanCardField{
			Label:    coalesce(f.Label, titleCase(f.Property)),
			PropType: propType,
		}

		// Handle array values (multi-select)
		if arr, ok := rawVal.([]interface{}); ok {
			if len(arr) == 0 {
				continue
			}
			for _, v := range arr {
				if s, ok := v.(string); ok {
					field.Values = append(field.Values, s)
				}
			}
			field.Value = e.GetAttributeString(f.Property) // Keep for backwards compat
		} else if strArr, ok := rawVal.([]string); ok {
			if len(strArr) == 0 {
				continue
			}
			field.Values = strArr
			field.Value = e.GetAttributeString(f.Property)
		} else {
			// Single value
			val := e.GetAttributeString(f.Property)
			if val == "" {
				continue
			}
			field.Value = val
		}

		card.Fields = append(card.Fields, field)
	}

	return card
}

// handleKanbanMove handles drag-and-drop card moves.
func (a *App) handleKanbanMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm() //nolint:errcheck // form parse errors handled by empty values

	kanbanID := r.FormValue("kanban_id")
	entityID := r.FormValue("entity_id")
	column := r.FormValue("column")
	swimlane := r.FormValue("swimlane")

	kanban, ok := a.Cfg.Kanbans[kanbanID]
	if !ok {
		http.Error(w, "Unknown kanban", http.StatusBadRequest)
		return
	}

	entity, ok := a.g.GetNode(entityID)
	if !ok {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	// Update column property
	entity.Properties[kanban.ColumnProperty] = column

	// Update swimlane property if board has swimlanes
	if kanban.SwimlaneProperty != "" && swimlane != "" {
		entity.Properties[kanban.SwimlaneProperty] = swimlane
	}

	// Write entity to disk
	if err := a.repo.WriteEntity(entity, a.meta); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Moved %s to %s=%s", entityID, kanban.ColumnProperty, column)

	// Re-render the kanban board
	r.URL.Path = "/kanban/" + kanbanID
	a.handleKanban(w, r)
}
