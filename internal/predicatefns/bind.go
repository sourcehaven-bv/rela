package predicatefns

import (
	"math"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// EntityRecord coerces an entity's stored property map into a
// predicate.Record whose field Values match the types EntityRecordType
// declares — the canonical entity->predicate binder shared by every
// consumer (CLI --filter, automation, validation) so binding can never
// drift from the type adapter.
//
// The declared metamodel type — not the raw Go type — drives each field:
// integer->Int, date/datetime->Date, boolean->Bool, list->List of the
// scalar, everything else->String. A missing, off-type, or unconvertible
// value binds Nil (fail-soft: permissive storage must not turn data drift
// into an Eval error). Fields whose type predicate cannot model (file,
// unknown) are omitted, matching EntityRecordType. `id` and `type` are
// always present as strings.
//
// This mirrors the affordances binder (bindings.go coerceScalar): both
// bind against the same declared types, so for the value shapes that
// reach them from markdown frontmatter (list props decode to []any) they
// agree. They are NOT yet a single implementation — affordances also
// binds current_user + the has_* host funcs on top, and its coerceList
// lacks this one's []string fast-path. Collapsing affordances onto this
// binder (keeping its extra bindings) is a tracked follow-up (RR-1NIV6A);
// until then, changes here that affect verdicts must be mirrored there.
func EntityRecord(
	meta *metamodel.Metamodel, def *metamodel.EntityDef, id, typ string, props map[string]any,
) predicate.Value {
	fields := map[string]predicate.Value{
		"id":   predicate.NewString(id),
		"type": predicate.NewString(typ),
	}
	for name, prop := range def.Properties {
		p := prop
		if _, ok := ScalarTypeForProp(meta, &p); !ok {
			continue // unmodelled type: omit (matches EntityRecordType)
		}
		fields[name] = coerceProp(meta, &p, props[name])
	}
	return predicate.NewRecord(fields)
}

// coerceProp coerces one raw stored value against its declared property
// type. List properties become a List of coerced scalars.
func coerceProp(meta *metamodel.Metamodel, prop *metamodel.PropertyDef, raw any) predicate.Value {
	if prop.List {
		return coerceList(meta, prop, raw)
	}
	return coerceScalar(meta, prop, raw)
}

func coerceList(meta *metamodel.Metamodel, prop *metamodel.PropertyDef, raw any) predicate.Value {
	elems := []predicate.Value{}
	switch v := raw.(type) {
	case []any:
		for _, e := range v {
			elems = append(elems, coerceScalar(meta, prop, e))
		}
	case []string:
		for _, e := range v {
			elems = append(elems, coerceScalar(meta, prop, e))
		}
	case nil:
		// empty list
	default:
		// a bare scalar reads as a one-element list
		elems = append(elems, coerceScalar(meta, prop, raw))
	}
	return predicate.NewList(elems)
}

func coerceScalar(meta *metamodel.Metamodel, prop *metamodel.PropertyDef, raw any) predicate.Value {
	if raw == nil {
		return predicate.NewNil()
	}
	switch prop.Type {
	case metamodel.PropertyTypeInteger:
		return coerceInt(raw)
	case metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime:
		return coerceDate(prop, raw)
	case metamodel.PropertyTypeBoolean:
		return coerceBool(raw)
	default:
		if s, ok := raw.(string); ok {
			return predicate.NewString(s)
		}
		// custom enum-like types resolve to string; a non-string raw
		// under one still binds Nil (off-type).
		_ = meta
		return predicate.NewNil()
	}
}

// coerceInt binds an integer field, preserving the permissive coercion
// (int/int64/integral-float64/parseable-string). A fractional float or
// non-numeric string binds Nil (no silent truncation).
func coerceInt(raw any) predicate.Value {
	switch v := raw.(type) {
	case int:
		return predicate.NewInt(int64(v))
	case int64:
		return predicate.NewInt(v)
	case float64:
		if v == math.Trunc(v) {
			return predicate.NewInt(int64(v))
		}
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return predicate.NewInt(n)
		}
	}
	return predicate.NewNil()
}

// coerceDate binds a date/datetime field. YAML decodes an unquoted date
// scalar to time.Time and a quoted one to string, so both are handled; a
// string is parsed via metamodel.ParseDateValue against the field format.
func coerceDate(prop *metamodel.PropertyDef, raw any) predicate.Value {
	switch v := raw.(type) {
	case time.Time:
		return predicate.NewDate(v)
	case string:
		if t, err := metamodel.ParseDateValue(v, prop); err == nil {
			return predicate.NewDate(t)
		}
	}
	return predicate.NewNil()
}

func coerceBool(raw any) predicate.Value {
	switch v := raw.(type) {
	case bool:
		return predicate.NewBool(v)
	case string:
		switch v {
		case "true":
			return predicate.NewBool(true)
		case "false":
			return predicate.NewBool(false)
		}
	}
	return predicate.NewNil()
}
