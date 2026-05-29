// Package affordances builds a policy-driven field/option/relation
// affordance resolver from an acl.yaml [acl.Policy]. It computes, per
// entity, which fields are writable/visible, which enum options are
// allowed, and which relation operations are permitted, returning its
// own verdict types ([FieldVerdicts], [RelationVerdicts]). The
// data-entry layer adapts these into its FieldVerdictResolver wire
// shape — this package does not import internal/dataentry, keeping the
// dependency edge one-way (dataentry → affordances).
//
// Each grant in the policy may carry a `when:` predicate (see
// internal/predicate). Predicates are compiled once at construction;
// a compile failure aborts construction with all errors joined. At
// resolve time the predicate evaluates against the entity, the
// principal, and a small set of host functions (has_role,
// has_global_role, has_relation, count_relations, string_in_list).
//
// # Opt-in, closed-world per type
//
// A grant block (fields / visible / options / relations) is opt-in
// per entity type: when a role declares the block for type T, that
// role's grants are closed-world for T — anything not granted is
// denied. A role that does not declare the block for T contributes
// nothing (it neither grants nor shrinks). Cross-role semantics are a
// union: a field is writable if ANY of the principal's effective
// roles grants it under a passing predicate. This keeps access
// monotonic in the role set.
//
// # Effective roles are entity-scoped
//
// The effective role set for (principal, entity) is the principal's
// global roles (assignments ∪ default) unioned with local roles
// conferred on that entity by a role-relation edge from the principal
// (e.g. alice --owner-of--> TKT-001 confers "owner" on TKT-001 only).
// v1 resolves direct local roles only; inherited local roles are
// deferred.
package affordances

import (
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// compiledFieldGrant pairs a field name with its optional compiled
// predicate. A nil program means an unconditional grant.
type compiledFieldGrant struct {
	field   string
	program *predicate.Program
}

// compiledOptionGrant pairs a (field, option) with its predicate.
type compiledOptionGrant struct {
	field   string
	option  string
	program *predicate.Program
}

// compiledRelationGrant carries a relation type's create/remove
// verdicts, its meta-field grants, and an optional whole-grant
// predicate.
type compiledRelationGrant struct {
	relation string
	create   *bool
	remove   *bool
	fields   []compiledFieldGrant
	program  *predicate.Program
}
