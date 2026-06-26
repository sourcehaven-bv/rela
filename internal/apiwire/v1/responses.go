package v1

import (
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Warning is the wire alias for a domain soft-validation warning, surfaced on
// mutation responses (DEC-HWZHA write-with-warnings).
type Warning = entity.Warning

// Entity is the JSON representation of an entity for API v1.
type Entity struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Title        string                 `json:"_title,omitempty"`
	Properties   map[string]interface{} `json:"properties"`
	Content      string                 `json:"content,omitempty"`
	Relations    map[string][]string    `json:"relations,omitempty"`
	Included     map[string]Entity      `json:"included,omitempty"`
	Self         string                 `json:"_self,omitempty"`
	Actions      map[string]bool        `json:"_actions,omitempty"`
	Inaccessible []InaccessibleField    `json:"inaccessible,omitempty"`
	// FieldAffordances carries per-field write affordances on per-entity
	// GET responses. Sparse: only fields whose verdict deviates from the
	// permissive default appear. Hidden fields are omitted from
	// `Properties` AND from this map entirely. Pointer semantics
	// distinguish "absent on the wire" (nil pointer; list / mutation
	// responses) from "present and empty" (`{}`; per-entity GET with no
	// deviations under nop resolver — closed-world signal matching the
	// `_actions` precedent).
	FieldAffordances *map[string]FieldAffordance `json:"_fields,omitempty"`
	// RelationAffordances carries per-relation-type affordances on
	// per-entity GET responses. Same pointer / closed-world semantics
	// as FieldAffordances.
	RelationAffordances *map[string]RelationAffordance `json:"_relations,omitempty"`
	// Attachments maps a `file`-type property name to the LIST of files
	// currently attached to it (a property may hold several when its
	// metamodel `max` > 1). The value is always an array — even a
	// single-attachment property reports a 1-element list — matching how
	// rela's `list:` properties and `_relations` are always arrays. Only
	// properties that actually carry a file appear. Same pointer /
	// closed-world semantics as FieldAffordances: present (possibly empty)
	// on every per-entity response (GET, PATCH, POST create, clone — the
	// ones that run serializeEntityForWire), nil on list rows and other
	// non-per-entity shapes. The SPA's file widget reads this to render the
	// download links / previews instead of the raw stored path string(s).
	Attachments *map[string][]Attachment `json:"_attachments,omitempty"`
	// Warnings lists soft-condition findings surfaced by the write
	// path. Populated only by mutation responses (PATCH); read paths
	// leave it nil. Each warning has a stable `code`, an RFC 6901
	// JSON Pointer `path`, and a human-readable `detail`.
	Warnings []Warning `json:"warnings,omitempty"`
}

// FieldAffordance describes per-field write / option affordances on
// the wire. Sparse: `Writable` is nil when the default (writable)
// holds; `Options` lists only the false entries (allowed options are
// implicit via the metamodel). See the closed-world contract in
// docs/data-entry/api-reference.md.
type FieldAffordance struct {
	Writable *bool           `json:"writable,omitempty"`
	Options  map[string]bool `json:"options,omitempty"`
}

// RelationAffordance describes per-relation-type affordances on the
// wire. Sparse: `Creatable` and `Removable` are nil when the default
// (true) holds. `Fields` lists meta-field writability overrides, also
// sparse.
type RelationAffordance struct {
	Creatable *bool                      `json:"creatable,omitempty"`
	Removable *bool                      `json:"removable,omitempty"`
	Fields    map[string]FieldAffordance `json:"fields,omitempty"`
}

