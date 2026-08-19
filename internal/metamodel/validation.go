package metamodel

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// validIDPrefixBase matches a prefix after trimming one trailing dash:
// the character set entity IDs allow. Dashes are permitted here — the
// dash-run constraints (no "--", no trailing dash on the base) are
// checked separately below so the non-dash case gets a more targeted
// error message.
var validIDPrefixBase = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// unsafeSchemaNameChar matches any character that must NOT appear in an
// entity-type or property name because these names are interpolated into
// backend DDL by the derived-schema reconciler (a partial unique index over
// `properties->>'<name>'` per `type = '<type>'`, TKT-3Q0GP1). The reconciler
// escapes them as SQL string LITERALS (there is no bind parameter for a DDL
// identifier or JSON key), so the only characters that can break out are the
// single-quote and backslash; control characters (newlines, tabs, NUL) are
// forbidden defensively — they have no legitimate use in a name and each is a
// classic escaping-bypass vector. Everything else a YAML metamodel legitimately
// uses — letters (incl. non-ASCII), digits, underscore, dash, internal spaces,
// dots — is permitted, matching the metamodel's existing lenient naming.
var unsafeSchemaNameChar = regexp.MustCompile(`['\\\x00-\x1f\x7f]`)

// ValidateSchemaName reports whether name is safe to interpolate into
// reconciler DDL (see [unsafeSchemaNameChar]). It is intentionally a blocklist
// of dangerous characters rather than an allowlist, because entity-type and
// property names in shipped metamodels legitimately use dashes and internal
// spaces (e.g. "review-response", "some property"); an allowlist would reject
// existing valid schemas. It also rejects a leading/trailing space, which is a
// likely typo and confuses the DDL literal. Exported so the reconciler can
// re-check as defense-in-depth before emitting DDL rather than trusting that
// load-time validation ran.
func ValidateSchemaName(name string) error {
	if name == "" {
		return errors.New("name must not be empty")
	}
	if loc := unsafeSchemaNameChar.FindStringIndex(name); loc != nil {
		return fmt.Errorf("name %q contains an unsafe character at position %d "+
			"(quotes, backslashes, and control characters are not allowed)", name, loc[0])
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("name %q must not have leading or trailing whitespace", name)
	}
	return nil
}

// ValidateIDPrefix rejects id_prefix values whose generated IDs would
// fail entity ID validation (BUG-RHFHTH). Generated short/sequential
// IDs have the shape <base>-<suffix>, where base is the prefix with
// one trailing dash trimmed — so the base must be non-empty, contain
// only [A-Za-z0-9_-], and neither contain nor end in a dash run that
// would re-create the forbidden "--" sequence (reserved as the
// relation key separator). Enforced at metamodel load;
// entity.GenerateShortID assumes a load-validated prefix.
func ValidateIDPrefix(prefix string) error {
	base := strings.TrimSuffix(prefix, "-")
	if base == "" {
		return fmt.Errorf("id_prefix %q has no characters besides %q", prefix, "-")
	}
	if !validIDPrefixBase.MatchString(base) {
		return fmt.Errorf(
			"id_prefix %q contains characters not allowed in entity IDs (allowed: A-Z a-z 0-9 _ -)", prefix)
	}
	if strings.Contains(base, "--") || strings.HasSuffix(base, "-") {
		return fmt.Errorf(
			"id_prefix %q would generate IDs with consecutive dashes (\"--\" is the relation key separator)",
			prefix)
	}
	return nil
}

// ValidationErrorType indicates the kind of validation error.
type ValidationErrorType string

