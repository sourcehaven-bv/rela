// Package dataentryconfig contains the configuration types and validation logic
// for the data-entry web application. This package is separated from the main
// dataentry package so that the CLI can import config/validation without pulling
// in the full web application layer (goldmark, templates, git, etc.).
package dataentryconfig

import (
	"fmt"
	"net/url"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/git"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ConfigFile is the conventional filename for data-entry configuration within a rela project.
const ConfigFile = "data-entry.yaml"

// Widget type constants for form fields.
const (
	WidgetText        = "text"
	WidgetSelect      = "select"
	WidgetMultiSelect = "multi-select"
	WidgetCheckbox    = "checkbox"
	WidgetTextarea    = "textarea"
	WidgetNumber      = "number"
	WidgetDate        = "date"
	WidgetRrule       = "rrule"
	WidgetCards       = "cards" // card-based UI for relations with properties
)

// Direction represents the edge direction for relation columns and form relations.
type Direction string

// Relation direction constants.
const (
	DirectionIncoming Direction = "incoming"
	DirectionOutgoing Direction = "outgoing"
)

// UnmarshalYAML validates the direction value during YAML parsing.
func (d *Direction) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	switch s {
	case "", "outgoing":
		*d = DirectionOutgoing
	case "incoming":
		*d = DirectionIncoming
	default:
		return fmt.Errorf("invalid direction %q (must be 'outgoing' or 'incoming')", s)
	}
	return nil
}

// IsIncoming returns true if the direction is incoming.
func (d Direction) IsIncoming() bool {
	return d == DirectionIncoming
}

// Config is the top-level configuration for a data entry application.
type Config struct {
	Version     string                       `yaml:"version"`
	App         AppConfig                    `yaml:"app"`
	Git         *git.Config                  `yaml:"git,omitempty"`
	Palette     *PaletteConfig               `yaml:"palette,omitempty"`
	Styles      map[string]map[string]string `yaml:"styles"`
	Forms       map[string]Form              `yaml:"forms"`
	Lists       map[string]List              `yaml:"lists"`
	Views       map[string]ViewConfig        `yaml:"views"`
	EntityViews map[string]EntityViewConfig  `yaml:"entity_views,omitempty" json:"entity_views,omitempty"`
	Kanbans     map[string]Kanban            `yaml:"kanbans"`
	Documents   map[string]DocumentConfig    `yaml:"documents,omitempty"`
	Dashboard   *DashboardConfig             `yaml:"dashboard,omitempty"`
	Commands    map[string]CommandConfig     `yaml:"commands,omitempty"`
	Actions     map[string]Action            `yaml:"actions,omitempty"`
	Navigation  []NavigationEntry            `yaml:"navigation"`
}

// EntityViewConfig declares UX bindings for a metamodel entity type.
// detail_view names the canonical view used to display an entity of this type
// — consumed by the SPA when an entity link needs to be rendered (entity-list
// rows, custom-view sections). Missing detail_view falls back to
// /entity/:type/:id.
type EntityViewConfig struct {
	DetailView string `yaml:"detail_view,omitempty" json:"detail_view,omitempty"`
}

// Action defines an operation that can be triggered from the UI.
//
// An action has either a declarative property mutation (Set) or a Lua script
// (Script), but not both. When referenced by a list's Actions field, the
// action is available as a keyboard-driven bulk operation on selected rows.
// When referenced by a navigation entry, it appears as a sidebar button.
type Action struct {
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Script      string            `yaml:"script,omitempty" json:"script,omitempty"`
	Params      map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
	Label       string            `yaml:"label,omitempty" json:"label,omitempty"`
	Key         string            `yaml:"key,omitempty" json:"key,omitempty"`
	Confirm     bool              `yaml:"confirm,omitempty" json:"confirm,omitempty"`
	Set         map[string]string `yaml:"set,omitempty" json:"set,omitempty"`
}

// AppConfig holds display metadata for the application.
type AppConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	// MaxAttachmentBytes optionally overrides the product-wide default
	// per-attachment upload cap (see dataentry.DefaultMaxAttachmentBytes).
	// Zero or unset means use the default. Set this lower for
	// semi-untrusted deployments. The store backends also enforce their
	// own backstop guard independent of this value.
	MaxAttachmentBytes int64 `yaml:"max_attachment_bytes,omitempty" json:"max_attachment_bytes,omitempty"`
}