// Attachment describes one file attached to a `file`-type property, as
// surfaced on a per-entity GET response. ID is the file's identifier
// within the property (its normalized file name) — used to build the
// per-file download/delete URL. Href is the download URL for the bytes (an
// ACL-gated endpoint that inherits the owning entity's read permission).
// ContentType is inferred from the filename — the store does not persist
// it on every backend.
type Attachment struct {
	ID          string `json:"id"`
	FileName    string `json:"filename"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Href        string `json:"href"`
}

// InaccessibleField describes a property that is known to exist but
// whose value is unreadable by the holder of the entity (e.g. the file
// is git-crypt encrypted and the key is not present locally).
type InaccessibleField struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ListResponse is the response for listing entities.
type ListResponse struct {
	Data    []Entity        `json:"data"`
	Meta    ListMeta        `json:"meta"`
	Actions map[string]bool `json:"_actions,omitempty"`
}

// ListMeta contains pagination metadata.
type ListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// Schema is the JSON representation of the metamodel.
type Schema struct {
	Entities  map[string]EntityType   `json:"entities"`
	Relations map[string]RelationType `json:"relations"`
	Types     map[string]CustomType   `json:"types,omitempty"`
}

// EntityType is the JSON representation of an entity type.
type EntityType struct {
	Label       string                 `json:"label"`
	Plural      string                 `json:"plural"`
	Description string                 `json:"description,omitempty"`
	Primary     string                 `json:"primary,omitempty"`
	IDType      string                 `json:"id_type,omitempty"`
	IDPrefix    string                 `json:"id_prefix,omitempty"`
	IDPrefixes  []string               `json:"id_prefixes,omitempty"`
	Properties  map[string]PropertyDef `json:"properties"`
}

// PropertyDef is the JSON representation of a property definition.
type PropertyDef struct {
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Values      []string `json:"values,omitempty"`
	Description string   `json:"description,omitempty"`
	List        bool     `json:"list,omitempty"`
	// Max is the attachment cap for a `file` property (default 1). The SPA's
	// file widget reads it to switch between replace-mode and multi-file
	// add-mode. Omitted unless set above 1.
	Max int `json:"max,omitempty"`
}

// RelationType is the JSON representation of a relation type.
type RelationType struct {
	Label       string                 `json:"label"`
	Description string                 `json:"description,omitempty"`
	From        []string               `json:"from"`
	To          []string               `json:"to"`
	Inverse     *InverseDef            `json:"inverse,omitempty"`
	Symmetric   bool                   `json:"symmetric,omitempty"`
	MinOutgoing *int                   `json:"min_outgoing,omitempty"`
	MaxOutgoing *int                   `json:"max_outgoing,omitempty"`
	MinIncoming *int                   `json:"min_incoming,omitempty"`
	MaxIncoming *int                   `json:"max_incoming,omitempty"`
	Properties  map[string]PropertyDef `json:"properties,omitempty"`
	// Orderable, when set, declares that the frontend may offer drag-to-reorder
	// controls on the corresponding side. The managed property names are
	// always the reserved `_order_out` (outgoing) and `_order_in` (incoming).
	Orderable *RelationOrderable `json:"orderable,omitempty"`
}

// RelationOrderable describes per-side orderability for a relation type.
type RelationOrderable struct {
	Outgoing bool `json:"outgoing,omitempty"`
	Incoming bool `json:"incoming,omitempty"`
}

// InverseDef mirrors metamodel.InverseDef on the wire. The SPA reads
// `inverse.id` to find the inverse body key for incoming-direction
// edits routed through the unified PATCH (TKT-GFQK).
type InverseDef struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// CustomType is the JSON representation of a custom type.
type CustomType struct {
	Values  []string `json:"values"`
	Default string   `json:"default,omitempty"`
}

// Config is the JSON representation of the UI config.
type Config struct {
	App         AppConfig                                   `json:"app"`
	Styles      map[string]map[string]string                `json:"styles"`
	Forms       map[string]dataentryconfig.Form             `json:"forms"`
	Lists       map[string]dataentryconfig.List             `json:"lists"`
	Views       map[string]dataentryconfig.ViewConfig       `json:"views"`
	EntityViews map[string]dataentryconfig.EntityViewConfig `json:"entity_views,omitempty"`
	Kanbans     map[string]dataentryconfig.Kanban           `json:"kanbans"`
	Dashboard   *dataentryconfig.DashboardConfig            `json:"dashboard,omitempty"`
	Actions     map[string]dataentryconfig.Action           `json:"actions,omitempty"`
	Navigation  []dataentryconfig.NavigationEntry           `json:"navigation"`
	Documents   map[string]dataentryconfig.DocumentConfig   `json:"documents,omitempty"`
	Apps        map[string]App                              `json:"apps,omitempty"`
	Palette     *dataentryconfig.ResolvedPalette            `json:"palette,omitempty"`
}

// App is the client-facing view of a custom app. It deliberately omits the
// on-disk File path and the csp_origins allow-list — the SPA only needs enough
// to render a sidebar entry and route to /app/{id}; the HTML is fetched from
// GET /api/v1/_apps/{id}.
type App struct {
	Title       string `json:"title,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// AppConfig is the JSON representation of the app config.
type AppConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// PlantUMLServerURL is the configured PlantUML server base URL, or empty
	// when PlantUML rendering is disabled. The SPA treats a non-empty value as
	// the on switch for ```plantuml diagram rendering.
	PlantUMLServerURL string `json:"plantuml_server_url,omitempty"`
}

