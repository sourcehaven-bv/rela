package metamodel

import (
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Metamodel represents the full metamodel configuration
//
// TODO(TKT-N0IKN9): 30 exported methods, over the 20 exported-method line.
// This is the schema accessor — wide read-API by nature — but a ratchet
// candidate: group the type/relation/property lookups behind focused accessors
// (the attachment-scan accessors moved behind [AttachmentPolicy] this way).
//
// 32nd exported method is ShapeProjection (TKT-0C57FS), the data-shape
// sibling of RenderProjection.
//
//plimsoll:max-exported-methods=32
type Metamodel struct {
	Version   string `yaml:"version"`
	Namespace string `yaml:"namespace"`
	// Description is optional end-user prose describing what this deployment /
	// application is for. Display-only, surfaced by the generated documentation
	// (the `rela docs` generator — FEAT-G4VO53). Empty by default; the metamodel
	// does not otherwise consult it.
	Description string                 `yaml:"description,omitempty"`
	Includes    []string               `yaml:"includes,omitempty"`
	Types       map[string]CustomType  `yaml:"types"`
	Entities    map[string]EntityDef   `yaml:"entities"`
	Relations   map[string]RelationDef `yaml:"relations"`
	Validations []ValidationRule       `yaml:"validations,omitempty"`
	Automations []AutomationDef        `yaml:"automations,omitempty"`

	// Attachments holds the global attachment safety floor (MIME allowlist,
	// scan policy) applied to every `file` property unless overridden.
	Attachments *AttachmentsConfig `yaml:"attachments,omitempty"`

	// Transforms is the view-export registry: named markdown -> format
	// conversions run via an external command (e.g. pandoc). Registering a
	// transform here makes it available to every markdown-producing surface
	// (entity view, list view, Lua document) as an "Export as ..." format. Lives
	// in the metamodel (not data-entry config) so the CLI can reach it too. See
	// internal/transform.
	Transforms map[string]TransformDef `yaml:"transforms,omitempty"`

	// Worlds declares named resolution functions over content states
	// (TKT-WAV8XP, design doc §4). Keyed by world name.
	//
	// ABSENT means the project has exactly the implicit DEFAULT world —
	// every entity contributes its default state — which is today's
	// graph, byte-identically. The name "default" is reserved
	// ([DefaultWorldName]) because that world is implicit and total, so
	// a declaration under that name could only shadow or contradict it.
	Worlds map[string]WorldDef `yaml:"worlds,omitempty"`

	// Computed lookups (not from YAML)
	aliasMap      map[string]string // alias -> canonical name
	inverseOwners map[string]string // inverse name -> owning canonical relation name
}

// InverseOwner returns the canonical relation type that declares the
// given inverse name, if any. Populated at load time by the metamodel
// loader after rejecting collisions and canonical-name shadowing. The
// data-entry unified-PATCH wire format uses this to resolve inverse
// body keys without scanning the relation map on every request.
//
// For a relation declared `symmetric: true` with `inverse: <self>`,
// the inverse maps back to the same canonical relation — callers that
// care about direction should consult `RelationDef.Symmetric` and
// treat the path entity as source regardless of the body key.
func (m *Metamodel) InverseOwner(inverseName string) (string, bool) {
	owner, ok := m.inverseOwners[inverseName]
	return owner, ok
}

// ValidationRule defines a custom validation rule for entities
type ValidationRule struct {
	// Name is a unique identifier for the validation rule
	Name string `yaml:"name"`

	// Description explains what this validation checks
	Description string `yaml:"description"`

	// EntityType limits the validation to a specific entity type (optional)
	// If empty, the validation applies to all entity types
	EntityType string `yaml:"entity_type,omitempty"`

	// When specifies filter conditions that select which entities this rule applies to
	// Uses the same syntax as --where filters (e.g., "status=approved")
	// Multiple conditions are ANDed together
	// If empty, the rule applies to all entities (of the specified type)
	When []string `yaml:"when,omitempty"`

	// Then specifies filter conditions that matching entities must satisfy
	// Uses the same syntax as --where filters (e.g., "owner!=")
	// Multiple conditions are ANDed together
	Then []string `yaml:"then,omitempty"`

	// WhenCondition is a predicate EXPRESSION selecting which entities the
	// rule applies to, ANDed with every When clause. Expression syntax
	// (unlike When's filter syntax) gets boolean composition and the
	// host-function stdlib, including date arithmetic.
	WhenCondition string `yaml:"when_condition,omitempty"`

	// ThenCondition is a predicate EXPRESSION that matching entities must
	// satisfy, ANDed with every Then clause.
	//
	// A rule may use any mix: `when:` + `then_condition:` is a filter
	// selecting entities that an expression then asserts over.
	ThenCondition string `yaml:"then_condition,omitempty"`

	// Content specifies validation rules for markdown body content
	Content *ContentRule `yaml:"content,omitempty"`

	// Severity is the severity level of violations: "error" or "warning"
	// Defaults to "warning" if not specified
	Severity string `yaml:"severity,omitempty"`

	// Lua specifies inline Lua code for custom validation logic.
	// The code should return true if the entity is valid, or false/nil for a violation.
	// The entity being validated is available as the `entity` global variable.
	// Read-only workspace access is available via rela.get_entity(), rela.list_entities(), etc.
	Lua string `yaml:"lua,omitempty"`

	// LuaFile specifies a path to a Lua script file in the scripts/ directory.
	// The script should return true if valid, or false/nil for a violation.
	// Example: "validate-dates.lua" loads scripts/validate-dates.lua
	LuaFile string `yaml:"lua_file,omitempty"`

	// LuaArgs specifies arguments to pass to Lua validation scripts.
	// Available as rela.args in the Lua runtime.
	LuaArgs []string `yaml:"lua_args,omitempty"`
}

// GetSeverity returns the severity level, defaulting to "warning"
func (v *ValidationRule) GetSeverity() string {
	if v.Severity == "" {
		return "warning"
	}
	return v.Severity
}

// IsError returns true if this validation has error severity
func (v *ValidationRule) IsError() bool {
	return v.GetSeverity() == "error"
}

// TypeValidation defines a regex validation for a custom type.
type TypeValidation struct {
	Pattern string `yaml:"pattern"` // Regex pattern that values must match
	Error   string `yaml:"error"`   // User-friendly error message if pattern doesn't match

	// compiled is the pre-compiled regex, populated during metamodel load.
	// Not exported to prevent YAML serialization issues.
	compiled *regexp.Regexp
}

// Compiled returns the pre-compiled regex pattern.
// Returns nil if the pattern hasn't been compiled yet.
func (tv *TypeValidation) Compiled() *regexp.Regexp {
	return tv.compiled
}

// SetCompiled sets the pre-compiled regex pattern.
func (tv *TypeValidation) SetCompiled(re *regexp.Regexp) {
	tv.compiled = re
}

// CustomType defines a reusable type with optional enum values and/or regex validations.
type CustomType struct {
	Values []string          `yaml:"values,omitempty"` // Allowed values (makes this an enum type)
	Labels map[string]string `yaml:"labels,omitempty"` // Optional display labels keyed by value (display-only; value stays the identity)
	// Descriptions is optional per-value prose keyed by value: what each value
	// MEANS, as opposed to Labels (the short display text). Display-only,
	// surfaced by the generated documentation (FEAT-G4VO53) so a reader
	// understands e.g. what "blocked" or "in-review" signifies. A value with no
	// entry simply has no description. Distinct from the type-level Description
	// scalar below (which documents the type as a whole).
	Descriptions map[string]string `yaml:"descriptions,omitempty"`
	Default      string            `yaml:"default,omitempty"`     // Default value
	Description  string            `yaml:"description,omitempty"` // Documentation for the type
	Validations  []TypeValidation  `yaml:"validations,omitempty"` // Regex validations with error messages

	// Transitions declares the legal value→value moves for this enum,
	// making it a state machine (TKT-E4LW2). This is declarative source
	// data only — the metamodel does not enforce it. At startup
	// internal/statemachine.Compile reads these into an executable machine
	// that the entitymanager runs on the write path (legality 422, guard
	// 403, precondition 422). Empty means "any value may change to any
	// other" (the historical, unconstrained behavior). Only meaningful on a
	// named type — inline `type: enum` properties carry no transitions.
	Transitions []TransitionDef `yaml:"transitions,omitempty"`

	// Initial names the only legal entry value on entity create when this
	// type is a state machine. Empty falls back to Default. Consumed by
	// internal/statemachine at compile time.
	Initial string `yaml:"initial,omitempty"`
}

// TransitionDef is one edge in an enum state machine: a legal move from one
// value to another, optionally gated by an ACL permission (Guard) and/or a
// data precondition (When). This is declarative source data; the executable
// machine is built from it by internal/statemachine.Compile.
type TransitionDef struct {
	From string `yaml:"from"` // Source value; must be one of CustomType.Values
	To   string `yaml:"to"`   // Target value; must be one of CustomType.Values

	// Label is optional display text for the MOVE (the action), not the
	// destination state — e.g. "Start progress" for todo→doing rather than the
	// state noun "Doing". Purely presentational: a machine-aware status control
	// lists transitions as verbs. Empty falls back to the target value's display
	// label (CustomType.Labels[To]) and then the raw To value. Display-only; the
	// executable machine ignores it for enforcement.
	Label string `yaml:"label,omitempty"`

	// Help is optional longer prose explaining WHY or WHEN a user would make
	// this move, beyond the short verb Label — e.g. "Send for review once the
	// implementation is complete and tests pass." Display-only, surfaced by the
	// generated documentation (FEAT-G4VO53); the executable machine ignores it.
	Help string `yaml:"help,omitempty"`

	// Guard names an ACL permission the acting principal must hold for this
	// transition. Enforced only on served paths (a principal exists); inert
	// on direct CLI writes. Empty means the transition is legal for anyone
	// who may otherwise write the entity.
	Guard string `yaml:"guard,omitempty"`

	// When is an internal/predicate expression evaluated as a precondition
	// against the entity + graph at write time. False rejects the transition
	// (422). Empty means no precondition.
	When string `yaml:"when,omitempty"`
}

// EntityDef defines an entity type in the metamodel
//
// TODO(TKT-N0IKN9): 24 exported methods, over the 20 exported-method line.
// Schema value type; ratchet candidate alongside Metamodel. DisplayProperties
// (TKT-NJTBQX) is the 24th — it reports the property set backing the display
// title so the ACL locked-title guard can gate on templated display_property
// (see internal/dataentry mentions); ratchet back down when this type is
// decomposed.
//
//plimsoll:max-exported-methods=24
type EntityDef struct {
	Label         string                 `yaml:"label"`
	LabelPlural   string                 `yaml:"label_plural,omitempty"`
	Description   string                 `yaml:"description,omitempty"` // Documentation explaining intent/usage
	Plural        string                 `yaml:"plural,omitempty"`      // Used for directory names (e.g., "policies" for "policy")
	Aliases       []string               `yaml:"aliases,omitempty"`
	IDType        string                 `yaml:"id_type,omitempty"`     // "short" (default), "sequential", or "manual"
	IDCaps        string                 `yaml:"id_caps,omitempty"`     // "upper" (default) or "lower" - capitalization for short ID suffix
	IDPrefix      string                 `yaml:"id_prefix,omitempty"`   // Single ID prefix (sugar for single-element id_prefixes)
	IDPrefixes    []string               `yaml:"id_prefixes,omitempty"` // Multiple ID prefixes
	RDFType       string                 `yaml:"rdf_type,omitempty"`
	Properties    map[string]PropertyDef `yaml:"properties"`
	PropertyOrder []string               `yaml:"-"`                      // Order of properties as defined in YAML (computed at load)
	DefaultSort   []SortSpec             `yaml:"default_sort,omitempty"` // Default sort order for this entity type
	Color         string                 `yaml:"color,omitempty"`
	BorderColor   string                 `yaml:"border_color,omitempty"`
	// DisplayProperty names the property whose value renders as the
	// human-readable display name in lists, cards, link text, etc.
	// When empty, GetPrimaryProperty() falls back to the autoderivation
	// (priority list title/name/label, then alphabetical fallback).
	// Validated at metamodel-load time: must reference a defined
	// property and must not have leading/trailing whitespace.
	DisplayProperty string `yaml:"display_property,omitempty"`

	// Pointers declares this type's content states (TKT-WAV8XP, design
	// doc §4.1). The map key is the pointer coordinate ("draft",
	// "published"); exactly one entry may set `default: true`, naming
	// the state stored under the zero pointer.
	//
	// ABSENT (the common case) means the type has no content states: it
	// contributes its single default state to EVERY world, needing no
	// per-type resolution. That is resolution rule 1, and it is why a
	// mixed graph (tickets without pointers beside pages with them)
	// needs no special handling — and why a project that never writes
	// this key behaves byte-identically to the pre-worlds system.
	Pointers map[string]PointerDef `yaml:"pointers,omitempty"`
}

// PointerDef declares one content state of an entity type.
//
// Deliberately near-empty in v1: a state is identified by its coordinate
// and nothing else. Per-state knobs (labels, ACL hints, retention) are
// NOT added speculatively — the design doc's §4.5 "no template language"
// discipline applies to declarations too.
type PointerDef struct {
	// Default marks the state stored under the ZERO pointer. At most one
	// per type (validated at load). A type declaring pointers without a
	// default still has a default STATE — every entity has one, since the
	// bare id addresses it (§2.1) — this flag only names which declared
	// coordinate that state answers to.
	Default bool `yaml:"default,omitempty"`
}

// Otherwise is a world's policy for an entity whose type declares
// pointers but none that the world selects (resolution rule 3).
//
// MANDATORY on every declared world, validated at load. A silent
// fallback here is precisely the leak content states exist to prevent —
// a `published` world quietly showing a draft face — so there is
// deliberately NO zero value that means "pick something sensible": the
// empty string is invalid and rejected.
type Otherwise string

const (
	// OtherwiseUnset is the invalid zero value; see [Otherwise].
	OtherwiseUnset Otherwise = ""
	// OtherwiseExclude contributes nothing. Public worlds say this.
	OtherwiseExclude Otherwise = "exclude"
	// OtherwiseDefault falls back to the type's default state. Internal
	// worlds may say this.
	OtherwiseDefault Otherwise = "default"
)

// IsValid reports whether the value is one of the two declared policies.
// The zero value is NOT valid — that is the point (see [Otherwise]).
func (o Otherwise) IsValid() bool {
	return o == OtherwiseExclude || o == OtherwiseDefault
}

// WorldDef declares one world: a resolution function mapping each entity
// to AT MOST ONE of its content states, the "prime" (design doc §4.1).
//
// The DEFAULT world is distinguished and needs no declaration — every
// entity contributes its default state, it is total by construction, and
// a metamodel with no `worlds:` block has exactly this world. All
// backward compatibility hangs on that.
type WorldDef struct {
	// Select is the ordered candidate chain; the first coordinate that
	// EXISTS for an entity is its prime. A bare string is accepted as a
	// one-element chain (see UnmarshalYAML).
	//
	// Order is the whole point: `[review, published]` means "the site as
	// it will look once pending reviews land". Chains keep resolution a
	// FUNCTION — they are how the single answer is computed, never a way
	// for one world to contain two faces of an entity.
	Select []string `yaml:"select,omitempty"`

	// Overrides replaces Select for named entity types. Keyed by entity
	// type; the value is that type's chain, same one-or-many spelling.
	Overrides map[string][]string `yaml:"overrides,omitempty"`

	// Otherwise is the mandatory rule-3 policy; see [Otherwise].
	Otherwise Otherwise `yaml:"otherwise,omitempty"`

	// Edits names the state that edits made from this world land in
	// (design doc §9.3, copy-on-write). PARSED BUT UNUSED in Step 2 —
	// the copy kernel is Step 4. Declared here so a project's schema
	// does not need rewriting when Step 4 lands, and validated as a
	// declared pointer so a typo surfaces now rather than then.
	Edits string `yaml:"edits,omitempty"`
}

// DefaultWorldName is reserved: the default world is implicit and total,
// so a declaration under this name could only shadow or contradict it.
const DefaultWorldName = "default"

// UnmarshalYAML accepts `select: published` as well as
// `select: [review, published]`.
//
// The one-element spelling is the overwhelmingly common case, and
// requiring a list for it is the kind of friction that gets worked
// around with copy-paste. This is sugar over the SAME type — not a
// second representation — so everything downstream sees a chain.
func (w *WorldDef) UnmarshalYAML(node *yaml.Node) error {
	// A shadow type without the method, so decoding does not recurse.
	type worldDefYAML struct {
		Select    oneOrMany            `yaml:"select,omitempty"`
		Overrides map[string]oneOrMany `yaml:"overrides,omitempty"`
		Otherwise Otherwise            `yaml:"otherwise,omitempty"`
		Edits     string               `yaml:"edits,omitempty"`
	}
	var raw worldDefYAML
	if err := node.Decode(&raw); err != nil {
		return err
	}
	w.Select = raw.Select
	w.Otherwise = raw.Otherwise
	w.Edits = raw.Edits
	if len(raw.Overrides) > 0 {
		w.Overrides = make(map[string][]string, len(raw.Overrides))
		for typ, chain := range raw.Overrides {
			w.Overrides[typ] = chain
		}
	}
	return nil
}

// oneOrMany decodes either a scalar or a sequence into a string slice.
type oneOrMany []string

func (o *oneOrMany) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var one string
		if err := node.Decode(&one); err != nil {
			return err
		}
		*o = []string{one}
		return nil
	}
	var many []string
	if err := node.Decode(&many); err != nil {
		return err
	}
	*o = many
	return nil
}