// Form defines a create/edit form for an entity type.
type Form struct {
	EntityType  string           `yaml:"entity_type" json:"entity"`
	Title       string           `yaml:"title" json:"title"`
	Description string           `yaml:"description" json:"description,omitempty"`
	Mode        string           `yaml:"mode" json:"mode,omitempty"`
	Body        *bool            `yaml:"body,omitempty" json:"body,omitempty"`
	Fields      []FormField      `yaml:"fields" json:"fields"`
	Relations   []FormRelation   `yaml:"relations" json:"relations,omitempty"`
	SidePanel   *SidePanelConfig `yaml:"side_panel,omitempty" json:"side_panel,omitempty"`
}

// SidePanelConfig defines an optional context panel shown alongside a form.
// It reuses the view traversal and section display system.
type SidePanelConfig struct {
	Traverse []ViewTraverse `yaml:"traverse" json:"traverse"`
	Sections []ViewSection  `yaml:"sections" json:"sections"`
}

// FormField defines a single field in a form.
type FormField struct {
	Property    string              `yaml:"property" json:"property"`
	Label       string              `yaml:"label" json:"label,omitempty"`
	Placeholder string              `yaml:"placeholder" json:"placeholder,omitempty"`
	Help        string              `yaml:"help" json:"help,omitempty"`
	Widget      string              `yaml:"widget" json:"widget,omitempty"`
	Required    *bool               `yaml:"required,omitempty" json:"required,omitempty"`
	Default     string              `yaml:"default" json:"default,omitempty"`
	Hidden      bool                `yaml:"hidden" json:"hidden,omitempty"`
	Transitions map[string][]string `yaml:"transitions,omitempty" json:"transitions,omitempty"`
}

// FormRelation defines a relation field in a form.
type FormRelation struct {
	Relation     string             `yaml:"relation" json:"relation"`
	Direction    Direction          `yaml:"direction" json:"direction,omitempty"`
	TargetType   string             `yaml:"target_type" json:"target_type,omitempty"`
	Label        string             `yaml:"label" json:"label,omitempty"`
	Required     bool               `yaml:"required" json:"required,omitempty"`
	Widget       string             `yaml:"widget" json:"widget,omitempty"`
	Display      string             `yaml:"display" json:"display,omitempty"`
	AllowCreate  bool               `yaml:"allow_create" json:"allow_create,omitempty"`
	CreateForm   string             `yaml:"create_form" json:"create_form,omitempty"`
	Properties   []RelationProperty `yaml:"properties" json:"properties,omitempty"`
	Fields       []ViewSectionField `yaml:"fields" json:"fields,omitempty"`
	EmptyMessage string             `yaml:"empty_message" json:"empty_message,omitempty"`
}

// RelationProperty defines an editable property on a relation.
type RelationProperty struct {
	Property string `yaml:"property" json:"property"`
	Label    string `yaml:"label" json:"label,omitempty"`
	Required bool   `yaml:"required" json:"required,omitempty"`
}

// List defines a list view for an entity type.
type List struct {
	EntityType     string          `yaml:"entity_type" json:"entity"`
	Title          string          `yaml:"title" json:"title"`
	Description    string          `yaml:"description" json:"description,omitempty"`
	Columns        []ListColumn    `yaml:"columns" json:"columns"`
	Sort           []SortSpec      `yaml:"sort,omitempty" json:"default_sort,omitempty"`
	Filters        []FilterConfig  `yaml:"filters" json:"filters,omitempty"`
	FilterControls []FilterControl `yaml:"filter_controls" json:"filter_controls,omitempty"`
	CreateForm     string          `yaml:"create_form" json:"create_form,omitempty"`
	EditForm       string          `yaml:"edit_form" json:"edit_form,omitempty"`
	DetailView     string          `yaml:"detail_view" json:"detail_view,omitempty"`
	PageSize       int             `yaml:"page_size" json:"page_size,omitempty"`
	Actions        []string        `yaml:"actions,omitempty" json:"actions,omitempty"`
}

// ListColumn defines a column in a list view.
// A column references either a Property (entity property) or a Relation
// (relation type whose target titles are shown comma-separated).
// For relation columns, Direction controls whether to show outgoing (default)
// or incoming edges. Use "incoming" to display entities that point to the current row.
type ListColumn struct {
	Property  string    `yaml:"property" json:"property,omitempty"`
	Relation  string    `yaml:"relation" json:"relation,omitempty"`
	Direction Direction `yaml:"direction" json:"direction,omitempty"` // "outgoing" (default) or "incoming"
	Label     string    `yaml:"label" json:"label,omitempty"`
	Sortable  bool      `yaml:"sortable" json:"sortable,omitempty"`
	Link      string    `yaml:"link" json:"link,omitempty"`
}

// SortSpec defines a single sort criterion for a list or dashboard card.
// This is the data-entry-specific alias matching the YAML config format.
// The migration system converts the legacy single-object format to a list.
type SortSpec = metamodel.SortSpec

