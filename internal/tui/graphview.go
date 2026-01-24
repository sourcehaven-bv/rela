package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/graph"
)

// GraphViewModel shows relationship graph from a root entity
type GraphViewModel struct {
	rootID   string
	depth    int
	maxDepth int
	tree     *graph.TraceResult
	flatList []flatNode
	selected int
}

type flatNode struct {
	id       string
	title    string
	nodeType string
	relation string
	depth    int
	isBack   bool
	incoming bool // True if this node is connected via an incoming relation
}

// NewGraphViewModel creates a new graph view
func NewGraphViewModel(app *App, rootID string) *GraphViewModel {
	g := &GraphViewModel{
		rootID:   rootID,
		depth:    2,
		maxDepth: 5,
	}
	g.load(app)
	return g
}

func (g *GraphViewModel) load(app *App) {
	g.tree = app.graph.TraceBoth(g.rootID, g.depth)
	g.flatList = nil
	g.flattenTree(g.tree, 0, make(map[string]bool))
}

func (g *GraphViewModel) flattenTree(node *graph.TraceResult, depth int, visited map[string]bool) {
	if node == nil {
		return
	}

	isBack := visited[node.ID]
	g.flatList = append(g.flatList, flatNode{
		id:       node.ID,
		title:    node.Title,
		nodeType: node.Type,
		relation: node.Relation,
		depth:    depth,
		isBack:   isBack,
		incoming: node.Incoming,
	})

	if !isBack {
		visited[node.ID] = true
		for _, child := range node.Children {
			g.flattenTree(child, depth+1, visited)
		}
	}
}

// Update handles key events
func (g *GraphViewModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if g.selected < len(g.flatList)-1 {
			g.selected++
		}
	case "k", "up":
		if g.selected > 0 {
			g.selected--
		}
	case "+", "=":
		if g.depth < g.maxDepth {
			g.depth++
			g.load(app)
		}
	case "-", "_":
		if g.depth > 1 {
			g.depth--
			g.load(app)
		}
	case "enter":
		if g.selected < len(g.flatList) {
			node := g.flatList[g.selected]
			if !node.isBack {
				// Create new graph view for selected node
				app.graphView = NewGraphViewModel(app, node.id)
				return app, app.pushScreen(ScreenGraph)
			}
		}
	case "d":
		// Open detail view
		if g.selected < len(g.flatList) {
			node := g.flatList[g.selected]
			app.detail = NewDetailModel(app, node.id)
			return app, app.pushScreen(ScreenDetail)
		}
	case "x":
		// Export graph
		app.export = NewExportModelFromGraph(g.rootID, g.depth)
		return app, app.pushScreen(ScreenExport)
	}
	return app, nil
}

// View renders the graph
func (g *GraphViewModel) View(_, height int) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	idStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	backStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	relStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("220"))

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Relationship Graph: %s", g.rootID)))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render(fmt.Sprintf("Depth: %d (use +/- to change)", g.depth)))
	sb.WriteString("\n\n")

	if len(g.flatList) == 0 {
		sb.WriteString(labelStyle.Render("No relationships found"))
		return sb.String()
	}

	// Calculate visible range
	visibleCount := height - 6
	if visibleCount < 5 {
		visibleCount = 5
	}

	startIdx := 0
	if g.selected >= visibleCount {
		startIdx = g.selected - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(g.flatList) {
		endIdx = len(g.flatList)
	}

	for i := startIdx; i < endIdx; i++ {
		node := g.flatList[i]

		// Build indentation
		indent := strings.Repeat("  ", node.depth)

		// Tree connector
		connector := ""
		if node.depth > 0 {
			connector = "├─ "
		}

		// Selection marker
		marker := "  "
		style := normalStyle
		if i == g.selected {
			marker = "► "
			style = selectedStyle
		}

		// Relation label with direction indicator
		relLabel := ""
		if node.relation != "" {
			direction := "->"
			if node.incoming {
				direction = "<-"
			}
			relLabel = relStyle.Render(fmt.Sprintf("[%s %s] ", direction, node.relation))
		}

		// Node ID and title
		id := idStyle.Render(node.id)
		if i == g.selected {
			id = style.Render(node.id)
		}

		title := node.title
		if len(title) > 35 {
			title = title[:32] + "..."
		}

		// Back reference indicator
		backIndicator := ""
		if node.isBack {
			backIndicator = backStyle.Render(" [↑ back]")
			style = backStyle
		}

		line := fmt.Sprintf("%s%s%s%s%s %s%s",
			marker,
			indent,
			connector,
			relLabel,
			id,
			style.Render(title),
			backIndicator)

		sb.WriteString(line + "\n")
	}

	if len(g.flatList) > visibleCount {
		sb.WriteString(labelStyle.Render(fmt.Sprintf("\n[%d/%d]", g.selected+1, len(g.flatList))))
	}

	return sb.String()
}

// Help returns help items
func (g *GraphViewModel) Help() [][2]string {
	return [][2]string{
		{"↑/↓", "navigate"},
		{"+/-", "depth"},
		{"enter", "focus"},
		{"d", "detail"},
		{"x", "export"},
	}
}
