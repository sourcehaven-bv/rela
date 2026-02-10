package metamodel

import "strings"

// RelationNotFoundError is returned when a relation type is not defined in the metamodel.
type RelationNotFoundError struct {
	Name string
}

func (e *RelationNotFoundError) Error() string {
	return "unknown relation: " + e.Name
}

// InvalidRelationError is returned when a relation is not valid between two entity types.
type InvalidRelationError struct {
	Relation string
	From     string
	To       string
	Message  string
}

func (e *InvalidRelationError) Error() string {
	return "invalid relation " + e.Relation + " from " + e.From + " to " + e.To + ": " + e.Message
}

// InvalidIDTypeError is returned when an entity has an invalid id_type value.
type InvalidIDTypeError struct {
	EntityType string
	IDType     string
}

func (e *InvalidIDTypeError) Error() string {
	return "invalid id_type for entity " + e.EntityType + ": " + e.IDType + " (must be 'short', 'sequential', or 'manual')"
}

// ReservedPropertyError is returned when a property name conflicts with a reserved name.
type ReservedPropertyError struct {
	EntityType   string
	PropertyName string
}

func (e *ReservedPropertyError) Error() string {
	return "entity " + e.EntityType + ": property \"" + e.PropertyName + "\" is reserved and cannot be used"
}

// WhitespacePropertyError is returned when a property name has leading or trailing whitespace.
type WhitespacePropertyError struct {
	EntityType   string
	PropertyName string
}

func (e *WhitespacePropertyError) Error() string {
	return "entity " + e.EntityType + ": property name \"" + e.PropertyName + "\" has leading or trailing whitespace"
}

// ConflictingIDPrefixError is returned when both id_prefix and id_prefixes are specified.
type ConflictingIDPrefixError struct {
	EntityType string
}

func (e *ConflictingIDPrefixError) Error() string {
	return "entity " + e.EntityType + " specifies both id_prefix and id_prefixes; use only one"
}

// ReservedTypeNameError is returned when a custom type name conflicts with a built-in type.
type ReservedTypeNameError struct {
	TypeName string
}

func (e *ReservedTypeNameError) Error() string {
	return "cannot define custom type \"" + e.TypeName +
		"\": name is reserved for built-in type (reserved: string, date, integer, boolean, enum)"
}

// SchemaValidationError collects multiple validation issues found in a metamodel.
type SchemaValidationError struct {
	Errors []string
}

func (e *SchemaValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0]
	}
	return "metamodel validation errors:\n  - " + strings.Join(e.Errors, "\n  - ")
}

// DuplicateDefinitionError is returned when the same name is defined in multiple included files.
type DuplicateDefinitionError struct {
	Kind  string // "type", "entity", "relation", or "validation"
	Name  string
	File1 string
	File2 string
}

func (e *DuplicateDefinitionError) Error() string {
	return "duplicate " + e.Kind + " \"" + e.Name + "\": defined in both " + e.File1 + " and " + e.File2
}

// CircularIncludeError is returned when a circular include chain is detected.
type CircularIncludeError struct {
	Chain []string // e.g., ["a.yaml", "b.yaml", "a.yaml"]
}

func (e *CircularIncludeError) Error() string {
	return "circular include detected: " + strings.Join(e.Chain, " → ")
}

// IncludeNotFoundError is returned when an included file does not exist.
type IncludeNotFoundError struct {
	Path         string
	IncludedFrom string
}

func (e *IncludeNotFoundError) Error() string {
	return "include file not found: " + e.Path + " (included from " + e.IncludedFrom + ")"
}

// IncludeHasRootFieldError is returned when an included file contains version or namespace.
type IncludeHasRootFieldError struct {
	Path  string
	Field string
}

func (e *IncludeHasRootFieldError) Error() string {
	return "included file " + e.Path + " must not contain \"" + e.Field +
		"\" (only allowed in root metamodel.yaml)"
}
