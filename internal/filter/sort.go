package filter

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Sort sorts entities by a property with type-aware comparison.
// The meta parameter is optional but required for proper enum/custom type ordering.
func Sort(
	entities []*model.Entity, propName string, propDef *metamodel.PropertyDef,
	meta *metamodel.Metamodel, descending bool,
) {
	// Build enum value index map for efficient lookup
	enumIndex := buildEnumIndex(propDef, meta)

	sort.SliceStable(entities, func(i, j int) bool {
		valI := entities[i].Properties[propName]
		valJ := entities[j].Properties[propName]

		// Handle nil values - sort them to the end
		if valI == nil && valJ == nil {
			return false
		}
		if valI == nil {
			return false // nil goes after non-nil
		}
		if valJ == nil {
			return true // non-nil goes before nil
		}

		var less bool
		switch propDef.Type {
		case metamodel.PropertyTypeDate:
			less = compareDates(valI, valJ, propDef)
		case metamodel.PropertyTypeInteger:
			less = compareIntegers(valI, valJ)
		case metamodel.PropertyTypeBoolean:
			less = compareBooleans(valI, valJ)
		case metamodel.PropertyTypeEnum:
			// Inline enum with values defined in property
			less = compareEnums(valI, valJ, enumIndex)
		default:
			// Check if this is a custom type (not a built-in type)
			if enumIndex != nil {
				less = compareEnums(valI, valJ, enumIndex)
			} else {
				// Fall back to string comparison for unknown types
				less = compareStrings(valI, valJ)
			}
		}

		if descending {
			return !less
		}
		return less
	})
}

// buildEnumIndex creates a map from enum value to its index position.
// Returns nil if no enum values are found.
func buildEnumIndex(propDef *metamodel.PropertyDef, meta *metamodel.Metamodel) map[string]int {
	var values []string

	// First check for inline enum values in the property definition
	if len(propDef.Values) > 0 {
		values = propDef.Values
	} else if meta != nil && !metamodel.IsBuiltinType(propDef.Type) {
		// Look up custom type in metamodel
		if customType, ok := meta.Types[propDef.Type]; ok {
			values = customType.Values
		}
	}

	if len(values) == 0 {
		return nil
	}

	// Build index map for O(1) lookup
	index := make(map[string]int, len(values))
	for i, v := range values {
		index[v] = i
	}
	return index
}

// compareEnums compares two enum values by their index position.
// Unknown values are sorted after known values, then alphabetically among themselves.
func compareEnums(valI, valJ interface{}, enumIndex map[string]int) bool {
	sI, okI := valI.(string)
	sJ, okJ := valJ.(string)
	if !okI || !okJ {
		return false
	}

	idxI, knownI := enumIndex[sI]
	idxJ, knownJ := enumIndex[sJ]

	// Both known: compare by index
	if knownI && knownJ {
		return idxI < idxJ
	}

	// Known values come before unknown values
	if knownI && !knownJ {
		return true
	}
	if !knownI && knownJ {
		return false
	}

	// Both unknown: fall back to string comparison
	return sI < sJ
}

// SortByID sorts entities by their ID (default sort)
func SortByID(entities []*model.Entity, descending bool) {
	sort.SliceStable(entities, func(i, j int) bool {
		less := entities[i].ID < entities[j].ID
		if descending {
			return !less
		}
		return less
	})
}

// compareStrings compares two values as strings
func compareStrings(valI, valJ interface{}) bool {
	sI, okI := valI.(string)
	sJ, okJ := valJ.(string)
	if !okI || !okJ {
		return false
	}
	return sI < sJ
}

// compareDates compares two values as dates
func compareDates(valI, valJ interface{}, propDef *metamodel.PropertyDef) bool {
	sI, okI := valI.(string)
	sJ, okJ := valJ.(string)
	if !okI || !okJ {
		return false
	}

	dateI, errI := metamodel.ParseDateValue(sI, propDef)
	dateJ, errJ := metamodel.ParseDateValue(sJ, propDef)
	if errI != nil || errJ != nil {
		// Fall back to string comparison if parsing fails
		return sI < sJ
	}

	return dateI.Before(dateJ)
}

// compareIntegers compares two values as integers
func compareIntegers(valI, valJ interface{}) bool {
	intI, errI := metamodel.ParseIntegerValue(valI)
	intJ, errJ := metamodel.ParseIntegerValue(valJ)
	if errI != nil || errJ != nil {
		// Fall back to string comparison if parsing fails
		return compareStrings(valI, valJ)
	}

	return intI < intJ
}

// compareBooleans compares two values as booleans (false < true)
func compareBooleans(valI, valJ interface{}) bool {
	boolI, errI := metamodel.ParseBooleanValue(valI)
	boolJ, errJ := metamodel.ParseBooleanValue(valJ)
	if errI != nil || errJ != nil {
		return false
	}

	// false < true
	if !boolI && boolJ {
		return true
	}
	return false
}
