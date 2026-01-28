package metamodel

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// ValidateEntity validates an entity against the metamodel
func (m *Metamodel) ValidateEntity(entity *model.Entity) []error {
	var errs []error

	def, ok := m.GetEntityDef(entity.Type)
	if !ok {
		errs = append(errs, fmt.Errorf("unknown entity type: %s", entity.Type))
		return errs
	}

	// Check required properties
	for propName, propDef := range def.Properties {
		if propDef.Required {
			val, exists := entity.Properties[propName]
			if !exists || val == nil || val == "" {
				errs = append(errs, fmt.Errorf("missing required property: %s", propName))
			}
		}
	}

	// Validate property types
	for propName, propDef := range def.Properties {
		val, exists := entity.Properties[propName]
		if !exists || val == nil {
			continue
		}

		// Skip empty strings for required properties - already reported as missing
		if propDef.Required && val == "" {
			continue
		}

		if err := m.ValidatePropertyValue(propName, &propDef, val); err != nil {
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
			errs = append(errs, fmt.Errorf("entity ID %s does not match any prefix for type %s: %v",
				entity.ID, entity.Type, prefixes))
		}
	}

	return errs
}

// ValidateRelation validates that a relation is allowed by the metamodel
func (m *Metamodel) ValidateRelationEntities(relationType string, from, to *model.Entity) error {
	return m.ValidateRelation(relationType, from.Type, to.Type)
}

// ValidatePropertyValue validates a single property value against its definition
func (m *Metamodel) ValidatePropertyValue(propName string, propDef *PropertyDef, val interface{}) error {
	switch propDef.Type {
	case PropertyTypeString:
		if _, ok := val.(string); !ok {
			return fmt.Errorf("property %s must be a string", propName)
		}

	case PropertyTypeDate:
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("property %s must be a date string", propName)
		}
		// Use ParseDateValue to validate - it accepts the configured format plus common fallbacks
		if _, err := ParseDateValue(s, propDef); err != nil {
			format := propDef.GetDateFormat()
			return fmt.Errorf("invalid date %q for property %s (expected format: %s)", s, propName, format)
		}

	case PropertyTypeInteger:
		switch v := val.(type) {
		case int, int64, float64:
			// OK - YAML may parse integers as these types
		case string:
			if _, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("invalid integer %q for property %s", v, propName)
			}
		default:
			return fmt.Errorf("property %s must be an integer", propName)
		}

	case PropertyTypeBoolean:
		switch v := val.(type) {
		case bool:
			// OK
		case string:
			if v != "true" && v != "false" {
				return fmt.Errorf("property %s must be true or false, got %q", propName, v)
			}
		default:
			return fmt.Errorf("property %s must be a boolean", propName)
		}

	case PropertyTypeEnum:
		if propDef.Values != nil {
			s, ok := val.(string)
			if !ok {
				return fmt.Errorf("property %s must be a string", propName)
			}
			valid := false
			for _, v := range propDef.Values {
				if v == s {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid value for %s: %s (allowed: %v)", propName, s, propDef.Values)
			}
		}

	case "status":
		// Legacy built-in status type
		if s, ok := val.(string); ok {
			if !model.Status(s).IsValid() {
				return fmt.Errorf("invalid status value: %s", s)
			}
		}

	case "priority":
		// Legacy built-in priority type
		if p, ok := val.(string); ok {
			if !model.Priority(p).IsValid() {
				return fmt.Errorf("invalid priority value: %s", p)
			}
		}

	default:
		// Check if it's a custom type (enum defined in types section)
		if customType, ok := m.Types[propDef.Type]; ok {
			s, ok := val.(string)
			if !ok {
				return fmt.Errorf("property %s must be a string", propName)
			}
			valid := false
			for _, v := range customType.Values {
				if v == s {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid value for %s: %s (allowed: %v)", propName, s, customType.Values)
			}
		}
	}

	return nil
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
