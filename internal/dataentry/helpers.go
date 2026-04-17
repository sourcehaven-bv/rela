package dataentry

import (
	"bytes"
	"cmp"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/htmlutil"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
)

// nowFunc is the clock used for filter variable substitution. Tests can
// override this to pin a deterministic "now". Default returns UTC time
// to keep $today consistent regardless of server timezone.
var nowFunc = func() time.Time { return time.Now().UTC() }

// resolveFilterVariable substitutes special variable references in filter
// values. Currently supports:
//
//	$today          today's date in YYYY-MM-DD format (UTC)
//	$tomorrow       tomorrow's date (UTC)
//	$yesterday      yesterday's date (UTC)
//
// Times are evaluated in UTC for predictability across server timezones.
// Other values are returned unchanged.
func resolveFilterVariable(value string) string {
	switch value {
	case "$today":
		return nowFunc().Format("2006-01-02")
	case "$tomorrow":
		return nowFunc().AddDate(0, 0, 1).Format("2006-01-02")
	case "$yesterday":
		return nowFunc().AddDate(0, 0, -1).Format("2006-01-02")
	}
	return value
}

// resolveFilterVariablesInList applies resolveFilterVariable to each
// comma-separated token in value. Used by the in/ne operators.
func resolveFilterVariablesInList(value string) string {
	if !strings.Contains(value, ",") {
		return resolveFilterVariable(value)
	}
	parts := strings.Split(value, ",")
	for i, p := range parts {
		parts[i] = resolveFilterVariable(strings.TrimSpace(p))
	}
	return strings.Join(parts, ",")
}

// compareValues compares two values using the given comparison operator
// (lt, lte, gt, gte). It uses strict same-type comparison: if both sides
// parse as dates, dates are compared; if both parse as numbers, numbers
// are compared; otherwise strings are compared lexicographically.
//
// On a type mismatch (e.g. left is a date string, right is not), the
// comparison returns false and a non-nil error so callers can decide
// whether to log/reject. This prevents the silent lexicographic-fallback
// trap where "2026-04-07" < "tomorrow" returned true.
func compareValues(left, right, operator string) (match bool, err error) {
	// Both sides parse as dates → compare as dates
	lt, lDateErr := time.Parse("2006-01-02", left)
	rt, rDateErr := time.Parse("2006-01-02", right)
	switch {
	case lDateErr == nil && rDateErr == nil:
		return compareOrdered(lt.Unix(), rt.Unix(), operator), nil
	case lDateErr == nil || rDateErr == nil:
		// One side is a date, the other isn't — refuse to guess.
		return false, fmt.Errorf("cannot compare date %q with non-date %q",
			pickDate(left, right, lDateErr == nil), pickNonDate(left, right, lDateErr == nil))
	}

	// Both sides parse as numbers → compare as numbers
	lf, lNumErr := strconv.ParseFloat(left, 64)
	rf, rNumErr := strconv.ParseFloat(right, 64)
	switch {
	case lNumErr == nil && rNumErr == nil:
		return compareOrdered(lf, rf, operator), nil
	case lNumErr == nil || rNumErr == nil:
		// One side is numeric, the other isn't — refuse to guess.
		return false, fmt.Errorf("cannot compare number %q with non-number %q",
			pickNumber(left, right, lNumErr == nil), pickNonNumber(left, right, lNumErr == nil))
	}

	// Neither parses as date or number → string comparison
	return compareOrdered(left, right, operator), nil
}

// pickDate returns the side that successfully parsed as a date.
func pickDate(left, right string, leftIsDate bool) string {
	if leftIsDate {
		return left
	}
	return right
}

// pickNonDate returns the side that did NOT parse as a date.
func pickNonDate(left, right string, leftIsDate bool) string {
	if leftIsDate {
		return right
	}
	return left
}

// pickNumber / pickNonNumber are the numeric equivalents.
func pickNumber(left, right string, leftIsNum bool) string {
	if leftIsNum {
		return left
	}
	return right
}

func pickNonNumber(left, right string, leftIsNum bool) string {
	if leftIsNum {
		return right
	}
	return left
}

// compareOrdered applies an ordering operator to two ordered values.
// Returns false for unknown operators (caller is expected to validate).
func compareOrdered[T cmp.Ordered](left, right T, operator string) bool {
	c := cmp.Compare(left, right)
	switch operator {
	case "lt":
		return c < 0
	case "lte":
		return c <= 0
	case "gt":
		return c > 0
	case "gte":
		return c >= 0
	}
	return false
}

