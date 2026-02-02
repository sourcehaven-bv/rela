package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/tui/searchparser"
)

// SearchModel handles entity search
type SearchModel struct {
	query         string
	cursorPos     int
	results       []*model.Entity
	resultIndex   int
	searching     bool
	searchVersion int // Incremented per keystroke to ignore stale results
	baseVersion   int // Base version from app counter at creation time
	lastQuery     string
	parseErrors   []string

	// Autocomplete state
	suggestions       []string
	suggestionIndex   int
	showSuggestions   bool
	autocompleteType  string // "type", "prop", or ""
	autocompleteQuery string // The query when suggestions were generated
}

// searchQueryMsg is sent when user types (debounced)
type searchQueryMsg struct {
	query       string
	version     int
	baseVersion int // App-level base version
}

// searchResultsMsg is sent when search completes
type searchResultsMsg struct {
	results     []*model.Entity
	query       string
	version     int
	baseVersion int // App-level base version
	errors      []string
}

// Constants for search behavior
const (
	searchDebounceDelay = 200 * time.Millisecond
	maxQueryLength      = 1000
	maxSuggestions      = 10
)

// valueCount tracks property value frequency for autocomplete ranking
type valueCount struct {
	val   string
	count int
}

// NewSearchModel creates a new search screen
func NewSearchModel(app *App) *SearchModel {
	return &SearchModel{
		baseVersion: app.searchVersionCounter,
	}
}

// getPropertyValueSuggestions returns autocomplete suggestions for property values
// ranked by frequency in current results, or from global index if no results yet
func (s *SearchModel) getPropertyValueSuggestions(app *App, propertyName, prefix string) []string {
	suggestions := []string{}

	// Strategy 1: If we have search results, extract values from them and rank by frequency
	if len(s.results) > 0 {
		valueMap := make(map[string]int)
		for _, entity := range s.results {
			if val, ok := entity.Properties[propertyName]; ok {
				strVal := s.valueToString(val)
				if strVal != "" && strings.HasPrefix(strings.ToLower(strVal), strings.ToLower(prefix)) {
					valueMap[strVal]++
				}
			}
		}

		// Convert to slice and sort by frequency (descending), then alphabetically
		sorted := make([]valueCount, 0, len(valueMap))
		for v, c := range valueMap {
			sorted = append(sorted, valueCount{v, c})
		}
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].count != sorted[j].count {
				return sorted[i].count > sorted[j].count // Higher count first
			}
			return sorted[i].val < sorted[j].val // Alphabetical for same count
		})

		// Take top N suggestions
		for i := 0; i < len(sorted) && i < maxSuggestions; i++ {
			suggestions = append(suggestions, sorted[i].val)
		}
	} else {
		// Strategy 2: No results yet - use global property index from graph
		allValues := app.graph.GetPropertyValues(propertyName, 100)
		for _, val := range allValues {
			if strings.HasPrefix(strings.ToLower(val), strings.ToLower(prefix)) {
				suggestions = append(suggestions, val)
				if len(suggestions) >= maxSuggestions {
					break
				}
			}
		}
	}

	return suggestions
}

// updateParseErrors validates the query and updates parse errors immediately
func (s *SearchModel) updateParseErrors() {
	if s.query == "" {
		s.parseErrors = nil
		return
	}

	// Parse query to get immediate validation errors
	sq := searchparser.ParseQuery(s.query)
	s.parseErrors = sq.ParseErrors
}