// ChainFor returns the candidate chain this world applies to entityType:
// the per-type override when one is declared, else the world's Select.
//
// The second result reports whether a non-empty chain applies. The two
// branches coincide today — an empty chain and no chain both mean rule 3,
// take Otherwise — so the compiler discards it, and the loader separately
// rejects an explicitly empty `select:` or override chain. It is returned
// because a caller that must tell "declared but empty" from "not
// declared" cannot recover the difference from the chain alone.
func (w WorldDef) ChainFor(entityType string) (chain []string, ok bool) {
	if override, found := w.Overrides[entityType]; found {
		return override, len(override) > 0
	}
	return w.Select, len(w.Select) > 0
}

// PropertyDefs implements PropertySchema for EntityDef.
func (e *EntityDef) PropertyDefs() map[string]PropertyDef {
	return e.Properties
}

// HasContent implements PropertySchema for EntityDef.
// Entities always support markdown body content.
func (e *EntityDef) HasContent() bool {
	return true
}

// Ensure EntityDef implements PropertySchema
var _ PropertySchema = (*EntityDef)(nil)

// PropertySchema abstracts property definitions for entities and relations.
// Both EntityDef and RelationDef implement this interface, allowing shared
// validation and form generation logic.
type PropertySchema interface {
	// PropertyDefs returns the property definitions map
	PropertyDefs() map[string]PropertyDef
	// HasContent returns true if markdown body content is supported
	HasContent() bool
}

