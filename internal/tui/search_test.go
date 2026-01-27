package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// createTestAppForSearch creates a test app with diverse entities for search testing
func createTestAppForSearch() *App {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
				Properties: map[string]metamodel.PropertyDef{
					"title":       {Type: "string", Required: true},
					"description": {Type: "string", Required: false},
				},
			},
			"decision": {
				Label:      "Decision",
				IDPatterns: []string{"DEC-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
	}

	g := graph.New()

	// Add test entities with different searchable content
	g.AddNode(&model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":       "User Authentication",
			"description": "The system must authenticate users",
		},
		Content: "Authentication requirements including password policy.",
	})
	g.AddNode(&model.Entity{
		ID:   "REQ-002",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":       "Data Security",
			"description": "Protect sensitive data",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "REQ-003",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title": "Performance Requirements",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "DEC-001",
		Type: "decision",
		Properties: map[string]interface{}{
			"title": "Use OAuth for Authentication",
		},
		Content: "We decided to use OAuth 2.0 for user authentication.",
	})

	app := &App{
		metamodel: meta,
		graph:     g,
		project:   &project.Context{Root: "/tmp/test"},
	}

	return app
}

func TestSearchModel_Initialization(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	if search == nil {
		t.Fatal("NewSearchModel returned nil")
	}

	if search.query != "" {
		t.Errorf("search.query should be empty, got %q", search.query)
	}

	if search.cursorPos != 0 {
		t.Errorf("search.cursorPos should be 0, got %d", search.cursorPos)
	}

	if search.searched {
		t.Error("search.searched should be false initially")
	}

	if search.results != nil {
		t.Error("search.results should be nil initially")
	}
}

func TestSearchModel_InputCapture(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Type "hello"
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})

	if search.query != "hello" {
		t.Errorf("search.query = %q, want 'hello'", search.query)
	}

	if search.cursorPos != 5 {
		t.Errorf("search.cursorPos = %d, want 5", search.cursorPos)
	}
}

func TestSearchModel_InputCaptureWithSpace(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Type "hello world"
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	search.Update(app, tea.KeyMsg{Type: tea.KeySpace})
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world")})

	if search.query != "hello world" {
		t.Errorf("search.query = %q, want 'hello world'", search.query)
	}

	if search.cursorPos != 11 {
		t.Errorf("search.cursorPos = %d, want 11", search.cursorPos)
	}
}

func TestSearchModel_Backspace(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Type "hello"
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})

	// Press backspace
	search.Update(app, tea.KeyMsg{Type: tea.KeyBackspace})

	if search.query != "hell" {
		t.Errorf("search.query = %q, want 'hell'", search.query)
	}

	if search.cursorPos != 4 {
		t.Errorf("search.cursorPos = %d, want 4", search.cursorPos)
	}
}

func TestSearchModel_BackspaceAtStart(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Press backspace with empty query (should not crash)
	search.Update(app, tea.KeyMsg{Type: tea.KeyBackspace})

	if search.query != "" {
		t.Errorf("search.query should be empty, got %q", search.query)
	}

	if search.cursorPos != 0 {
		t.Errorf("search.cursorPos = %d, want 0", search.cursorPos)
	}
}

func TestSearchModel_CursorMovement(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Type "hello"
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})

	// Cursor should be at end (5)
	if search.cursorPos != 5 {
		t.Fatalf("cursorPos should be 5, got %d", search.cursorPos)
	}

	// Move left
	search.Update(app, tea.KeyMsg{Type: tea.KeyLeft})
	if search.cursorPos != 4 {
		t.Errorf("after left, cursorPos = %d, want 4", search.cursorPos)
	}

	// Move right
	search.Update(app, tea.KeyMsg{Type: tea.KeyRight})
	if search.cursorPos != 5 {
		t.Errorf("after right, cursorPos = %d, want 5", search.cursorPos)
	}

	// Move left to start
	for i := 0; i < 5; i++ {
		search.Update(app, tea.KeyMsg{Type: tea.KeyLeft})
	}
	if search.cursorPos != 0 {
		t.Errorf("after moving to start, cursorPos = %d, want 0", search.cursorPos)
	}

	// Try to move left again (should stay at 0)
	search.Update(app, tea.KeyMsg{Type: tea.KeyLeft})
	if search.cursorPos != 0 {
		t.Errorf("after left at start, cursorPos = %d, want 0", search.cursorPos)
	}
}

