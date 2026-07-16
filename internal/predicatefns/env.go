package predicatefns

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// EntityRecordType builds the predicate.RecordType for an entity type by
// mapping each declared property to its predicate type. This is the
// metamodel->Env adapter (planning task 4): it is what lets a predicate
// like `entity.due < '2026-02-01'` coerce the date literal at COMPILE
// time against the field's real layout — the metamodel knowledge (type,
// list-ness, date format) enters here as plain predicate.Type values, so
// the predicate package never imports metamodel (arch-fenced).
//
// Type mapping:
//   - integer            -> IntType
//   - date / datetime    -> DateTypeWithLayout(field format) so string
//     literals parse against the declared layout
//   - boolean            -> BoolType
//   - string / enum / other scalar -> StringType (enum values are treated
//     as plain strings in phase 1; see RR-XJBGB for the validation gap)
//   - any `list: true` property -> ListType{Elem: <scalar mapping>}
//
// A property whose predicate type would be a list wraps the scalar
// mapping; `contains(entity.tags, 'x')` then type-checks.
func EntityRecordType(def *metamodel.EntityDef) predicate.RecordType {
	rt := make(predicate.RecordType, len(def.Properties))
	for name, prop := range def.Properties {
		p := prop
		scalar := scalarPredicateType(&p)
		if prop.List {
			rt[name] = predicate.ListType{Elem: scalar}
			continue
		}
		rt[name] = scalar
	}
	return rt
}

// scalarPredicateType maps a single (non-list) property to its scalar
// predicate type.
func scalarPredicateType(prop *metamodel.PropertyDef) predicate.Type {
	switch prop.Type {
	case metamodel.PropertyTypeInteger:
		return predicate.IntType
	case metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime:
		return predicate.DateTypeWithLayout(prop.GetDateFormat())
	case metamodel.PropertyTypeBoolean:
		return predicate.BoolType
	default:
		// string, enum, file, rrule, and any custom/unknown type are
		// treated as strings. Enum-value validation (which filter.Match
		// performs) is intentionally NOT reproduced here in phase 1 —
		// see RR-XJBGB.
		return predicate.StringType
	}
}