// ResolvedField represents a form field with all values resolved for rendering.
// Used by form templates to render property inputs consistently.
type ResolvedField struct {
	Name           string              // HTML input name attribute (defaults to Property if empty)
	Property       string              // Property name (used for IDs)
	Label          string              // Display label
	Placeholder    string              // Input placeholder
	Help           string              // Help text shown below field
	Required       bool                // Field is required
	Default        string              // Default value
	Value          string              // Current value
	SelectedValues []string            // For multi-select widgets
	Hidden         bool                // Field is hidden (rendered as hidden input)
	Widget         string              // Widget type: text, textarea, select, multi-select, checkbox
	InputType      string              // HTML input type: text, date, number, etc.
	Values         []string            // Allowed values for select/multi-select
	Transitions    map[string][]string // Status transitions (for workflow fields)
	Error          string              // Validation error message
}

// propertyContains checks if a property value contains the given string.
// Handles string, []string, and []interface{} property types.
func propertyContains(prop interface{}, value string) bool {
	if prop == nil {
		return value == ""
	}
	switch v := prop.(type) {
	case string:
		return v == value
	case []string:
		for _, s := range v {
			if s == value {
				return true
			}
		}
		return false
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s == value {
				return true
			}
		}
		return false
	default:
		return fmt.Sprintf("%v", prop) == value
	}
}

