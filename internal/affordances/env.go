package affordances

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// userRecordType is the static type for the current_user variable.
// A predicate may reference:
//   - current_user.id   — the principal's user identity (the value
//     that appears on role-relation edges).
//   - current_user.tool — the entry-point tool the request came
//     through ("data-entry", "mcp", "cli", ...). It is NOT a user
//     classification; gate by role via has_role / has_global_role,
//     not by inspecting current_user.tool (M1).
var userRecordType = predicate.RecordType{
	"id":   predicate.StringType,
	"tool": predicate.StringType,
}

// buildEnv constructs the predicate environment for one entity type:
// the `entity` record (materialized from the metamodel), `current_user`,
// and the host-function signatures. The same env compiles every grant
// for that type.
//
// Properties whose metamodel type maps to a predicate type are
// declared; unsupported types are omitted so a predicate referencing
// them fails at compile with "unknown variable" rather than silently
// evaluating against a wrong runtime type (DR-C2). The `id` and `type`
// pseudo-fields are always present.
func buildEnv(meta *metamodel.Metamodel, entityType string) (*predicate.Env, error) {
	env := predicate.NewEnv()

	if err := env.DeclareVar("entity", entityRecordType(meta, entityType)); err != nil {
		return nil, err
	}
	if err := env.DeclareVar("current_user", userRecordType); err != nil {
		return nil, err
	}

	rec := predicate.RecordType{}
	str := predicate.StringType
	num := predicate.NumberType
	boolT := predicate.BoolType
	strList := predicate.ListType{Elem: predicate.StringType}

	funcs := []struct {
		name string
		sig  predicate.FuncSig
	}{
		{"has_role", predicate.FuncSig{Params: []predicate.Type{rec, rec, str}, Return: boolT}},
		{"has_global_role", predicate.FuncSig{Params: []predicate.Type{rec, str}, Return: boolT}},
		{"has_relation", predicate.FuncSig{Params: []predicate.Type{rec, str}, Return: boolT}},
		{"count_relations", predicate.FuncSig{Params: []predicate.Type{rec, str}, Return: num}},
		{"string_in_list", predicate.FuncSig{Params: []predicate.Type{str, strList}, Return: boolT}},
	}
	for _, f := range funcs {
		if err := env.DeclareFunc(f.name, f.sig); err != nil {
			return nil, err
		}
	}
	return env, nil
}

// entityRecordType builds the RecordType for an entity type's
// properties. id and type are always present; each property maps per
// propertyPredicateType. Unsupported property types are omitted (DR-C2).
func entityRecordType(meta *metamodel.Metamodel, entityType string) predicate.RecordType {
	rec := predicate.RecordType{
		"id":   predicate.StringType,
		"type": predicate.StringType,
	}
	if meta == nil {
		return rec
	}
	def, ok := meta.Entities[entityType]
	if !ok {
		return rec
	}
	for name, prop := range def.Properties {
		if t, ok := propertyPredicateType(meta, prop); ok {
			rec[name] = t
		}
	}
	return rec
}

// propertyPredicateType maps a metamodel property to a predicate type.
// Returns (_, false) for types the predicate type system can't model,
// so the caller omits them from the env (a predicate referencing them
// then fails at compile with "unknown variable" — DR-C2).
//
// The scalar type choice (integer->IntType, date->DateTypeWithLayout,
// boolean->BoolType, string/enum/custom->StringType, file/unknown->omit)
// is delegated to predicatefns.ScalarTypeForProp — the single canonical
// adapter (RR-TBG91). affordances keeps only its own policy: the list
// wrapping here, and the id/type pseudo-fields in entityRecordType. The
// list handling stays local because affordances declares list properties
// as ListType and binds them with coerceList; predicatefns owns the
// scalar decision, not the list-vs-scalar shape.
func propertyPredicateType(meta *metamodel.Metamodel, prop metamodel.PropertyDef) (predicate.Type, bool) {
	elem, ok := predicatefns.ScalarTypeForProp(meta, &prop)
	if !ok {
		return nil, false
	}
	if prop.List {
		return predicate.ListType{Elem: elem}, true
	}
	return elem, true
}
