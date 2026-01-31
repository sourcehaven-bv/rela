package dataentry

import (
	"encoding/json"
	"fmt"
	"html/template"
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

// sortEntities sorts entities by a property according to the given config.
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
	inList := false
	paragraph := make([]string, 0, len(lines))

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

		// Regular text
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
		"renderMarkdown": simpleMarkdownToHTML,
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
	}
}
