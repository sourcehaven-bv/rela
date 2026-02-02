package filter

import (
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Sort sorts entities by a property with type-aware comparison.
// The meta parameter is optional but required for proper enum/custom type ordering.
func Sort(
	entities []*model.Entity,
	propName string,
	propDef *metamodel.PropertyDef,
	meta *metamodel.Metamodel,
	descending bool,
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

// compareDates compares two values as dates.
// Handles both string values (from JSON cache) and time.Time values (from YAML parsing).
func compareDates(valI, valJ interface{}, propDef *metamodel.PropertyDef) bool {
	dateI, okI := toTime(valI, propDef)
	dateJ, okJ := toTime(valJ, propDef)
	if !okI || !okJ {
		return compareStrings(valI, valJ)
	}
	return dateI.Before(dateJ)
}

// toTime converts a property value to time.Time.
// Supports string values (parsed via metamodel) and native time.Time values.
func toTime(val interface{}, propDef *metamodel.PropertyDef) (time.Time, bool) {
	switch v := val.(type) {
	case time.Time:
		return v, true
	case string:
		t, err := metamodel.ParseDateValue(v, propDef)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	default:
		return time.Time{}, false
	}
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

// SortMulti sorts entities by multiple criteria using stable sort.
// Specs are applied in priority order: first spec is the primary sort key,
// second spec is the tiebreaker, etc.
//
// The entityDefs map provides type definitions keyed by entity type name,
// enabling type-aware comparison for properties across different entity types.
// Both entityDefs and meta may be nil for basic string-only comparison.
//
// Virtual properties "id" and "modified" are supported:
//   - "id" sorts by Entity.ID
//   - "modified" sorts by Entity.ModTime
func SortMulti(
	entities []*model.Entity,
	specs []model.SortSpec,
	entityDefs map[string]*metamodel.EntityDef,
	meta *metamodel.Metamodel,
) {
	if len(specs) == 0 || len(entities) == 0 {
		return
	}

	// Apply sorts in reverse order (least significant key first).
	// Because SliceStable preserves order for equal elements,
	// the primary (first) key ends up dominant.
	for idx := len(specs) - 1; idx >= 0; idx-- {
		spec := specs[idx]
		sortBySingleSpec(entities, spec, entityDefs, meta)
	}
}

// sortBySingleSpec sorts entities by a single SortSpec with type-aware comparison.
func sortBySingleSpec(
	entities []*model.Entity,
	spec model.SortSpec,
	entityDefs map[string]*metamodel.EntityDef,
	meta *metamodel.Metamodel,
) {
	descending := spec.IsDescending()

	switch spec.Property {
	case "id":
		SortByID(entities, descending)
	case "modified":
		sortByModified(entities, descending)
	default:
		sortByProperty(entities, spec.Property, descending, entityDefs, meta)
	}
}

// sortByModified sorts entities by file modification time.
func sortByModified(entities []*model.Entity, descending bool) {
	sort.SliceStable(entities, func(i, j int) bool {
		ti := entities[i].ModTime
		tj := entities[j].ModTime

		// Zero times (unset) sort to end
		zi := ti.IsZero()
		zj := tj.IsZero()
		if zi && zj {
			return false
		}
		if zi {
			return false
		}
		if zj {
			return true
		}

		less := ti.Before(tj)
		if descending {
			return !less
		}
		return less
	})
}

// propInfo caches the property definition and enum index for a specific
// entity type, used during cross-type property sorting.
type propInfo struct {
	def       *metamodel.PropertyDef
	enumIndex map[string]int
}

// sortByProperty sorts entities by a named property with cross-type awareness.
func sortByProperty(
	entities []*model.Entity,
	propName string,
	descending bool,
	entityDefs map[string]*metamodel.EntityDef,
	meta *metamodel.Metamodel,
) {
	propInfoCache := make(map[string]*propInfo)

	getPropInfo := func(entityType string) *propInfo {
		if pi, ok := propInfoCache[entityType]; ok {
			return pi
		}
		var pi propInfo
		if entityDefs != nil {
			if entDef, ok := entityDefs[entityType]; ok {
				if pd, ok := entDef.Properties[propName]; ok {
					pi.def = &pd
					pi.enumIndex = buildEnumIndex(&pd, meta)
				}
			}
		}
		propInfoCache[entityType] = &pi
		return &pi
	}

	sort.SliceStable(entities, func(i, j int) bool {
		valI := entities[i].Properties[propName]
		valJ := entities[j].Properties[propName]

		// Handle nil values - sort them to the end
		if valI == nil && valJ == nil {
			return false
		}
		if valI == nil {
			return false
		}
		if valJ == nil {
			return true
		}

		piI := getPropInfo(entities[i].Type)
		piJ := getPropInfo(entities[j].Type)

		less := comparePropValues(valI, valJ, piI, piJ, meta)

		if descending {
			return !less
		}
		return less
	})
}

// comparePropValues compares two property values using their respective property definitions.
func comparePropValues(valI, valJ interface{}, piI, piJ *propInfo, meta *metamodel.Metamodel) bool {
	switch {
	case piI.def != nil && piJ.def != nil && piI.def.Type == piJ.def.Type:
		// Same property type on both entities — use type-aware comparison
		return compareByPropDef(valI, valJ, piI.def, piI.enumIndex)
	case piI.def != nil && piJ.def != nil:
		// Different property types — compare by type rank
		rankI := typeRank(piI.def, meta)
		rankJ := typeRank(piJ.def, meta)
		if rankI != rankJ {
			return rankI < rankJ
		}
		return compareStrings(valI, valJ)
	case piI.def != nil:
		// Only I has a property def — I comes first
		return true
	case piJ.def != nil:
		// Only J has a property def — J comes first
		return false
	default:
		// Neither has a property def — string comparison
		return compareStrings(valI, valJ)
	}
}

// compareByPropDef compares two values using the given property definition.
func compareByPropDef(valI, valJ interface{}, propDef *metamodel.PropertyDef, enumIndex map[string]int) bool {
	switch propDef.Type {
	case metamodel.PropertyTypeDate:
		return compareDates(valI, valJ, propDef)
	case metamodel.PropertyTypeInteger:
		return compareIntegers(valI, valJ)
	case metamodel.PropertyTypeBoolean:
		return compareBooleans(valI, valJ)
	case metamodel.PropertyTypeEnum:
		return compareEnums(valI, valJ, enumIndex)
	default:
		if enumIndex != nil {
			return compareEnums(valI, valJ, enumIndex)
		}
		return compareStrings(valI, valJ)
	}
}

// Type rank constants for cross-type property comparison.
// Lower rank sorts first when property types differ.
const (
	typeRankInteger = iota + 1
	typeRankDate
	typeRankBoolean
	typeRankEnum
	typeRankString
)

// typeRank returns a numeric rank for property types, used when comparing
// values across different property types. Lower rank sorts first.
func typeRank(propDef *metamodel.PropertyDef, meta *metamodel.Metamodel) int {
	switch propDef.Type {
	case metamodel.PropertyTypeInteger:
		return typeRankInteger
	case metamodel.PropertyTypeDate:
		return typeRankDate
	case metamodel.PropertyTypeBoolean:
		return typeRankBoolean
	case metamodel.PropertyTypeEnum:
		return typeRankEnum
	case metamodel.PropertyTypeString:
		return typeRankString
	default:
		// Custom types (enums) rank with enum
		if meta != nil {
			if _, ok := meta.Types[propDef.Type]; ok {
				return typeRankEnum
			}
		}
		return typeRankString
	}
}