// PropertyDef defines a property on an entity or relation
type PropertyDef struct {
	Type        string            `yaml:"type"`
	Required    bool              `yaml:"required,omitempty"`
	Values      []string          `yaml:"values,omitempty"` // For inline enum types
	Labels      map[string]string `yaml:"labels,omitempty"` // Optional display labels keyed by value (display-only; value stays the identity)
	Default     string            `yaml:"default,omitempty"`
	Description string            `yaml:"description,omitempty"` // Documentation for the property
	Format      string            `yaml:"format,omitempty"`      // Date format (Go layout, e.g., "2006-01-02")
	List        bool              `yaml:"list,omitempty"`        // True for multi-select properties (allows multiple values)
	// Unique constrains the property to a natural key: no two entities of
	// the same type may carry the same non-empty value. Enforced at write
	// time by the entitymanager (a colliding create/update is rejected as
	// a validation error → 422). Empty values are exempt (a property is
	// unique among the entities that set it). Ignored on `list` properties
	// (a natural key is a scalar).
	//
	// Guarantee level is not uniform across backends. The write-path check
	// is a check-then-write, not an atomic constraint, so under concurrent
	// writers two racing creates with the same value can both commit on
	// ANY backend. For race-free enforcement an operator adds a store-level
	// unique index (a partial unique index on pgstore), which is the only
	// mechanism that makes the constraint atomic. Uniqueness is therefore
	// NOT part of the store conformance contract — a new store.Store
	// implementation is not required to enforce it. See
	// `internal/entitymanager` checkUniqueProperties and the ACL
	// `principal_property` gate, which requires the referenced property to
	// be unique (and non-list).
	Unique bool `yaml:"unique,omitempty"`
	// Max caps how many attachments a `file`-type property may hold.
	// Zero/unset means 1 (the default, single-attachment). When > 1 the
	// property holds a list of attachment paths and the data-entry UI
	// switches from replace-mode to multi-file add-mode. Only meaningful
	// for `type: file`.
	Max int `yaml:"max,omitempty"`

	// Accept narrows the MIME allowlist for this `file` property to these
	// sniffed MIME types (e.g. ["application/pdf"]). Empty means inherit the
	// global allowlist. Only meaningful for `type: file`.
	Accept []string `yaml:"accept,omitempty"`

	// Scan overrides the global virus-scan policy for this `file` property.
	// ScanUnset (the zero value) means inherit the global policy. Only
	// meaningful for `type: file`.
	Scan ScanPolicy `yaml:"scan,omitempty"`

	// ScanCmd is the external scan command (array args) run when the effective
	// scan policy is `required`. Empty inherits the global scan command. Only
	// meaningful for `type: file`. See [AttachmentsConfig.ScanCmd].
	ScanCmd []string `yaml:"scan_cmd,omitempty"`

	// Transform is the ordered list of byte transforms (each an external
	// command) applied to this `file` property's uploads. Only meaningful for
	// `type: file`.
	Transform []TransformStep `yaml:"transform,omitempty"`
}

