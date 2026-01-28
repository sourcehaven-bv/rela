// Package tui implements the terminal user interface using Bubbletea.
// Core TUI components (browser, detail, search screens) have unit tests.
// Full app integration and lifecycle methods remain untestable without interactive terminal.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// Screen represents different screens in the TUI
type Screen int

const (
	ScreenBrowser Screen = iota
	ScreenDetail
	ScreenCreate
	ScreenSearch
	ScreenLink
	ScreenGraph
	ScreenAnalysis
	ScreenMetamodel
	ScreenTemplates
	ScreenHelp
	ScreenInit
	ScreenExport
)

// App holds the main application state
type App struct {
	width      int
	height     int
	project    *project.Context
	metamodel  *metamodel.Metamodel
	graph      *graph.Graph
	screen     Screen
	message    string
	messageErr bool

	// Screen-specific models
	browser   *BrowserModel
	detail    *DetailModel
	create    *CreateModel
	search    *SearchModel
	link      *LinkModel
	graphView *GraphViewModel
	analysis  *AnalysisModel
	meta      *MetamodelModel
	templates *TemplatesModel
	help      *HelpModel
	initModel *InitModel
	export    *ExportModel

	// Navigation stack
	screenStack []Screen
}

// NewApp creates a new TUI application
// coverage-ignore: TUI initialization - unreasonable to unit test interactive terminal UI
func NewApp(ctx *project.Context, meta *metamodel.Metamodel, g *graph.Graph) *App {
	app := &App{
		project:     ctx,
		metamodel:   meta,
		graph:       g,
		screen:      ScreenBrowser,
		screenStack: []Screen{ScreenBrowser},
	}

	// Initialize screen models
	app.browser = NewBrowserModel(app)
	app.help = NewHelpModel()

	return app
}

// Run starts the TUI
// coverage-ignore: TUI entry point - unreasonable to unit test interactive terminal UI
func Run(ctx *project.Context, meta *metamodel.Metamodel, g *graph.Graph) error {
	app := NewApp(ctx, meta, g)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunInit starts the TUI in initialization mode (no existing project)
// coverage-ignore: TUI entry point - unreasonable to unit test interactive terminal UI
func RunInit(projectDir string) error {
	app := &App{
		screen:      ScreenInit,
		screenStack: []Screen{ScreenInit},
	}
	app.initModel = NewInitModel(projectDir)
	app.help = NewHelpModel()

	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init implements tea.Model
// coverage-ignore: Bubbletea lifecycle method - unreasonable to unit test interactive terminal UI
func (a *App) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
// coverage-ignore: Bubbletea lifecycle method - unreasonable to unit test interactive terminal UI
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyMsg:
		// Clear message on any key
		a.message = ""
		a.messageErr = false

		// Global keys
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if len(a.screenStack) > 1 {
				return a, a.popScreen()
			}
			return a, tea.Quit
		case "esc":
			if len(a.screenStack) > 1 {
				return a, a.popScreen()
			}
		case "?":
			if a.screen != ScreenHelp {
				return a, a.pushScreen(ScreenHelp)
			}
		case "/":
			if a.screen != ScreenSearch {
				return a, a.pushScreen(ScreenSearch)
			}
		}

		// Delegate to current screen
		return a.handleScreenKey(msg)

	case screenPushMsg:
		return a, a.pushScreen(msg.screen)

	case screenPopMsg:
		return a, a.popScreen()

	case setMessageMsg:
		a.message = msg.message
		a.messageErr = msg.isError
		return a, nil

	case editorFinishedMsg:
		if msg.err != nil {
			a.message = "Editor error: " + msg.err.Error()
			a.messageErr = true
			return a, nil
		}
		// Reload after edit
		if a.detail != nil {
			if msg.isRelation {
				// Find the relation that was edited
				if msg.relIndex < len(a.detail.allRels) {
					rel := a.detail.allRels[msg.relIndex]
					if err := a.detail.reloadRelation(a, rel); err != nil {
						a.message = "Failed to reload: " + err.Error()
						a.messageErr = true
					} else {
						a.message = "Relation updated"
						a.messageErr = false
					}
				}
			} else {
				if err := a.detail.reloadEntity(a); err != nil {
					a.message = "Failed to reload: " + err.Error()
					a.messageErr = true
				} else {
					a.message = "Entity updated"
					a.messageErr = false
				}
			}
		}
		return a, nil

	case metamodelEditorFinishedMsg:
		if msg.err != nil {
			a.message = "Editor error: " + msg.err.Error()
			a.messageErr = true
			return a, nil
		}
		// Reload metamodel after edit
		if err := a.reloadMetamodel(); err != nil {
			a.message = "Failed to reload metamodel: " + err.Error()
			a.messageErr = true
		} else {
			a.message = "Metamodel updated"
			a.messageErr = false
		}
		return a, nil
	}

	return a, nil
}

