package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpModel shows help information
type HelpModel struct {
	scrollY int
}

// NewHelpModel creates a new help screen
func NewHelpModel() *HelpModel {
	return &HelpModel{}
}

// Update handles key events
func (h *HelpModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		h.scrollY++
	case "k", "up":
		if h.scrollY > 0 {
			h.scrollY--
		}
	}
	return app, nil
}

// View renders the help screen
func (h *HelpModel) View(_, height int) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("220")).
		MarginTop(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Width(12)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sb.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	sb.WriteString("\n\n")

	sections := []struct {
		title string
		items [][2]string
	}{
		{
			title: "Global",
			items: [][2]string{
				{"q", "Quit / Go back"},
				{"Esc", "Go back"},
				{"?", "Show this help"},
				{"/", "Search"},
				{"Ctrl+C", "Force quit"},
			},
		},
		{
			title: "Navigation",
			items: [][2]string{
				{"j / ↓", "Move down"},
				{"k / ↑", "Move up"},
				{"Enter", "Select / Open"},
				{"Backspace", "Go back"},
				{"g / Home", "Go to top"},
				{"G / End", "Go to bottom"},
			},
		},
		{
			title: "Browser",
			items: [][2]string{
				{"c", "Create new entity"},
				{"l", "Link from selected"},
				{"a / A", "Analysis screen"},
				{"m / M", "Metamodel screen"},
				{"t / T", "Templates screen"},
				{"x", "Export entities"},
			},
		},
		{
			title: "Entity Detail",
			items: [][2]string{
				{"Tab", "Toggle relation navigation"},
				{"Enter", "Follow relation"},
				{"l", "Create link"},
				{"g", "View graph"},
			},
		},
		{
			title: "Graph View",
			items: [][2]string{
				{"+", "Increase depth"},
				{"-", "Decrease depth"},
				{"Enter", "Focus on node"},
				{"d", "Open detail view"},
				{"x", "Export graph (DOT)"},
			},
		},
		{
			title: "Link Wizard",
			items: [][2]string{
				{"Enter", "Confirm / Next step"},
				{"type", "Filter targets"},
				{"Backspace", "Previous step"},
			},
		},
		{
			title: "Search",
			items: [][2]string{
				{"type", "Live search (auto-updates)"},
				{"Enter", "Open selected result"},
				{"Ctrl+U", "Clear search"},
				{"↑/↓", "Navigate results"},
			},
		},
		{
			title: "Search Syntax",
			items: [][2]string{
				{"type:req", "Filter by entity type"},
				{"prop:status=draft", "Property filter (=, !=, <, <=, >, >=)"},
				{"status:published", "Status shortcut"},
				{"\"exact phrase\"", "Exact phrase match"},
				{"word1 word2", "Free text (OR, ranked)"},
			},
		},
		{
			title: "Templates",
			items: [][2]string{
				{"c", "Create new template"},
				{"e / Enter", "Edit template"},
				{"d", "Delete template"},
				{"Backspace", "Go back"},
			},
		},
		{
			title: "Analysis Results",
			items: [][2]string{
				{"x", "Export results"},
				{"Enter/Esc", "Go back"},
			},
		},
		{
			title: "Export",
			items: [][2]string{
				{"Enter", "Select / Confirm"},
				{"Esc", "Cancel / Go back"},
			},
		},
	}

	// Build all lines first - estimate ~4 lines per section
	lines := make([]string, 0, len(sections)*4)
	for _, section := range sections {
		lines = append(lines, sectionStyle.Render(section.title))

		for _, item := range section.items {
			line := "  " + keyStyle.Render(item[0]) + descStyle.Render(item[1])
			lines = append(lines, line)
		}
		lines = append(lines, "") // Empty line between sections
	}

	// Apply scrolling
	visibleCount := height - 6 // Account for title, margins, and help bar
	if visibleCount < 5 {
		visibleCount = 5
	}

	// Clamp scrollY
	maxScroll := len(lines) - visibleCount
	if maxScroll < 0 {
		maxScroll = 0
	}
	if h.scrollY > maxScroll {
		h.scrollY = maxScroll
	}

	endIdx := h.scrollY + visibleCount
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	for i := h.scrollY; i < endIdx; i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}

	// Show scroll indicator if needed
	if len(lines) > visibleCount {
		scrollIndicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(fmt.Sprintf("[%d-%d of %d lines] j/k to scroll", h.scrollY+1, endIdx, len(lines)))
		sb.WriteString("\n")
		sb.WriteString(scrollIndicator)
	}

	return sb.String()
}

// Help returns help items for the help screen
func (h *HelpModel) Help() [][2]string {
	return [][2]string{
		{"↑/↓", "scroll"},
		{"esc/?", "close"},
	}
}
