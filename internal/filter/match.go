package filter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Match checks if an entity property matches a filter
func Match(entity *model.Entity, filter *Filter, propDef *metamodel.PropertyDef, m *metamodel.Metamodel) (bool, error) {
	val := entity.Properties[filter.Property]

	// Handle nil/missing values
	if val == nil {
		// Only match if explicitly comparing to empty
		if filter.Operator == OpEqual && filter.Value == "" {
			return true, nil
		}
		if filter.Operator == OpNotEqual && filter.Value != "" {
			return true, nil
		}
		return false, nil
	}

	// Validate operator is supported for this property type
	if err := validateOperatorForType(filter.Operator, propDef, m); err != nil {
		return false, err
	}

	// Match based on property type
	switch propDef.Type {
	case metamodel.PropertyTypeString:
		return matchString(val, filter)
	case metamodel.PropertyTypeDate:
		return matchDate(val, filter, propDef)
	case metamodel.PropertyTypeInteger:
		return matchInteger(val, filter)
	case metamodel.PropertyTypeBoolean:
		return matchBoolean(val, filter)
	case metamodel.PropertyTypeEnum:
		return matchEnum(val, filter, propDef.Values)
	case "status", "priority":
		// Legacy built-in types - treat as enum
		return matchEnumLegacy(val, filter, propDef.Type, m)
	default:
		// Custom type - treat as enum
		if customType, ok := m.Types[propDef.Type]; ok {
			return matchEnum(val, filter, customType.Values)
		}
		// Unknown type - fall back to string comparison
		return matchString(val, filter)
	}
}

// MatchAll checks if entity matches all filters (AND semantics)
func MatchAll(
	entity *model.Entity, filters []*Filter, entityDef *metamodel.EntityDef, m *metamodel.Metamodel,
) (bool, error) {
	for _, f := range filters {
		propDef, ok := entityDef.Properties[f.Property]
		if !ok {
			return false, fmt.Errorf("unknown property %q for entity type %q", f.Property, entity.Type)
		}

		matches, err := Match(entity, f, &propDef, m)
		if err != nil {
			return false, err
		}
		if !matches {
			return false, nil
		}
	}
	return true, nil
}

// validateOperatorForType checks if an operator is valid for a property type
func validateOperatorForType(op Operator, propDef *metamodel.PropertyDef, m *metamodel.Metamodel) error {
	propType := propDef.Type

	// Determine the effective type category
	isEnum := propType == metamodel.PropertyTypeEnum || propType == "status" || propType == "priority"
	if !isEnum {
		if _, ok := m.Types[propType]; ok {
			isEnum = true
		}
	}

	switch propType {
	case metamodel.PropertyTypeString:
		// String supports: =, !=, =~ (regex)
		if op == OpLess || op == OpLessEqual || op == OpGreater || op == OpGreaterEqual {
			return fmt.Errorf("operator %q not supported for string property", op)
		}

	case metamodel.PropertyTypeDate, metamodel.PropertyTypeInteger:
		// Date and integer support: =, !=, <, <=, >, >=
		if op == OpRegex {
			return fmt.Errorf("operator %q not supported for %s property", op, propType)
		}

	case metamodel.PropertyTypeBoolean:
		// Boolean supports: =, !=
		if op != OpEqual && op != OpNotEqual {
			return fmt.Errorf("operator %q not supported for boolean property", op)
		}

	default:
		// Enum types (including custom types) support: =, !=
		if isEnum {
			if op != OpEqual && op != OpNotEqual {
				return fmt.Errorf("operator %q not supported for enum property", op)
			}
		}
	}

	return nil
}

// matchString matches a string property value
func matchString(val interface{}, filter *Filter) (bool, error) {
	s, ok := val.(string)
	if !ok {
		return false, fmt.Errorf("expected string value, got %T", val)
	}

	switch filter.Operator {
	case OpEqual:
		if filter.IsGlob {
			// Convert glob to regex and match
			pattern := GlobToRegex(filter.Value)
			re, err := regexp.Compile(pattern)
			if err != nil {
				return false, fmt.Errorf("invalid glob pattern: %w", err)
			}
			return re.MatchString(s), nil
		}
		return s == filter.Value, nil

	case OpNotEqual:
		if filter.IsGlob {
			pattern := GlobToRegex(filter.Value)
			re, err := regexp.Compile(pattern)
			if err != nil {
				return false, fmt.Errorf("invalid glob pattern: %w", err)
			}
			return !re.MatchString(s), nil
		}
		return s != filter.Value, nil

	case OpRegex:
		return filter.Regex.MatchString(s), nil

	default:
		return false, fmt.Errorf("operator %q not supported for string", filter.Operator)
	}
}

