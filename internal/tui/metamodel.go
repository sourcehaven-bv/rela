package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MetamodelTab represents the active tab
type MetamodelTab int

const (
	MetamodelTabEntities MetamodelTab = iota
	MetamodelTabRelations
)

// MetamodelModel shows metamodel configuration
type MetamodelModel struct {
	tab           MetamodelTab
	entityTypes   []entityTypeInfo
	relations     []relationInfo
	entityIndex   int
	relationIndex int
}

type entityTypeInfo struct {
	name       string
	label      string
	idPatterns []string
	aliases    []string
	properties []string
}

type relationInfo struct {
	name        string
	label       string
	from        []string
	to          []string
	inverse     string
	description string
}

// NewMetamodelModel creates a new metamodel screen
func NewMetamodelModel(app *App) *MetamodelModel {
	m := &MetamodelModel{
		tab: MetamodelTabEntities,
	}
	m.load(app)
	return m
}

func (m *MetamodelModel) load(app *App) {
	// Load entity types
	m.entityTypes = nil
	for name, def := range app.metamodel.Entities {
		props := make([]string, 0)
		for propName, propDef := range def.Properties {
			required := ""
			if propDef.Required {
				required = "*"
			}
			props = append(props, fmt.Sprintf("%s%s: %s", propName, required, propDef.Type))
		}
		sort.Strings(props)

		m.entityTypes = append(m.entityTypes, entityTypeInfo{
			name:       name,
			label:      def.Label,
			idPatterns: def.IDPatterns,
			aliases:    def.Aliases,
			properties: props,
		})
	}
	sort.Slice(m.entityTypes, func(i, j int) bool {
		return m.entityTypes[i].label < m.entityTypes[j].label
	})

	// Load relations
	m.relations = nil
	for name, def := range app.metamodel.Relations {
		inverse := ""
		if def.Inverse != nil {
			inverse = def.Inverse.GetID()
		}

		m.relations = append(m.relations, relationInfo{
			name:        name,
			label:       def.Label,
			from:        def.From,
			to:          def.To,
			inverse:     inverse,
			description: def.Description,
		})
	}
	sort.Slice(m.relations, func(i, j int) bool {
		return m.relations[i].label < m.relations[j].label
	})
}

// metamodelEditorFinishedMsg is sent when the metamodel editor closes
type metamodelEditorFinishedMsg struct {
	err error
}

// Update handles key events
func (m *MetamodelModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "r":
		if m.tab == MetamodelTabEntities {
			m.tab = MetamodelTabRelations
		} else {
			m.tab = MetamodelTabEntities
		}
	case "e":
		// Edit metamodel.yaml
		return app, m.editMetamodel(app)
	case "j", "down":
		if m.tab == MetamodelTabEntities {
			if m.entityIndex < len(m.entityTypes)-1 {
				m.entityIndex++
			}
		} else {
			if m.relationIndex < len(m.relations)-1 {
				m.relationIndex++
			}
		}
	case "k", "up":
		if m.tab == MetamodelTabEntities {
			if m.entityIndex > 0 {
				m.entityIndex--
			}
		} else {
			if m.relationIndex > 0 {
				m.relationIndex--
			}
		}
	}
	return app, nil
}

// View renders the metamodel screen
func (m *MetamodelModel) View(_, _ int) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	tabActiveStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Padding(0, 2).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("205"))

	tabInactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	sb.WriteString(titleStyle.Render("Metamodel"))
	sb.WriteString("\n\n")

	// Tabs
	entitiesTab := tabInactiveStyle
	relationsTab := tabInactiveStyle
	if m.tab == MetamodelTabEntities {
		entitiesTab = tabActiveStyle
	} else {
		relationsTab = tabActiveStyle
	}

	tabs := entitiesTab.Render("Entities") + "  " + relationsTab.Render("Relations")
	sb.WriteString(tabs)
	sb.WriteString("\n\n")

	switch m.tab {
	case MetamodelTabEntities:
		for i, et := range m.entityTypes {
			marker := "  "
			style := normalStyle
			if i == m.entityIndex {
				marker = "► "
				style = selectedStyle
			}

			sb.WriteString(fmt.Sprintf("%s%s\n", marker, style.Render(et.label)))

			// Show details if selected
			if i == m.entityIndex {
				sb.WriteString(fmt.Sprintf("    %s: %s\n",
					labelStyle.Render("ID Patterns"),
					detailStyle.Render(strings.Join(et.idPatterns, ", "))))

				if len(et.aliases) > 0 {
					sb.WriteString(fmt.Sprintf("    %s: %s\n",
						labelStyle.Render("Aliases"),
						detailStyle.Render(strings.Join(et.aliases, ", "))))
				}

				if len(et.properties) > 0 {
					sb.WriteString(fmt.Sprintf("    %s:\n", labelStyle.Render("Properties")))
					for _, prop := range et.properties {
						sb.WriteString(fmt.Sprintf("      %s\n", detailStyle.Render(prop)))
					}
				}
				sb.WriteString("\n")
			}
		}

	case MetamodelTabRelations:
		for i, rel := range m.relations {
			marker := "  "
			style := normalStyle
			if i == m.relationIndex {
				marker = "► "
				style = selectedStyle
			}

			sb.WriteString(fmt.Sprintf("%s%s\n", marker, style.Render(rel.label)))

			// Show details if selected
			if i == m.relationIndex {
				sb.WriteString(fmt.Sprintf("    %s: [%s] → [%s]\n",
					labelStyle.Render("Types"),
					detailStyle.Render(strings.Join(rel.from, ", ")),
					detailStyle.Render(strings.Join(rel.to, ", "))))

				if rel.inverse != "" {
					sb.WriteString(fmt.Sprintf("    %s: %s\n",
						labelStyle.Render("Inverse"),
						detailStyle.Render(rel.inverse)))
				}

				if rel.description != "" {
					sb.WriteString(fmt.Sprintf("    %s: %s\n",
						labelStyle.Render("Description"),
						detailStyle.Render(rel.description)))
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// Help returns help items
func (m *MetamodelModel) Help() [][2]string {
	return [][2]string{
		{"tab", "switch"},
		{"↑/↓", "navigate"},
		{"e", "edit"},
		{"esc", "back"},
	}
}

// editMetamodel launches the editor for metamodel.yaml
func (m *MetamodelModel) editMetamodel(app *App) tea.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command(editor, app.project.MetamodelPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return metamodelEditorFinishedMsg{err: err}
	})
}