// View implements tea.Model
// coverage-ignore: Bubbletea lifecycle method - unreasonable to unit test interactive terminal UI
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	// Layout: header (3 lines) + content + footer (3 lines)
	headerHeight := 3
	footerHeight := 3
	contentHeight := a.height - headerHeight - footerHeight

	header := a.renderHeader()
	content := a.renderContent(contentHeight)
	footer := a.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (a *App) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Padding(0, 1)

	screenStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 1)

	projectStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Padding(0, 1)

	title := titleStyle.Render("RELA")
	screenTitle := screenStyle.Render(a.screenTitle())
	projectRoot := ""
	if a.project != nil {
		projectRoot = a.project.Root
	}
	projectName := projectStyle.Render(projectRoot)

	// Calculate spacing
	leftPart := title + " │ " + screenTitle
	rightPart := projectName
	padding := a.width - lipgloss.Width(leftPart) - lipgloss.Width(rightPart)
	if padding < 0 {
		padding = 0
	}

	headerLine := leftPart + lipgloss.NewStyle().Width(padding).Render("") + rightPart

	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("240")).
		Width(a.width)

	return border.Render(headerLine)
}

func (a *App) renderContent(height int) string {
	var content string
	switch a.screen {
	case ScreenBrowser:
		content = a.browser.View(a.width, height)
	case ScreenDetail:
		if a.detail != nil {
			content = a.detail.View(a.width, height)
		}
	case ScreenCreate:
		if a.create != nil {
			content = a.create.View(a.width, height)
		}
	case ScreenSearch:
		if a.search != nil {
			content = a.search.View(a.width, height)
		}
	case ScreenLink:
		if a.link != nil {
			content = a.link.View(a.width, height)
		}
	case ScreenGraph:
		if a.graphView != nil {
			content = a.graphView.View(a.width, height)
		}
	case ScreenAnalysis:
		if a.analysis != nil {
			content = a.analysis.View(a.width, height)
		}
	case ScreenMetamodel:
		if a.meta != nil {
			content = a.meta.View(a.width, height)
		}
	case ScreenTemplates:
		if a.templates != nil {
			content = a.templates.View(a.width, height)
		}
	case ScreenHelp:
		content = a.help.View(a.width, height)
	case ScreenExport:
		if a.export != nil {
			content = a.export.View(a.width, height)
		}
	case ScreenInit:
		if a.initModel != nil {
			content = a.initModel.View(a.width, height)
		}
	default:
		content = "Unknown screen"
	}

	// Pad content to fill the height (ensures screen is cleared)
	lines := strings.Split(content, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	// Truncate if too long
	if len(lines) > height {
		lines = lines[:height]
	}

	// Ensure each line is the full width
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < a.width {
			lines[i] = line + strings.Repeat(" ", a.width-lineWidth)
		}
	}

	return strings.Join(lines, "\n")
}

func (a *App) renderFooter() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1)

	messageStyle := lipgloss.NewStyle().
		Padding(0, 1)

	if a.messageErr {
		messageStyle = messageStyle.Foreground(lipgloss.Color("196"))
	} else {
		messageStyle = messageStyle.Foreground(lipgloss.Color("82"))
	}

	// Get help from current screen
	helpItems := a.screenHelp()
	helpText := ""
	for _, item := range helpItems {
		helpText += fmt.Sprintf("[%s] %s  ", item[0], item[1])
	}

	// Always add quit
	helpText += "[q] quit  [?] help"

	// Build footer
	help := helpStyle.Render(helpText)
	msg := ""
	if a.message != "" {
		msg = messageStyle.Render(a.message)
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("240")).
		Width(a.width)

	footer := help
	if msg != "" {
		padding := a.width - lipgloss.Width(help) - lipgloss.Width(msg) - 2
		if padding > 0 {
			footer = help + lipgloss.NewStyle().Width(padding).Render("") + msg
		}
	}

	return border.Render(footer)
}

func (a *App) screenTitle() string {
	switch a.screen {
	case ScreenBrowser:
		return "Browser"
	case ScreenDetail:
		return "Entity Detail"
	case ScreenCreate:
		return "Create Entity"
	case ScreenSearch:
		return "Search"
	case ScreenLink:
		return "Create Link"
	case ScreenGraph:
		return "Relationship Graph"
	case ScreenAnalysis:
		return "Analysis"
	case ScreenMetamodel:
		return "Metamodel"
	case ScreenTemplates:
		return "Templates"
	case ScreenHelp:
		return "Help"
	case ScreenExport:
		return "Export"
	case ScreenInit:
		return "Initialize Project"
	}
	return ""
}

