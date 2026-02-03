package dataentry

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/tui/searchparser"
)

// applyFilters filters entities by a set of filter conditions.
func applyFilters(entities []*model.Entity, filters []FilterConfig) []*model.Entity {
	if len(filters) == 0 {
		return entities
	}
	var result []*model.Entity
	for _, e := range entities {
		match := true
		for _, f := range filters {
			if strings.HasPrefix(f.Value, "$") {
				continue // skip variable substitution
			}
			val := e.GetAttributeString(f.Property)
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

// sortEntitiesMulti sorts entities by multiple sort specs using type-aware comparison.
func (a *App) sortEntitiesMulti(entities []*model.Entity, specs []model.SortSpec) {
	if len(specs) == 0 {
		return
	}
	entityDefs := make(map[string]*metamodel.EntityDef)
	for _, e := range entities {
		if _, ok := entityDefs[e.Type]; !ok {
			if def, ok := a.meta.GetEntityDef(e.Type); ok {
				entityDefs[e.Type] = def
			}
		}
	}
	filter.SortMulti(entities, specs, entityDefs, a.meta)
}

// resolvePropertyValues returns allowed values for a property from its definition or custom type.
func resolvePropertyValues(prop metamodel.PropertyDef, meta *metamodel.Metamodel) []string {
	if len(prop.Values) > 0 {
		return prop.Values
	}
	if ct, ok := meta.Types[prop.Type]; ok {
		return ct.Values
	}
	return nil
}

// resolveWidget returns the appropriate widget type for a property.
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

// widgetToInputType maps a widget name to an HTML input type.
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

// coalesce returns the first non-empty string.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// slugify converts a string to a URL-safe slug (lowercase, hyphens, no special chars).
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prev := byte('-')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteByte(c)
			prev = c
		} else if prev != '-' {
			b.WriteByte('-')
			prev = '-'
		}
	}
	return strings.Trim(b.String(), "-")
}

// titleCase converts snake_case/kebab-case to Title Case.
func titleCase(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// resolvePropertyType returns the metamodel type name for a property on an entity type.
func resolvePropertyType(prop, entityType string, meta *metamodel.Metamodel) string {
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

// simpleMarkdownToHTML converts basic markdown to HTML.
func simpleMarkdownToHTML(md string) template.HTML {
	if md == "" {
		return ""
	}
	lines := strings.Split(md, "\n")
	var out []string
	inCodeBlock := false
	listTag := "" // "", "ul", or "ol"
	paragraph := make([]string, 0, len(lines))

	flushParagraph := func() {
		if len(paragraph) > 0 {
			text := strings.Join(paragraph, " ")
			text = inlineFormat(text)
			out = append(out, "<p>"+text+"</p>")
			paragraph = nil
		}
	}

	closeList := func() {
		if listTag != "" {
			out = append(out, "</"+listTag+">")
			listTag = ""
		}
	}

	inMermaidBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code block toggle
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock || inMermaidBlock {
				if inMermaidBlock {
					out = append(out, "</pre>")
					inMermaidBlock = false
				} else {
					out = append(out, "</code></pre>")
					inCodeBlock = false
				}
			} else {
				flushParagraph()
				closeList()
				// Check for mermaid code block
				lang := strings.TrimSpace(trimmed[3:])
				if lang == "mermaid" {
					out = append(out, `<pre class="mermaid">`)
					inMermaidBlock = true
				} else {
					out = append(out, "<pre><code>")
					inCodeBlock = true
				}
			}
			continue
		}
		if inCodeBlock {
			out = append(out, template.HTMLEscapeString(line))
			continue
		}
		if inMermaidBlock {
			out = append(out, template.HTMLEscapeString(line))
			continue
		}

		// Empty line
		if trimmed == "" {
			flushParagraph()
			closeList()
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
			if listTag != "" && listTag != "ul" {
				closeList()
			}
			if listTag == "" {
				out = append(out, "<ul>")
				listTag = "ul"
			}
			out = append(out, "<li>"+inlineFormat(trimmed[2:])+"</li>")
			continue
		}

		// Ordered list
		if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			if idx := strings.Index(trimmed, ". "); idx > 0 && idx < 4 {
				flushParagraph()
				if listTag != "" && listTag != "ol" {
					closeList()
				}
				if listTag == "" {
					out = append(out, "<ol>")
					listTag = "ol"
				}
				out = append(out, "<li>"+inlineFormat(trimmed[idx+2:])+"</li>")
				continue
			}
		}

		// Regular text
		closeList()
		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	closeList()
	if inCodeBlock {
		out = append(out, "</code></pre>")
	}
	if inMermaidBlock {
		out = append(out, "</pre>")
	}

	return template.HTML(strings.Join(out, "\n")) //nolint:gosec // HTML is constructed from escaped user content
}