// propertyIsEmpty checks if a property value is empty/nil.
func propertyIsEmpty(prop interface{}) bool {
	if prop == nil {
		return true
	}
	switch v := prop.(type) {
	case string:
		return v == ""
	case []string:
		return len(v) == 0
	case []interface{}:
		return len(v) == 0
	default:
		return false
	}
}

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
			prop := e.Properties[f.Property]
			switch f.Operator {
			case "=":
				if f.Value == "" {
					if !propertyIsEmpty(prop) {
						match = false
					}
				} else if !propertyContains(prop, f.Value) {
					match = false
				}
			case "!=":
				if f.Value == "" {
					if propertyIsEmpty(prop) {
						match = false
					}
				} else if propertyContains(prop, f.Value) {
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
func (a *App) sortEntitiesMulti(entities []*model.Entity, specs []filter.SortSpec) {
	if len(specs) == 0 {
		return
	}
	s := a.State()
	entityDefs := make(map[string]*metamodel.EntityDef)
	for _, e := range entities {
		if _, ok := entityDefs[e.Type]; !ok {
			if def, ok := s.Meta.GetEntityDef(e.Type); ok {
				entityDefs[e.Type] = def
			}
		}
	}
	filter.SortMulti(entities, entityRecord, specs, entityDefs, s.Meta)
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
func resolveWidget(prop metamodel.PropertyDef, meta *metamodel.Metamodel) string {
	// Check if property is a list (multi-select) - only applies to enum types
	_, isCustomType := meta.Types[prop.Type]
	isEnum := prop.Type == metamodel.PropertyTypeEnum || isCustomType
	if prop.List && isEnum {
		return WidgetMultiSelect
	}

	return meta.ResolveWidgetFromType(prop.Type)
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

// containsString returns true if slice contains the given string.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
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

// mdConverter is the goldmark instance with GFM extensions (tables, task lists, etc.).
var mdConverter = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// simpleMarkdownToHTML converts markdown to HTML using goldmark with GFM extensions.
func simpleMarkdownToHTML(md string) template.HTML {
	if md == "" {
		return ""
	}

	var buf bytes.Buffer
	if err := mdConverter.Convert([]byte(md), &buf); err != nil {
		//nolint:gosec // fallback to escaped input on conversion error
		return template.HTML(template.HTMLEscapeString(md))
	}

	result := buf.String()

	// Post-process: add md-table class to tables
	result = strings.ReplaceAll(result, "<table>", `<table class="md-table">`)

	// Post-process: convert mermaid code blocks
	result = htmlutil.ConvertMermaidBlocks(result)

	// Post-process: add checkbox indices for interactive toggling
	result = addCheckboxIndices(result)

	//nolint:gosec // HTML is generated by goldmark from user markdown
	return template.HTML(result)
}

var checkboxRe = regexp.MustCompile(`<input[^>]*type="checkbox"[^>]*>`)

func addCheckboxIndices(s string) string {
	idx := 0
	return checkboxRe.ReplaceAllStringFunc(s, func(match string) string {
		// Add data-cb-idx attribute
		result := strings.Replace(match, "<input", fmt.Sprintf(`<input data-cb-idx="%d"`, idx), 1)
		idx++
		return result
	})
}

// checkboxPattern matches markdown task list items: - [ ], - [x], - [X].
var checkboxPattern = regexp.MustCompile(`^(- \[)([ xX])(\] )`)

// toggleCheckbox flips the checkbox at the given 0-based index in a markdown string.
// Returns the modified content and an error if the index is out of range.
func toggleCheckbox(content string, index int) (string, error) {
	lines := strings.Split(content, "\n")
	cbIdx := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if checkboxPattern.MatchString(trimmed) {
			if cbIdx == index {
				// Find the bracket position in the original (untrimmed) line
				pos := strings.Index(line, "- [")
				if pos < 0 {
					return "", fmt.Errorf("checkbox %d: bracket not found", index)
				}
				charPos := pos + 3 // position of the check character
				if line[charPos] == ' ' {
					line = line[:charPos] + "x" + line[charPos+1:]
				} else {
					line = line[:charPos] + " " + line[charPos+1:]
				}
				lines[i] = line
				return strings.Join(lines, "\n"), nil
			}
			cbIdx++
		}
	}
	return "", fmt.Errorf("checkbox index %d out of range (found %d)", index, cbIdx)
}

// CheckboxStats holds completion counts for task list items.
type CheckboxStats struct {
	Checked int
	Total   int
}

// checkboxStats counts checked and total task list items in markdown content.
func checkboxStats(content string) CheckboxStats {
	var stats CheckboxStats
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if checkboxPattern.MatchString(trimmed) {
			stats.Total++
			if trimmed[3] != ' ' {
				stats.Checked++
			}
		}
	}
	return stats
}

// executeQuery parses a search query and returns all matching entities from the graph.
// It supports the same query syntax as the search page: type:, prop:, status:, and free text.
// Free-text words use OR logic with fuzzy matching via Bleve; results are ranked by score.
func (a *App) executeQuery(query string) []*model.Entity {
	sq := searchparser.ParseQuery(query)
	if sq.IsEmpty() {
		return nil
	}

	const maxSearchResults = 1000

	type scored struct {
		entity *model.Entity
		score  float64
	}
	var scoredResults []scored

	// If there's free text, search via Bleve first
	if sq.HasFreeText() {
		entities, scores, err := a.ws.Search(sq.FreeTextWords, sq.FreeTextPhrases, maxSearchResults)
		if err != nil {
			return nil
		}

		for i, e := range entities {
			// Filter by entity type if specified
			if len(sq.EntityTypes) > 0 {
				typeMatch := false
				for _, t := range sq.EntityTypes {
					if e.Type == t {
						typeMatch = true
						break
					}
				}
				if !typeMatch {
					continue
				}
			}

			// Apply property filters
			if !a.matchesPropertyFilters(e, sq.PropertyFilters) {
				continue
			}

			scoredResults = append(scoredResults, scored{entity: e, score: scores[i]})
		}
	} else {
		// No free text - get candidates from graph and filter
		g := a.State().Graph
		var candidates []*model.Entity
		if len(sq.EntityTypes) > 0 {
			for _, t := range sq.EntityTypes {
				candidates = append(candidates, g.NodesByType(t)...)
			}
		} else {
			candidates = g.AllNodes()
		}

		for _, e := range candidates {
			if !a.matchesPropertyFilters(e, sq.PropertyFilters) {
				continue
			}
			scoredResults = append(scoredResults, scored{entity: e, score: 1.0})
		}
	}

	results := make([]*model.Entity, len(scoredResults))
	for i, sr := range scoredResults {
		results[i] = sr.entity
	}

	// Apply sort from query syntax (Bleve results are already ranked by relevance)
	if sq.HasSort() {
		a.sortEntitiesMulti(results, sq.SortClauses)
	}

	return results
}

// resolveRelationColumnValues returns display titles for all targets of the given
// relation type from an entity. Direction controls whether to follow edges pointing
// to the entity (incoming) or from the entity (outgoing, the default).
func (a *App) resolveRelationColumnValues(entityID, relationType string, direction dataentryconfig.Direction) []string {
	s := a.State()
	var edges []*model.Relation
	if direction.IsIncoming() {
		edges = s.Graph.IncomingEdges(entityID)
	} else {
		edges = s.Graph.OutgoingEdges(entityID)
	}
	titles := make([]string, 0, len(edges))
	for _, edge := range edges {
		if edge.Type != relationType {
			continue
		}
		var targetID string
		if direction.IsIncoming() {
			targetID = edge.From
		} else {
			targetID = edge.To
		}
		target, ok := s.Graph.GetNode(targetID)
		if !ok {
			continue
		}
		titles = append(titles, s.Meta.DisplayTitle(target.ID, target.Type, target.Properties))
	}
	return titles
}

// filterByRelation filters entities to those that have an outgoing edge of the given
// relation type pointing to a target whose display title matches value.
func (a *App) filterByRelation(entities []*model.Entity, relationType, value string) []*model.Entity {
	s := a.State()
	var result []*model.Entity
	for _, e := range entities {
		for _, edge := range s.Graph.OutgoingEdges(e.ID) {
			if edge.Type != relationType {
				continue
			}
			target, ok := s.Graph.GetNode(edge.To)
			if !ok {
				continue
			}
			if s.Meta.DisplayTitle(target.ID, target.Type, target.Properties) == value {
				result = append(result, e)
				break
			}
		}
	}
	return result
}

// resolveRelationFilterValues returns sorted, unique display titles of all entities
// reachable via the given relation type from any of the provided entities.
func (a *App) resolveRelationFilterValues(entities []*model.Entity, relationType string) []string {
	s := a.State()
	seen := make(map[string]bool)
	var vals []string
	for _, e := range entities {
		for _, edge := range s.Graph.OutgoingEdges(e.ID) {
			if edge.Type != relationType {
				continue
			}
			target, ok := s.Graph.GetNode(edge.To)
			if !ok {
				continue
			}
			title := s.Meta.DisplayTitle(target.ID, target.Type, target.Properties)
			if !seen[title] {
				seen[title] = true
				vals = append(vals, title)
			}
		}
	}
	sort.Strings(vals)
	return vals
}

// ScopeNav holds prev/next navigation context for browsing through a list of entities.
type ScopeNav struct {
	PrevURL  string // URL for previous entity (empty if at first)
	NextURL  string // URL for next entity (empty if at last)
	Progress string // e.g. "[3/12]"
	Label    string // e.g. "12 tickets" or "5 results for 'auth'"
	BackURL  string // URL to return to the list/search
}

// resolveScope parses the "scope" query parameter and reconstructs the ordered
// entity list to determine prev/next navigation links. Returns nil when no
// scope is present or the current entity isn't found in the scope.
func (a *App) resolveScope(currentEntityID string, r *http.Request) *ScopeNav {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		return nil
	}

	s := a.State()
	var ids []string
	var label string
	var backURL string

	switch {
	case strings.HasPrefix(scope, "list:"):
		listID := strings.TrimPrefix(scope, "list:")
		list, ok := s.Cfg.Lists[listID]
		if !ok {
			return nil
		}
		entities := s.Graph.NodesByType(list.EntityType)
		entities = applyFilters(entities, list.Filters)

		// Apply dynamic filter params (same as handleList)
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

		// Apply sort (same as handleList)
		sortProp := r.URL.Query().Get("sort")
		sortDir := r.URL.Query().Get("sort_dir")
		if sortProp != "" {
			a.sortEntitiesMulti(entities, []filter.SortSpec{{Property: sortProp, Direction: sortDir}})
		} else {
			a.sortEntitiesMulti(entities, toFilterSortSpecs(list.Sort))
		}

		ids = make([]string, len(entities))
		for i, e := range entities {
			ids[i] = e.ID
		}
		label = fmt.Sprintf("%d %s", len(ids), list.Title)
		backURL = "/list/" + listID

	case strings.HasPrefix(scope, "search:"):
		query := strings.TrimPrefix(scope, "search:")
		entities := a.executeQuery(query)
		sort.Slice(entities, func(i, j int) bool { return natsort.Less(entities[i].ID, entities[j].ID) })
		ids = make([]string, len(entities))
		for i, e := range entities {
			ids[i] = e.ID
		}
		displayQuery := query
		if len(displayQuery) > 30 {
			displayQuery = displayQuery[:30] + "..."
		}
		label = fmt.Sprintf("%d results for \"%s\"", len(ids), displayQuery)
		backURL = "/search?q=" + url.QueryEscape(query)

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
		BackURL:  backURL,
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

// matchesPropertyFilters checks whether an entity matches the given property filters.
// Returns true if no filters are specified or all filters match.
func (a *App) matchesPropertyFilters(e *model.Entity, filters []*filter.Filter) bool {
	if len(filters) == 0 {
		return true
	}
	s := a.State()
	entDef, ok := s.Meta.GetEntityDef(e.Type)
	if !ok {
		return false
	}
	matched, err := filter.MatchAll(entityRecord(e), filters, entDef, s.Meta)
	return err == nil && matched
}

// isRelationLinked checks whether a form relation field (formRel) corresponds
// to a link relation (linkRel) coming from a view's "Add" button. It returns
// true when the link relation's inverse matches the form relation, when the
// form relation's inverse matches the link relation, or when they are equal.
func (a *App) isRelationLinked(formRel, linkRel string) bool {
	if formRel == linkRel {
		return true
	}
	s := a.State()
	// Check if linkRel has an inverse that equals formRel.
	if def, ok := s.Meta.GetRelationDef(linkRel); ok && def.Inverse != nil {
		if def.Inverse.GetID() == formRel {
			return true
		}
	}
	// Check if formRel has an inverse that equals linkRel.
	if def, ok := s.Meta.GetRelationDef(formRel); ok && def.Inverse != nil {
		if def.Inverse.GetID() == linkRel {
			return true
		}
	}
	return false
}