// TransformStep is one entry in a `transform:` pipeline. A step is EITHER an
// external command (Cmd) OR a native, in-process image operation (Image) —
// exactly one must be set, enforced at load. The two kinds have very different
// trust models: a Cmd shells out to an operator-configured binary (sandboxed by
// internal/cmdexec), while an Image step runs a memory-safe pure-Go decoder
// in-process with no external tool and no sandbox. See the attachment-security
// guide.
type TransformStep struct {
	// Cmd is an external command (array args) that rewrites the bytes; it
	// receives templated {in}/{out} paths owned by the runner.
	Cmd []string `yaml:"cmd,omitempty"`
	// Image is a native in-process image transform (decode, orient, re-encode).
	// Mutually exclusive with Cmd.
	Image *ImageStep `yaml:"image,omitempty"`
}

// ImageStep is a native, in-process image transform: it decodes the upload with
// a memory-safe pure-Go decoder, bakes in EXIF orientation, and re-encodes to a
// canonical format, dropping all metadata. Supported inputs are PNG, JPEG, GIF,
// and WebP (decode); the output is always PNG or JPEG (there is no pure-Go WebP
// encoder). Because decoding is memory-safe, this needs no external tool and no
// OS sandbox. Resize/thumbnail is a later phase and deliberately absent here.
type ImageStep struct {
	// Reencode is the canonical output format: "jpeg" (default) or "png". Both
	// are always within the default-safe MIME allowlist, so the re-encoded
	// output is safe by construction.
	Reencode string `yaml:"reencode,omitempty"`
	// Quality is the JPEG quality (1..100) when Reencode is "jpeg". Zero uses
	// the package default. Ignored for PNG.
	Quality int `yaml:"quality,omitempty"`
}