// inlineFormat handles bold, italic, inline code.
func inlineFormat(s string) string {
	s = template.HTMLEscapeString(s)
	// Inline code
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

// executeQuery parses a search query and returns all matching entities from the graph.
// It supports the same query syntax as the search page: type:, prop:, status:, and free text.
func (a *App) executeQuery(query string) []*model.Entity {
	sq := searchparser.ParseQuery(query)
	if sq.IsEmpty() {
		return nil
	}

	var candidates []*model.Entity
	if len(sq.EntityTypes) > 0 {
		for _, t := range sq.EntityTypes {
			candidates = append(candidates, a.g.NodesByType(t)...)
		}
	} else {
		candidates = a.g.AllNodes()
	}

	results := make([]*model.Entity, 0, len(candidates))
	for _, e := range candidates {
		if len(sq.PropertyFilters) > 0 {
			entDef, ok := a.meta.GetEntityDef(e.Type)
			if !ok {
				continue
			}
			matched, err := filter.MatchAll(e, sq.PropertyFilters, entDef, a.meta)
			if err != nil || !matched {
				continue
			}
		}

		if sq.HasFreeText() {
			searchText := strings.ToLower(e.ID + " " + a.entityDisplayTitle(e) + " " + e.Content)
			for _, v := range e.Properties {
				searchText += " " + strings.ToLower(fmt.Sprintf("%v", v))
			}
			match := true
			for _, word := range sq.FreeTextWords {
				if !strings.Contains(searchText, strings.ToLower(word)) {
					match = false
					break
				}
			}
			if match {
				for _, phrase := range sq.FreeTextPhrases {
					if !strings.Contains(searchText, strings.ToLower(phrase)) {
						match = false
						break
					}
				}
			}
			if !match {
				continue
			}
		}

		results = append(results, e)
	}

	// Apply sort from query syntax
	if sq.HasSort() {
		a.sortEntitiesMulti(results, sq.SortClauses)
	}

	return results
}

// templateFuncs returns the template.FuncMap used by all HTML templates.
func templateFuncs(styleMap map[string]map[string]string, styledTypes map[string]bool) template.FuncMap {
	return template.FuncMap{
		"join": strings.Join,
		"json": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"jsJSON": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b) //nolint:gosec // controlled data from metamodel
		},
		"contains": func(slice []string, val string) bool {
			for _, s := range slice {
				if s == val {
					return true
				}
			}
			return false
		},
		"badgeClass": func(propType, val string) string {
			if vals, ok := styleMap[propType]; ok {
				if cls, ok := vals[val]; ok {
					return cls
				}
			}
			return "badge-gray"
		},
		"isBadgeType": func(propType string) bool {
			return styledTypes[propType]
		},
		"add":            func(a, b int) int { return a + b },
		"boolTrue":       func(b *bool) bool { return b != nil && *b },
		"renderMarkdown": simpleMarkdownToHTML,
		"map": func(pairs ...interface{}) map[string]interface{} {
			m := make(map[string]interface{}, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				key, _ := pairs[i].(string)
				m[key] = pairs[i+1]
			}
			return m
		},
		"formatValue": func(val string) string {
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
		"sortedKeys": func(m map[string]interface{}) []string {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return keys
		},
	}
}

// resolveRelationColumnValue returns a comma-separated string of display titles
// for all targets of the given relation type from an entity.
func (a *App) resolveRelationColumnValue(entityID, relationType string) string {
	edges := a.g.OutgoingEdges(entityID)
	titles := make([]string, 0, len(edges))
	for _, edge := range edges {
		if edge.Type != relationType {
			continue
		}
		target, ok := a.g.GetNode(edge.To)
		if !ok {
			continue
		}
		titles = append(titles, a.entityDisplayTitle(target))
	}
	return strings.Join(titles, ", ")
}

