package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// SearchModel handles entity search
type SearchModel struct {
	query       string
	cursorPos   int
	results     []*model.Entity
	resultIndex int
	searched    bool
}

// NewSearchModel creates a new search screen
func NewSearchModel(_ *App) *SearchModel {
	return &SearchModel{}
}

// Update handles key events
func (s *SearchModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if s.searched && len(s.results) > 0 {
			// Open selected result
			entity := s.results[s.resultIndex]
			app.detail = NewDetailModel(app, entity.ID)
			return app, app.pushScreen(ScreenDetail)
		}
		// Perform search
		s.search(app)
	case "backspace":
		if s.cursorPos > 0 && s.query != "" {
			s.query = s.query[:s.cursorPos-1] + s.query[s.cursorPos:]
			s.cursorPos--
			s.searched = false
		}
	case "j", "down":
		if s.searched && s.resultIndex < len(s.results)-1 {
			s.resultIndex++
		}
	case "k", "up":
		if s.searched && s.resultIndex > 0 {
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
		s.searched = false
		s.results = nil
	default:
		// Insert character - use KeyRunes type for proper character detection
		// This handles both regular keyboard input and automated testing (e.g., tmux send-keys)
		if msg.Type == tea.KeyRunes {
			chars := string(msg.Runes)
			s.query = s.query[:s.cursorPos] + chars + s.query[s.cursorPos:]
			s.cursorPos += len(msg.Runes)
			s.searched = false
		} else if msg.Type == tea.KeySpace {
			s.query = s.query[:s.cursorPos] + " " + s.query[s.cursorPos:]
			s.cursorPos++
			s.searched = false
		}
	}
	return app, nil
}

func (s *SearchModel) search(app *App) {
	s.results = nil
	s.resultIndex = 0
	s.searched = true

	if s.query == "" {
		return
	}

	queryLower := strings.ToLower(s.query)
	allEntities := app.graph.AllNodes()

	for _, entity := range allEntities {
		// Search in ID
		if strings.Contains(strings.ToLower(entity.ID), queryLower) {
			s.results = append(s.results, entity)
			continue
		}

		// Search in title
		if strings.Contains(strings.ToLower(entity.Title()), queryLower) {
			s.results = append(s.results, entity)
			continue
		}

		// Search in description
		if strings.Contains(strings.ToLower(entity.Description()), queryLower) {
			s.results = append(s.results, entity)
			continue
		}

		// Search in content
		if strings.Contains(strings.ToLower(entity.Content), queryLower) {
			s.results = append(s.results, entity)
			continue
		}
	}

	// Limit results
	if len(s.results) > 50 {
		s.results = s.results[:50]
	}
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
		Width(50)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	idStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	sb.WriteString(titleStyle.Render("Search"))
	sb.WriteString("\n\n")

	// Search input
	cursor := "_"
	displayText := s.query[:s.cursorPos] + cursor + s.query[s.cursorPos:]
	sb.WriteString(inputStyle.Render(displayText))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Type to search, Enter to search/select, Ctrl+U to clear"))
	sb.WriteString("\n\n")

	// Results
	if s.searched {
		if len(s.results) == 0 {
			sb.WriteString(labelStyle.Render("No results found"))
		} else {
			sb.WriteString(labelStyle.Render(fmt.Sprintf("Found %d results:", len(s.results))))
			sb.WriteString("\n\n")

			// Show results with scrolling
			visibleCount := height - 10
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
	}

	return sb.String()
}

// Help returns help items
func (s *SearchModel) Help() [][2]string {
	if s.searched && len(s.results) > 0 {
		return [][2]string{
			{"↑/↓", "navigate"},
			{"enter", "open"},
			{"esc", "back"},
		}
	}
	return [][2]string{
		{"enter", "search"},
		{"esc", "back"},
		{"ctrl+u", "clear"},
	}
}
