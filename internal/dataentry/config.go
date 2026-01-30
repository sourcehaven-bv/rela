// Package dataentry provides a config-driven data entry web application
// built on top of rela's metamodel system. It reads a data-entry.yaml config
// file alongside a rela project and serves an interactive UI for CRUD operations
// on entities stored as markdown files.
package dataentry

// Config is the top-level configuration for a data entry application.
type Config struct {
	Version    string                       `yaml:"version"`
	App        AppConfig                    `yaml:"app"`
	Styles     map[string]map[string]string `yaml:"styles"`
	Forms      map[string]Form              `yaml:"forms"`
	Lists      map[string]List              `yaml:"lists"`
	Views      map[string]ViewConfig        `yaml:"views"`
	Navigation []NavigationEntry            `yaml:"navigation"`
}

// AppConfig holds display metadata for the application.
type AppConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Form defines a create/edit form for an entity type.
type Form struct {
	EntityType  string         `yaml:"entity_type"`
	Title       string         `yaml:"title"`
	Description string         `yaml:"description"`
	Mode        string         `yaml:"mode"`
	Body        *bool          `yaml:"body,omitempty"`
	Fields      []FormField    `yaml:"fields"`
	Relations   []FormRelation `yaml:"relations"`
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
	Relation    string             `yaml:"relation"`
	Direction   string             `yaml:"direction"`
	TargetType  string             `yaml:"target_type"`
	Label       string             `yaml:"label"`
	Required    bool               `yaml:"required"`
	Widget      string             `yaml:"widget"`
	AllowCreate bool               `yaml:"allow_create"`
	CreateForm  string             `yaml:"create_form"`
	Properties  []RelationProperty `yaml:"properties"`
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
	Sort           *SortConfig     `yaml:"sort,omitempty"`
	Filters        []FilterConfig  `yaml:"filters"`
	FilterControls []FilterControl `yaml:"filter_controls"`
	CreateForm     string          `yaml:"create_form"`
	EditForm       string          `yaml:"edit_form"`
	DetailView     string          `yaml:"detail_view"`
	PageSize       int             `yaml:"page_size"`
}

// ListColumn defines a column in a list view.
type ListColumn struct {
	Property string `yaml:"property"`
	Label    string `yaml:"label"`
	Sortable bool   `yaml:"sortable"`
	Link     bool   `yaml:"link"`
}

// SortConfig defines default sort order for a list.
type SortConfig struct {
	Property  string `yaml:"property"`
	Direction string `yaml:"direction"`
}

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

// NavigationEntry defines a sidebar navigation item.
type NavigationEntry struct {
	Label string `yaml:"label"`
	List  string `yaml:"list"`
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
