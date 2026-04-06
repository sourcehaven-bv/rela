package metamodel

import (
	"regexp"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Metamodel represents the full metamodel configuration
type Metamodel struct {
	Version     string                 `yaml:"version"`
	Namespace   string                 `yaml:"namespace"`
	Includes    []string               `yaml:"includes,omitempty"`
	Types       map[string]CustomType  `yaml:"types"`
	Entities    map[string]EntityDef   `yaml:"entities"`
	Relations   map[string]RelationDef `yaml:"relations"`
	Validations []ValidationRule       `yaml:"validations,omitempty"`
	Automations []AutomationDef        `yaml:"automations,omitempty"`

	// Computed lookups (not from YAML)
	aliasMap map[string]string // alias -> canonical name
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
	Values      []string         `yaml:"values,omitempty"`      // Allowed values (makes this an enum type)
	Default     string           `yaml:"default,omitempty"`     // Default value
	Description string           `yaml:"description,omitempty"` // Documentation for the type
	Validations []TypeValidation `yaml:"validations,omitempty"` // Regex validations with error messages
}

// EntityDef defines an entity type in the metamodel
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
	DefaultSort   []model.SortSpec       `yaml:"default_sort,omitempty"` // Default sort order for this entity type
	Color         string                 `yaml:"color,omitempty"`
	BorderColor   string                 `yaml:"border_color,omitempty"`
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
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required,omitempty"`
	Values      []string `yaml:"values,omitempty"` // For inline enum types
	Default     string   `yaml:"default,omitempty"`
	Description string   `yaml:"description,omitempty"` // Documentation for the property
	Format      string   `yaml:"format,omitempty"`      // Date format (Go layout, e.g., "2006-01-02")
	List        bool     `yaml:"list,omitempty"`        // True for multi-select properties (allows multiple values)
}

// Built-in property types
const (
	PropertyTypeString  = "string"
	PropertyTypeDate    = "date"
	PropertyTypeInteger = "integer"
	PropertyTypeBoolean = "boolean"
	PropertyTypeEnum    = "enum"
	PropertyTypeFile    = "file"
	PropertyTypeRrule   = "rrule"
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

// DefaultDateFormat is the default format for date properties (ISO 8601)
const DefaultDateFormat = "2006-01-02"

// IsBuiltinType returns true if the type is a built-in property type
func IsBuiltinType(t string) bool {
	switch t {
	case PropertyTypeString, PropertyTypeDate, PropertyTypeInteger,
		PropertyTypeBoolean, PropertyTypeEnum, PropertyTypeFile, PropertyTypeRrule:
		return true
	}
	return false
}

// GetDateFormat returns the date format for a property, defaulting to ISO 8601
func (p *PropertyDef) GetDateFormat() string {
	if p.Format != "" {
		return p.Format
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

	// Properties defines typed properties that can be attached to relations of this type.
	// Uses the same PropertyDef structure as entity properties.
	Properties map[string]PropertyDef `yaml:"properties,omitempty"`

	// Content indicates whether relations of this type support markdown body content.
	// When true, the data-entry UI will show a content editor for the relation.
	Content bool `yaml:"content,omitempty"`
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

	// Label is the display label for the inverse relation (e.g., "addressed by")
	// If not specified, it's auto-derived from ID by converting camelCase to space-separated.
	Label string `yaml:"label,omitempty"`
}

// GetID returns the inverse relation identifier
func (i *InverseDef) GetID() string {
	return i.ID
}

// GetLabel returns the display label, auto-deriving from ID if not specified
func (i *InverseDef) GetLabel() string {
	if i.Label != "" {
		return i.Label
	}
	// Auto-derive from ID by converting camelCase to space-separated lowercase
	if i.ID == "" {
		return ""
	}
	return camelCaseToSpaced(i.ID)
}

// camelCaseToSpaced converts camelCase/PascalCase to space-separated lowercase.
// Examples: "addressedBy" → "addressed by", "implementedBy" → "implemented by"
func camelCaseToSpaced(s string) string {
	if s == "" {
		return ""
	}

	const asciiCaseOffset = 'a' - 'A'   // 32, but as a named constant
	result := make([]byte, 0, len(s)+4) // Extra space for inserted spaces

	for i := 0; i < len(s); i++ {
		c := s[i]
		isUpper := c >= 'A' && c <= 'Z'

		switch {
		case i > 0 && isUpper:
			// Insert space before uppercase letters (except at start) and convert to lowercase
			result = append(result, ' ', c+asciiCaseOffset)
		case isUpper:
			// First character - just convert to lowercase
			result = append(result, c+asciiCaseOffset)
		default:
			result = append(result, c)
		}
	}
	return string(result)
}

// UnmarshalYAML allows InverseDef to be unmarshaled from either a string or an object.
// String form: "addressedBy" (ID only, label auto-derived)
// Object form: { id: "addressedBy", label: "addressed by" }
func (i *InverseDef) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
}

// AutomationAction specifies an operation to perform.
type AutomationAction struct {
	Set            string                `yaml:"set,omitempty"`
	Value          string                `yaml:"value,omitempty"`
	CreateRelation *CreateRelationAction `yaml:"create_relation,omitempty"`
	CreateEntity   *CreateEntityAction   `yaml:"create_entity,omitempty"`
	Lua            string                `yaml:"lua,omitempty"`      // Inline Lua code to execute
	LuaFile        string                `yaml:"lua_file,omitempty"` // Path to Lua script in scripts/ directory
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
func (h *HeaderCheck) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
func (s *StringOrSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
