package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// InitModel handles project initialization
type InitModel struct {
	step        int // 0 = confirm, 1 = name input, 2 = description input
	projectName string
	description string
	cursorPos   int
	projectDir  string
	errorMsg    string
}

// NewInitModel creates a new init screen
func NewInitModel(projectDir string) *InitModel {
	// Default project name from directory
	defaultName := filepath.Base(projectDir)
	return &InitModel{
		step:        0,
		projectName: defaultName,
		projectDir:  projectDir,
	}
}

// Update handles key events
func (m *InitModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errorMsg = ""

	switch m.step {
	case 0:
		return m.updateConfirm(app, msg)
	case 1:
		return m.updateNameInput(app, msg)
	case 2:
		return m.updateDescInput(app, msg)
	}
	return app, nil
}

func (m *InitModel) updateConfirm(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.step = 1
		m.cursorPos = len(m.projectName)
	case "n", "N":
		return app, tea.Quit
	}
	return app, nil
}

func (m *InitModel) updateNameInput(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.projectName != "" {
			m.step = 2
			m.cursorPos = 0
		}
	case "esc":
		m.step = 0
	case "backspace":
		if m.cursorPos > 0 && m.projectName != "" {
			m.projectName = m.projectName[:m.cursorPos-1] + m.projectName[m.cursorPos:]
			m.cursorPos--
		}
	case "left":
		if m.cursorPos > 0 {
			m.cursorPos--
		}
	case "right":
		if m.cursorPos < len(m.projectName) {
			m.cursorPos++
		}
	default:
		// Insert character - use KeyRunes type for proper character detection
		// This handles both regular keyboard input and automated testing (e.g., tmux send-keys)
		if msg.Type == tea.KeyRunes {
			chars := string(msg.Runes)
			m.projectName = m.projectName[:m.cursorPos] + chars + m.projectName[m.cursorPos:]
			m.cursorPos += len(chars)
		} else if msg.Type == tea.KeySpace {
			m.projectName = m.projectName[:m.cursorPos] + " " + m.projectName[m.cursorPos:]
			m.cursorPos++
		}
	}
	return app, nil
}

func (m *InitModel) updateDescInput(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Initialize the project
		return m.initProject(app)
	case "esc":
		m.step = 1
		m.cursorPos = len(m.projectName)
	case "backspace":
		if m.cursorPos > 0 && m.description != "" {
			m.description = m.description[:m.cursorPos-1] + m.description[m.cursorPos:]
			m.cursorPos--
		}
	case "left":
		if m.cursorPos > 0 {
			m.cursorPos--
		}
	case "right":
		if m.cursorPos < len(m.description) {
			m.cursorPos++
		}
	default:
		// Insert character - use KeyRunes type for proper character detection
		// This handles both regular keyboard input and automated testing (e.g., tmux send-keys)
		if msg.Type == tea.KeyRunes {
			chars := string(msg.Runes)
			m.description = m.description[:m.cursorPos] + chars + m.description[m.cursorPos:]
			m.cursorPos += len(chars)
		} else if msg.Type == tea.KeySpace {
			m.description = m.description[:m.cursorPos] + " " + m.description[m.cursorPos:]
			m.cursorPos++
		}
	}
	return app, nil
}

func (m *InitModel) initProject(app *App) (tea.Model, tea.Cmd) {
	metamodelPath := filepath.Join(m.projectDir, project.MetamodelFile)

	// Create project context
	ctx := &project.Context{
		Root:          m.projectDir,
		MetamodelPath: metamodelPath,
		CacheDir:      filepath.Join(m.projectDir, project.CacheDir),
		CachePath:     filepath.Join(m.projectDir, project.CacheDir, project.CacheFile),
		EntitiesDir:   filepath.Join(m.projectDir, project.EntitiesDir),
		RelationsDir:  filepath.Join(m.projectDir, project.RelationsDir),
	}

	// Create directories
	if err := ctx.Initialize(); err != nil {
		m.errorMsg = fmt.Sprintf("Failed to create directories: %v", err)
		return app, nil
	}

	// Write default metamodel
	if err := os.WriteFile(metamodelPath, []byte(metamodel.DefaultMetamodelYAML()), 0644); err != nil {
		m.errorMsg = fmt.Sprintf("Failed to write metamodel: %v", err)
		return app, nil
	}

	// Add .rela to .gitignore if it exists
	gitignorePath := filepath.Join(m.projectDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		content, err := os.ReadFile(gitignorePath)
		if err == nil && !strings.Contains(string(content), ".rela") {
			f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				_, _ = f.WriteString("\n# rela cache\n.rela/\n")
				f.Close()
			}
		}
	}

	// Load the metamodel we just created
	meta, err := metamodel.Load(metamodelPath)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Failed to load metamodel: %v", err)
		return app, nil
	}

	// Initialize graph
	g := graph.New()

	// Sync from files (will be empty but sets up properly)
	if _, err := markdown.SyncFromFiles(ctx, meta, g); err != nil {
		m.errorMsg = fmt.Sprintf("Failed to sync: %v", err)
		return app, nil
	}

	// Update app with the new project context
	app.project = ctx
	app.metamodel = meta
	app.graph = g

	// Initialize browser and switch to it
	app.browser = NewBrowserModel(app)
	app.screen = ScreenBrowser
	app.screenStack = []Screen{ScreenBrowser}

	return app, SetMessage(fmt.Sprintf("Initialized project in %s", m.projectDir), false)
}

// View renders the init screen
func (m *InitModel) View(width, _ int) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(min(50, width-4))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	sb.WriteString(titleStyle.Render("Initialize New Project"))
	sb.WriteString("\n\n")

	if m.errorMsg != "" {
		sb.WriteString(errorStyle.Render(m.errorMsg))
		sb.WriteString("\n\n")
	}

	switch m.step {
	case 0:
		sb.WriteString(subtitleStyle.Render("No rela project found in this directory."))
		sb.WriteString("\n\n")
		sb.WriteString(labelStyle.Render("Directory: "))
		sb.WriteString(m.projectDir)
		sb.WriteString("\n\n")
		sb.WriteString("Would you like to initialize a new project? ")
		sb.WriteString(lipgloss.NewStyle().Bold(true).Render("[Y/n]"))

	case 1:
		sb.WriteString(labelStyle.Render("Project Name:"))
		sb.WriteString("\n")
		cursor := "_"
		displayText := m.projectName[:m.cursorPos] + cursor + m.projectName[m.cursorPos:]
		sb.WriteString(inputStyle.Render(displayText))
		sb.WriteString("\n\n")
		sb.WriteString(labelStyle.Render("Press Enter to continue, Esc to go back"))

	case 2:
		sb.WriteString(labelStyle.Render("Project Name: "))
		sb.WriteString(m.projectName)
		sb.WriteString("\n\n")
		sb.WriteString(labelStyle.Render("Description (optional):"))
		sb.WriteString("\n")
		cursor := "_"
		displayText := m.description[:m.cursorPos] + cursor + m.description[m.cursorPos:]
		sb.WriteString(inputStyle.Render(displayText))
		sb.WriteString("\n\n")
		sb.WriteString(labelStyle.Render("Press Enter to create project, Esc to go back"))
	}

	return sb.String()
}

// Help returns help items
func (m *InitModel) Help() [][2]string {
	switch m.step {
	case 0:
		return [][2]string{
			{"y", "yes"},
			{"n", "no"},
		}
	case 1, 2:
		return [][2]string{
			{"enter", "continue"},
			{"esc", "back"},
		}
	}
	return nil
}