// updateSuggestions generates autocomplete suggestions based on cursor position
func (s *SearchModel) updateSuggestions(app *App) {
	ctx := searchparser.GetAutocompleteContext(s.query, s.cursorPos)

	// Clear suggestions if not in autocomplete context
	if ctx.Type == "" {
		s.showSuggestions = false
		s.suggestions = nil
		return
	}

	// Don't regenerate if query hasn't changed
	if s.autocompleteQuery == s.query && s.autocompleteType == ctx.Type {
		return
	}

	s.autocompleteQuery = s.query
	s.autocompleteType = ctx.Type
	s.suggestionIndex = 0

	switch ctx.Type {
	case "type":
		// Get all entity types from metamodel
		s.suggestions = []string{}
		for typeName := range app.metamodel.Entities {
			// Filter by prefix
			if strings.HasPrefix(strings.ToLower(typeName), strings.ToLower(ctx.Prefix)) {
				s.suggestions = append(s.suggestions, typeName)
			}
		}
		s.showSuggestions = len(s.suggestions) > 0

	case "prop":
		// Get all property names from all entity types
		propMap := make(map[string]bool)
		for _, entityDef := range app.metamodel.Entities {
			for propName := range entityDef.Properties {
				if strings.HasPrefix(strings.ToLower(propName), strings.ToLower(ctx.Prefix)) {
					propMap[propName] = true
				}
			}
		}
		s.suggestions = []string{}
		for propName := range propMap {
			s.suggestions = append(s.suggestions, propName)
		}
		s.showSuggestions = len(s.suggestions) > 0

	case "propvalue":
		// Get property values ranked by frequency
		s.suggestions = s.getPropertyValueSuggestions(app, ctx.PropertyName, ctx.Prefix)
		s.showSuggestions = len(s.suggestions) > 0

	case "status":
		// Status is a common property shortcut, use same logic as propvalue
		s.suggestions = s.getPropertyValueSuggestions(app, "status", ctx.Prefix)
		s.showSuggestions = len(s.suggestions) > 0

	case "sort":
		// Suggest property names plus virtual properties "id" and "modified"
		propMap := make(map[string]bool)
		propMap["id"] = true
		propMap["modified"] = true
		for _, entityDef := range app.metamodel.Entities {
			for propName := range entityDef.Properties {
				propMap[propName] = true
			}
		}
		s.suggestions = []string{}
		for propName := range propMap {
			if strings.HasPrefix(strings.ToLower(propName), strings.ToLower(ctx.Prefix)) {
				s.suggestions = append(s.suggestions, propName)
			}
		}
		sort.Strings(s.suggestions)
		s.showSuggestions = len(s.suggestions) > 0

	case "sortdir":
		// Suggest "asc" and "desc"
		s.suggestions = []string{}
		for _, dir := range []string{"asc", "desc"} {
			if strings.HasPrefix(dir, strings.ToLower(ctx.Prefix)) {
				s.suggestions = append(s.suggestions, dir)
			}
		}
		s.showSuggestions = len(s.suggestions) > 0

	default:
		s.showSuggestions = false
		s.suggestions = nil
	}
}

