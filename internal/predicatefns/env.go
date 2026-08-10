package predicatefns

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// ScalarType maps a scalar metamodel property type to its canonical
// predicate type — the single source of truth for the type CHOICE that
// every predicate consumer shares (RR-TBG91, RR-23W88J):
//
//   - integer            -> IntType
//   - date / datetime     -> DateTypeWithLayout(field format) so string
//     literals coerce against the field's declared layout at compile
//   - boolean             -> BoolType
//   - string / enum / rrule / an enum-like custom (named) type -> StringType
//   - file, and any type predicate cannot model -> (nil, false)
//
// The (Type, ok) shape lets a caller OMIT a field it can't model so a
// predicate referencing it fails at compile ("unknown variable") rather
// than evaluating against a wrong runtime type. `meta` is consulted only
// to resolve a custom named type to string-valued; pass nil to skip that
// resolution (a named type then reports unmodelled).
//
// This function owns the int/date/bool decision; consumers
// (predicatefns.EntityRecordType, affordances) share it so the
// integer->Int / date->Date choice can never drift between them again.
func ScalarType(meta *metamodel.Metamodel, typeName string) (predicate.Type, bool) {
	switch typeName {
	case metamodel.PropertyTypeInteger:
		return predicate.IntType, true
	case metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime:
		// Layout is resolved per-property by ScalarTypeForProp; without a
		// PropertyDef here we fall back to the bare DateType (default
		// layouts). Callers that have the PropertyDef should use
		// ScalarTypeForProp to honor a custom `format:`.
		return predicate.DateType, true
	case metamodel.PropertyTypeBoolean:
		return predicate.BoolType, true
	case "", metamodel.PropertyTypeString, metamodel.PropertyTypeEnum,
		metamodel.PropertyTypeRrule:
		return predicate.StringType, true
	case metamodel.PropertyTypeFile:
		return nil, false
	}
	// Custom named types: an enum-like custom type carries string values.
	if meta != nil {
		if _, ok := meta.Types[typeName]; ok {
			return predicate.StringType, true
		}
	}
	return nil, false
}

// ScalarTypeForProp is ScalarType with the property's declared date
// layout applied: a date/datetime property maps to
// DateTypeWithLayout(prop.GetDateFormat()) so string date literals in a
// predicate parse against the field's real format at compile time. For
// every non-date type it defers to ScalarType.
func ScalarTypeForProp(meta *metamodel.Metamodel, prop *metamodel.PropertyDef) (predicate.Type, bool) {
	switch prop.Type {
	case metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime:
		return predicate.DateTypeWithLayout(prop.GetDateFormat()), true
	default:
		return ScalarType(meta, prop.Type)
	}
}

// EntityRecordType builds the predicate.RecordType for an entity type by
// mapping each declared property to its predicate type via
// ScalarTypeForProp. This is the metamodel->Env adapter: it is what lets
// a predicate like `entity.due < '2026-02-01'` coerce the date literal at
// COMPILE time against the field's real layout — the metamodel knowledge
// (type, list-ness, date format) enters here as plain predicate.Type
// values, so the predicate package never imports metamodel (arch-fenced).
//
// A property whose type predicate cannot model (file, unknown) is OMITTED
// — a predicate referencing it fails at compile rather than evaluating
// against a wrong type. A `list: true` property wraps the scalar mapping
// in a ListType; `contains(entity.tags, 'x')` then type-checks.
//
// This is the SINGLE canonical adapter (RR-TBG91). affordances shares the
// scalar type choice via ScalarType/ScalarTypeForProp; it layers its own
// id/type pseudo-fields on top (see affordances.entityRecordType).
func EntityRecordType(meta *metamodel.Metamodel, def *metamodel.EntityDef) predicate.RecordType {
	rt := make(predicate.RecordType, len(def.Properties))
	for name, prop := range def.Properties {
		p := prop
		scalar, ok := ScalarTypeForProp(meta, &p)
		if !ok {
			continue // unmodelled type: omit so a reference fails at compile
		}
		if prop.List {
			rt[name] = predicate.ListType{Elem: scalar}
			continue
		}
		rt[name] = scalar
	}
	return rt
}
