package dataentry

import (
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// PropertyHelp holds documentation for a single property.
type PropertyHelp struct {
	Name        string
	Type        string
	Required    bool
	Description htmltemplate.HTML
}

// RelationHelp holds documentation for a single relation.
type RelationHelp struct {
	Name        string
	Label       string
	TargetType  string // target type for outgoing, source type for incoming
	Cardinality string
	Required    bool // true if min cardinality >= 1
	Description htmltemplate.HTML
}

// handleToggleCheckbox toggles a markdown checkbox in entity content.
func (a *App) handleToggleCheckbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm() //nolint:errcheck // form parse errors are handled by empty values

	entityID := r.FormValue("entity_id")
	indexStr := r.FormValue("index")

	idx, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "Invalid checkbox index", http.StatusBadRequest)
		return
	}

	// Serialize against other mutations and against workspace reloads.
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	live, ok := a.getEntityAsModel(entityID)
	if !ok {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	newContent, err := toggleCheckbox(live.Content, idx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Clone to avoid mutating the live graph node while other readers
	// (which take no lock) may be iterating it.
	updated := live.Clone()
	updated.Content = newContent
	if _, err := a.ws.EntityManager().UpdateEntity(r.Context(), model.EntityToDomain(updated)); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, simpleMarkdownToHTML(updated.Content))
}

// handleEntityHelp returns HTML fragment with documentation for an entity type.
// GET /api/help/{entityType}
func (a *App) handleEntityHelp(w http.ResponseWriter, r *http.Request) {
	entityType := strings.TrimPrefix(r.URL.Path, "/api/help/")
	if entityType == "" {
		http.Error(w, "entity type required", http.StatusBadRequest)
		return
	}

	s := a.State()
	entDef, ok := s.Meta.GetEntityDef(entityType)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Gather property documentation
	props := make([]PropertyHelp, 0, len(entDef.Properties))
	for name, prop := range entDef.Properties {
		ph := PropertyHelp{
			Name:     name,
			Type:     prop.Type,
			Required: prop.Required,
		}
		if prop.Description != "" {
			ph.Description = simpleMarkdownToHTML(prop.Description)
		}
		props = append(props, ph)
	}
	// Sort properties alphabetically
	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })

	// Gather outgoing and incoming relations
	outgoingRels := a.gatherRelations(s.Meta, entityType, true)
	incomingRels := a.gatherRelations(s.Meta, entityType, false)

	// Render entity description
	var entityDesc htmltemplate.HTML
	if entDef.Description != "" {
		entityDesc = simpleMarkdownToHTML(entDef.Description)
	}

	// Generate inline HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	a.renderHelpContent(w, entityDesc, props, outgoingRels, incomingRels)
}

// renderHelpContent generates HTML for entity help content.
func (a *App) renderHelpContent(w http.ResponseWriter, entityDesc htmltemplate.HTML, props []PropertyHelp, outgoingRels, incomingRels []RelationHelp) {
	fmt.Fprint(w, `<div class="help-content">`)
	if entityDesc != "" {
		fmt.Fprintf(w, `<div class="entity-description">%s</div>`, entityDesc)
	}

	// Properties section
	if len(props) > 0 {
		fmt.Fprint(w, `<h4>Properties</h4><table class="help-table"><thead><tr><th>Name</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>`)
		for _, p := range props {
			required := ""
			if p.Required {
				required = "Yes"
			}
			fmt.Fprintf(w, `<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				htmltemplate.HTMLEscapeString(p.Name),
				htmltemplate.HTMLEscapeString(p.Type),
				required,
				p.Description)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}

	// Outgoing relations section
	if len(outgoingRels) > 0 {
		fmt.Fprint(w, `<h4>Outgoing Relations</h4><table class="help-table"><thead><tr><th>Name</th><th>Target</th><th>Cardinality</th><th>Description</th></tr></thead><tbody>`)
		for _, r := range outgoingRels {
			name := r.Name
			if r.Label != "" {
				name = r.Label + " (" + r.Name + ")"
			}
			fmt.Fprintf(w, `<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				htmltemplate.HTMLEscapeString(name),
				htmltemplate.HTMLEscapeString(r.TargetType),
				htmltemplate.HTMLEscapeString(r.Cardinality),
				r.Description)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}

	// Incoming relations section
	if len(incomingRels) > 0 {
		fmt.Fprint(w, `<h4>Incoming Relations</h4><table class="help-table"><thead><tr><th>Name</th><th>Source</th><th>Cardinality</th><th>Description</th></tr></thead><tbody>`)
		for _, r := range incomingRels {
			name := r.Name
			if r.Label != "" {
				name = r.Label + " (" + r.Name + ")"
			}
			fmt.Fprintf(w, `<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				htmltemplate.HTMLEscapeString(name),
				htmltemplate.HTMLEscapeString(r.TargetType),
				htmltemplate.HTMLEscapeString(r.Cardinality),
				r.Description)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}
	fmt.Fprint(w, `</div>`)
}

// gatherRelations collects relation documentation for an entity type.
// If outgoing is true, gathers relations where entityType is in "from";
// otherwise gathers relations where entityType is in "to".
func (a *App) gatherRelations(meta *metamodel.Metamodel, entityType string, outgoing bool) []RelationHelp {
	rels := make([]RelationHelp, 0, len(meta.Relations))
	for name, rel := range meta.Relations {
		var matchTypes, targetTypes []string
		var minCard, maxCard *int
		if outgoing {
			matchTypes, targetTypes = rel.From, rel.To
			minCard, maxCard = rel.MinOutgoing, rel.MaxOutgoing
		} else {
			matchTypes, targetTypes = rel.To, rel.From
			minCard, maxCard = rel.MinIncoming, rel.MaxIncoming
		}
		if !containsString(matchTypes, entityType) {
			continue
		}
		rh := RelationHelp{
			Name:        name,
			Label:       rel.Label,
			TargetType:  strings.Join(targetTypes, ", "),
			Cardinality: formatCardinality(minCard, maxCard),
			Required:    minCard != nil && *minCard >= 1,
		}
		if rel.Description != "" {
			rh.Description = simpleMarkdownToHTML(rel.Description)
		}
		rels = append(rels, rh)
	}
	sort.Slice(rels, func(i, j int) bool { return rels[i].Name < rels[j].Name })
	return rels
}

// formatCardinality formats min/max constraints as a human-readable string.
func formatCardinality(minC, maxC *int) string {
	if minC == nil && maxC == nil {
		return ""
	}
	minVal := 0
	if minC != nil {
		minVal = *minC
	}
	if maxC == nil {
		if minVal == 0 {
			return ""
		}
		return fmt.Sprintf("min %d", minVal)
	}
	maxVal := *maxC
	if minVal == maxVal {
		return fmt.Sprintf("exactly %d", minVal)
	}
	if minVal == 0 {
		return fmt.Sprintf("max %d", maxVal)
	}
	return fmt.Sprintf("%d-%d", minVal, maxVal)
}