// Error is an RFC 7807 Problem Details response.
type Error struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Detail   string       `json:"detail,omitempty"`
	Instance string       `json:"instance,omitempty"`
	Errors   []FieldError `json:"errors,omitempty"`
}

// FieldError represents a validation error on a specific field.
type FieldError struct {
	Source ErrorSource `json:"source"`
	Code   string      `json:"code"`
	Detail string      `json:"detail"`
}

// ErrorSource points to the location of an error.
type ErrorSource struct {
	Pointer string `json:"pointer"`
}

// SidePanelSection represents a section in the side panel response.
type SidePanelSection struct {
	Heading      string            `json:"heading"`
	SectionID    string            `json:"sectionId"`
	Display      string            `json:"display"`
	IsEmpty      bool              `json:"isEmpty"`
	EmptyMessage string            `json:"emptyMessage,omitempty"`
	Fields       []SectionField    `json:"fields,omitempty"`
	Entities     []SidePanelEntity `json:"entities,omitempty"`
	AddInfo      *ViewAddInfo      `json:"addInfo,omitempty"`
	LinkInfo     *ViewLinkInfo     `json:"linkInfo,omitempty"`
}

// SectionField represents a field in a side panel section.
// Values is always an array so that list-typed properties retain per-item
// structure; scalar properties become a one-element array. Empty fields emit
// an empty array (omitted via omitempty when nil).
//
// Property carries the raw property name so consumers can correlate the
// field with metamodel data (e.g. inaccessibility lookup); Label is the
// human-readable rendering. Inaccessible is true when the underlying entity
// is git-crypt encrypted — the field is known to exist in the schema but
// its value cannot be read.
type SectionField struct {
	Property     string   `json:"property,omitempty"`
	Label        string   `json:"label"`
	Values       []string `json:"values,omitempty"`
	PropType     string   `json:"propType,omitempty"`
	Inaccessible bool     `json:"inaccessible,omitempty"`
}

// SidePanelEntity represents an entity in a side panel section.
type SidePanelEntity struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Type       string         `json:"type"`
	EditFormID string         `json:"editFormId,omitempty"`
	Fields     []SectionField `json:"fields,omitempty"`
	Content    string         `json:"content,omitempty"`
	HasContent bool           `json:"hasContent"`
}

// SidebarItem represents a navigation item with count.
type SidebarItem struct {
	Label  string `json:"label"`
	Href   string `json:"href"`
	Icon   string `json:"icon,omitempty"`
	Count  *int   `json:"count,omitempty"`
	Action string `json:"action,omitempty"`
}

// SidebarGroup represents a navigation group with items.
type SidebarGroup struct {
	Group     string        `json:"group,omitempty"`
	Collapsed bool          `json:"collapsed,omitempty"`
	Items     []SidebarItem `json:"items"`
}

// SidebarResponse contains the sidebar data with app info and navigation.
type SidebarResponse struct {
	App        AppConfig      `json:"app"`
	Navigation []SidebarGroup `json:"navigation"`
	// LogoURL is the cache-busted URL of the user-uploaded sidebar logo,
	// or nil when no logo is set. Included here (rather than in
	// `_settings`) so the SPA can render the logo on first paint without
	// blocking on a settings fetch.
	LogoURL *string `json:"logoUrl,omitempty"`
}

// ConflictItem represents a conflicted file.
type ConflictItem struct {
	Path        string `json:"path"`
	EntityType  string `json:"entity_type,omitempty"`
	EntityID    string `json:"entity_id,omitempty"`
	MarkerCount int    `json:"marker_count"`
}

// ConflictsResponse contains the list of conflicts.
type ConflictsResponse struct {
	Conflicts []ConflictItem `json:"conflicts"`
	Count     int            `json:"count"`
}

// PropertyDiff represents a property difference.
type PropertyDiff struct {
	Property    string `json:"property"`
	OursValue   string `json:"ours_value"`
	TheirsValue string `json:"theirs_value"`
	IsSame      bool   `json:"is_same"`
}

