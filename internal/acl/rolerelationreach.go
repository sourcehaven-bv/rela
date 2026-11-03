package acl

import (
	"maps"
	"slices"
	"strings"
)

// RoleRelationsNoMemberHolds returns, sorted, the `role_relations:` keys of p that
// confer a role but whose declared source types include no type a principal
// or its groups can be. The resolver looks for an edge from a member to the
// entity, so such an entry never matches. It still costs a lookup per member
// per entity on every unprimed check (BUG-E2W1LS), and the operator believes
// it grants a role. The usual cause is an entry declared against the
// relation's direction.
//
// The member types are [Policy.UserEntityType] and, transitively, the target
// types of the membership relation. It returns nil when user_entity_type is
// unset, because a principal may then be any entity. An entry whose relation
// is undeclared, or declares a source type "*", is not reported. It is a
// function rather than a method so Policy stays under its method budget.
//
// Nil: accepted — a nil p or meta returns nil.
func RoleRelationsNoMemberHolds(p *Policy, meta MetamodelView) []string {
	if p == nil || meta == nil {
		return nil
	}
	userType := strings.TrimSpace(p.UserEntityType)
	if userType == "" {
		return nil
	}
	canonical := func(t string) string {
		c, _ := meta.FaceNames(t)
		if c == "" {
			return t
		}
		return c
	}
	members := memberTypes(meta.RelationInfo(p.EffectiveMembershipRelation()), canonical(userType), canonical)
	var dead []string
	for _, relType := range slices.Sorted(maps.Keys(p.RoleRelations)) {
		if p.RoleRelations[relType].Confers == "" {
			continue
		}
		info := meta.RelationInfo(relType)
		if !info.Exists || len(info.From) == 0 {
			continue
		}
		if !slices.ContainsFunc(info.From, func(t string) bool { return t == "*" || members[canonical(t)] }) {
			dead = append(dead, relType)
		}
	}
	return dead
}

// memberTypes is the set of types a membership walk can reach from userType:
// userType itself, plus every target of the membership relation when its
// source admits a member. One check suffices, because adding the targets
// cannot admit a further target.
func memberTypes(m RelationInfo, userType string, canonical func(string) string) map[string]bool {
	members := map[string]bool{userType: true}
	if !m.Exists {
		return members
	}
	if slices.ContainsFunc(m.From, func(t string) bool { return t == "*" || members[canonical(t)] }) {
		for _, t := range m.To {
			members[canonical(t)] = true
		}
	}
	return members
}
