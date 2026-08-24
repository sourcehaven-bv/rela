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
	ID           string              `json:"id"`
	Type         string              `json:"type"`
	Title        string              `json:"_title,omitempty"`
	Properties   map[string]any      `json:"properties"`
	Content      string              `json:"content,omitempty"`
	Relations    map[string][]string `json:"relations,omitempty"`
	Included     map[string]Entity   `json:"included,omitempty"`
	Self         string              `json:"_self,omitempty"`
	Actions      map[string]bool     `json:"_actions,omitempty"`
	Inaccessible []InaccessibleField `json:"inaccessible,omitempty"`
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
	// Redacted names the properties withheld from `Properties` by
	// field-level ACL (`visible:`) on THIS response (DEC-T0XIWQ). It is the
	// field-level sibling of Inaccessible, which says the same thing
	// ("exists, value unreadable") for git-crypt-locked content.
	//
	// It exists because absence from `Properties` is ambiguous — a key can be
	// missing because it was redacted OR because it was never set — and a
	// WRITE surface has to tell those apart to know which inputs it may
	// offer. Read-out surfaces are happy to conflate them; an edit form is
	// not. Clients MUST NOT infer redaction from absence: consult this list.
	//
	// Disclosure boundary: this leaks property NAMES, never VALUES. That is
	// not a new disclosure — the metamodel endpoint already serves the
	// declared property names per type, and `visible:` redaction is defined
	// as hiding values only, making no claim to conceal which properties
	// exist. Row-level ACL is unaffected: whether an ENTITY exists remains a
	// genuine secret, and this list only ever rides a response the caller was
	// already authorized to read.
	//
	// Same pointer / closed-world semantics as FieldAffordances: present
	// (possibly empty) on per-entity responses — `[]` meaning "evaluated,
	// nothing redacted" — and nil on list rows and other non-per-entity
	// shapes, which carry no write affordances. Names are sorted for a
	// deterministic wire.
	Redacted *[]string `json:"_redacted,omitempty"`
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
	// World carries the PROVENANCE of the face in this response: which world
	// resolved it, which coordinate it was stored at, and which resolution
	// rule chose it (TKT-WRLDAPI item 2).
	//
	// It exists because the bytes alone do not say. Under a world with a
	// fallback chain, "the Dutch face" and "the English face, because no
	// Dutch face exists" arrive byte-identically — same id, same type, same
	// shape — and a client rendering a publication badge, offering a
	// translate affordance, or deciding whether an edit is safe has to tell
	// them apart. Re-deriving it client-side would require the chain, the
	// per-type overrides and the fallback policy, which is a second
	// implementation of the semantics that decide which face a reader sees.
	//
	// Present on the single-entity GET ONLY — deliberately NOT on every
	// per-entity response the way FieldAffordances is. The distinction is
	// worth stating because a client will otherwise look for it on a PATCH:
	//
	//   - Writes are refused a `?world=` outright (worlds are read-only on
	//     this API), so a create / update / clone response is default-world
	//     by construction and a provenance block would be noise.
	//   - A history snapshot is rebuilt from a stored version and carries no
	//     pointer, so labeling it would state a coordinate the code cannot
	//     back — the "affordance map that lies" failure, one layer down.
	//   - Views, attachments and restore are not world-capable routes.
	//
	// The single-entity GET is the only response whose pointer is the
	// store's actual answer to a world query, which is why it is the only
	// one that carries this.
	World *EntityWorld `json:"_world,omitempty"`
	// Transitions maps a state-machine-typed property name to the LIST of its
	// outgoing transitions resolved for the requesting principal on this entity
	// (TKT-3G93B8): each carries the target value, an optional action label,
	// the guard permission (if any), whether the principal may perform it right
	// now, and — when not — which gate blocked it. Only properties whose type is
	// a state machine appear; a plain enum field has no entry, and the SPA falls
	// back to the ordinary enum control. Same pointer / closed-world semantics as
	// FieldAffordances: present (possibly empty) on every per-entity response
	// (GET / PATCH / POST create / clone) when the resolver can answer
	// transitions, nil on list rows and when no state machines are wired. It is a
	// UI hint, never authorization — the write path re-enforces every transition.
	Transitions *map[string][]Transition `json:"_transitions,omitempty"`
	// Copies lists the declared copy definitions available FROM this entity's
	// current face — the promote / translate affordances (RULING 9).
	//
	// It rides the entity response alongside [Entity.Actions] because that is
	// how every other affordance in this app is delivered: computed at read
	// time from data the client already fetched, rather than requiring a
	// second request keyed on (type, pointer, id). An earlier revision shipped
	// a `GET /_copies` list endpoint; it was removed for exactly that reason.
	//
	// Same contract as `_actions`: a UI HINT, never a boundary. `POST
	// /_copies/{name}` re-authorizes through the kernel, so a client that
	// ignores `allowed` and posts anyway gets the same refusal it would have
	// got regardless. Each entry's verdict is computed by running the kernel's
	// own authorization path, not a re-derivation, so it cannot drift from
	// what the write does (RULING 11 — an affordance map that lies is a trap).
	//
	// SAME-ENTITY definitions only: a copy that creates a DIFFERENT entity has
	// no target id until the caller names one, so no honest verdict exists for
	// it here. The kernel still supports those; they are simply not offered as
	// an affordance. See entitymanager.CopiesForSource.
	//
	// Same pointer / closed-world semantics as FieldAffordances: present
	// (possibly empty) on per-entity responses, nil on list rows.
	Copies *[]CopyOffer `json:"_copies,omitempty"`
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

