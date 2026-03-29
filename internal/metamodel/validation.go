package metamodel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// ValidationErrorType indicates the kind of validation error.
type ValidationErrorType string

const (
	ValidationErrorRequired     ValidationErrorType = "required"
	ValidationErrorInvalidValue ValidationErrorType = "invalid_value"
	ValidationErrorInvalidType  ValidationErrorType = "invalid_type"
	ValidationErrorUnknownType  ValidationErrorType = "unknown_type"
	ValidationErrorIDPrefix     ValidationErrorType = "id_prefix"
)

// ValidationError represents a structured validation error with field information.
type ValidationError struct {
	Type     ValidationErrorType
	Property string // The property name that failed validation (empty for entity-level errors)
	Message  string // Human-readable error message
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return e.Message
}

// ValidateEntity validates an entity against the metamodel.
// Returns a slice of *ValidationError for structured error handling.
func (m *Metamodel) ValidateEntity(entity *model.Entity) []*ValidationError {
	var errs []*ValidationError

	def, ok := m.GetEntityDef(entity.Type)
	if !ok {
		errs = append(errs, &ValidationError{
			Type:    ValidationErrorUnknownType,
			Message: fmt.Sprintf("unknown entity type: %s", entity.Type),
		})
		return errs
	}

	// Check required properties
	for propName, propDef := range def.Properties {
		if propDef.Required {
			val, exists := entity.Properties[propName]
			if !exists || val == nil || val == "" {
				errs = append(errs, &ValidationError{
					Type:     ValidationErrorRequired,
					Property: propName,
					Message:  "This field is required",
				})
			}
		}
	}

	// Validate property types
	for propName, propDef := range def.Properties {
		val, exists := entity.Properties[propName]
		if !exists || val == nil {
			continue
		}

		// Skip empty strings - they represent "no value"
		// For required properties, this is already reported as missing above
		if val == "" {
			continue
		}

		if err := m.validatePropertyValue(propName, &propDef, val); err != nil {
			errs = append(errs, err)
		}
	}

	// Validate ID matches prefix
	prefixes := def.GetIDPrefixes()
	if len(prefixes) > 0 {
		matched := false
		for _, prefix := range prefixes {
			if len(entity.ID) >= len(prefix) && entity.ID[:len(prefix)] == prefix {
				matched = true
				break
			}
		}
		if !matched {
			errs = append(errs, &ValidationError{
				Type:    ValidationErrorIDPrefix,
				Message: fmt.Sprintf("entity ID %s does not match any prefix for type %s: %v", entity.ID, entity.Type, prefixes),
			})
		}
	}

	return errs
}

// ValidateRelation validates that a relation is allowed by the metamodel
func (m *Metamodel) ValidateRelationEntities(relationType string, from, to *model.Entity) error {
	return m.ValidateRelation(relationType, from.Type, to.Type)
}

// ValidatePropertyValue validates a single property value against its definition.
// Returns a plain error for backward compatibility with existing callers.
//
// Note: The explicit nil check is required because returning a nil *ValidationError
// directly as error creates a non-nil interface with nil value (Go interface gotcha).
func (m *Metamodel) ValidatePropertyValue(propName string, propDef *PropertyDef, val interface{}) error {
	err := m.validatePropertyValue(propName, propDef, val)
	if err != nil {
		return err
	}
	return nil
}