// ImageReencode returns the effective re-encode target for the step, applying
// the "jpeg" default when unset.
func (s ImageStep) ImageReencode() string {
	if s.Reencode == "" {
		return "jpeg"
	}
	return s.Reencode
}

// Kind reports which of the two mutually-exclusive step kinds is set, or an
// empty string when the step is malformed (both/neither set).
func (t TransformStep) Kind() string {
	switch {
	case len(t.Cmd) > 0 && t.Image == nil:
		return "cmd"
	case t.Image != nil && len(t.Cmd) == 0:
		return "image"
	default:
		return ""
	}
}

// TransformDef is one entry in the top-level `transforms:` view-export registry.
// It converts From-format bytes (v1: always "markdown") to the Produces
// content-type by running Command as an argv array. Command may reference the
// {in}/{out} placeholders (temp paths owned by the runner); otherwise input is
// on stdin and output on stdout. Commands come from project config (this file),
// never from a request — a request may only name a registered transform.
//
// The mirror of this in internal/transform is transform.Def; the metamodel keeps
// its own YAML-tagged type so internal/transform need not be imported here.
type TransformDef struct {
	From     string   `yaml:"from"`
	Command  []string `yaml:"command"`
	Produces string   `yaml:"produces"`
}

// FileMax returns the effective attachment cap for a file property:
// Max when set (>0), otherwise 1. Callers should use this rather than
// reading Max directly so the unset-means-one default lives in one place.
func (p PropertyDef) FileMax() int {
	if p.Max > 0 {
		return p.Max
	}
	return 1
}

// Built-in property types
const (
	PropertyTypeString   = "string"
	PropertyTypeDate     = "date"
	PropertyTypeDatetime = "datetime"
	PropertyTypeInteger  = "integer"
	PropertyTypeBoolean  = "boolean"
	PropertyTypeEnum     = "enum"
	PropertyTypeFile     = "file"
	PropertyTypeRrule    = "rrule"
)

// ID types for entities
const (
	IDTypeShort      = "short"      // IDs are random base36 strings (e.g., REQ-a3f8) - default
	IDTypeSequential = "sequential" // IDs are auto-generated with numeric suffix (e.g., REQ-001)
	IDTypeManual     = "manual"     // IDs are manually specified strings (e.g., auth-module)

	// Deprecated alias (still accepted for backwards compatibility)
	IDTypeString = "string" // Deprecated: use "manual" instead
)

// ID capitalization modes for short IDs
const (
	IDCapsUpper = "upper" // Random suffix is uppercase (e.g., REQ-A3F8) - default
	IDCapsLower = "lower" // Random suffix is lowercase (e.g., REQ-a3f8)
)

