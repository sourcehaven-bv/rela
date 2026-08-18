// Package dataentryconfig contains the configuration types and validation logic
// for the data-entry web application. This package is separated from the main
// dataentry package so that the CLI can import config/validation without pulling
// in the full web application layer (goldmark, templates, git, etc.).
package dataentryconfig

import (
	"fmt"
	"net/url"
	"strings"

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
	WidgetDatetime    = "datetime"
	WidgetRrule       = "rrule"
	// WidgetFile is registered by the SPA (frontend/src/widgets/registry.ts)
	// but had no Go constant until TKT-3R7RF3 needed to name it in the section
	// field widget table. Note Metamodel.ResolveWidgetFromType has no `file`
	// case and resolves a file property to "text" — a real divergence from the
	// SPA's defaultWidgetFor, documented at sectionFieldWidgetTypes.
	WidgetFile  = "file"
	WidgetCards = "cards" // card-based UI for relations with properties
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
	case "":
		// A written-but-empty `direction: ""` (or a bare `direction:`) is NOT
		// the same as an absent key. Absent means "infer from the metamodel";
		// collapsing an empty value to outgoing here would let a config walk
		// straight past the ambiguity check that makes a self-referencing
		// relation an error. Leave it empty so the single inference rule in
		// InferDirection owns the decision.
		*d = ""
	case "outgoing":
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
	Feeds       map[string]Feed              `yaml:"feeds,omitempty" json:"feeds,omitempty"`
	CalDAV      CalDAVConfig                 `yaml:"caldav,omitempty" json:"caldav,omitzero"`
	Dashboard   *DashboardConfig             `yaml:"dashboard,omitempty"`
	Commands    map[string]CommandConfig     `yaml:"commands,omitempty"`
	Actions     map[string]Action            `yaml:"actions,omitempty"`
	Navigation  []NavigationEntry            `yaml:"navigation"`

	// NextActionBands is the operator's ordered priority vocabulary; list
	// order IS priority order, highest first. See nextaction.go.
	NextActionBands []NextActionBand `yaml:"next_action_bands,omitempty" json:"next_action_bands,omitempty"`
	// NextActions are the suggestion sources, keyed by source id. The id is
	// half the suggestion key and the unit of muting, so it is stable
	// operator-facing vocabulary, not an implementation detail.
	NextActions map[string]NextActionSource `yaml:"next_actions,omitempty" json:"next_actions,omitempty"`
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
	// PlantUMLServerURL is the base URL of a PlantUML rendering server (e.g.
	// "https://plantuml.internal.example.com"). When set, the SPA renders
	// ```plantuml fenced code blocks as diagrams by pointing an <img> at
	// "<url>/svg/<encoded>". Empty (the default) disables PlantUML rendering
	// entirely — blocks degrade to plain code, and no diagram source ever
	// leaves the browser. Deliberately not defaulted to the public
	// plantuml.com server: that would silently publish private diagram source
	// to a third party. Operators opt in by configuring a server they trust.
	PlantUMLServerURL string `yaml:"plantuml_server_url,omitempty" json:"plantuml_server_url,omitempty"`
	// DisableCustomInjection turns off referencing the operator's
	// custom/custom.css and custom/custom.js from the SPA shell, guaranteeing a
	// stock UI.
	//
	// Named as a *disable* flag against the opt-in direction of its
	// neighbors because the feature is on by default: dropping custom.css into
	// custom/ should just work, with no second step. An operator who
	// wants to guarantee an unmodified UI (or to bisect whether a
	// customisation is causing a bug) sets this to true. Files in custom/ remain
	// individually fetchable under /_custom/ either way — only the shell
	// references are suppressed.
	DisableCustomInjection bool `yaml:"disable_custom_injection,omitempty" json:"disable_custom_injection,omitempty"`
}

// Form defines a create/edit form for an entity type.
//
// A form is EITHER single-page (Fields/Relations at the top level) OR a wizard
// (Steps). The two are mutually exclusive — validateForms rejects a form that
// sets both. A wizard renders one step at a time with next/back navigation;
// single-page remains the default and is unchanged when Steps is empty.
type Form struct {
	EntityType  string           `yaml:"entity_type" json:"entity"`
	Title       string           `yaml:"title" json:"title"`
	Description string           `yaml:"description" json:"description,omitempty"`
	Mode        string           `yaml:"mode" json:"mode,omitempty"`
	Body        *bool            `yaml:"body,omitempty" json:"body,omitempty"`
	Fields      []FormField      `yaml:"fields" json:"fields"`
	Relations   []FormRelation   `yaml:"relations" json:"relations,omitempty"`
	Steps       []FormStep       `yaml:"steps,omitempty" json:"steps,omitempty"`
	SidePanel   *SidePanelConfig `yaml:"side_panel,omitempty" json:"side_panel,omitempty"`
}

