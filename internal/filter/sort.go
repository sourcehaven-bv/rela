package filter

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Sort sorts entities by a property with type-aware comparison
func Sort(entities []*model.Entity, propName string, propDef *metamodel.PropertyDef, descending bool) {
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
		default:
			// String, enum, and custom types - lexicographic comparison
			less = compareStrings(valI, valJ)
		}

		if descending {
			return !less
		}
		return less
	})
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