// matchDate matches a date property value
func matchDate(val interface{}, filter *Filter, propDef *metamodel.PropertyDef) (bool, error) {
	s, ok := val.(string)
	if !ok {
		return false, fmt.Errorf("expected date string value, got %T", val)
	}

	// Parse entity's date value
	entityDate, err := metamodel.ParseDateValue(s, propDef)
	if err != nil {
		return false, fmt.Errorf("invalid date value %q: %w", s, err)
	}

	// Parse filter's date value
	filterDate, err := metamodel.ParseDateValue(filter.Value, propDef)
	if err != nil {
		return false, fmt.Errorf("invalid date in filter %q (expected format: %s): %w",
			filter.Value, propDef.GetDateFormat(), err)
	}

	switch filter.Operator {
	case OpEqual:
		return entityDate.Equal(filterDate), nil
	case OpNotEqual:
		return !entityDate.Equal(filterDate), nil
	case OpLess:
		return entityDate.Before(filterDate), nil
	case OpLessEqual:
		return entityDate.Before(filterDate) || entityDate.Equal(filterDate), nil
	case OpGreater:
		return entityDate.After(filterDate), nil
	case OpGreaterEqual:
		return entityDate.After(filterDate) || entityDate.Equal(filterDate), nil
	default:
		return false, fmt.Errorf("operator %q not supported for date", filter.Operator)
	}
}

// matchInteger matches an integer property value
func matchInteger(val interface{}, filter *Filter) (bool, error) {
	entityVal, err := metamodel.ParseIntegerValue(val)
	if err != nil {
		return false, fmt.Errorf("invalid integer value: %w", err)
	}

	filterVal, err := metamodel.ParseIntegerValue(filter.Value)
	if err != nil {
		return false, fmt.Errorf("invalid integer in filter %q: %w", filter.Value, err)
	}

	switch filter.Operator {
	case OpEqual:
		return entityVal == filterVal, nil
	case OpNotEqual:
		return entityVal != filterVal, nil
	case OpLess:
		return entityVal < filterVal, nil
	case OpLessEqual:
		return entityVal <= filterVal, nil
	case OpGreater:
		return entityVal > filterVal, nil
	case OpGreaterEqual:
		return entityVal >= filterVal, nil
	default:
		return false, fmt.Errorf("operator %q not supported for integer", filter.Operator)
	}
}

// matchBoolean matches a boolean property value
func matchBoolean(val interface{}, filter *Filter) (bool, error) {
	entityVal, err := metamodel.ParseBooleanValue(val)
	if err != nil {
		return false, fmt.Errorf("invalid boolean value: %w", err)
	}

	filterVal, err := metamodel.ParseBooleanValue(filter.Value)
	if err != nil {
		return false, fmt.Errorf("invalid boolean in filter %q: %w", filter.Value, err)
	}

	switch filter.Operator {
	case OpEqual:
		return entityVal == filterVal, nil
	case OpNotEqual:
		return entityVal != filterVal, nil
	default:
		return false, fmt.Errorf("operator %q not supported for boolean", filter.Operator)
	}
}

// matchEnum matches an enum property value
func matchEnum(val interface{}, filter *Filter, allowedValues []string) (bool, error) {
	s, ok := val.(string)
	if !ok {
		return false, fmt.Errorf("expected string value for enum, got %T", val)
	}

	// Special case: allow empty string for = and != operators
	// This enables checking if a property has any value (e.g., "priority!=" means "has priority")
	if filter.Value == "" {
		switch filter.Operator {
		case OpEqual:
			return s == "", nil
		case OpNotEqual:
			return s != "", nil
		default:
			return false, fmt.Errorf("operator %q not supported for enum", filter.Operator)
		}
	}

	// Validate filter value is a valid enum value
	filterValid := false
	for _, v := range allowedValues {
		if v == filter.Value {
			filterValid = true
			break
		}
	}
	if !filterValid {
		return false, fmt.Errorf("invalid value %q (allowed: %s)", filter.Value, strings.Join(allowedValues, ", "))
	}

	switch filter.Operator {
	case OpEqual:
		return s == filter.Value, nil
	case OpNotEqual:
		return s != filter.Value, nil
	default:
		return false, fmt.Errorf("operator %q not supported for enum", filter.Operator)
	}
}

// matchEnumLegacy matches legacy status/priority types
func matchEnumLegacy(val interface{}, filter *Filter, legacyType string, m *metamodel.Metamodel) (bool, error) {
	s, ok := val.(string)
	if !ok {
		return false, fmt.Errorf("expected string value, got %T", val)
	}

	// Get allowed values from model package
	var allowedValues []string
	switch legacyType {
	case "status":
		allowedValues = []string{"draft", "proposed", "accepted", "deprecated", "rejected", "retired"}
	case "priority":
		allowedValues = []string{"critical", "high", "medium", "low"}
	}

	// Check if there's a custom type override
	if customType, ok := m.Types[legacyType]; ok {
		allowedValues = customType.Values
	}

	return matchEnum(s, filter, allowedValues)
}