// FormStep is one ordered, titled step of a wizard form. VisibleWhen is an
// optional condition expression (see the frontend conditions engine) evaluated
// against earlier field values; when it evaluates false the step is skipped in
// navigation and its values are excluded from the submitted entity.
type FormStep struct {
	Title       string         `yaml:"title" json:"title"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	VisibleWhen string         `yaml:"visible_when,omitempty" json:"visible_when,omitempty"`
	Fields      []FormField    `yaml:"fields,omitempty" json:"fields,omitempty"`
	Relations   []FormRelation `yaml:"relations,omitempty" json:"relations,omitempty"`
}

// SidePanelConfig defines an optional context panel shown alongside a form.
// It reuses the view traversal and section display system.
type SidePanelConfig struct {
	Traverse []ViewTraverse `yaml:"traverse" json:"traverse"`
	Sections []ViewSection  `yaml:"sections" json:"sections"`
}

// ClearWhenHidden values. Default (empty) is ClearWhenHiddenNo: a
// condition-hidden field KEEPS its stored value (BUG-FB0LN8).
//
//	no       keep the value; hide → reveal is lossless (default)
//	yes      clear it when the branch hides
//	confirm  ask first; on decline, undo the triggering change too
//
// "confirm" is not merely "yes with a prompt" — it can also UNDO the change
// that triggered it, which "yes" never does.
//
// It was rejected outright until TKT-7S5735: it needs the form to separate
// "the user proposed a change" from "the change was committed", and an edit
// used to mutate form state and arm the autosave in one step, so a decline had
// to reconstruct state after the fact. That is where several bugs lived. The
// propose/commit seam now decides before anything is applied, so a decline is
// a true no-op rather than a rollback.
const (
	ClearWhenHiddenNo      = "no"
	ClearWhenHiddenYes     = "yes"
	ClearWhenHiddenConfirm = "confirm"
)

// ValidClearWhenHidden is the allowlist for FormField.ClearWhenHidden.
var ValidClearWhenHidden = map[string]bool{
	ClearWhenHiddenNo:      true,
	ClearWhenHiddenYes:     true,
	ClearWhenHiddenConfirm: true,
}

// FormField defines a single field in a form.
//
// VisibleWhen / RequiredWhen are optional condition expressions (see the
// frontend conditions engine) evaluated against earlier field values. They are
// opaque strings to Go — the SPA parses and evaluates them; the `rela` config
// lint checks them syntactically. VisibleWhen hides the field when false;
// RequiredWhen makes the field required only when true. See ClearWhenHidden
// for what happens to a hidden field's stored value.
type FormField struct {
	Property     string `yaml:"property" json:"property"`
	Label        string `yaml:"label" json:"label,omitempty"`
	Placeholder  string `yaml:"placeholder" json:"placeholder,omitempty"`
	Help         string `yaml:"help" json:"help,omitempty"`
	Widget       string `yaml:"widget" json:"widget,omitempty"`
	Required     *bool  `yaml:"required,omitempty" json:"required,omitempty"`
	RequiredWhen string `yaml:"required_when,omitempty" json:"required_when,omitempty"`
	VisibleWhen  string `yaml:"visible_when,omitempty" json:"visible_when,omitempty"`
	// ClearWhenHidden decides the fate of this field's STORED value when
	// VisibleWhen turns false (BUG-FB0LN8). "" / "no" (default) keeps it —
	// hiding is presentation, not a delete; "yes" clears it. Per-FIELD only:
	// a step hiding is simply "all of its fields hid", each honoring its own
	// setting, so there is no step-level key.
	ClearWhenHidden string              `yaml:"clear_when_hidden,omitempty" json:"clear_when_hidden,omitempty"`
	Default         string              `yaml:"default" json:"default,omitempty"`
	Hidden          bool                `yaml:"hidden" json:"hidden,omitempty"`
	Transitions     map[string][]string `yaml:"transitions,omitempty" json:"transitions,omitempty"`

	// Span places the field on the 12-column layout grid; 0 means full width.
	// Same semantics as ViewSectionField.Span — forms and view sections are
	// separate structs but share one layout model, so an author doesn't have
	// to learn two.
	Span Span `yaml:"span,omitempty" json:"span,omitempty"`
}

// SpanColumns is the width of the layout grid that FormField.Span and
// ViewSectionField.Span address. 12 is the Filament/Bootstrap convention,
// chosen because it divides cleanly into halves, thirds, quarters and sixths.
//
// Mirrored by SPAN_COLUMNS in frontend/src/utils/fieldSpan.ts and by the
// `repeat(12, ...)` literal in frontend/src/styles/properties-list.css. The CSS
// copy is unavoidable (media queries and repeat() need literals); keep all
// three in step if the grid width ever changes.
const SpanColumns = 12

// Span is a field's width on the layout grid, decoded strictly.
//
// It exists as a named type ONLY to reject a fractional value. yaml.v3 happily
// decodes `span: 6.5` into a plain int as 6 — it rejects `span: half` and
// `span: true` but truncates a float in silence. An author reaching for "half
// of a third" would get 6 and no diagnostic, which is precisely the
// layout-ignores-what-you-wrote failure the loud validation exists to prevent.
// validateSpan cannot catch it: by the time it sees an int, the fraction is
// already gone.
type Span int

// UnmarshalYAML decodes a span, rejecting non-integer numbers.
func (s *Span) UnmarshalYAML(value *yaml.Node) error {
	var f float64
	if err := value.Decode(&f); err != nil {
		// Not a number at all — let the int decode produce the usual
		// "cannot unmarshal !!str into int" message rather than inventing one.
		var i int
		if err := value.Decode(&i); err != nil {
			return err
		}
		*s = Span(i)
		return nil
	}
	if f != float64(int(f)) {
		return fmt.Errorf("span must be a whole number of columns, got %v", f)
	}
	*s = Span(int(f))
	return nil
}

// ValidIconNames is the allowlist of icon names an author may reference from
// data-entry.yaml. It MUST stay in step with the SPA's registry in
// frontend/src/utils/icons.ts — TestIconAllowlistMatchesFrontend reads that
// file and fails on drift, in either direction: a name the config accepts but
// the SPA can't render silently degrades to a fallback icon, and a name the SPA
// knows but the config rejects is a feature an author can't reach.
//
// Exported so the icon-name check and its test share one source.
var ValidIconNames = map[string]bool{
	// Navigation
	"dashboard": true,
	"list":      true,
	"kanban":    true,
	"search":    true,
	"warning":   true,
	"apps":      true,
	"settings":  true,
	"document":  true,
	// Theme toggle
	"sun":  true,
	"moon": true,
	// Workflow-ish names, useful for kanban columns
	"inbox":  true,
	"wrench": true,
	"done":   true,
	"clock":  true,
	"status": true,
}

// validateIconName reports a config error for an unknown icon name.
//
// Loud at load, like validateSpan: the SPA falls back to a default icon so a
// stale config still renders, but silently swapping an author's chosen icon for
// a generic one with no diagnostic is the failure mode strict validation exists
// to prevent. An empty name means "no icon" and is always valid.
func validateIconName(icon, context string) []string {
	if icon == "" || ValidIconNames[icon] {
		return nil
	}
	return []string{fmt.Sprintf("%s: unknown icon %q (valid: %s)",
		context, icon, strings.Join(sortedMapKeys(ValidIconNames), ", "))}
}

// validateSpan reports a config error for an out-of-range span.
//
// Deliberately loud rather than clamped: this codebase validates data-entry.yaml
// strictly at load (ValidateConfig even suggests corrections for typo'd keys),
// so silently rendering `span: 13` as full width would leave an author with a
// layout that ignores what they wrote and no diagnostic to grep for. The
// frontend still defends independently — a bad value arriving over the wire
// falls back to full width rather than emitting broken CSS.
func validateSpan(span Span, context string) []string {
	if span == 0 || (span >= 1 && span <= SpanColumns) {
		return nil
	}
	return []string{fmt.Sprintf("%s: span %d is out of range (must be 1-%d, or omitted for full width)",
		context, span, SpanColumns)}
}

// FormRelation defines a relation field in a form. VisibleWhen is an optional
// condition expression (see FormField) that hides the relation widget when
// false.
type FormRelation struct {
	Relation     string             `yaml:"relation" json:"relation"`
	Direction    Direction          `yaml:"direction" json:"direction,omitempty"`
	TargetType   string             `yaml:"target_type" json:"target_type,omitempty"`
	Label        string             `yaml:"label" json:"label,omitempty"`
	Required     bool               `yaml:"required" json:"required,omitempty"`
	VisibleWhen  string             `yaml:"visible_when,omitempty" json:"visible_when,omitempty"`
	Widget       string             `yaml:"widget" json:"widget,omitempty"`
	Display      string             `yaml:"display" json:"display,omitempty"`
	Properties   []RelationProperty `yaml:"properties" json:"properties,omitempty"`
	Fields       []ViewSectionField `yaml:"fields" json:"fields,omitempty"`
	EmptyMessage string             `yaml:"empty_message" json:"empty_message,omitempty"`

	// Span is captured ONLY so it can be rejected. A relation renders via the
	// card/picker widgets, which have a natural minimum width — a narrow grid
	// column would break them — so unlike a form field, a relation has no
	// meaningful span. Without this field yaml.v3 would drop the key in
	// silence, and an author who wrote `span: 6` here (reasonably, since it
	// works one line above on a field) would get no error and no effect.
	// validateFormRelation turns it into a specific message instead.
	Span Span `yaml:"span,omitempty" json:"-"`
}

// RelationProperty defines an editable property on a relation.
type RelationProperty struct {
	Property string `yaml:"property" json:"property"`
	Label    string `yaml:"label" json:"label,omitempty"`
	Required bool   `yaml:"required" json:"required,omitempty"`
}

// List defines a list view for an entity type.
//
// Header and Footer carry admin-authored markdown rendered above and below the
// list, respectively (sanitized client-side via renderMarkdown). Description
// predates this feature but was never rendered; the SPA now adopts it as a
// fallback for Header (used only when Header is empty) so existing configs that
// happen to set it get a header region without a rewrite.
type List struct {
	EntityType     string          `yaml:"entity_type" json:"entity"`
	Title          string          `yaml:"title" json:"title"`
	Header         string          `yaml:"header" json:"header,omitempty"`
	Footer         string          `yaml:"footer" json:"footer,omitempty"`
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

	// ExportRender is an optional Lua script (relative path under scripts/,
	// e.g. "docs/ticket_report.lua") that renders THIS LIST for export. When
	// set, "Export as PDF/ODT" on the list routes through the script instead
	// of the built-in column table, so an operator fully controls the
	// exported document (grouped sections, summaries, a cover header —
	// anything a table cannot express).
	//
	// The script runs in list-document mode and receives the rows the server
	// already resolved: rela.document.rows()/row(i)/count, plus the resolved
	// query as read-only context. It must NOT derive its own row set — the
	// handler resolved exactly the ACL-scoped, filtered, sorted, capped set
	// the user is looking at, and re-querying would both diverge from that
	// view and escape the row cap. Empty → built-in column table.
	ExportRender string `yaml:"export_render,omitempty" json:"export_render,omitempty"`
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

// HasProperty reports whether the filter names a property to filter on
// (filters may also be written without one, e.g. operator-only entries
// that validation flags separately).
func (f FilterConfig) HasProperty() bool { return f.Property != "" }

// FilterControl defines a user-facing filter control in a list.
// Exactly one of Property or Relation must be set:
//   - Property: filter on a scalar property of the entity.
//   - Relation: filter by the target title of a relation; the relation name
//     must exist in the metamodel. Direction controls whether the filter
//     follows edges pointing FROM the row (outgoing, the default) or TO the
//     row (incoming). An incoming relation filter keeps rows whose incoming
//     source titles match the requested value, mirroring ListColumn.Direction.
//
// Label is an optional display label override for the control.
type FilterControl struct {
	Property  string    `yaml:"property,omitempty" json:"property,omitempty"`
	Relation  string    `yaml:"relation,omitempty" json:"relation,omitempty"`
	Direction Direction `yaml:"direction,omitempty" json:"direction,omitempty"` // "outgoing" (default) or "incoming"
	Label     string    `yaml:"label,omitempty" json:"label,omitempty"`
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

// RelationFilterDirection returns the configured Direction for a relation
// filter control keyed by relation on any list of the given entity type. It
// scans every list whose EntityType matches, returning the matching
// FilterControl's direction. Returns (DirectionOutgoing, false) when no such
// filter control is configured — callers use the `ok` return to decide whether
// a relation filter applies at all (RR-B0JPPL: a relation filter only applies
// when a control configures it).
//
// Lowest-list-ID wins: Config.Lists is a map, so iterating it directly would
// randomize which list's direction wins when two lists of the same entity type
// configure the same relation with conflicting directions (RR-9MJRJG). We
// iterate list IDs in sorted order so the answer is deterministic per process.
// CollectConfigWarnings surfaces conflicting directions at load time; this
// resolver just needs a stable answer.
func (c *Config) RelationFilterDirection(entityType, relation string) (Direction, bool) {
	for _, listID := range sortedListIDs(c) {
		list := c.Lists[listID]
		if list.EntityType != entityType {
			continue
		}
		for _, fc := range list.FilterControls {
			if fc.Relation == relation {
				// Normalize the empty (unset) YAML value to the outgoing
				// default so callers get a concrete direction.
				if fc.Direction.IsIncoming() {
					return DirectionIncoming, true
				}
				return DirectionOutgoing, true
			}
		}
	}
	return DirectionOutgoing, false
}

// HasPropertyFilterControl reports whether any list of the given entity type
// declares a property (non-relation) filter control for the named property.
// Used by the list pipeline to resolve a property/relation name collision in
// favor of the property when the config explicitly configures it as a property
// filter (RR-0HWAS0).
func (c *Config) HasPropertyFilterControl(entityType, property string) bool {
	for _, listID := range sortedListIDs(c) {
		list := c.Lists[listID]
		if list.EntityType != entityType {
			continue
		}
		for _, fc := range list.FilterControls {
			if !fc.IsRelation() && fc.Property == property {
				return true
			}
		}
	}
	return false
}

// Kanban defines a kanban board view for an entity type.
//
// Header and Footer carry admin-authored markdown rendered above and below the
// board, respectively (sanitized client-side via renderMarkdown), matching the
// list info regions. Unlike List there is no Description fallback: that alias
// exists only because List.Description predated the feature and was already
// present in configs, and a Kanban has no such legacy field to accommodate.
type Kanban struct {
	EntityType       string           `yaml:"entity_type" json:"entity"`
	Title            string           `yaml:"title" json:"title"`
	Header           string           `yaml:"header" json:"header,omitempty"`
	Footer           string           `yaml:"footer" json:"footer,omitempty"`
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
// Icon names an icon from the shared registry to render beside the label
// (see ValidIconNames). It is a NAME, never a glyph: putting an emoji in
// Label works and is left alone, but the SPA will never parse one back out
// of label text — that would silently rewrite what an author typed.
type KanbanColumn struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	Icon  string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// KanbanSwimlane defines a swimlane row in the kanban board.
//
// Carries Icon for the same reason KanbanColumn does: identical Value/Label
// shape, rendered the same way, so supporting one and not the other would be
// an arbitrary asymmetry an author would trip over.
type KanbanSwimlane struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	Icon  string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// KanbanCard defines how cards are displayed on the board.
type KanbanCard struct {
	Title  string            `yaml:"title" json:"title"`
	Fields []KanbanCardField `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// KanbanCardField defines a single field shown on a kanban card.
// A field references either a Property (entity property) or a Relation
// (relation type whose target titles are shown). For relation fields,
// Direction controls whether to show outgoing (default) or incoming edges.
//
// This is a dedicated type rather than a reuse of ViewSectionField: the
// latter is shared by form relations, side panels, and view sections, and
// widening it would leak card-relation semantics into all of them. An
// existing property-only card field (`- property: X`) unmarshals unchanged
// because Property carries the same yaml tag.
type KanbanCardField struct {
	Property  string    `yaml:"property,omitempty" json:"property,omitempty"`
	Relation  string    `yaml:"relation,omitempty" json:"relation,omitempty"`
	Direction Direction `yaml:"direction,omitempty" json:"direction,omitempty"` // "outgoing" (default) or "incoming"
	Label     string    `yaml:"label,omitempty" json:"label,omitempty"`
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
	// Document names a STANDALONE document (one with no entity_type) under
	// documents:. Entity-anchored documents cannot appear here — they need
	// an entry id the sidebar has no way to supply. Enforced by
	// validateNavEntry.
	Document string `yaml:"document,omitempty" json:"document,omitempty"`

	// Icon overrides the icon derived from the entry's kind. Without it every
	// list entry gets the same list glyph and every board the same board one,
	// so "My Tickets" and "Open Tickets" are visually identical — the sidebar
	// carries no signal beyond its labels. Names come from the shared registry
	// (see ValidIconNames); an unknown one is a load-time error.
	//
	// Action entries have no derived icon at all, so for those this is the
	// only way to get one.
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`

	// Permission optionally hides this entry from the sidebar for principals
	// who do not hold the named global ACL permission (TKT-TXDK8U). Empty —
	// the overwhelmingly common case — means the entry is always shown.
	//
	// This is a UX filter, NOT an access control. The entry's target enforces
	// (or does not enforce) exactly what it did before: a hidden list is still
	// reachable by typing its URL, and still returns its normal ACL-scoped
	// rows, which for a principal who may read none of them is an empty list.
	// The point is to keep menu entries a user cannot act on out of their way,
	// not to conceal that they are configured — `/api/v1/_config` still serves
	// the whole navigation tree to everyone (see "The configuration is not a
	// secret; the data is" in the root CLAUDE.md).
	//
	// Behavior with no `acl.yaml`: shown. No policy configured means no
	// restrictions, matching the read gate's allow-all posture. Same under
	// `--read-only`, which restricts writes only and so has no permission
	// model to consult — hiding there would remove read surfaces an
	// observe-only principal can use, and would hide them from everyone
	// rather than from non-holders.
	//
	// Not valid on a group entry — a group is a container, not a destination;
	// groups disappear on their own when every child is filtered out.
	Permission string `yaml:"permission,omitempty" json:"permission,omitempty"`

	// Group fields
	Group     string            `yaml:"group,omitempty" json:"group,omitempty"`
	Collapsed bool              `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`
	Items     []NavigationEntry `yaml:"items,omitempty" json:"items,omitempty"`
}

// IsGroup returns true if this entry is a navigation group.
func (n NavigationEntry) IsGroup() bool {
	return n.Group != ""
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

	// Permission optionally hides this card from the dashboard for principals
	// who do not hold the named global ACL permission (TKT-53KICM). Empty —
	// the overwhelmingly common case — means the card is always shown.
	//
	// This is a UX filter, NOT an access control, exactly as on
	// [NavigationEntry.Permission]. The card's query runs through the
	// ACL-scoped search path either way, so a principal who may read none of
	// the matching entities already sees a card reading 0 or an empty table.
	// Hiding it removes a useless tile; it does not conceal that the card is
	// configured — `/api/v1/_config` still serves the whole `dashboard:` block
	// to everyone (see "The configuration is not a secret; the data is" in the
	// root CLAUDE.md). Only `/api/v1/_dashboard` is filtered.
	//
	// Behavior with no `acl.yaml`: shown. No policy configured means no
	// restrictions. Same under `--read-only`, which restricts writes only and
	// so has no permission model to consult — hiding there would remove read
	// surfaces an observe-only principal can use, and would hide them from
	// everyone rather than from non-holders.
	Permission string `yaml:"permission,omitempty" json:"permission,omitempty"`
}

// ViewConfig defines a detailed entity view with traversal and sections.
type ViewConfig struct {
	Title    string         `yaml:"title" json:"title"`
	Entry    ViewEntry      `yaml:"entry" json:"entry"`
	Traverse []ViewTraverse `yaml:"traverse" json:"traverse"`
	Sections []ViewSection  `yaml:"sections" json:"sections"`
	// ExportRender is an optional Lua script (relative path under scripts/, e.g.
	// "docs/book_card.lua") that renders this entity type for EXPORT. When set,
	// "Export as PDF/ODT" on an entity of this type routes through the script
	// instead of the built-in property renderer, so an operator fully controls
	// the exported document. The script runs in document mode (rela.document.*)
	// through the same ACL-gated path as documents:, and its stdout is the
	// markdown fed to the transform. Empty → built-in renderer.
	ExportRender string `yaml:"export_render,omitempty" json:"export_render,omitempty"`
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

// Render modes for view section fields (TKT-HOIX1). A field renders as a
// view-oriented display value unless it opts in to inline editing.
//
// RenderInput is an opt-in to *presentation*, not a grant: the ACL verdict
// still decides writability, so `render: input` on a read-only field renders
// display. Config can downgrade an editable field, never upgrade a
// read-only one.
const (
	RenderDisplay = "display"
	RenderInput   = "input"
)

// ViewSection defines a section within a view.
//
// Render sets the default render mode for this section's fields; an
// individual field's own Render overrides it. Empty means RenderDisplay.
type ViewSection struct {
	Heading      string             `yaml:"heading,omitempty" json:"heading,omitempty"`
	Source       string             `yaml:"source" json:"source"`
	Display      string             `yaml:"display" json:"display"`
	Render       string             `yaml:"render,omitempty" json:"render,omitempty"`
	Fields       []ViewSectionField `yaml:"fields,omitempty" json:"fields,omitempty"`
	Columns      []ListColumn       `yaml:"columns,omitempty" json:"columns,omitempty"`
	GroupBy      string             `yaml:"group_by,omitempty" json:"group_by,omitempty"`
	EmptyMessage string             `yaml:"empty_message,omitempty" json:"empty_message,omitempty"`
	Link         string             `yaml:"link,omitempty" json:"link,omitempty"`
}

// ViewSectionField defines a field within a view section.
//
// Span places the field on the shared 12-column layout grid (see SpanColumns).
// Zero — the default, and what every auto-generated view emits — means full
// width, so a section with no spans authored reads as one scannable column.
// Adjacency is therefore always DECLARED: two fields share a row because an
// author said so, never because a viewport happened to fit them.
//
// Render is resolved server-side against the containing section — see
// ResolveFieldRender — so consumers receive an already-resolved value and
// never reimplement the inheritance rule.
//
// Widget overrides which registered widget renders this property, instead of
// the type-derived default (TKT-3R7RF3). Empty means "use the default", which
// is the SPA's defaultWidgetFor dispatch — NOT a value this package resolves.
//
// Deliberately field-level only, with no section-level counterpart, which is
// the one structural difference from Render. Render can inherit because BOTH
// of its values are valid for every field; a section-level widget would be a
// config-load error on every field whose type does not match it, turning one
// authored line into N errors the operator must override back field by field.
// Validating one would also need each field's property type — exactly the
// metamodel-dependent context RR-4ICH8M moved the field-level check out of.
type ViewSectionField struct {
	Property string `yaml:"property" json:"property"`
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
	Span     Span   `yaml:"span,omitempty" json:"span,omitempty"`
	Render   string `yaml:"render,omitempty" json:"render,omitempty"`
	Widget   string `yaml:"widget,omitempty" json:"widget,omitempty"`
}

// ResolveFieldRender returns the effective render mode for a field within a
// section: the field's own Render, else the section's, else RenderDisplay.
//
// Single source of truth for the inheritance rule — both section builders
// call this so they cannot drift.
func ResolveFieldRender(sectionRender, fieldRender string) string {
	if fieldRender != "" {
		return fieldRender
	}
	if sectionRender != "" {
		return sectionRender
	}
	return RenderDisplay
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

	// Permission names the global ACL permission a principal must hold to
	// execute this command (e.g. "command:nightly-export"), granted via a
	// role's `permissions:` list in acl.yaml. It follows the
	// [acl.PermHistoryRead] pattern: commands have no entity Subject, so
	// they cannot be authorized through acl.ACL.AuthorizeWrite.
	//
	// The key is only consulted when an acl.yaml is configured. Per
	// DEC-EIHQSU authorization is bimodal: with no policy every command
	// runs (unchanged pre-ACL behavior); with a policy a command is denied
	// unless its Permission is set and held. Under `--read-only` every
	// command is denied regardless.
	//
	// NOT honored for `context: view` — view commands are denied outright
	// under any configured policy because their payload is the whole view
	// traversal closure rather than one entity (TKT-MJ02AO). Setting it on
	// a view command is a config warning.
	Permission string `yaml:"permission,omitempty"`
}

// CommandScope controls where a command button appears in the UI.
type CommandScope struct {
	Views       []string `yaml:"views,omitempty"`
	Lists       []string `yaml:"lists,omitempty"`
	EntityTypes []string `yaml:"entity_types,omitempty"`
	Dashboard   bool     `yaml:"dashboard,omitempty"`
}

// DocumentConfig defines how to render a document, either from an entry
// entity or standalone.
//
// Exactly one of Command or Script must be set. Command shells out to an
// external process that produces markdown on stdout; Script executes a Lua
// script from scripts/ under the project root and captures its stdout.
// Validated via validateDocuments at config-load time.
//
// A document is one of two KINDS, discriminated by EntityType:
//
//   - Entity-anchored (EntityType set) — renders about one entity, served at
//     /_documents/{name}/{entryID}, ACL-gated on that entity.
//   - Standalone (EntityType empty) — renders company-wide content aggregated
//     across the graph, served at /_documents/{name}, reachable from a
//     `document:` navigation entry. rela.document.entry_id is nil.
//
// The two are mutually exclusive at every consumer: each endpoint shape
// rejects the other kind rather than inventing a missing entry id.
type DocumentConfig struct {
	// Title is the display title for the document.
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
	// EntityType specifies which entity types this document applies to.
	// Used by the frontend to filter which documents to show for a given entity,
	// and by the HTTP handler to reject cross-type requests (a doc with
	// entity_type=release cannot render against a ticket entity).
	//
	// EMPTY means the document is standalone (see the type doc). It is not a
	// missing-required-field: standalone is a first-class kind.
	EntityType string `yaml:"entity_type,omitempty" json:"entity_type,omitempty"`
	// Permission optionally gates rendering on a global named permission from
	// acl.yaml (the acl.PermHistoryRead / delegate-X family). Empty means any
	// principal may render the document.
	//
	// WHAT THIS GUARDS DEPENDS ON AllowACLBypass (TKT-Y3JVFK) — the two
	// meanings are different in kind, so be clear which one applies:
	//
	// Without elevation (the common case) it is an INTENT and UX gate, NOT a
	// confidentiality boundary. The document's Lua reads already go through
	// the ACL-gated lua.ReadDeps.VisibleReader, so a principal who may not
	// read the underlying entities renders a partial document either way and
	// nothing leaks. What Permission buys is that a report claiming a scope
	// its reader cannot actually compute — "company-wide revenue" rendered
	// over one manager's clients — is withheld rather than served as a
	// smaller number that looks authoritative. That is misinformation, not
	// disclosure. It also keeps unusable entries out of a sidebar. Because it
	// is not the boundary, it is optional here.
	//
	// WITH elevation it IS the confidentiality boundary, and is REQUIRED
	// (enforced in validateDocuments). An elevated render reads through a raw
	// handle, so nothing downstream bounds its output; the permission is the
	// only thing between a principal and everything the script reads. Note
	// this grants "may read whatever this script reads", not "may view this
	// report" — see the AllowACLBypass godoc.
	//
	// Honored for both document kinds. On an entity-anchored document it
	// applies IN ADDITION to the per-entity read gate (both must pass); it can
	// never widen entity visibility.
	Permission string `yaml:"permission,omitempty" json:"permission,omitempty"`
	// AllowACLBypass, when set, unlocks rela.bypass_acl inside this document's
	// Lua script, letting it read entities the requesting principal cannot see
	// (TKT-Y3JVFK). The motivating case is a report that must compute over
	// hidden rows — benchmarking a sales manager against peers whose clients
	// are invisible to them — which no acl.yaml role can express, because
	// granting enough to compute the benchmark grants enough to enumerate the
	// competitors.
	//
	// ONLY metamodel.ACLBypassRead is accepted. `write` and `read+write` are a
	// config error, because a render is served on a GET and elevated writes
	// there would be neither idempotent (browsers prefetch, users refresh, the
	// SPA retries) nor compatible with caching a principal-independent render
	// (TKT-OGR566, RR-P4E9GL). Writes a report seems to want — memoizing an
	// expensive aggregate, logging that a report was viewed — belong in an
	// automation action or a schedule, which are event-triggered, idempotent by
	// design, and already audited as writes.
	//
	// This rule REFUSES TO WIDEN an existing gap rather than closing one: a
	// document script today still has the ordinary gated rela.* write bindings
	// (TKT-PX5YL7), so a render can already mutate within the caller's own
	// permissions. What this refusal prevents is a render mutating BEYOND
	// them.
	//
	// Setting this REQUIRES Permission (see above): an elevated document with
	// no permission publishes whatever the script reads to every principal.
	//
	// The script is TRUSTED CODE. bypass_acl hands it a raw reader and nothing
	// stops it printing what it reads, so review the bypass block before
	// deploying — that review IS the mitigation, and it is why this value is
	// declared in config where a reviewer will see it.
	AllowACLBypass metamodel.ACLBypass `yaml:"allow_acl_bypass,omitempty" json:"allow_acl_bypass,omitempty"`
	// Command is the external render command as an ARGUMENT ARRAY, e.g.
	//   command: ["my-renderer", "{in}"]
	// It is executed directly — there is no shell, so pipes, redirection, and
	// variable expansion are not available and no quoting is required.
	//
	// The single placeholder is {in}: a temp file holding the entry entity's
	// markdown, frontmatter included. The id is the `id:` key of that
	// frontmatter, so a renderer that needs it reads it from the file.
	//
	// The former {id} / {id_lower} placeholders are GONE (TKT-QGHNVA). They
	// spliced a request-derived value into a shell string, which made the
	// entity id the one piece of user-controlled data reaching `sh -c`; an id
	// leading with "-" then landed as an option flag rather than an operand.
	// A config still using them is rejected at load time with a message naming
	// {in}, rather than silently substituting nothing.
	//
	// Mutually exclusive with Script.
	Command []string `yaml:"command,omitempty" json:"command,omitempty"`
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

// IsStandalone reports whether the document renders without an entry entity.
// It is the single discriminator between the two document kinds; consumers
// must branch on this rather than testing EntityType inline, so the meaning of
// "empty entity_type" stays in one place.
func (d DocumentConfig) IsStandalone() bool {
	return d.EntityType == ""
}

// DocumentEdit configures the Edit button on the full-page document view.
// Both fields are required when the parent block is present.
type DocumentEdit struct {
	// Form is the form ID to navigate to. Must reference an existing form.
	Form string `yaml:"form" json:"form"`
	// Label is the visible button text. Author-controlled to disambiguate
	// multi-entity docs (e.g. "Edit release", "Open ticket").
	Label string `yaml:"label" json:"label"`
}

// Feed is a declarative calendar feed: a named calendar composed of one or more
// [FeedSource] projections that merge into a single calendar. It is served as
// iCalendar (.ics) and JSON at /api/v1/_feeds/<name>.{ics,json}. See
// TKT-RDM9M5 and the calfeed package.
type Feed struct {
	// Meta is optional calendar-level metadata (name, color, description).
	Meta FeedMeta `yaml:"meta,omitempty" json:"meta,omitzero"`
	// Sources are the event projections; at least one is required. Their events
	// are merged into one calendar (which is also how OR is expressed, since a
	// single source's filter clauses are ANDed).
	Sources []FeedSource `yaml:"sources" json:"sources"`
}

// FeedMeta is calendar-level metadata for a [Feed].
type FeedMeta struct {
	// Name is the calendar's display name (iCalendar X-WR-CALNAME). Defaults to
	// the feed's config key when empty.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// Color is an optional calendar color, e.g. "#C2185B".
	Color string `yaml:"color,omitempty" json:"color,omitempty"`
	// Description is optional calendar-level text.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// FeedSource projects entities of one type into calendar events. Each surviving
// entity yields one all-day event (Phase 1). Properties are mapped to event
// fields by name; nothing is computed.
type FeedSource struct {
	// EntityType is the entity type to project. Required; validated at load.
	EntityType string `yaml:"entity_type" json:"entity_type"`
	// Where is a list of filter clauses (e.g. "status != done", "due != "),
	// all ANDed. Uses the internal/filter language. Empty selects all entities
	// of the type. There is no OR — use a second source for OR.
	Where []string `yaml:"where,omitempty" json:"where,omitempty"`
	// Date names a date-typed property mapped to the event's day
	// (DTSTART;VALUE=DATE). Required in Phase 1; entities lacking a value are
	// skipped.
	Date string `yaml:"date" json:"date"`
	// EndDate optionally names a date-typed property mapped to the (exclusive)
	// end of an all-day range (DTEND;VALUE=DATE). Omit for single-day events.
	EndDate string `yaml:"end_date,omitempty" json:"end_date,omitempty"`
	// Rrule optionally makes events recurring. Its value is disambiguated by
	// SYNTAX: a value containing "=" is a literal RFC 5545 rule applied to every
	// event ("FREQ=DAILY"); a bare identifier is a property name whose value is
	// used per entity. An unbounded rule keeps an event visible until it leaves
	// the feed. Validated at load.
	Rrule string `yaml:"rrule,omitempty" json:"rrule,omitempty"`
	// Summary names a property mapped to the event title (SUMMARY). Optional;
	// defaults to the entity type's display property when omitted.
	Summary string `yaml:"summary,omitempty" json:"summary,omitempty"`
	// Description names an optional property mapped to DESCRIPTION.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Alarm is an optional static RFC 5545 duration (e.g. "-PT9H") mapped to a
	// VALARM reminder on every event from this source.
	Alarm string `yaml:"alarm,omitempty" json:"alarm,omitempty"`
}

// CalDAVConfig groups the CalDAV collections by how they come into existence.
//
//	caldav:
//	  static:
//	    tasks: {entity_type: task, ...}
//
// Only `static:` exists today — one YAML key, one collection, at
// /calendars/<key>/. The nesting is here because a second kind is coming
// (TKT-JPDXMO): collections GENERATED from the graph, one per entity of some
// driver type, where a key names a PATTERN that expands rather than a
// collection that exists.
//
// # Why the level of nesting exists before its second member does
//
// Because adding it later is a breaking config change and adding it now is
// free. `caldav:` has never shipped in a release, so there is exactly one
// moment where the shape can be fixed without a migration and a deprecation
// cycle, and this is it.
//
// The alternative considered was a top-level sibling (`caldav:` +
// `caldav_dynamic:`). Rejected because it leaves the COMMON case unnamed: a
// reader meeting a bare `caldav:` cannot tell there is another kind until they
// happen to see the sibling, and the sibling is the rarer feature. Naming both
// halves makes the pairing self-describing at the point of confusion.
//
// `dynamic:` is deliberately NOT declared yet. A config key that parses and
// then errors with "not implemented" is worse than one that does not parse —
// the second tells the operator the truth immediately. It slots in as a pure
// addition when the feature lands.
type CalDAVConfig struct {
	// Static declares collections one-to-one with config keys: the key is the
	// URL segment and the alias key (so it must stay stable — users paste the
	// URL into clients), and Meta.Name is the display label.
	Static map[string]CalDAVCollection `yaml:"static,omitempty" json:"static,omitempty"`
	// Dynamic declares PATTERNS that expand: each key yields one collection per
	// entity of its driver type, so `project_tasks` over 40 projects serves 40
	// collections named `project_tasks--PROJ-1` … See [CalDAVDynamicCollection].
	Dynamic map[string]CalDAVDynamicCollection `yaml:"dynamic,omitempty" json:"dynamic,omitempty"`
}

// IsZero reports whether any CalDAV collection is configured, so `omitzero`
// keeps an unconfigured server's JSON free of an empty caldav object.
func (c CalDAVConfig) IsZero() bool { return len(c.Static) == 0 && len(c.Dynamic) == 0 }

// CalDAVUnlinkPolicy decides what a DELETE against a dynamic collection does
// when it removes an entity's LAST membership.
//
// A DELETE addressed at `project_tasks--PRJ-home/TSK-1.ics` names a MEMBERSHIP,
// not the entity: the user removed the to-do from Home, not from existence. So
// the edge is always what goes. The only real question is what should happen to
// an entity that now belongs to nothing, and that is genuinely
// deployment-specific — hence a policy rather than a hardcoded answer.
//
// Removing a NON-last membership is never affected: the entity still belongs
// somewhere, so only the edge is removed whatever the policy says.
type CalDAVUnlinkPolicy string

// Unlink policies.
const (
	// CalDAVUnlinkAuto derives the answer from the relation's own cardinality,
	// and is the default.
	//
	// If the relation declares `min_outgoing >= 1`, an entity with no
	// memberships violates a constraint the OPERATOR already stated in the
	// metamodel, so the collection's `on_delete:` is applied — the schema is
	// the single source of truth for whether membership is mandatory, rather
	// than a second setting that can disagree with it.
	//
	// Otherwise the entity is kept with its edge removed: membership is
	// optional by declaration, so an entity belonging to nothing is a legal
	// state and destroying it would exceed what the user asked for.
	//
	// CAVEAT: a cardinality violation is a WARNING at write time in rela
	// (DEC-HWZHA — a write is never blocked for one), so `min_outgoing` is read
	// here as a statement of intent, not as an enforced invariant.
	CalDAVUnlinkAuto CalDAVUnlinkPolicy = "auto"
	// CalDAVUnlinkKeep always keeps the entity, removing only the edge. The
	// entity survives outside every dynamic collection, reachable through a
	// static collection or the web app.
	CalDAVUnlinkKeep CalDAVUnlinkPolicy = "keep"
	// CalDAVUnlinkDelete always applies the collection's `on_delete:` when the
	// last membership goes — for deployments where a to-do outside every
	// project is meaningless.
	CalDAVUnlinkDelete CalDAVUnlinkPolicy = "delete"
)

// OrDefault returns the policy, defaulting to [CalDAVUnlinkAuto].
func (p CalDAVUnlinkPolicy) OrDefault() CalDAVUnlinkPolicy {
	if p == "" {
		return CalDAVUnlinkAuto
	}
	return p
}

// dynamicNameSep joins a pattern key to its driver id in the URL segment
// (`project_tasks--PRJ-1`). Mirrors internal/dataentry's feedUIDSep; duplicated
// rather than imported because dataentryconfig must not depend on dataentry.
const dynamicNameSep = "--"

// CalDAVDynamicCollection declares a PATTERN that expands into one collection
// per entity of a driver type — a to-do list per project, per sprint, per
// person.
//
// The key names the pattern, not a collection: `project_tasks` is not
// addressable, `project_tasks--PROJ-1` is. That is why `dynamic:` is a named
// sibling of `static:` rather than more entries in one map — a reader can tell
// which keys are collections and which expand.
//
// # The composite URL segment is FORCED, not chosen
//
// go-webdav classifies a resource by its DEPTH below the mount prefix (root /
// principal / home-set / calendar / object), so `calendars/project_tasks/PROJ-1/`
// would be read as an OBJECT, not a collection. The pattern key and the driver
// id must therefore share one path segment.
//
// `--` is the separator because entity ids cannot contain it (they match
// `^[A-Za-z0-9][A-Za-z0-9_-]*$`), so the split is unambiguous and needs no
// escaping — which matters, because a collection URL has to stay human-typable:
// Thunderbird does not auto-discover collections and the user pastes it by hand.
//
// # Why the key stays
//
// Deriving the segment from EntityType instead (`task--PROJ-1`) would drop the
// key and read more cleanly, but it collides the moment two patterns share a
// driver — tasks-per-project AND bugs-per-project both want a segment from
// `project`. A map keyed by name makes that collision impossible by
// construction; a list would only catch it in validation.
type CalDAVDynamicCollection struct {
	// CalDAVCollection carries the whole mapping — entity_type, summary, due,
	// completion, read_only, defaults, on_delete. Embedded rather than repeated
	// so a dynamic collection maps EXACTLY like a static one, and a mapping
	// feature added later applies to both without a second implementation.
	CalDAVCollection `yaml:",inline" json:",inline"`
	// DriverType is the entity type whose instances become collections. One
	// collection per readable entity of this type.
	DriverType string `yaml:"driver_type" json:"driver_type"`
	// Relation is the relation type linking a member to its driver entity.
	//
	// Serves BOTH directions, which is why there is no separate create block:
	// outbound it selects members ("tasks with `belongs-to` → this project"),
	// and inbound a client-created to-do gets exactly this edge. Without the
	// inbound half a new to-do would land in the entity type but in NO
	// collection, and vanish from the client on the next sync.
	Relation string `yaml:"relation" json:"relation"`
	// Direction is the member→driver edge direction, defaulting to outgoing
	// (the member points AT the driver, e.g. task --belongs-to--> project).
	Direction Direction `yaml:"direction,omitempty" json:"direction,omitempty"`
	// OnUnlink decides what a DELETE means when it removes the entity's LAST
	// membership. See [CalDAVUnlinkPolicy]; empty means [CalDAVUnlinkAuto].
	OnUnlink CalDAVUnlinkPolicy `yaml:"on_unlink,omitempty" json:"on_unlink,omitempty"`
}

// CalDAVCollection declares one CalDAV collection: a single entity type
// projected to a calendar component, and the inverse mapping applied when a
// client writes back.
//
// ONE COLLECTION = ONE ENTITY TYPE = ONE SYMMETRICAL MAPPING. Unlike [Feed],
// there is no sources list and no separate create block: the same declaration
// serves both directions, so the create-target is simply the collection's type.
// This diverges from `feeds:` deliberately, because the protocols differ. ICS is
// one URL per feed and read-only, so [Feed.Sources] is its only way to combine
// entity types into one calendar. CalDAV is one account URL enumerating N
// collections — a client discovers every collection from a single account — so
// an operator who wants tasks AND bugs declares two collections and the user
// still configures the account once.
//
// The payoff is that the mapping is bidirectional by construction: with several
// sources the read mapping would be a union while the write mapping is one
// branch of it, requiring a create block purely to re-state which branch.
//
// Trade accepted: an interleaved mixed-type list is not expressible. Two
// collections give the same visibility with separate colors and independent
// toggling, which is arguably the better default.
type CalDAVCollection struct {
	// Meta is optional collection-level metadata (display name, color).
	Meta FeedMeta `yaml:"meta,omitempty" json:"meta,omitzero"`
	// Component is the calendar component this collection carries: "vtodo"
	// (default) or "vevent". A collection advertises exactly one, because
	// Apple's clients segregate by component set — Reminders binds only to a
	// VTODO collection and Calendar.app creates its own separate VEVENT one, so
	// a mixed collection is invisible to one of them.
	Component string `yaml:"component,omitempty" json:"component,omitempty"`
	// EntityType is the entity type this collection projects, and the type an
	// inbound create constructs. Required; validated at load.
	EntityType string `yaml:"entity_type" json:"entity_type"`
	// Where is a list of filter clauses, all ANDed, in the internal/filter
	// language. Empty selects every entity of the type.
	Where []string `yaml:"where,omitempty" json:"where,omitempty"`
	// Due names the date- or datetime-typed property mapped to the entry's
	// deadline (VTODO DUE / VEVENT DTSTART). Optional: a to-do without a
	// deadline is legal and common.
	Due string `yaml:"due,omitempty" json:"due,omitempty"`
	// Summary names the property mapped to SUMMARY. Optional; defaults to the
	// entity type's display property.
	Summary string `yaml:"summary,omitempty" json:"summary,omitempty"`
	// Description names an optional property mapped to DESCRIPTION, or the
	// sentinel [CalDAVDescriptionBody] to map the entity's markdown BODY.
	//
	// The body is usually the right target. DESCRIPTION is the one free-text,
	// multi-line field a to-do has, and the body is where rela puts multi-line
	// prose — a `string` property is a single-line form field everywhere else in
	// the app, so routing a client's notes into one makes the SPA render a
	// paragraph in a text input.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Priority names an optional INTEGER property mapped straight onto the RFC
	// 5545 PRIORITY value (0-9). Mutually exclusive with PriorityMap, which is
	// what a project modeling priority as an enum wants instead.
	Priority string `yaml:"priority,omitempty" json:"priority,omitempty"`
	// PriorityMap maps PRIORITY onto a non-integer property (typically an enum)
	// by BUCKETING the 0-9 range. See [CalDAVPriorityMap].
	PriorityMap *CalDAVPriorityMap `yaml:"priority_map,omitempty" json:"priority_map,omitempty"`
	// Location names an optional string property mapped to LOCATION.
	Location string `yaml:"location,omitempty" json:"location,omitempty"`
	// Categories names an optional property mapped to CATEGORIES. A list-typed
	// property maps element-wise; a string property maps to a single category.
	Categories string `yaml:"categories,omitempty" json:"categories,omitempty"`
	// Start names an optional date- or datetime-typed property mapped to
	// DTSTART — when work on a to-do begins, as against the DUE deadline.
	Start string `yaml:"start,omitempty" json:"start,omitempty"`
	// Rrule is an optional recurrence rule, in the same two forms `feeds:`
	// accepts: a literal RFC 5545 rule ("FREQ=WEEKLY") or a bare property name
	// whose per-entity value supplies one. Read-only — see the CalDAV docs.
	Rrule string `yaml:"rrule,omitempty" json:"rrule,omitempty"`
	// ReadOnly names mapped fields whose inbound value is DISCARDED: they are
	// still projected outward, but a client's edit to one never reaches the
	// entity. See [CalDAVReadOnlyFields] for the accepted names.
	//
	// This is a CONTAINMENT control, not an authorization one — the two are
	// orthogonal and compose. ACL answers "may this principal write at all",
	// per type and op, by refusing the request with a 403. ReadOnly answers
	// "may a CalDAV client own this field", per mapped field, by accepting
	// the request and dropping the field. An operator who wants clients to
	// touch nothing should withhold the ACL grant, not populate this list.
	//
	// What it contains is a foreign data model. A CalDAV client is software
	// the operator does not control: it decides what to send and reconstructs
	// the whole VTODO on every edit, so a field it maps badly is rewritten
	// even when the user touched something else. Observed on the wire: Apple
	// Reminders normalizes DTSTART to match DUE on an all-day to-do, and
	// Thunderbird's rich-text notes arrive flattened.
	//
	// A discarded field is not a failed write: the fields that ARE writable
	// apply, and the response carries the entity's real values. Rejecting the
	// whole PUT instead would discard the completion tick the user actually
	// meant, since clients send every field on every edit.
	//
	// Whether the CLIENT then shows the revert is its own choice and they
	// differ — Reminders reverts within seconds, Thunderbird has been seen
	// keeping an optimistic copy across a restart. The guarantee here is that
	// the value never reaches rela, not that the app looks right.
	ReadOnly []string `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	// Completion maps the completion state in both directions. Required for a
	// vtodo collection: without it a client could not check anything off.
	Completion *CalDAVCompletion `yaml:"completion,omitempty" json:"completion,omitempty"`
	// Defaults are literal property values applied when a CLIENT creates an
	// entry. A client-created to-do carries only a summary (verified against
	// Apple Reminders), so this is how a required property that VTODO cannot
	// supply gets a value.
	Defaults map[string]string `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	// OnDelete is the property mutation applied when a client DELETEs an entry.
	// Absent means deletion is refused (403) — see the type doc.
	OnDelete *CalDAVOnDelete `yaml:"on_delete,omitempty" json:"on_delete,omitempty"`
}

// CalDAVCompletion maps a VTODO's completion state to entity properties.
//
// STATUS, COMPLETED and PERCENT-COMPLETE are treated as ONE logical event, not
// three independent property mappings: Apple writes all three together, and RFC
// 4791 §7.8.9's canonical "pending to-dos" filter keys on COMPLETED while a UI
// reads STATUS — so a half-set state reads as done in one client and pending in
// another.
type CalDAVCompletion struct {
	// StatusProperty names the entity property holding completion state.
	// Required.
	StatusProperty string `yaml:"status_property" json:"status_property"`
	// CompletedValue is the StatusProperty value meaning "done". Required, and
	// validated against the property's enum when it has one.
	CompletedValue string `yaml:"completed_value" json:"completed_value"`
	// PendingValue is the value an inbound un-completion restores. Required.
	PendingValue string `yaml:"pending_value" json:"pending_value"`
	// CompletedAt optionally names a datetime property that receives the
	// COMPLETED timestamp. Omit to discard it.
	CompletedAt string `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
}

// CalDAVPriorityMap maps the RFC 5545 PRIORITY integer onto a property whose
// values are not integers — typically an enum like low/normal/high.
//
// # Why buckets rather than exact values
//
// PRIORITY is an integer 0-9 (RFC 5545 3.8.1.9: 1-4 high, 5 normal, 6-9 low,
// 0 undefined), but clients expose three or four labels and pick their own
// number inside each band. Verified on the wire: Thunderbird sends 1 for its
// "Hoog" and Apple Reminders sends 9 for its "Laag". An exact-value table would
// therefore have to enumerate every integer a client might choose, and would
// silently drop the ones it missed.
//
// So each entry claims a RANGE. Inbound, the first bucket containing the
// received value wins. Outbound, the bucket's Value is emitted — the number a
// client will map back to the same label.
type CalDAVPriorityMap struct {
	// Property names the entity property holding the priority. Required.
	Property string `yaml:"property" json:"property"`
	// Buckets map ranges of the 0-9 PRIORITY space onto property values.
	// Required and non-empty; validated for range sanity and enum membership.
	Buckets []CalDAVPriorityBucket `yaml:"buckets" json:"buckets"`
}

// CalDAVPriorityBucket is one band of the PRIORITY range.
type CalDAVPriorityBucket struct {
	// Value is the property value this band means, e.g. "high".
	Value string `yaml:"value" json:"value"`
	// From and To bound the band inclusively, within 0-9. A single-number band
	// sets both to the same value.
	From int `yaml:"from" json:"from"`
	To   int `yaml:"to" json:"to"`
	// Emit is the number rendered outbound for this value. Defaults to From,
	// which is the strongest number in the band — a client shown "high" should
	// see the value most likely to round-trip as high elsewhere.
	Emit int `yaml:"emit,omitempty" json:"emit,omitempty"`
}

// EmitValue is the PRIORITY integer to render for this bucket.
func (b CalDAVPriorityBucket) EmitValue() int {
	if b.Emit != 0 {
		return b.Emit
	}
	return b.From
}

// CalDAVOnDelete is the mutation applied when a client deletes an entry.
//
// A CalDAV DELETE maps to a property change rather than an entity delete,
// because the client gesture is a swipe: rela has no soft-delete, and
// DeleteEntity cascades to relations, so a mis-swipe would destroy a graph node
// and its edges. Set Hard to opt into a real delete.
type CalDAVOnDelete struct {
	// Set is the property mutation to apply, e.g. {status: cancelled}.
	Set map[string]string `yaml:"set,omitempty" json:"set,omitempty"`
	// Hard opts into a real entity delete instead of a property mutation.
	// Mutually exclusive with Set.
	Hard bool `yaml:"hard,omitempty" json:"hard,omitempty"`
}

// CalDAV component kinds, as spelled in `component:`.
const (
	CalDAVComponentTodo  = "vtodo"
	CalDAVComponentEvent = "vevent"
)

// CalDAVDescriptionBody is the sentinel `description:` value that maps
// DESCRIPTION to the entity's markdown BODY rather than to a property.
//
// A reserved word rather than a separate `description_body: true` key, so the
// mapping stays one line with one meaning. The cost is that a property genuinely
// named "body" is not addressable this way; the config validation says so out
// loud rather than silently preferring one reading.
const CalDAVDescriptionBody = "body"

// The field names accepted in `read_only:`. These are the MAPPING's names —
// the YAML keys of [CalDAVCollection] — not entity property names and not
// iCalendar property names.
//
// Naming the mapping is what makes the rule survive a config edit: an operator
// who repoints `due: deadline` at another property does not have to remember to
// update a read-only list that named `deadline`. It also means one name covers
// both spellings of a mapping — `priority` covers `priority_map` too, and
// `description` covers the body sentinel — because from the client's side those
// are one field either way.
const (
	CalDAVFieldSummary     = "summary"
	CalDAVFieldDescription = "description"
	CalDAVFieldDue         = "due"
	CalDAVFieldPriority    = "priority"
	CalDAVFieldLocation    = "location"
	CalDAVFieldCategories  = "categories"
	CalDAVFieldStart       = "start"
	CalDAVFieldCompletion  = "completion"
)

// CalDAVReadOnlyFields lists every name `read_only:` accepts, in the order the
// config validation reports them.
//
// `rrule` is absent deliberately: it is already read-only in every
// configuration, so listing it would imply the others are writable by contrast
// and that naming it changes something. `uid` and `url` are likewise absent —
// neither is a mapped field.
var CalDAVReadOnlyFields = []string{
	CalDAVFieldSummary,
	CalDAVFieldDescription,
	CalDAVFieldDue,
	CalDAVFieldPriority,
	CalDAVFieldLocation,
	CalDAVFieldCategories,
	CalDAVFieldStart,
	CalDAVFieldCompletion,
}

// IsReadOnly reports whether inbound writes to a mapped field are discarded.
//
// Case-insensitive, matching how the rest of the config treats operator-typed
// identifiers, so `read_only: [Summary]` behaves as written rather than
// silently doing nothing.
func (c CalDAVCollection) IsReadOnly(field string) bool {
	for _, f := range c.ReadOnly {
		if strings.EqualFold(strings.TrimSpace(f), field) {
			return true
		}
	}
	return false
}

// ComponentOrDefault returns the collection's component, defaulting to vtodo.
// VTODO is the default because a VEVENT projection is already served read-only
// by `feeds:`; a CalDAV collection exists primarily to make to-dos writable.
func (c CalDAVCollection) ComponentOrDefault() string {
	if c.Component == "" {
		return CalDAVComponentTodo
	}
	return c.Component
}