// ScopeNav holds prev/next navigation context for browsing through a list of entities.
type ScopeNav struct {
	PrevURL  string // URL for previous entity (empty if at first)
	NextURL  string // URL for next entity (empty if at last)
	Progress string // e.g. "[3/12]"
	Label    string // e.g. "12 tickets" or "5 results for 'auth'"
}

// resolveScope parses the "scope" query parameter and reconstructs the ordered
// entity list to determine prev/next navigation links. Returns nil when no
// scope is present or the current entity isn't found in the scope.
func (a *App) resolveScope(currentEntityID string, r *http.Request) *ScopeNav {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		return nil
	}

	var ids []string
	var label string

	switch {
	case strings.HasPrefix(scope, "list:"):
		listID := strings.TrimPrefix(scope, "list:")
		list, ok := a.Cfg.Lists[listID]
		if !ok {
			return nil
		}
		entities := a.g.NodesByType(list.EntityType)
		entities = applyFilters(entities, list.Filters)

		// Apply dynamic filter params (same as handleList)
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

		// Apply sort (same as handleList)
		sortProp := r.URL.Query().Get("sort")
		sortDir := r.URL.Query().Get("sort_dir")
		if sortProp != "" {
			a.sortEntitiesMulti(entities, []model.SortSpec{{Property: sortProp, Direction: sortDir}})
		} else {
			a.sortEntitiesMulti(entities, list.Sort)
		}

		ids = make([]string, len(entities))
		for i, e := range entities {
			ids[i] = e.ID
		}
		label = fmt.Sprintf("%d %s", len(ids), list.Title)

	case strings.HasPrefix(scope, "search:"):
		query := strings.TrimPrefix(scope, "search:")
		entities := a.executeQuery(query)
		sort.Slice(entities, func(i, j int) bool { return entities[i].ID < entities[j].ID })
		ids = make([]string, len(entities))
		for i, e := range entities {
			ids[i] = e.ID
		}
		displayQuery := query
		if len(displayQuery) > 30 {
			displayQuery = displayQuery[:30] + "..."
		}
		label = fmt.Sprintf("%d results for \"%s\"", len(ids), displayQuery)

	default:
		return nil
	}

	// Find current entity in the scope
	idx := -1
	for i, id := range ids {
		if id == currentEntityID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}

	nav := &ScopeNav{
		Progress: fmt.Sprintf("[%d/%d]", idx+1, len(ids)),
		Label:    label,
	}

	// Build prev/next URLs by swapping the entity ID in the path
	buildURL := func(targetID string) string {
		// Replace the last path segment (entity ID) with the target ID
		path := r.URL.Path
		lastSlash := strings.LastIndex(path, "/")
		if lastSlash < 0 {
			return ""
		}
		newPath := path[:lastSlash+1] + targetID

		// Preserve all query params
		q := r.URL.Query()
		return newPath + "?" + q.Encode()
	}

	if idx > 0 {
		nav.PrevURL = buildURL(ids[idx-1])
	}
	if idx < len(ids)-1 {
		nav.NextURL = buildURL(ids[idx+1])
	}

	return nav
}

// isRelationLinked checks whether a form relation field (formRel) corresponds
// to a link relation (linkRel) coming from a view's "Add" button. It returns
// true when the link relation's inverse matches the form relation, when the
// form relation's inverse matches the link relation, or when they are equal.
func (a *App) isRelationLinked(formRel, linkRel string) bool {
	if formRel == linkRel {
		return true
	}
	// Check if linkRel has an inverse that equals formRel.
	if def, ok := a.meta.GetRelationDef(linkRel); ok && def.Inverse != nil {
		if def.Inverse.GetID() == formRel {
			return true
		}
	}
	// Check if formRel has an inverse that equals linkRel.
	if def, ok := a.meta.GetRelationDef(formRel); ok && def.Inverse != nil {
		if def.Inverse.GetID() == linkRel {
			return true
		}
	}
	return false
}
