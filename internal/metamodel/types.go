package metamodel

// Metamodel represents the full metamodel configuration
type Metamodel struct {
	Version     string                 `yaml:"version"`
	Namespace   string                 `yaml:"namespace"`
	Types       map[string]CustomType  `yaml:"types"`
	Entities    map[string]EntityDef   `yaml:"entities"`
	Relations   map[string]RelationDef `yaml:"relations"`
	Validations []ValidationRule       `yaml:"validations,omitempty"`

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

	// Match specifies filter conditions that select which entities this rule applies to
	// Uses the same syntax as --where filters (e.g., "status=approved")
	// Multiple conditions are ANDed together
	// If empty, the rule applies to all entities (of the specified type)
	Match []string `yaml:"match,omitempty"`

	// Require specifies filter conditions that matching entities must satisfy
	// Uses the same syntax as --where filters (e.g., "owner!=")
	// Multiple conditions are ANDed together
	Require []string `yaml:"require"`

	// Severity is the severity level of violations: "error" or "warning"
	// Defaults to "warning" if not specified
	Severity string `yaml:"severity,omitempty"`
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

// CustomType defines a reusable enum type
type CustomType struct {
	Values  []string `yaml:"values"`
	Default string   `yaml:"default,omitempty"`
}

// EntityDef defines an entity type in the metamodel
type EntityDef struct {
	Label       string                 `yaml:"label"`
	LabelPlural string                 `yaml:"label_plural,omitempty"`
	Plural      string                 `yaml:"plural,omitempty"` // Used for directory names (e.g., "policies" for "policy")
	Aliases     []string               `yaml:"aliases,omitempty"`
	IDType      string                 `yaml:"id_type,omitempty"` // "sequential" (default) or "string"
	IDPatterns  []string               `yaml:"id_patterns"`
	RDFType     string                 `yaml:"rdf_type,omitempty"`
	Properties  map[string]PropertyDef `yaml:"properties"`
	Color       string                 `yaml:"color,omitempty"`
	BorderColor string                 `yaml:"border_color,omitempty"`
}

// PropertyDef defines a property on an entity
type PropertyDef struct {
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required,omitempty"`
	Values      []string `yaml:"values,omitempty"` // For inline enum types
	Default     string   `yaml:"default,omitempty"`
	Description string   `yaml:"description,omitempty"` // Documentation for the property
	Format      string   `yaml:"format,omitempty"`      // Date format (Go layout, e.g., "2006-01-02")
}

// Built-in property types
const (
	PropertyTypeString  = "string"
	PropertyTypeDate    = "date"
	PropertyTypeInteger = "integer"
	PropertyTypeBoolean = "boolean"
	PropertyTypeEnum    = "enum"
)

// ID types for entities
const (
	IDTypeSequential = "sequential" // IDs are auto-generated with numeric suffix (e.g., REQ-001)
	IDTypeString     = "string"     // IDs are manually specified strings (e.g., auth-module)
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
	case PropertyTypeString, PropertyTypeDate, PropertyTypeInteger, PropertyTypeBoolean, PropertyTypeEnum:
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
	Label       string          `yaml:"label"`
	Description string          `yaml:"description,omitempty"`
	From        []string        `yaml:"from"`
	To          []string        `yaml:"to"`
	Inverse     *InverseDef     `yaml:"inverse,omitempty"`
	Symmetric   bool            `yaml:"symmetric,omitempty"`
	Cardinality *CardinalityDef `yaml:"cardinality,omitempty"` // New: nested cardinality with UML notation

	// Deprecated: use Cardinality instead
	SourceMin *int `yaml:"source_min,omitempty"`
	SourceMax *int `yaml:"source_max,omitempty"`
	TargetMin *int `yaml:"target_min,omitempty"`
	TargetMax *int `yaml:"target_max,omitempty"`
}

// CardinalityDef defines cardinality constraints using UML notation
// Examples: "1", "0..1", "1..*", "0..*", "*", "2..5"
type CardinalityDef struct {
	From string `yaml:"from,omitempty"` // Cardinality on the "from" side
	To   string `yaml:"to,omitempty"`   // Cardinality on the "to" side
}

// InverseDef defines the inverse of a relation
type InverseDef struct {
	Name  string `yaml:"name"`
	Label string `yaml:"label"`
}

// GetPlural returns the plural label for an entity type
func (e *EntityDef) GetPlural() string {
	if e.LabelPlural != "" {
		return e.LabelPlural
	}
	return e.Label + "s"
}

// GetDirPlural returns the plural form to use for directory names
func (e *EntityDef) GetDirPlural(typeName string) string {
	if e.Plural != "" {
		return e.Plural
	}
	// Fall back to naive pluralization of the type name
	return typeName + "s"
}

// GetDefaultStatus returns the default status value for this entity type.
// It checks the entity's status property definition for a custom type or inline values.
// If no explicit default exists, returns the first valid value, or "draft" as final fallback.
func (e *EntityDef) GetDefaultStatus(m *Metamodel) string {
	statusProp, ok := e.Properties["status"]
	if !ok {
		// No status property defined, use standard default
		return "draft"
	}

	// Check for explicit default in property definition
	if statusProp.Default != "" {
		return statusProp.Default
	}

	// Check for inline enum values
	if len(statusProp.Values) > 0 {
		return statusProp.Values[0]
	}

	// Check for custom type
	if statusProp.Type != "" && statusProp.Type != "status" && statusProp.Type != "string" {
		if customType, ok := m.Types[statusProp.Type]; ok {
			if customType.Default != "" {
				return customType.Default
			}
			if len(customType.Values) > 0 {
				return customType.Values[0]
			}
		}
	}

	// Standard "status" type - use "draft" as default
	return "draft"
}

// GetPrimaryProperty returns the name of the primary required string property.
// This is typically "title" or "name" - the first required string property found.
// Returns empty string if no suitable property exists.
func (e *EntityDef) GetPrimaryProperty() string {
	// Check common names first in priority order
	priorityNames := []string{"title", "name", "label"}
	for _, name := range priorityNames {
		if prop, ok := e.Properties[name]; ok {
			if prop.Required && (prop.Type == PropertyTypeString || prop.Type == "") {
				return name
			}
		}
	}

	// Fall back to finding any required string property (sorted for determinism)
	var candidates []string
	for name, prop := range e.Properties {
		if prop.Required && (prop.Type == PropertyTypeString || prop.Type == "") {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) > 0 {
		// Sort for deterministic behavior
		for i := 1; i < len(candidates); i++ {
			for j := i; j > 0 && candidates[j] < candidates[j-1]; j-- {
				candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
			}
		}
		return candidates[0]
	}

	return ""
}

// GetIDType returns the ID type for this entity, defaulting to "sequential"
func (e *EntityDef) GetIDType() string {
	if e.IDType == "" {
		return IDTypeSequential
	}
	return e.IDType
}

// IsSequentialID returns true if this entity type uses sequential IDs
func (e *EntityDef) IsSequentialID() bool {
	return e.GetIDType() == IDTypeSequential
}

// IsStringID returns true if this entity type uses string IDs
func (e *EntityDef) IsStringID() bool {
	return e.GetIDType() == IDTypeString
}

// HasPattern checks if the entity type matches a given ID pattern
func (e *EntityDef) HasPattern(pattern string) bool {
	for _, p := range e.IDPatterns {
		if p == pattern {
			return true
		}
	}
	return false
}

// MatchesID checks if an ID matches any of this entity type's patterns
func (e *EntityDef) MatchesID(id string) bool {
	for _, pattern := range e.IDPatterns {
		if len(id) >= len(pattern) && id[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}

// ResolveAlias returns the canonical entity type name for an alias
func (m *Metamodel) ResolveAlias(alias string) string {
	if m.aliasMap == nil {
		return alias
	}
	if canonical, ok := m.aliasMap[alias]; ok {
		return canonical
	}
	return alias
}

// GetEntityDef returns the entity definition for a type (resolving aliases)
func (m *Metamodel) GetEntityDef(entityType string) (*EntityDef, bool) {
	// First try direct lookup
	if def, ok := m.Entities[entityType]; ok {
		return &def, true
	}
	// Try alias resolution
	canonical := m.ResolveAlias(entityType)
	if def, ok := m.Entities[canonical]; ok {
		return &def, true
	}
	return nil, false
}

// GetRelationDef returns the relation definition
func (m *Metamodel) GetRelationDef(name string) (*RelationDef, bool) {
	if def, ok := m.Relations[name]; ok {
		return &def, true
	}
	return nil, false
}

// InferEntityType tries to determine the entity type from an ID
func (m *Metamodel) InferEntityType(id string) string {
	for name, def := range m.Entities {
		if def.MatchesID(id) {
			return name
		}
	}
	return ""
}

// ValidateRelation checks if a relation is valid between two entity types
func (m *Metamodel) ValidateRelation(relationType, fromType, toType string) error {
	rel, ok := m.GetRelationDef(relationType)
	if !ok {
		return &RelationNotFoundError{Name: relationType}
	}

	fromValid := false
	for _, t := range rel.From {
		if t == fromType {
			fromValid = true
			break
		}
	}
	if !fromValid {
		return &InvalidRelationError{
			Relation: relationType,
			From:     fromType,
			To:       toType,
			Message:  "source entity type not allowed",
		}
	}

	toValid := false
	for _, t := range rel.To {
		if t == toType {
			toValid = true
			break
		}
	}
	if !toValid {
		return &InvalidRelationError{
			Relation: relationType,
			From:     fromType,
			To:       toType,
			Message:  "target entity type not allowed",
		}
	}

	return nil
}

// EntityTypes returns all entity type names
func (m *Metamodel) EntityTypes() []string {
	types := make([]string, 0, len(m.Entities))
	for name := range m.Entities {
		types = append(types, name)
	}
	return types
}

// RelationTypes returns all relation type names
func (m *Metamodel) RelationTypes() []string {
	types := make([]string, 0, len(m.Relations))
	for name := range m.Relations {
		types = append(types, name)
	}
	return types
}

// Errors

type RelationNotFoundError struct {
	Name string
}

func (e *RelationNotFoundError) Error() string {
	return "unknown relation: " + e.Name
}

type InvalidRelationError struct {
	Relation string
	From     string
	To       string
	Message  string
}

func (e *InvalidRelationError) Error() string {
	return "invalid relation " + e.Relation + " from " + e.From + " to " + e.To + ": " + e.Message
}

type InvalidIDTypeError struct {
	EntityType string
	IDType     string
}

func (e *InvalidIDTypeError) Error() string {
	return "invalid id_type for entity " + e.EntityType + ": " + e.IDType + " (must be 'sequential' or 'string')"
}

type ReservedPropertyError struct {
	EntityType   string
	PropertyName string
}

func (e *ReservedPropertyError) Error() string {
	return "entity " + e.EntityType + ": property \"" + e.PropertyName + "\" is reserved and cannot be used"
}

// WhitespacePropertyError is returned when a property name has leading or trailing whitespace
type WhitespacePropertyError struct {
	EntityType   string
	PropertyName string
}

func (e *WhitespacePropertyError) Error() string {
	return "entity " + e.EntityType + ": property name \"" + e.PropertyName + "\" has leading or trailing whitespace"
}

// ConflictingCardinalityError is returned when both old and new cardinality syntax are used
type ConflictingCardinalityError struct {
	RelationType string
}

func (e *ConflictingCardinalityError) Error() string {
	return "relation " + e.RelationType +
		" uses both 'cardinality' and 'source_min/source_max/target_min/target_max'; use only one style"
}

// InvalidCardinalityError is returned when cardinality notation is invalid
type InvalidCardinalityError struct {
	RelationType string
	Side         string // "from" or "to"
	Notation     string
	Message      string
}

func (e *InvalidCardinalityError) Error() string {
	return "invalid cardinality for relation " + e.RelationType + " " + e.Side + ": " +
		e.Notation + " (" + e.Message + ")"
}

// ValidateCardinalityNotation validates UML cardinality notation.
// Valid formats: "1", "0..1", "1..*", "0..*", "*", "2..5"
func ValidateCardinalityNotation(notation string) error {
	if notation == "" || notation == "*" {
		return nil
	}

	idx := findDoubleDot(notation)
	if idx > 0 {
		// Range format "min..max"
		minStr := notation[:idx]
		maxStr := notation[idx+2:]

		if parseInt(minStr) == nil {
			return &cardinalityParseError{msg: "invalid minimum value"}
		}
		if maxStr != "*" && parseInt(maxStr) == nil {
			return &cardinalityParseError{msg: "invalid maximum value"}
		}

		// Validate min <= max if both are numbers
		if maxStr != "*" {
			minVal := parseInt(minStr)
			maxVal := parseInt(maxStr)
			if minVal != nil && maxVal != nil && *minVal > *maxVal {
				return &cardinalityParseError{msg: "minimum cannot exceed maximum"}
			}
		}
		return nil
	}

	// Single value
	if parseInt(notation) == nil {
		return &cardinalityParseError{msg: "invalid format, expected number or range (e.g., '1', '0..1', '1..*')"}
	}
	return nil
}

type cardinalityParseError struct {
	msg string
}

func (e *cardinalityParseError) Error() string {
	return e.msg
}

// Schema output interface methods for Metamodel

// GetVersion returns the metamodel version
func (m *Metamodel) GetVersion() string {
	return m.Version
}

// GetNamespace returns the metamodel namespace
func (m *Metamodel) GetNamespace() string {
	return m.Namespace
}

// GetEntities returns the entities map for JSON output
func (m *Metamodel) GetEntities() interface{} {
	return m.Entities
}

// GetRelations returns the relations map for JSON output
func (m *Metamodel) GetRelations() interface{} {
	return m.Relations
}

// GetTypes returns the custom types map for JSON output
func (m *Metamodel) GetTypes() interface{} {
	return m.Types
}

// Schema output interface methods for EntityDef

// GetLabel returns the entity label
func (e *EntityDef) GetLabel() string {
	return e.Label
}

// GetAliases returns the entity aliases
func (e *EntityDef) GetAliases() []string {
	return e.Aliases
}

// GetIDPatterns returns the entity ID patterns
func (e *EntityDef) GetIDPatterns() []string {
	return e.IDPatterns
}

// GetProperties returns the entity properties for JSON output
func (e *EntityDef) GetProperties() interface{} {
	return e.Properties
}

// GetRDFType returns the RDF type
func (e *EntityDef) GetRDFType() string {
	return e.RDFType
}

// GetColor returns the color
func (e *EntityDef) GetColor() string {
	return e.Color
}

// GetBorderColor returns the border color
func (e *EntityDef) GetBorderColor() string {
	return e.BorderColor
}

// Schema output interface methods for RelationDef

// GetLabel returns the relation label
func (r *RelationDef) GetLabel() string {
	return r.Label
}

// GetFrom returns the source entity types
func (r *RelationDef) GetFrom() []string {
	return r.From
}

// GetTo returns the target entity types
func (r *RelationDef) GetTo() []string {
	return r.To
}

// GetDescription returns the relation description
func (r *RelationDef) GetDescription() string {
	return r.Description
}

// GetInverse returns the inverse definition for JSON output
func (r *RelationDef) GetInverse() interface{} {
	if r.Inverse == nil {
		return nil
	}
	return r.Inverse
}

// IsSymmetric returns whether the relation is symmetric
func (r *RelationDef) IsSymmetric() bool {
	return r.Symmetric
}

// GetFromMin returns the minimum cardinality on the "from" side.
// Checks new Cardinality.From first, then falls back to deprecated SourceMin.
func (r *RelationDef) GetFromMin() *int {
	if r.Cardinality != nil && r.Cardinality.From != "" {
		minVal, _ := ParseCardinality(r.Cardinality.From)
		return minVal
	}
	return r.SourceMin
}

// GetFromMax returns the maximum cardinality on the "from" side.
// Checks new Cardinality.From first, then falls back to deprecated SourceMax.
func (r *RelationDef) GetFromMax() *int {
	if r.Cardinality != nil && r.Cardinality.From != "" {
		_, maxVal := ParseCardinality(r.Cardinality.From)
		return maxVal
	}
	return r.SourceMax
}

// GetToMin returns the minimum cardinality on the "to" side.
// Checks new Cardinality.To first, then falls back to deprecated TargetMin.
func (r *RelationDef) GetToMin() *int {
	if r.Cardinality != nil && r.Cardinality.To != "" {
		minVal, _ := ParseCardinality(r.Cardinality.To)
		return minVal
	}
	return r.TargetMin
}

// GetToMax returns the maximum cardinality on the "to" side.
// Checks new Cardinality.To first, then falls back to deprecated TargetMax.
func (r *RelationDef) GetToMax() *int {
	if r.Cardinality != nil && r.Cardinality.To != "" {
		_, maxVal := ParseCardinality(r.Cardinality.To)
		return maxVal
	}
	return r.TargetMax
}

// GetSourceMin returns the source minimum cardinality.
// Deprecated: Use GetFromMin instead.
func (r *RelationDef) GetSourceMin() *int {
	return r.GetFromMin()
}

// GetSourceMax returns the source maximum cardinality.
// Deprecated: Use GetFromMax instead.
func (r *RelationDef) GetSourceMax() *int {
	return r.GetFromMax()
}

// GetTargetMin returns the target minimum cardinality.
// Deprecated: Use GetToMin instead.
func (r *RelationDef) GetTargetMin() *int {
	return r.GetToMin()
}

// GetTargetMax returns the target maximum cardinality.
// Deprecated: Use GetToMax instead.
func (r *RelationDef) GetTargetMax() *int {
	return r.GetToMax()
}

// ParseCardinality parses UML-style cardinality notation.
// Supported formats: "1", "0..1", "1..*", "0..*", "*", "2..5"
// Returns (minVal, maxVal) where nil maxVal means unbounded.
func ParseCardinality(notation string) (minVal, maxVal *int) {
	if notation == "" {
		return nil, nil
	}

	// Handle "*" as shorthand for "0..*"
	if notation == "*" {
		zero := 0
		return &zero, nil
	}

	// Check for range notation "min..max"
	if idx := findDoubleDot(notation); idx > 0 {
		minStr := notation[:idx]
		maxStr := notation[idx+2:]

		minVal := parseInt(minStr)
		if minVal == nil {
			return nil, nil // Invalid format
		}

		var maxVal *int
		if maxStr != "*" {
			maxVal = parseInt(maxStr)
			if maxVal == nil {
				return nil, nil // Invalid format
			}
		}
		// maxStr == "*" means unbounded (nil)

		return minVal, maxVal
	}

	// Single value means exactly that number
	val := parseInt(notation)
	if val == nil {
		return nil, nil
	}
	return val, val
}

// findDoubleDot finds the index of ".." in a string, returns -1 if not found
func findDoubleDot(s string) int {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return i
		}
	}
	return -1
}

// parseInt parses a string to an int pointer, returns nil on error
func parseInt(s string) *int {
	val := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return nil
		}
		val = val*10 + int(c-'0')
	}
	result := val
	return &result
}

// FormatCardinality converts min/max integers to UML notation string.
func FormatCardinality(minPtr, maxPtr *int) string {
	if minPtr == nil && maxPtr == nil {
		return ""
	}

	minVal := 0
	if minPtr != nil {
		minVal = *minPtr
	}

	if maxPtr == nil {
		// Unbounded
		if minVal == 0 {
			return "*"
		}
		return intToString(minVal) + "..*"
	}

	maxVal := *maxPtr
	if minVal == maxVal {
		return intToString(minVal)
	}

	return intToString(minVal) + ".." + intToString(maxVal)
}

// intToString converts an int to a string without using strconv
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