// valueToString converts a property value to string
func (s *SearchModel) valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Update handles key events
func (s *SearchModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Accept autocomplete suggestion
		if s.showSuggestions && len(s.suggestions) > 0 {
			s.acceptSuggestion()
			s.updateSuggestions(app)
			return app, s.triggerSearch()
		}

	case "enter":
		// If showing suggestions, accept the selected suggestion
		if s.showSuggestions && len(s.suggestions) > 0 {
			s.acceptSuggestion()
			s.updateSuggestions(app)
			return app, s.triggerSearch()
		}
		// Otherwise, open selected result (single item, no scope)
		if len(s.results) > 0 {
			entity := s.results[s.resultIndex]
			app.detail = NewDetailModel(app, entity.ID)
			return app, app.pushScreen(ScreenDetail)
		}
		return app, nil

	case "ctrl+b":
		// Browse all results with scope navigation (ctrl+b since 'b' is text input)
		if !s.showSuggestions && len(s.results) > 0 {
			return s.enterBrowseMode(app)
		}

	case "backspace":
		if s.cursorPos > 0 && s.query != "" {
			s.query = s.query[:s.cursorPos-1] + s.query[s.cursorPos:]
			s.cursorPos--
			s.updateParseErrors()
			s.updateSuggestions(app)
			// Trigger live search
			return app, s.triggerSearch()
		}

	case "down":
		// Navigate suggestions or results (arrow keys only — j/k are text input on search screen)
		if s.showSuggestions {
			if s.suggestionIndex < len(s.suggestions)-1 {
				s.suggestionIndex++
			}
		} else if s.resultIndex < len(s.results)-1 {
			s.resultIndex++
		}

	case "up":
		// Navigate suggestions or results (arrow keys only — j/k are text input on search screen)
		if s.showSuggestions {
			if s.suggestionIndex > 0 {
				s.suggestionIndex--
			}
		} else if s.resultIndex > 0 {
			s.resultIndex--
		}

	case "left":
		if s.cursorPos > 0 {
			s.cursorPos--
			s.updateSuggestions(app)
		}

	case "right":
		if s.cursorPos < len(s.query) {
			s.cursorPos++
			s.updateSuggestions(app)
		}

	case "ctrl+u":
		s.query = ""
		s.cursorPos = 0
		s.results = nil
		s.resultIndex = 0
		s.parseErrors = nil
		s.searching = false
		s.lastQuery = ""
		s.showSuggestions = false
		s.suggestions = nil

	default:
		// Insert character
		if msg.Type == tea.KeyRunes {
			chars := string(msg.Runes)
			newQuery := s.query[:s.cursorPos] + chars + s.query[s.cursorPos:]
			// Enforce max query length
			if len(newQuery) <= maxQueryLength {
				s.query = newQuery
				s.cursorPos += len(msg.Runes)
				s.updateParseErrors()
				s.updateSuggestions(app)
				// Trigger live search
				return app, s.triggerSearch()
			}
		} else if msg.Type == tea.KeySpace {
			newQuery := s.query[:s.cursorPos] + " " + s.query[s.cursorPos:]
			// Enforce max query length
			if len(newQuery) <= maxQueryLength {
				s.query = newQuery
				s.cursorPos++
				s.updateParseErrors()
				s.updateSuggestions(app)
				// Trigger live search
				return app, s.triggerSearch()
			}
		}
	}
	return app, nil
}

// acceptSuggestion inserts the selected suggestion into the query
func (s *SearchModel) acceptSuggestion() {
	if !s.showSuggestions || len(s.suggestions) == 0 {
		return
	}

	// Defensive: ensure cursor is within bounds
	if s.cursorPos > len(s.query) {
		s.cursorPos = len(s.query)
	}
	if s.cursorPos < 0 {
		s.cursorPos = 0
	}

	suggestion := s.suggestions[s.suggestionIndex]
	ctx := searchparser.GetAutocompleteContext(s.query, s.cursorPos)

	// Find where the current token starts
	textToCursor := s.query[:s.cursorPos]
	lastSpace := strings.LastIndexAny(textToCursor, " \t")
	tokenStart := 0
	if lastSpace != -1 {
		tokenStart = lastSpace + 1
	}

	// Replace the incomplete token with the suggestion
	prefix := s.query[:tokenStart]
	suffix := s.query[s.cursorPos:]

	switch ctx.Type {
	case "type":
		s.query = prefix + "type:" + suggestion + " " + suffix
		s.cursorPos = len(prefix) + len("type:") + len(suggestion) + 1
	case "prop":
		s.query = prefix + "prop:" + suggestion + "=" + suffix
		s.cursorPos = len(prefix) + len("prop:") + len(suggestion) + 1
	case "propvalue":
		// Find the operator position in the current token
		currentToken := s.query[tokenStart:s.cursorPos]
		opPos := -1
		opLen := 0

		// Check multi-char operators first to avoid matching "=" in "=~"
		for _, op := range []string{"!=", "<=", ">=", "=~"} {
			if idx := strings.Index(currentToken, op); idx != -1 {
				opPos = idx
				opLen = len(op)
				break
			}
		}

		// Then check single-char operators
		if opPos == -1 {
			for _, op := range []string{"=", "<", ">"} {
				if idx := strings.Index(currentToken, op); idx != -1 {
					opPos = idx
					opLen = len(op)
					break
				}
			}
		}

		// Replace everything after the operator
		if opPos != -1 {
			beforeOp := s.query[:tokenStart] + currentToken[:opPos+opLen]
			s.query = beforeOp + suggestion + " " + suffix
			s.cursorPos = len(beforeOp) + len(suggestion) + 1
		} else {
			// Defensive: if no operator found (shouldn't happen), just append to property
			// This handles C2 - the silent failure case
			s.query = prefix + "prop:" + ctx.PropertyName + "=" + suggestion + " " + suffix
			s.cursorPos = len(prefix) + len("prop:") + len(ctx.PropertyName) + 1 + len(suggestion) + 1
		}
	case "status":
		s.query = prefix + "status:" + suggestion + " " + suffix
		s.cursorPos = len(prefix) + len("status:") + len(suggestion) + 1
	case "sort":
		// After accepting a sort property, add ":" so user can type direction
		s.query = prefix + "sort:" + suggestion + ":" + suffix
		s.cursorPos = len(prefix) + len("sort:") + len(suggestion) + 1
	case "sortdir":
		// Find the sort: token start, rebuild with chosen direction
		currentToken := s.query[tokenStart:s.cursorPos]
		// currentToken is like "sort:property:de" — replace everything after last ":"
		lastColon := strings.LastIndex(currentToken, ":")
		if lastColon != -1 {
			beforeDir := s.query[:tokenStart] + currentToken[:lastColon+1]
			s.query = beforeDir + suggestion + " " + suffix
			s.cursorPos = len(beforeDir) + len(suggestion) + 1
		}
	}

	// Hide suggestions after accepting
	s.showSuggestions = false
	s.suggestions = nil
}

