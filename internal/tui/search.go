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

// Update handles key events
func (s *SearchModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if len(s.results) > 0 {
			// Open selected result
			entity := s.results[s.resultIndex]
			app.detail = NewDetailModel(app, entity.ID)
			return app, app.pushScreen(ScreenDetail)
		}
		return app, nil

	case "backspace":
		if s.cursorPos > 0 && s.query != "" {
			s.query = s.query[:s.cursorPos-1] + s.query[s.cursorPos:]
			s.cursorPos--
			// Trigger live search
			return app, s.triggerSearch()
		}

	case "j", "down":
		if s.resultIndex < len(s.results)-1 {
			s.resultIndex++
		}

	case "k", "up":
		if s.resultIndex > 0 {
			s.resultIndex--
		}

	case "left":
		if s.cursorPos > 0 {
			s.cursorPos--
		}

	case "right":
		if s.cursorPos < len(s.query) {
			s.cursorPos++
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
			// Trigger live search
			return app, s.triggerSearch()
		} else if msg.Type == tea.KeySpace {
			s.query = s.query[:s.cursorPos] + " " + s.query[s.cursorPos:]
			s.cursorPos++
			// Trigger live search
			return app, s.triggerSearch()
		}
	}
	return app, nil
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
			if strings.EqualFold(entity.Type, t) {
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

	// Results
	if s.lastQuery != "" {
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
	} else if s.query == "" {
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