func TestSearchModel_ClearInput(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Type "hello"
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})

	// Press Ctrl+U to clear
	search.Update(app, tea.KeyMsg{Type: tea.KeyCtrlU})

	if search.query != "" {
		t.Errorf("search.query should be empty after Ctrl+U, got %q", search.query)
	}

	if search.cursorPos != 0 {
		t.Errorf("search.cursorPos should be 0 after Ctrl+U, got %d", search.cursorPos)
	}

	if search.searched {
		t.Error("search.searched should be false after Ctrl+U")
	}

	if search.results != nil {
		t.Error("search.results should be nil after Ctrl+U")
	}
}

func TestSearchModel_SearchByID(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Search for "REQ-001"
	search.query = "REQ-001"
	search.search(app)

	if !search.searched {
		t.Error("search.searched should be true after search")
	}

	if len(search.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(search.results))
	}

	if search.results[0].ID != "REQ-001" {
		t.Errorf("result ID = %q, want 'REQ-001'", search.results[0].ID)
	}
}

func TestSearchModel_SearchByTitle(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Search for "authentication"
	search.query = "authentication"
	search.search(app)

	// Should find REQ-001 (title) and DEC-001 (title)
	if len(search.results) != 2 {
		t.Errorf("expected 2 results, got %d", len(search.results))
	}

	// Check that both entities are in results
	foundREQ001 := false
	foundDEC001 := false
	for _, result := range search.results {
		if result.ID == "REQ-001" {
			foundREQ001 = true
		}
		if result.ID == "DEC-001" {
			foundDEC001 = true
		}
	}

	if !foundREQ001 {
		t.Error("REQ-001 should be in results")
	}
	if !foundDEC001 {
		t.Error("DEC-001 should be in results")
	}
}

func TestSearchModel_SearchByDescription(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Search for "sensitive"
	search.query = "sensitive"
	search.search(app)

	// Should find REQ-002 (description contains "sensitive")
	if len(search.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(search.results))
	}

	if search.results[0].ID != "REQ-002" {
		t.Errorf("result ID = %q, want 'REQ-002'", search.results[0].ID)
	}
}

func TestSearchModel_SearchByContent(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Search for "OAuth"
	search.query = "OAuth"
	search.search(app)

	// Should find DEC-001 (content contains "OAuth")
	if len(search.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(search.results))
	}

	if search.results[0].ID != "DEC-001" {
		t.Errorf("result ID = %q, want 'DEC-001'", search.results[0].ID)
	}
}

func TestSearchModel_SearchCaseInsensitive(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Search for "AUTHENTICATION" (uppercase)
	search.query = "AUTHENTICATION"
	search.search(app)

	// Should find entities with "Authentication" (mixed case)
	if len(search.results) < 1 {
		t.Error("search should be case-insensitive and find at least 1 result")
	}
}

func TestSearchModel_SearchNoResults(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Search for something that doesn't exist
	search.query = "nonexistent"
	search.search(app)

	if !search.searched {
		t.Error("search.searched should be true after search")
	}

	if len(search.results) != 0 {
		t.Errorf("expected 0 results, got %d", len(search.results))
	}
}

func TestSearchModel_SearchEmptyQuery(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Search with empty query
	search.query = ""
	search.search(app)

	if !search.searched {
		t.Error("search.searched should be true after search")
	}

	if len(search.results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(search.results))
	}
}

func TestSearchModel_SearchResultNavigation(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Perform a search that returns multiple results
	search.query = "requirement"
	search.search(app)

	// Should have multiple results
	if len(search.results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(search.results))
	}

	// Start at index 0
	if search.resultIndex != 0 {
		t.Fatalf("resultIndex should start at 0, got %d", search.resultIndex)
	}

	// Navigate down
	search.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if search.resultIndex != 1 {
		t.Errorf("after down, resultIndex = %d, want 1", search.resultIndex)
	}

	// Navigate up
	search.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if search.resultIndex != 0 {
		t.Errorf("after up, resultIndex = %d, want 0", search.resultIndex)
	}

	// Navigate up again (should stay at 0)
	search.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if search.resultIndex != 0 {
		t.Errorf("after up at start, resultIndex = %d, want 0", search.resultIndex)
	}
}

func TestSearchModel_SearchWithEnter(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Type a query
	search.query = "authentication"
	search.cursorPos = len(search.query)

	// Press Enter to search
	search.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	if !search.searched {
		t.Error("search.searched should be true after pressing Enter")
	}

	if len(search.results) == 0 {
		t.Error("should have found results after pressing Enter")
	}
}

func TestSearchModel_View(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	view := search.View(80, 40)

	// Should contain "Search" title
	if !contains(view, "Search") {
		t.Error("View should contain 'Search' title")
	}

	// Should show cursor
	if !contains(view, "_") {
		t.Error("View should show cursor")
	}

	// Should show instructions
	if !contains(view, "Type to search") {
		t.Error("View should show instructions")
	}
}