// triggerSearch creates a debounced search command
func (s *SearchModel) triggerSearch() tea.Cmd {
	s.searching = true
	s.searchVersion++
	version := s.searchVersion
	baseVersion := s.baseVersion
	query := s.query

	return func() tea.Msg {
		// Debounce to avoid excessive search triggers
		time.Sleep(searchDebounceDelay)
		return searchQueryMsg{
			query:       query,
			version:     version,
			baseVersion: baseVersion,
		}
	}
}

// HandleSearchQuery processes a search query message (after debounce)
func (s *SearchModel) HandleSearchQuery(app *App, msg searchQueryMsg) tea.Cmd {
	// Ignore if this is a stale search (from old keystroke or old screen instance)
	if msg.version != s.searchVersion || msg.baseVersion != s.baseVersion {
		return nil
	}

	// Perform search in background
	return func() tea.Msg {
		results, errors := s.performSearch(app, msg.query)
		return searchResultsMsg{
			results:     results,
			query:       msg.query,
			version:     msg.version,
			baseVersion: msg.baseVersion,
			errors:      errors,
		}
	}
}

// HandleSearchResults processes search results
func (s *SearchModel) HandleSearchResults(msg searchResultsMsg) {
	// Ignore if this is a stale result (from old keystroke or old screen instance)
	if msg.version != s.searchVersion || msg.baseVersion != s.baseVersion {
		return
	}

	s.results = msg.results
	s.resultIndex = 0
	s.searching = false
	s.lastQuery = msg.query
	s.parseErrors = msg.errors
}

