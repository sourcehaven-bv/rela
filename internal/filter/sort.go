package filter

import (
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// Sort sorts items by a property with type-aware comparison.
// The meta parameter is optional but required for proper enum/custom type ordering.
func Sort[T any](
	items []T,
	access Accessor[T],
	propName string,
	propDef *metamodel.PropertyDef,
	meta *metamodel.Metamodel,
	descending bool,
) {
	enumIndex := buildEnumIndex(propDef, meta)

	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := access(items[i]), access(items[j])
		valI := ri.Properties[propName]
		valJ := rj.Properties[propName]

		if valI == nil && valJ == nil {
			return false
		}
		if valI == nil {
			return false
		}
		if valJ == nil {
			return true
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
			less = compareEnums(valI, valJ, enumIndex)
		default:
			if enumIndex != nil {
				less = compareEnums(valI, valJ, enumIndex)
			} else {
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

	if len(propDef.Values) > 0 {
		values = propDef.Values
	} else if meta != nil && !metamodel.IsBuiltinType(propDef.Type) {
		if customType, ok := meta.Types[propDef.Type]; ok {
			values = customType.Values
		}
	}

	if len(values) == 0 {
		return nil
	}

	index := make(map[string]int, len(values))
	for i, v := range values {
		index[v] = i
	}
	return index
}

// compareEnums compares two enum values by their index position.
func compareEnums(valI, valJ interface{}, enumIndex map[string]int) bool {
	sI, okI := valI.(string)
	sJ, okJ := valJ.(string)
	if !okI || !okJ {
		return false
	}

	idxI, knownI := enumIndex[sI]
	idxJ, knownJ := enumIndex[sJ]

	if knownI && knownJ {
		return idxI < idxJ
	}
	if knownI && !knownJ {
		return true
	}
	if !knownI && knownJ {
		return false
	}
	return sI < sJ
}

// SortByID sorts items by ID using natural ordering.
func SortByID[T any](items []T, access Accessor[T], descending bool) {
	sort.SliceStable(items, func(i, j int) bool {
		less := natsort.Less(access(items[i]).ID, access(items[j]).ID)
		if descending {
			return !less
		}
		return less
	})
}

// compareStrings compares two values as strings using natural ordering.
func compareStrings(valI, valJ interface{}) bool {
	sI, okI := valI.(string)
	sJ, okJ := valJ.(string)
	if !okI || !okJ {
		return false
	}
	return natsort.Less(sI, sJ)
}

// compareDates compares two values as dates.
func compareDates(valI, valJ interface{}, propDef *metamodel.PropertyDef) bool {
	dateI, okI := toTime(valI, propDef)
	dateJ, okJ := toTime(valJ, propDef)
	if !okI || !okJ {
		return compareStrings(valI, valJ)
	}
	return dateI.Before(dateJ)
}

// toTime converts a property value to time.Time.
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

// compareIntegers compares two values as integers.
func compareIntegers(valI, valJ interface{}) bool {
	intI, errI := metamodel.ParseIntegerValue(valI)
	intJ, errJ := metamodel.ParseIntegerValue(valJ)
	if errI != nil || errJ != nil {
		return compareStrings(valI, valJ)
	}
	return intI < intJ
}

// compareBooleans compares two values as booleans (false < true).
func compareBooleans(valI, valJ interface{}) bool {
	boolI, errI := metamodel.ParseBooleanValue(valI)
	boolJ, errJ := metamodel.ParseBooleanValue(valJ)
	if errI != nil || errJ != nil {
		return false
	}
	return !boolI && boolJ
}

// SortMulti sorts items by multiple criteria using stable sort.
// Specs are applied in priority order: first spec is the primary sort key.
//
// Virtual properties "id" and "modified" are supported.
func SortMulti[T any](
	items []T,
	access Accessor[T],
	specs []SortSpec,
	entityDefs map[string]*metamodel.EntityDef,
	meta *metamodel.Metamodel,
) {
	if len(specs) == 0 || len(items) == 0 {
		return
	}

	// Apply sorts in reverse order (least significant key first).
	for idx := len(specs) - 1; idx >= 0; idx-- {
		spec := specs[idx]
		sortBySingleSpec(items, access, spec, entityDefs, meta)
	}
}

// sortBySingleSpec sorts items by a single SortSpec with type-aware comparison.
func sortBySingleSpec[T any](
	items []T,
	access Accessor[T],
	spec SortSpec,
	entityDefs map[string]*metamodel.EntityDef,
	meta *metamodel.Metamodel,
) {
	descending := spec.IsDescending()

	switch spec.Property {
	case "id":
		SortByID(items, access, descending)
	case "modified":
		sortByModified(items, access, descending)
	default:
		sortByProperty(items, access, spec.Property, descending, entityDefs, meta)
	}
}

// sortByModified sorts items by modification time.
func sortByModified[T any](items []T, access Accessor[T], descending bool) {
	sort.SliceStable(items, func(i, j int) bool {
		ti := access(items[i]).ModifiedAt
		tj := access(items[j]).ModifiedAt

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

// propInfo caches the property definition and enum index for a specific entity type.
type propInfo struct {
	def       *metamodel.PropertyDef
	enumIndex map[string]int
}

// sortByProperty sorts items by a named property with cross-type awareness.
func sortByProperty[T any](
	items []T,
	access Accessor[T],
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

	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := access(items[i]), access(items[j])
		valI := ri.Properties[propName]
		valJ := rj.Properties[propName]

		if valI == nil && valJ == nil {
			return false
		}
		if valI == nil {
			return false
		}
		if valJ == nil {
			return true
		}

		piI := getPropInfo(ri.Type)
		piJ := getPropInfo(rj.Type)

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
		return compareByPropDef(valI, valJ, piI.def, piI.enumIndex)
	case piI.def != nil && piJ.def != nil:
		rankI := typeRank(piI.def, meta)
		rankJ := typeRank(piJ.def, meta)
		if rankI != rankJ {
			return rankI < rankJ
		}
		return compareStrings(valI, valJ)
	case piI.def != nil:
		return true
	case piJ.def != nil:
		return false
	default:
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

const (
	typeRankInteger = iota + 1
	typeRankDate
	typeRankBoolean
	typeRankEnum
	typeRankString
)

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
		if meta != nil {
			if _, ok := meta.Types[propDef.Type]; ok {
				return typeRankEnum
			}
		}
		return typeRankString
	}
}