// FilterConfig defines a static filter applied to a list.
type FilterConfig struct {
	Property string `yaml:"property" json:"property"`
	Operator string `yaml:"operator" json:"operator"`
	Value    string `yaml:"value" json:"value"`
}

// FilterControl defines a user-facing filter control in a list.
// Exactly one of Property or Relation must be set:
//   - Property: filter on a scalar property of the entity.
//   - Relation: filter by the target title of an outgoing relation; the
//     relation name must exist in the metamodel.
//
// Label is an optional display label override for the control.
type FilterControl struct {
	Property string `yaml:"property,omitempty" json:"property,omitempty"`
	Relation string `yaml:"relation,omitempty" json:"relation,omitempty"`
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
}

// Key returns the filter key (Relation if set, otherwise Property).
func (fc FilterControl) Key() string {
	if fc.Relation != "" {
		return fc.Relation
	}
	return fc.Property
}

// IsRelation returns true if this filter control filters by relation.
func (fc FilterControl) IsRelation() bool {
	return fc.Relation != ""
}

// QueryParamKey returns the URL query parameter key for this filter control.
func (fc FilterControl) QueryParamKey() string {
	return "filter_" + fc.Key()
}

// CurrentValue returns the current filter value from the given query parameters.
func (fc FilterControl) CurrentValue(query url.Values) string {
	return query.Get(fc.QueryParamKey())
}

// Kanban defines a kanban board view for an entity type.
type Kanban struct {
	EntityType       string           `yaml:"entity_type" json:"entity"`
	Title            string           `yaml:"title" json:"title"`
	ColumnProperty   string           `yaml:"column_property" json:"column_property"`
	Columns          []KanbanColumn   `yaml:"columns,omitempty" json:"columns,omitempty"`
	SwimlaneProperty string           `yaml:"swimlane_property,omitempty" json:"swimlane_property,omitempty"`
	Swimlanes        []KanbanSwimlane `yaml:"swimlanes,omitempty" json:"swimlanes,omitempty"`
	Card             KanbanCard       `yaml:"card" json:"card"`
	EditForm         string           `yaml:"edit_form,omitempty" json:"edit_form,omitempty"`
	CreateForm       string           `yaml:"create_form,omitempty" json:"create_form,omitempty"`
	Filters          []FilterConfig   `yaml:"filters,omitempty" json:"filters,omitempty"`
	FilterControls   []FilterControl  `yaml:"filter_controls,omitempty" json:"filter_controls,omitempty"`
}

// KanbanColumn defines a column in the kanban board.
type KanbanColumn struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
}

// KanbanSwimlane defines a swimlane row in the kanban board.
type KanbanSwimlane struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
}