// performSearch executes the search logic
func (s *SearchModel) performSearch(app *App, query string) (results []*model.Entity, errors []string) {
	if query == "" {
		return nil, nil
	}

	// Parse query into components
	sq := searchparser.ParseQuery(query)

	// Return parse errors if any
	if len(sq.ParseErrors) > 0 {
		return nil, sq.ParseErrors
	}

	// Get all entities from graph
	allEntities := app.graph.AllNodes()
	var filtered []*model.Entity

	// Apply filters
	for _, entity := range allEntities {
		if s.matchesFilters(entity, sq) {
			filtered = append(filtered, entity)
		}
	}

	// Validate sort properties against result set
	if sq.HasSort() {
		allProps := collectAllPropertyNames(app.metamodel, filtered)
		for _, sc := range sq.SortClauses {
			if sc.Property != "id" && sc.Property != "modified" && !allProps[sc.Property] {
				errors = append(errors, fmt.Sprintf("sort: unknown property %q", sc.Property))
			}
		}
		if len(errors) > 0 {
			return filtered, errors
		}
	}

	// Apply sort
	if sq.HasSort() {
		entityDefs := collectEntityDefs(app.metamodel, filtered)
		filter.SortMulti(filtered, sq.SortClauses, entityDefs, app.metamodel)
	} else {
		// Apply default_sort if single entity type
		types := uniqueTypes(filtered)
		if len(types) == 1 {
			if def, ok := app.metamodel.GetEntityDef(types[0]); ok && len(def.DefaultSort) > 0 {
				entityDefs := map[string]*metamodel.EntityDef{types[0]: def}
				filter.SortMulti(filtered, def.DefaultSort, entityDefs, app.metamodel)
			} else {
				filter.SortByID(filtered, false)
			}
		} else {
			filter.SortByID(filtered, false)
		}
	}

	return filtered, nil
}

// collectEntityDefs builds a map of entity type definitions for the given entities.
func collectEntityDefs(meta *metamodel.Metamodel, entities []*model.Entity) map[string]*metamodel.EntityDef {
	defs := make(map[string]*metamodel.EntityDef)
	for _, e := range entities {
		if _, ok := defs[e.Type]; !ok {
			if def, ok := meta.GetEntityDef(e.Type); ok {
				defs[e.Type] = def
			}
		}
	}
	return defs
}

// collectAllPropertyNames returns a set of all property names across all entity types in the result set.
func collectAllPropertyNames(meta *metamodel.Metamodel, entities []*model.Entity) map[string]bool {
	seen := make(map[string]bool)
	typesSeen := make(map[string]bool)
	for _, e := range entities {
		if typesSeen[e.Type] {
			continue
		}
		typesSeen[e.Type] = true
		if def, ok := meta.GetEntityDef(e.Type); ok {
			for propName := range def.Properties {
				seen[propName] = true
			}
		}
	}
	return seen
}

// uniqueTypes returns the distinct entity types in the given slice.
func uniqueTypes(entities []*model.Entity) []string {
	seen := make(map[string]bool)
	var types []string
	for _, e := range entities {
		if !seen[e.Type] {
			seen[e.Type] = true
			types = append(types, e.Type)
		}
	}
	return types
}