// Transition is one resolved outgoing move of a state-machine-typed property
// on a per-entity response (TKT-3G93B8). It is the wire projection of
// statemachine.TransitionVerdict.
//
// To is the target value. Label is optional display text for the MOVE (the
// action, e.g. "Start progress"); when empty the SPA falls back to the target
// value's display label. Guard is the ACL permission the move requires (empty
// when unguarded). Allowed reports whether the requesting principal may perform
// it now (guard held AND precondition met). Reason names the blocking gate when
// Allowed is false ("guard" or "precondition"); empty when Allowed is true. The
// SPA shows only Allowed transitions, so Reason is advisory (tooltip/CLI), not a
// rendering gate.
type Transition struct {
	To      string `json:"to"`
	Label   string `json:"label,omitempty"`
	Guard   string `json:"guard,omitempty"`
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
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

// EntityWorld is the provenance of a resolved face — see [Entity.World] for
// why a client needs it.
//
// It describes the RESOLUTION that produced this response, never the entity's
// other faces. A client learns which coordinate it is looking at; it learns
// nothing about which coordinates exist for this id. That distinction is the
// existence oracle the read gate closes, and this shape stays on the safe
// side of it deliberately.
type EntityWorld struct {
	// Name is the world this response was resolved in — `default` for the
	// implicit total world. Matches a key of [Schema.Worlds].
	Name string `json:"name"`
	// Pointer is the coordinate the served state was stored at. EMPTY means
	// the default state, which is not the same claim as "the default world":
	// a fallback under `otherwise: default` also serves the default state.
	// Via is what separates those, which is why Pointer alone is not enough.
	Pointer string `json:"pointer"`
	// Via names the resolution rule that chose this face:
	//
	//   - "unscoped"         — this world applies no per-type resolution, so
	//                          the entity contributes its default state.
	//                          Covers BOTH a type that declares no content
	//                          states and the total default world, which
	//                          resolves everything this way — the default
	//                          world is exactly "no resolution applied", and
	//                          worldreader.Rule reports it identically.
	//   - "chain"            — a coordinate the world SELECTS exists; this is
	//                          the face the world asked for.
	//   - "fallback-default" — no selected coordinate exists, and the world's
	//                          `otherwise: default` stood the default state
	//                          in. The reader is seeing a substitute.
	//
	// The third value is the load-bearing one: it is the difference between
	// "published" and "not published, showing you the draft".
	//
	// There is deliberately no "excluded" value. A world that excludes an
	// entity produces no response to carry provenance on — it is a 404,
	// indistinguishable from a genuine miss, because existence in a world IS
	// the publication bit.
	Via string `json:"via"`
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
	// Worlds enumerates the declared worlds a client may pass to `?world=`,
	// keyed by world name, plus the implicit `default` world (TKT-WRLDAPI).
	//
	// Present even for a project declaring no `worlds:` block, where it holds
	// the single `default` entry: a client needs to distinguish "this server
	// has no other worlds" from "this server is too old to tell me", and an
	// omitted key cannot say the first.
	Worlds map[string]World `json:"worlds,omitempty"`
}

// World is the JSON representation of one declared world — a named
// resolution function picking at most one content state per entity (design
// doc §4.1).
//
// # The declared set is NOT filtered per principal
//
// Every world the schema declares appears here for every caller. World names
// are operator-authored config living in `schema.yaml`, routinely a public
// repo, so their contents are already disclosed and CLAUDE.md is explicit
// that code must not contort to conceal a config name. What IS per-principal
// is [World.Readable] — which says whether this caller may SELECT the world,
// never whether it exists.
//
// The distinction is load-bearing: what a world CONTAINS is secret (a denied
// world serves an empty result, never a 403, precisely so it cannot be told
// apart from a world holding nothing), while the fact that the operator
// declared a world named `published` is not.
//
// This carries no per-ENTITY information. Which faces a given entity has is
// exactly the existence oracle the read gate closes, and nothing here
// narrows it: a type's declared coordinates ([EntityType.Pointers]) say what
// the schema permits, never what any row holds.
// Note the field set is deliberately incomplete: a world's `edits:` key
// (which state edits made from it land in) is NOT surfaced. It is a
// write-side concept, and the affordance a client actually needs — "which
// faces can I create from here" — is a separate gated query over the
// declared copy definitions (Ruling 9), not something derivable from a world.
type World struct {
	// Select is the ordered candidate chain: the first coordinate that
	// EXISTS for an entity is the face served. Empty for the default world,
	// which resolves every entity to its default state by construction.
	Select []string `json:"select,omitempty"`
	// Overrides replaces Select for the named entity types, keyed by type.
	Overrides map[string][]string `json:"overrides,omitempty"`
	// Otherwise is the rule-3 policy for a type that declares pointers but
	// none this world selects: `exclude` (contributes nothing) or `default`
	// (falls back to the type's default state). Empty for the default world,
	// which never reaches rule 3.
	Otherwise string `json:"otherwise,omitempty"`
	// Readable reports whether THIS caller may select the world via
	// `?world=`. False means a request naming it is served an empty result
	// rather than a 403 — so a client that respects this flag shows the user
	// an honest selector instead of a world that silently returns nothing.
	//
	// A UI hint, never a boundary: the server re-checks the grant on every
	// request (`resolveWorld`), so a client ignoring this learns nothing it
	// could not learn by asking.
	//
	// NO omitempty, unlike Default below, and the asymmetry is deliberate:
	// `false` is the load-bearing answer here — it is precisely what a
	// selector needs — and an omitted key would be indistinguishable from a
	// server too old to compute it. `default: false`, by contrast, is noise
	// on every declared world.
	Readable bool `json:"readable"`
	// Default marks the implicit default world — today's graph, total by
	// construction, always present and always selectable. Spelled as a flag
	// rather than left for the client to infer from the reserved name, so a
	// selector can label it without hardcoding the string.
	Default bool `json:"default,omitempty"`
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
	// Pointers declares this type's content-state coordinates, keyed by
	// coordinate name ("draft", "published", "en", "nl"). ABSENT for the
	// common case of a type with no content states, which contributes its
	// single default state to every world.
	//
	// This is SCHEMA, not data: it says which coordinates the operator
	// declared for the type, never which faces any particular entity holds.
	// Two entities of the same type report identical pointers whether one has
	// a published face and the other does not.
	//
	// DO NOT add the per-entity variant here — "which faces does THIS id
	// have" is the existence oracle the row gate exists to close, since
	// existence in a world IS the publication bit. An "other faces" indicator
	// is a real product need (Ruling 9 item 8), but it has to omit faces the
	// viewer may not read, which makes it a gated per-entity query rather
	// than a field on the type's schema.
	Pointers map[string]Pointer `json:"pointers,omitempty"`
}

// Pointer is the JSON representation of one declared content state.
//
// Deliberately near-empty, mirroring metamodel.PointerDef: a state is
// identified by its coordinate and nothing else. Per-state knobs (labels,
// retention) are not added speculatively.
type Pointer struct {
	// Default marks the coordinate stored under the ZERO pointer — the
	// state a bare id addresses. At most one per type.
	Default bool `json:"default,omitempty"`
}

// PropertyDef is the JSON representation of a property definition.
type PropertyDef struct {
	Type        string            `json:"type"`
	Required    bool              `json:"required"`
	Default     string            `json:"default,omitempty"`
	Values      []string          `json:"values,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"` // Display labels keyed by value; value stays the wire identity
	Description string            `json:"description,omitempty"`
	List        bool              `json:"list,omitempty"`
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
	Values  []string          `json:"values"`
	Labels  map[string]string `json:"labels,omitempty"` // Display labels keyed by value; value stays the wire identity
	Default string            `json:"default,omitempty"`
}

// Config is the JSON representation of the UI config.
type Config struct {
	App AppConfig `json:"app"`
	// AboutDescription is the deployment description shown by the SPA's global
	// "About" help (TKT-DUQBD0): the data-entry.yaml `app.description`, falling
	// back to the metamodel's top-level `description`. Distinct from
	// AppConfig.Description (which SettingsView renders as a plain one-liner) so
	// a multi-paragraph markdown metamodel description doesn't leak into that
	// view. Empty → the About button is hidden.
	AboutDescription string                                      `json:"about_description,omitempty"`
	Styles           map[string]map[string]string                `json:"styles"`
	Forms            map[string]dataentryconfig.Form             `json:"forms"`
	Lists            map[string]dataentryconfig.List             `json:"lists"`
	Views            map[string]dataentryconfig.ViewConfig       `json:"views"`
	EntityViews      map[string]dataentryconfig.EntityViewConfig `json:"entity_views,omitempty"`
	Kanbans          map[string]dataentryconfig.Kanban           `json:"kanbans"`
	Dashboard        *dataentryconfig.DashboardConfig            `json:"dashboard,omitempty"`
	Actions          map[string]dataentryconfig.Action           `json:"actions,omitempty"`
	Navigation       []dataentryconfig.NavigationEntry           `json:"navigation"`
	Documents        map[string]dataentryconfig.DocumentConfig   `json:"documents,omitempty"`
	Apps             map[string]App                              `json:"apps,omitempty"`
	Palette          *dataentryconfig.ResolvedPalette            `json:"palette,omitempty"`

	// NextActionBands is the operator's ordered priority vocabulary, so the
	// SPA can label a suggestion's band ("Someone is waiting") rather than
	// echoing a raw id.
	//
	// The SOURCES are deliberately NOT here. A suggestion arrives fully
	// resolved from /_next_action — message already interpolated, affordances
	// attached — so the SPA never needs the rules, and shipping them would
	// invite a client-side re-implementation of the engine. Same reasoning as
	// "no useACL() composable": the SPA renders what the server computed.
	NextActionBands []dataentryconfig.NextActionBand `json:"next_action_bands,omitempty"`
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
// Span is the field's width on the 12-column layout grid (0 = full width).
//
// Render is the already-resolved render mode ("display" | "input", TKT-HOIX1),
// set server-side from the section + field config. View sections and cards/list
// rows honor it; the side-panel renderer does not implement inline edit today
// and ignores it.
//
// Field order and types must stay in lockstep with dataentry.SectionFieldData:
// the handlers convert between them with a direct struct conversion, so the
// compiler is what keeps the wire surface and the internal DTO from drifting.
type SectionField struct {
	Property     string   `json:"property,omitempty"`
	Label        string   `json:"label"`
	Values       []string `json:"values,omitempty"`
	PropType     string   `json:"propType,omitempty"`
	Inaccessible bool     `json:"inaccessible,omitempty"`
	Span         int      `json:"span,omitempty"`
	Render       string   `json:"render,omitempty"`
	Widget       string   `json:"widget,omitempty"`
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

// DashboardResponse contains the dashboard page config with the cards this
// principal may see (TKT-53KICM).
//
// This exists as a separate endpoint from `_config` because the card list is
// per-principal while `_config` is deliberately identical for everyone (root
// CLAUDE.md, "The configuration is not a secret; the data is"). `_config` still
// carries the full `dashboard:` block; only this response is filtered, and only
// so a user is not offered a card they cannot act on.
//
// Cards is always a non-nil slice: a project with no `dashboard:` configured,
// one with an empty `cards:`, and one where every card was filtered all
// serialize as `[]`, so the SPA has a single "render what you got" path.
type DashboardResponse struct {
	Title       string                          `json:"title,omitempty"`
	Description string                          `json:"description,omitempty"`
	Cards       []dataentryconfig.DashboardCard `json:"cards"`
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

	// InlineCreate maps an entity type to the form id the SPA should use
	// to create one inline from a relation field (TKT-OMUD56). A type
	// appears ONLY when both conditions hold: the principal may create it,
	// and a create form resolves for it. So presence alone is the offer —
	// the client needs no second lookup and no permission arithmetic.
	//
	// Sending the resolved form id (rather than letting the client find
	// its own) keeps `createFormForType`'s ordering authoritative in one
	// place; the natsort-and-prefer-non-edit rule is not reimplemented
	// client-side where it could silently diverge.
	//
	// It rides on the sidebar because this is the one boot-time payload
	// that is already principal-scoped: `_config` is pinned
	// principal-INDEPENDENT (TestNavPermission_ConfigUnfiltered) and
	// `_schema` is a pure metamodel projection.
	//
	// A UI hint, never authorization: POST /api/v1/{plural} re-authorizes,
	// so a stale or forged map can only surface a button that then 403s.
	InlineCreate map[string]string `json:"inline_create,omitempty"`
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
	Name       string             `json:"name"`
	Properties map[string]any     `json:"properties"`
	Content    string             `json:"content"`
	Relations  []TemplateRelation `json:"relations"`
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
	// World is the PROVENANCE of this collection entity's face — which world
	// resolved it, which coordinate it was stored at, and which rule chose it
	// (TKT-WRLDAPI item 4b).
	//
	// This is the slot the per-neighbor provenance deferred in item 4 belongs
	// in, and the reason 4b exists. A view collection carries whole ENTITIES,
	// unlike the entity GET's `relations` map, which carries bare id strings
	// and so had nowhere to put this without a wire-type change.
	//
	// It lets a client distinguish "the Dutch page" from "the English page,
	// because no Dutch face exists" for each item in a collection — the same
	// question [Entity.World] answers for a single entity, which arrives
	// byte-identically either way and cannot be re-derived client-side without
	// the chain and the fallback policy.
	//
	// Present only under a NON-DEFAULT world. Under the default world every
	// entity resolves to its default state by definition, so a provenance
	// block would be noise on every row of every existing view.
	World *EntityWorld `json:"_world,omitempty"`
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

// CopyOffer is one declared copy definition offered for a source face
// (TKT-WRLDAPI item 5), as the SPA needs it to render RULING 9's affordances.
type CopyOffer struct {
	// Name is the `copies:` key and the ONLY thing a client may send back to
	// invoke it. A request names a definition; it never supplies one.
	Name string `json:"name"`
	// Label is the operator-configured display text, falling back to Name.
	// Display-only.
	Label string `json:"label"`
	// TargetFace is the declared target (`policy@published`, `new page`), for
	// a UI that wants to say what the action will produce.
	TargetFace string `json:"targetFace"`
	// SameEntity distinguishes a promote/revise (another face of the SAME
	// entity) from a copy that creates a different entity. Not cosmetic: a
	// cross-entity copy REQUIRES a target id, so a client cannot build a valid
	// invoke without knowing which it has.
	SameEntity bool `json:"sameEntity"`
	// Allowed reports whether the requesting principal may invoke this copy on
	// this source right now.
	//
	// A HINT, never a boundary — the same contract as `_actions`. The invoke
	// endpoint re-authorizes through the kernel, so a client that ignores this
	// and POSTs anyway receives the same 403 it would have received regardless.
	// It is computed by running the kernel's own authorization path, not a
	// parallel implementation, so it cannot drift from what the write does.
	Allowed bool `json:"allowed"`
	// Reason names why Allowed is false, for a tooltip. Empty when allowed.
	// Advisory, and never carries content from an entity the caller cannot
	// read.
	Reason string `json:"reason,omitempty"`
}

// CopyOffersResponse wraps the offers for one face.
type CopyOffersResponse struct {
	Data []CopyOffer `json:"data"`
}

// CopyResult reports what an invoked copy produced.
type CopyResult struct {
	// Definition is the name that was invoked.
	Definition string `json:"definition"`
	// EntityID is the entity whose face was written.
	EntityID string `json:"entityId"`
	// Pointer is the coordinate of the face that was written. Empty means the
	// default state.
	Pointer string `json:"pointer"`
	// Created is true when the copy brought the target face into existence
	// rather than overwriting one — the difference between "published for the
	// first time" and "re-published".
	Created bool `json:"created"`
}