const (
	ValidationErrorRequired     ValidationErrorType = "required"
	ValidationErrorInvalidValue ValidationErrorType = "invalid_value"
	ValidationErrorInvalidType  ValidationErrorType = "invalid_type"
	ValidationErrorUnknownType  ValidationErrorType = "unknown_type"
	ValidationErrorIDPrefix     ValidationErrorType = "id_prefix"
	// ValidationErrorUnique reports a duplicate value for a property
	// declared `unique: true`. It is a HARD error (IsSoft returns false):
	// a natural-key collision is a constraint violation the write path
	// must reject with a 422, not a tolerable hand-edit state. Unlike the
	// other types, this one is raised by the entitymanager write path
	// (which can query other entities), not by the pure per-entity
	// [Metamodel.ValidateEntity].
	ValidationErrorUnique ValidationErrorType = "unique"
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

// IsSoft reports whether the error describes a soft condition per
// DEC-HWZHA — a state a hand-edited markdown file can produce that
// the API should tolerate at write time and surface as a warning
// rather than reject with a 422.
//
// Property-level mistakes (required-field-missing, type mismatch,
// invalid value such as out-of-enum / bad date / bad RRULE) are soft:
// the file already on disk likely contains them after a hand-edit, so
// rejecting them on the next API write would create a hostile
// asymmetry. Entity-level structural problems (unknown entity type,
// ID prefix that doesn't match the type) are hard: the storage layer
// can't construct a path to persist the entity at all.
//
// The categorization lives next to the error type so every consumer
// (workspace, future per-edge endpoints, MCP, etc.) gets a single
// authoritative answer.
func (e *ValidationError) IsSoft() bool {
	//exhaustive:ignore // Default-false fall-through is the intent.
	switch e.Type {
	case ValidationErrorRequired,
		ValidationErrorInvalidType,
		ValidationErrorInvalidValue:
		return true
	}
	return false
}

// ValidateProperties validates a properties map against a PropertySchema.
// This is shared between entity and relation validation.
func (m *Metamodel) ValidateProperties(props map[string]any, schema PropertySchema) []*ValidationError {
	var errs []*ValidationError

	// Check required properties
	for propName, propDef := range schema.PropertyDefs() {
		if propDef.Required {
			val, exists := props[propName]
			if !exists || val == nil || val == "" || isEmptyList(val) {
				errs = append(errs, &ValidationError{
					Type:     ValidationErrorRequired,
					Property: propName,
					Message:  "This field is required",
				})
			}
		}
	}

	// Validate property types
	for propName, propDef := range schema.PropertyDefs() {
		val, exists := props[propName]
		if !exists || val == nil {
			continue
		}

		// Skip empty strings and empty lists - they represent "no value".
		// For required properties, this is already reported as missing above.
		if val == "" || isEmptyList(val) {
			continue
		}

		if err := m.validatePropertyValue(propName, &propDef, val); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// ValidateEntity validates an entity's type, properties, and ID prefix against the metamodel.
func (m *Metamodel) ValidateEntity(id, entityType string, properties map[string]any) []*ValidationError {
	var errs []*ValidationError

	def, ok := m.GetEntityDef(entityType)
	if !ok {
		errs = append(errs, &ValidationError{
			Type:    ValidationErrorUnknownType,
			Message: "unknown entity type: " + entityType,
		})
		return errs
	}

	// Validate properties using shared function
	errs = append(errs, m.ValidateProperties(properties, def)...)

	// Validate ID matches prefix
	prefixes := def.GetIDPrefixes()
	if len(prefixes) > 0 {
		matched := false
		for _, prefix := range prefixes {
			if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
				matched = true
				break
			}
		}
		if !matched {
			errs = append(errs, &ValidationError{
				Type:    ValidationErrorIDPrefix,
				Message: fmt.Sprintf("entity ID %s does not match any prefix for type %s: %v", id, entityType, prefixes),
			})
		}
	}

	return errs
}

// ValidateRelationProperties validates a relation's properties against the metamodel.
func (m *Metamodel) ValidateRelationProperties(
	relationType string, properties map[string]any,
) []*ValidationError {
	def, ok := m.Relations[relationType]
	if !ok {
		return nil // Unknown type - handled elsewhere
	}

	if len(def.Properties) == 0 {
		return nil // No properties defined for this relation type
	}

	return m.ValidateProperties(properties, &def)
}

// isEmptyList reports whether val is a zero-length slice. Both []string
// (coerced from form submissions) and []interface{} (from YAML frontmatter)
// are treated as list values.
func isEmptyList(val any) bool {
	switch v := val.(type) {
	case []string:
		return len(v) == 0
	case []any:
		return len(v) == 0
	}
	return false
}

// ValidatePropertyValue validates a single property value against its definition.
// Returns a plain error for backward compatibility with existing callers.
//
// Note: The explicit nil check is required because returning a nil *ValidationError
// directly as error creates a non-nil interface with nil value (Go interface gotcha).
func (m *Metamodel) ValidatePropertyValue(propName string, propDef *PropertyDef, val any) error {
	err := m.validatePropertyValue(propName, propDef, val)
	if err != nil {
		return err
	}
	return nil
}

// validatePropertyValue validates a single property value and returns a structured ValidationError.
//
//nolint:funlen // large switch for property type validation; splitting would reduce readability
func (m *Metamodel) validatePropertyValue(propName string, propDef *PropertyDef, val any) *ValidationError {
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

	case PropertyTypeDatetime:
		// A datetime value is an RFC3339 instant. yaml.v3 auto-decodes an
		// unquoted timestamp scalar to time.Time, while machine-written /
		// quoted values arrive as string — accept both (RR-NY7PRB). A bare
		// date (no time-of-day) is accepted and means midnight; we do NOT
		// reject it, because after parsing "2026-07-13" and
		// "2026-07-13T00:00:00Z" are indistinguishable time.Time values
		// (RR-MYC2B6).
		switch v := val.(type) {
		case time.Time:
			// Already a valid instant.
		case string:
			if _, err := ParseDateValue(v, propDef); err != nil {
				format := propDef.GetDateFormat()
				return &ValidationError{
					Type:     ValidationErrorInvalidValue,
					Property: propName,
					Message:  fmt.Sprintf("Invalid datetime %q (expected format: %s)", v, format),
				}
			}
		default:
			return &ValidationError{
				Type:     ValidationErrorInvalidType,
				Property: propName,
				Message:  "Must be a datetime string or timestamp",
			}
		}

	case PropertyTypeInteger:
		switch v := val.(type) {
		case int, int64:
			// OK
		case float64:
			// YAML parses bare integers (count: 3) as int, but a value
			// with a fractional part (count: 3.5) arrives as float64.
			// Accept only when it is integral — silently truncating 3.5
			// to 3 would corrupt the value on a hand-edit typo.
			if v != math.Trunc(v) {
				return &ValidationError{
					Type:     ValidationErrorInvalidValue,
					Property: propName,
					Message:  fmt.Sprintf("Invalid integer %v (must not have a fractional part)", v),
				}
			}
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
			valid := slices.Contains(propDef.Values, s)
			if !valid {
				return &ValidationError{
					Type:     ValidationErrorInvalidValue,
					Property: propName,
					Message:  fmt.Sprintf("Invalid value %q (allowed: %v)", s, propDef.Values),
				}
			}
		}

	case PropertyTypeRrule:
		s, ok := val.(string)
		if !ok {
			return &ValidationError{
				Type:     ValidationErrorInvalidType,
				Property: propName,
				Message:  "Must be an RRULE string",
			}
		}
		if err := ValidateRrule(s); err != nil {
			return &ValidationError{
				Type:     ValidationErrorInvalidValue,
				Property: propName,
				Message:  err.Error(),
			}
		}

	case PropertyTypeFile:
		// File-type properties hold attachment path string(s) pointing at
		// blobs in the attachment store. With the default cap (1) the value
		// is a single string; with max > 1 it is a list of strings.
		// Structural validation is just "string(s)"; content-level checks
		// (file exists, hash matches) are the attachment store's concern.
		return validateFileValue(propName, propDef, val)

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
// validateFileValue checks a `file`-type property value. The value may be
// a single string path or a list of string paths regardless of the cap
// (a 1-element list is tolerated even at FileMax()==1, consistent with
// rela's permissive-storage philosophy); the list length must not exceed
// the cap. Content-level checks (the blob exists, hash matches) are the
// attachment store's concern, not this.
func validateFileValue(propName string, propDef *PropertyDef, val any) *ValidationError {
	maxCount := propDef.FileMax()

	// Coerce to a list of paths regardless of scalar/list shape so the
	// count check and item-type check share one path.
	var paths []string
	switch v := val.(type) {
	case string:
		paths = []string{v}
	case []string:
		paths = v
	case []any:
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return &ValidationError{
					Type:     ValidationErrorInvalidType,
					Property: propName,
					Message:  fmt.Sprintf("item[%d]: must be a string (attachment path)", i),
				}
			}
			paths = append(paths, s)
		}
	default:
		return &ValidationError{
			Type:     ValidationErrorInvalidType,
			Property: propName,
			Message:  "Must be a string or list of strings (attachment path)",
		}
	}

	if len(paths) > maxCount {
		return &ValidationError{
			Type:     ValidationErrorInvalidValue,
			Property: propName,
			Message:  fmt.Sprintf("at most %d attachment(s) allowed, got %d", maxCount, len(paths)),
		}
	}
	return nil
}

func validateCustomTypeValue(propName string, customType CustomType, val any) *ValidationError {
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

	// Handle []string (multi-select from form submission).
	// An empty list means "no value" and is the caller's job to reject
	// via the required check — here we only validate present items.
	if list, ok := val.([]string); ok {
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

	// Handle []interface{} (from YAML parsing).
	// An empty list is treated as "no value" — see []string branch above.
	if list, ok := val.([]any); ok {
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

// ParseIntegerValue parses an integer from various input types. A
// float64 is accepted only when it has no fractional part — truncating
// 3.5 to 3 would silently corrupt the value (matching the integer
// property-validation rule).
func ParseIntegerValue(val any) (int, error) {
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != math.Trunc(v) {
			return 0, fmt.Errorf("invalid integer %v (must not have a fractional part)", v)
		}
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot parse %T as integer", val)
	}
}

// ParseBooleanValue parses a boolean from various input types
func ParseBooleanValue(val any) (bool, error) {
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