// ReservedPropertyNames contains property names that cannot be used in metamodel definitions
// because they conflict with built-in entity fields.
var ReservedPropertyNames = map[string]bool{
	"id":   true, // Entity.ID
	"type": true, // Entity.Type
}

// RelationScope declares what a relation type attaches to under content
// states (TKT-DOFYR1, design doc §2.2).
//
//   - identity: the edge attaches to the entity's bare id and is shared
//     by all its states. Ownership, containment, membership — a draft
//     does not get a different owner than its published face by
//     accident. The DEFAULT, so a project without pointers behaves
//     identically either way.
//   - content: the edge attaches to a specific state on its TAIL side
//     (Relation.FromPointer); a draft may cite different targets than
//     the published face. Heads stay entity-level (§2.3).
//
// DECLARATIVE-ONLY in Step 1: entity delete cascades every edge and
// rename re-keys bare ids regardless of scope, so no store behavior
// branches on it yet. Its consumers land with later steps — role
// conferral (worlds/ACL), cardinality subjects, and the copy kernel's
// per-relation-type merge semantics all read this one declaration.
type RelationScope string

const (
	// ScopeIdentity is the zero/default scope; see RelationScope.
	ScopeIdentity RelationScope = ""
	// ScopeIdentityExplicit is the spelled-out identity scope.
	ScopeIdentityExplicit RelationScope = "identity"
	// ScopeContent marks state-tailed edges; see RelationScope.
	ScopeContent RelationScope = "content"
)

// IsValid reports whether the scope is a declared value.
func (s RelationScope) IsValid() bool {
	switch s {
	case ScopeIdentity, ScopeIdentityExplicit, ScopeContent:
		return true
	default:
		return false
	}
}

// IsContent reports whether edges of this type attach to a specific
// state on their tail side.
func (s RelationScope) IsContent() bool { return s == ScopeContent }

// IsIdentity reports whether edges of this type attach to the entity's
// bare id. ALWAYS branch through IsIdentity/IsContent, never by
// comparing against [ScopeIdentity]: the identity scope has two
// spellings ("" and "identity"), so `scope == ScopeIdentity` is wrong
// for any metamodel that writes it out — and passes every test written
// against one that doesn't.
func (s RelationScope) IsIdentity() bool { return !s.IsContent() }

// OrderableMode controls which side(s) of a relation type are user-orderable.
type OrderableMode string

const (
	OrderableNone     OrderableMode = ""
	OrderableOutgoing OrderableMode = "outgoing"
	OrderableIncoming OrderableMode = "incoming"
	OrderableBoth     OrderableMode = "both"
)

// Reserved relation-property names that hold the user-controlled order value.
// The names are stable across mode changes so that promoting a relation from
// outgoing-only to both (or vice versa) keeps existing values on disk valid.
const (
	OrderPropertyOut = "_order_out"
	OrderPropertyIn  = "_order_in"
)

// IsValid reports whether the value is a recognized OrderableMode (including the
// empty "not orderable" value).
func (m OrderableMode) IsValid() bool {
	switch m {
	case OrderableNone, OrderableOutgoing, OrderableIncoming, OrderableBoth:
		return true
	}
	return false
}

// DefaultDateFormat is the default format for date properties (ISO 8601)
const DefaultDateFormat = "2006-01-02"

// DefaultDatetimeFormat is the default format for datetime properties (RFC3339,
// a time-bearing ISO 8601 instant). Unlike date, a datetime value carries a
// time-of-day and (canonically) a UTC offset.
const DefaultDatetimeFormat = time.RFC3339

// IsBuiltinType returns true if the type is a built-in property type
func IsBuiltinType(t string) bool {
	switch t {
	case PropertyTypeString, PropertyTypeDate, PropertyTypeDatetime, PropertyTypeInteger,
		PropertyTypeBoolean, PropertyTypeEnum, PropertyTypeFile, PropertyTypeRrule:
		return true
	}
	return false
}

// GetDateFormat returns the date format for a property, defaulting to ISO 8601.
// For datetime properties the default is RFC3339 (time-bearing); an explicit
// Format still overrides.
func (p *PropertyDef) GetDateFormat() string {
	if p.Format != "" {
		return p.Format
	}
	if p.Type == PropertyTypeDatetime {
		return DefaultDatetimeFormat
	}
	return DefaultDateFormat
}

// RelationDef defines a relation type in the metamodel
type RelationDef struct {
	Label       string      `yaml:"label"`
	Description string      `yaml:"description,omitempty"`
	From        []string    `yaml:"from"`
	To          []string    `yaml:"to"`
	Inverse     *InverseDef `yaml:"inverse,omitempty"`
	Symmetric   bool        `yaml:"symmetric,omitempty"`
	MinOutgoing *int        `yaml:"min_outgoing,omitempty"`
	MaxOutgoing *int        `yaml:"max_outgoing,omitempty"`
	MinIncoming *int        `yaml:"min_incoming,omitempty"`
	MaxIncoming *int        `yaml:"max_incoming,omitempty"`

	// Scope declares whether edges of this type attach to the entity
	// (identity, the default) or to a specific content state on the
	// tail side (content). See RelationScope. Unrelated to the Content
	// field below, which is about markdown bodies — the shared word is
	// an unfortunate collision, not a connection.
	Scope RelationScope `yaml:"scope,omitempty"`

	// Properties defines typed properties that can be attached to relations of this type.
	// Uses the same PropertyDef structure as entity properties.
	Properties map[string]PropertyDef `yaml:"properties,omitempty"`

	// Content indicates whether relations of this type support markdown body content.
	// When true, the data-entry UI will show a content editor for the relation.
	// Unrelated to Scope's "content" value above (state-tailed edges) —
	// same word, different concern.
	Content bool `yaml:"content,omitempty"`

	// Orderable declares which side(s) of this relation type are user-orderable.
	// When set, the data-entry UI offers drag-to-reorder controls on the enabled
	// side(s); the API returns relations sorted by the corresponding managed
	// order property (OrderPropertyOut / OrderPropertyIn).
	Orderable OrderableMode `yaml:"orderable,omitempty"`
}