func TestSearchModel_ViewWithResults(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Perform a search
	search.query = "authentication"
	search.search(app)

	view := search.View(80, 40)

	// Should show results count
	if !contains(view, "Found") {
		t.Error("View should show 'Found' with results")
	}

	if !contains(view, "results") {
		t.Error("View should show 'results' text")
	}

	// Should show result entities
	if !contains(view, "REQ-001") {
		t.Error("View should show REQ-001 in results")
	}

	// Should show selection marker
	if !contains(view, "►") {
		t.Error("View should show selection marker")
	}
}

func TestSearchModel_ViewWithNoResults(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Search for something that doesn't exist
	search.query = "nonexistent"
	search.search(app)

	view := search.View(80, 40)

	// Should show "No results found" message
	if !contains(view, "No results found") {
		t.Error("View should show 'No results found' message")
	}
}

func TestSearchModel_Help(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Help before search
	help := search.Help()
	if len(help) == 0 {
		t.Error("Help should return items before search")
	}

	// Check for expected keys
	foundSearch := false
	foundClear := false
	for _, item := range help {
		if item[1] == "search" {
			foundSearch = true
		}
		if item[1] == "clear" {
			foundClear = true
		}
	}
	if !foundSearch {
		t.Error("Help should contain 'search'")
	}
	if !foundClear {
		t.Error("Help should contain 'clear'")
	}

	// Help after search with results
	search.query = "authentication"
	search.search(app)
	help = search.Help()

	foundNavigate := false
	foundOpen := false
	for _, item := range help {
		if item[1] == "navigate" {
			foundNavigate = true
		}
		if item[1] == "open" {
			foundOpen = true
		}
	}
	if !foundNavigate {
		t.Error("Help should contain 'navigate' after search with results")
	}
	if !foundOpen {
		t.Error("Help should contain 'open' after search with results")
	}
}

func TestSearchModel_TypingClearsSearchedFlag(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Perform a search
	search.query = "authentication"
	search.search(app)

	if !search.searched {
		t.Fatal("search.searched should be true after search")
	}

	// Type a new character
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// Should clear searched flag
	if search.searched {
		t.Error("search.searched should be false after typing new character")
	}
}

func TestSearchModel_InsertAtCursor(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Type "hello"
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})

	// Move cursor to position 2 (between 'e' and 'l')
	search.cursorPos = 2

	// Type 'x'
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	if search.query != "hexllo" {
		t.Errorf("search.query = %q, want 'hexllo'", search.query)
	}

	if search.cursorPos != 3 {
		t.Errorf("search.cursorPos = %d, want 3", search.cursorPos)
	}
}

func TestSearchModel_BackspaceAtCursor(t *testing.T) {
	app := createTestAppForSearch()
	search := NewSearchModel(app)

	// Type "hello"
	search.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})

	// Move cursor to position 3 (after 'l')
	search.cursorPos = 3

	// Press backspace (should delete 'l')
	search.Update(app, tea.KeyMsg{Type: tea.KeyBackspace})

	if search.query != "helo" {
		t.Errorf("search.query = %q, want 'helo'", search.query)
	}

	if search.cursorPos != 2 {
		t.Errorf("search.cursorPos = %d, want 2", search.cursorPos)
	}
}

func TestSearchModel_SearchLimit(t *testing.T) {
	// Create app with many entities
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
	}

	g := graph.New()

	// Add 60 entities (more than the 50 limit)
	for i := 1; i <= 60; i++ {
		g.AddNode(&model.Entity{
			ID:   "REQ-" + padNumber(i, 3),
			Type: "requirement",
			Properties: map[string]interface{}{
				"title": "Requirement",
			},
		})
	}

	app := &App{
		metamodel: meta,
		graph:     g,
		project:   &project.Context{Root: "/tmp/test"},
	}

	search := NewSearchModel(app)

	// Search for "requirement" (should match all entities)
	search.query = "requirement"
	search.search(app)

	// Should be limited to 50 results
	if len(search.results) != 50 {
		t.Errorf("results should be limited to 50, got %d", len(search.results))
	}
}

// Helper function to pad numbers with leading zeros
func padNumber(n, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s = "0" + s
	}
	s += string(rune('0' + n%10))
	n /= 10
	for i := 1; i < width && n > 0; i++ {
		s = s[:width-i-1] + string(rune('0'+n%10)) + s[width-i:]
		n /= 10
	}
	return s
}
