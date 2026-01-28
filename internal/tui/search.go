package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/filter"
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
	searchVersion int // Track search version to ignore stale results
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
	query   string
	version int
}

// searchResultsMsg is sent when search completes
type searchResultsMsg struct {
	results []*model.Entity
	query   string
	version int
	errors  []string
}

// NewSearchModel creates a new search screen
func NewSearchModel(_ *App) *SearchModel {
	return &SearchModel{}
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

	case "status":
		// Status is a common property, get its values if it's an enum
		s.suggestions = []string{}
		for _, entityDef := range app.metamodel.Entities {
			if statusProp, ok := entityDef.Properties["status"]; ok {
				for _, val := range statusProp.Values {
					if strings.HasPrefix(strings.ToLower(val), strings.ToLower(ctx.Prefix)) {
						s.suggestions = append(s.suggestions, val)
					}
				}
				break // Assume status values are same across types
			}
		}
		s.showSuggestions = len(s.suggestions) > 0

	default:
		s.showSuggestions = false
		s.suggestions = nil
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
		// Otherwise, open selected result
		if len(s.results) > 0 {
			entity := s.results[s.resultIndex]
			app.detail = NewDetailModel(app, entity.ID)
			return app, app.pushScreen(ScreenDetail)
		}
		return app, nil

	case "backspace":
		if s.cursorPos > 0 && s.query != "" {
			s.query = s.query[:s.cursorPos-1] + s.query[s.cursorPos:]
			s.cursorPos--
			s.updateSuggestions(app)
			// Trigger live search
			return app, s.triggerSearch()
		}

	case "j", "down":
		// If showing suggestions, navigate suggestions
		if s.showSuggestions {
			if s.suggestionIndex < len(s.suggestions)-1 {
				s.suggestionIndex++
			}
		} else if s.resultIndex < len(s.results)-1 {
			s.resultIndex++
		}

	case "k", "up":
		// If showing suggestions, navigate suggestions
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
		s.parseErrors = nil
		s.searching = false
		s.lastQuery = ""

	default:
		// Insert character
		if msg.Type == tea.KeyRunes {
			chars := string(msg.Runes)
			s.query = s.query[:s.cursorPos] + chars + s.query[s.cursorPos:]
			s.cursorPos += len(msg.Runes)
			s.updateSuggestions(app)
			// Trigger live search
			return app, s.triggerSearch()
		} else if msg.Type == tea.KeySpace {
			s.query = s.query[:s.cursorPos] + " " + s.query[s.cursorPos:]
			s.cursorPos++
			s.updateSuggestions(app)
			// Trigger live search
			return app, s.triggerSearch()
		}
	}
	return app, nil
}

// acceptSuggestion inserts the selected suggestion into the query
func (s *SearchModel) acceptSuggestion() {
	if !s.showSuggestions || len(s.suggestions) == 0 {
		return
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
	case "status":
		s.query = prefix + "status:" + suggestion + " " + suffix
		s.cursorPos = len(prefix) + len("status:") + len(suggestion) + 1
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
	query := s.query

	return func() tea.Msg {
		// Debounce: wait 200ms
		time.Sleep(200 * time.Millisecond)
		return searchQueryMsg{
			query:   query,
			version: version,
		}
	}
}

// HandleSearchQuery processes a search query message (after debounce)
func (s *SearchModel) HandleSearchQuery(app *App, msg searchQueryMsg) tea.Cmd {
	// Ignore if this is a stale search
	if msg.version != s.searchVersion {
		return nil
	}

	// Perform search in background
	return func() tea.Msg {
		results, errors := s.performSearch(app, msg.query)
		return searchResultsMsg{
			results: results,
			query:   msg.query,
			version: msg.version,
			errors:  errors,
		}
	}
}

// HandleSearchResults processes search results
func (s *SearchModel) HandleSearchResults(msg searchResultsMsg) {
	// Ignore if this is a stale result
	if msg.version != s.searchVersion {
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

	return filtered, nil
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

	// Show parse errors
	if len(s.parseErrors) > 0 {
		sb.WriteString(errorStyle.Render("⚠ " + strings.Join(s.parseErrors, "; ")))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Show autocomplete suggestions if available
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
		sb.WriteString("\n")
		sb.WriteString("  type:requirement prop:priority>3 auth\n")
		sb.WriteString("  type:decision,solution \"REST API\"\n")
	}

	return sb.String()
}

// highlightSyntax applies basic syntax highlighting to the query
func (s *SearchModel) highlightSyntax(text string) string {
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))   // blue
	propStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))   // green
	quoteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // yellow

	// Simple highlighting - check prefixes
	if strings.HasPrefix(text, "type:") {
		return typeStyle.Render(text)
	}
	if strings.HasPrefix(text, "prop:") {
		return propStyle.Render(text)
	}
	if strings.HasPrefix(text, "status:") {
		return propStyle.Render(text)
	}
	if strings.HasPrefix(text, "\"") {
		return quoteStyle.Render(text)
	}

	// Highlight individual tokens
	tokens := strings.Fields(text)
	var highlighted []string
	for _, token := range tokens {
		switch {
		case strings.HasPrefix(token, "type:"):
			highlighted = append(highlighted, typeStyle.Render(token))
		case strings.HasPrefix(token, "prop:"), strings.HasPrefix(token, "status:"):
			highlighted = append(highlighted, propStyle.Render(token))
		case strings.HasPrefix(token, "\""):
			highlighted = append(highlighted, quoteStyle.Render(token))
		default:
			highlighted = append(highlighted, token)
		}
	}

	return strings.Join(highlighted, " ")
}

// Help returns help items
func (s *SearchModel) Help() [][2]string {
	if len(s.results) > 0 {
		return [][2]string{
			{"↑/↓", "navigate"},
			{"enter", "open"},
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
