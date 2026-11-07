package metamodel

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ReservedPrefix starts every type, relation and property name that rela
// manages itself, such as the `_flow-run` type and its `_carries` and
// `_produced` relations (FEAT-J0O10QI) and the order properties of an
// orderable relation. A schema may not declare a name that starts with it,
// so a rela-managed name never collides with an operator's.
const ReservedPrefix = entity.ManagedPrefix

// IsReservedName reports whether name, or its local part after a
// namespace (`isms._run`), starts with [ReservedPrefix].
func IsReservedName(name string) bool { return entity.IsManagedName(name) }

// validateReservedNames refuses a declared entity type, type alias,
// relation type, inverse relation or property whose name starts with
// [ReservedPrefix]. The managed order properties have their own, more
// specific message in [validateRelationProperties], so they are skipped
// here, and so are the definitions [WithFlowRuns] added.
func validateReservedNames(m *Metamodel) []string {
	return append(reservedEntityNames(m), reservedRelationNames(m)...)
}

// reservedMsg is the message of a reserved name declared as what.
func reservedMsg(what, name string) string {
	return fmt.Sprintf("%s %q: names starting with %q are reserved for rela", what, name, ReservedPrefix)
}

// reservedEntityNames is the entity half of [validateReservedNames].
func reservedEntityNames(m *Metamodel) []string {
	var errs []string
	reserved := func(what, name string) { errs = append(errs, reservedMsg(what, name)) }
	for _, name := range sortedKeys(m.Entities) {
		def := m.Entities[name]
		if m.isManagedDecl(name) {
			continue
		}
		if IsReservedName(name) {
			reserved("entity type", name)
		}
		for _, alias := range def.Aliases {
			if IsReservedName(alias) {
				reserved(fmt.Sprintf("entity %q: alias", name), alias)
			}
		}
		for _, prop := range sortedKeys(def.Properties) {
			if IsReservedName(prop) {
				reserved(fmt.Sprintf("entity %q: property", name), prop)
			}
		}
	}
	return errs
}

// reservedRelationNames is the relation half of [validateReservedNames].
func reservedRelationNames(m *Metamodel) []string {
	var errs []string
	reserved := func(what, name string) { errs = append(errs, reservedMsg(what, name)) }
	for _, name := range sortedKeys(m.Relations) {
		rel := m.Relations[name]
		if m.isManagedDecl(name) {
			continue
		}
		if IsReservedName(name) {
			reserved("relation type", name)
		}
		if rel.Inverse != nil && IsReservedName(rel.Inverse.ID) {
			reserved(fmt.Sprintf("relation %q: inverse", name), rel.Inverse.ID)
		}
		for _, prop := range sortedKeys(rel.Properties) {
			if prop == OrderPropertyOut || prop == OrderPropertyIn {
				continue
			}
			if IsReservedName(prop) {
				reserved(fmt.Sprintf("relation %q: property", name), prop)
			}
		}
	}
	return errs
}

// validateManagedReferences refuses an automation or validation rule that
// names a rela-managed type or relation. Only the flow engine writes those
// (TKT-BC5G8RP); an automation that fired on a run, created one, or linked
// one would act on rela's records with the operator's rules.
func validateManagedReferences(m *Metamodel) []string {
	var errs []string
	ref := func(where, name string) {
		if IsReservedName(name) {
			errs = append(errs, fmt.Sprintf("%s: %q is managed by rela and cannot be named here", where, name))
		}
	}
	for _, auto := range m.Automations {
		where := fmt.Sprintf("automation %q", auto.Name)
		for _, t := range auto.On.Entity {
			ref(where+": on.entity", t)
		}
		ref(where+": on.relation_created", auto.On.RelationCreated)
		ref(where+": on.relation_removed", auto.On.RelationRemoved)
		for _, a := range auto.Do {
			if a.CreateRelation != nil {
				ref(where+": create_relation.relation", a.CreateRelation.Relation)
			}
			if a.CreateEntity != nil {
				ref(where+": create_entity.type", a.CreateEntity.Type)
				ref(where+": create_entity.relation", a.CreateEntity.Relation)
			}
		}
	}
	for _, v := range m.Validations {
		ref(fmt.Sprintf("validation %q: entity_type", v.Name), v.EntityType)
	}
	return errs
}