// validatePropertyValue validates a single property value and returns a structured ValidationError.
//
//nolint:funlen // large switch for property type validation; splitting would reduce readability
func (m *Metamodel) validatePropertyValue(propName string, propDef *PropertyDef, val interface{}) *ValidationError {
	switch propDef.Type {
	case PropertyTypeString:
		if _, ok := val.(string); !ok {
			return &ValidationError{
				Type:     ValidationErrorInvalidType,
				Property: propName,
				Message:  "Must be a string",
			}
		}

	case PropertyTypeDate:
		s, ok := val.(string)
		if !ok {
			return &ValidationError{
				Type:     ValidationErrorInvalidType,
				Property: propName,
				Message:  "Must be a date string",
			}
		}
		// Use ParseDateValue to validate - it accepts the configured format plus common fallbacks
		if _, err := ParseDateValue(s, propDef); err != nil {
			format := propDef.GetDateFormat()
			return &ValidationError{
				Type:     ValidationErrorInvalidValue,
				Property: propName,
				Message:  fmt.Sprintf("Invalid date %q (expected format: %s)", s, format),
			}
		}

	case PropertyTypeInteger:
		switch v := val.(type) {
		case int, int64, float64:
			// OK - YAML may parse integers as these types
		case string:
			if _, err := strconv.Atoi(v); err != nil {
				return &ValidationError{
					Type:     ValidationErrorInvalidValue,
					Property: propName,
					Message:  fmt.Sprintf("Invalid integer %q", v),
				}
			}
		default:
			return &ValidationError{
				Type:     ValidationErrorInvalidType,
				Property: propName,
				Message:  "Must be an integer",
			}
		}

	case PropertyTypeBoolean:
		switch v := val.(type) {
		case bool:
			// OK
		case string:
			if v != "true" && v != "false" {
				return &ValidationError{
					Type:     ValidationErrorInvalidValue,
					Property: propName,
					Message:  fmt.Sprintf("Must be true or false, got %q", v),
				}
			}
		default:
			return &ValidationError{
				Type:     ValidationErrorInvalidType,
				Property: propName,
				Message:  "Must be a boolean",
			}
		}

	case PropertyTypeEnum:
		if propDef.Values != nil {
			s, ok := val.(string)
			if !ok {
				return &ValidationError{
					Type:     ValidationErrorInvalidType,
					Property: propName,
					Message:  "Must be a string",
				}
			}
			valid := false
			for _, v := range propDef.Values {
				if v == s {
					valid = true
					break
				}
			}
			if !valid {
				return &ValidationError{
					Type:     ValidationErrorInvalidValue,
					Property: propName,
					Message:  fmt.Sprintf("Invalid value %q (allowed: %v)", s, propDef.Values),
				}
			}
		}

	default:
		// Custom type (enum defined in types section)
		if customType, ok := m.Types[propDef.Type]; ok {
			return validateCustomTypeValue(propName, customType, val)
		}
		return &ValidationError{
			Type:     ValidationErrorUnknownType,
			Property: propName,
			Message:  fmt.Sprintf("Unknown type %q", propDef.Type),
		}
	}

	return nil
}

// validateCustomTypeValue validates a value against a custom type's allowed values and regex validations.
// Supports both single string values and []string (multi-select).
// Returns an error combining all validation failures.
//
//nolint:funlen // validation logic for multiple cases; splitting would reduce readability
func validateCustomTypeValue(propName string, customType CustomType, val interface{}) *ValidationError {
	hasEnumValues := len(customType.Values) > 0
	hasValidations := len(customType.Validations) > 0

	// If no values and no validations, treat as plain string (no validation needed)
	if !hasEnumValues && !hasValidations {
		if _, ok := val.(string); !ok {
			return &ValidationError{
				Type:     ValidationErrorInvalidType,
				Property: propName,
				Message:  "Must be a string",
			}
		}
		return nil
	}

	// Build allowed values map for enum validation
	allowed := make(map[string]bool, len(customType.Values))
	for _, v := range customType.Values {
		allowed[v] = true
	}

	// Handle []string (multi-select from form submission)
	if list, ok := val.([]string); ok {
		if len(list) == 0 && hasEnumValues {
			return &ValidationError{
				Type:     ValidationErrorInvalidValue,
				Property: propName,
				Message:  fmt.Sprintf("Empty list (allowed: %v)", customType.Values),
			}
		}
		// Collect all errors from all list items
		var allErrors []string
		for i, s := range list {
			if hasEnumValues && !allowed[s] {
				allErrors = append(allErrors, fmt.Sprintf("item[%d]: invalid value %q", i, s))
			}
			// Run regex validations on each item
			if err := validateRegexPatterns(propName, customType.Validations, s); err != nil {
				allErrors = append(allErrors, fmt.Sprintf("item[%d]: %s", i, err.Message))
			}
		}
		if len(allErrors) > 0 {
			return &ValidationError{
				Type:     ValidationErrorInvalidValue,
				Property: propName,
				Message:  strings.Join(allErrors, "; "),
			}
		}
		return nil
	}

	// Handle []interface{} (from YAML parsing)
	if list, ok := val.([]interface{}); ok {
		if len(list) == 0 && hasEnumValues {
			return &ValidationError{
				Type:     ValidationErrorInvalidValue,
				Property: propName,
				Message:  fmt.Sprintf("Empty list (allowed: %v)", customType.Values),
			}
		}
		// Collect all errors from all list items
		var allErrors []string
		for i, item := range list {
			s, ok := item.(string)
			if !ok {
				allErrors = append(allErrors, fmt.Sprintf("item[%d]: must be a string", i))
				continue
			}
			if hasEnumValues && !allowed[s] {
				allErrors = append(allErrors, fmt.Sprintf("item[%d]: invalid value %q", i, s))
			}
			// Run regex validations on each item
			if err := validateRegexPatterns(propName, customType.Validations, s); err != nil {
				allErrors = append(allErrors, fmt.Sprintf("item[%d]: %s", i, err.Message))
			}
		}
		if len(allErrors) > 0 {
			return &ValidationError{
				Type:     ValidationErrorInvalidValue,
				Property: propName,
				Message:  strings.Join(allErrors, "; "),
			}
		}
		return nil
	}

	// Handle single string value
	s, ok := val.(string)
	if !ok {
		return &ValidationError{
			Type:     ValidationErrorInvalidType,
			Property: propName,
			Message:  "Must be a string or list of strings",
		}
	}

	// Empty string handling:
	// - For enum types: empty is not a valid value, so fail
	// - For regex-only types: empty can be skipped (let 'required' handle it)
	if s == "" {
		if hasEnumValues {
			return &ValidationError{
				Type:     ValidationErrorInvalidValue,
				Property: propName,
				Message:  fmt.Sprintf("Invalid value %q (allowed: %v)", s, customType.Values),
			}
		}
		// For regex-only types, skip validation on empty
		return nil
	}

	// Validate against enum values if present
	if hasEnumValues && !allowed[s] {
		return &ValidationError{
			Type:     ValidationErrorInvalidValue,
			Property: propName,
			Message:  fmt.Sprintf("Invalid value %q (allowed: %v)", s, customType.Values),
		}
	}

	// Run regex validations
	return validateRegexPatterns(propName, customType.Validations, s)
}