// matchesFilters checks if an entity matches all search criteria
func (s *SearchModel) matchesFilters(entity *model.Entity, sq *searchparser.SearchQuery) bool {
	// 1. Filter by entity type (if specified)
	if len(sq.EntityTypes) > 0 {
		found := false
		for _, t := range sq.EntityTypes {
			// Support prefix matching (case-insensitive)
			// e.g., "req" matches "requirement", "dec" matches "decision"
			if strings.HasPrefix(strings.ToLower(entity.Type), strings.ToLower(t)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 2. Apply property filters (AND logic)
	for _, propFilter := range sq.PropertyFilters {
		value, exists := entity.Properties[propFilter.Property]
		if !exists {
			return false
		}
		if !filter.MatchValue(value, propFilter) {
			return false
		}
	}

	// 3. Apply free-text search (AND logic - all words must be present)
	if sq.HasFreeText() {
		// Combine all searchable text
		searchableText := strings.ToLower(strings.Join([]string{
			entity.ID,
			entity.Title(),
			entity.Description(),
			entity.Content,
		}, " "))

		// Check all free text words
		for _, word := range sq.FreeTextWords {
			if !strings.Contains(searchableText, strings.ToLower(word)) {
				return false
			}
		}

		// Check all exact phrases
		for _, phrase := range sq.FreeTextPhrases {
			if !strings.Contains(searchableText, strings.ToLower(phrase)) {
				return false
			}
		}
	}

	return true
}

// View renders the search screen
func (s *SearchModel) View(_, height int) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(70)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	idStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	sb.WriteString(titleStyle.Render("Search"))
	sb.WriteString("\n\n")

	// Search input with syntax highlighting
	cursor := "_"
	displayText := s.highlightSyntax(s.query[:s.cursorPos]) + cursor
	if s.cursorPos < len(s.query) {
		displayText += s.highlightSyntax(s.query[s.cursorPos:])
	}
	sb.WriteString(inputStyle.Render(displayText))
	sb.WriteString("\n")

	// Help text
	helpText := "Type to search (live)"
	sb.WriteString(labelStyle.Render(helpText))
	sb.WriteString("\n")

	// Show autocomplete suggestions if available
	// Don't show parse errors when suggestions are available - user is mid-typing
	if s.showSuggestions && len(s.suggestions) > 0 {
		suggestionStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))
		selectedSuggStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

		sb.WriteString(labelStyle.Render("Suggestions:"))
		sb.WriteString("\n\n")

		// Show up to 10 suggestions
		maxSugg := len(s.suggestions)
		if maxSugg > 10 {
			maxSugg = 10
		}

		for i := 0; i < maxSugg; i++ {
			marker := "  "
			style := suggestionStyle
			if i == s.suggestionIndex {
				marker = "► "
				style = selectedSuggStyle
			}
			sb.WriteString(marker + style.Render(s.suggestions[i]) + "\n")
		}

		if len(s.suggestions) > 10 {
			sb.WriteString(labelStyle.Render(fmt.Sprintf("\n  ... and %d more", len(s.suggestions)-10)))
		}
		sb.WriteString("\n")
		sb.WriteString(labelStyle.Render("[Tab] to accept, [↑/↓] to navigate"))
		sb.WriteString("\n")
	} else {
		// Show parse errors only when NOT showing suggestions
		if len(s.parseErrors) > 0 {
			sb.WriteString(errorStyle.Render("⚠ " + strings.Join(s.parseErrors, "; ")))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Results
	if !s.showSuggestions && s.lastQuery != "" {
		if len(s.results) == 0 {
			sb.WriteString(labelStyle.Render("No results found"))
		} else {
			sb.WriteString(labelStyle.Render(fmt.Sprintf("Found %d results:", len(s.results))))
			sb.WriteString("\n\n")

			// Show results with scrolling
			visibleCount := height - 12
			if visibleCount < 5 {
				visibleCount = 5
			}

			startIdx := 0
			if s.resultIndex >= visibleCount {
				startIdx = s.resultIndex - visibleCount + 1
			}
			endIdx := startIdx + visibleCount
			if endIdx > len(s.results) {
				endIdx = len(s.results)
			}

			for i := startIdx; i < endIdx; i++ {
				entity := s.results[i]
				marker := "  "
				style := normalStyle
				if i == s.resultIndex {
					marker = "► "
					style = selectedStyle
				}

				id := idStyle.Render(entity.ID)
				if i == s.resultIndex {
					id = style.Render(entity.ID)
				}

				title := entity.Title()
				if len(title) > 40 {
					title = title[:37] + "..."
				}

				typeLabel := labelStyle.Render(fmt.Sprintf("(%s)", entity.Type))

				line := fmt.Sprintf("%s%-15s %s %s", marker, id, style.Render(title), typeLabel)
				sb.WriteString(line + "\n")
			}

			if len(s.results) > visibleCount {
				sb.WriteString(labelStyle.Render(fmt.Sprintf("\n[%d/%d]", s.resultIndex+1, len(s.results))))
			}
		}
	} else if !s.showSuggestions && s.query == "" {
		// Show help when empty
		sb.WriteString(labelStyle.Render("Search syntax examples:"))
		sb.WriteString("\n\n")
		sb.WriteString("  type:requirement              - filter by entity type\n")
		sb.WriteString("  prop:status=published         - filter by property\n")
		sb.WriteString("  authentication                - free text search\n")
		sb.WriteString("  \"exact phrase\"                - exact phrase match\n")
		sb.WriteString("  sort:priority:desc            - sort by property\n")
		sb.WriteString("  sort:modified:desc            - sort by last modified\n")
		sb.WriteString("\n")
		sb.WriteString("  type:requirement prop:priority>3 auth\n")
		sb.WriteString("  type:decision,solution \"REST API\"\n")
		sb.WriteString("  type:requirement sort:priority:desc sort:title\n")
	}

	return sb.String()
}

// highlightSyntax applies basic syntax highlighting to the query
func (s *SearchModel) highlightSyntax(text string) string {
	if text == "" {
		return text
	}

	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))   // blue
	propStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))   // green
	quoteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // yellow
	sortStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213"))  // magenta

	// Parse text to handle quoted strings correctly
	var result strings.Builder
	i := 0
	for i < len(text) {
		// Skip whitespace
		if text[i] == ' ' || text[i] == '\t' {
			result.WriteByte(text[i])
			i++
			continue
		}

		// Check for quoted string
		if text[i] == '"' {
			start := i
			i++ // Skip opening quote
			// Find closing quote
			for i < len(text) && text[i] != '"' {
				if text[i] == '\\' && i+1 < len(text) {
					i++ // Skip escaped character
				}
				i++
			}
			if i < len(text) {
				i++ // Skip closing quote
			}
			result.WriteString(quoteStyle.Render(text[start:i]))
			continue
		}

		// Regular token - find end
		start := i
		for i < len(text) && text[i] != ' ' && text[i] != '\t' && text[i] != '"' {
			i++
		}
		token := text[start:i]

		// Apply highlighting based on token type
		switch {
		case strings.HasPrefix(token, "type:"):
			result.WriteString(typeStyle.Render(token))
		case strings.HasPrefix(token, "prop:"), strings.HasPrefix(token, "status:"):
			result.WriteString(propStyle.Render(token))
		case strings.HasPrefix(token, "sort:"):
			result.WriteString(sortStyle.Render(token))
		default:
			result.WriteString(token)
		}
	}

	return result.String()
}

