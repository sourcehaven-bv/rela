package affordances

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// errArgType is returned when a host func receives an argument whose
// runtime type doesn't match its declared signature. The compile-time
// type checker should prevent this; the runtime guard fails the Eval
// (which the resolver treats as deny — fail closed).
var errArgType = errors.New("affordances: host function argument type mismatch")

// hasRole reports whether the principal holds role_name scoped to the
// given entity: global roles ∪ local roles conferred on the entity.
// Signature: has_role(user_record, entity_record, role_name) bool.
//
// The user and entity records are ignored beyond confirming arity —
// the binding closes over the principal and entity, so the predicate
// can't ask about a different user or entity than the one in scope.
func (bc *bindingContext) hasRole(ctx context.Context, args []predicate.Value) (predicate.Value, error) {
	role, ok := stringArg(args, 2)
	if !ok {
		return nil, errArgType
	}
	if bc.globalRoles[role] {
		return predicate.NewBool(true), nil
	}
	return predicate.NewBool(bc.holdsLocalRole(ctx, role)), nil
}

// hasGlobalRole reports whether the principal holds role_name as a
// global (assignment-based) role, ignoring local roles.
// Signature: has_global_role(user_record, role_name) bool.
func (bc *bindingContext) hasGlobalRole(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	role, ok := stringArg(args, 1)
	if !ok {
		return nil, errArgType
	}
	return predicate.NewBool(bc.globalRoles[role]), nil
}

// hasRelation reports whether the in-scope entity has any outgoing
// relation of the given type. Signature: has_relation(entity, type) bool.
func (bc *bindingContext) hasRelation(ctx context.Context, args []predicate.Value) (predicate.Value, error) {
	relType, ok := stringArg(args, 1)
	if !ok {
		return nil, errArgType
	}
	return predicate.NewBool(bc.outgoingCounts(ctx)[relType] > 0), nil
}

// countRelations returns the number of outgoing relations of the given
// type from the in-scope entity. Signature: count_relations(entity, type) number.
func (bc *bindingContext) countRelations(ctx context.Context, args []predicate.Value) (predicate.Value, error) {
	relType, ok := stringArg(args, 1)
	if !ok {
		return nil, errArgType
	}
	return predicate.NewNumberFromInt(bc.outgoingCounts(ctx)[relType]), nil
}

// stringInList reports whether value is an element of allowed.
// Signature: string_in_list(value string, allowed list_of_string) bool.
func stringInList(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	if len(args) != 2 {
		return nil, errArgType
	}
	value, ok := args[0].(predicate.String)
	if !ok {
		return nil, errArgType
	}
	list, ok := args[1].(predicate.List)
	if !ok {
		return nil, errArgType
	}
	for _, e := range list.Elems() {
		if s, ok := e.(predicate.String); ok && s.String() == value.String() {
			return predicate.NewBool(true), nil
		}
	}
	return predicate.NewBool(false), nil
}

// holdsLocalRole reports whether the principal holds role via a
// role-relation edge to the in-scope entity. For each role-relation
// type R conferring role, it checks principal --R--> entity. v1 =
// direct local roles only.
func (bc *bindingContext) holdsLocalRole(ctx context.Context, role string) bool {
	for _, relType := range bc.localRoleRelations(role) {
		if bc.lookup.HasEdge(ctx, bc.userID, relType, bc.entity.ID) {
			return true
		}
	}
	return false
}

// stringArg returns the string value at args[i] when args has exactly
// want elements and args[i] is a predicate.String.
func stringArg(args []predicate.Value, i int) (string, bool) {
	want := i + 1
	if len(args) != want {
		return "", false
	}
	s, ok := args[i].(predicate.String)
	if !ok {
		return "", false
	}
	return s.String(), true
}
