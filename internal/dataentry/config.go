// Package dataentry provides a config-driven data entry web application
// built on top of rela's metamodel system. It reads a data-entry.yaml config
// file alongside a rela project and serves an interactive UI for CRUD operations
// on entities stored as markdown files.
package dataentry

import (
	"github.com/Sourcehaven-BV/rela/internal/git"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Config is the top-level configuration for a data entry application.
type Config struct {
	Version    string                       `yaml:"version"`
	App        AppConfig                    `yaml:"app"`
	Git        *git.Config                  `yaml:"git,omitempty"`
	Styles     map[string]map[string]string `yaml:"styles"`
	Forms      map[string]Form              `yaml:"forms"`
	Lists      map[string]List              `yaml:"lists"`
	Views      map[string]ViewConfig        `yaml:"views"`
	Kanbans    map[string]Kanban            `yaml:"kanbans"`
	Dashboard  *DashboardConfig             `yaml:"dashboard,omitempty"`
	Commands   map[string]CommandConfig     `yaml:"commands,omitempty"`
	Navigation []NavigationEntry            `yaml:"navigation"`
}

// AppConfig holds display metadata for the application.
type AppConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Form defines a create/edit form for an entity type.
type Form struct {
	EntityType  string           `yaml:"entity_type"`
	Title       string           `yaml:"title"`
	Description string           `yaml:"description"`
	Mode        string           `yaml:"mode"`
	Body        *bool            `yaml:"body,omitempty"`
	Fields      []FormField      `yaml:"fields"`
	Relations   []FormRelation   `yaml:"relations"`
	SidePanel   *SidePanelConfig `yaml:"side_panel,omitempty"`
}

// SidePanelConfig defines an optional context panel shown alongside a form.
// It reuses the view traversal and section display system.
type SidePanelConfig struct {
	Traverse []ViewTraverse `yaml:"traverse"`
	Sections []ViewSection  `yaml:"sections"`
}

// FormField defines a single field in a form.
type FormField struct {
	Property    string              `yaml:"property"`
	Label       string              `yaml:"label"`
	Placeholder string              `yaml:"placeholder"`
	Help        string              `yaml:"help"`
	Required    *bool               `yaml:"required,omitempty"`
	Default     string              `yaml:"default"`
	Hidden      bool                `yaml:"hidden"`
	Widget      string              `yaml:"widget"`
	Transitions map[string][]string `yaml:"transitions,omitempty"`
}

// FormRelation defines a relation field in a form.
type FormRelation struct {
	Relation     string             `yaml:"relation"`
	Direction    string             `yaml:"direction"`
	TargetType   string             `yaml:"target_type"`
	Label        string             `yaml:"label"`
	Required     bool               `yaml:"required"`
	Widget       string             `yaml:"widget"`
	Display      string             `yaml:"display"`
	AllowCreate  bool               `yaml:"allow_create"`
	CreateForm   string             `yaml:"create_form"`
	Properties   []RelationProperty `yaml:"properties"`
	Fields       []ViewSectionField `yaml:"fields"`
	EmptyMessage string             `yaml:"empty_message"`
}

// RelationProperty defines an editable property on a relation.
type RelationProperty struct {
	Property string `yaml:"property"`
	Label    string `yaml:"label"`
	Widget   string `yaml:"widget"`
	Required bool   `yaml:"required"`
}

// List defines a list view for an entity type.
type List struct {
	EntityType     string          `yaml:"entity_type"`
	Title          string          `yaml:"title"`
	Description    string          `yaml:"description"`
	Columns        []ListColumn    `yaml:"columns"`
	Sort           []SortSpec      `yaml:"sort,omitempty"`
	Filters        []FilterConfig  `yaml:"filters"`
	FilterControls []FilterControl `yaml:"filter_controls"`
	CreateForm     string          `yaml:"create_form"`
	EditForm       string          `yaml:"edit_form"`
	DetailView     string          `yaml:"detail_view"`
	PageSize       int             `yaml:"page_size"`
}

// ListColumn defines a column in a list view.
// A column references either a Property (entity property) or a Relation
// (outgoing relation type whose target titles are shown comma-separated).
type ListColumn struct {
	Property string `yaml:"property"`
	Relation string `yaml:"relation"`
	Label    string `yaml:"label"`
	Sortable bool   `yaml:"sortable"`
	Link     bool   `yaml:"link"`
}

// SortSpec defines a single sort criterion for a list or dashboard card.
// This is the data-entry-specific alias matching the YAML config format.
// The migration system converts the legacy single-object format to a list.
type SortSpec = model.SortSpec

// FilterConfig defines a static filter applied to a list.
type FilterConfig struct {
	Property string `yaml:"property"`
	Operator string `yaml:"operator"`
	Value    string `yaml:"value"`
}

// FilterControl defines a user-facing filter control in a list.
type FilterControl struct {
	Property string `yaml:"property"`
	Widget   string `yaml:"widget"`
}

// Kanban defines a kanban board view for an entity type.
type Kanban struct {
	EntityType       string           `yaml:"entity_type"`
	Title            string           `yaml:"title"`
	ColumnProperty   string           `yaml:"column_property"`
	Columns          []KanbanColumn   `yaml:"columns,omitempty"`
	SwimlaneProperty string           `yaml:"swimlane_property,omitempty"`
	Swimlanes        []KanbanSwimlane `yaml:"swimlanes,omitempty"`
	Card             KanbanCard       `yaml:"card"`
	EditForm         string           `yaml:"edit_form,omitempty"`
	CreateForm       string           `yaml:"create_form,omitempty"`
	Filters          []FilterConfig   `yaml:"filters,omitempty"`
	FilterControls   []FilterControl  `yaml:"filter_controls,omitempty"`
}

// KanbanColumn defines a column in the kanban board.
type KanbanColumn struct {
	Value string `yaml:"value"`
	Label string `yaml:"label,omitempty"`
}

// KanbanSwimlane defines a swimlane row in the kanban board.
type KanbanSwimlane struct {
	Value string `yaml:"value"`
	Label string `yaml:"label,omitempty"`
}

// KanbanCard defines how cards are displayed on the board.
type KanbanCard struct {
	Title  string             `yaml:"title"`
	Fields []ViewSectionField `yaml:"fields,omitempty"`
}

// NavigationEntry defines a sidebar navigation item or a group of items.
// It is a union type: either a direct item (Label + List/Dashboard/Graph/Kanban)
// or a group (Group + Items). Nested groups are not supported.
type NavigationEntry struct {
	// Direct item fields
	Label     string `yaml:"label,omitempty"`
	List      string `yaml:"list,omitempty"`
	Dashboard bool   `yaml:"dashboard,omitempty"`
	Graph     bool   `yaml:"graph,omitempty"`
	Kanban    string `yaml:"kanban,omitempty"`

	// Group fields
	Group     string            `yaml:"group,omitempty"`
	Collapsed bool              `yaml:"collapsed,omitempty"`
	Items     []NavigationEntry `yaml:"items,omitempty"`
}

// IsGroup returns true if this entry is a navigation group.
func (n NavigationEntry) IsGroup() bool {
	return n.Group != ""
}

// UIState holds user-specific UI preferences persisted in .rela/ui-state.json.
type UIState struct {
	CollapsedGroups map[string]bool `json:"collapsed_groups"`
}

// UserDefaults holds user-configurable default values for entity creation,
// persisted in .rela/user-defaults.yaml.
type UserDefaults struct {
	Defaults         map[string]string `yaml:"defaults,omitempty"`
	RelationDefaults map[string]string `yaml:"relation_defaults,omitempty"`
	Overrides        []DefaultOverride `yaml:"overrides,omitempty"`
}

// DefaultOverride defines property and relation defaults for specific entity types.
type DefaultOverride struct {
	Types            []string          `yaml:"entity_types"`
	Defaults         map[string]string `yaml:"defaults,omitempty"`
	RelationDefaults map[string]string `yaml:"relation_defaults,omitempty"`
}

// ResolvePropertyDefault returns the best default value for a property on the given entity type.
// It checks overrides first (first matching), then global defaults.
func (ud *UserDefaults) ResolvePropertyDefault(entityType, property string) string {
	if ud == nil {
		return ""
	}
	for _, o := range ud.Overrides {
		for _, t := range o.Types {
			if t == entityType {
				if val, ok := o.Defaults[property]; ok {
					return val
				}
			}
		}
	}
	if val, ok := ud.Defaults[property]; ok {
		return val
	}
	return ""
}

// ResolveRelationDefault returns the best default target for a relation on the given entity type.
// It checks overrides first (first matching), then global relation defaults.
func (ud *UserDefaults) ResolveRelationDefault(entityType, relation string) string {
	if ud == nil {
		return ""
	}
	for _, o := range ud.Overrides {
		for _, t := range o.Types {
			if t == entityType {
				if val, ok := o.RelationDefaults[relation]; ok {
					return val
				}
			}
		}
	}
	if val, ok := ud.RelationDefaults[relation]; ok {
		return val
	}
	return ""
}

// DashboardConfig defines a dashboard page with query-driven cards.
type DashboardConfig struct {
	Title       string          `yaml:"title"`
	Description string          `yaml:"description"`
	Cards       []DashboardCard `yaml:"cards"`
}

// DashboardCard defines a single card on the dashboard, driven by a search query.
type DashboardCard struct {
	Title   string       `yaml:"title"`
	Query   string       `yaml:"query"`
	Display string       `yaml:"display"` // "count", "table", "breakdown"
	GroupBy string       `yaml:"group_by,omitempty"`
	Columns []ListColumn `yaml:"columns,omitempty"`
	Sort    []SortSpec   `yaml:"sort,omitempty"`
	Limit   int          `yaml:"limit,omitempty"`
}

// ViewConfig defines a detailed entity view with traversal and sections.
type ViewConfig struct {
	Title    string         `yaml:"title"`
	Entry    ViewEntry      `yaml:"entry"`
	Traverse []ViewTraverse `yaml:"traverse"`
	Sections []ViewSection  `yaml:"sections"`
}

// ViewEntry specifies the entry entity type for a view.
type ViewEntry struct {
	Type string `yaml:"type"`
}

// ViewTraverse defines a graph traversal rule for collecting related entities.
type ViewTraverse struct {
	From           string `yaml:"from"`
	Follow         string `yaml:"follow,omitempty"`
	FollowIncoming string `yaml:"follow_incoming,omitempty"`
	CollectAs      string `yaml:"collect_as"`
	Recursive      bool   `yaml:"recursive,omitempty"`
	MaxDepth       int    `yaml:"max_depth,omitempty"`
	Where          string `yaml:"where,omitempty"`
}

// ViewSection defines a section within a view.
type ViewSection struct {
	Heading      string             `yaml:"heading,omitempty"`
	Source       string             `yaml:"source"`
	Display      string             `yaml:"display"`
	Fields       []ViewSectionField `yaml:"fields,omitempty"`
	Columns      []ListColumn       `yaml:"columns,omitempty"`
	GroupBy      string             `yaml:"group_by,omitempty"`
	EmptyMessage string             `yaml:"empty_message,omitempty"`
	Link         bool               `yaml:"link,omitempty"`
}

// ViewSectionField defines a field within a view section.
type ViewSectionField struct {
	Property string `yaml:"property"`
	Label    string `yaml:"label,omitempty"`
}

// CommandConfig defines an executable command triggered from the UI.
// Context must be one of: entity, list, view, global.
type CommandConfig struct {
	Label       string            `yaml:"label"`
	Script      string            `yaml:"script"`
	Context     string            `yaml:"context"`
	AvailableOn *CommandScope     `yaml:"available_on,omitempty"`
	Confirm     string            `yaml:"confirm,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	AutoOpen    *bool             `yaml:"auto_open,omitempty"`
}

// CommandScope controls where a command button appears in the UI.
type CommandScope struct {
	Views       []string `yaml:"views,omitempty"`
	Lists       []string `yaml:"lists,omitempty"`
	EntityTypes []string `yaml:"entity_types,omitempty"`
	Dashboard   bool     `yaml:"dashboard,omitempty"`
}