// validateRegexPatterns validates a string value against a list of regex patterns.
// Returns an error containing all failing validation messages combined.
// Uses pre-compiled regexes cached during metamodel load.
func validateRegexPatterns(propName string, validations []TypeValidation, value string) *ValidationError {
	if len(validations) == 0 {
		return nil
	}

	var failedMessages []string

	for i := range validations {
		v := &validations[i]

		// Use the pre-compiled regex from metamodel load
		re := v.Compiled()
		if re == nil {
			// Fallback: compile if not cached (shouldn't happen in normal usage)
			var err error
			re, err = regexp.Compile(v.Pattern)
			if err != nil {
				failedMessages = append(failedMessages, fmt.Sprintf("[internal] invalid pattern: %v", err))
				continue
			}
		}

		if !re.MatchString(value) {
			failedMessages = append(failedMessages, v.Error)
		}
	}

	if len(failedMessages) == 0 {
		return nil
	}

	// Combine all error messages
	message := strings.Join(failedMessages, "; ")
	return &ValidationError{
		Type:     ValidationErrorInvalidValue,
		Property: propName,
		Message:  message,
	}
}

// ParseDateValue parses a date string using the property's format.
// It tries the specified format first, then falls back to common formats
// to handle dates stored with timestamps (e.g., from YAML parsing).
func ParseDateValue(s string, propDef *PropertyDef) (time.Time, error) {
	format := propDef.GetDateFormat()

	// Try the specified format first
	if t, err := time.Parse(format, s); err == nil {
		return t, nil
	}

	// Try common fallback formats (dates may be stored with timestamps)
	fallbackFormats := []string{
		time.RFC3339,           // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05Z", // ISO 8601 with Z
		"2006-01-02T15:04:05",  // ISO 8601 without timezone
		"2006-01-02",           // ISO 8601 date only
	}

	for _, f := range fallbackFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("parsing time %q: cannot parse with format %q or common fallbacks", s, format)
}

// ParseIntegerValue parses an integer from various input types
func ParseIntegerValue(val interface{}) (int, error) {
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot parse %T as integer", val)
	}
}

// ParseBooleanValue parses a boolean from various input types
func ParseBooleanValue(val interface{}) (bool, error) {
	switch v := val.(type) {
	case bool:
		return v, nil
	case string:
		if v == "true" {
			return true, nil
		}
		if v == "false" {
			return false, nil
		}
		return false, fmt.Errorf("invalid boolean value: %s", v)
	default:
		return false, fmt.Errorf("cannot parse %T as boolean", val)
	}
}