func (a *App) screenHelp() [][2]string {
	switch a.screen {
	case ScreenBrowser:
		return a.browser.Help()
	case ScreenDetail:
		if a.detail != nil {
			return a.detail.Help()
		}
	case ScreenCreate:
		if a.create != nil {
			return a.create.Help()
		}
	case ScreenSearch:
		if a.search != nil {
			return a.search.Help()
		}
	case ScreenTemplates:
		if a.templates != nil {
			return a.templates.Help()
		}
	case ScreenMetamodel:
		if a.meta != nil {
			return a.meta.Help()
		}
	case ScreenHelp:
		return a.help.Help()
	case ScreenExport:
		if a.export != nil {
			return a.export.Help()
		}
	case ScreenInit:
		if a.initModel != nil {
			return a.initModel.Help()
		}
	}
	return nil
}

func (a *App) handleScreenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.screen {
	case ScreenBrowser:
		return a.browser.Update(a, msg)
	case ScreenDetail:
		if a.detail != nil {
			return a.detail.Update(a, msg)
		}
	case ScreenCreate:
		if a.create != nil {
			return a.create.Update(a, msg)
		}
	case ScreenSearch:
		if a.search != nil {
			return a.search.Update(a, msg)
		}
	case ScreenLink:
		if a.link != nil {
			return a.link.Update(a, msg)
		}
	case ScreenGraph:
		if a.graphView != nil {
			return a.graphView.Update(a, msg)
		}
	case ScreenAnalysis:
		if a.analysis != nil {
			return a.analysis.Update(a, msg)
		}
	case ScreenMetamodel:
		if a.meta != nil {
			return a.meta.Update(a, msg)
		}
	case ScreenTemplates:
		if a.templates != nil {
			return a.templates.Update(a, msg)
		}
	case ScreenHelp:
		return a.help.Update(a, msg)
	case ScreenExport:
		if a.export != nil {
			return a.export.Update(a, msg)
		}
	case ScreenInit:
		if a.initModel != nil {
			return a.initModel.Update(a, msg)
		}
	}
	return a, nil
}

// Navigation commands
type screenPushMsg struct {
	screen Screen
}

type screenPopMsg struct{}

type setMessageMsg struct {
	message string
	isError bool
}

func (a *App) pushScreen(screen Screen) tea.Cmd {
	// Initialize screen synchronously before pushing
	a.screenStack = append(a.screenStack, screen)
	a.screen = screen

	// Initialize screen-specific models
	switch screen {
	case ScreenSearch:
		a.search = NewSearchModel(a)
	case ScreenCreate:
		a.create = NewCreateModel(a)
	case ScreenAnalysis:
		a.analysis = NewAnalysisModel(a)
	case ScreenMetamodel:
		a.meta = NewMetamodelModel(a)
	case ScreenTemplates:
		a.templates = NewTemplatesModel(a)
	}

	return nil
}

func (a *App) popScreen() tea.Cmd {
	// Pop synchronously
	if len(a.screenStack) > 1 {
		a.screenStack = a.screenStack[:len(a.screenStack)-1]
		a.screen = a.screenStack[len(a.screenStack)-1]
	}
	return nil
}

// SetMessage sets a status message
func SetMessage(msg string, isError bool) tea.Cmd {
	return func() tea.Msg {
		return setMessageMsg{message: msg, isError: isError}
	}
}

// PushScreen pushes a new screen
func PushScreen(screen Screen) tea.Cmd {
	return func() tea.Msg {
		return screenPushMsg{screen: screen}
	}
}

// PopScreen pops the current screen
func PopScreen() tea.Cmd {
	return func() tea.Msg {
		return screenPopMsg{}
	}
}

// reloadMetamodel reloads the metamodel from disk and updates the screen
func (a *App) reloadMetamodel() error {
	meta, err := metamodel.Load(a.project.MetamodelPath)
	if err != nil {
		return err
	}

	// Update metamodel
	a.metamodel = meta

	// Refresh metamodel screen if active
	if a.meta != nil {
		a.meta.load(a)
	}

	return nil
}

// reloadFromDisk reloads all entities and relations from disk
func (a *App) reloadFromDisk() error {
	// Reload metamodel first
	if err := a.reloadMetamodel(); err != nil {
		return err
	}

	// Sync graph from files
	_, err := markdown.SyncFromFiles(a.project, a.metamodel, a.graph)
	if err != nil {
		return err
	}

	// Save updated cache
	_ = a.graph.SaveCache(a.project.CachePath)

	// Refresh browser
	if a.browser != nil {
		a.browser.Refresh(a)
	}

	return nil
}