// enterBrowseMode creates a browse scope from search results and enters detail view
func (s *SearchModel) enterBrowseMode(app *App) (tea.Model, tea.Cmd) {
	if len(s.results) == 0 {
		return app, nil
	}

	// Collect entity IDs from results
	ids := make([]string, len(s.results))
	for i, entity := range s.results {
		ids[i] = entity.ID
	}

	// Create scope label
	label := fmt.Sprintf("%d search results", len(ids))
	if s.lastQuery != "" {
		// Truncate long queries
		query := s.lastQuery
		if len(query) > 30 {
			query = query[:27] + "..."
		}
		label = fmt.Sprintf("%d results for \"%s\"", len(ids), query)
	}

	// Create browse scope starting at the currently selected result
	scope := NewBrowseScope(ids, label, ScreenSearch)
	if scope == nil {
		return app, nil
	}

	// Set scope to currently selected item
	scope.SetIndex(s.resultIndex)

	// Create detail model with scope
	app.detail = NewDetailModelWithScope(app, scope)
	if app.detail == nil {
		return app, SetMessage("Failed to open entity", true)
	}

	return app, app.pushScreen(ScreenDetail)
}

// Help returns help items
func (s *SearchModel) Help() [][2]string {
	if len(s.results) > 0 {
		return [][2]string{
			{"↑/↓", "navigate"},
			{"enter", "open"},
			{"ctrl+b", "browse all"},
			{"ctrl+u", "clear"},
			{"esc", "back"},
		}
	}
	return [][2]string{
		{"type", "search"},
		{"ctrl+u", "clear"},
		{"esc", "back"},
	}
}