// OutgoingOrderProperty returns the relation-property name that holds the
// outgoing-side order value, or "" if outgoing ordering is not enabled.
func (r *RelationDef) OutgoingOrderProperty() string {
	if r.Orderable == OrderableOutgoing || r.Orderable == OrderableBoth {
		return OrderPropertyOut
	}
	return ""
}

// IncomingOrderProperty returns the relation-property name that holds the
// incoming-side order value, or "" if incoming ordering is not enabled.
func (r *RelationDef) IncomingOrderProperty() string {
	if r.Orderable == OrderableIncoming || r.Orderable == OrderableBoth {
		return OrderPropertyIn
	}
	return ""
}

// PropertyDefs implements PropertySchema for RelationDef.
func (r *RelationDef) PropertyDefs() map[string]PropertyDef {
	return r.Properties
}

// HasContent implements PropertySchema for RelationDef.
func (r *RelationDef) HasContent() bool {
	return r.Content
}

// HasAdvancedFeatures returns true if this relation type has properties or content,
// indicating that the data-entry UI should use the advanced cards+modal interface.
func (r *RelationDef) HasAdvancedFeatures() bool {
	return len(r.Properties) > 0 || r.Content
}

// Ensure RelationDef implements PropertySchema
var _ PropertySchema = (*RelationDef)(nil)

// InverseDef defines the inverse of a relation.
// Can be unmarshaled from either a simple string (inverse identifier only)
// or an object with id and label fields.
type InverseDef struct {
	// ID is the identifier for the inverse relation (e.g., "addressedBy")
	ID string `yaml:"id,omitempty"`

	// Label is the display label for the inverse relation (e.g., "addressed by").
	// If not specified, the raw ID is displayed — labels are authored, never
	// derived (DEC-6C1NAA).
	Label string `yaml:"label,omitempty"`
}

// GetID returns the inverse relation identifier
func (i *InverseDef) GetID() string {
	return i.ID
}

// GetLabel returns the display label, falling back to the raw ID.
//
// A label is authored, never derived (DEC-6C1NAA). This used to convert
// camelCase to space-separated lowercase ("addressedBy" → "addressed by"),
// which bakes an English orthographic convention into a language-neutral
// metamodel. Write an explicit `label:` to control the display text.
func (i *InverseDef) GetLabel() string {
	if i.Label != "" {
		return i.Label
	}
	return i.ID
}

// UnmarshalYAML allows InverseDef to be unmarshaled from either a string or an object.
// String form: "addressedBy" (ID only; the ID doubles as the display label)
// Object form: { id: "addressedBy", label: "addressed by" }
func (i *InverseDef) UnmarshalYAML(unmarshal func(any) error) error {
	// First try to unmarshal as a string (simple form)
	var simpleForm string
	if err := unmarshal(&simpleForm); err == nil {
		i.ID = simpleForm
		// Label will be auto-derived by GetLabel()
		return nil
	}

	// Try to unmarshal as an object (expanded form)
	type inverseDefAlias InverseDef // Alias to avoid infinite recursion
	var objectForm inverseDefAlias
	if err := unmarshal(&objectForm); err != nil {
		return err
	}

	*i = InverseDef(objectForm)
	return nil
}

// AutomationDef defines a trigger-action automation rule.
type AutomationDef struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description,omitempty"`
	On          AutomationTrigger  `yaml:"on"`
	Do          []AutomationAction `yaml:"do,omitempty"`
	Validate    []AutomationCheck  `yaml:"validate,omitempty"`
}

// AutomationTrigger specifies conditions that activate an automation.
type AutomationTrigger struct {
	Entity          StringOrSlice `yaml:"entity,omitempty"`
	Property        string        `yaml:"property,omitempty"`
	Becomes         string        `yaml:"becomes,omitempty"`
	From            string        `yaml:"from,omitempty"`
	Created         bool          `yaml:"created,omitempty"`
	RelationCreated string        `yaml:"relation_created,omitempty"`
	RelationRemoved string        `yaml:"relation_removed,omitempty"`
	When            []string      `yaml:"when,omitempty"` // Property conditions that must match (AND logic)

	// Condition is a predicate EXPRESSION that must hold for the
	// automation to fire, ANDed with every When clause.
	//
	// When and Condition are separate keys because their syntaxes
	// overlap without erroring: filter.Parse accepts
	// "days_between(entity.due, today()) <= 7" as a filter on a property
	// literally named "days_between(entity.due, today())", which then
	// matches nothing, silently. Sniffing which dialect a string is
	// written in would guess, and guess quietly — so the operator says
	// which one they meant by choosing the key.
	//
	// `when:` is filter syntax (`status=todo`) transpiled to predicate on
	// load; `condition:` is predicate source evaluated as written, so it
	// gets boolean composition and the host-function stdlib — notably the
	// date arithmetic (today/days_between/date_add/rrule_next) that a
	// property filter cannot express.
	Condition string `yaml:"condition,omitempty"`
}