// ConflictDetail contains detailed info for resolving a conflict.
type ConflictDetail struct {
	Path          string         `json:"path"`
	EntityType    string         `json:"entity_type,omitempty"`
	EntityID      string         `json:"entity_id,omitempty"`
	PropertyDiffs []PropertyDiff `json:"property_diffs"`
	ContentSame   bool           `json:"content_same"`
	ContentOurs   string         `json:"content_ours,omitempty"`
	ContentTheirs string         `json:"content_theirs,omitempty"`
}

// ConflictResolveRequest contains the resolution choices.
type ConflictResolveRequest struct {
	Path            string            `json:"path"`
	PropertyChoices map[string]string `json:"property_choices"`
	ContentChoice   string            `json:"content_choice"`
	ManualContent   string            `json:"manual_content,omitempty"`
}

// DocumentResponse contains the rendered document content.
type DocumentResponse struct {
	HTML      string   `json:"html"`
	Cached    bool     `json:"cached"`
	EntityIDs []string `json:"entity_ids"` // IDs of entities involved in this document (for SSE filtering)
}

// Command is the JSON representation of an available command.
type Command struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Confirm  string `json:"confirm,omitempty"`
	Context  string `json:"context"`
	AutoOpen *bool  `json:"auto_open,omitempty"`
}

// Template represents a template for API responses.
type Template struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
	Content    string                 `json:"content"`
	Relations  []TemplateRelation     `json:"relations"`
}

// TemplateRelation represents a pre-filled relation in a template.
type TemplateRelation struct {
	Relation string `json:"relation"`
	Target   string `json:"target"`
}

// ViewResponse contains the executed view data.
//
// Mentions carries the implicit-relation set discovered by scanning the
// entry and section markdown contents for bare-content entity-ID code
// spans (see collectMentions). The SPA's markdown renderer consumes this
// map to rewrite those code spans into titled in-app links. Mirrors the
// Lua-side `rela.md.entity_refs` shape (TKT-LXYHQ) for SPA consumers
// that don't go through the Lua document path.
//
// Wire stability: `mentions` is part of the public v1 API. The set of
// `inaccessible_reason` values may grow as new locking mechanisms are
// added (today: `git-crypt`); clients must treat unknown reasons as
// opaque rather than enumerating them.
type ViewResponse struct {
	Entry    Entity             `json:"entry"`
	Sections []ViewSection      `json:"sections"`
	Mentions map[string]Mention `json:"mentions,omitempty"`
}

// ViewSection represents a section with resolved data.
type ViewSection struct {
	Heading      string         `json:"heading"`
	SectionID    string         `json:"sectionId"`
	Display      string         `json:"display"`
	IsEmpty      bool           `json:"isEmpty"`
	EmptyMessage string         `json:"emptyMessage,omitempty"`
	Fields       []SectionField `json:"fields,omitempty"`
	Entities     []ViewEntity   `json:"entities,omitempty"`
	Columns      []ViewColumn   `json:"columns,omitempty"`
	Rows         []ViewRow      `json:"rows,omitempty"`
	Groups       []ViewGroup    `json:"groups,omitempty"`
	IsGrouped    bool           `json:"isGrouped"`
	Content      string         `json:"content,omitempty"`
	HasContent   bool           `json:"hasContent"`
}

// ViewEntity represents an entity in a view section.
//
// Props and FieldAffordances (TKT-IHC7D) carry the typed property
// values and per-cell writability verdict that inline-edit hosts on
// cards/list view sections consume. Both are hidden-property-stripped
// — the consumer can assume:
//
//   - `keys(Props) ∩ hidden(e) == ∅` (hidden properties never leak via
//     this surface)
//   - `keys(FieldAffordances) ∩ hidden(e) == ∅` (same for the verdict)
//   - `FieldAffordances` may have keys absent from `Props` when the
//     property has no stored value but a non-default verdict (e.g.
//     `writable: false` on an unset field)
//
// The pointer-to-map idiom on `FieldAffordances` mirrors
// `Entity.FieldAffordances`: `nil` means "absent on the wire"
// (table rows / non-cards paths), `&{}` means "evaluated, no
// deviations" (closed-world signal matching `_actions`).
//
// `Props` is a plain map with `omitempty`: presence/absence is
// sufficient, no closed-world semantic is needed.
type ViewEntity struct {
	ID               string                      `json:"id"`
	Title            string                      `json:"title"`
	Type             string                      `json:"type"`
	EditFormID       string                      `json:"editFormId,omitempty"`
	Fields           []SectionField              `json:"fields,omitempty"`
	Content          string                      `json:"content,omitempty"`
	HasContent       bool                        `json:"hasContent"`
	Props            map[string]any              `json:"_props,omitempty"`
	FieldAffordances *map[string]FieldAffordance `json:"_fields,omitempty"`
}