// KanbanCard defines how cards are displayed on the board.
type KanbanCard struct {
	Title  string             `yaml:"title" json:"title"`
	Fields []ViewSectionField `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// NavigationEntry defines a sidebar navigation item or a group of items.
// It is a union type: either a direct item (Label + List/Dashboard/Kanban)
// or a group (Group + Items). Nested groups are not supported.
type NavigationEntry struct {
	// Direct item fields
	Label     string `yaml:"label,omitempty" json:"label,omitempty"`
	List      string `yaml:"list,omitempty" json:"list,omitempty"`
	Dashboard bool   `yaml:"dashboard,omitempty" json:"dashboard,omitempty"`
	Kanban    string `yaml:"kanban,omitempty" json:"kanban,omitempty"`
	Search    bool   `yaml:"search,omitempty" json:"search,omitempty"`
	Settings  bool   `yaml:"settings,omitempty" json:"settings,omitempty"`
	Action    string `yaml:"action,omitempty" json:"action,omitempty"`

	// Group fields
	Group     string            `yaml:"group,omitempty" json:"group,omitempty"`
	Collapsed bool              `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`
	Items     []NavigationEntry `yaml:"items,omitempty" json:"items,omitempty"`
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
	Title       string          `yaml:"title" json:"title"`
	Description string          `yaml:"description" json:"description,omitempty"`
	Cards       []DashboardCard `yaml:"cards" json:"cards"`
}

// DashboardCard defines a single card on the dashboard, driven by a search query.
type DashboardCard struct {
	Title   string       `yaml:"title" json:"title"`
	Query   string       `yaml:"query" json:"query"`
	Display string       `yaml:"display" json:"display"` // "count", "table", "breakdown"
	GroupBy string       `yaml:"group_by,omitempty" json:"group_by,omitempty"`
	Columns []ListColumn `yaml:"columns,omitempty" json:"columns,omitempty"`
	Sort    []SortSpec   `yaml:"sort,omitempty" json:"sort,omitempty"`
	Limit   int          `yaml:"limit,omitempty" json:"limit,omitempty"`
}

// ViewConfig defines a detailed entity view with traversal and sections.
type ViewConfig struct {
	Title    string         `yaml:"title" json:"title"`
	Entry    ViewEntry      `yaml:"entry" json:"entry"`
	Traverse []ViewTraverse `yaml:"traverse" json:"traverse"`
	Sections []ViewSection  `yaml:"sections" json:"sections"`
}

// ViewEntry specifies the entry entity type for a view.
type ViewEntry struct {
	Type string `yaml:"type" json:"type"`
}

// ViewTraverse defines a graph traversal rule for collecting related entities.
type ViewTraverse struct {
	From           string `yaml:"from" json:"from"`
	Follow         string `yaml:"follow,omitempty" json:"follow,omitempty"`
	FollowIncoming string `yaml:"follow_incoming,omitempty" json:"follow_incoming,omitempty"`
	CollectAs      string `yaml:"collect_as" json:"collect_as"`
	Recursive      bool   `yaml:"recursive,omitempty" json:"recursive,omitempty"`
	MaxDepth       int    `yaml:"max_depth,omitempty" json:"max_depth,omitempty"`
	Where          string `yaml:"where,omitempty" json:"where,omitempty"`
}

// ViewSection defines a section within a view.
type ViewSection struct {
	Heading      string             `yaml:"heading,omitempty" json:"heading,omitempty"`
	Source       string             `yaml:"source" json:"source"`
	Display      string             `yaml:"display" json:"display"`
	Fields       []ViewSectionField `yaml:"fields,omitempty" json:"fields,omitempty"`
	Columns      []ListColumn       `yaml:"columns,omitempty" json:"columns,omitempty"`
	GroupBy      string             `yaml:"group_by,omitempty" json:"group_by,omitempty"`
	EmptyMessage string             `yaml:"empty_message,omitempty" json:"empty_message,omitempty"`
	Link         string             `yaml:"link,omitempty" json:"link,omitempty"`
}

// ViewSectionField defines a field within a view section.
type ViewSectionField struct {
	Property string `yaml:"property" json:"property"`
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
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

// DocumentConfig defines how to render a document from an entry entity.
//
// Exactly one of Command or Script must be set. Command shells out to an
// external process that produces markdown on stdout; Script executes a Lua
// script from scripts/ under the project root and captures its stdout.
// Validated via validateDocuments at config-load time.
type DocumentConfig struct {
	// Title is the display title for the document.
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
	// EntityType specifies which entity types this document applies to.
	// Used by the frontend to filter which documents to show for a given entity,
	// and by the HTTP handler to reject cross-type requests (a doc with
	// entity_type=release cannot render against a ticket entity).
	EntityType string `yaml:"entity_type,omitempty" json:"entity_type,omitempty"`
	// Command is the external render command. Placeholders:
	//   {id}       - entry ID
	//   {id_lower} - lowercase entry ID
	// Mutually exclusive with Script.
	Command string `yaml:"command,omitempty" json:"command,omitempty"`
	// Script is a relative path to a Lua file under scripts/ (e.g.
	// "docs/release_notes.lua"). The script runs in document mode with
	// rela.mode="document", rela.document.{id,entry_id}, and captures its
	// stdout as markdown. Mutually exclusive with Command.
	Script string `yaml:"script,omitempty" json:"script,omitempty"`
	// Timeout is the render timeout in seconds. Defaults to 30. Applies
	// to both Command and Script renderers.
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	// Edit, when set, exposes an Edit button in the standalone document view
	// header that navigates to the named form for the document's entity.
	// Absent = no button. Validated against cfg.Forms at config-load time.
	//
	// YAML caveat: a bare `edit:` line with no subkeys deserialises to nil
	// (not &DocumentEdit{}), so the no-button case includes both "field
	// omitted" and "field present but empty". Authors who want validation
	// to flag a stub block must write `edit: {}` explicitly.
	Edit *DocumentEdit `yaml:"edit,omitempty" json:"edit,omitempty"`
}

// DocumentEdit configures the Edit button on the standalone document view.
// Both fields are required when the parent block is present.
type DocumentEdit struct {
	// Form is the form ID to navigate to. Must reference an existing form.
	Form string `yaml:"form" json:"form"`
	// Label is the visible button text. Author-controlled to disambiguate
	// multi-entity docs (e.g. "Edit release", "Open ticket").
	Label string `yaml:"label" json:"label"`
}