// AutomationAction specifies an operation to perform.
type AutomationAction struct {
	Set            string                `yaml:"set,omitempty"`
	Value          string                `yaml:"value,omitempty"`
	CreateRelation *CreateRelationAction `yaml:"create_relation,omitempty"`
	CreateEntity   *CreateEntityAction   `yaml:"create_entity,omitempty"`
	Lua            string                `yaml:"lua,omitempty"`      // Inline Lua code to execute
	LuaFile        string                `yaml:"lua_file,omitempty"` // Path to Lua script in scripts/ directory

	// AllowACLBypass unlocks rela.bypass_acl in this Lua action (TKT-D8T148).
	// Operator-only (lives in the schema file). When set, the script may call
	// rela.bypass_acl(fn) to obtain a closure-scoped elevated handle whose
	// access skips the ACL deny (still audited, real principal preserved).
	// Ignored for non-Lua actions.
	//
	// Since TKT-Y3JVFK this is an enum, not a bool: `read`, `write` or
	// `read+write` select which methods the handle carries. The legacy
	// `true` is refused at parse time with a message naming `read+write`;
	// `rela migrate` rewrites it.
	AllowACLBypass ACLBypass `yaml:"allow_acl_bypass,omitempty"`
}

// CreateRelationAction specifies parameters for creating a relation.
type CreateRelationAction struct {
	Relation string `yaml:"relation"`
	To       string `yaml:"to"`
}

// CreateEntityAction specifies parameters for creating a new entity.
type CreateEntityAction struct {
	Type       string            `yaml:"type"`                 // Entity type to create
	Template   string            `yaml:"template,omitempty"`   // Optional: template variant, supports interpolation (e.g., "{{new.kind}}")
	Properties map[string]string `yaml:"properties,omitempty"` // Properties (values support interpolation)
	Relation   string            `yaml:"relation,omitempty"`   // Optional: relation FROM trigger TO created entity
	IfExists   string            `yaml:"if_exists,omitempty"`  // Behavior when relation already exists: skip (default), error, replace
}

// AutomationCheck specifies a validation condition.
type AutomationCheck struct {
	Check    string `yaml:"check"`
	Severity string `yaml:"severity,omitempty"`
	Message  string `yaml:"message"`
}

// ContentRule defines validation rules for markdown body content.
type ContentRule struct {
	// RequiredHeaders specifies headers that must appear in the content
	RequiredHeaders []HeaderCheck `yaml:"required-headers,omitempty"`

	// Checklist specifies validation rules for markdown checklists (task lists)
	Checklist *ChecklistRule `yaml:"checklist,omitempty"`
}

// ChecklistRule defines validation rules for markdown checklists.
type ChecklistRule struct {
	// AllChecked requires all checklist items to be checked
	AllChecked bool `yaml:"all-checked,omitempty"`

	// AllowSkipped treats strikethrough items as complete (e.g., "- [x] ~~task~~ (N/A: reason)")
	AllowSkipped bool `yaml:"allow-skipped,omitempty"`
}

// HeaderCheck specifies a header to check for in markdown content.
// Can be unmarshaled from either a simple string (exact match) or an object with pattern field.
type HeaderCheck struct {
	// Header is an exact header string to match (e.g., "## Context")
	Header string `yaml:"header,omitempty"`

	// Pattern is a regex pattern to match headers (e.g., "## (Alternative|Alternatives)")
	Pattern string `yaml:"pattern,omitempty"`
}

// IsPattern returns true if this is a regex pattern match
func (h *HeaderCheck) IsPattern() bool {
	return h.Pattern != ""
}

// GetMatchString returns the pattern or header string to match against
func (h *HeaderCheck) GetMatchString() string {
	if h.Pattern != "" {
		return h.Pattern
	}
	return h.Header
}

// UnmarshalYAML allows HeaderCheck to be unmarshaled from either a string or an object.
// String form: "## Context" (exact header match)
// Object form: { pattern: "## (Alternative|Alternatives)" }
func (h *HeaderCheck) UnmarshalYAML(unmarshal func(any) error) error {
	// First try to unmarshal as a string (simple form - exact match)
	var simpleForm string
	if err := unmarshal(&simpleForm); err == nil {
		h.Header = simpleForm
		return nil
	}

	// Try to unmarshal as an object (expanded form with pattern)
	type headerCheckAlias HeaderCheck // Alias to avoid infinite recursion
	var objectForm headerCheckAlias
	if err := unmarshal(&objectForm); err != nil {
		return err
	}

	*h = HeaderCheck(objectForm)
	return nil
}

// StringOrSlice is a YAML type that can be unmarshaled from either a string or []string.
type StringOrSlice []string

// UnmarshalYAML allows StringOrSlice to be unmarshaled from either a string or a slice.
func (s *StringOrSlice) UnmarshalYAML(unmarshal func(any) error) error {
	// Try string first
	var single string
	if err := unmarshal(&single); err == nil {
		*s = []string{single}
		return nil
	}
	// Try slice
	var slice []string
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}