// ViewColumn represents a column definition.
type ViewColumn struct {
	Property string `json:"property,omitempty"`
	Label    string `json:"label,omitempty"`
	Relation string `json:"relation,omitempty"`
	Link     string `json:"link,omitempty"`
}

// ViewRow represents a table row.
type ViewRow struct {
	EntityID   string     `json:"entityId"`
	EntityType string     `json:"entityType"`
	EditFormID string     `json:"editFormId,omitempty"`
	Cells      []ViewCell `json:"cells"`
	Content    string     `json:"content,omitempty"`
}

// ViewCell represents a table cell.
type ViewCell struct {
	Values     []string `json:"values"`
	PropType   string   `json:"propType,omitempty"`
	Widget     string   `json:"widget,omitempty"`
	Link       string   `json:"link,omitempty"`
	EntityID   string   `json:"entityId,omitempty"`
	EntityType string   `json:"entityType,omitempty"`
}

// ViewGroup represents a group of rows.
type ViewGroup struct {
	GroupName string       `json:"groupName"`
	Rows      []ViewRow    `json:"rows,omitempty"`
	Entities  []ViewEntity `json:"entities,omitempty"`
}

// ViewAddInfo describes an add button configuration. Despite the "View"
// prefix this is now used only by SidePanelSection — see TKT-6ETQ for
// the rename to V1SidePanelAddInfo. Do not reach for this type from a new
// view-related response: the read-only-view invariant established by
// TKT-651W means no view section should carry add affordances.
type ViewAddInfo struct {
	Relation string          `json:"relation"`
	LinkAs   string          `json:"linkAs"`
	PeerID   string          `json:"peerId"`
	Targets  []ViewAddTarget `json:"targets"`
}

// ViewAddTarget represents a possible target for add action.
// Side-panel-only post TKT-651W; see TKT-6ETQ for the rename plan.
type ViewAddTarget struct {
	EntityType string `json:"entityType"`
	FormID     string `json:"formId"`
	Label      string `json:"label"`
}

// ViewLinkInfo describes a link existing button configuration.
// Side-panel-only post TKT-651W; see TKT-6ETQ for the rename plan.
type ViewLinkInfo struct {
	Relation    string   `json:"relation"`
	LinkAs      string   `json:"linkAs"`
	PeerID      string   `json:"peerId"`
	EntityTypes []string `json:"entityTypes"`
}

// PositionRef identifies a neighboring entity in a scope. Type is included
// because a scope (notably a search scope) can span entity types, so the SPA
// must build the target's detail route from *its* type, not the current
// entity's. ID alone would break cross-type prev/next.
type PositionRef struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Position is the scope-navigator payload: the neighbors plus the counter
// the SPA needs, with no entity bodies shipped. current is 1-based; prev/next
// are nil at the ends of the set.
type Position struct {
	Prev    *PositionRef `json:"prev"`
	Next    *PositionRef `json:"next"`
	Current int          `json:"current"`
	Total   int          `json:"total"`
}

// ActionResponse mirrors script.ActionResponse for API JSON output.
// Has both successful response fields and error fields with correlation ID.
type ActionResponse struct {
	Redirect      string `json:"redirect,omitempty"`
	Message       string `json:"message,omitempty"`
	MessageType   string `json:"message_type,omitempty"`
	Error         string `json:"error,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// Mention is the resolved target of an entity-ID code span found in
// markdown content. Mirrors the Lua-side `rela.md.entity_refs`/`resolve_refs`
// semantics (TKT-LXYHQ): only bare-content code spans whose entire text is
// an entity ID are collected; the data-entry SPA uses this map to rewrite
// those code spans into titled in-app links.
//
// `Inaccessible` is true when the entity's display title is unreadable
// (e.g. the file is git-crypt encrypted) — the SPA renders such links
// with a lock affordance using the same tooltip copy as inaccessible
// properties. `InaccessibleReason` carries the matching
// `entity.InaccessibleReason` value as a string so the wire shape stays
// stable across reason-enum additions.
type Mention struct {
	Type               string `json:"type"`
	Title              string `json:"title"`
	Inaccessible       bool   `json:"inaccessible,omitempty"`
	InaccessibleReason string `json:"inaccessible_reason,omitempty"`
}
